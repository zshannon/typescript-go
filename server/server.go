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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/evanw/esbuild/pkg/api"
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
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

var (
	serverVersion = "1.0.0"
	startTime     = time.Now()
	gitCommit     = "unknown" // Set at build time with -ldflags

	// S3 client and configuration
	s3Client    S3ClientInterface
	s3Bucket    string

	// Disk cache configuration
	diskCachePath string
)

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
	DiskCachePath string `json:"disk_cache_path"`
	Status        string `json:"status"`
	Uptime        string `json:"uptime"`
	Version       string `json:"version"`
}

type diskFS struct {
	basePath     string            // e.g., "/data/cache/5.7.0"
	hasUserFiles bool              // true if user provided multiple files (v2 mode)
	mu           sync.RWMutex      // protects userFiles map
	userFiles    map[string]string // user-provided files (v2 mode)
	version      string
}

func newDiskFS(ctx context.Context, version string) (*diskFS, error) {
	if err := ensureVersionSynced(ctx, version); err != nil {
		return nil, err
	}
	return &diskFS{
		basePath:  filepath.Join(diskCachePath, version),
		userFiles: make(map[string]string),
		version:   version,
	}, nil
}

// newDiskFSFromDeps creates a diskFS backed by a dependency cache directory (v3 mode).
// Unlike newDiskFS, this does not sync from S3 — the caller ensures deps are already installed.
func newDiskFSFromDeps(depCachePath string) *diskFS {
	return &diskFS{
		basePath:  depCachePath,
		userFiles: make(map[string]string),
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

// ensureVersionSynced checks if version dir exists on disk, syncs from S3 if not
func ensureVersionSynced(ctx context.Context, version string) error {
	versionPath := filepath.Join(diskCachePath, version)

	// If directory exists, we're done
	if _, err := os.Stat(versionPath); err == nil {
		return nil
	}

	log.Printf("[SYNC] Starting sync for version %s", version)
	start := time.Now()

	// List all objects with prefix "{version}/"
	var keys []string
	var continuationToken *string
	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(s3Bucket),
			Prefix: aws.String(version + "/"),
		}
		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		page, err := s3Client.ListObjectsV2(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to list S3 objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}

		if page.IsTruncated == nil || !*page.IsTruncated {
			break
		}
		continuationToken = page.NextContinuationToken
	}

	if len(keys) == 0 {
		return fmt.Errorf("version %s not found in S3", version)
	}

	log.Printf("[SYNC] Found %d files to download for version %s", len(keys), version)

	// Download files in parallel (20 workers)
	const numWorkers = 20
	keysChan := make(chan string, len(keys))
	var wg sync.WaitGroup
	var downloadErrors int64
	var errorsMu sync.Mutex
	var cleanupMu sync.Mutex // Mutex to coordinate disk cleanup across workers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range keysChan {
				localPath := filepath.Join(diskCachePath, key)

				// Retry loop for disk full errors
				for {
					err := downloadFile(ctx, key, localPath)
					if err == nil {
						break // Success
					}

					if !isDiskFullError(err) {
						log.Printf("[SYNC] Failed to download %s: %v", key, err)
						errorsMu.Lock()
						downloadErrors++
						errorsMu.Unlock()
						break // Non-disk-full error, don't retry
					}

					// Disk full - try to delete oldest version
					cleanupMu.Lock()
					deleted := deleteOldestVersion(version)
					cleanupMu.Unlock()

					if !deleted {
						// No more versions to delete, crash
						log.Fatalf("[FATAL] Disk full and no old versions to delete. Cannot continue.")
					}

					// Retry download after cleanup
					log.Printf("[SYNC] Retrying download after cleanup: %s", key)
				}
			}
		}()
	}

	// Send all keys to workers
	for _, key := range keys {
		keysChan <- key
	}
	close(keysChan)
	wg.Wait()

	duration := time.Since(start)
	log.Printf("[SYNC] Completed sync for version %s: %d files in %v (%d errors)",
		version, len(keys), duration, downloadErrors)

	return nil
}

type versionInfo struct {
	modTime time.Time
	name    string
}

// getVersionsByModTime returns version directories sorted by modification time (oldest first)
func getVersionsByModTime(cachePath string) ([]versionInfo, error) {
	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return nil, err
	}

	var versions []versionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		versions = append(versions, versionInfo{
			modTime: info.ModTime(),
			name:    entry.Name(),
		})
	}

	// Sort oldest first
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].modTime.Before(versions[j].modTime)
	})

	return versions, nil
}

// deleteOldestVersion finds and deletes the oldest version directory (excluding keepVersion)
// Returns true if a version was deleted, false if none available to delete
func deleteOldestVersion(keepVersion string) bool {
	versions, err := getVersionsByModTime(diskCachePath)
	if err != nil {
		log.Printf("[CLEANUP] Failed to list versions: %v", err)
		return false
	}

	for _, v := range versions {
		if v.name == keepVersion {
			continue
		}

		versionPath := filepath.Join(diskCachePath, v.name)
		log.Printf("[CLEANUP] Disk full - removing oldest version %s", v.name)

		if err := os.RemoveAll(versionPath); err != nil {
			log.Printf("[CLEANUP] Failed to remove %s: %v", v.name, err)
			continue
		}
		return true
	}
	return false
}

// isDiskFullError checks if an error is a disk full error
func isDiskFullError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no space left on device") ||
		strings.Contains(err.Error(), "disk quota exceeded")
}

// downloadFile downloads a single file from S3 to local disk
func downloadFile(ctx context.Context, key, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, result.Body)
	return err
}


func (fs *diskFS) UseCaseSensitiveFileNames() bool { return true }

func (fs *diskFS) FileExists(path string) bool {
	// Check user files first
	fs.mu.RLock()
	_, ok := fs.userFiles[path]
	fs.mu.RUnlock()
	if ok {
		return true
	}

	// Check disk
	fullPath := filepath.Join(fs.basePath, path)
	info, err := os.Stat(fullPath)
	return err == nil && !info.IsDir()
}

func (fs *diskFS) ReadFile(path string) (string, bool) {
	// Check user files first
	fs.mu.RLock()
	if content, ok := fs.userFiles[path]; ok {
		fs.mu.RUnlock()
		return content, true
	}
	fs.mu.RUnlock()

	// Read from disk
	fullPath := filepath.Join(fs.basePath, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (fs *diskFS) WriteFile(path string, data string) error {
	fs.mu.Lock()
	fs.userFiles[path] = data
	fs.mu.Unlock()
	return nil
}

func (fs *diskFS) Remove(path string) error {
	fs.mu.Lock()
	delete(fs.userFiles, path)
	fs.mu.Unlock()
	return nil
}

func (fs *diskFS) Chtimes(path string, aTime time.Time, mTime time.Time) error {
	return nil
}

func (fs *diskFS) DirectoryExists(path string) bool {
	if path == "/" || path == "" {
		return true
	}

	// Check user files for v2 mode
	fs.mu.RLock()
	hasUserFiles := fs.hasUserFiles
	if hasUserFiles {
		prefix := path
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		for filePath := range fs.userFiles {
			if strings.HasPrefix(filePath, prefix) {
				fs.mu.RUnlock()
				return true
			}
		}
	}
	fs.mu.RUnlock()

	// Check disk
	fullPath := filepath.Join(fs.basePath, path)
	info, err := os.Stat(fullPath)
	return err == nil && info.IsDir()
}

func (fs *diskFS) GetAccessibleEntries(path string) vfs.Entries {
	filesSet := make(map[string]struct{})
	dirsSet := make(map[string]struct{})

	// Scan user files for v2 mode
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

		for filePath := range fs.userFiles {
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
				filesSet[parts[0]] = struct{}{}
			} else if len(parts) > 1 && parts[0] != "" {
				dirsSet[parts[0]] = struct{}{}
			}
		}
	}
	fs.mu.RUnlock()

	// Read from disk
	fullPath := filepath.Join(fs.basePath, path)
	entries, err := os.ReadDir(fullPath)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				dirsSet[entry.Name()] = struct{}{}
			} else {
				filesSet[entry.Name()] = struct{}{}
			}
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
		Directories: dirs,
		Files:       files,
	}
}

func (fs *diskFS) Stat(path string) vfs.FileInfo { return nil }
func (fs *diskFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error { return nil }
func (fs *diskFS) Realpath(path string) string { return path }

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

	// Create disk-backed filesystem for this version
	ctx := context.Background()
	fs, err := newDiskFS(ctx, version)
	if err != nil {
		return TypecheckResponse{Errors: []DiagnosticError{{Message: "failed to sync version: " + err.Error()}}}
	}

	// Always use .tsx to support JSX
	fileName := "/input.tsx"

	fs.userFiles[fileName] = code
	
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
		Config: config,
		Host:   host,
	})

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

	// Create disk-backed filesystem for this version
	fsStart := time.Now()
	ctx := context.Background()
	fs, err := newDiskFS(ctx, version)
	if err != nil {
		return BuildResponse{Errors: []DiagnosticError{{Message: "failed to sync version: " + err.Error()}}}
	}
	log.Printf("[PERF] newDiskFS: %v", time.Since(fsStart))

	// Always use .tsx to support JSX
	fileName := "/input.tsx"

	fs.userFiles[fileName] = code
	
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
	response := HealthResponse{
		DiskCachePath: diskCachePath,
		Status:        "healthy",
		Uptime:        fmt.Sprintf("%v", time.Since(startTime).Round(time.Second)),
		Version:       serverVersion,
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

func syncVersion(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version := req.URL.Query().Get("version")
	if version == "" {
		http.Error(w, "version parameter required", http.StatusBadRequest)
		return
	}

	// Delete existing version directory and re-sync
	versionPath := filepath.Join(diskCachePath, version)
	if err := os.RemoveAll(versionPath); err != nil {
		log.Printf("[SYNC] Warning: failed to remove %s: %v", versionPath, err)
	}

	if err := ensureVersionSynced(req.Context(), version); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "synced",
		"version": version,
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

	// Create disk-backed filesystem for this version
	ctx := context.Background()
	fs, err := newDiskFS(ctx, version)
	if err != nil {
		return TypecheckV2Response{Errors: []DiagnosticErrorV2{{Message: "failed to sync version: " + err.Error()}}}
	}
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
		fs.userFiles[normalized] = content
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
	})

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

	// Create disk-backed filesystem for this version
	ctx := context.Background()
	fs, err := newDiskFS(ctx, version)
	if err != nil {
		return BuildV2Response{Errors: []DiagnosticErrorV2{{Message: "failed to sync version: " + err.Error()}}}
	}
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
		fs.userFiles[normalized] = content
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
	_, entryExists := fs.userFiles[normalizedEntryPoint]
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

	// Initialize disk cache path
	diskCachePath = os.Getenv("DISK_CACHE_PATH")
	if diskCachePath == "" {
		diskCachePath = "/data/cache"
	}
	if err := os.MkdirAll(diskCachePath, 0755); err != nil {
		log.Fatalf("Failed to create disk cache directory %s: %v", diskCachePath, err)
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

	log.Printf("Initialized with S3 bucket: %s, disk cache: %s", s3Bucket, diskCachePath)

	initAuth()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Set up routes
	http.HandleFunc("/", loggingMiddleware(hello))
	http.HandleFunc("/build", loggingMiddleware(authMiddleware(build)))
	http.HandleFunc("/health", loggingMiddleware(health))
	http.HandleFunc("/sync", loggingMiddleware(authMiddleware(syncVersion)))
	http.HandleFunc("/typecheck", loggingMiddleware(authMiddleware(typecheck)))
	http.HandleFunc("/v2/build", loggingMiddleware(authMiddleware(buildV2)))
	http.HandleFunc("/v2/typecheck", loggingMiddleware(authMiddleware(typecheckV2)))
	http.HandleFunc("/v3/compile", loggingMiddleware(authMiddleware(compileV3Handler)))
	http.HandleFunc("/v3/typecheck", loggingMiddleware(authMiddleware(typecheckV3Handler)))

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
		log.Printf("Endpoints: /, /build, /health, /sync, /typecheck, /v2/build, /v2/typecheck, /v3/compile, /v3/typecheck")
		log.Printf("Metrics available at :9091/metrics")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	// Give ongoing requests 30 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Printf("Server shutdown complete")
}