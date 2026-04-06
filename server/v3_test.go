package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
