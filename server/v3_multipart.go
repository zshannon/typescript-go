package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
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

// v3PackageJSON represents the parsed package.json for v3 requests.
type v3PackageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Esbuild         v3EsbuildConfig   `json:"esbuild"`
	Main            string            `json:"main"`
	ResolveS3       []string          `json:"resolve-s3"`
}

// v3EsbuildConfig holds the esbuild configuration from package.json.
type v3EsbuildConfig struct {
	Bundle            bool     `json:"bundle"`
	External          []string `json:"external"`
	Format            string   `json:"format"`
	MinifyIdentifiers bool     `json:"minifyIdentifiers"`
	MinifySyntax      bool     `json:"minifySyntax"`
	MinifyWhitespace  bool     `json:"minifyWhitespace"`
	Platform          string   `json:"platform"`
	Target            string   `json:"target"`
}

// esbuildOptions converts the v3EsbuildConfig to esbuild Go API BuildOptions,
// applying defaults when fields are zero/empty.
//
// Defaults:
//   - Bundle: true
//   - External: nil (bundle everything)
//   - Format: CJS
//   - MinifyIdentifiers: false
//   - MinifySyntax: true
//   - MinifyWhitespace: true
//   - Platform: browser
//   - Target: ES2022
//   - Write: false
func (c v3EsbuildConfig) esbuildOptions() api.BuildOptions {
	opts := api.BuildOptions{
		Bundle:           true,
		MinifySyntax:     true,
		MinifyWhitespace: true,
		Write:            false,
	}

	// Format
	switch strings.ToLower(c.Format) {
	case "esm", "es", "module":
		opts.Format = api.FormatESModule
	case "iife":
		opts.Format = api.FormatIIFE
	default:
		opts.Format = api.FormatCommonJS
	}

	// Platform
	switch strings.ToLower(c.Platform) {
	case "node":
		opts.Platform = api.PlatformNode
	case "neutral":
		opts.Platform = api.PlatformNeutral
	default:
		opts.Platform = api.PlatformBrowser
	}

	// Target
	switch strings.ToLower(c.Target) {
	case "es2015", "es6":
		opts.Target = api.ES2015
	case "es2016":
		opts.Target = api.ES2016
	case "es2017":
		opts.Target = api.ES2017
	case "es2018":
		opts.Target = api.ES2018
	case "es2019":
		opts.Target = api.ES2019
	case "es2020":
		opts.Target = api.ES2020
	case "es2021":
		opts.Target = api.ES2021
	case "es2023":
		opts.Target = api.ES2023
	case "esnext":
		opts.Target = api.ESNext
	default:
		opts.Target = api.ES2022
	}

	// MinifyIdentifiers: only enable if explicitly set true
	if c.MinifyIdentifiers {
		opts.MinifyIdentifiers = true
	}

	// MinifySyntax: only disable if explicitly set false AND format was specified
	if c.Format != "" && !c.MinifySyntax {
		opts.MinifySyntax = false
	}

	// MinifyWhitespace: only disable if explicitly set false AND format was specified
	if c.Format != "" && !c.MinifyWhitespace {
		opts.MinifyWhitespace = false
	}

	// External: nil means bundle everything (default)
	if len(c.External) > 0 {
		opts.External = c.External
	}

	return opts
}

// parsePackageJSON parses raw package.json bytes.
func parsePackageJSON(raw []byte) (*v3PackageJSON, error) {
	var pkg v3PackageJSON
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, fmt.Errorf("invalid package.json: %w", err)
	}
	return &pkg, nil
}
