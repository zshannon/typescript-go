package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadDiskFileDoesNotCacheTransientErrors(t *testing.T) {
	basePath := t.TempDir()
	cachePath := "/node_modules/pkg/index.js"
	diskPath := filepath.Join(basePath, cachePath)

	if err := os.MkdirAll(diskPath, 0o755); err != nil {
		t.Fatalf("create directory at file path: %v", err)
	}

	fs := newDiskFSFromDeps(basePath)
	if _, exists := fs.readDiskFile(cachePath); exists {
		t.Fatal("directory read unexpectedly reported a file")
	}
	waitForDiskMemoryCache()

	if err := os.RemoveAll(diskPath); err != nil {
		t.Fatalf("remove directory at file path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		t.Fatalf("create file parent: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte("export default 1"), 0o644); err != nil {
		t.Fatalf("write file after transient error: %v", err)
	}

	content, exists := fs.readDiskFile(cachePath)
	if !exists || content != "export default 1" {
		t.Fatalf("read after transient error = (%q, %v), want fresh file", content, exists)
	}
}

func TestFlushDepsHashKeepsUnrelatedDependencyTreeHot(t *testing.T) {
	setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	targetHash := hashBunLock([]byte("target dependency tree"))
	unrelatedHash := hashBunLock([]byte("unrelated dependency tree"))
	targetDir := depsCacheDir(targetHash)
	unrelatedDir := depsCacheDir(unrelatedHash)
	cachePath := "/node_modules/pkg/index.js"
	targetDiskPath := filepath.Join(targetDir, cachePath)
	diskPath := filepath.Join(unrelatedDir, cachePath)
	if err := os.MkdirAll(filepath.Dir(targetDiskPath), 0o755); err != nil {
		t.Fatalf("create target dependency directory: %v", err)
	}
	if err := os.WriteFile(targetDiskPath, []byte("export default 0"), 0o644); err != nil {
		t.Fatalf("write target dependency: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		t.Fatalf("create unrelated dependency directory: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte("export default 1"), 0o644); err != nil {
		t.Fatalf("write unrelated dependency: %v", err)
	}

	targetFS := newDiskFSFromDeps(targetDir)
	if _, exists := targetFS.readDiskFile(cachePath); !exists {
		t.Fatal("prime target dependency cache: file not found")
	}
	unrelatedFS := newDiskFSFromDeps(unrelatedDir)
	if _, exists := unrelatedFS.readDiskFile(cachePath); !exists {
		t.Fatal("prime unrelated dependency cache: file not found")
	}
	waitForDiskMemoryCache()

	if _, err := flushDepsHash(context.Background(), targetHash); err != nil {
		t.Fatalf("flush target dependency tree: %v", err)
	}

	if _, exists := targetFS.readDiskFile(cachePath); exists {
		t.Fatal("target dependency remained cached after targeted flush")
	}
	if _, exists := unrelatedFS.readDiskFile(cachePath); !exists {
		t.Fatal("read unrelated dependency after targeted flush: file not found")
	}
	stats := unrelatedFS.cacheSnapshot()
	if stats.fileHits != 1 || stats.fileMisses != 1 {
		t.Fatalf("unrelated cache stats after targeted flush = hits %d, misses %d; want 1 hit and 1 miss", stats.fileHits, stats.fileMisses)
	}
}

func TestInvalidateDiskMemoryCacheBoundsGenerationMetadata(t *testing.T) {
	const maxGenerationEntries = 16_384

	clearDiskMemoryCache()
	t.Cleanup(clearDiskMemoryCache)
	startingGlobalGeneration := diskCacheGlobalGeneration.Load()

	for index := 0; index <= maxGenerationEntries; index++ {
		invalidateDiskMemoryCache(filepath.Join("/cache", strconv.Itoa(index)))
	}

	entryCount := 0
	diskCacheGenerations.Range(func(_, _ any) bool {
		entryCount++
		return true
	})
	if entryCount != 1 {
		t.Fatalf("generation metadata entries after rollover = %d, want 1", entryCount)
	}
	if got := diskCacheGlobalGeneration.Load(); got != startingGlobalGeneration+1 {
		t.Fatalf("global generation after rollover = %d, want %d", got, startingGlobalGeneration+1)
	}
}
