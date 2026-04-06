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
