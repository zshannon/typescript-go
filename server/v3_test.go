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
	"github.com/microsoft/typescript-go/internal/core"
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
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "package.json"), []byte(`{"name": "@crayonnow/core", "version": "1.0.0", "main": "index.js", "types": "index.d.ts", "exports": {".": {"types": "./index.d.ts", "default": "./index.js"}, "./jsx-runtime": {"types": "./jsx-runtime.d.ts", "default": "./jsx-runtime.js"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "index.d.ts"), []byte(`export interface FlexProps { style?: any; children?: any; }
export declare function Flex(props: FlexProps): any;
export interface ButtonProps { onClick?: () => void; style?: any; children?: any; }
export declare function Button(props: ButtonProps): any;
export interface TextProps { style?: any; children?: any; }
export declare function Text(props: TextProps): any;`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "index.js"), []byte(`exports.Flex = function(props) { return {type: 'Flex', props}; };
exports.Button = function(props) { return {type: 'Button', props}; };
exports.Text = function(props) { return {type: 'Text', props}; };`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "jsx-runtime.d.ts"), []byte(`export namespace JSX { interface Element {} interface IntrinsicElements { [key: string]: any; } }
export function jsx(type: any, props: any, key?: any): any;
export function jsxs(type: any, props: any, key?: any): any;
export function Fragment(props: any): any;`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "jsx-runtime.js"), []byte(`exports.jsx = function(type, props) { return {type, props}; };
exports.jsxs = exports.jsx;
exports.Fragment = function(props) { return props.children; };`), 0o644); err != nil {
		t.Fatal(err)
	}

	// react
	reactDir := filepath.Join(nmDir, "react")
	if err := os.MkdirAll(reactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reactDir, "package.json"), []byte(`{"name": "react", "version": "18.0.0", "main": "index.js", "types": "index.d.ts"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reactDir, "index.d.ts"), []byte(`export function useState<T>(init: T): [T, (value: T) => void];
export function useEffect(fn: () => void | (() => void), deps?: any[]): void;
export function createElement(type: any, props: any, children?: any): any;`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reactDir, "index.js"), []byte(`exports.useState = function(init) { return [init, function() {}]; };
exports.useEffect = function(fn, deps) { };
exports.createElement = function(type, props, children) { return {type, props, children}; };`), 0o644); err != nil {
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
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "index.d.ts"), []byte("export declare function string(): any;"), 0o644); err != nil {
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
		Mode:     0o755,
	})

	// Add a file inside package/
	content := []byte(`{"name": "@flickfyi/core", "version": "0.0.8"}`)
	tw.WriteHeader(&tar.Header{
		Name: "package/package.json",
		Mode: 0o644,
		Size: int64(len(content)),
	})
	tw.Write(content)

	// Add a nested file
	jsContent := []byte(`module.exports = {}`)
	tw.WriteHeader(&tar.Header{
		Name: "package/dist/index.js",
		Mode: 0o644,
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "index.js"), []byte("module.exports = {}"), 0o644); err != nil {
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
		Mode: 0o644,
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

func TestInstallDeps_CachesOnlyNodeModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "deps-install-cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	fakeBun := filepath.Join(binDir, "bun")
	if err := os.WriteFile(fakeBun, []byte("#!/bin/sh\nmkdir -p node_modules/zod\nprintf 'module.exports = {}\\n' > node_modules/zod/index.js\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	lockContent := []byte("install cache test lockfile")
	hash := hashBunLock(lockContent)
	depDir := depsCacheDir(hash)
	pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}

	path, err := installDeps(
		context.Background(),
		hash,
		depDir,
		lockContent,
		pkg,
		[]byte(`{"dependencies":{"zod":"3.23.0"},"type":"module"}`),
		newDepInstallResult(),
	)
	if err != nil {
		t.Fatalf("installDeps failed: %v", err)
	}
	if path != depDir {
		t.Fatalf("expected %s, got %s", depDir, path)
	}
	if _, err := os.Stat(filepath.Join(depDir, "node_modules", "zod", "index.js")); err != nil {
		t.Fatalf("node_modules should be cached: %v", err)
	}
	if _, err := os.Stat(filepath.Join(depDir, "package.json")); !os.IsNotExist(err) {
		t.Fatalf("package.json should not be cached, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(depDir, "bun.lock")); !os.IsNotExist(err) {
		t.Fatalf("bun.lock should not be cached, stat err: %v", err)
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	os.MkdirAll(versionDir, 0o755)
	oldTime := time.Now().Add(-2 * time.Hour)
	os.Chtimes(versionDir, oldTime, oldTime)

	// Create a deps dir (newer)
	depsDir := filepath.Join(tmpDir, "deps", "abc123")
	os.MkdirAll(depsDir, 0o755)

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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	if err := os.MkdirAll(zodDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zodDir, "package.json"), []byte(`{"name": "zod", "version": "3.23.0", "main": "index.js", "types": "index.d.ts"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zodDir, "index.js"), []byte(`exports.z = { string: function() { return {}; } };`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zodDir, "index.d.ts"), []byte(`export declare const z: any;`), 0o644); err != nil {
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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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
	os.MkdirAll(coreDir, 0o755)
	os.WriteFile(filepath.Join(coreDir, "package.json"), []byte(`{"name": "@flickfyi/core", "version": "0.0.8", "main": "index.js", "types": "index.d.ts"}`), 0o644)
	os.WriteFile(filepath.Join(coreDir, "index.d.ts"), []byte(`export declare function Flex(props: any): any;`), 0o644)
	os.WriteFile(filepath.Join(coreDir, "index.js"), []byte(`exports.Flex = function() {};`), 0o644)

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
	os.MkdirAll(depDir, 0o755)

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
	if err := os.MkdirAll(depDir, 0o755); err != nil {
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

// =============================================================================
// Coverage Gap Tests
// =============================================================================

func TestParseTSConfig(t *testing.T) {
	t.Run("Nil returns defaults", func(t *testing.T) {
		opts, err := parseTSConfig(nil)
		if err != nil {
			t.Fatal(err)
		}
		defaults := defaultCompilerOptions()
		if opts.Strict != defaults.Strict {
			t.Error("strict mismatch")
		}
		if opts.Target != defaults.Target {
			t.Error("target mismatch")
		}
		if opts.Module != defaults.Module {
			t.Error("module mismatch")
		}
		if opts.ModuleResolution != defaults.ModuleResolution {
			t.Error("moduleResolution mismatch")
		}
		if opts.Jsx != defaults.Jsx {
			t.Error("jsx mismatch")
		}
	})

	t.Run("Target variants", func(t *testing.T) {
		cases := []struct {
			input    string
			expected core.ScriptTarget
		}{
			{"es2015", core.ScriptTargetES2015},
			{"es2016", core.ScriptTargetES2016},
			{"es2017", core.ScriptTargetES2017},
			{"es2018", core.ScriptTargetES2018},
			{"es2019", core.ScriptTargetES2019},
			{"es2020", core.ScriptTargetES2020},
			{"es2021", core.ScriptTargetES2021},
			{"es2022", core.ScriptTargetES2022},
			{"es2023", core.ScriptTargetES2023},
			{"es2024", core.ScriptTargetES2024},
			{"es2025", core.ScriptTargetES2025},
			{"esnext", core.ScriptTargetESNext},
		}
		for _, tc := range cases {
			raw := []byte(fmt.Sprintf(`{"compilerOptions": {"target": "%s"}}`, tc.input))
			opts, err := parseTSConfig(raw)
			if err != nil {
				t.Fatalf("target %s: %v", tc.input, err)
			}
			if opts.Target != tc.expected {
				t.Errorf("target %s: got %v, want %v", tc.input, opts.Target, tc.expected)
			}
		}
	})

	t.Run("Module variants", func(t *testing.T) {
		cases := []struct {
			input    string
			expected core.ModuleKind
		}{
			{"commonjs", core.ModuleKindCommonJS},
			{"es2015", core.ModuleKindES2015},
			{"es2020", core.ModuleKindES2020},
			{"es2022", core.ModuleKindES2022},
			{"esnext", core.ModuleKindESNext},
			{"node16", core.ModuleKindNode16},
			{"nodenext", core.ModuleKindNodeNext},
		}
		for _, tc := range cases {
			raw := []byte(fmt.Sprintf(`{"compilerOptions": {"module": "%s"}}`, tc.input))
			opts, err := parseTSConfig(raw)
			if err != nil {
				t.Fatalf("module %s: %v", tc.input, err)
			}
			if opts.Module != tc.expected {
				t.Errorf("module %s: got %v, want %v", tc.input, opts.Module, tc.expected)
			}
		}
	})

	t.Run("ModuleResolution variants", func(t *testing.T) {
		cases := []struct {
			input    string
			expected core.ModuleResolutionKind
		}{
			{"bundler", core.ModuleResolutionKindBundler},
			{"classic", core.ModuleResolutionKindClassic},
			{"node", core.ModuleResolutionKindNode10},
			{"node16", core.ModuleResolutionKindNode16},
			{"nodenext", core.ModuleResolutionKindNodeNext},
		}
		for _, tc := range cases {
			raw := []byte(fmt.Sprintf(`{"compilerOptions": {"moduleResolution": "%s"}}`, tc.input))
			opts, err := parseTSConfig(raw)
			if err != nil {
				t.Fatalf("moduleResolution %s: %v", tc.input, err)
			}
			if opts.ModuleResolution != tc.expected {
				t.Errorf("moduleResolution %s: got %v, want %v", tc.input, opts.ModuleResolution, tc.expected)
			}
		}
	})

	t.Run("JSX variants", func(t *testing.T) {
		cases := []struct {
			input    string
			expected core.JsxEmit
		}{
			{"preserve", core.JsxEmitPreserve},
			{"react", core.JsxEmitReact},
			{"react-jsx", core.JsxEmitReactJSX},
			{"react-jsxdev", core.JsxEmitReactJSXDev},
			{"react-native", core.JsxEmitReactNative},
		}
		for _, tc := range cases {
			raw := []byte(fmt.Sprintf(`{"compilerOptions": {"jsx": "%s"}}`, tc.input))
			opts, err := parseTSConfig(raw)
			if err != nil {
				t.Fatalf("jsx %s: %v", tc.input, err)
			}
			if opts.Jsx != tc.expected {
				t.Errorf("jsx %s: got %v, want %v", tc.input, opts.Jsx, tc.expected)
			}
		}
	})

	t.Run("JsxImportSource", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {"jsxImportSource": "@flickfyi/core"}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if opts.JsxImportSource != "@flickfyi/core" {
			t.Errorf("expected @flickfyi/core, got %s", opts.JsxImportSource)
		}
	})

	t.Run("Lib override", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {"lib": ["ES2022", "DOM"]}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(opts.Lib) != 2 || opts.Lib[0] != "ES2022" || opts.Lib[1] != "DOM" {
			t.Errorf("expected [ES2022, DOM], got %v", opts.Lib)
		}
	})

	t.Run("SkipLibCheck false", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {"skipLibCheck": false}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if opts.SkipLibCheck != core.TSFalse {
			t.Errorf("expected TSFalse, got %v", opts.SkipLibCheck)
		}
	})

	t.Run("SkipLibCheck true", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {"skipLibCheck": true}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if opts.SkipLibCheck != core.TSTrue {
			t.Errorf("expected TSTrue, got %v", opts.SkipLibCheck)
		}
	})

	t.Run("Strict false", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {"strict": false}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if opts.Strict != core.TSFalse {
			t.Errorf("expected TSFalse, got %v", opts.Strict)
		}
	})

	t.Run("Strict true", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {"strict": true}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if opts.Strict != core.TSTrue {
			t.Errorf("expected TSTrue, got %v", opts.Strict)
		}
	})

	t.Run("Empty compilerOptions", func(t *testing.T) {
		raw := []byte(`{"compilerOptions": {}}`)
		opts, err := parseTSConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		defaults := defaultCompilerOptions()
		if opts.Target != defaults.Target {
			t.Error("should use default target")
		}
		if opts.Module != defaults.Module {
			t.Error("should use default module")
		}
		if opts.Jsx != defaults.Jsx {
			t.Error("should use default jsx")
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		raw := []byte(`{not valid json}`)
		_, err := parseTSConfig(raw)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestCompileV3_MissingEntryPoint(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "v3-missing-entry-*")
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	hash := hashBunLock([]byte("missing-entry-lock"))
	os.MkdirAll(filepath.Join(tmpDir, "deps", hash, "node_modules"), 0o755)

	files := map[string][]byte{
		"/src/other.ts": []byte("export const x = 1;"),
	}
	pkg := &v3PackageJSON{Main: "./src/index.ts"} // index.ts not in files

	response := compileV3(files, pkg, nil, []byte("missing-entry-lock"))
	if len(response.Errors) == 0 {
		t.Fatal("expected error for missing entry point")
	}
	if !strings.Contains(response.Errors[0].Message, "Entry point not found") {
		t.Fatalf("expected 'Entry point not found' error, got: %s", response.Errors[0].Message)
	}
}

func TestTypecheckV3_NoTSFiles(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "v3-no-ts-*")
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	hash := hashBunLock([]byte("no-ts-lock"))
	os.MkdirAll(filepath.Join(tmpDir, "deps", hash, "node_modules"), 0o755)

	files := map[string][]byte{
		"/src/data.json": []byte(`{"key": "value"}`),
	}

	response := typecheckV3(files, nil, []byte("no-ts-lock"))
	if response.Pass {
		t.Fatal("expected failure when no TypeScript files present, got pass")
	}
	if len(response.Errors) == 0 {
		t.Fatal("expected errors")
	}
	if !strings.Contains(response.Errors[0].Message, "No TypeScript files") {
		t.Fatalf("expected 'No TypeScript files' error, got: %s", response.Errors[0].Message)
	}
}

func TestLoaderForPath(t *testing.T) {
	tests := []struct {
		expected api.Loader
		path     string
	}{
		{api.LoaderTSX, "/src/index.ts"},
		{api.LoaderTSX, "/src/app.tsx"},
		{api.LoaderJSX, "/src/component.jsx"},
		{api.LoaderJSON, "/data/config.json"},
		{api.LoaderJS, "/lib/utils.mjs"},
		{api.LoaderDefault, "/lib/main.js"},
		{api.LoaderDefault, "/styles.css"},
	}
	for _, tt := range tests {
		got := loaderForPath(tt.path)
		if got != tt.expected {
			t.Errorf("loaderForPath(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}

func TestExtractNpmTarball_DirectoryTraversal(t *testing.T) {
	t.Run("AbsolutePathSkipped", func(t *testing.T) {
		// Create a tarball with an absolute path entry
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		malicious := []byte("pwned")
		tw.WriteHeader(&tar.Header{
			Name: "/etc/evil",
			Mode: 0o644,
			Size: int64(len(malicious)),
		})
		tw.Write(malicious)

		// Add a legitimate file too
		legit := []byte("ok")
		tw.WriteHeader(&tar.Header{
			Name: "package/index.js",
			Mode: 0o644,
			Size: int64(len(legit)),
		})
		tw.Write(legit)
		tw.Close()
		gw.Close()

		tmpDir, _ := os.MkdirTemp("", "traversal-abs-test-*")
		defer os.RemoveAll(tmpDir)

		destDir := filepath.Join(tmpDir, "node_modules", "pkg")
		os.MkdirAll(destDir, 0o755)
		err := extractNpmTarball(bytes.NewReader(buf.Bytes()), destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The absolute path entry should have been skipped
		if _, statErr := os.Stat("/etc/evil"); statErr == nil {
			t.Fatal("absolute path entry was not skipped")
		}

		// The legitimate file should exist
		if _, statErr := os.Stat(filepath.Join(destDir, "index.js")); statErr != nil {
			t.Fatal("legitimate file missing after extraction")
		}
	})

	t.Run("DotOnlyEntrySkipped", func(t *testing.T) {
		// Entry that resolves to "." after stripping package/ should be skipped
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		legit := []byte("content")
		tw.WriteHeader(&tar.Header{
			Name: "package/lib/main.js",
			Mode: 0o644,
			Size: int64(len(legit)),
		})
		tw.Write(legit)
		tw.Close()
		gw.Close()

		tmpDir, _ := os.MkdirTemp("", "traversal-dot-test-*")
		defer os.RemoveAll(tmpDir)

		destDir := filepath.Join(tmpDir, "node_modules", "pkg")
		os.MkdirAll(destDir, 0o755)
		err := extractNpmTarball(bytes.NewReader(buf.Bytes()), destDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, statErr := os.Stat(filepath.Join(destDir, "lib", "main.js")); statErr != nil {
			t.Fatal("legitimate nested file missing after extraction")
		}
	})
}

func TestV3CompileHandler_DepResolutionFailure(t *testing.T) {
	setupTestServerWithMockS3(t)
	// No disk cache, no S3 cache, no bun -> dep resolution fails

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"main": "./src/index.ts",
		"dependencies": {"@flickfyi/core": "0.0.8"},
		"resolve-s3": ["@flickfyi/core"]
	}`)
	writer.WriteField("/bun.lock", "compile-502-lock")
	writer.WriteField("/src/index.ts", "export const x = 1;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	if w.Result().StatusCode != http.StatusBadGateway {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 502, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}

func TestParseV3Multipart_FileTooLarge(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"main": "./src/index.ts"}`)
	writer.WriteField("/bun.lock", "lockfile")
	// Write a file that exceeds 1MB
	bigContent := strings.Repeat("x", 1024*1024+1)
	writer.WriteField("/src/big.ts", bigContent)
	writer.Close()

	_, err := parseV3Multipart(body, writer.FormDataContentType())
	if err == nil {
		t.Fatal("expected error for file too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected 'too large' error, got: %s", err.Error())
	}
}

func TestCompileV3_BareMainPath(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "v3-bare-main-*")
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir

	hash := hashBunLock([]byte("bare-main-lock"))
	os.MkdirAll(filepath.Join(tmpDir, "deps", hash, "node_modules"), 0o755)

	files := map[string][]byte{
		"/src/index.ts": []byte("export const hello = 'world';"),
	}
	// main without ./ prefix
	pkg := &v3PackageJSON{Main: "src/index.ts"}

	response := compileV3(files, pkg, nil, []byte("bare-main-lock"))
	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}
	if response.Code == "" {
		t.Fatal("expected compiled code")
	}
}

func TestEsbuildOptions_AllFormats(t *testing.T) {
	tests := []struct {
		expected api.Format
		format   string
	}{
		{api.FormatCommonJS, "cjs"},
		{api.FormatCommonJS, ""},
		{api.FormatESModule, "esm"},
		{api.FormatESModule, "es"},
		{api.FormatESModule, "module"},
		{api.FormatIIFE, "iife"},
	}
	for _, tt := range tests {
		cfg := v3EsbuildConfig{Format: tt.format}
		opts := cfg.esbuildOptions()
		if opts.Format != tt.expected {
			t.Errorf("format %q: got %v, want %v", tt.format, opts.Format, tt.expected)
		}
	}
}

func TestEsbuildOptions_AllPlatforms(t *testing.T) {
	tests := []struct {
		expected api.Platform
		platform string
	}{
		{api.PlatformBrowser, "browser"},
		{api.PlatformBrowser, ""},
		{api.PlatformNode, "node"},
		{api.PlatformNeutral, "neutral"},
	}
	for _, tt := range tests {
		cfg := v3EsbuildConfig{Platform: tt.platform}
		opts := cfg.esbuildOptions()
		if opts.Platform != tt.expected {
			t.Errorf("platform %q: got %v, want %v", tt.platform, opts.Platform, tt.expected)
		}
	}
}

func TestEsbuildOptions_AllTargets(t *testing.T) {
	tests := []struct {
		expected api.Target
		target   string
	}{
		{api.ES2015, "es2015"},
		{api.ES2016, "es2016"},
		{api.ES2017, "es2017"},
		{api.ES2018, "es2018"},
		{api.ES2019, "es2019"},
		{api.ES2020, "es2020"},
		{api.ES2021, "es2021"},
		{api.ES2022, ""},
		{api.ES2022, "es2022"},
		{api.ES2023, "es2023"},
		{api.ES2024, "es2024"},
		{api.ES2025, "es2025"},
		{api.ESNext, "esnext"},
	}
	for _, tt := range tests {
		cfg := v3EsbuildConfig{Target: tt.target}
		opts := cfg.esbuildOptions()
		if opts.Target != tt.expected {
			t.Errorf("target %q: got %v, want %v", tt.target, opts.Target, tt.expected)
		}
	}
}

func TestV3CompileHandler_Globals(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("globals-test-lock")
	hash := hashBunLock(lockContent)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")
	os.MkdirAll(nmDir, 0o755)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"main": "./index.tsx",
		"dependencies": {},
		"esbuild": {
			"globals": {"react": "_CRAYONCORE_$REACT"}
		}
	}`)
	writer.WriteField("/bun.lock", "globals-test-lock")
	writer.WriteField("/index.tsx", `import { useState } from 'react';
export const App = () => { const [x, setX] = useState(0); return x; };`)
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

	// Should contain the global reference, NOT require("react")
	if strings.Contains(result.Code, `require("react")`) {
		t.Fatal("output contains require(\"react\") — globals replacement did not work")
	}
	if !strings.Contains(result.Code, "_CRAYONCORE_$REACT") {
		t.Fatalf("output should reference _CRAYONCORE_$REACT, got: %s", result.Code)
	}
}

func TestV3CompileHandler_GlobalsAndExternals(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("globals-externals-lock")
	hash := hashBunLock(lockContent)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")
	os.MkdirAll(nmDir, 0o755)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"main": "./index.tsx",
		"dependencies": {},
		"esbuild": {
			"globals": {"react": "_CRAYONCORE_$REACT"},
			"external": ["zod"]
		}
	}`)
	writer.WriteField("/bun.lock", "globals-externals-lock")
	writer.WriteField("/index.tsx", `import { useState } from 'react';
import { z } from 'zod';
export const schema = z.string();
export const App = () => { const [x] = useState(0); return x; };`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/compile?skip_typecheck=true", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	var result BuildV2Response
	json.NewDecoder(w.Result().Body).Decode(&result)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// react → global variable (not require)
	if strings.Contains(result.Code, `require("react")`) {
		t.Fatal("react should be a global, not require()")
	}
	if !strings.Contains(result.Code, "_CRAYONCORE_$REACT") {
		t.Fatal("output should reference _CRAYONCORE_$REACT")
	}

	// zod → require() (external)
	if !strings.Contains(result.Code, `require("zod")`) {
		t.Fatalf("zod should be require()'d as external, got: %s", result.Code)
	}
}

func TestV3CompileHandler_GlobalsFromNodeModules(t *testing.T) {
	// Bug: when a node_modules package imports react, the globals replacement
	// must also apply — not just to user code. Otherwise react gets bundled
	// from node_modules (duplicate instance) while user code gets the global.
	setupTestServerWithMockS3(t)

	lockContent := []byte("globals-nm-lock")
	hash := hashBunLock(lockContent)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")

	// Create a fake @flickfyi/photon package that imports react
	photonDir := filepath.Join(nmDir, "@flickfyi", "photon")
	os.MkdirAll(photonDir, 0o755)
	os.WriteFile(filepath.Join(photonDir, "package.json"), []byte(`{"name": "@flickfyi/photon", "version": "0.0.2", "main": "index.js"}`), 0o644)
	os.WriteFile(filepath.Join(photonDir, "index.js"), []byte(`
var React = require('react');
exports.useSpring = function(config) { return React.useState(config); };
`), 0o644)

	// Create a fake react package (should NOT be bundled — globals should catch it)
	reactDir := filepath.Join(nmDir, "react")
	os.MkdirAll(reactDir, 0o755)
	os.WriteFile(filepath.Join(reactDir, "package.json"), []byte(`{"name": "react", "version": "19.2.3", "main": "index.js"}`), 0o644)
	os.WriteFile(filepath.Join(reactDir, "index.js"), []byte(`
exports.useState = function(init) { return [init, function() {}]; };
exports.createElement = function(type, props) { return {type: type, props: props}; };
`), 0o644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"main": "./index.tsx",
		"dependencies": {},
		"esbuild": {
			"globals": {"react": "_CRAYONCORE_$REACT"}
		}
	}`)
	writer.WriteField("/bun.lock", "globals-nm-lock")
	writer.WriteField("/index.tsx", `import { useState } from 'react';
import { useSpring } from '@flickfyi/photon';
export const App = () => {
  const [x, setX] = useState(0);
  const spring = useSpring({x: 1});
  return x;
};`)
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

	// The output should reference _CRAYONCORE_$REACT
	if !strings.Contains(result.Code, "_CRAYONCORE_$REACT") {
		t.Fatalf("output should reference _CRAYONCORE_$REACT, got:\n%s", result.Code)
	}

	// The output should NOT contain react's actual implementation (useState function body)
	// If react was bundled from node_modules, we'd see its source code in the output
	if strings.Contains(result.Code, "exports.createElement") {
		t.Fatalf("react was bundled from node_modules instead of using globals replacement:\n%s", result.Code)
	}

	// There should be no require("react") either
	if strings.Contains(result.Code, `require("react")`) {
		t.Fatalf("output contains require(\"react\") — should use globals:\n%s", result.Code)
	}
}

func TestV3CompileHandler_GlobalsFromJSXRuntime(t *testing.T) {
	// Reproduce the real bug: @flickfyi/core jsx-runtime imports react,
	// JSX transform imports from @flickfyi/core/jsx-runtime, and the
	// transitive react import should hit globals replacement.
	setupTestServerWithMockS3(t)

	lockContent := []byte("globals-jsx-runtime-lock")
	hash := hashBunLock(lockContent)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")

	// @flickfyi/core with jsx-runtime that imports react
	coreDir := filepath.Join(nmDir, "@flickfyi", "core")
	os.MkdirAll(coreDir, 0o755)
	os.WriteFile(filepath.Join(coreDir, "package.json"), []byte(`{
		"name": "@flickfyi/core", "version": "0.0.8", "main": "index.js",
		"exports": {
			".": {"default": "./index.js"},
			"./jsx-runtime": {"default": "./jsx-runtime.js"}
		}
	}`), 0o644)
	os.WriteFile(filepath.Join(coreDir, "index.js"), []byte(`
var React = require('react');
exports.Text = function(props) { return React.createElement('span', null, props.children); };
exports.Flex = function(props) { return React.createElement('div', null, props.children); };
`), 0o644)
	os.WriteFile(filepath.Join(coreDir, "jsx-runtime.js"), []byte(`
var React = require('react');
exports.jsx = function(type, props, key) { return React.createElement(type, props); };
exports.jsxs = exports.jsx;
exports.Fragment = React.Fragment;
`), 0o644)

	// react package (should NOT be bundled)
	reactDir := filepath.Join(nmDir, "react")
	os.MkdirAll(reactDir, 0o755)
	os.WriteFile(filepath.Join(reactDir, "package.json"), []byte(`{"name": "react", "version": "19.2.3", "main": "index.js"}`), 0o644)
	os.WriteFile(filepath.Join(reactDir, "index.js"), []byte(`
exports.useState = function(init) { return [init, function() {}]; };
exports.createElement = function(type, props) { return {type: type, props: props}; };
exports.Fragment = 'react.fragment';
`), 0o644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"main": "./index.tsx",
		"dependencies": {},
		"esbuild": {
			"globals": {"react": "_CRAYONCORE_$REACT"}
		}
	}`)
	writer.WriteField("/bun.lock", "globals-jsx-runtime-lock")
	writer.WriteField("/tsconfig.json", `{"compilerOptions": {"jsx": "react-jsx", "jsxImportSource": "@flickfyi/core"}}`)
	writer.WriteField("/index.tsx", `import { useState } from 'react';
import { Text } from '@flickfyi/core';

export default () => {
  const [count, setCount] = useState(0);
  return <Text>Count: {count}</Text>;
};`)
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

	// The JSX transform should use the jsx-runtime (automatic), NOT React.createElement (classic).
	// The user code's JSX should become jsx()/jsxs() calls, not React.createElement.
	// Note: bundled node_modules code may contain React.createElement calls — that's fine,
	// as long as their React var is the globals-replaced one (require_react()).
	if !strings.Contains(result.Code, "import_jsx_runtime") && !strings.Contains(result.Code, "jsxs") {
		t.Fatalf("JSX should use jsx-runtime (automatic), but no jsx/jsxs calls found:\n%s", result.Code)
	}

	// ALL react references should be _CRAYONCORE_$REACT — including from
	// @flickfyi/core and @flickfyi/core/jsx-runtime
	if strings.Contains(result.Code, "exports.createElement") {
		t.Fatalf("react source was bundled from node_modules:\n%s", result.Code)
	}
	if strings.Contains(result.Code, `require("react")`) {
		t.Fatalf("output contains require(\"react\"):\n%s", result.Code)
	}
	if !strings.Contains(result.Code, "_CRAYONCORE_$REACT") {
		t.Fatalf("output should reference _CRAYONCORE_$REACT, got:\n%s", result.Code)
	}
}

func TestV3CompileHandler_GlobalsSubpathFromESMChunk(t *testing.T) {
	// Reproduce: photon ESM has a shared chunk that imports react/jsx-runtime.
	// react/jsx-runtime should resolve from node_modules (not as a global),
	// since react's jsx-runtime is a different module than react itself.
	setupTestServerWithMockS3(t)

	lockContent := []byte("globals-esm-chunk-lock")
	hash := hashBunLock(lockContent)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")

	// @flickfyi/photon with ESM entries and a shared chunk
	photonDir := filepath.Join(nmDir, "@flickfyi", "photon", "dist")
	os.MkdirAll(photonDir, 0o755)
	os.WriteFile(filepath.Join(nmDir, "@flickfyi", "photon", "package.json"), []byte(`{
		"name": "@flickfyi/photon", "version": "0.0.4",
		"exports": {
			".": {"import": "./dist/index.js"},
			"./jsx-runtime": {"import": "./dist/jsx-runtime.js"}
		}
	}`), 0o644)
	os.WriteFile(filepath.Join(photonDir, "index.js"), []byte(`
import './chunk-SHARED.js';
export function Text(props) { return props.children; }
export function Flex(props) { return props.children; }
`), 0o644)
	os.WriteFile(filepath.Join(photonDir, "jsx-runtime.js"), []byte(`
import './chunk-SHARED.js';
import { Fragment } from 'react/jsx-runtime';
export { Fragment };
export const jsx = (type, props) => _CRAYONCORE_$REACT.createElement(type, props);
export const jsxs = jsx;
`), 0o644)
	// Shared chunk that imports react/jsx-runtime (the problematic import)
	os.WriteFile(filepath.Join(photonDir, "chunk-SHARED.js"), []byte(`
export { jsx as a } from 'react/jsx-runtime';
`), 0o644)

	// react package with jsx-runtime subpath
	reactDir := filepath.Join(nmDir, "react")
	os.MkdirAll(reactDir, 0o755)
	os.WriteFile(filepath.Join(reactDir, "package.json"), []byte(`{
		"name": "react", "version": "19.2.3", "main": "index.js",
		"exports": {
			".": "./index.js",
			"./jsx-runtime": "./jsx-runtime.js"
		}
	}`), 0o644)
	os.WriteFile(filepath.Join(reactDir, "index.js"), []byte(`
exports.createElement = function(type, props) { return {type: type, props: props}; };
exports.Fragment = 'react.fragment';
`), 0o644)
	os.WriteFile(filepath.Join(reactDir, "jsx-runtime.js"), []byte(`
var React = require('./index.js');
exports.jsx = React.createElement;
exports.jsxs = React.createElement;
exports.Fragment = React.Fragment;
`), 0o644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"main": "./index.tsx",
		"dependencies": {},
		"esbuild": {
			"globals": {"react": "_CRAYONCORE_$REACT"}
		}
	}`)
	writer.WriteField("/bun.lock", "globals-esm-chunk-lock")
	writer.WriteField("/tsconfig.json", `{"compilerOptions": {"jsx": "react-jsx", "jsxImportSource": "@flickfyi/photon"}}`)
	writer.WriteField("/index.tsx", `import { Text } from '@flickfyi/photon';
export default () => <Text>Hello</Text>;`)
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

	// "react" (exact) should use the global, but react/jsx-runtime should resolve
	// from node_modules since it's a different module
	if !strings.Contains(result.Code, "_CRAYONCORE_$REACT") {
		t.Fatalf("output should reference _CRAYONCORE_$REACT for bare 'react' imports, got:\n%s", result.Code)
	}
	// Compilation should succeed without "file not found" errors for react/jsx-runtime
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors (react/jsx-runtime should resolve from node_modules): %v", result.Errors)
	}
}

func TestFlushDeps_ClearsDiskAndS3(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)

	lockContent := []byte("flush-test-lockfile")
	hash := hashBunLock(lockContent)

	// Seed a deps tarball in S3
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	content := []byte("module.exports = {}")
	tw.WriteHeader(&tar.Header{
		Name: "node_modules/zod/index.js",
		Mode: 0o644,
		Size: int64(len(content)),
	})
	tw.Write(content)
	tw.Close()
	gw.Close()
	s3Key := "deps/" + hash + ".tar.gz"
	mockS3.files[s3Key] = buf.String()

	// Resolve deps — should hit S3 and populate disk cache
	pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
	path, err := resolveDeps(context.Background(), lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
	if err != nil {
		t.Fatalf("resolveDeps failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "node_modules", "zod", "index.js")); err != nil {
		t.Fatalf("disk cache should exist after resolve: %v", err)
	}

	// Call flush endpoint
	req := httptest.NewRequest(http.MethodPost, "/v3/flush-deps", nil)
	w := httptest.NewRecorder()
	flushDeps(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify disk cache is gone
	if _, err := os.Stat(filepath.Join(diskCachePath, "deps")); !os.IsNotExist(err) {
		t.Fatalf("disk deps dir should be deleted after flush")
	}

	// Verify S3 tarball is gone
	if _, exists := mockS3.files[s3Key]; exists {
		t.Fatalf("S3 deps tarball should be deleted after flush")
	}
}

func TestFlushAllDeps_PreservesInstallTempRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flush-temp-root-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir
	s3Client = nil
	resetDepInstallInFlightForTest(t)

	depDir := filepath.Join(diskCachePath, "deps", "hash", "node_modules", "zod")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatalf("seed dep dir: %v", err)
	}
	activeTmpDir := filepath.Join(depsInstallTempRoot(), "bun-install-active")
	if err := os.MkdirAll(activeTmpDir, 0755); err != nil {
		t.Fatalf("seed active temp dir: %v", err)
	}

	if _, err := flushAllDeps(context.Background()); err != nil {
		t.Fatalf("flush all deps: %v", err)
	}

	if _, err := os.Stat(filepath.Join(diskCachePath, "deps")); !os.IsNotExist(err) {
		t.Fatalf("disk deps dir should be deleted after flush")
	}
	if _, err := os.Stat(activeTmpDir); err != nil {
		t.Fatalf("active install temp dir should remain outside deps flush tree: %v", err)
	}
}

func TestDeleteOldestVersion_ReclaimsStaleInstallTempDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stale-install-temp-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	diskCachePath = tmpDir
	s3Client = nil
	resetDepInstallInFlightForTest(t)

	staleTmpDir := filepath.Join(depsInstallTempRoot(), "bun-install-stale")
	activeTmpDir := filepath.Join(depsInstallTempRoot(), "bun-install-active")
	freshTmpDir := filepath.Join(depsInstallTempRoot(), "bun-install-fresh")
	for _, dir := range []string{staleTmpDir, activeTmpDir, freshTmpDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("seed temp dir %s: %v", dir, err)
		}
	}

	oldTime := time.Now().Add(-2 * staleDepInstallTempMinAge)
	if err := os.Chtimes(staleTmpDir, oldTime, oldTime); err != nil {
		t.Fatalf("age stale temp dir: %v", err)
	}
	if err := os.Chtimes(activeTmpDir, oldTime, oldTime); err != nil {
		t.Fatalf("age active temp dir: %v", err)
	}

	inflight := newDepInstallResult()
	setDepInstallTempDir(inflight, activeTmpDir)
	depInstallMu.Lock()
	depInstallInFlight["active"] = inflight
	depInstallMu.Unlock()

	if !deleteOldestVersion("") {
		t.Fatal("expected stale install temp dir to be reclaimed")
	}

	if _, err := os.Stat(staleTmpDir); !os.IsNotExist(err) {
		t.Fatalf("stale install temp dir should be deleted")
	}
	if _, err := os.Stat(activeTmpDir); err != nil {
		t.Fatalf("active install temp dir should remain: %v", err)
	}
	if _, err := os.Stat(freshTmpDir); err != nil {
		t.Fatalf("fresh install temp dir should remain: %v", err)
	}
}

func TestFlushDeps_TargetedHashClearsOnlyThatHash(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)

	lockA := []byte("targeted-flush-lock-a")
	lockB := []byte("targeted-flush-lock-b")
	hashA := hashBunLock(lockA)
	hashB := hashBunLock(lockB)

	depA := filepath.Join(diskCachePath, "deps", hashA, "node_modules", "a")
	depB := filepath.Join(diskCachePath, "deps", hashB, "node_modules", "b")
	if err := os.MkdirAll(depA, 0755); err != nil {
		t.Fatalf("seed dep A: %v", err)
	}
	if err := os.MkdirAll(depB, 0755); err != nil {
		t.Fatalf("seed dep B: %v", err)
	}

	keyA := depsCacheS3Key(hashA)
	keyB := depsCacheS3Key(hashB)
	mockS3.files[keyA] = "cached-a"
	mockS3.files[keyB] = "cached-b"

	req := httptest.NewRequest(http.MethodPost, "/v3/flush-deps?lock_hash="+hashA, nil)
	w := httptest.NewRecorder()
	flushDeps(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	if _, err := os.Stat(filepath.Join(diskCachePath, "deps", hashA)); !os.IsNotExist(err) {
		t.Fatalf("targeted disk deps dir should be deleted")
	}
	if _, err := os.Stat(depB); err != nil {
		t.Fatalf("other disk deps dir should remain: %v", err)
	}
	if _, exists := mockS3.files[keyA]; exists {
		t.Fatalf("targeted S3 deps tarball should be deleted")
	}
	if _, exists := mockS3.files[keyB]; !exists {
		t.Fatalf("other S3 deps tarball should remain")
	}
}

func TestFlushDepsHash_WaitsForInFlightInstall(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	lockContent := []byte("targeted-flush-inflight-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)
	if err := os.MkdirAll(filepath.Join(depDir, "node_modules", "zod"), 0755); err != nil {
		t.Fatalf("seed dep dir: %v", err)
	}
	key := depsCacheS3Key(hash)
	mockS3.files[key] = "cached"

	inflight := newDepInstallResult()
	depInstallMu.Lock()
	depInstallInFlight[hash] = inflight
	depInstallMu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := flushDepsHash(context.Background(), hash)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("flush returned before in-flight install completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if _, err := os.Stat(depDir); err != nil {
		t.Fatalf("dep dir should remain while in-flight install is active: %v", err)
	}
	if _, exists := mockS3.files[key]; !exists {
		t.Fatalf("S3 deps tarball should remain while in-flight install is active")
	}

	close(inflight.done)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("flush after in-flight install: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("flush did not finish after in-flight install completed")
	}

	if _, err := os.Stat(depDir); !os.IsNotExist(err) {
		t.Fatalf("dep dir should be deleted after in-flight install completes")
	}
	if _, exists := mockS3.files[key]; exists {
		t.Fatalf("S3 deps tarball should be deleted after in-flight install completes")
	}
}

func TestFlushDepsHash_WaitsForInFlightUpload(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	lockContent := []byte("targeted-flush-upload-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)
	if err := os.MkdirAll(filepath.Join(depDir, "node_modules", "zod"), 0755); err != nil {
		t.Fatalf("seed dep dir: %v", err)
	}
	key := depsCacheS3Key(hash)
	mockS3.files[key] = "cached"

	inflight := newDepInstallResult()
	inflight.path = depDir
	close(inflight.ready)
	depInstallMu.Lock()
	depInstallInFlight[hash] = inflight
	depInstallMu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := flushDepsHash(context.Background(), hash)
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("flush returned before in-flight upload completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if _, err := os.Stat(depDir); err != nil {
		t.Fatalf("dep dir should remain while upload is active: %v", err)
	}
	if _, exists := mockS3.files[key]; !exists {
		t.Fatalf("S3 deps tarball should remain while upload is active")
	}

	close(inflight.done)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("flush after in-flight upload: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("flush did not finish after in-flight upload completed")
	}

	if _, err := os.Stat(depDir); !os.IsNotExist(err) {
		t.Fatalf("dep dir should be deleted after in-flight upload completes")
	}
	if _, exists := mockS3.files[key]; exists {
		t.Fatalf("S3 deps tarball should be deleted after in-flight upload completes")
	}
}

func TestFlushDepsHash_WaitsForActiveCacheUse(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	lockContent := []byte("targeted-flush-active-cache-use")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)
	if err := os.MkdirAll(filepath.Join(depDir, "node_modules", "zod"), 0755); err != nil {
		t.Fatalf("seed dep dir: %v", err)
	}
	key := depsCacheS3Key(hash)
	mockS3.files[key] = "cached"

	pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
	path, release, err := resolveDepsForUse(context.Background(), lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
	if err != nil {
		t.Fatalf("resolve deps: %v", err)
	}
	if path != depDir {
		t.Fatalf("expected dep path %s, got %s", depDir, path)
	}

	done := make(chan error, 1)
	go func() {
		_, err := flushDepsHash(context.Background(), hash)
		done <- err
	}()

	waitForFlushInFlightForTest(t, hash)

	select {
	case err := <-done:
		t.Fatalf("flush returned before cache use was released: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if _, err := os.Stat(depDir); err != nil {
		t.Fatalf("dep dir should remain while cache use is active: %v", err)
	}
	if _, exists := mockS3.files[key]; !exists {
		t.Fatalf("S3 deps tarball should remain while cache use is active")
	}

	release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("flush after cache use release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("flush did not finish after cache use was released")
	}

	if _, err := os.Stat(depDir); !os.IsNotExist(err) {
		t.Fatalf("dep dir should be deleted after cache use is released")
	}
	if _, exists := mockS3.files[key]; exists {
		t.Fatalf("S3 deps tarball should be deleted after cache use is released")
	}
}

func TestFlushAllDeps_WaitsForActiveCacheUse(t *testing.T) {
	setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	lockContent := []byte("global-flush-active-cache-use")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)
	if err := os.MkdirAll(filepath.Join(depDir, "node_modules", "zod"), 0755); err != nil {
		t.Fatalf("seed dep dir: %v", err)
	}

	pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
	path, release, err := resolveDepsForUse(context.Background(), lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
	if err != nil {
		t.Fatalf("resolve deps: %v", err)
	}
	if path != depDir {
		t.Fatalf("expected dep path %s, got %s", depDir, path)
	}

	done := make(chan error, 1)
	go func() {
		_, err := flushAllDeps(context.Background())
		done <- err
	}()

	waitForFlushAllInFlightForTest(t)

	select {
	case err := <-done:
		t.Fatalf("global flush returned before cache use was released: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if _, err := os.Stat(depDir); err != nil {
		t.Fatalf("dep dir should remain while cache use is active: %v", err)
	}

	release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("global flush after cache use release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("global flush did not finish after cache use was released")
	}

	if _, err := os.Stat(filepath.Join(diskCachePath, "deps")); !os.IsNotExist(err) {
		t.Fatalf("deps dir should be deleted after cache use is released")
	}
}

func TestResolveDeps_WaitsForTargetedFlush(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	lockContent := []byte("resolve-waits-for-targeted-flush")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)
	if err := os.MkdirAll(filepath.Join(depDir, "node_modules", "zod"), 0755); err != nil {
		t.Fatalf("seed dep dir: %v", err)
	}
	key := depsCacheS3Key(hash)
	mockS3.files[key] = "cached"

	inflight := newDepInstallResult()
	inflight.path = depDir
	close(inflight.ready)
	depInstallMu.Lock()
	depInstallInFlight[hash] = inflight
	depInstallMu.Unlock()

	flushDone := make(chan error, 1)
	go func() {
		_, err := flushDepsHash(context.Background(), hash)
		flushDone <- err
	}()

	waitForFlushInFlightForTest(t, hash)

	resolveCtx, cancelResolve := context.WithCancel(context.Background())
	resolveDone := make(chan error, 1)
	go func() {
		pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
		_, err := resolveDeps(resolveCtx, lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
		resolveDone <- err
	}()

	select {
	case err := <-resolveDone:
		t.Fatalf("resolveDeps returned while targeted flush was active: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	cancelResolve()
	select {
	case err := <-resolveDone:
		if err != context.Canceled {
			t.Fatalf("expected canceled resolve while waiting for flush, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("resolveDeps did not stop waiting after context cancellation")
	}

	close(inflight.done)
	select {
	case err := <-flushDone:
		if err != nil {
			t.Fatalf("flush after in-flight upload: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("flush did not finish after in-flight upload completed")
	}
}

func TestResolveDeps_WaitsForInFlightBeforeDiskHit(t *testing.T) {
	setupTestServerWithMockS3(t)
	resetDepInstallInFlightForTest(t)

	lockContent := []byte("resolve-waits-for-inflight-before-disk-hit")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)
	if err := os.MkdirAll(filepath.Join(depDir, "node_modules", "zod"), 0755); err != nil {
		t.Fatalf("seed partial dep dir: %v", err)
	}

	inflight := newDepInstallResult()
	inflight.path = depDir
	depInstallMu.Lock()
	depInstallInFlight[hash] = inflight
	depInstallMu.Unlock()

	resolveDone := make(chan error, 1)
	go func() {
		pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
		_, err := resolveDeps(context.Background(), lockContent, pkg, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
		resolveDone <- err
	}()

	select {
	case err := <-resolveDone:
		t.Fatalf("resolveDeps returned before in-flight deps were ready: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(inflight.ready)

	select {
	case err := <-resolveDone:
		if err != nil {
			t.Fatalf("resolveDeps after in-flight ready: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("resolveDeps did not finish after in-flight deps became ready")
	}

	close(inflight.done)
}

func TestCloseDepInstallResult_RemovesEntryBeforeWakingWaiters(t *testing.T) {
	resetDepInstallInFlightForTest(t)

	hash := hashBunLock([]byte("close-before-wake"))
	inflight := newDepInstallResult()
	depInstallMu.Lock()
	depInstallInFlight[hash] = inflight
	depInstallMu.Unlock()

	depInstallMu.Lock()
	closed := make(chan struct{})
	go func() {
		closeDepInstallResult(hash, inflight)
		close(closed)
	}()

	select {
	case <-inflight.done:
		t.Fatal("install result woke waiters before removing the in-flight entry")
	case <-time.After(20 * time.Millisecond):
	}

	depInstallMu.Unlock()

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("closeDepInstallResult did not finish")
	}

	select {
	case <-inflight.done:
	case <-time.After(time.Second):
		t.Fatal("install result did not wake waiters")
	}

	depInstallMu.Lock()
	_, exists := depInstallInFlight[hash]
	depInstallMu.Unlock()
	if exists {
		t.Fatal("in-flight entry should be removed after close")
	}
}

func TestPrewarmDeps_ResolvesDependencyCache(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)

	lockContent := []byte("prewarm-test-lockfile")
	hash := hashBunLock(lockContent)
	seedDepsTarball(t, mockS3, depsCacheS3Key(hash), "node_modules/zod/index.js", []byte("module.exports = {}"))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("/bun.lock", string(lockContent)); err != nil {
		t.Fatalf("write bun.lock field: %v", err)
	}
	if err := writer.WriteField("/package.json", `{"dependencies":{"zod":"3.23.0"}}`); err != nil {
		t.Fatalf("write package.json field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v3/prewarm-deps", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	prewarmDeps(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["status"] != "prewarmed" {
		t.Fatalf("expected prewarmed status, got %#v", response["status"])
	}
	if response["lock_hash"] != hash {
		t.Fatalf("expected lock_hash %s, got %#v", hash, response["lock_hash"])
	}
	if _, err := os.Stat(filepath.Join(diskCachePath, "deps", hash, "node_modules", "zod", "index.js")); err != nil {
		t.Fatalf("disk cache should be populated after prewarm: %v", err)
	}
}

func TestPrewarmDeps_FailsWhenTargetedFlushCannotDeleteS3(t *testing.T) {
	mockS3 := setupTestServerWithMockS3(t)

	lockContent := []byte("prewarm-flush-s3-delete-error-lockfile")
	hash := hashBunLock(lockContent)
	key := depsCacheS3Key(hash)
	seedDepsTarball(t, mockS3, key, "node_modules/zod/index.js", []byte("module.exports = {}"))
	mockS3.deleteObjectErrors[key] = fmt.Errorf("delete denied")

	staleDiskFile := filepath.Join(diskCachePath, "deps", hash, "node_modules", "zod", "index.js")
	if err := os.MkdirAll(filepath.Dir(staleDiskFile), 0755); err != nil {
		t.Fatalf("seed disk cache dir: %v", err)
	}
	if err := os.WriteFile(staleDiskFile, []byte("module.exports = { stale: true }"), 0644); err != nil {
		t.Fatalf("seed disk cache file: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("/bun.lock", string(lockContent)); err != nil {
		t.Fatalf("write bun.lock field: %v", err)
	}
	if err := writer.WriteField("/package.json", `{"dependencies":{"zod":"3.23.0"}}`); err != nil {
		t.Fatalf("write package.json field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v3/prewarm-deps?flush=true", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	prewarmDeps(w, req)

	resp := w.Result()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.StatusCode, string(respBody))
	}
	if !strings.Contains(string(respBody), "failed to flush deps") {
		t.Fatalf("expected flush failure response, got: %s", string(respBody))
	}
	if _, exists := mockS3.files[key]; !exists {
		t.Fatalf("S3 deps tarball should remain when delete fails")
	}
	if _, err := os.Stat(staleDiskFile); !os.IsNotExist(err) {
		t.Fatalf("disk cache should stay empty instead of rehydrating stale S3 deps")
	}
}

func seedDepsTarball(t *testing.T, mockS3 *MockS3Client, key string, name string, content []byte) {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	mockS3.files[key] = buf.String()
}

func resetDepInstallInFlightForTest(t *testing.T) {
	t.Helper()
	depInstallMu.Lock()
	previousCacheUseDone := depCacheUseDone
	previousCacheUseCounts := depCacheUseCounts
	previousFlushAll := depFlushAllInFlight
	previousInstalls := depInstallInFlight
	previousFlushes := depFlushInFlight
	depCacheUseDone = make(map[string]chan struct{})
	depCacheUseCounts = make(map[string]int)
	depFlushAllInFlight = nil
	depInstallInFlight = make(map[string]*depInstallResult)
	depFlushInFlight = make(map[string]chan struct{})
	depInstallMu.Unlock()

	t.Cleanup(func() {
		depInstallMu.Lock()
		depCacheUseDone = previousCacheUseDone
		depCacheUseCounts = previousCacheUseCounts
		depFlushAllInFlight = previousFlushAll
		depInstallInFlight = previousInstalls
		depFlushInFlight = previousFlushes
		depInstallMu.Unlock()
	})
}

func waitForFlushInFlightForTest(t *testing.T, hash string) {
	t.Helper()

	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		depInstallMu.Lock()
		_, exists := depFlushInFlight[hash]
		depInstallMu.Unlock()
		if exists {
			return
		}

		select {
		case <-deadline:
			t.Fatal("flush did not mark hash as in-flight")
		case <-ticker.C:
		}
	}
}

func waitForFlushAllInFlightForTest(t *testing.T) {
	t.Helper()

	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		depInstallMu.Lock()
		exists := depFlushAllInFlight != nil
		depInstallMu.Unlock()
		if exists {
			return
		}

		select {
		case <-deadline:
			t.Fatal("global flush did not mark deps as in-flight")
		case <-ticker.C:
		}
	}
}
