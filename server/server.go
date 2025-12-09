package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/evanw/esbuild/pkg/api"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/execute/tsc"
	"github.com/microsoft/typescript-go/internal/locale"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs"
)

// S3ClientInterface defines the interface for S3 operations
type S3ClientInterface interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

var (
	serverVersion = "1.0.0"
	startTime     = time.Now()
	gitCommit     = "unknown" // Set at build time with -ldflags
	
	// S3 client and configuration
	s3Client    S3ClientInterface
	s3Bucket    string
	
	// LRU cache for S3 content
	cache       *lru.Cache[string, *CacheEntry]
	cacheMutex  sync.RWMutex
	
	// Cache configuration
	cacheSize   int64
	
	// Cache TTL settings
	cacheTTLSuccess   = 24 * time.Hour   // TTL for found entries
	cacheTTLNotFound  = 60 * time.Second // TTL for not found entries
	
	// Pre-warming tracking
	cachePrewarmStarted   time.Time
	cachePrewarmCompleted time.Time
	cachePrewarmBytes     int64
	cachePrewarmFiles     int
)

type CacheEntry struct {
	Exists    bool      // false if not found in S3
	IsFile    bool      // true for file, false for directory
	Content   []byte    // file content (only for files)
	Files     []string  // immediate child files (only for dirs)
	Dirs      []string  // immediate child dirs (only for dirs)
	ExpiresAt time.Time // expiration time for this entry
}

type TypecheckRequest struct {
	Code    string `json:"code"`
	Version string `json:"version"`
}

type TypecheckResponse struct {
	Pass   bool              `json:"pass,omitempty"`
	Errors []DiagnosticError `json:"errors,omitempty"`
}

type BuildRequest struct {
	Code    string `json:"code"`
	Version string `json:"version"`
}

type BuildResponse struct {
	Code   string            `json:"code,omitempty"`
	Errors []DiagnosticError `json:"errors,omitempty"`
}

type DiagnosticError struct {
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

// V2 API types for multi-file support
type TypecheckV2Request struct {
	Files       map[string]string `json:"files"`                 // path -> content
	EntryPoints []string          `json:"entryPoints,omitempty"` // optional, defaults to all .ts/.tsx
	Version     string            `json:"version"`
}

type BuildV2Request struct {
	Files      map[string]string `json:"files"`      // path -> content
	EntryPoint string            `json:"entryPoint"` // required for bundling
	Version    string            `json:"version"`
}

type DiagnosticErrorV2 struct {
	File    string `json:"file"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type TypecheckV2Response struct {
	Pass   bool                `json:"pass,omitempty"`
	Errors []DiagnosticErrorV2 `json:"errors,omitempty"`
}

type BuildV2Response struct {
	Code   string              `json:"code,omitempty"`
	Errors []DiagnosticErrorV2 `json:"errors,omitempty"`
}

// V2 API limits
const (
	maxFilesPerRequest = 100
	maxFileSizeBytes   = 1 * 1024 * 1024  // 1MB per file
	maxTotalSizeBytes  = 10 * 1024 * 1024 // 10MB total
)

type HealthResponse struct {
	Status         string            `json:"status"`
	Version        string            `json:"version"`
	Uptime         string            `json:"uptime"`
	CacheSize      string            `json:"cache_size"`
	CacheEntries   int               `json:"cache_entries"`
	CachePrewarm   CachePrewarmStatus `json:"cache_prewarm,omitempty"`
}

type CachePrewarmStatus struct {
	Status    string  `json:"status"`
	Files     int     `json:"files"`
	BytesMB   float64 `json:"bytes_mb"`
	DurationS float64 `json:"duration_s,omitempty"`
}

type s3FS struct {
	version      string
	files        map[string]string // in-memory cache for this request
	mu           sync.RWMutex      // protects files map
	hasUserFiles bool              // true if user provided multiple files (v2 mode)
}

func newS3FS(version string) *s3FS {
	return &s3FS{
		version: version,
		files: map[string]string{
			"/input.ts": "",
		},
	}
}

// normalizeAndValidatePath ensures paths are absolute and prevents security issues
func normalizeAndValidatePath(path string) (string, error) {
	// Ensure absolute path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Clean the path (resolve . and ..)
	path = filepath.Clean(path)
	path = strings.ReplaceAll(path, "\\", "/")

	// Security: prevent node_modules override
	if strings.HasPrefix(path, "/node_modules/") || path == "/node_modules" {
		return "", fmt.Errorf("cannot override node_modules: %s", path)
	}

	// Security: prevent lib override (TypeScript lib files)
	if strings.HasPrefix(path, "/lib.") || strings.HasPrefix(path, "/lib/") {
		return "", fmt.Errorf("cannot override TypeScript lib: %s", path)
	}

	return path, nil
}

// validateV2Files checks file count and size limits
func validateV2Files(files map[string]string) error {
	if len(files) == 0 {
		return fmt.Errorf("at least one file is required")
	}

	if len(files) > maxFilesPerRequest {
		return fmt.Errorf("too many files: %d (max %d)", len(files), maxFilesPerRequest)
	}

	var totalSize int64
	for path, content := range files {
		size := int64(len(content))
		if size > maxFileSizeBytes {
			return fmt.Errorf("file too large: %s (%d bytes, max %d)", path, size, maxFileSizeBytes)
		}
		totalSize += size
	}

	if totalSize > maxTotalSizeBytes {
		return fmt.Errorf("total size too large: %d bytes (max %d)", totalSize, maxTotalSizeBytes)
	}

	return nil
}

// getFromCache retrieves an entry from cache or fetches from S3
func getFromCache(ctx context.Context, version, path string) *CacheEntry {
	start := time.Now()
	defer func() {
		if duration := time.Since(start); duration > 10*time.Millisecond {
			log.Printf("[PERF] getFromCache slow: %v for path=%s", duration, path)
		}
	}()
	
	// Normalize version - ensure it ends with exactly one slash
	if !strings.HasSuffix(version, "/") {
		version = version + "/"
	}
	cacheKey := fmt.Sprintf("%s%s", version, strings.TrimPrefix(path, "/"))
	
	// Check cache first
	cacheMutex.RLock()
	if cached, ok := cache.Get(cacheKey); ok {
		// Check if entry has expired
		if time.Now().Before(cached.ExpiresAt) {
			cacheMutex.RUnlock()
			s3CacheHits.Inc()
			// Log important cache hits
			if strings.Contains(path, "react") && strings.Contains(path, ".d.ts") {
				log.Printf("[CACHE HIT] Found in cache: %s", cacheKey)
			}
			return cached
		}
		// Entry expired, will refetch
		cacheMutex.RUnlock()
		// Remove expired entry
		cacheMutex.Lock()
		cache.Remove(cacheKey)
		cacheMutex.Unlock()
	} else {
		cacheMutex.RUnlock()
	}
	
	// Before hitting S3, check if parent directory is cached
	// If so, we can determine if this file doesn't exist without S3
	dirPath := filepath.Dir(strings.TrimPrefix(path, "/"))
	if dirPath != "." && dirPath != "" {
		dirCacheKey := fmt.Sprintf("%s%s", version, dirPath)
		cacheMutex.RLock()
		if dirEntry, ok := cache.Get(dirCacheKey); ok && time.Now().Before(dirEntry.ExpiresAt) {
			cacheMutex.RUnlock()
			if dirEntry.Exists && !dirEntry.IsFile {
				// Parent directory is cached, check if file is in the list
				filename := filepath.Base(path)
				fileFound := false
				for _, f := range dirEntry.Files {
					if f == filename {
						fileFound = true
						break
					}
				}
				if !fileFound {
					// File not in parent directory's file list, so it doesn't exist
					s3CacheHits.Inc() // This is still a cache hit - we used cached dir info
					entry := &CacheEntry{
						Exists:    false,
						ExpiresAt: time.Now().Add(cacheTTLNotFound),
					}
					cacheMutex.Lock()
					cache.Add(cacheKey, entry)
					cacheMutex.Unlock()
					return entry
				}
				// File is in directory listing, continue to fetch it from S3 or cache
			}
		} else {
			cacheMutex.RUnlock()
		}
	}
	
	s3CacheMisses.Inc()
	
	// Try to fetch as a file first
	s3FetchStart := time.Now()
	// Create timeout context for S3 operation (1 second timeout)
	s3Ctx, s3Cancel := context.WithTimeout(ctx, 1*time.Second)
	defer s3Cancel()
	result, err := s3Client.GetObject(s3Ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(cacheKey),
	})
	s3FetchDuration.Observe(time.Since(s3FetchStart).Seconds())
	if duration := time.Since(s3FetchStart); duration > 10*time.Millisecond {
		log.Printf("[PERF] S3 GetObject: %v for key=%s", duration, cacheKey)
	}
	
	if err == nil {
		defer result.Body.Close()
		content, err := io.ReadAll(result.Body)
		if err == nil {
			// Log important files being loaded
			if strings.Contains(cacheKey, "react") && strings.Contains(cacheKey, ".d.ts") {
				log.Printf("[S3 FETCH] Loading type definitions: %s (%d bytes)", cacheKey, len(content))
			}
			entry := &CacheEntry{
				Exists:    true,
				IsFile:    true,
				Content:   content,
				ExpiresAt: time.Now().Add(cacheTTLSuccess),
			}
			cacheMutex.Lock()
			cache.Add(cacheKey, entry)
			cacheMutex.Unlock()
			return entry
		} else {
			// log.Printf("[getFromCache] S3 file read error for %s: %v", cacheKey, err)
		}
	} else {
		// log.Printf("[getFromCache] S3 file fetch error for %s: %v", cacheKey, err)
	}
	
	// Not a file, try as directory
	prefix := cacheKey
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	
	s3ListStart := time.Now()
	// Create timeout context for S3 list operation (5 second timeout)
	listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
	defer listCancel()
	listResult, err := s3Client.ListObjectsV2(listCtx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s3Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	s3ListDuration.Observe(time.Since(s3ListStart).Seconds())
	if duration := time.Since(s3ListStart); duration > 10*time.Millisecond {
		log.Printf("[PERF] S3 ListObjectsV2: %v for prefix=%s", duration, prefix)
	}
	
	if err != nil {
		s3ListErrors.Inc()
		// log.Printf("[getFromCache] S3 list error for key %s: %v", cacheKey, err)
		// Cache as non-existent with shorter TTL
		entry := &CacheEntry{
			Exists:    false,
			ExpiresAt: time.Now().Add(cacheTTLNotFound),
		}
		cacheMutex.Lock()
		cache.Add(cacheKey, entry)
		cacheMutex.Unlock()
		return entry
	}
	
	// Build directory entry
	entry := &CacheEntry{
		Exists: false,
		IsFile: false,
		Files:  []string{},
		Dirs:   []string{},
	}
	
	// Add files
	for _, obj := range listResult.Contents {
		if obj.Key != nil {
			relativePath := strings.TrimPrefix(*obj.Key, prefix)
			if relativePath != "" && !strings.Contains(relativePath, "/") {
				entry.Files = append(entry.Files, relativePath)
				entry.Exists = true
			}
		}
	}
	
	// Add directories
	for _, commonPrefix := range listResult.CommonPrefixes {
		if commonPrefix.Prefix != nil {
			dirName := strings.TrimPrefix(*commonPrefix.Prefix, prefix)
			dirName = strings.TrimSuffix(dirName, "/")
			if dirName != "" {
				entry.Dirs = append(entry.Dirs, dirName)
				entry.Exists = true
			}
		}
	}
	
	// Set expiration based on whether entry was found
	if entry.Exists {
		entry.ExpiresAt = time.Now().Add(cacheTTLSuccess)
	} else {
		entry.ExpiresAt = time.Now().Add(cacheTTLNotFound)
	}
	
	// Cache the result
	cacheMutex.Lock()
	cache.Add(cacheKey, entry)
	cacheMutex.Unlock()
	
	return entry
}

// compareVersions compares two semantic versions
// Returns true if v1 > v2
func compareVersions(v1, v2 string) bool {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")
	
	// Handle pre-release versions (e.g., 5.7.0-beta)
	v1Base := v1
	v2Base := v2
	v1Pre := ""
	v2Pre := ""
	
	if idx := strings.IndexAny(v1, "-+"); idx != -1 {
		v1Base = v1[:idx]
		v1Pre = v1[idx:]
	}
	if idx := strings.IndexAny(v2, "-+"); idx != -1 {
		v2Base = v2[:idx]
		v2Pre = v2[idx:]
	}
	
	// Split base version into parts
	parts1 := strings.Split(v1Base, ".")
	parts2 := strings.Split(v2Base, ".")
	
	// Compare each numeric part
	maxParts := len(parts1)
	if len(parts2) > maxParts {
		maxParts = len(parts2)
	}
	
	for i := 0; i < maxParts; i++ {
		var num1, num2 int
		
		if i < len(parts1) {
			num1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			num2, _ = strconv.Atoi(parts2[i])
		}
		
		if num1 != num2 {
			return num1 > num2
		}
	}
	
	// If base versions are equal, compare pre-release versions
	// No pre-release > has pre-release (5.7.0 > 5.7.0-beta)
	if v1Pre == "" && v2Pre != "" {
		return true
	}
	if v1Pre != "" && v2Pre == "" {
		return false
	}
	
	// Both have pre-release, compare lexically
	return v1Pre > v2Pre
}

// prewarmCache loads S3 files into cache on startup
func prewarmCache(ctx context.Context) {
	cachePrewarmStarted = time.Now()
	log.Printf("Starting cache pre-warm...")
	
	// Track bytes loaded to prevent overflow
	var bytesLoadedMutex sync.Mutex
	var bytesLoaded int64
	var filesLoaded int
	targetBytes := cacheSize // Use 100% of cache
	
	// List all versions (sorted newest first) with timeout
	listVersionsCtx, listVersionsCancel := context.WithTimeout(ctx, 5*time.Second)
	versionsResult, err := s3Client.ListObjectsV2(listVersionsCtx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s3Bucket),
		Delimiter: aws.String("/"),
	})
	listVersionsCancel()
	if err != nil {
		log.Printf("Failed to list S3 versions for pre-warm: %v", err)
		return
	}
	
	// Extract version prefixes and sort (newest first)
	var versions []string
	for _, prefix := range versionsResult.CommonPrefixes {
		if prefix.Prefix != nil {
			version := strings.TrimSuffix(*prefix.Prefix, "/")
			versions = append(versions, version)
		}
	}
	
	// Sort versions in reverse order using semantic versioning
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j])
	})
	
	// Create a worker pool for parallel loading
	const numWorkers = 10
	type loadJob struct {
		version string
		obj     types.Object
	}
	
	jobsChan := make(chan loadJob, 100)
	resultsChan := make(chan int64, 100)
	var wg sync.WaitGroup
	
	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				// Extract path from key (remove version prefix)
				path := "/" + strings.TrimPrefix(*job.obj.Key, job.version+"/")
				
				// Check if already cached
				cacheKey := fmt.Sprintf("%s/%s", job.version, strings.TrimPrefix(path, "/"))
				cacheMutex.RLock()
				_, exists := cache.Get(cacheKey)
				cacheMutex.RUnlock()
				if exists {
					resultsChan <- 0
					continue
				}
				
				// Create timeout context for S3 operation (1 second timeout)
				s3Ctx, s3Cancel := context.WithTimeout(ctx, 1*time.Second)
				
				// Fetch and cache the file
				result, err := s3Client.GetObject(s3Ctx, &s3.GetObjectInput{
					Bucket: aws.String(s3Bucket),
					Key:    job.obj.Key,
				})
				if err != nil {
					s3Cancel()
					resultsChan <- 0
					continue
				}
				
				content, err := io.ReadAll(result.Body)
				result.Body.Close()
				s3Cancel()
				if err != nil {
					resultsChan <- 0
					continue
				}
				
				// Add to cache
				entry := &CacheEntry{
					Exists:    true,
					IsFile:    true,
					Content:   content,
					ExpiresAt: time.Now().Add(cacheTTLSuccess),
				}
				
				cacheMutex.Lock()
				cache.Add(cacheKey, entry)
				cacheMutex.Unlock()
				
				resultsChan <- *job.obj.Size
			}
		}()
	}
	
	// Start a goroutine to collect results
	var resultsWg sync.WaitGroup
	resultsWg.Add(1)
	go func() {
		defer resultsWg.Done()
		for size := range resultsChan {
			if size > 0 {
				bytesLoadedMutex.Lock()
				bytesLoaded += size
				filesLoaded++
				currentBytes := bytesLoaded
				currentFiles := filesLoaded
				bytesLoadedMutex.Unlock()
				
				// Log progress every 50 files
				if currentFiles%50 == 0 {
					log.Printf("Pre-warmed %d files (%.1f MB / %.1f MB)", 
						currentFiles, 
						float64(currentBytes)/(1024*1024),
						float64(targetBytes)/(1024*1024))
				}
			}
		}
	}()
	
	// Load files from each version
	outerLoop:
	for _, version := range versions {
		// Check context cancellation
		select {
		case <-ctx.Done():
			log.Printf("Cache pre-warm cancelled")
			break outerLoop
		default:
		}
		
		bytesLoadedMutex.Lock()
		currentBytes := bytesLoaded
		bytesLoadedMutex.Unlock()
		if currentBytes >= targetBytes {
			break
		}
		
		// First, collect all unique directories from the file paths
		allDirs := make(map[string]bool)
		
		// List all files in this version with pagination
		var continuationToken *string
		allFiles := []types.Object{}
		for {
			listInput := &s3.ListObjectsV2Input{
				Bucket: aws.String(s3Bucket),
				Prefix: aws.String(version + "/"),
			}
			if continuationToken != nil {
				listInput.ContinuationToken = continuationToken
			}
			
			// Create timeout context for listing objects (5 second timeout)
			listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
			listResult, err := s3Client.ListObjectsV2(listCtx, listInput)
			listCancel()
			if err != nil {
				log.Printf("Failed to list objects for version %s: %v", version, err)
				break
			}
			
			// Collect all files and track directories
			for _, obj := range listResult.Contents {
				if obj.Key != nil {
					allFiles = append(allFiles, obj)
					// Extract all parent directories
					path := strings.TrimPrefix(*obj.Key, version+"/")
					dir := filepath.Dir(path)
					for dir != "." && dir != "" {
						allDirs[dir] = true
						dir = filepath.Dir(dir)
					}
				}
			}
			
			// Queue files for loading
			for _, obj := range listResult.Contents {
			if obj.Key == nil || obj.Size == nil {
				continue
			}
			
			// Skip large TypeScript compiler files
			if strings.Contains(*obj.Key, "typescript/lib/") && 
			   (strings.HasSuffix(*obj.Key, "typescript.js") || 
			    strings.HasSuffix(*obj.Key, "_tsc.js") ||
			    *obj.Size > 1024*1024) { // Skip files over 1MB in typescript/lib
				continue
			}
			
				// Check if we have space
				bytesLoadedMutex.Lock()
				currentBytes := bytesLoaded
				bytesLoadedMutex.Unlock()
				if currentBytes + *obj.Size > targetBytes {
					break outerLoop
				}
			
				// Send to workers
				select {
				case jobsChan <- loadJob{version: version, obj: obj}:
				case <-ctx.Done():
					break outerLoop
				}
			}
			
			// Check if there are more pages
			if listResult.IsTruncated != nil && *listResult.IsTruncated {
				continuationToken = listResult.NextContinuationToken
			} else {
				break
			}
		}
		
		// Cache each directory by doing a ListObjectsV2 with delimiter
		dirCount := 0
		for dir := range allDirs {
			prefix := version + "/" + dir + "/"
			
			// List this specific directory
			listCtx, listCancel := context.WithTimeout(ctx, 2*time.Second)
			listResult, err := s3Client.ListObjectsV2(listCtx, &s3.ListObjectsV2Input{
				Bucket:    aws.String(s3Bucket),
				Prefix:    aws.String(prefix),
				Delimiter: aws.String("/"),
			})
			listCancel()
			
			if err != nil {
				continue
			}
			
			// Build the directory entry
			entry := &CacheEntry{
				Exists: true,
				IsFile: false,
				Files:  []string{},
				Dirs:   []string{},
				ExpiresAt: time.Now().Add(cacheTTLSuccess),
			}
			
			// Add immediate files
			for _, obj := range listResult.Contents {
				if obj.Key != nil {
					relativePath := strings.TrimPrefix(*obj.Key, prefix)
					if relativePath != "" && !strings.Contains(relativePath, "/") {
						entry.Files = append(entry.Files, relativePath)
					}
				}
			}
			
			// Add immediate subdirectories
			for _, commonPrefix := range listResult.CommonPrefixes {
				if commonPrefix.Prefix != nil {
					dirName := strings.TrimPrefix(*commonPrefix.Prefix, prefix)
					dirName = strings.TrimSuffix(dirName, "/")
					if dirName != "" {
						entry.Dirs = append(entry.Dirs, dirName)
					}
				}
			}
			
			// Cache the directory
			cacheKey := version + "/" + dir
			cacheMutex.Lock()
			cache.Add(cacheKey, entry)
			cacheMutex.Unlock()
			dirCount++
		}
		
		log.Printf("Cached %d directories for version %s", dirCount, version)
	}
	
	// Close jobs channel and wait for workers to finish
	close(jobsChan)
	wg.Wait()
	close(resultsChan)
	
	// Wait for results collector to finish
	resultsWg.Wait()
	
	cachePrewarmCompleted = time.Now()
	bytesLoadedMutex.Lock()
	cachePrewarmBytes = bytesLoaded
	cachePrewarmFiles = filesLoaded
	bytesLoadedMutex.Unlock()
	
	duration := cachePrewarmCompleted.Sub(cachePrewarmStarted)
	log.Printf("Cache pre-warm completed: %d files (%.1f MB) loaded in %v",
		filesLoaded, float64(bytesLoaded)/(1024*1024), duration.Round(time.Millisecond))
	
	// Set Prometheus metrics
	cachePrewarmDuration.Set(duration.Seconds())
	cachePrewarmFilesLoaded.Set(float64(filesLoaded))
	cachePrewarmBytesLoaded.Set(float64(bytesLoaded))
}


func (fs *s3FS) UseCaseSensitiveFileNames() bool { return true }

func (fs *s3FS) FileExists(path string) bool {
	// Check in-memory cache first
	fs.mu.RLock()
	_, ok := fs.files[path]
	fs.mu.RUnlock()
	if ok {
		return true
	}
	
	// Skip input file
	if strings.HasPrefix(path, "/input.") {
		return false
	}
	
	// Check S3 cache
	ctx := context.Background()
	entry := getFromCache(ctx, fs.version, path)
	return entry.Exists && entry.IsFile
}

func (fs *s3FS) ReadFile(path string) (string, bool) {
	// Check in-memory cache first
	fs.mu.RLock()
	content, ok := fs.files[path]
	fs.mu.RUnlock()
	if ok {
		return content, true
	}
	
	// Skip input file
	if strings.HasPrefix(path, "/input.") {
		return "", false
	}
	
	// Check S3 cache
	ctx := context.Background()
	entry := getFromCache(ctx, fs.version, path)
	if !entry.Exists || !entry.IsFile {
		return "", false
	}
	
	contentStr := string(entry.Content)
	fs.mu.Lock()
	fs.files[path] = contentStr // Cache locally for this request
	fs.mu.Unlock()
	return contentStr, true
}

func (fs *s3FS) WriteFile(path string, data string, _ bool) error {
	fs.mu.Lock()
	fs.files[path] = data
	fs.mu.Unlock()
	return nil
}

func (fs *s3FS) Remove(path string) error {
	fs.mu.Lock()
	delete(fs.files, path)
	fs.mu.Unlock()
	return nil
}

func (fs *s3FS) Chtimes(path string, aTime time.Time, mTime time.Time) error {
	// S3-backed file system doesn't support changing file times
	// This is a no-op implementation for compatibility
	return nil
}

func (fs *s3FS) DirectoryExists(path string) bool {
	if path == "/" || path == "" {
		return true
	}

	// Only check in-memory files for v2 mode (multi-file requests)
	fs.mu.RLock()
	hasUserFiles := fs.hasUserFiles
	if hasUserFiles {
		prefix := path
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		for filePath := range fs.files {
			if strings.HasPrefix(filePath, prefix) {
				fs.mu.RUnlock()
				return true
			}
		}
	}
	fs.mu.RUnlock()

	// Check S3 cache
	ctx := context.Background()
	entry := getFromCache(ctx, fs.version, path)
	return entry.Exists && !entry.IsFile
}

func (fs *s3FS) GetAccessibleEntries(path string) vfs.Entries {
	filesSet := make(map[string]struct{})
	dirsSet := make(map[string]struct{})

	// Only scan in-memory files for v2 mode (multi-file requests)
	fs.mu.RLock()
	hasUserFiles := fs.hasUserFiles
	if hasUserFiles {
		prefix := path
		if prefix != "/" {
			if !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}
		} else {
			prefix = "/"
		}

		for filePath := range fs.files {
			var relativePath string
			if prefix == "/" {
				relativePath = strings.TrimPrefix(filePath, "/")
			} else {
				if !strings.HasPrefix(filePath, prefix) {
					continue
				}
				relativePath = strings.TrimPrefix(filePath, prefix)
			}

			if relativePath == "" {
				continue
			}

			parts := strings.SplitN(relativePath, "/", 2)
			if len(parts) == 1 {
				// Direct file in this directory
				filesSet[parts[0]] = struct{}{}
			} else if len(parts) > 1 && parts[0] != "" {
				// Subdirectory
				dirsSet[parts[0]] = struct{}{}
			}
		}
	}
	fs.mu.RUnlock()

	// Get S3 entries
	ctx := context.Background()
	entry := getFromCache(ctx, fs.version, path)

	// If no user files (v1 mode), return S3 entries directly (original behavior)
	if !hasUserFiles {
		if !entry.Exists || entry.IsFile {
			return vfs.Entries{Files: []string{}, Directories: []string{}}
		}
		return vfs.Entries{
			Files:       entry.Files,
			Directories: entry.Dirs,
		}
	}

	// v2 mode: merge with S3 entries
	if entry.Exists && !entry.IsFile {
		for _, f := range entry.Files {
			filesSet[f] = struct{}{}
		}
		for _, d := range entry.Dirs {
			dirsSet[d] = struct{}{}
		}
	}

	// Convert sets to slices
	files := make([]string, 0, len(filesSet))
	for f := range filesSet {
		files = append(files, f)
	}

	dirs := make([]string, 0, len(dirsSet))
	for d := range dirsSet {
		dirs = append(dirs, d)
	}

	return vfs.Entries{
		Files:       files,
		Directories: dirs,
	}
}

func (fs *s3FS) Stat(path string) vfs.FileInfo { return nil }
func (fs *s3FS) WalkDir(root string, walkFn vfs.WalkDirFunc) error { return nil }
func (fs *s3FS) Realpath(path string) string { return path }

func calculateLineColumn(text string, pos int) (int, int) {
	if pos < 0 || pos >= len(text) {
		return 0, 0
	}
	line, col := 0, 0
	for i := 0; i < pos; i++ {
		if text[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func typecheckTypeScript(code string, version string) TypecheckResponse {
	// Track typecheck duration
	typecheckStart := time.Now()
	defer func() {
		duration := time.Since(typecheckStart)
		typecheckDuration.Observe(duration.Seconds())
		log.Printf("[PERF] typecheckTypeScript total: %v", duration)
	}()
	
	// Create S3-backed filesystem for this version
	fs := newS3FS(version)
	
	// Always use .tsx to support JSX
	fileName := "/input.tsx"
	
	fs.files[fileName] = code
	
	wrappedFS := bundled.WrapFS(fs)
	
	// Create minimal compiler options (matching CrayonDeveloper settings)
	jsxImportSource := "@crayonnow/core"
	compilerOptions := &core.CompilerOptions{
		AllowJs:                          core.TSTrue,
		Declaration:                      core.TSTrue,
		ESModuleInterop:                  core.TSTrue,
		ForceConsistentCasingInFileNames: core.TSTrue,
		IsolatedModules:                  core.TSTrue,
		Jsx:                              core.JsxEmitReactJSX,
		JsxImportSource:                  jsxImportSource,
		Module:                           core.ModuleKindCommonJS,
		ModuleResolution:                 core.ModuleResolutionKindBundler,
		NoEmit:                           core.TSTrue,
		ResolveJsonModule:                core.TSTrue,
		SkipLibCheck:                     core.TSTrue,
		Strict:                           core.TSTrue,
		StrictNullChecks:                 core.TSTrue,
		Target:                           core.ScriptTargetES2022,
		Lib:                              []string{"ES2022"},
	}
	
	// Create parsed options
	parsedOptions := &core.ParsedOptions{
		CompilerOptions: compilerOptions,
		FileNames:       []string{fileName},
	}
	
	// Create config
	config := &tsoptions.ParsedCommandLine{
		ParsedConfig: parsedOptions,
	}
	
	// Create cache
	extendedConfigCache := &tsc.ExtendedConfigCache{}
	
	// Create host
	host := compiler.NewCachedFSCompilerHost("/", wrappedFS, bundled.LibPath(), extendedConfigCache, nil)
	
	// Create program
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           config,
		Host:             host,
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})
	
	ctx := context.Background()
	
	// Get diagnostics
	diagnostics := program.GetSyntacticDiagnostics(ctx, nil)
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
	}
	
	if len(diagnostics) > 0 {
		errors := make([]DiagnosticError, 0, len(diagnostics))
		for _, diag := range diagnostics {
			err := DiagnosticError{
				Message: diag.Localize(locale.Default),
			}
			if diag.File() != nil && diag.Loc().Pos() >= 0 {
				line, col := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
				err.Line = line + 1
				err.Column = col + 1
			}
			errors = append(errors, err)
		}
		typecheckResults.WithLabelValues("error").Inc()
		return TypecheckResponse{Errors: errors}
	}
	
	typecheckResults.WithLabelValues("success").Inc()
	return TypecheckResponse{Pass: true}
}

func buildTypeScript(code string, version string) BuildResponse {
	// Track compile duration
	compileStart := time.Now()
	defer func() {
		duration := time.Since(compileStart)
		compileDuration.Observe(duration.Seconds())
		log.Printf("[PERF] buildTypeScript total: %v", duration)
	}()
	
	// Create S3-backed filesystem for this version
	fsStart := time.Now()
	fs := newS3FS(version)
	log.Printf("[PERF] newS3FS: %v", time.Since(fsStart))
	
	// Always use .tsx to support JSX
	fileName := "/input.tsx"
	
	fs.files[fileName] = code
	
	// Create virtual file resolver for esbuild
	resolverCalls := 0
	resolver := func(path string) (api.OnLoadResult, error) {
		resolverCalls++
		// Track package resolutions
		trackPackageResolution(path)
		
		// Only try exact path for absolute paths or relative paths
		// Bare module specifiers like "@use-gesture/react" should go through resolution
		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
			if content, exists := fs.ReadFile(path); exists {
				// Log cache hit
				if strings.Contains(path, "node_modules") {
					log.Printf("[BUILD] Cache HIT for path: %s", path)
				}
				loader := api.LoaderDefault
				if strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts") {
					loader = api.LoaderTSX
				} else if strings.HasSuffix(path, ".jsx") {
					loader = api.LoaderJSX
				} else if strings.HasSuffix(path, ".json") {
					loader = api.LoaderJSON
				}
				return api.OnLoadResult{
					Contents: &content,
					Loader:   loader,
				}, nil
			}
		}
		
		// Log cache miss
		if strings.Contains(path, "node_modules") {
			log.Printf("[BUILD] Cache MISS for path: %s", path)
		}
		
		// Try with common extensions if exact path not found
		if strings.HasPrefix(path, "/") {
			extensions := []string{".js", ".jsx", ".mjs", ".json", ".ts", ".tsx"}
			for _, ext := range extensions {
				testPath := path + ext
				if content, exists := fs.ReadFile(testPath); exists {
					loader := api.LoaderDefault
					if ext == ".tsx" || ext == ".ts" {
						loader = api.LoaderTSX
					} else if ext == ".jsx" {
						loader = api.LoaderJSX
					} else if ext == ".json" {
						loader = api.LoaderJSON
					}
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loader,
					}, nil
				}
			}
		}
		
		// Try to resolve using the clean module resolver  
		if !strings.HasPrefix(path, "/") {
			if strings.Contains(path, "node_modules") || strings.Contains(path, "@") {
				log.Printf("[BUILD] Resolving module: %s", path)
			}
			resolvedPath := resolveModule(fs, path, "")
			if strings.Contains(path, "@") && resolvedPath != "" {
				log.Printf("[BUILD] Resolved %s -> %s", path, resolvedPath)
			}
			if resolvedPath != "" {
				if content, exists := fs.ReadFile(resolvedPath); exists {
					loader := api.LoaderDefault
					if strings.HasSuffix(resolvedPath, ".tsx") || strings.HasSuffix(resolvedPath, ".ts") {
						loader = api.LoaderTSX
					} else if strings.HasSuffix(resolvedPath, ".jsx") {
						loader = api.LoaderJSX
					} else if strings.HasSuffix(resolvedPath, ".json") {
						loader = api.LoaderJSON
					} else if strings.HasSuffix(resolvedPath, ".mjs") {
						loader = api.LoaderJS
					}
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loader,
					}, nil
				}
			}
		}
		
		return api.OnLoadResult{}, fmt.Errorf("file not found: %s", path)
	}
	
	// Build with esbuild (matching Swift configuration)
	esbuildStart := time.Now()
	result := api.Build(api.BuildOptions{
		EntryPoints:        []string{fileName},
		Bundle:             true,
		Format:             api.FormatCommonJS,
		JSXFactory:         "_CRAYONCORE_$REACT.createElement",
		JSXFragment:        "_CRAYONCORE_$REACT.Fragment",
		MinifyWhitespace:   true,
		MinifyIdentifiers:  false,
		MinifySyntax:       true,
		Platform:           api.PlatformBrowser,
		Target:             api.ES2022,
		Write:              false,
		External:           []string{"*"},
		Plugins: []api.Plugin{{
			Name: "virtual-fs",
			Setup: func(pb api.PluginBuild) {
				pb.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					// Track package resolutions in esbuild plugin
					trackPackageResolution(args.Path)
					
					// Transform react imports to use global variable
					if args.Path == "react" {
						return api.OnResolveResult{
							Path:      "react",
							Namespace: "use-crayon-react-global",
						}, nil
					}
					
					// Handle absolute imports
					if strings.HasPrefix(args.Path, "/") {
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}
					
					// Handle relative imports using the clean resolver
					if strings.HasPrefix(args.Path, "./") || strings.HasPrefix(args.Path, "../") {
						// Use the clean resolveModule function
						resolvedPath := resolveModule(fs, args.Path, args.Importer)
						if resolvedPath == "" {
							// Fallback: just resolve path relative to importer
							importerPath := args.Importer
							if !strings.HasPrefix(importerPath, "/") {
								// If importer is a bare package, resolve it first
								importerPath = resolveBarePackageImporter(fs, importerPath)
							}
							importerDir := filepath.Dir(importerPath)
							resolvedPath = filepath.Join(importerDir, args.Path)
							resolvedPath = strings.ReplaceAll(resolvedPath, "\\", "/")
						}
						return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
					}
					
					// Handle bare imports (no relative path)
					if !strings.Contains(args.Path, "/") || strings.HasPrefix(args.Path, "@") {
						// This is a node_modules import
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}
					
					// Handle subpath imports that aren't relative (like "cjs/react.production.js")
					// These are relative to the importer's package
					if args.Importer != "" && strings.Contains(args.Importer, "/node_modules/") {
						// Extract the package path from the importer
						parts := strings.Split(args.Importer, "/node_modules/")
						if len(parts) >= 2 {
							// Find the package name
							remainingPath := parts[1]
							packageParts := strings.Split(remainingPath, "/")
							packageName := packageParts[0]
							if strings.HasPrefix(packageName, "@") && len(packageParts) > 1 {
								packageName = packageParts[0] + "/" + packageParts[1]
							}
							// Resolve relative to package root
							resolvedPath := "/node_modules/" + packageName + "/" + args.Path
							return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
						}
					}
					
					// Default: treat as node_modules import
					return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
				})
				
				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "virtual"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					return resolver(args.Path)
				})
				
				// Handle react global transform
				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "use-crayon-react-global"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					contents := "module.exports = _CRAYONCORE_$REACT"
					return api.OnLoadResult{
						Contents: &contents,
						Loader:   api.LoaderJS,
					}, nil
				})
			},
		}},
	})
	log.Printf("[PERF] esbuild.Build: %v (resolver called %d times)", time.Since(esbuildStart), resolverCalls)
	
	if len(result.Errors) > 0 {
		errors := make([]DiagnosticError, 0, len(result.Errors))
		for _, err := range result.Errors {
			diagErr := DiagnosticError{
				Message: err.Text,
			}
			if err.Location != nil {
				diagErr.Line = err.Location.Line
				diagErr.Column = err.Location.Column
			}
			errors = append(errors, diagErr)
		}
		compileResults.WithLabelValues("error").Inc()
		return BuildResponse{Errors: errors}
	}
	
	if len(result.OutputFiles) == 0 {
		compileResults.WithLabelValues("error").Inc()
		// log.Printf("Build failed: No output files generated")
		return BuildResponse{Errors: []DiagnosticError{{Message: "No output generated"}}}
	}
	
	outputCode := string(result.OutputFiles[0].Contents)
	// log.Printf("Build successful: Generated %d bytes of code", len(outputCode))
	compileResults.WithLabelValues("success").Inc()
	return BuildResponse{Code: outputCode}
}

// Middleware for request logging
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Add git commit header to all responses
		w.Header().Set("X-Git-Commit", gitCommit)
		w.Header().Set("X-Server-Version", serverVersion)
		
		// Create a custom response writer to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Call the next handler
		next(lrw, r)
		
		// Log the request
		duration := time.Since(start)
		log.Printf("%s %s - %d - %v", r.Method, r.URL.Path, lrw.statusCode, duration)
		
		// Record metrics
		recordHTTPMetrics(r, lrw.statusCode, duration)
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}


func health(w http.ResponseWriter, req *http.Request) {
	uptime := time.Since(startTime)
	
	var cacheEntries int
	cacheMutex.RLock()
	if cache != nil {
		cacheEntries = cache.Len()
	}
	cacheMutex.RUnlock()
	
	response := HealthResponse{
		Status:       "healthy",
		Version:      serverVersion,
		Uptime:       fmt.Sprintf("%v", uptime.Round(time.Second)),
		CacheSize:    fmt.Sprintf("%d MB", cacheSize/(1024*1024)),
		CacheEntries: cacheEntries,
	}
	
	// Add pre-warm status
	if !cachePrewarmStarted.IsZero() {
		prewarmStatus := CachePrewarmStatus{
			Files:   cachePrewarmFiles,
			BytesMB: float64(cachePrewarmBytes) / (1024 * 1024),
		}
		
		if cachePrewarmCompleted.IsZero() {
			prewarmStatus.Status = "in_progress"
		} else {
			prewarmStatus.Status = "completed"
			prewarmStatus.DurationS = cachePrewarmCompleted.Sub(cachePrewarmStarted).Seconds()
		}
		
		response.CachePrewarm = prewarmStatus
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func hello(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	fmt.Fprintf(w, "TypeScript Go Server v%s\nUptime: %v\n", 
		serverVersion, time.Since(startTime).Round(time.Second))
}

func typecheck(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var typecheckReq TypecheckRequest
	if err := json.NewDecoder(req.Body).Decode(&typecheckReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if typecheckReq.Code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}
	
	if typecheckReq.Version == "" {
		http.Error(w, "Version is required", http.StatusBadRequest)
		return
	}

	response := typecheckTypeScript(typecheckReq.Code, typecheckReq.Version)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func build(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var buildReq BuildRequest
	if err := json.NewDecoder(req.Body).Decode(&buildReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if buildReq.Code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}
	
	if buildReq.Version == "" {
		http.Error(w, "Version is required", http.StatusBadRequest)
		return
	}

	// Check if type validation is requested
	validateTypes := req.URL.Query().Get("validate_types") == "true"
	
	if validateTypes {
		// First run typecheck
		typecheckResponse := typecheckTypeScript(buildReq.Code, buildReq.Version)
		if len(typecheckResponse.Errors) > 0 {
			// Return type errors as build errors
			response := BuildResponse{
				Errors: typecheckResponse.Errors,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Proceed with build
	response := buildTypeScript(buildReq.Code, buildReq.Version)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func flushCache(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get cache metrics before flush
	var entriesBefore int
	cacheMutex.RLock()
	if cache != nil {
		entriesBefore = cache.Len()
	}
	cacheMutex.RUnlock()

	// Flush the cache
	cacheMutex.Lock()
	if cache != nil {
		cache.Purge()
	}
	cacheMutex.Unlock()

	// Return flush summary
	response := map[string]interface{}{
		"status":          "success",
		"message":         "Cache flushed successfully",
		"entries_cleared": entriesBefore,
		"timestamp":       time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// typecheckV2 handles multi-file typecheck requests
func typecheckV2(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var v2Req TypecheckV2Request
	if err := json.NewDecoder(req.Body).Decode(&v2Req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateV2Files(v2Req.Files); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if v2Req.Version == "" {
		http.Error(w, "Version is required", http.StatusBadRequest)
		return
	}

	response := typecheckTypeScriptV2(v2Req.Files, v2Req.EntryPoints, v2Req.Version)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// typecheckTypeScriptV2 handles multi-file typechecking
func typecheckTypeScriptV2(files map[string]string, entryPoints []string, version string) TypecheckV2Response {
	typecheckStart := time.Now()
	defer func() {
		duration := time.Since(typecheckStart)
		typecheckDuration.Observe(duration.Seconds())
		log.Printf("[PERF] typecheckTypeScriptV2 total: %v (%d files)", duration, len(files))
	}()

	// Create S3-backed filesystem for this version
	fs := newS3FS(version)
	fs.hasUserFiles = true // Enable v2 mode for directory resolution

	// Populate with all user files and collect TypeScript entry points
	var fileNames []string
	for path, content := range files {
		normalized, err := normalizeAndValidatePath(path)
		if err != nil {
			return TypecheckV2Response{
				Errors: []DiagnosticErrorV2{{
					File:    path,
					Message: err.Error(),
				}},
			}
		}

		fs.mu.Lock()
		fs.files[normalized] = content
		fs.mu.Unlock()

		// Collect .ts and .tsx files as potential entry points
		if strings.HasSuffix(normalized, ".ts") || strings.HasSuffix(normalized, ".tsx") {
			fileNames = append(fileNames, normalized)
		}
	}

	// Use specified entry points if provided
	if len(entryPoints) > 0 {
		fileNames = make([]string, 0, len(entryPoints))
		for _, ep := range entryPoints {
			normalized, err := normalizeAndValidatePath(ep)
			if err != nil {
				return TypecheckV2Response{
					Errors: []DiagnosticErrorV2{{
						File:    ep,
						Message: err.Error(),
					}},
				}
			}
			fileNames = append(fileNames, normalized)
		}
	}

	if len(fileNames) == 0 {
		return TypecheckV2Response{
			Errors: []DiagnosticErrorV2{{
				Message: "No TypeScript files found to check",
			}},
		}
	}

	wrappedFS := bundled.WrapFS(fs)

	// Create compiler options (matching existing settings)
	jsxImportSource := "@crayonnow/core"
	compilerOptions := &core.CompilerOptions{
		AllowJs:                          core.TSTrue,
		Declaration:                      core.TSTrue,
		ESModuleInterop:                  core.TSTrue,
		ForceConsistentCasingInFileNames: core.TSTrue,
		IsolatedModules:                  core.TSTrue,
		Jsx:                              core.JsxEmitReactJSX,
		JsxImportSource:                  jsxImportSource,
		Module:                           core.ModuleKindCommonJS,
		ModuleResolution:                 core.ModuleResolutionKindBundler,
		NoEmit:                           core.TSTrue,
		ResolveJsonModule:                core.TSTrue,
		SkipLibCheck:                     core.TSTrue,
		Strict:                           core.TSTrue,
		StrictNullChecks:                 core.TSTrue,
		Target:                           core.ScriptTargetES2022,
		Lib:                              []string{"ES2022"},
	}

	parsedOptions := &core.ParsedOptions{
		CompilerOptions: compilerOptions,
		FileNames:       fileNames,
	}

	config := &tsoptions.ParsedCommandLine{
		ParsedConfig: parsedOptions,
	}

	extendedConfigCache := &tsc.ExtendedConfigCache{}
	host := compiler.NewCachedFSCompilerHost("/", wrappedFS, bundled.LibPath(), extendedConfigCache, nil)

	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           config,
		Host:             host,
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})

	ctx := context.Background()

	// Get diagnostics
	diagnostics := program.GetSyntacticDiagnostics(ctx, nil)
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
	}

	if len(diagnostics) > 0 {
		errors := make([]DiagnosticErrorV2, 0, len(diagnostics))
		for _, diag := range diagnostics {
			err := DiagnosticErrorV2{
				Message: diag.Localize(locale.Default),
			}
			if diag.File() != nil {
				err.File = diag.File().FileName()
				if diag.Loc().Pos() >= 0 {
					line, col := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
					err.Line = line + 1
					err.Column = col + 1
				}
			}
			errors = append(errors, err)
		}
		typecheckResults.WithLabelValues("error").Inc()
		return TypecheckV2Response{Errors: errors}
	}

	typecheckResults.WithLabelValues("success").Inc()
	return TypecheckV2Response{Pass: true}
}

// buildV2 handles multi-file build requests
func buildV2(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var v2Req BuildV2Request
	if err := json.NewDecoder(req.Body).Decode(&v2Req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateV2Files(v2Req.Files); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if v2Req.Version == "" {
		http.Error(w, "Version is required", http.StatusBadRequest)
		return
	}

	if v2Req.EntryPoint == "" {
		http.Error(w, "EntryPoint is required", http.StatusBadRequest)
		return
	}

	// Check if type validation is requested
	validateTypes := req.URL.Query().Get("validate_types") == "true"

	if validateTypes {
		// First run typecheck on all files
		typecheckResponse := typecheckTypeScriptV2(v2Req.Files, []string{v2Req.EntryPoint}, v2Req.Version)
		if len(typecheckResponse.Errors) > 0 {
			response := BuildV2Response{
				Errors: typecheckResponse.Errors,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	response := buildTypeScriptV2(v2Req.Files, v2Req.EntryPoint, v2Req.Version)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// buildTypeScriptV2 handles multi-file bundling
func buildTypeScriptV2(files map[string]string, entryPoint string, version string) BuildV2Response {
	compileStart := time.Now()
	defer func() {
		duration := time.Since(compileStart)
		compileDuration.Observe(duration.Seconds())
		log.Printf("[PERF] buildTypeScriptV2 total: %v (%d files)", duration, len(files))
	}()

	// Create S3-backed filesystem for this version
	fs := newS3FS(version)
	fs.hasUserFiles = true // Enable v2 mode for directory resolution

	// Populate with all user files
	for path, content := range files {
		normalized, err := normalizeAndValidatePath(path)
		if err != nil {
			return BuildV2Response{
				Errors: []DiagnosticErrorV2{{
					File:    path,
					Message: err.Error(),
				}},
			}
		}

		fs.mu.Lock()
		fs.files[normalized] = content
		fs.mu.Unlock()
	}

	// Normalize entry point
	normalizedEntryPoint, err := normalizeAndValidatePath(entryPoint)
	if err != nil {
		return BuildV2Response{
			Errors: []DiagnosticErrorV2{{
				File:    entryPoint,
				Message: err.Error(),
			}},
		}
	}

	// Verify entry point exists in provided files
	fs.mu.RLock()
	_, entryExists := fs.files[normalizedEntryPoint]
	fs.mu.RUnlock()
	if !entryExists {
		return BuildV2Response{
			Errors: []DiagnosticErrorV2{{
				File:    entryPoint,
				Message: fmt.Sprintf("Entry point not found in provided files: %s", entryPoint),
			}},
		}
	}

	// Create virtual file resolver for esbuild
	resolverCalls := 0
	resolver := func(path string) (api.OnLoadResult, error) {
		resolverCalls++
		trackPackageResolution(path)

		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
			if content, exists := fs.ReadFile(path); exists {
				if strings.Contains(path, "node_modules") {
					log.Printf("[BUILD V2] Cache HIT for path: %s", path)
				}
				loader := api.LoaderDefault
				if strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts") {
					loader = api.LoaderTSX
				} else if strings.HasSuffix(path, ".jsx") {
					loader = api.LoaderJSX
				} else if strings.HasSuffix(path, ".json") {
					loader = api.LoaderJSON
				}
				return api.OnLoadResult{
					Contents: &content,
					Loader:   loader,
				}, nil
			}
		}

		if strings.Contains(path, "node_modules") {
			log.Printf("[BUILD V2] Cache MISS for path: %s", path)
		}

		// Try with common extensions
		if strings.HasPrefix(path, "/") {
			extensions := []string{".js", ".jsx", ".mjs", ".json", ".ts", ".tsx"}
			for _, ext := range extensions {
				testPath := path + ext
				if content, exists := fs.ReadFile(testPath); exists {
					loader := api.LoaderDefault
					if ext == ".tsx" || ext == ".ts" {
						loader = api.LoaderTSX
					} else if ext == ".jsx" {
						loader = api.LoaderJSX
					} else if ext == ".json" {
						loader = api.LoaderJSON
					}
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loader,
					}, nil
				}
			}
		}

		// Try to resolve using module resolver
		if !strings.HasPrefix(path, "/") {
			if strings.Contains(path, "node_modules") || strings.Contains(path, "@") {
				log.Printf("[BUILD V2] Resolving module: %s", path)
			}
			resolvedPath := resolveModule(fs, path, "")
			if strings.Contains(path, "@") && resolvedPath != "" {
				log.Printf("[BUILD V2] Resolved %s -> %s", path, resolvedPath)
			}
			if resolvedPath != "" {
				if content, exists := fs.ReadFile(resolvedPath); exists {
					loader := api.LoaderDefault
					if strings.HasSuffix(resolvedPath, ".tsx") || strings.HasSuffix(resolvedPath, ".ts") {
						loader = api.LoaderTSX
					} else if strings.HasSuffix(resolvedPath, ".jsx") {
						loader = api.LoaderJSX
					} else if strings.HasSuffix(resolvedPath, ".json") {
						loader = api.LoaderJSON
					} else if strings.HasSuffix(resolvedPath, ".mjs") {
						loader = api.LoaderJS
					}
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loader,
					}, nil
				}
			}
		}

		return api.OnLoadResult{}, fmt.Errorf("file not found: %s", path)
	}

	// Build with esbuild
	esbuildStart := time.Now()
	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{normalizedEntryPoint},
		Bundle:            true,
		Format:            api.FormatCommonJS,
		JSXFactory:        "_CRAYONCORE_$REACT.createElement",
		JSXFragment:       "_CRAYONCORE_$REACT.Fragment",
		MinifyWhitespace:  true,
		MinifyIdentifiers: false,
		MinifySyntax:      true,
		Platform:          api.PlatformBrowser,
		Target:            api.ES2022,
		Write:             false,
		External:          []string{"*"},
		Plugins: []api.Plugin{{
			Name: "virtual-fs-v2",
			Setup: func(pb api.PluginBuild) {
				pb.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					trackPackageResolution(args.Path)

					// Transform react imports to use global variable
					if args.Path == "react" {
						return api.OnResolveResult{
							Path:      "react",
							Namespace: "use-crayon-react-global",
						}, nil
					}

					// Handle absolute imports
					if strings.HasPrefix(args.Path, "/") {
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}

					// Handle relative imports
					if strings.HasPrefix(args.Path, "./") || strings.HasPrefix(args.Path, "../") {
						resolvedPath := resolveModule(fs, args.Path, args.Importer)
						if resolvedPath == "" {
							importerPath := args.Importer
							if !strings.HasPrefix(importerPath, "/") {
								importerPath = resolveBarePackageImporter(fs, importerPath)
							}
							importerDir := filepath.Dir(importerPath)
							resolvedPath = filepath.Join(importerDir, args.Path)
							resolvedPath = strings.ReplaceAll(resolvedPath, "\\", "/")
						}
						return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
					}

					// Handle bare imports
					if !strings.Contains(args.Path, "/") || strings.HasPrefix(args.Path, "@") {
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}

					// Handle subpath imports
					if args.Importer != "" && strings.Contains(args.Importer, "/node_modules/") {
						parts := strings.Split(args.Importer, "/node_modules/")
						if len(parts) >= 2 {
							remainingPath := parts[1]
							packageParts := strings.Split(remainingPath, "/")
							packageName := packageParts[0]
							if strings.HasPrefix(packageName, "@") && len(packageParts) > 1 {
								packageName = packageParts[0] + "/" + packageParts[1]
							}
							resolvedPath := "/node_modules/" + packageName + "/" + args.Path
							return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
						}
					}

					return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
				})

				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "virtual"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					return resolver(args.Path)
				})

				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "use-crayon-react-global"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					contents := "module.exports = _CRAYONCORE_$REACT"
					return api.OnLoadResult{
						Contents: &contents,
						Loader:   api.LoaderJS,
					}, nil
				})
			},
		}},
	})
	log.Printf("[PERF] esbuild.Build V2: %v (resolver called %d times)", time.Since(esbuildStart), resolverCalls)

	if len(result.Errors) > 0 {
		errors := make([]DiagnosticErrorV2, 0, len(result.Errors))
		for _, err := range result.Errors {
			diagErr := DiagnosticErrorV2{
				Message: err.Text,
			}
			if err.Location != nil {
				diagErr.File = err.Location.File
				diagErr.Line = err.Location.Line
				diagErr.Column = err.Location.Column
			}
			errors = append(errors, diagErr)
		}
		compileResults.WithLabelValues("error").Inc()
		return BuildV2Response{Errors: errors}
	}

	if len(result.OutputFiles) == 0 {
		compileResults.WithLabelValues("error").Inc()
		return BuildV2Response{Errors: []DiagnosticErrorV2{{Message: "No output generated"}}}
	}

	outputCode := string(result.OutputFiles[0].Contents)
	compileResults.WithLabelValues("success").Inc()
	return BuildV2Response{Code: outputCode}
}

func main() {
	log.Printf("TypeScript Go Server v%s starting...", serverVersion)
	
	// Initialize S3 configuration from environment
	s3Bucket = os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		log.Fatal("S3_BUCKET environment variable is required")
	}
	
	// Parse cache size
	cacheSizeStr := os.Getenv("CACHE_SIZE")
	if cacheSizeStr == "" {
		cacheSize = 32 * 1024 * 1024 // Default 32MB
	} else {
		parsed, err := strconv.ParseInt(cacheSizeStr, 10, 64)
		if err != nil {
			log.Fatalf("Invalid CACHE_SIZE: %v", err)
		}
		cacheSize = parsed
	}
	
	// Initialize AWS SDK
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	
	// Override endpoint if specified
	if endpoint := os.Getenv("AWS_ENDPOINT_URL_S3"); endpoint != "" {
		s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}
	
	// Initialize LRU cache
	// Estimate average entry size for capacity calculation
	avgEntrySize := int64(4096) // 4KB average
	capacity := int(cacheSize / avgEntrySize)
	
	cache, err = lru.New[string, *CacheEntry](capacity)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	
	log.Printf("Initialized with S3 bucket: %s, cache size: %d MB", s3Bucket, cacheSize/(1024*1024))
	
	// Create a cancellable context for graceful shutdown
	mainCtx, mainCancel := context.WithCancel(ctx)
	defer mainCancel()
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Create a cancellable context for pre-warming
	prewarmCtx, prewarmCancel := context.WithCancel(mainCtx)
	defer prewarmCancel()
	
	// Start cache pre-warming in background
	go prewarmCache(prewarmCtx)
	
	// Set up routes with logging middleware
	http.HandleFunc("/health", loggingMiddleware(health))
	http.HandleFunc("/typecheck", loggingMiddleware(typecheck))
	http.HandleFunc("/build", loggingMiddleware(build))
	http.HandleFunc("/flush-cache", loggingMiddleware(flushCache))
	http.HandleFunc("/v2/typecheck", loggingMiddleware(typecheckV2))
	http.HandleFunc("/v2/build", loggingMiddleware(buildV2))
	http.HandleFunc("/", loggingMiddleware(hello))
	
	// Start Prometheus metrics server on port 9091
	go startMetricsServer()
	
	// Create HTTP server with graceful shutdown support
	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil, // Use default ServeMux
	}
	
	// Start server in goroutine
	go func() {
		log.Printf("Server ready! Listening on :8080...")
		log.Printf("Endpoints: /, /health, /typecheck, /build, /flush-cache, /v2/typecheck, /v2/build")
		log.Printf("Metrics available at :9091/metrics")
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()
	
	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down gracefully...", sig)
	
	// Cancel pre-warming first
	prewarmCancel()
	
	// Give ongoing requests 30 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	
	log.Printf("Server shutdown complete")
}