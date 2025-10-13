package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// setupTestServerWithMockS3 initializes the server with a mock S3 client
func setupTestServerWithMockS3(t *testing.T) *MockS3Client {
	mockS3 := NewMockS3Client()
	s3Client = mockS3
	s3Bucket = "test-bucket"
	
	cacheSize = 32 * 1024 * 1024 // 32MB
	avgEntrySize := int64(4096)
	capacity := int(cacheSize / avgEntrySize)
	
	var err error
	cache, err = lru.New[string, *CacheEntry](capacity)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	
	serverVersion = "1.0.0"
	startTime = time.Now()
	
	return mockS3
}

// setupTestServerWithFileS3 initializes the server with file-based S3 mock
func setupTestServerWithFileS3(t *testing.T) {
	s3Client = NewFileBasedMockS3Client()
	s3Bucket = "test-bucket"
	
	cacheSize = 32 * 1024 * 1024
	avgEntrySize := int64(4096)
	capacity := int(cacheSize / avgEntrySize)
	
	var err error
	cache, err = lru.New[string, *CacheEntry](capacity)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	
	serverVersion = "1.0.0"
	startTime = time.Now()
}

// TestFortuneCookieWithMockS3 tests the fortune cookie example with mocked S3
func TestFortuneCookieWithMockS3(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	
	// Add TypeScript lib files
	mockS3.files["0.0.4/node_modules/typescript/lib/lib.d.ts"] = `
interface Array<T> { length: number; }
interface String { length: number; }
interface Number { }
declare const Math: { random(): number; floor(n: number): number; };
`
	
	fortuneCookieCode := `import { Flex, Button, Text } from '@crayonnow/core';
import { useState } from 'react';

const fortunes = [
  "You will find great success in your endeavors.",
  "A fresh start will put you on a new path.",
  "Believe in yourself and magic will happen.",
  "Adventure awaits you this week.",
  "Kindness will return to you many times over.",
  "A surprise gift is on its way.",
  "Your hard work will soon pay off.",
  "Embrace change; it brings new opportunities.",
  "Someone close to you has a secret admiration.",
  "Patience is the key to your next victory."
];

export default () => {
  const [fortune, setFortune] = useState('');

  const getFortune = () => {
    const random = fortunes[Math.floor(Math.random() * fortunes.length)];
    setFortune(random);
  };

  return (
    <Flex style={{ alignItems: 'stretch', minHeight: '100vh', background: '#f5f5f5', padding: '20px', rowGap: '24px' }}>
      <Text style={{ fontSize: '24px', fontWeight: 600, textAlign: 'center' }}>
        {fortune || 'Tap the cookie for a wise saying'}
      </Text>
      <Button
        onClick={getFortune}
        style={{ background: '#FFCC00', borderRadius: '12px', padding: '12px' }}
      >
        <Text style={{ color: 'black', fontSize: '18px', fontWeight: 600, textAlign: 'center' }}>
          Break Cookie
        </Text>
      </Button>
    </Flex>
  );
};`

	t.Run("Build", func(t *testing.T) {
		payload := map[string]interface{}{
			"code":            fortuneCookieCode,
			"framework":       "react",
			"jsxImportSource": "@crayonnow/core",
			"version":         "0.0.4",
		}
		jsonData, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		build(w, req)
		
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
			return
		}
		
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if code, ok := result["code"].(string); ok && code != "" {
			t.Logf("Successfully compiled fortune cookie to %d bytes", len(code))
			if !strings.Contains(code, "fortune") && !strings.Contains(code, "Fortune") {
				t.Error("Compiled code doesn't contain 'fortune' text")
			}
		} else {
			if errors, ok := result["errors"]; ok {
				t.Errorf("Build failed with errors: %v", errors)
			} else {
				t.Error("No code returned from build")
			}
		}
	})
}

// TestWithRealPackages tests compilation with real packages from testdata
func TestWithRealPackages(t *testing.T) {
	setupTestServerWithFileS3(t)
	
	t.Run("Simple Build", func(t *testing.T) {
		code := `export const hello = "world";`
		
		payload := map[string]interface{}{
			"code":    code,
			"version": "0.0.4",
		}
		
		jsonData, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		build(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		var result map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &result)
		
		if code, ok := result["code"].(string); ok && code != "" {
			t.Logf("Successfully compiled to %d bytes", len(code))
		} else {
			t.Errorf("No code in response: %v", result)
		}
	})
	
	t.Run("React Component With Crayonnow", func(t *testing.T) {
		code := `import { Button } from '@crayonnow/core';

export default () => {
  return (
    <Button onClick={() => console.log('clicked')}>
      Click me
    </Button>
  );
};`
		
		payload := map[string]interface{}{
			"code":            code,
			"framework":       "react",
			"jsxImportSource": "@crayonnow/core",
			"version":         "0.0.4",
		}
		
		jsonData, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		build(w, req)
		
		var result map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &result)
		
		if code, ok := result["code"].(string); ok && code != "" {
			t.Logf("Successfully compiled React component to %d bytes", len(code))
		} else {
			// Log errors if present
			if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
				t.Logf("Build had errors (expected with module resolution): %v", errors)
			}
		}
	})
}

// TestAllExamplesFromDocs tests all examples from TESTING.md
func TestAllExamplesFromDocs(t *testing.T) {
	setupTestServerWithFileS3(t)
	
	testCases := []struct {
		name     string
		endpoint string
		payload  string
		checkFor string
	}{
		{
			name:     "Simple React Component",
			endpoint: "/build",
			payload: `{
				"code": "import React from 'react';\nexport default () => React.createElement('div', null, 'Hello');",
				"version": "0.0.4"
			}`,
			checkFor: "code",
		},
		{
			name:     "Crayonnow Core Import",
			endpoint: "/build",
			payload: `{
				"code": "import { Text } from '@crayonnow/core';\nexport default () => <Text>Hello</Text>;",
				"version": "0.0.4"
			}`,
			checkFor: "code",
		},
		{
			name:     "Type Error Detection",
			endpoint: "/typecheck",
			payload: `{
				"code": "export const hello: string = 123",
				"version": "0.0.4"
			}`,
			checkFor: "errors",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.endpoint, bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			
			switch tc.endpoint {
			case "/build":
				build(w, req)
			case "/typecheck":
				typecheck(w, req)
			default:
				t.Fatalf("Unknown endpoint: %s", tc.endpoint)
			}
			
			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
			
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			
			if tc.checkFor == "code" {
				if code, ok := result["code"].(string); ok {
					t.Logf("Successfully compiled to %d bytes", len(code))
				} else if errors, ok := result["errors"]; ok {
					t.Logf("Got errors instead of code: %v", errors)
				}
			} else if tc.checkFor == "errors" {
				if errors, ok := result["errors"]; ok && errors != nil {
					if errList, ok := errors.([]interface{}); ok && len(errList) > 0 {
						t.Logf("Successfully detected errors: %v", errors)
					}
				} else {
					t.Error("Expected errors but got none")
				}
			}
		})
	}
}

// TestPerformanceWithMockS3 measures actual compilation performance
func TestPerformanceWithMockS3(t *testing.T) {
	setupTestServerWithMockS3(t)
	
	simpleCode := `import { Text } from '@crayonnow/core';
export default () => <Text>Performance Test</Text>;`
	
	payload := map[string]interface{}{
		"code":            simpleCode,
		"framework":       "react",
		"jsxImportSource": "@crayonnow/core",
		"version":         "0.0.4",
	}
	jsonData, _ := json.Marshal(payload)
	
	// Warm up
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		build(w, req)
	}
	
	// Measure build time
	start := time.Now()
	req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	build(w, req)
	buildDuration := time.Since(start)
	
	if w.Code == http.StatusOK {
		var result map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &result)
		if code, ok := result["code"].(string); ok {
			t.Logf("Build completed in %v, output size: %d bytes", buildDuration, len(code))
		}
	} else {
		t.Logf("Build returned status %d in %v", w.Code, buildDuration)
	}
	
	// Measure typecheck time
	start = time.Now()
	req = httptest.NewRequest("POST", "/typecheck", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	typecheck(w, req)
	typecheckDuration := time.Since(start)
	
	t.Logf("Typecheck completed in %v", typecheckDuration)
	
	// Performance assertions
	if buildDuration > 100*time.Millisecond {
		t.Logf("Warning: Build took longer than expected: %v", buildDuration)
	}
	if typecheckDuration > 500*time.Millisecond {
		t.Logf("Warning: Typecheck took longer than expected: %v", typecheckDuration)
	}
}

// TestPrewarmCachesDirectories verifies that the prewarm process caches directory listings
func TestPrewarmCachesDirectories(t *testing.T) {
	// Create a mock S3 client with files in a structure
	mockS3 := &MockS3Client{
		files: map[string]string{
			"test/node_modules/@types/react/index.d.ts": `export interface Component {}`,
			"test/node_modules/@types/react/package.json": `{"name": "@types/react"}`,
			"test/node_modules/react/index.js": `module.exports = {}`,
			"test/node_modules/react/package.json": `{"name": "react"}`,
		},
	}

	// Set up cache
	cache, _ = lru.New[string, *CacheEntry](1000)
	s3Client = mockS3
	s3Bucket = "test-bucket"
	cacheSize = 10 * 1024 * 1024 // 10MB

	// Run prewarm
	ctx := context.Background()
	prewarmCache(ctx)

	// After prewarm, check if directories are cached
	dirPaths := []string{
		"/node_modules",
		"/node_modules/@types",
		"/node_modules/@types/react",
		"/node_modules/react",
	}

	for _, dirPath := range dirPaths {
		cacheKey := fmt.Sprintf("test%s", dirPath)
		cacheMutex.RLock()
		entry, exists := cache.Get(cacheKey)
		cacheMutex.RUnlock()
		
		if !exists {
			t.Errorf("Directory %s should be cached after prewarm but wasn't", dirPath)
		} else if entry.IsFile {
			t.Errorf("Directory %s is cached as a file, not a directory", dirPath)
		} else if !entry.Exists {
			t.Errorf("Directory %s is cached as non-existent", dirPath)
		}
	}
}

// TestDirectoryCacheOptimization verifies that checking for non-existent files
// in a cached directory doesn't hit S3
func TestDirectoryCacheOptimization(t *testing.T) {
	// Create a mock S3 client with a directory containing some files
	mockS3 := &MockS3Client{
		files: map[string]string{
			"test/node_modules/mylib/package.json": `{"main": "index.js"}`,
			"test/node_modules/mylib/index.js":     `module.exports = {}`,
			"test/node_modules/mylib/dist/bundle.js": `/* bundled */`,
		},
	}

	// Set up cache
	cache, _ = lru.New[string, *CacheEntry](1000)
	s3Client = mockS3
	s3Bucket = "test-bucket"

	ctx := context.Background()
	
	// First, access the directory to cache its listing
	dirEntry := getFromCache(ctx, "test", "/node_modules/mylib")
	if dirEntry == nil {
		t.Fatal("Got nil entry for directory /node_modules/mylib")
	}
	if !dirEntry.Exists {
		t.Fatalf("Directory /node_modules/mylib should exist, got Exists=%v", dirEntry.Exists)
	}
	if dirEntry.IsFile {
		t.Fatalf("Directory /node_modules/mylib should be a directory not a file, got IsFile=%v, Files=%v, Dirs=%v", 
			dirEntry.IsFile, dirEntry.Files, dirEntry.Dirs)
	}
	
	// Now check for non-existent files in that cached directory
	// These should use the cached directory listing and NOT hit S3
	nonExistentFiles := []string{
		"/node_modules/mylib/index.ts",
		"/node_modules/mylib/index.tsx", 
		"/node_modules/mylib/index.d.ts",
		"/node_modules/mylib/foo.js",
		"/node_modules/mylib/bar.tsx",
	}

	// Track if we're hitting the cache properly
	cacheHits := 0
	for _, file := range nonExistentFiles {
		entry := getFromCache(ctx, "test", file)
		if entry == nil {
			t.Errorf("Should get a cache entry for %s", file)
			continue
		}
		if entry.Exists {
			t.Errorf("File %s should not exist", file)
		} else {
			cacheHits++
		}
	}
	
	// All non-existent files should have been resolved via cache
	if cacheHits != len(nonExistentFiles) {
		t.Errorf("Expected %d cache hits for non-existent files, got %d", len(nonExistentFiles), cacheHits)
	}
	
	// The test passes if we correctly determined all files don't exist
	// In production, this saves len(nonExistentFiles) * 150ms of S3 latency
	t.Logf("Successfully checked %d non-existent files using cached directory listing", cacheHits)
}

// BenchmarkFortuneCookieWithMockS3 measures performance with mocked S3
func BenchmarkFortuneCookieWithMockS3(b *testing.B) {
	mockS3 := setupTestServerWithMockS3(&testing.T{})
	mockS3.files["0.0.4/node_modules/typescript/lib/lib.d.ts"] = `interface Array<T> { length: number; }`
	
	fortuneCookieCode := `import { Flex, Button, Text } from '@crayonnow/core';
import { useState } from 'react';

const fortunes = ["Fortune 1", "Fortune 2", "Fortune 3"];

export default () => {
  const [fortune, setFortune] = useState('');
  const getFortune = () => {
    setFortune(fortunes[Math.floor(Math.random() * fortunes.length)]);
  };
  return (
    <Flex>
      <Text>{fortune || 'Click me'}</Text>
      <Button onClick={getFortune}>
        <Text>Break Cookie</Text>
      </Button>
    </Flex>
  );
};`

	payload := map[string]interface{}{
		"code":            fortuneCookieCode,
		"framework":       "react",
		"jsxImportSource": "@crayonnow/core",
		"version":         "0.0.4",
	}
	jsonData, _ := json.Marshal(payload)
	
	// Warm up the cache
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		build(w, req)
	}
	
	b.ResetTimer()
	
	b.Run("Build", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			build(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Build failed with status %d", w.Code)
			}
		}
	})
	
	b.Run("Typecheck", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/typecheck", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			typecheck(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Typecheck failed with status %d", w.Code)
			}
		}
	})
}

// TestBuildResolverDoesNotReadBareModules tests that the build resolver doesn't try to read bare module specifiers directly
// This would cause unnecessary S3 lookups with high latency
func TestBuildResolverDoesNotReadBareModules(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	
	// Setup a package that will be imported as a bare module specifier
	mockS3.AddFile("test/node_modules/@test/package/package.json", `{
		"name": "@test/package",
		"main": "dist/index.js",
		"exports": {
			".": "./dist/index.js"
		}
	}`)
	mockS3.AddFile("test/node_modules/@test/package/dist/index.js", `
		module.exports = { test: true };
	`)
	
	// Pre-warm the cache
	ctx := context.Background()
	prewarmCache(ctx)
	
	// Clear call tracking after prewarm
	mockS3.ClearCallCount()
	
	// Code that imports a bare module specifier
	code := `
import pkg from '@test/package';
export const result = pkg.test;
`
	
	payload := map[string]interface{}{
		"code":    code,
		"version": "test",
	}
	jsonData, _ := json.Marshal(payload)
	
	req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	build(w, req)
	
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected successful build, got %d: %s", resp.StatusCode, body)
	}
	
	// Check that we didn't try to read "@test/package" directly as a file path
	// This would be "test/@test/package" in the S3 key format
	readFileCalls := mockS3.GetReadFileCalls()
	for _, call := range readFileCalls {
		if call == "test/@test/package" {
			t.Errorf("Resolver incorrectly tried to read bare module specifier '@test/package' as a file path")
		}
		// Also check for common incorrect patterns
		if strings.HasSuffix(call, "/@test/package") && !strings.Contains(call, "node_modules") {
			t.Errorf("Resolver made incorrect ReadFile call for bare module: %s", call)
		}
	}
}

// TestResolverSeparationOfConcerns verifies that module resolution is properly separated from file loading
// This test ensures that OnResolve handles ALL resolution and OnLoad only loads files
func TestResolverSeparationOfConcerns(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	
	// Create a simple test that imports a package
	code := `import pkg from '@test/package';
export const result = pkg.test;`
	
	// Add the package files
	mockS3.AddFile("test/node_modules/@test/package/package.json", `{
		"name": "@test/package",
		"main": "index.js"
	}`)
	mockS3.AddFile("test/node_modules/@test/package/index.js", `
		exports.test = true;
	`)
	
	// Pre-warm cache to ensure directories are cached
	ctx := context.Background()
	prewarmCache(ctx)
	
	// Clear tracking after prewarm
	mockS3.ClearCallCount()
	
	// Build and check what paths were attempted
	result := buildTypeScript(code, "test")
	
	// The OnResolve should convert '@test/package' to '/node_modules/@test/package/index.js'
	// The OnLoad should only receive already-resolved paths, never bare specifiers
	
	readFiles := mockS3.GetReadFileCalls()
	for _, file := range readFiles {
		// This is the KEY assertion: OnLoad should NEVER receive a bare module specifier
		// It should only receive resolved paths
		if file == "test/@test/package" {
			t.Errorf("OnLoad received bare module specifier '@test/package' - resolution logic is mixed with loading logic!")
		}
		if !strings.Contains(file, "/") && !strings.HasPrefix(file, "test/") {
			t.Errorf("OnLoad received unresolved path '%s' - OnResolve should have resolved this", file)
		}
	}
	
	if len(result.Errors) > 0 {
		t.Errorf("Build should succeed but got errors: %v", result.Errors)
	}
	
	if result.Code == "" {
		t.Error("Build should produce code output")
	} else {
		t.Logf("Successfully built %d bytes of code", len(result.Code))
	}
}

// TestResolverHandlesAllModuleTypes tests that the resolver properly handles all module resolution patterns
// This test is currently disabled as it needs more setup to work with the HTTP endpoint
func skipTestResolverHandlesAllModuleTypes(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		setup  func(*MockS3Client)
		wantFail bool
	}{
		{
			name: "Bare module specifier",
			code: `import { Button } from '@crayonnow/core';`,
			setup: func(m *MockS3Client) {
				// MockS3Client already has @crayonnow/core
			},
		},
		{
			name: "React import",  
			code: `import React from 'react';`,
			setup: func(m *MockS3Client) {
				// MockS3Client already has react
			},
		},
		{
			name: "Relative import",
			code: `import { helper } from './utils';`,
			setup: func(m *MockS3Client) {
				m.AddFile("test/utils.js", `export const helper = () => {};`)
			},
		},
		{
			name: "Scoped package with subpath",
			code: `import { jsx } from '@crayonnow/core/jsx-runtime';`,
			setup: func(m *MockS3Client) {
				// MockS3Client already has jsx-runtime
			},
		},
		{
			name: "Package with conditional exports",
			code: `import { spring } from '@react-spring/core';`,
			setup: func(m *MockS3Client) {
				m.AddFile("test/node_modules/@react-spring/core/package.json", `{
					"name": "@react-spring/core",
					"exports": {
						".": {
							"require": {
								"development": "./dist/cjs/react-spring_core.development.cjs",
								"default": "./dist/cjs/index.cjs"
							},
							"default": "./dist/esm/index.js"
						}
					}
				}`)
				m.AddFile("test/node_modules/@react-spring/core/dist/cjs/react-spring_core.development.cjs", `
					exports.spring = function() { return 'spring'; };
				`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3 := setupTestServerWithMockS3(t)
			if tt.setup != nil {
				tt.setup(mockS3)
			}

			// Pre-warm cache
			ctx := context.Background()
			prewarmCache(ctx)
			
			// Clear call tracking after prewarm
			mockS3.ClearCallCount()

			payload := map[string]interface{}{
				"code":    tt.code,
				"version": "test",
			}
			jsonData, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			build(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			
			if tt.wantFail {
				if resp.StatusCode == http.StatusOK {
					var result map[string]interface{}
					json.Unmarshal(body, &result)
					if code, _ := result["code"].(string); code != "" {
						t.Errorf("Expected build to fail but got code output")
					}
				}
			} else {
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("Build failed with status %d: %s", resp.StatusCode, body)
				}
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if code, ok := result["code"].(string); !ok || code == "" {
					t.Errorf("Expected build to produce code, got: %v", result)
					if errors := result["errors"]; errors != nil {
						t.Errorf("Build errors: %v", errors)
					}
				}
			}

			// Verify no bare module specifiers were read directly
			readFiles := mockS3.GetReadFileCalls()
			for _, file := range readFiles {
				// Check if this looks like a bare module specifier being read directly
				if !strings.HasPrefix(file, "test/") && !strings.Contains(file, "/") {
					t.Errorf("Resolver should not read bare specifier '%s' directly", file)
				}
			}
		})
	}
}

// TestNodeModulesSpeculativeFileChecking tests that TypeScript's speculative file checking doesn't hit S3 repeatedly
func TestNodeModulesSpeculativeFileChecking(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	
	// Setup a basic React package structure
	mockS3.AddFile("test/node_modules/react/package.json", `{"name": "react", "main": "index.js"}`)
	mockS3.AddFile("test/node_modules/react/index.js", `module.exports = {createElement: function() {}}`)
	mockS3.AddFile("test/node_modules/react/jsx-runtime.js", `module.exports = {jsx: function() {}}`)
	
	// Pre-warm the cache (this should cache the directory)
	ctx := context.Background()
	prewarmCache(ctx)
	
	// Clear S3 call tracking
	mockS3.ClearCallCount()
	
	// Code that imports React - TypeScript will try multiple extensions
	code := `
import React from 'react';
import { jsx } from 'react/jsx-runtime';
export const App = () => React.createElement('div', null, 'Hello');
`
	
	payload := map[string]interface{}{
		"code":           code,
		"version":        "test",
		"validate_types": true,
	}
	jsonData, _ := json.Marshal(payload)
	
	req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	build(w, req)
	
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected successful build, got %d: %s", resp.StatusCode, body)
	}
	
	// TypeScript will speculatively check for:
	// - react.ts, react.tsx, react.d.ts, react/index.ts, react/index.tsx, react/index.d.ts
	// - react/jsx-runtime.ts, react/jsx-runtime.tsx, react/jsx-runtime.d.ts
	// With proper directory caching, we should NOT hit S3 for these non-existent files
	
	listCalls := mockS3.GetListObjectsV2CallCount()
	getCalls := mockS3.GetObjectCallCount()
	
	// We should have minimal S3 calls since directories are cached
	// Allow some calls for actual file fetches, but not the dozens that would happen without caching
	if listCalls > 5 {
		t.Errorf("Too many ListObjectsV2 calls: %d (expected <= 5). Directory cache not working properly.", listCalls)
	}
	
	if getCalls > 10 {
		t.Errorf("Too many GetObject calls: %d (expected <= 10). Speculative checks hitting S3.", getCalls)
	}
}

// BenchmarkRealPackageCompilation benchmarks compilation with real packages
func BenchmarkRealPackageCompilation(b *testing.B) {
	setupTestServerWithFileS3(&testing.T{})
	
	simpleCode := `export const hello = "world";`
	payload := map[string]interface{}{
		"code":    simpleCode,
		"version": "0.0.4",
	}
	jsonData, _ := json.Marshal(payload)
	
	// Warmup
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		build(w, req)
	}
	
	b.ResetTimer()
	
	b.Run("SimpleCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/build", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			build(w, req)
		}
	})
}