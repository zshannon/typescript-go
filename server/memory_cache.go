package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/singleflight"
)

const (
	// A rollover advances the global generation, making prior cache keys unreachable.
	diskCacheGenerationMaxEntries = 16_384
	diskMemoryCacheMaxCost        = 256 << 20
)

type cacheAccessSnapshot struct {
	fileHits         int64
	fileMisses       int64
	resolutionHits   int64
	resolutionMisses int64
}

type cacheAccessStats struct {
	fileHits         atomic.Int64
	fileMisses       atomic.Int64
	resolutionHits   atomic.Int64
	resolutionMisses atomic.Int64
}

type cachedLookup struct {
	found bool
	value string
}

var (
	diskCacheGlobalGeneration   atomic.Uint64
	diskCacheGenerationSequence atomic.Uint64
	diskCacheGenerationMu       sync.Mutex
	diskCacheGenerationCount    int
	diskCacheGenerations        sync.Map
	diskFileLoads               singleflight.Group
	diskMemoryCache             = newDiskMemoryCache()
)

func newDiskMemoryCache() *ristretto.Cache[string, cachedLookup] {
	cache, err := ristretto.NewCache(&ristretto.Config[string, cachedLookup]{
		BufferItems: 64,
		MaxCost:     diskMemoryCacheMaxCost,
		NumCounters: 1_000_000,
	})
	if err != nil {
		panic(err)
	}
	return cache
}

func (fs *diskFS) cacheResolution(importPath string, importer string, resolvedPath string) {
	key := memoryCacheKey("resolve", fs.basePath, importPath+"\x00"+importer)
	diskMemoryCache.Set(key, cachedLookup{found: true, value: resolvedPath}, int64(len(key)+len(resolvedPath)))
}

func (fs *diskFS) cacheSnapshot() cacheAccessSnapshot {
	return cacheAccessSnapshot{
		fileHits:         fs.cacheStats.fileHits.Load(),
		fileMisses:       fs.cacheStats.fileMisses.Load(),
		resolutionHits:   fs.cacheStats.resolutionHits.Load(),
		resolutionMisses: fs.cacheStats.resolutionMisses.Load(),
	}
}

func (fs *diskFS) cachedResolution(importPath string, importer string) (string, bool) {
	key := memoryCacheKey("resolve", fs.basePath, importPath+"\x00"+importer)
	value, ok := diskMemoryCache.Get(key)
	if ok {
		fs.cacheStats.resolutionHits.Add(1)
		return value.value, true
	}
	fs.cacheStats.resolutionMisses.Add(1)
	return "", false
}

func (fs *diskFS) readDiskFile(path string) (string, bool) {
	fullPath := filepath.Join(fs.basePath, path)
	key := memoryCacheKey("file", fs.basePath, path)
	if value, ok := diskMemoryCache.Get(key); ok {
		fs.cacheStats.fileHits.Add(1)
		return value.value, value.found
	}
	fs.cacheStats.fileMisses.Add(1)

	loaded, loadErr, _ := diskFileLoads.Do(key, func() (any, error) {
		if value, ok := diskMemoryCache.Get(key); ok {
			return value, nil
		}
		data, err := os.ReadFile(fullPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		value := cachedLookup{found: err == nil}
		if err == nil {
			value.value = string(data)
		}
		diskMemoryCache.Set(key, value, int64(len(key)+len(data)+1))
		return value, nil
	})
	if loadErr != nil {
		return "", false
	}
	value := loaded.(cachedLookup)
	return value.value, value.found
}

func invalidateDiskMemoryCache(basePath string) {
	diskCacheGenerationMu.Lock()
	defer diskCacheGenerationMu.Unlock()

	cleanBasePath := filepath.Clean(basePath)
	generation := diskCacheGenerationSequence.Add(1)
	if _, exists := diskCacheGenerations.Load(cleanBasePath); exists {
		diskCacheGenerations.Store(cleanBasePath, generation)
		return
	}
	if diskCacheGenerationCount >= diskCacheGenerationMaxEntries {
		// Leave stale values to the existing bounded Ristretto cache; clearing it here
		// would disrupt concurrent readers for unrelated dependency trees.
		diskCacheGlobalGeneration.Add(1)
		diskCacheGenerations.Clear()
		diskCacheGenerationCount = 0
	}
	diskCacheGenerations.Store(cleanBasePath, generation)
	diskCacheGenerationCount++
}

func clearDiskMemoryCache() {
	diskCacheGenerationMu.Lock()
	defer diskCacheGenerationMu.Unlock()

	diskCacheGlobalGeneration.Add(1)
	diskMemoryCache.Clear()
	diskCacheGenerations.Clear()
	diskCacheGenerationCount = 0
}

func memoryCacheKey(kind string, basePath string, key string) string {
	cleanBasePath := filepath.Clean(basePath)
	for {
		globalGeneration := diskCacheGlobalGeneration.Load()
		var generation uint64
		if value, ok := diskCacheGenerations.Load(cleanBasePath); ok {
			generation = value.(uint64)
		}
		if globalGeneration != diskCacheGlobalGeneration.Load() {
			continue
		}
		return kind + ":" + strconv.FormatUint(globalGeneration, 10) + ":" +
			strconv.FormatUint(generation, 10) + ":" + cleanBasePath + ":" + key
	}
}

func waitForDiskMemoryCache() {
	diskMemoryCache.Wait()
}

func warmLatestDiskMemoryCache(ctx context.Context) {
	_, span := startSpan(ctx, "fly_tsgo.cache.warm",
		attribute.Int64("fly_tsgo.cache.warm.capacity.bytes", diskMemoryCacheMaxCost),
	)
	startedAt := time.Now()
	result := "empty"
	var byteCount int64
	var costCount int64
	var errorCount int
	var fileCount int
	var truncated bool
	defer func() {
		duration := time.Since(startedAt)
		span.SetAttributes(
			attribute.Int64("fly_tsgo.cache.warm.bytes", byteCount),
			attribute.Int64("fly_tsgo.cache.warm.cost.bytes", costCount),
			attribute.Float64("fly_tsgo.cache.warm.duration_ms", spanDurationMS(duration)),
			attribute.Int("fly_tsgo.cache.warm.errors.count", errorCount),
			attribute.Int("fly_tsgo.cache.warm.files.count", fileCount),
			attribute.String("fly_tsgo.cache.warm.result", result),
			attribute.Bool("fly_tsgo.cache.warm.truncated", truncated),
		)
		span.End()
		log.Printf("[CACHE] Startup warm: %s (%d files, %d bytes, %d cost bytes, %d errors, %v)", result, fileCount, byteCount, costCount, errorCount, duration)
	}()

	versions, err := getVersionsByModTime(diskCachePath)
	if err != nil {
		result = "error"
		errorCount = 1
		recordSpanError(span, "err-cache-warm-list-versions", err)
		return
	}

	var depDir string
	for index := len(versions) - 1; index >= 0; index-- {
		if filepath.Dir(versions[index].name) != "deps" {
			continue
		}
		candidate := filepath.Join(diskCachePath, versions[index].name)
		info, statErr := os.Stat(filepath.Join(candidate, "node_modules"))
		if statErr == nil && info.IsDir() {
			depDir = candidate
			break
		}
	}
	if depDir == "" {
		return
	}

	result = "success"
	span.SetAttributes(attribute.String("fly_tsgo.cache.warm.hash", filepath.Base(depDir)))
	fs := newDiskFSFromDeps(depDir)
	var firstErr error
	walkErr := filepath.WalkDir(filepath.Join(depDir, "node_modules"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			errorCount++
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(depDir, path)
		if err != nil {
			errorCount++
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		cachePath := "/" + filepath.ToSlash(relativePath)
		info, err := os.Stat(path)
		if err != nil {
			errorCount++
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		key := memoryCacheKey("file", fs.basePath, cachePath)
		estimatedCost := int64(len(key)) + info.Size() + 1
		if costCount+estimatedCost > diskMemoryCacheMaxCost {
			result = "bounded"
			truncated = true
			return filepath.SkipAll
		}

		content, exists := fs.readDiskFile(cachePath)
		if !exists {
			errorCount++
			if firstErr == nil {
				firstErr = errors.New("dependency file could not be read")
			}
			return nil
		}
		byteCount += int64(len(content))
		costCount += int64(len(key) + len(content) + 1)
		fileCount++
		return nil
	})
	waitForDiskMemoryCache()
	if walkErr != nil {
		errorCount++
		firstErr = walkErr
	}
	if firstErr != nil {
		result = "partial"
		recordSpanError(span, "err-cache-warm-read", firstErr)
	}
}
