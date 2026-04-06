package main

import (
	"context"
	"io"
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
