package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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
