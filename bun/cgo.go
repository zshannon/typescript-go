package main

/*
#include <stdlib.h>
#include <string.h>

// Simple diagnostic structure for FFI
typedef struct {
    int code;
    char* category;
    char* message;
    char* file;
    int line;
    int column;
    int length;
} ffi_diagnostic;

// Result structure for type checking
typedef struct {
    int success;
    char* output;           // JSON output or compiled code
    ffi_diagnostic* diagnostics;
    int diagnostic_count;
} ffi_result;
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/locale"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs"
	"github.com/microsoft/typescript-go/internal/vfs/osvfs"
)

var debugMode = os.Getenv("TSGO_DEBUG") == "1"

func debugLog(format string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "[TSGO] "+format+"\n", args...)
	}
}

// InMemoryFS implements vfs.FS for in-memory files
type InMemoryFS struct {
	files map[string]string
	dirs  map[string]bool
	mu    sync.RWMutex
}

func NewInMemoryFS() *InMemoryFS {
	return &InMemoryFS{
		files: make(map[string]string),
		dirs:  make(map[string]bool),
	}
}

func (fs *InMemoryFS) AddFile(path, content string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.files[path] = content
}

func (fs *InMemoryFS) AddDirectory(path string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.dirs[path] = true
}

func (fs *InMemoryFS) UseCaseSensitiveFileNames() bool { return true }

func (fs *InMemoryFS) FileExists(path string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	_, ok := fs.files[path]
	return ok
}

func (fs *InMemoryFS) ReadFile(path string) (string, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	content, ok := fs.files[path]
	return content, ok
}

func (fs *InMemoryFS) WriteFile(path string, data string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.files[path] = data
	return nil
}

func (fs *InMemoryFS) Remove(path string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	delete(fs.files, path)
	return nil
}

func (fs *InMemoryFS) DirectoryExists(path string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if fs.dirs[path] {
		return true
	}
	// Check if any file has this as a prefix
	for filePath := range fs.files {
		if len(filePath) > len(path) && filePath[:len(path)] == path && filePath[len(path)] == '/' {
			return true
		}
	}
	return false
}

func (fs *InMemoryFS) GetAccessibleEntries(path string) vfs.Entries {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filesSet := make(map[string]bool)
	dirsSet := make(map[string]bool)

	prefix := path
	if prefix != "/" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	for filePath := range fs.files {
		if len(filePath) > len(prefix) && filePath[:len(prefix)] == prefix {
			rest := filePath[len(prefix):]
			slashIdx := -1
			for i, c := range rest {
				if c == '/' {
					slashIdx = i
					break
				}
			}
			if slashIdx == -1 {
				filesSet[rest] = true
			} else {
				dirsSet[rest[:slashIdx]] = true
			}
		}
	}

	var files []string
	var dirs []string
	for f := range filesSet {
		files = append(files, f)
	}
	for d := range dirsSet {
		dirs = append(dirs, d)
	}

	return vfs.Entries{Files: files, Directories: dirs}
}

func (fs *InMemoryFS) Stat(path string) vfs.FileInfo   { return nil }
func (fs *InMemoryFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error { return nil }
func (fs *InMemoryFS) Realpath(path string) string     { return path }
func (fs *InMemoryFS) Chtimes(path string, aTime time.Time, mTime time.Time) error { return nil }

// HybridFS combines in-memory files with real filesystem for node_modules
type HybridFS struct {
	memFS      *InMemoryFS
	diskFS     vfs.FS
	projectDir string // The real project directory with node_modules
}

func NewHybridFS(memFS *InMemoryFS, projectDir string) *HybridFS {
	return &HybridFS{
		memFS:      memFS,
		diskFS:     osvfs.FS(),
		projectDir: projectDir,
	}
}

func (h *HybridFS) UseCaseSensitiveFileNames() bool { return true }

func (h *HybridFS) toRealPath(path string) string {
	// Map virtual paths to real paths
	if len(path) > 0 && path[0] == '/' {
		// /node_modules/... -> {projectDir}/node_modules/...
		if strings.HasPrefix(path, "/node_modules/") {
			realPath := h.projectDir + path
			debugLog("toRealPath: %s -> %s", path, realPath)
			return realPath
		}
		// Already a real path under projectDir/node_modules - pass through
		nodeModulesPrefix := h.projectDir + "/node_modules/"
		if strings.HasPrefix(path, nodeModulesPrefix) {
			debugLog("toRealPath: %s -> %s (passthrough)", path, path)
			return path
		}
		// Handle absolute real paths (after symlink resolution)
		// These are paths like /Users/... that exist on the real filesystem
		if h.diskFS.FileExists(path) || h.diskFS.DirectoryExists(path) {
			debugLog("toRealPath: %s -> %s (real filesystem path)", path, path)
			return path
		}
		// /project/... -> keep in memory
	}
	return ""
}

func (h *HybridFS) FileExists(path string) bool {
	// Check in-memory first
	if h.memFS.FileExists(path) {
		debugLog("FileExists (mem): %s = true", path)
		return true
	}
	// Check real filesystem for node_modules
	realPath := h.toRealPath(path)
	if realPath != "" {
		exists := h.diskFS.FileExists(realPath)
		debugLog("FileExists (disk): %s (%s) = %v", path, realPath, exists)
		return exists
	}
	debugLog("FileExists: %s = false (no mapping)", path)
	return false
}

func (h *HybridFS) ReadFile(path string) (string, bool) {
	// Check in-memory first
	if content, ok := h.memFS.ReadFile(path); ok {
		debugLog("ReadFile (mem): %s = found (%d bytes)", path, len(content))
		return content, true
	}
	// Check real filesystem for node_modules
	realPath := h.toRealPath(path)
	if realPath != "" {
		content, ok := h.diskFS.ReadFile(realPath)
		debugLog("ReadFile (disk): %s (%s) = %v (%d bytes)", path, realPath, ok, len(content))
		return content, ok
	}
	debugLog("ReadFile: %s = not found (no mapping)", path)
	return "", false
}

func (h *HybridFS) WriteFile(path string, data string) error {
	return h.memFS.WriteFile(path, data)
}

func (h *HybridFS) Remove(path string) error {
	return h.memFS.Remove(path)
}

func (h *HybridFS) DirectoryExists(path string) bool {
	if h.memFS.DirectoryExists(path) {
		debugLog("DirectoryExists (mem): %s = true", path)
		return true
	}
	realPath := h.toRealPath(path)
	if realPath != "" {
		exists := h.diskFS.DirectoryExists(realPath)
		debugLog("DirectoryExists (disk): %s (%s) = %v", path, realPath, exists)
		return exists
	}
	debugLog("DirectoryExists: %s = false (no mapping)", path)
	return false
}

func (h *HybridFS) GetAccessibleEntries(path string) vfs.Entries {
	// Combine entries from memory and disk
	memEntries := h.memFS.GetAccessibleEntries(path)

	realPath := h.toRealPath(path)
	if realPath != "" {
		diskEntries := h.diskFS.GetAccessibleEntries(realPath)
		// Merge
		filesSet := make(map[string]bool)
		dirsSet := make(map[string]bool)
		for _, f := range memEntries.Files {
			filesSet[f] = true
		}
		for _, d := range memEntries.Directories {
			dirsSet[d] = true
		}
		for _, f := range diskEntries.Files {
			filesSet[f] = true
		}
		for _, d := range diskEntries.Directories {
			dirsSet[d] = true
		}
		var files, dirs []string
		for f := range filesSet {
			files = append(files, f)
		}
		for d := range dirsSet {
			dirs = append(dirs, d)
		}
		return vfs.Entries{Files: files, Directories: dirs}
	}
	return memEntries
}

func (h *HybridFS) Stat(path string) vfs.FileInfo {
	realPath := h.toRealPath(path)
	if realPath != "" {
		return h.diskFS.Stat(realPath)
	}
	return nil
}

func (h *HybridFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error {
	realPath := h.toRealPath(root)
	if realPath != "" {
		return h.diskFS.WalkDir(realPath, walkFn)
	}
	return nil
}

func (h *HybridFS) Realpath(path string) string {
	realPath := h.toRealPath(path)
	if realPath != "" {
		return h.diskFS.Realpath(realPath)
	}
	return path
}

func (h *HybridFS) Chtimes(path string, aTime time.Time, mTime time.Time) error {
	return nil
}

// Diagnostic represents a TypeScript diagnostic
type Diagnostic struct {
	Code     int    `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Length   int    `json:"length,omitempty"`
}

// TypeCheckResult represents the result of type checking
type TypeCheckResult struct {
	Success     bool         `json:"success"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Duration    float64      `json:"duration_ms"`
}

func calculateLineColumn(text string, pos int) (line, column int) {
	if pos < 0 || pos > len(text) {
		return 0, 0
	}
	for i := 0; i < pos && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			column = 0
		} else {
			column++
		}
	}
	return line, column
}

func convertDiagnostics(diagnostics []*ast.Diagnostic) []Diagnostic {
	result := make([]Diagnostic, len(diagnostics))
	for i, diag := range diagnostics {
		result[i] = Diagnostic{
			Code:     int(diag.Code()),
			Category: diag.Category().Name(),
			Message:  diag.Localize(locale.Default),
		}
		if diag.File() != nil {
			result[i].File = diag.File().FileName()
			if diag.Loc().Pos() >= 0 {
				line, col := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
				result[i].Line = line + 1
				result[i].Column = col + 1
				result[i].Length = diag.Loc().End() - diag.Loc().Pos()
			}
		}
	}
	return result
}

// typeCheckCode performs type checking on TypeScript code
// projectDir is the real directory containing node_modules (for reading installed packages)
func typeCheckCode(code string, fileName string, options *core.CompilerOptions, projectDir string) TypeCheckResult {
	start := time.Now()

	// Create in-memory filesystem
	memFS := NewInMemoryFS()
	memFS.AddDirectory("/project")
	memFS.AddDirectory("/node_modules")

	// Normalize fileName to be under /project for the compiler host
	if strings.HasPrefix(fileName, "/") {
		// Absolute path - strip leading / and put under /project/
		fileName = "/project" + fileName
	} else {
		// Relative path - put under /project/
		fileName = "/project/" + fileName
	}
	memFS.AddFile(fileName, code)

	// Create hybrid FS that combines in-memory with real node_modules
	var fs vfs.FS
	if projectDir != "" {
		fs = NewHybridFS(memFS, projectDir)
	} else {
		fs = memFS
	}

	// Wrap with bundled TypeScript libs
	wrappedFS := bundled.WrapFS(fs)

	// Default compiler options if not provided
	if options == nil {
		options = &core.CompilerOptions{
			AllowJs:                          core.TSTrue,
			Declaration:                      core.TSTrue,
			ESModuleInterop:                  core.TSTrue,
			ForceConsistentCasingInFileNames: core.TSTrue,
			IsolatedModules:                  core.TSTrue,
			Jsx:                              core.JsxEmitReactJSX,
			Module:                           core.ModuleKindESNext,
			ModuleResolution:                 core.ModuleResolutionKindBundler,
			NoEmit:                           core.TSTrue,
			ResolveJsonModule:                core.TSTrue,
			SkipLibCheck:                     core.TSTrue,
			Strict:                           core.TSTrue,
			StrictNullChecks:                 core.TSTrue,
			Target:                           core.ScriptTargetES2022,
			Lib:                              []string{"ES2022", "DOM"},
		}
	}

	// Create parsed options
	parsedOptions := &core.ParsedOptions{
		CompilerOptions: options,
		FileNames:       []string{fileName},
	}

	// Create config
	config := &tsoptions.ParsedCommandLine{
		ParsedConfig: parsedOptions,
	}

	// Create host
	host := compiler.NewCachedFSCompilerHost("/project", wrappedFS, bundled.LibPath(), nil, nil)

	// Create program
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           config,
		Host:             host,
	})

	ctx := context.Background()

	// Get all diagnostics - both syntactic AND semantic
	var allDiagnostics []*ast.Diagnostic
	allDiagnostics = append(allDiagnostics, program.GetSyntacticDiagnostics(ctx, nil)...)
	allDiagnostics = append(allDiagnostics, program.GetSemanticDiagnostics(ctx, nil)...)

	duration := time.Since(start).Seconds() * 1000

	// Check for errors
	hasErrors := false
	for _, diag := range allDiagnostics {
		if diag.Category().Name() == "error" {
			hasErrors = true
			break
		}
	}

	return TypeCheckResult{
		Success:     !hasErrors,
		Diagnostics: convertDiagnostics(allDiagnostics),
		Duration:    duration,
	}
}

//export tsgo_typecheck
func tsgo_typecheck(code *C.char, fileName *C.char, projectDir *C.char) *C.char {
	goCode := C.GoString(code)
	goFileName := C.GoString(fileName)
	goProjectDir := C.GoString(projectDir)

	if goFileName == "" {
		goFileName = "/project/input.tsx"
	}

	result := typeCheckCode(goCode, goFileName, nil, goProjectDir)

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		errorResult := TypeCheckResult{
			Success: false,
			Diagnostics: []Diagnostic{{
				Code:     0,
				Category: "error",
				Message:  "Failed to marshal result: " + err.Error(),
			}},
		}
		jsonBytes, _ = json.Marshal(errorResult)
	}

	return C.CString(string(jsonBytes))
}

//export tsgo_typecheck_with_options
func tsgo_typecheck_with_options(code *C.char, fileName *C.char, optionsJSON *C.char, projectDir *C.char) *C.char {
	goCode := C.GoString(code)
	goFileName := C.GoString(fileName)
	goOptionsJSON := C.GoString(optionsJSON)
	goProjectDir := C.GoString(projectDir)

	if goFileName == "" {
		goFileName = "/project/input.tsx"
	}

	// Parse options from JSON
	var optionsMap map[string]interface{}
	var options *core.CompilerOptions

	if goOptionsJSON != "" {
		if err := json.Unmarshal([]byte(goOptionsJSON), &optionsMap); err == nil {
			options = &core.CompilerOptions{}

			// Parse target
			if target, ok := optionsMap["target"].(string); ok {
				switch target {
				case "ES5":
					options.Target = core.ScriptTargetES5
				case "ES6", "ES2015":
					options.Target = core.ScriptTargetES2015
				case "ES2016":
					options.Target = core.ScriptTargetES2016
				case "ES2017":
					options.Target = core.ScriptTargetES2017
				case "ES2018":
					options.Target = core.ScriptTargetES2018
				case "ES2019":
					options.Target = core.ScriptTargetES2019
				case "ES2020":
					options.Target = core.ScriptTargetES2020
				case "ES2021":
					options.Target = core.ScriptTargetES2021
				case "ES2022":
					options.Target = core.ScriptTargetES2022
				case "ESNext":
					options.Target = core.ScriptTargetESNext
				}
			}

			// Parse module
			if module, ok := optionsMap["module"].(string); ok {
				switch module {
				case "CommonJS":
					options.Module = core.ModuleKindCommonJS
				case "AMD":
					options.Module = core.ModuleKindAMD
				case "UMD":
					options.Module = core.ModuleKindUMD
				case "System":
					options.Module = core.ModuleKindSystem
				case "ES6", "ES2015":
					options.Module = core.ModuleKindES2015
				case "ES2020":
					options.Module = core.ModuleKindES2020
				case "ES2022":
					options.Module = core.ModuleKindES2022
				case "ESNext":
					options.Module = core.ModuleKindESNext
				case "Node16":
					options.Module = core.ModuleKindNode16
				case "NodeNext":
					options.Module = core.ModuleKindNodeNext
				case "Preserve":
					options.Module = core.ModuleKindPreserve
				}
			}

			// Parse moduleResolution
			if moduleRes, ok := optionsMap["moduleResolution"].(string); ok {
				switch moduleRes {
				case "Classic":
					options.ModuleResolution = core.ModuleResolutionKindClassic
				case "Node", "Node10":
					options.ModuleResolution = core.ModuleResolutionKindNode10
				case "Node16":
					options.ModuleResolution = core.ModuleResolutionKindNode16
				case "NodeNext":
					options.ModuleResolution = core.ModuleResolutionKindNodeNext
				case "Bundler":
					options.ModuleResolution = core.ModuleResolutionKindBundler
				}
			}

			// Parse JSX options
			if jsx, ok := optionsMap["jsx"].(string); ok {
				switch jsx {
				case "react":
					options.Jsx = core.JsxEmitReact
				case "react-jsx":
					options.Jsx = core.JsxEmitReactJSX
				case "react-jsxdev":
					options.Jsx = core.JsxEmitReactJSXDev
				case "preserve":
					options.Jsx = core.JsxEmitPreserve
				}
			}

			// Parse jsxImportSource (critical for custom JSX runtimes)
			if jsxImportSource, ok := optionsMap["jsxImportSource"].(string); ok {
				options.JsxImportSource = jsxImportSource
			}

			// Parse jsxFactory
			if jsxFactory, ok := optionsMap["jsxFactory"].(string); ok {
				options.JsxFactory = jsxFactory
			}

			// Parse jsxFragmentFactory
			if jsxFragmentFactory, ok := optionsMap["jsxFragmentFactory"].(string); ok {
				options.JsxFragmentFactory = jsxFragmentFactory
			}

			// Parse boolean options
			if strict, ok := optionsMap["strict"].(bool); ok && strict {
				options.Strict = core.TSTrue
				options.StrictNullChecks = core.TSTrue
			}

			if noEmit, ok := optionsMap["noEmit"].(bool); ok && noEmit {
				options.NoEmit = core.TSTrue
			}

			if skipLibCheck, ok := optionsMap["skipLibCheck"].(bool); ok && skipLibCheck {
				options.SkipLibCheck = core.TSTrue
			}

			if allowJs, ok := optionsMap["allowJs"].(bool); ok && allowJs {
				options.AllowJs = core.TSTrue
			}

			if declaration, ok := optionsMap["declaration"].(bool); ok && declaration {
				options.Declaration = core.TSTrue
			}

			if esModuleInterop, ok := optionsMap["esModuleInterop"].(bool); ok && esModuleInterop {
				options.ESModuleInterop = core.TSTrue
			}

			if forceConsistentCasingInFileNames, ok := optionsMap["forceConsistentCasingInFileNames"].(bool); ok && forceConsistentCasingInFileNames {
				options.ForceConsistentCasingInFileNames = core.TSTrue
			}

			if isolatedModules, ok := optionsMap["isolatedModules"].(bool); ok && isolatedModules {
				options.IsolatedModules = core.TSTrue
			}

			if resolveJsonModule, ok := optionsMap["resolveJsonModule"].(bool); ok && resolveJsonModule {
				options.ResolveJsonModule = core.TSTrue
			}

			// Parse lib array
			if lib, ok := optionsMap["lib"].([]interface{}); ok {
				for _, l := range lib {
					if s, ok := l.(string); ok {
						options.Lib = append(options.Lib, s)
					}
				}
			}
		}
	}

	result := typeCheckCode(goCode, goFileName, options, goProjectDir)

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		errorResult := TypeCheckResult{
			Success: false,
			Diagnostics: []Diagnostic{{
				Code:     0,
				Category: "error",
				Message:  "Failed to marshal result: " + err.Error(),
			}},
		}
		jsonBytes, _ = json.Marshal(errorResult)
	}

	return C.CString(string(jsonBytes))
}

//export tsgo_typecheck_multiple
func tsgo_typecheck_multiple(filesJSON *C.char, optionsJSON *C.char, projectDir *C.char) *C.char {
	goFilesJSON := C.GoString(filesJSON)
	goOptionsJSON := C.GoString(optionsJSON)
	goProjectDir := C.GoString(projectDir)

	start := time.Now()

	// Parse files
	var files map[string]string
	if err := json.Unmarshal([]byte(goFilesJSON), &files); err != nil {
		errorResult := TypeCheckResult{
			Success: false,
			Diagnostics: []Diagnostic{{
				Code:     0,
				Category: "error",
				Message:  "Failed to parse files JSON: " + err.Error(),
			}},
		}
		jsonBytes, _ := json.Marshal(errorResult)
		return C.CString(string(jsonBytes))
	}

	// Create in-memory filesystem
	memFS := NewInMemoryFS()
	memFS.AddDirectory("/project")
	memFS.AddDirectory("/node_modules")

	var fileNames []string
	for path, content := range files {
		memFS.AddFile(path, content)
		fileNames = append(fileNames, path)
	}

	// Create hybrid FS that combines in-memory with real node_modules
	var fs vfs.FS
	if goProjectDir != "" {
		fs = NewHybridFS(memFS, goProjectDir)
	} else {
		fs = memFS
	}

	// Wrap with bundled TypeScript libs
	wrappedFS := bundled.WrapFS(fs)

	// Parse options
	options := &core.CompilerOptions{
		Target:           core.ScriptTargetES2022,
		Module:           core.ModuleKindESNext,
		Strict:           core.TSTrue,
		StrictNullChecks: core.TSTrue,
		NoEmit:           core.TSTrue,
		SkipLibCheck:     core.TSTrue,
		Jsx:              core.JsxEmitReactJSX,
		Lib:              []string{"ES2022", "DOM"},
	}

	if goOptionsJSON != "" {
		var optionsMap map[string]interface{}
		if err := json.Unmarshal([]byte(goOptionsJSON), &optionsMap); err == nil {
			if strict, ok := optionsMap["strict"].(bool); ok && strict {
				options.Strict = core.TSTrue
			}
			if jsx, ok := optionsMap["jsx"].(string); ok && jsx == "react-jsx" {
				options.Jsx = core.JsxEmitReactJSX
			}
		}
	}

	// Create parsed options
	parsedOptions := &core.ParsedOptions{
		CompilerOptions: options,
		FileNames:       fileNames,
	}

	// Create config
	config := &tsoptions.ParsedCommandLine{
		ParsedConfig: parsedOptions,
	}

	// Create host
	host := compiler.NewCachedFSCompilerHost("/project", wrappedFS, bundled.LibPath(), nil, nil)

	// Create program
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           config,
		Host:             host,
	})

	ctx := context.Background()

	// Get all diagnostics - both syntactic AND semantic
	var allDiagnostics []*ast.Diagnostic
	allDiagnostics = append(allDiagnostics, program.GetSyntacticDiagnostics(ctx, nil)...)
	allDiagnostics = append(allDiagnostics, program.GetSemanticDiagnostics(ctx, nil)...)

	duration := time.Since(start).Seconds() * 1000

	hasErrors := false
	for _, diag := range allDiagnostics {
		if diag.Category().Name() == "error" {
			hasErrors = true
			break
		}
	}

	result := TypeCheckResult{
		Success:     !hasErrors,
		Diagnostics: convertDiagnostics(allDiagnostics),
		Duration:    duration,
	}

	jsonBytes, _ := json.Marshal(result)
	return C.CString(string(jsonBytes))
}

//export tsgo_free_string
func tsgo_free_string(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

//export tsgo_version
func tsgo_version() *C.char {
	return C.CString("1.0.0")
}

func main() {
	runtime.LockOSThread()
}
