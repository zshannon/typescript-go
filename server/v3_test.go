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

// setupV3DepCache creates a pre-populated dependency cache with @crayonnow/core and react
// packages on disk, simulating a bun install cache hit. Returns the lock hash.
func setupV3DepCache(t *testing.T, lockContent string) string {
	t.Helper()
	hash := hashBunLock([]byte(lockContent))
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")

	// @crayonnow/core
	coreDir := filepath.Join(nmDir, "@crayonnow", "core")
	if err := os.MkdirAll(coreDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "package.json"), []byte(`{"name": "@crayonnow/core", "version": "1.0.0", "main": "index.js", "types": "index.d.ts", "exports": {".": {"types": "./index.d.ts", "default": "./index.js"}, "./jsx-runtime": {"types": "./jsx-runtime.d.ts", "default": "./jsx-runtime.js"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "index.d.ts"), []byte(`export interface FlexProps { style?: any; children?: any; }
export declare function Flex(props: FlexProps): any;
export interface ButtonProps { onClick?: () => void; style?: any; children?: any; }
export declare function Button(props: ButtonProps): any;
export interface TextProps { style?: any; children?: any; }
export declare function Text(props: TextProps): any;`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "index.js"), []byte(`exports.Flex = function(props) { return {type: 'Flex', props}; };
exports.Button = function(props) { return {type: 'Button', props}; };
exports.Text = function(props) { return {type: 'Text', props}; };`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "jsx-runtime.d.ts"), []byte(`export namespace JSX { interface Element {} interface IntrinsicElements { [key: string]: any; } }
export function jsx(type: any, props: any, key?: any): any;
export function jsxs(type: any, props: any, key?: any): any;
export function Fragment(props: any): any;`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "jsx-runtime.js"), []byte(`exports.jsx = function(type, props) { return {type, props}; };
exports.jsxs = exports.jsx;
exports.Fragment = function(props) { return props.children; };`), 0644); err != nil {
		t.Fatal(err)
	}

	// react
	reactDir := filepath.Join(nmDir, "react")
	if err := os.MkdirAll(reactDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reactDir, "package.json"), []byte(`{"name": "react", "version": "18.0.0", "main": "index.js", "types": "index.d.ts"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reactDir, "index.d.ts"), []byte(`export function useState<T>(init: T): [T, (value: T) => void];
export function useEffect(fn: () => void | (() => void), deps?: any[]): void;
export function createElement(type: any, props: any, children?: any): any;`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reactDir, "index.js"), []byte(`exports.useState = function(init) { return [init, function() {}]; };
exports.useEffect = function(fn, deps) { };
exports.createElement = function(type, props, children) { return {type, props, children}; };`), 0644); err != nil {
		t.Fatal(err)
	}

	return hash
}

// buildV3Multipart creates a multipart/form-data body and content-type for v3 handler tests.
func buildV3Multipart(files map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for path, content := range files {
		fw, _ := writer.CreateFormFile(path, path)
		fw.Write([]byte(content))
	}
	writer.Close()
	return body, writer.FormDataContentType()
}

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
	if err != nil {
		t.Fatalf("unexpected error: main is no longer required at parse time, got: %v", err)
	}
}

func TestParsePackageJSON_ResolveS3(t *testing.T) {
	raw := []byte(`{"main": "./src/index.ts", "dependencies": {"@flickfyi/core": "0.0.8"}, "resolve-s3": ["@flickfyi/core"]}`)
	pkg, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.ResolveS3) != 1 || pkg.ResolveS3[0] != "@flickfyi/core" {
		t.Fatalf("expected resolve-s3 [@flickfyi/core], got %v", pkg.ResolveS3)
	}
}

func TestParsePackageJSON_NoMainAllowed(t *testing.T) {
	raw := []byte(`{"dependencies": {"zod": "^3.23.0"}}`)
	_, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveS3Packages_VersionLookup(t *testing.T) {
	pkg := &v3PackageJSON{
		Dependencies: map[string]string{"@flickfyi/core": "0.0.8"},
	}
	version, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.0.8" {
		t.Fatalf("expected 0.0.8, got %s", version)
	}
}

func TestResolveS3Packages_NotInDeps(t *testing.T) {
	pkg := &v3PackageJSON{
		Dependencies: map[string]string{"zod": "3.23.0"},
	}
	_, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err == nil {
		t.Fatal("expected error for package not in deps, got nil")
	}
	if !strings.Contains(err.Error(), "not found in dependencies") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestResolveS3Packages_RangeVersion(t *testing.T) {
	pkg := &v3PackageJSON{
		Dependencies: map[string]string{"@flickfyi/core": "^0.0.8"},
	}
	_, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err == nil {
		t.Fatal("expected error for range version, got nil")
	}
	if !strings.Contains(err.Error(), "exact version") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestResolveS3Packages_DevDeps(t *testing.T) {
	pkg := &v3PackageJSON{
		DevDependencies: map[string]string{"@flickfyi/core": "0.0.8"},
	}
	version, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.0.8" {
		t.Fatalf("expected 0.0.8, got %s", version)
	}
}

func TestExtractNpmTarball(t *testing.T) {
	// Create a tarball with package/ prefix like npm produces
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add a directory entry
	tw.WriteHeader(&tar.Header{
		Name:     "package/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	})

	// Add a file inside package/
	content := []byte(`{"name": "@flickfyi/core", "version": "0.0.8"}`)
	tw.WriteHeader(&tar.Header{
		Name: "package/package.json",
		Mode: 0644,
		Size: int64(len(content)),
	})
	tw.Write(content)

	// Add a nested file
	jsContent := []byte(`module.exports = {}`)
	tw.WriteHeader(&tar.Header{
		Name: "package/dist/index.js",
		Mode: 0644,
		Size: int64(len(jsContent)),
	})
	tw.Write(jsContent)

	tw.Close()
	gw.Close()

	// Extract to temp dir
	destDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(destDir)

	if err := extractNpmTarball(&buf, destDir); err != nil {
		t.Fatalf("extractNpmTarball failed: %v", err)
	}

	// Verify package/ prefix was stripped
	pkgJSON, err := os.ReadFile(filepath.Join(destDir, "package.json"))
	if err != nil {
		t.Fatalf("package.json not found after extraction: %v", err)
	}
	if string(pkgJSON) != string(content) {
		t.Fatalf("unexpected package.json content: %s", string(pkgJSON))
	}

	jsFile, err := os.ReadFile(filepath.Join(destDir, "dist", "index.js"))
	if err != nil {
		t.Fatalf("dist/index.js not found after extraction: %v", err)
	}
	if string(jsFile) != string(jsContent) {
		t.Fatalf("unexpected dist/index.js content: %s", string(jsFile))
	}
}

func TestV3CompileHandler_MissingMain(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("missing-main-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}}`)
	writer.WriteField("/bun.lock", "missing-main-lock")
	writer.WriteField("/src/index.ts", "export const x = 1;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestV3TypecheckHandler_NoMainRequired(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("no-main-typecheck-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}}`)
	writer.WriteField("/bun.lock", "no-main-typecheck-lock")
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

	pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
	path, err := resolveDeps(context.Background(), lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
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

	pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
	path, err := resolveDeps(context.Background(), lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
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

// =============================================================================
// V3 End-to-End Integration Tests
// =============================================================================

// Test 1: Fortune Cookie — full JSX app with React + Crayon via v3 multipart API
func TestV3FortuneCookie(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "fortune-cookie-lock-v3"
	setupV3DepCache(t, lockContent)

	fortuneCookieCode := `import { Flex, Button, Text } from '@crayonnow/core';
import { useState } from 'react';

const fortunes = [
  "You will find great success.",
  "A fresh start awaits.",
  "Believe in yourself.",
];

export default () => {
  const [fortune, setFortune] = useState('');

  const getFortune = () => {
    const random = fortunes[Math.floor(Math.random() * fortunes.length)];
    setFortune(random);
  };

  return (
    <Flex style={{ alignItems: 'stretch', minHeight: '100vh' }}>
      <Text style={{ fontSize: '24px' }}>
        {fortune || 'Tap the cookie'}
      </Text>
      <Button onClick={getFortune} style={{ background: '#FFCC00' }}>
        <Text style={{ color: 'black' }}>Break Cookie</Text>
      </Button>
    </Flex>
  );
};`

	packageJSON := `{"name": "fortune-cookie", "main": "./src/index.tsx", "dependencies": {"@crayonnow/core": "1.0.0", "react": "18.0.0"}}`
	tsconfigJSON := `{"compilerOptions": {"jsx": "react-jsx", "jsxImportSource": "@crayonnow/core"}}`

	files := map[string]string{
		"/package.json":  packageJSON,
		"/bun.lock":      lockContent,
		"/tsconfig.json": tsconfigJSON,
		"/src/index.tsx": fortuneCookieCode,
	}

	t.Run("Typecheck", func(t *testing.T) {
		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
		req.Header.Set("Content-Type", ct)
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
			t.Fatalf("expected typecheck pass, got errors: %v", result.Errors)
		}
	})

	t.Run("Compile", func(t *testing.T) {
		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
		req.Header.Set("Content-Type", ct)
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
			t.Fatalf("unexpected compile errors: %v", result.Errors)
		}
		if result.Code == "" {
			t.Fatal("expected compiled code, got empty string")
		}
		if !strings.Contains(result.Code, "fortune") && !strings.Contains(result.Code, "Fortune") {
			t.Errorf("compiled code does not contain 'fortune' text")
		}
		if !strings.Contains(result.Code, "Break Cookie") {
			t.Errorf("compiled code does not contain 'Break Cookie' text")
		}
	})
}

// Test 2: Multi-file project with imports between files
func TestV3MultiFileImports(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "multi-file-imports-lock"
	setupV3DepCache(t, lockContent)

	files := map[string]string{
		"/package.json": `{"name": "multi-file", "main": "./src/index.ts", "dependencies": {}}`,
		"/bun.lock":     lockContent,
		"/src/types.ts": `export interface User {
  name: string;
  age: number;
}

export interface Greeting {
  message: string;
  user: User;
}`,
		"/src/utils.ts": `import { User, Greeting } from './types';

export function greet(user: User): Greeting {
  return {
    message: "Hello, " + user.name,
    user: user,
  };
}

export function isAdult(user: User): boolean {
  return user.age >= 18;
}`,
		"/src/index.ts": `import { User } from './types';
import { greet, isAdult } from './utils';

const user: User = { name: "Alice", age: 30 };
const greeting = greet(user);
const adult = isAdult(user);

export { greeting, adult };`,
	}

	t.Run("Typecheck", func(t *testing.T) {
		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
		req.Header.Set("Content-Type", ct)
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
			t.Fatalf("expected typecheck pass, got errors: %v", result.Errors)
		}
	})

	t.Run("Compile", func(t *testing.T) {
		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
		req.Header.Set("Content-Type", ct)
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
			t.Fatalf("unexpected compile errors: %v", result.Errors)
		}
		if result.Code == "" {
			t.Fatal("expected compiled code")
		}
		// Verify content from all files is bundled
		if !strings.Contains(result.Code, "Alice") {
			t.Errorf("compiled code missing content from index.ts")
		}
		if !strings.Contains(result.Code, "Hello") {
			t.Errorf("compiled code missing content from utils.ts")
		}
	})
}

// Test 3: Type error detection across files
func TestV3TypeErrorAcrossFiles(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "type-error-across-files-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{"name": "type-error-test", "main": "./src/index.ts", "dependencies": {}}`,
		"/bun.lock":     lockContent,
		"/src/types.ts": `export interface Config {
  port: number;
  host: string;
}`,
		"/src/index.ts": `import { Config } from './types';

const config: Config = {
  port: "not-a-number",
  host: "localhost",
};

export { config };`,
	}

	body, ct := buildV3Multipart(files)
	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result TypecheckV2Response
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Pass {
		t.Fatal("expected typecheck failure, got pass")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}

	// Verify the error references the correct file
	foundFileRef := false
	for _, e := range result.Errors {
		if strings.Contains(e.File, "index.ts") {
			foundFileRef = true
			if e.Line <= 0 {
				t.Errorf("expected positive line number, got %d", e.Line)
			}
		}
	}
	if !foundFileRef {
		t.Errorf("expected error to reference index.ts, got errors: %v", result.Errors)
	}
}

// Test 4: Compile with typecheck (default behavior — no skip_typecheck)
func TestV3CompileWithTypecheck(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "compile-with-typecheck-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{"name": "typecheck-fail", "main": "./src/index.ts", "dependencies": {}}`,
		"/bun.lock":     lockContent,
		"/src/index.ts": `export const x: string = 123;`,
	}

	body, ct := buildV3Multipart(files)
	// No skip_typecheck query param — typecheck should run by default
	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result BuildV2Response
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Errors) == 0 {
		t.Fatal("expected typecheck errors when compiling without skip_typecheck")
	}
	if result.Code != "" {
		t.Errorf("expected no compiled code when typecheck fails, got %d bytes", len(result.Code))
	}
}

// Test 5: Compile with skip_typecheck=true
func TestV3CompileSkipTypecheck(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "skip-typecheck-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{"name": "skip-typecheck", "main": "./src/index.ts", "dependencies": {}}`,
		"/bun.lock":     lockContent,
		"/src/index.ts": `export const x: string = 123;`,
	}

	body, ct := buildV3Multipart(files)
	req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
	req.Header.Set("Content-Type", ct)
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
		t.Fatalf("expected no errors with skip_typecheck=true, got: %v", result.Errors)
	}
	if result.Code == "" {
		t.Fatal("expected compiled code with skip_typecheck=true")
	}
}

// Test 6: Custom tsconfig — strict vs non-strict
func TestV3StrictVsNonStrict(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "strict-test-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	// This function has an implicit 'any' parameter — strict mode should reject it
	code := `export function foo(x) { return x; }`

	t.Run("StrictTrue", func(t *testing.T) {
		files := map[string]string{
			"/package.json":  `{"name": "strict-test", "main": "./src/index.ts", "dependencies": {}}`,
			"/bun.lock":      lockContent,
			"/tsconfig.json": `{"compilerOptions": {"strict": true}}`,
			"/src/index.ts":  code,
		}

		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()

		typecheckV3Handler(w, req)

		var result TypecheckV2Response
		json.NewDecoder(w.Result().Body).Decode(&result)
		if result.Pass {
			t.Fatal("expected strict mode to fail on implicit any parameter")
		}
		// Should mention 'any' in one of the errors
		foundAnyError := false
		for _, e := range result.Errors {
			if strings.Contains(e.Message, "any") {
				foundAnyError = true
				break
			}
		}
		if !foundAnyError {
			t.Errorf("expected error mentioning 'any', got: %v", result.Errors)
		}
	})

	t.Run("StrictFalse", func(t *testing.T) {
		files := map[string]string{
			"/package.json":  `{"name": "non-strict-test", "main": "./src/index.ts", "dependencies": {}}`,
			"/bun.lock":      lockContent,
			"/tsconfig.json": `{"compilerOptions": {"strict": false}}`,
			"/src/index.ts":  code,
		}

		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()

		typecheckV3Handler(w, req)

		var result TypecheckV2Response
		json.NewDecoder(w.Result().Body).Decode(&result)
		if !result.Pass {
			t.Fatalf("expected non-strict mode to pass, got errors: %v", result.Errors)
		}
	})
}

// Test 7: Custom esbuild config — ESM output
func TestV3EsbuildESMOutput(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "esm-output-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{"name": "esm-test", "main": "./src/index.ts", "dependencies": {}, "esbuild": {"format": "esm"}}`,
		"/bun.lock":     lockContent,
		"/src/index.ts": `export const greeting = "hello world";
export function greet(name: string): string {
  return greeting + " " + name;
}`,
	}

	body, ct := buildV3Multipart(files)
	req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
	req.Header.Set("Content-Type", ct)
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
	// ESM output should contain export statements
	if !strings.Contains(result.Code, "export") {
		t.Errorf("expected ESM output to contain 'export', got:\n%s", result.Code)
	}
	// Should NOT contain module.exports (that's CJS)
	if strings.Contains(result.Code, "module.exports") {
		t.Errorf("ESM output should not contain 'module.exports', got:\n%s", result.Code)
	}
}

// Test 8: Custom esbuild config — externals
func TestV3EsbuildExternals(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "externals-lock"
	setupV3DepCache(t, lockContent)

	// Also add a zod package to the dep cache so resolution doesn't fail at typecheck
	hash := hashBunLock([]byte(lockContent))
	zodDir := filepath.Join(diskCachePath, "deps", hash, "node_modules", "zod")
	if err := os.MkdirAll(zodDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zodDir, "package.json"), []byte(`{"name": "zod", "version": "3.23.0", "main": "index.js", "types": "index.d.ts"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zodDir, "index.js"), []byte(`exports.z = { string: function() { return {}; } };`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zodDir, "index.d.ts"), []byte(`export declare const z: any;`), 0644); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{"name": "externals-test", "main": "./src/index.ts", "dependencies": {"zod": "3.23.0"}, "esbuild": {"external": ["zod"]}}`,
		"/bun.lock":     lockContent,
		"/src/index.ts": `import { z } from 'zod';
export const schema = z;`,
	}

	body, ct := buildV3Multipart(files)
	req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
	req.Header.Set("Content-Type", ct)
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
	// The import should be preserved as external (require("zod"))
	if !strings.Contains(result.Code, "require(\"zod\")") {
		t.Errorf("expected external import to produce require(\"zod\"), got:\n%s", result.Code)
	}
}

// Test 9: Bad request — missing package.json
func TestV3BadRequest_MissingPackageJSON(t *testing.T) {
	setupTestServerWithMockS3(t)

	files := map[string]string{
		"/bun.lock":     "some-lock",
		"/src/index.ts": "export const x = 1;",
	}

	body, ct := buildV3Multipart(files)
	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Result().StatusCode)
	}
}

// Test 10: Bad request — invalid JSON in package.json
func TestV3BadRequest_InvalidPackageJSON(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "invalid-pkg-json-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{this is not valid json!!!`,
		"/bun.lock":     lockContent,
		"/src/index.ts": "export const x = 1;",
	}

	body, ct := buildV3Multipart(files)
	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid package.json, got %d", w.Result().StatusCode)
	}
}

// =============================================================================
// resolve-s3 Integration Tests
// =============================================================================

func TestV3TypecheckHandler_WithS3Packages(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("s3-pkg-typecheck-lock")
	hash := hashBunLock(lockContent)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	coreDir := filepath.Join(depBase, "node_modules", "@flickfyi", "core")
	os.MkdirAll(coreDir, 0755)
	os.WriteFile(filepath.Join(coreDir, "package.json"), []byte(`{"name": "@flickfyi/core", "version": "0.0.8", "main": "index.js", "types": "index.d.ts"}`), 0644)
	os.WriteFile(filepath.Join(coreDir, "index.d.ts"), []byte(`export declare function Flex(props: any): any;`), 0644)
	os.WriteFile(filepath.Join(coreDir, "index.js"), []byte(`exports.Flex = function() {};`), 0644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {"@flickfyi/core": "0.0.8"}, "resolve-s3": ["@flickfyi/core"]}`)
	writer.WriteField("/bun.lock", "s3-pkg-typecheck-lock")
	writer.WriteField("/src/index.ts", "import { Flex } from '@flickfyi/core';\nexport const f = Flex;")
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

func TestV3_ResolveS3_BadVersion(t *testing.T) {
	setupTestServerWithMockS3(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {"@flickfyi/core": "^0.0.8"}, "resolve-s3": ["@flickfyi/core"]}`)
	writer.WriteField("/bun.lock", "bad-version-lock")
	writer.WriteField("/src/index.ts", "import { Flex } from '@flickfyi/core';\nexport const f = Flex;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusBadGateway {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 502, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}

func TestV3_ResolveS3_NotInDeps(t *testing.T) {
	setupTestServerWithMockS3(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}, "resolve-s3": ["@flickfyi/core"]}`)
	writer.WriteField("/bun.lock", "not-in-deps-lock")
	writer.WriteField("/src/index.ts", "export const x = 1;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusBadGateway {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 502, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}

func TestV3_ResolveS3_EmptyList(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("empty-resolve-s3-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	os.MkdirAll(depDir, 0755)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}, "resolve-s3": []}`)
	writer.WriteField("/bun.lock", "empty-resolve-s3-lock")
	writer.WriteField("/src/index.ts", "export const x: string = 'hello';")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}

// Test 11: Bad request — GET instead of POST
func TestV3BadRequest_MethodNotAllowed(t *testing.T) {
	t.Run("TypecheckGET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v3/typecheck", nil)
		w := httptest.NewRecorder()
		typecheckV3Handler(w, req)

		if w.Result().StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for GET /v3/typecheck, got %d", w.Result().StatusCode)
		}
	})

	t.Run("CompileGET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v3/compile", nil)
		w := httptest.NewRecorder()
		compileV3Handler(w, req)

		if w.Result().StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for GET /v3/compile, got %d", w.Result().StatusCode)
		}
	})
}

// Test 12: Dep cache hit verification — two requests with same bun.lock
func TestV3DepCacheHit(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "cache-hit-lock"
	setupV3DepCache(t, lockContent)

	files := map[string]string{
		"/package.json": `{"name": "cache-hit-test", "main": "./src/index.ts", "dependencies": {}}`,
		"/bun.lock":     lockContent,
		"/src/index.ts": `export const x: string = "first";`,
	}

	// First request
	body1, ct1 := buildV3Multipart(files)
	req1 := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body1)
	req1.Header.Set("Content-Type", ct1)
	w1 := httptest.NewRecorder()
	typecheckV3Handler(w1, req1)

	if w1.Result().StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(w1.Result().Body)
		t.Fatalf("first request: expected 200, got %d: %s", w1.Result().StatusCode, string(respBody))
	}

	var result1 TypecheckV2Response
	json.NewDecoder(w1.Result().Body).Decode(&result1)
	if !result1.Pass {
		t.Fatalf("first request: expected pass, got errors: %v", result1.Errors)
	}

	// Second request with same lock content — should reuse cache
	files["/src/index.ts"] = `export const y: number = 42;`
	body2, ct2 := buildV3Multipart(files)
	req2 := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body2)
	req2.Header.Set("Content-Type", ct2)
	w2 := httptest.NewRecorder()
	typecheckV3Handler(w2, req2)

	if w2.Result().StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(w2.Result().Body)
		t.Fatalf("second request: expected 200, got %d: %s", w2.Result().StatusCode, string(respBody))
	}

	var result2 TypecheckV2Response
	json.NewDecoder(w2.Result().Body).Decode(&result2)
	if !result2.Pass {
		t.Fatalf("second request: expected pass, got errors: %v", result2.Errors)
	}
}

// Test 13: Multi-file compile with relative imports (../lib pattern)
func TestV3RelativeImports(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := "relative-imports-lock"
	hash := hashBunLock([]byte(lockContent))
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"/package.json": `{"name": "relative-imports", "main": "./src/index.ts", "dependencies": {}}`,
		"/bun.lock":     lockContent,
		"/shared/math.ts": `export function add(a: number, b: number): number {
  return a + b;
}

export function multiply(a: number, b: number): number {
  return a * b;
}`,
		"/shared/strings.ts": `export function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}`,
		"/src/helpers.ts": `import { add } from '../shared/math';
import { capitalize } from '../shared/strings';

export function formatResult(a: number, b: number): string {
  return capitalize("sum: " + add(a, b).toString());
}`,
		"/src/index.ts": `import { formatResult } from './helpers';
import { multiply } from '../shared/math';

const result = formatResult(2, 3);
const product = multiply(4, 5);
export { result, product };`,
	}

	t.Run("Typecheck", func(t *testing.T) {
		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()

		typecheckV3Handler(w, req)

		var result TypecheckV2Response
		json.NewDecoder(w.Result().Body).Decode(&result)
		if !result.Pass {
			t.Fatalf("expected typecheck pass, got errors: %v", result.Errors)
		}
	})

	t.Run("Compile", func(t *testing.T) {
		body, ct := buildV3Multipart(files)
		req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()

		compileV3Handler(w, req)

		var result BuildV2Response
		json.NewDecoder(w.Result().Body).Decode(&result)
		if len(result.Errors) > 0 {
			t.Fatalf("unexpected compile errors: %v", result.Errors)
		}
		if result.Code == "" {
			t.Fatal("expected compiled code")
		}
		// Verify content from nested files is included
		if !strings.Contains(result.Code, "capitalize") || !strings.Contains(result.Code, "sum") {
			t.Errorf("compiled code missing content from nested relative imports")
		}
	})
}
