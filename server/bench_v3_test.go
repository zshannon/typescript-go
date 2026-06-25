package main

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const benchLockContent = "bench-lock"

// setupV3BenchServer initializes the server with mock S3 and pre-populated v3 dep cache.
// Silences log output during benchmarks. Must be called once per top-level benchmark.
func setupV3BenchServer(b *testing.B) {
	b.Helper()

	mockS3 := NewMockS3Client()
	s3Client = mockS3
	s3Bucket = "test-bucket"
	serverVersion = "1.0.0"
	startTime = time.Now()

	tmpDir, err := os.MkdirTemp("", "bench-v3-cache-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	diskCachePath = tmpDir

	// Pre-populate dep cache with @crayonnow/core and react packages on disk
	hash := hashBunLock([]byte(benchLockContent))
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")

	// @crayonnow/core
	coreDir := filepath.Join(nmDir, "@crayonnow", "core")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		b.Fatalf("Failed to create core dir: %v", err)
	}
	writeOrFatal(b, filepath.Join(coreDir, "package.json"),
		`{"name": "@crayonnow/core", "version": "1.0.0", "main": "index.js", "types": "index.d.ts", "exports": {".": {"types": "./index.d.ts", "default": "./index.js"}, "./jsx-runtime": {"types": "./jsx-runtime.d.ts", "default": "./jsx-runtime.js"}}}`)
	writeOrFatal(b, filepath.Join(coreDir, "index.d.ts"),
		`export interface ButtonProps { onClick?: () => void; style?: any; children?: any; [key: string]: any; }
export declare function Button(props: ButtonProps): any;
export interface FlexProps { style?: any; children?: any; [key: string]: any; }
export declare function Flex(props: FlexProps): any;
export interface PickerProps { value?: string; onChange?: (val: string) => void; style?: any; children?: any; [key: string]: any; }
export declare function Picker(props: PickerProps): any;
export interface TextProps { style?: any; children?: any; value?: string; [key: string]: any; }
export declare function Text(props: TextProps): any;`)
	writeOrFatal(b, filepath.Join(coreDir, "index.js"),
		`exports.Button = function(props) { return {type: 'Button', props}; };
exports.Flex = function(props) { return {type: 'Flex', props}; };
exports.Picker = function(props) { return {type: 'Picker', props}; };
exports.Text = function(props) { return {type: 'Text', props}; };`)
	writeOrFatal(b, filepath.Join(coreDir, "jsx-runtime.d.ts"),
		`export namespace JSX { interface Element {} interface IntrinsicElements { [key: string]: any; } }
export function Fragment(props: any): any;
export function jsx(type: any, props: any, key?: any): any;
export function jsxs(type: any, props: any, key?: any): any;`)
	writeOrFatal(b, filepath.Join(coreDir, "jsx-runtime.js"),
		`exports.Fragment = function(props) { return props.children; };
exports.jsx = function(type, props) { return {type, props}; };
exports.jsxs = exports.jsx;`)

	// react
	reactDir := filepath.Join(nmDir, "react")
	if err := os.MkdirAll(reactDir, 0o755); err != nil {
		b.Fatalf("Failed to create react dir: %v", err)
	}
	writeOrFatal(b, filepath.Join(reactDir, "package.json"),
		`{"name": "react", "version": "18.0.0", "main": "index.js", "types": "index.d.ts"}`)
	writeOrFatal(b, filepath.Join(reactDir, "index.d.ts"),
		`export function createElement(type: any, props: any, children?: any): any;
export function useEffect(fn: () => void | (() => void), deps?: any[]): void;
export function useState<T>(init: T): [T, (value: T | ((prev: T) => T)) => void];`)
	writeOrFatal(b, filepath.Join(reactDir, "index.js"),
		`exports.createElement = function(type, props, children) { return {type, props, children}; };
exports.useEffect = function(fn, deps) { };
exports.useState = function(init) { return [init, function() {}]; };`)

	// Silence logs
	origWriter := log.Writer()
	log.SetOutput(io.Discard)

	b.Cleanup(func() {
		log.SetOutput(origWriter)
		os.RemoveAll(tmpDir)
	})
}

// writeOrFatal writes content to a file path, failing the benchmark on error.
func writeOrFatal(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("Failed to write %s: %v", path, err)
	}
}

// v3Files converts v2 string fixtures to v3 []byte format.
func v3Files(files map[string]string) map[string][]byte {
	result := make(map[string][]byte, len(files))
	for k, v := range files {
		result[k] = []byte(v)
	}
	return result
}

// buildV3BenchMultipart creates a multipart/form-data body and content-type for v3 HTTP benchmarks.
func buildV3BenchMultipart(files map[string]string, packageJSON string, tsconfigJSON string, lockContent string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// Config files first
	fw, _ := writer.CreateFormFile("/bun.lock", "/bun.lock")
	fw.Write([]byte(lockContent))
	fw, _ = writer.CreateFormFile("/package.json", "/package.json")
	fw.Write([]byte(packageJSON))
	if tsconfigJSON != "" {
		fw, _ = writer.CreateFormFile("/tsconfig.json", "/tsconfig.json")
		fw.Write([]byte(tsconfigJSON))
	}
	// Source files
	for path, content := range files {
		fw, _ = writer.CreateFormFile(path, path)
		fw.Write([]byte(content))
	}
	writer.Close()
	return body, writer.FormDataContentType()
}

const benchPackageJSON = `{"main": "/index.tsx", "dependencies": {"@crayonnow/core": "1.0.0", "react": "18.0.0"}, "esbuild": {"bundle": true}}`

var benchTsconfigRaw = []byte(`{"compilerOptions": {"lib": ["ES2022", "DOM"], "jsx": "react-jsx", "jsxImportSource": "@crayonnow/core", "module": "preserve", "moduleResolution": "bundler", "skipLibCheck": true, "strict": true, "target": "es2022"}}`)

// Layer 1: Pure Compiler -- calls typecheckV3 directly
func BenchmarkV3Typecheck(b *testing.B) {
	setupV3BenchServer(b)

	cases := []struct {
		name  string
		files map[string][]byte
	}{
		{"MediumComponent", v3Files(singleFileFixture(fixtureMediumComponent))},
		{"MultiFile", v3Files(fixtureMultiFile)},
		{"SmallComponent", v3Files(singleFileFixture(fixtureSmallComponent))},
		{"Trivial", v3Files(singleFileFixture(fixtureTrivial))},
	}

	lockBytes := []byte(benchLockContent)

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				resp := typecheckV3(tc.files, benchTsconfigRaw, lockBytes)
				if len(resp.Errors) > 0 {
					b.Fatalf("Unexpected error: %s", resp.Errors[0].Message)
				}
			}
		})
	}
}

// Layer 2: Full Pipeline -- calls compileV3 directly (compiler + esbuild)
func BenchmarkV3Build(b *testing.B) {
	setupV3BenchServer(b)

	pkg := &v3PackageJSON{
		Dependencies: map[string]string{
			"@crayonnow/core": "1.0.0",
			"react":           "18.0.0",
		},
		Esbuild: v3EsbuildConfig{Bundle: true},
		Main:    "/index.tsx",
	}

	cases := []struct {
		name  string
		files map[string][]byte
	}{
		{"MediumComponent", v3Files(singleFileFixture(fixtureMediumComponent))},
		{"MultiFile", v3Files(fixtureMultiFile)},
		{"SmallComponent", v3Files(singleFileFixture(fixtureSmallComponent))},
		{"Trivial", v3Files(singleFileFixture(fixtureTrivial))},
	}

	lockBytes := []byte(benchLockContent)

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var lastOutputBytes int
			for i := 0; i < b.N; i++ {
				resp := compileV3(tc.files, pkg, benchTsconfigRaw, lockBytes)
				if len(resp.Errors) > 0 {
					b.Fatalf("Unexpected error: %s", resp.Errors[0].Message)
				}
				lastOutputBytes = len(resp.Code)
			}
			b.ReportMetric(float64(lastOutputBytes), "output_bytes/op")
		})
	}
}

// Layer 3: Typecheck + Build -- calls typecheckV3 then compileV3
func BenchmarkV3TypecheckAndBuild(b *testing.B) {
	setupV3BenchServer(b)

	pkg := &v3PackageJSON{
		Dependencies: map[string]string{
			"@crayonnow/core": "1.0.0",
			"react":           "18.0.0",
		},
		Esbuild: v3EsbuildConfig{Bundle: true},
		Main:    "/index.tsx",
	}

	cases := []struct {
		name  string
		files map[string][]byte
	}{
		{"MediumComponent", v3Files(singleFileFixture(fixtureMediumComponent))},
		{"MultiFile", v3Files(fixtureMultiFile)},
		{"SmallComponent", v3Files(singleFileFixture(fixtureSmallComponent))},
		{"Trivial", v3Files(singleFileFixture(fixtureTrivial))},
	}

	lockBytes := []byte(benchLockContent)

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var lastOutputBytes int
			for i := 0; i < b.N; i++ {
				tcResp := typecheckV3(tc.files, benchTsconfigRaw, lockBytes)
				if len(tcResp.Errors) > 0 {
					b.Fatalf("Unexpected typecheck error: %s", tcResp.Errors[0].Message)
				}
				buildResp := compileV3(tc.files, pkg, benchTsconfigRaw, lockBytes)
				if len(buildResp.Errors) > 0 {
					b.Fatalf("Unexpected build error: %s", buildResp.Errors[0].Message)
				}
				lastOutputBytes = len(buildResp.Code)
			}
			b.ReportMetric(float64(lastOutputBytes), "output_bytes/op")
		})
	}
}

// Layer 4: HTTP Handler -- full request path through v3 handlers
func BenchmarkV3HTTP(b *testing.B) {
	setupV3BenchServer(b)

	type httpCase struct {
		body        []byte
		contentType string
		handler     http.HandlerFunc
		method      string
		name        string
		path        string
	}

	// Pre-serialize multipart bodies outside the benchmark loop
	mediumTypecheckBody, mediumTypecheckCT := buildV3BenchMultipart(
		singleFileFixture(fixtureMediumComponent), benchPackageJSON, string(benchTsconfigRaw), benchLockContent,
	)
	multiFileTypecheckBody, multiFileTypecheckCT := buildV3BenchMultipart(
		fixtureMultiFile, benchPackageJSON, string(benchTsconfigRaw), benchLockContent,
	)
	mediumBuildBody, mediumBuildCT := buildV3BenchMultipart(
		singleFileFixture(fixtureMediumComponent), benchPackageJSON, string(benchTsconfigRaw), benchLockContent,
	)
	multiFileBuildBody, multiFileBuildCT := buildV3BenchMultipart(
		fixtureMultiFile, benchPackageJSON, string(benchTsconfigRaw), benchLockContent,
	)
	mediumBuildSkipBody, mediumBuildSkipCT := buildV3BenchMultipart(
		singleFileFixture(fixtureMediumComponent), benchPackageJSON, string(benchTsconfigRaw), benchLockContent,
	)
	multiFileBuildSkipBody, multiFileBuildSkipCT := buildV3BenchMultipart(
		fixtureMultiFile, benchPackageJSON, string(benchTsconfigRaw), benchLockContent,
	)

	cases := []httpCase{
		{mediumBuildBody.Bytes(), mediumBuildCT, compileV3Handler, "POST", "Build/MediumComponent", "/v3/compile"},
		{multiFileBuildBody.Bytes(), multiFileBuildCT, compileV3Handler, "POST", "Build/MultiFile", "/v3/compile"},
		{mediumBuildSkipBody.Bytes(), mediumBuildSkipCT, compileV3Handler, "POST", "BuildSkipTypecheck/MediumComponent", "/v3/compile?skip_typecheck=true"},
		{multiFileBuildSkipBody.Bytes(), multiFileBuildSkipCT, compileV3Handler, "POST", "BuildSkipTypecheck/MultiFile", "/v3/compile?skip_typecheck=true"},
		{mediumTypecheckBody.Bytes(), mediumTypecheckCT, typecheckV3Handler, "POST", "Typecheck/MediumComponent", "/v3/typecheck"},
		{multiFileTypecheckBody.Bytes(), multiFileTypecheckCT, typecheckV3Handler, "POST", "Typecheck/MultiFile", "/v3/typecheck"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
				req.Header.Set("Content-Type", tc.contentType)
				w := httptest.NewRecorder()
				tc.handler(w, req)
				if w.Code != http.StatusOK {
					b.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
				}
			}
		})
	}
}
