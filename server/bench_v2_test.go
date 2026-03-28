package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// setupBenchServer initializes the server with mock S3 and pre-synced disk cache.
// Silences log output during benchmarks. Must be called once per top-level benchmark.
func setupBenchServer(b *testing.B) {
	b.Helper()

	mockS3 := NewMockS3Client()
	s3Client = mockS3
	s3Bucket = "test-bucket"
	serverVersion = "1.0.0"
	startTime = time.Now()

	tmpDir, err := os.MkdirTemp("", "bench-cache-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	diskCachePath = tmpDir

	// Pre-sync: trigger S3 download so iterations only see os.Stat
	_, err = newDiskFS(context.Background(), benchVersion)
	if err != nil {
		b.Fatalf("Failed to pre-sync diskFS: %v", err)
	}

	// Silence logs
	origWriter := log.Writer()
	log.SetOutput(io.Discard)

	b.Cleanup(func() {
		log.SetOutput(origWriter)
		os.RemoveAll(tmpDir)
	})
}

// Layer 1: Pure Compiler — calls typecheckTypeScriptV2 directly
func BenchmarkV2Typecheck(b *testing.B) {
	setupBenchServer(b)

	cases := []struct {
		name        string
		files       map[string]string
		entryPoints []string
	}{
		{"Trivial", singleFileFixture(fixtureTrivial), []string{"/index.tsx"}},
		{"SmallComponent", singleFileFixture(fixtureSmallComponent), []string{"/index.tsx"}},
		{"MediumComponent", singleFileFixture(fixtureMediumComponent), []string{"/index.tsx"}},
		{"MultiFile", fixtureMultiFile, []string{"/index.tsx"}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resp := typecheckTypeScriptV2(tc.files, tc.entryPoints, benchVersion)
				if len(resp.Errors) > 0 && resp.Errors[0].Message == "failed to sync version" {
					b.Fatalf("Unexpected sync error: %s", resp.Errors[0].Message)
				}
			}
		})
	}
}

// Layer 2: Full Pipeline — calls buildTypeScriptV2 directly (compiler + esbuild)
func BenchmarkV2Build(b *testing.B) {
	setupBenchServer(b)

	cases := []struct {
		name       string
		files      map[string]string
		entryPoint string
	}{
		{"Trivial", singleFileFixture(fixtureTrivial), "/index.tsx"},
		{"SmallComponent", singleFileFixture(fixtureSmallComponent), "/index.tsx"},
		{"MediumComponent", singleFileFixture(fixtureMediumComponent), "/index.tsx"},
		{"MultiFile", fixtureMultiFile, "/index.tsx"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var lastOutputBytes int
			for i := 0; i < b.N; i++ {
				resp := buildTypeScriptV2(tc.files, tc.entryPoint, benchVersion)
				if len(resp.Errors) > 0 && resp.Errors[0].Message == "failed to sync version" {
					b.Fatalf("Unexpected sync error: %s", resp.Errors[0].Message)
				}
				lastOutputBytes = len(resp.Code)
			}
			b.ReportMetric(float64(lastOutputBytes), "output_bytes/op")
		})
	}
}

// Layer 3: HTTP Handler — full request path through typecheckV2/buildV2 handlers
func BenchmarkV2HTTP(b *testing.B) {
	setupBenchServer(b)

	type httpCase struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
		body    []byte
	}

	// Pre-serialize request bodies outside the benchmark loop
	mediumTypecheckBody, _ := json.Marshal(TypecheckV2Request{
		Files:       singleFileFixture(fixtureMediumComponent),
		EntryPoints: []string{"/index.tsx"},
		Version:     benchVersion,
	})
	multiFileTypecheckBody, _ := json.Marshal(TypecheckV2Request{
		Files:       fixtureMultiFile,
		EntryPoints: []string{"/index.tsx"},
		Version:     benchVersion,
	})
	mediumBuildBody, _ := json.Marshal(BuildV2Request{
		Files:      singleFileFixture(fixtureMediumComponent),
		EntryPoint: "/index.tsx",
		Version:    benchVersion,
	})
	multiFileBuildBody, _ := json.Marshal(BuildV2Request{
		Files:      fixtureMultiFile,
		EntryPoint: "/index.tsx",
		Version:    benchVersion,
	})

	cases := []httpCase{
		{"Typecheck/MediumComponent", "POST", "/v2/typecheck", typecheckV2, mediumTypecheckBody},
		{"Typecheck/MultiFile", "POST", "/v2/typecheck", typecheckV2, multiFileTypecheckBody},
		{"Build/MediumComponent", "POST", "/v2/build", buildV2, mediumBuildBody},
		{"Build/MultiFile", "POST", "/v2/build", buildV2, multiFileBuildBody},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				tc.handler(w, req)
				if w.Code != http.StatusOK {
					b.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
				}
			}
		})
	}
}
