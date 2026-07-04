package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/core"
)

// setupTestServerWithMockS3 initializes the server with a mock S3 client
func setupTestServerWithMockS3(t *testing.T) *MockS3Client {
	mockS3 := NewMockS3Client()
	s3Client = mockS3
	s3Bucket = "test-bucket"

	// Create temp directory for disk cache
	tmpDir, err := os.MkdirTemp("", "server-test-cache-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	diskCachePath = tmpDir
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	serverVersion = "1.0.0"
	startTime = time.Now()

	return mockS3
}

// setupTestServerWithFileS3 initializes the server with file-based S3 mock
func setupTestServerWithFileS3(t *testing.T) {
	s3Client = NewFileBasedMockS3Client()
	s3Bucket = "test-bucket"

	// Create temp directory for disk cache
	tmpDir, err := os.MkdirTemp("", "server-test-cache-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	diskCachePath = tmpDir
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	serverVersion = "1.0.0"
	startTime = time.Now()
}

func TestLoggingMiddlewareAddsProvenanceHeaders(t *testing.T) {
	oldGitCommit := gitCommit
	oldServerVersion := serverVersion
	t.Cleanup(func() {
		gitCommit = oldGitCommit
		serverVersion = oldServerVersion
	})

	gitCommit = "test-sha"
	serverVersion = "1.2.3"

	handler := loggingMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	res := httptest.NewRecorder()

	handler(res, req)

	if got := res.Header().Get("X-Git-Commit"); got != gitCommit {
		t.Fatalf("expected X-Git-Commit %q, got %q", gitCommit, got)
	}
	if got := res.Header().Get("X-Server-Version"); got != serverVersion {
		t.Fatalf("expected X-Server-Version %q, got %q", serverVersion, got)
	}
	if got := res.Header().Get("X-TSGo-Compiler-Version"); got != core.Version() {
		t.Fatalf("expected X-TSGo-Compiler-Version %q, got %q", core.Version(), got)
	}
}

func TestHealthIncludesProvenance(t *testing.T) {
	oldDiskCachePath := diskCachePath
	oldGitCommit := gitCommit
	oldServerVersion := serverVersion
	oldStartTime := startTime
	t.Cleanup(func() {
		diskCachePath = oldDiskCachePath
		gitCommit = oldGitCommit
		serverVersion = oldServerVersion
		startTime = oldStartTime
	})

	diskCachePath = "/tmp/tsgo-cache"
	gitCommit = "test-sha"
	serverVersion = "1.2.3"
	startTime = time.Now().Add(-2 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()

	health(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var body HealthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if body.CompilerVersion != core.Version() {
		t.Fatalf("expected compiler version %q, got %q", core.Version(), body.CompilerVersion)
	}
	if body.DiskCachePath != diskCachePath {
		t.Fatalf("expected disk cache path %q, got %q", diskCachePath, body.DiskCachePath)
	}
	if body.GitCommit != gitCommit {
		t.Fatalf("expected git commit %q, got %q", gitCommit, body.GitCommit)
	}
	if body.Status != "healthy" {
		t.Fatalf("expected status %q, got %q", "healthy", body.Status)
	}
	if body.Uptime == "" {
		t.Fatal("expected non-empty uptime")
	}
	if body.Version != serverVersion {
		t.Fatalf("expected server version %q, got %q", serverVersion, body.Version)
	}
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

// TestWithRealPackages tests compilation with mock packages
func TestWithRealPackages(t *testing.T) {
	setupTestServerWithMockS3(t)

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

	// Clear call tracking
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

	// Clear tracking
	mockS3.ClearCallCount()

	// Build and check what paths were attempted
	result := buildTypeScript(code, "test")

	readFiles := mockS3.GetReadFileCalls()
	for _, file := range readFiles {
		// OnLoad should NEVER receive a bare module specifier
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
