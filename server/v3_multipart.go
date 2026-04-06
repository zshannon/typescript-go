package main

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"
)

// parseV3Multipart parses a multipart/form-data request body where each field
// name is a file path and the value is the file content.
//
// Limits (using package-level constants):
//   - max 100 source files + 2 config files (package.json, bun.lock)
//   - 1MB per file
//   - 10MB total
//
// Required files: /package.json and /bun.lock.
// /node_modules/ paths are rejected.
func parseV3Multipart(body io.Reader, contentType string) (map[string][]byte, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("invalid content-type: %w", err)
	}
	boundary, ok := params["boundary"]
	if !ok {
		return nil, fmt.Errorf("missing multipart boundary")
	}

	mr := multipart.NewReader(body, boundary)

	files := make(map[string][]byte)
	var totalSize int64

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading multipart: %w", err)
		}

		// Field name is the file path
		path := part.FormName()
		if path == "" {
			path = part.FileName()
		}

		// Normalize: prepend / if missing
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		// Reject node_modules paths
		if strings.HasPrefix(path, "/node_modules/") || path == "/node_modules" {
			return nil, fmt.Errorf("rejected path in node_modules: %s", path)
		}

		// Read content with per-file size limit
		limited := io.LimitReader(part, maxFileSizeBytes+1)
		content, err := io.ReadAll(limited)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", path, err)
		}
		if int64(len(content)) > maxFileSizeBytes {
			return nil, fmt.Errorf("file too large: %s (max %d bytes)", path, maxFileSizeBytes)
		}

		files[path] = content
		totalSize += int64(len(content))

		if totalSize > maxTotalSizeBytes {
			return nil, fmt.Errorf("total upload size exceeds %d bytes", maxTotalSizeBytes)
		}
	}

	// Count source files (non-config)
	sourceFiles := 0
	for path := range files {
		if path != "/package.json" && path != "/bun.lock" {
			sourceFiles++
		}
	}

	if sourceFiles > maxFilesPerRequest {
		return nil, fmt.Errorf("too many source files: %d (max %d)", sourceFiles, maxFilesPerRequest)
	}

	// Validate required files
	if _, ok := files["/package.json"]; !ok {
		return nil, fmt.Errorf("missing required file: /package.json")
	}
	if _, ok := files["/bun.lock"]; !ok {
		return nil, fmt.Errorf("missing required file: /bun.lock")
	}

	return files, nil
}
