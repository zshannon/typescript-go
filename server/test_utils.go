package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// MockS3Client implements a mock S3 client for testing with in-memory files
type MockS3Client struct {
	deleteObjectErrors map[string]error
	files              map[string]string
	getObjectCalls     int
	listObjectsCalls   int
	readFileCalls      []string // Track all file reads for testing
}

func NewMockS3Client() *MockS3Client {
	m := &MockS3Client{
		deleteObjectErrors: make(map[string]error),
		files:              make(map[string]string),
	}
	// Pre-populate with auto-generated package files so ListObjectsV2 returns them
	m.prePopulatePackages("0.0.4")
	return m
}

// prePopulatePackages adds standard mock package files for a version
func (m *MockS3Client) prePopulatePackages(version string) {
	prefix := version + "/node_modules/"

	// @crayonnow/core package
	m.files[prefix+"@crayonnow/core/package.json"] = `{
		"name": "@crayonnow/core",
		"version": "1.0.0",
		"main": "index.js",
		"types": "index.d.ts",
		"exports": {
			".": {
				"types": "./index.d.ts",
				"default": "./index.js"
			},
			"./jsx-runtime": {
				"types": "./jsx-runtime.d.ts",
				"default": "./jsx-runtime.js"
			}
		}
	}`
	m.files[prefix+"@crayonnow/core/index.js"] = `exports.Flex = function(props) { return {type: 'Flex', props}; };
exports.Button = function(props) { return {type: 'Button', props}; };
exports.Text = function(props) { return {type: 'Text', props}; };
exports.Picker = function(props) { return {type: 'Picker', props}; };`
	m.files[prefix+"@crayonnow/core/index.d.ts"] = `
export interface FlexProps {
	style?: any;
	children?: any;
}
export declare function Flex(props: FlexProps): any;

export interface ButtonProps {
	onClick?: () => void;
	style?: any;
	children?: any;
}
export declare function Button(props: ButtonProps): any;

export interface TextProps {
	style?: any;
	children?: any;
}
export declare function Text(props: TextProps): any;`
	m.files[prefix+"@crayonnow/core/jsx-runtime.js"] = `exports.jsx = function(type, props) { return {type, props}; };
exports.jsxs = exports.jsx;
exports.Fragment = function(props) { return props.children; };`
	m.files[prefix+"@crayonnow/core/jsx-runtime.d.ts"] = `export namespace JSX {
	interface Element {}
	interface IntrinsicElements {
		[key: string]: any;
	}
}
export function jsx(type: any, props: any, key?: any): any;
export function jsxs(type: any, props: any, key?: any): any;
export function Fragment(props: any): any;`

	// react package
	m.files[prefix+"react/package.json"] = `{
		"name": "react",
		"version": "18.0.0",
		"main": "index.js",
		"types": "index.d.ts"
	}`
	m.files[prefix+"react/index.js"] = `exports.useState = function(init) { return [init, function() {}]; };
exports.useEffect = function(fn, deps) { };
exports.createElement = function(type, props, children) { return {type, props, children}; };`
	m.files[prefix+"react/index.d.ts"] = `export function useState<T>(init: T): [T, (value: T) => void];
export function useEffect(fn: () => void | (() => void), deps?: any[]): void;
export function createElement(type: any, props: any, children?: any): any;`
}

func (m *MockS3Client) AddFile(key string, content string) {
	m.files[key] = content
}

func (m *MockS3Client) ClearCallCount() {
	m.getObjectCalls = 0
	m.listObjectsCalls = 0
}

func (m *MockS3Client) GetObjectCallCount() int {
	return m.getObjectCalls
}

func (m *MockS3Client) GetListObjectsV2CallCount() int {
	return m.listObjectsCalls
}

func (m *MockS3Client) GetReadFileCalls() []string {
	return m.readFileCalls
}

func (m *MockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getObjectCalls++
	key := aws.ToString(params.Key)
	m.readFileCalls = append(m.readFileCalls, key)

	// Mock file content based on the requested key
	content, exists := m.files[key]
	if !exists {
		// Generate mock content for common packages
		if strings.Contains(key, "@crayonnow/core") {
			if strings.Contains(key, "package.json") {
				content = `{
					"name": "@crayonnow/core",
					"version": "1.0.0",
					"main": "index.js",
					"types": "index.d.ts",
					"exports": {
						".": {
							"types": "./index.d.ts",
							"default": "./index.js"
						},
						"./jsx-runtime": {
							"types": "./jsx-runtime.d.ts",
							"default": "./jsx-runtime.js"
						}
					}
				}`
			} else if strings.Contains(key, "jsx-runtime.d.ts") {
				content = `export namespace JSX {
	interface Element {}
	interface IntrinsicElements {
		[key: string]: any;
	}
}
export function jsx(type: any, props: any, key?: any): any;
export function jsxs(type: any, props: any, key?: any): any;
export function Fragment(props: any): any;`
			} else if strings.Contains(key, "index.d.ts") {
				content = `
export interface FlexProps {
	style?: any;
	children?: any;
}
export declare function Flex(props: FlexProps): any;

export interface ButtonProps {
	onClick?: () => void;
	style?: any;
	children?: any;
}
export declare function Button(props: ButtonProps): any;

export interface TextProps {
	style?: any;
	children?: any;
}
export declare function Text(props: TextProps): any;`
			} else if strings.Contains(key, "jsx-runtime") {
				content = `exports.jsx = function(type, props) { return {type, props}; };
exports.jsxs = exports.jsx;
exports.Fragment = function(props) { return props.children; };`
			} else {
				content = `exports.Flex = function(props) { return {type: 'Flex', props}; };
exports.Button = function(props) { return {type: 'Button', props}; };
exports.Text = function(props) { return {type: 'Text', props}; };
exports.Picker = function(props) { return {type: 'Picker', props}; };`
			}
		} else if strings.Contains(key, "react") && !strings.Contains(key, "@crayonnow") {
			if strings.Contains(key, "package.json") {
				content = `{
					"name": "react",
					"version": "18.0.0",
					"main": "index.js",
					"types": "index.d.ts"
				}`
			} else if strings.Contains(key, "index.d.ts") {
				content = `export function useState<T>(init: T): [T, (value: T) => void];
export function useEffect(fn: () => void | (() => void), deps?: any[]): void;
export function createElement(type: any, props: any, children?: any): any;`
			} else {
				content = `exports.useState = function(init) { return [init, function() {}]; };
exports.useEffect = function(fn, deps) { };
exports.createElement = function(type, props, children) { return {type, props, children}; };`
			}
		} else if strings.Contains(key, "typescript") && strings.Contains(key, ".d.ts") {
			// Return empty TypeScript definitions to avoid type errors
			content = `declare module '@crayonnow/core' {
	export const Flex: any;
	export const Button: any;
	export const Text: any;
	export const Picker: any;
}
declare module 'react' {
	export function useState<T>(init: T): [T, (v: T) => void];
	export function useEffect(fn: () => void, deps?: any[]): void;
}`
		} else {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		m.files[key] = content
	}

	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(content)),
	}, nil
}

func (m *MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.listObjectsCalls++
	prefix := aws.ToString(params.Prefix)
	delimiter := aws.ToString(params.Delimiter)

	var objects []types.Object
	dirs := make(map[string]bool)

	// Find all files matching the prefix
	for key := range m.files {
		if strings.HasPrefix(key, prefix) {
			relativePath := strings.TrimPrefix(key, prefix)

			// If delimiter is set, check for "directories"
			if delimiter != "" && strings.Contains(relativePath, delimiter) {
				// This is in a subdirectory
				dirName := strings.Split(relativePath, delimiter)[0] + delimiter
				dirs[prefix+dirName] = true
			} else if relativePath != "" {
				// This is a direct file
				keyCopy := key
				objects = append(objects, types.Object{
					Key:  &keyCopy,
					Size: aws.Int64(int64(len(m.files[key]))),
				})
			}
		}
	}

	// Convert directories to CommonPrefixes
	var commonPrefixes []types.CommonPrefix
	for dir := range dirs {
		dirCopy := dir
		commonPrefixes = append(commonPrefixes, types.CommonPrefix{
			Prefix: &dirCopy,
		})
	}

	return &s3.ListObjectsV2Output{
		Contents:       objects,
		CommonPrefixes: commonPrefixes,
	}, nil
}

func (m *MockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	key := aws.ToString(params.Key)
	if err := m.deleteObjectErrors[key]; err != nil {
		return nil, err
	}
	delete(m.files, key)
	return &s3.DeleteObjectOutput{}, nil
}

func (m *MockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	key := aws.ToString(params.Key)
	data, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	m.files[key] = string(data)
	return &s3.PutObjectOutput{}, nil
}

// FileBasedMockS3Client reads from testdata directory instead of real S3
type FileBasedMockS3Client struct {
	basePath string
}

// NewFileBasedMockS3Client creates a mock S3 client that reads from testdata
func NewFileBasedMockS3Client() *FileBasedMockS3Client {
	return &FileBasedMockS3Client{
		basePath: "testdata",
	}
}

func (m *FileBasedMockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := *params.Key

	// Convert S3 key to file path
	filePath := filepath.Join(m.basePath, key)

	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		// Try common variations if file not found
		if os.IsNotExist(err) {
			// Try without version prefix if it looks like a node_modules path
			if strings.HasPrefix(key, "0.0.4/") {
				altPath := filepath.Join(m.basePath, key)
				file, err = os.Open(altPath)
			}
		}

		if err != nil {
			return nil, fmt.Errorf("key not found: %s (tried %s): %w", key, filePath, err)
		}
	}

	return &s3.GetObjectOutput{
		Body: file, // os.File already implements io.ReadCloser
	}, nil
}

func (m *FileBasedMockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	prefix := ""
	if params.Prefix != nil {
		prefix = *params.Prefix
	}

	basePath := filepath.Join(m.basePath, prefix)

	var objects []types.Object

	// Walk the directory
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !info.IsDir() {
			// Convert file path back to S3 key
			relPath, _ := filepath.Rel(m.basePath, path)
			relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")

			size := info.Size()
			objects = append(objects, types.Object{
				Key:  &relPath,
				Size: &size,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &s3.ListObjectsV2Output{
		Contents: objects,
	}, nil
}

func (m *FileBasedMockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	key := aws.ToString(params.Key)
	filePath := filepath.Join(m.basePath, key)
	os.Remove(filePath)
	return &s3.DeleteObjectOutput{}, nil
}

func (m *FileBasedMockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	key := aws.ToString(params.Key)
	filePath := filepath.Join(m.basePath, key)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return nil, err
	}
	return &s3.PutObjectOutput{}, nil
}

// LoadRealS3Content loads actual S3 content from downloaded files for testing
func LoadRealS3Content(t *testing.T, mockS3 *MockS3Client) {
	// Load @crayonnow/core files
	if content, err := os.ReadFile("/tmp/crayonnow-package.json"); err == nil {
		mockS3.files["0.0.4/node_modules/@crayonnow/core/package.json"] = string(content)
	}

	if content, err := os.ReadFile("/tmp/crayonnow-index.d.ts"); err == nil {
		mockS3.files["0.0.4/node_modules/@crayonnow/core/dist/index.d.ts"] = string(content)
		// Also put it at the root for TypeScript resolution
		mockS3.files["0.0.4/node_modules/@crayonnow/core/index.d.ts"] = string(content)
	}

	if content, err := os.ReadFile("/tmp/crayonnow-jsx-runtime.d.ts"); err == nil {
		mockS3.files["0.0.4/node_modules/@crayonnow/core/dist/jsx-runtime.d.ts"] = string(content)
		mockS3.files["0.0.4/node_modules/@crayonnow/core/jsx-runtime.d.ts"] = string(content)
	}

	// Load React types
	if content, err := os.ReadFile("/tmp/react-types.d.ts"); err == nil {
		mockS3.files["0.0.4/node_modules/@types/react/index.d.ts"] = string(content)
		mockS3.files["0.0.4/node_modules/react/index.d.ts"] = string(content)
	}

	// React package.json
	mockS3.files["0.0.4/node_modules/react/package.json"] = `{
		"name": "react",
		"version": "18.0.0",
		"main": "index.js",
		"types": "index.d.ts"
	}`

	// Add JavaScript implementations (minimal stubs for runtime)
	mockS3.files["0.0.4/node_modules/@crayonnow/core/dist/index.js"] = `
exports.Button = function(props) { return {type: 'Button', props}; };
exports.Text = function(props) { return {type: 'Text', props}; };
exports.Flex = function(props) { return {type: 'Flex', props}; };
exports.Picker = function(props) { return {type: 'Picker', props}; };
`

	mockS3.files["0.0.4/node_modules/@crayonnow/core/dist/jsx-runtime.js"] = `
exports.jsx = function(type, props) { return {type, props}; };
exports.jsxs = exports.jsx;
exports.Fragment = function(props) { return props.children; };
`
}
