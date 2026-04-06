package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/evanw/esbuild/pkg/api"
)

func TestNewDiskFSFromDeps(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "deps-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake node_modules structure
	nmDir := filepath.Join(tmpDir, "node_modules", "zod")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "index.d.ts"), []byte("export declare function string(): any;"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := newDiskFSFromDeps(tmpDir)
	fs.hasUserFiles = true
	fs.userFiles["/src/index.ts"] = "import { string } from 'zod';"

	// Should be able to read the user file
	content, exists := fs.ReadFile("/src/index.ts")
	if !exists {
		t.Fatal("user file not found")
	}
	if content != "import { string } from 'zod';" {
		t.Fatalf("unexpected content: %s", content)
	}

	// Should be able to read from node_modules on disk
	content, exists = fs.ReadFile("/node_modules/zod/index.d.ts")
	if !exists {
		t.Fatal("node_modules file not found")
	}
	if content != "export declare function string(): any;" {
		t.Fatalf("unexpected content: %s", content)
	}
}

func TestMockS3PutObject(t *testing.T) {
	mock := NewMockS3Client()
	ctx := context.Background()

	body := strings.NewReader("test content")
	_, err := mock.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("deps/abc123.tar.gz"),
		Body:   body,
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	result, err := mock.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("deps/abc123.tar.gz"),
	})
	if err != nil {
		t.Fatalf("GetObject after PutObject failed: %v", err)
	}
	defer result.Body.Close()
	data, _ := io.ReadAll(result.Body)
	if string(data) != "test content" {
		t.Fatalf("expected 'test content', got %q", string(data))
	}
}

// buildMultipart creates a multipart/form-data body from a map of path->content.
func buildMultipart(files map[string][]byte) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for name, content := range files {
		fw, _ := w.CreateFormFile(name, name)
		fw.Write(content)
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

// Task 4: parseV3Multipart tests

func TestParseV3Multipart_Valid(t *testing.T) {
	files := map[string][]byte{
		"/package.json": []byte(`{"name":"test","main":"src/index.ts"}`),
		"/bun.lock":     []byte(`{}`),
		"/src/index.ts": []byte(`export const x = 1;`),
	}
	buf, ct := buildMultipart(files)

	result, err := parseV3Multipart(buf, ct)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result))
	}
	if _, ok := result["/package.json"]; !ok {
		t.Error("missing /package.json")
	}
	if _, ok := result["/bun.lock"]; !ok {
		t.Error("missing /bun.lock")
	}
	if _, ok := result["/src/index.ts"]; !ok {
		t.Error("missing /src/index.ts")
	}
}

func TestParseV3Multipart_MissingPackageJSON(t *testing.T) {
	files := map[string][]byte{
		"/bun.lock":     []byte(`{}`),
		"/src/index.ts": []byte(`export const x = 1;`),
	}
	buf, ct := buildMultipart(files)

	_, err := parseV3Multipart(buf, ct)
	if err == nil {
		t.Fatal("expected error for missing package.json, got nil")
	}
}

func TestParseV3Multipart_MissingBunLock(t *testing.T) {
	files := map[string][]byte{
		"/package.json": []byte(`{"name":"test","main":"src/index.ts"}`),
		"/src/index.ts": []byte(`export const x = 1;`),
	}
	buf, ct := buildMultipart(files)

	_, err := parseV3Multipart(buf, ct)
	if err == nil {
		t.Fatal("expected error for missing bun.lock, got nil")
	}
}

func TestParseV3Multipart_NodeModulesRejected(t *testing.T) {
	files := map[string][]byte{
		"/package.json":               []byte(`{"name":"test","main":"src/index.ts"}`),
		"/bun.lock":                   []byte(`{}`),
		"/node_modules/evil/index.js": []byte(`module.exports = {}`),
	}
	buf, ct := buildMultipart(files)

	_, err := parseV3Multipart(buf, ct)
	if err == nil {
		t.Fatal("expected error for node_modules path, got nil")
	}
}

func TestParseV3Multipart_TooManyFiles(t *testing.T) {
	files := map[string][]byte{
		"/package.json": []byte(`{"name":"test","main":"src/index.ts"}`),
		"/bun.lock":     []byte(`{}`),
	}
	// Add 101 source files (exceeds maxFilesPerRequest=100)
	for i := 0; i < 101; i++ {
		files[fmt.Sprintf("/src/file%d.ts", i)] = []byte(`export const x = 1;`)
	}
	buf, ct := buildMultipart(files)

	_, err := parseV3Multipart(buf, ct)
	if err == nil {
		t.Fatal("expected error for too many files, got nil")
	}
}

// Task 5: parsePackageJSON and esbuildOptions tests

func TestParsePackageJSON_Valid(t *testing.T) {
	raw := []byte(`{
		"name": "my-app",
		"main": "src/index.ts",
		"dependencies": {"react": "^18.0.0"},
		"devDependencies": {"typescript": "^5.0.0"},
		"esbuild": {
			"bundle": true,
			"external": ["react"],
			"format": "esm",
			"minifyIdentifiers": true,
			"minifySyntax": false,
			"minifyWhitespace": false,
			"platform": "node",
			"target": "es2020"
		}
	}`)

	pkg, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Main != "src/index.ts" {
		t.Errorf("expected main=src/index.ts, got %q", pkg.Main)
	}
	if len(pkg.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(pkg.Dependencies))
	}
	if len(pkg.DevDependencies) != 1 {
		t.Errorf("expected 1 devDependency, got %d", len(pkg.DevDependencies))
	}
	if pkg.Esbuild.Format != "esm" {
		t.Errorf("expected format=esm, got %q", pkg.Esbuild.Format)
	}
	if len(pkg.Esbuild.External) != 1 || pkg.Esbuild.External[0] != "react" {
		t.Errorf("expected external=[react], got %v", pkg.Esbuild.External)
	}
}

func TestParsePackageJSON_Defaults(t *testing.T) {
	raw := []byte(`{"name":"app","main":"index.ts"}`)

	pkg, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts := pkg.Esbuild.esbuildOptions()

	if !opts.Bundle {
		t.Error("expected Bundle=true by default")
	}
	if len(opts.External) != 0 {
		t.Errorf("expected External=[] by default, got %v", opts.External)
	}
	if opts.Format != api.FormatCommonJS {
		t.Errorf("expected Format=CJS by default, got %v", opts.Format)
	}
	if !opts.MinifySyntax {
		t.Error("expected MinifySyntax=true by default")
	}
	if !opts.MinifyWhitespace {
		t.Error("expected MinifyWhitespace=true by default")
	}
	if opts.MinifyIdentifiers {
		t.Error("expected MinifyIdentifiers=false by default")
	}
	if opts.Platform != api.PlatformBrowser {
		t.Errorf("expected Platform=browser by default, got %v", opts.Platform)
	}
	if opts.Target != api.ES2022 {
		t.Errorf("expected Target=ES2022 by default, got %v", opts.Target)
	}
	if opts.Write {
		t.Error("expected Write=false")
	}
}

func TestParsePackageJSON_MissingMain(t *testing.T) {
	raw := []byte(`{"name":"app","dependencies":{}}`)

	_, err := parsePackageJSON(raw)
	if err == nil {
		t.Fatal("expected error for missing main field, got nil")
	}
}

// Task 6: hash and local disk lookup tests

func TestHashBunLock(t *testing.T) {
	lockContent := []byte("some lockfile content")
	hash := hashBunLock(lockContent)
	expected := sha256.Sum256(lockContent)
	expectedHex := hex.EncodeToString(expected[:])
	if hash != expectedHex {
		t.Fatalf("expected %s, got %s", expectedHex, hash)
	}
}

func TestResolveDeps_LocalHit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "deps-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	lockContent := []byte("test lockfile")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(tmpDir, "deps", hash, "node_modules", "zod")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "index.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := resolveDeps(context.Background(), lockContent, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPath := filepath.Join(tmpDir, "deps", hash)
	if path != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, path)
	}
}

// Task 7: S3 hit test

func TestResolveDeps_S3Hit(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)

	lockContent := []byte("s3 test lockfile")
	hash := hashBunLock(lockContent)

	// Create a tarball in mock S3
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	content := []byte("module.exports = {}")
	tw.WriteHeader(&tar.Header{
		Name: "node_modules/zod/index.js",
		Mode: 0644,
		Size: int64(len(content)),
	})
	tw.Write(content)
	tw.Close()
	gw.Close()

	mockS3.files["deps/"+hash+".tar.gz"] = buf.String()

	path, err := resolveDeps(context.Background(), lockContent, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	extracted, err := os.ReadFile(filepath.Join(path, "node_modules", "zod", "index.js"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(extracted) != "module.exports = {}" {
		t.Fatalf("unexpected content: %s", string(extracted))
	}
}

// Task 8: typecheckV3 tests

func TestTypecheckV3_PassingCode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "v3-typecheck-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	hash := hashBunLock([]byte("test-lock"))
	depDir := filepath.Join(tmpDir, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		"/src/index.ts": []byte("export const x: string = 'hello';"),
	}

	response := typecheckV3(files, nil, []byte("test-lock"))
	if !response.Pass {
		t.Fatalf("expected pass, got errors: %v", response.Errors)
	}
}

func TestTypecheckV3_TypeError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "v3-typecheck-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	hash := hashBunLock([]byte("test-lock"))
	depDir := filepath.Join(tmpDir, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		"/src/index.ts": []byte("export const x: string = 123;"),
	}

	response := typecheckV3(files, nil, []byte("test-lock"))
	if response.Pass {
		t.Fatal("expected type error, got pass")
	}
	if len(response.Errors) == 0 {
		t.Fatal("expected errors, got none")
	}
}

// Task 9: compileV3 and isExternal tests

func TestCompileV3_SimpleBundle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "v3-compile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	hash := hashBunLock([]byte("test-lock"))
	depDir := filepath.Join(tmpDir, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		"/src/index.ts": []byte("export const hello = 'world';"),
	}

	pkg := &v3PackageJSON{Main: "./src/index.ts"}

	response := compileV3(files, pkg, nil, []byte("test-lock"))
	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}
	if response.Code == "" {
		t.Fatal("expected compiled code, got empty string")
	}
	if !strings.Contains(response.Code, "hello") {
		t.Fatalf("expected output to contain 'hello', got: %s", response.Code)
	}
}

func TestIsExternal(t *testing.T) {
	tests := []struct {
		path      string
		externals []string
		expected  bool
	}{
		{"zod", []string{"*"}, true},
		{"zod", []string{"zod"}, true},
		{"zod/lib", []string{"zod"}, true},
		{"@scope/pkg", []string{"*"}, true},
		{"@scope/pkg", []string{"@scope/pkg"}, true},
		{"react", []string{"zod"}, false},
		{"./local", []string{"*"}, false},
		{"../parent", []string{"*"}, false},
		{"/absolute", []string{"*"}, false},
		{"zod", nil, false},
		{"zod", []string{}, false},
	}
	for _, tt := range tests {
		result := isExternal(tt.path, tt.externals)
		if result != tt.expected {
			t.Errorf("isExternal(%q, %v) = %v, want %v", tt.path, tt.externals, result, tt.expected)
		}
	}
}

// Task 10: V3 HTTP handler tests

func TestV3TypecheckHandler_Pass(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("test-lock-for-handler")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"main": "./src/index.ts", "dependencies": {}}`)
	writer.WriteField("/bun.lock", "test-lock-for-handler")
	writer.WriteField("/src/index.ts", "export const x: string = 'hello';")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result TypecheckV2Response
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Pass {
		t.Fatalf("expected pass, got errors: %v", result.Errors)
	}
}

func TestV3TypecheckHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v3/typecheck", nil)
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Result().StatusCode)
	}
}

func TestV3CompileHandler_Success(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("compile-handler-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"main": "./src/index.ts", "dependencies": {}}`)
	writer.WriteField("/bun.lock", "compile-handler-lock")
	writer.WriteField("/src/index.ts", "export const hello = 'world';")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result BuildV2Response
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.Code == "" {
		t.Fatal("expected compiled code")
	}
}

func TestDeleteOldestVersion_IncludesDeps(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "eviction-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	// Create a version dir (old)
	versionDir := filepath.Join(tmpDir, "5.7.0")
	os.MkdirAll(versionDir, 0755)
	oldTime := time.Now().Add(-2 * time.Hour)
	os.Chtimes(versionDir, oldTime, oldTime)

	// Create a deps dir (newer)
	depsDir := filepath.Join(tmpDir, "deps", "abc123")
	os.MkdirAll(depsDir, 0755)

	// deleteOldestVersion should delete the older version dir first
	deleted := deleteOldestVersion("")
	if !deleted {
		t.Fatal("expected a deletion")
	}

	// Version dir should be gone
	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Fatal("expected version dir to be deleted")
	}

	// Deps dir should still exist
	if _, err := os.Stat(depsDir); err != nil {
		t.Fatal("deps dir should still exist")
	}
}
