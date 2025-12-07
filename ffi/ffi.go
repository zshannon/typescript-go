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
	"runtime"
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
)

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

func (fs *InMemoryFS) WriteFile(path string, data string, _ bool) error {
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
func typeCheckCode(code string, fileName string, options *core.CompilerOptions) TypeCheckResult {
	start := time.Now()

	// Create in-memory filesystem
	memFS := NewInMemoryFS()
	memFS.AddFile(fileName, code)
	memFS.AddDirectory("/project")

	// Wrap with bundled TypeScript libs
	wrappedFS := bundled.WrapFS(memFS)

	// Default compiler options if not provided
	if options == nil {
		options = &core.CompilerOptions{
			Target:           core.ScriptTargetES2022,
			Module:           core.ModuleKindESNext,
			Strict:           core.TSTrue,
			StrictNullChecks: core.TSTrue,
			NoEmit:           core.TSTrue,
			SkipLibCheck:     core.TSTrue,
			Jsx:              core.JsxEmitReactJSX,
			Lib:              []string{"ES2022", "DOM"},
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
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})

	ctx := context.Background()

	// Get all diagnostics
	var allDiagnostics []*ast.Diagnostic
	allDiagnostics = append(allDiagnostics, program.GetSyntacticDiagnostics(ctx, nil)...)
	if len(allDiagnostics) == 0 {
		allDiagnostics = append(allDiagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
	}

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
func tsgo_typecheck(code *C.char, fileName *C.char) *C.char {
	goCode := C.GoString(code)
	goFileName := C.GoString(fileName)

	if goFileName == "" {
		goFileName = "/project/input.tsx"
	}

	result := typeCheckCode(goCode, goFileName, nil)

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
func tsgo_typecheck_with_options(code *C.char, fileName *C.char, optionsJSON *C.char) *C.char {
	goCode := C.GoString(code)
	goFileName := C.GoString(fileName)
	goOptionsJSON := C.GoString(optionsJSON)

	if goFileName == "" {
		goFileName = "/project/input.tsx"
	}

	// Parse options from JSON
	var optionsMap map[string]interface{}
	var options *core.CompilerOptions

	if goOptionsJSON != "" {
		if err := json.Unmarshal([]byte(goOptionsJSON), &optionsMap); err == nil {
			options = &core.CompilerOptions{}

			// Parse common options
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

			if lib, ok := optionsMap["lib"].([]interface{}); ok {
				for _, l := range lib {
					if s, ok := l.(string); ok {
						options.Lib = append(options.Lib, s)
					}
				}
			}
		}
	}

	result := typeCheckCode(goCode, goFileName, options)

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
func tsgo_typecheck_multiple(filesJSON *C.char, optionsJSON *C.char) *C.char {
	goFilesJSON := C.GoString(filesJSON)
	goOptionsJSON := C.GoString(optionsJSON)

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

	var fileNames []string
	for path, content := range files {
		memFS.AddFile(path, content)
		fileNames = append(fileNames, path)
	}

	// Wrap with bundled TypeScript libs
	wrappedFS := bundled.WrapFS(memFS)

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
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})

	ctx := context.Background()

	// Get all diagnostics
	var allDiagnostics []*ast.Diagnostic
	allDiagnostics = append(allDiagnostics, program.GetSyntacticDiagnostics(ctx, nil)...)
	if len(allDiagnostics) == 0 {
		allDiagnostics = append(allDiagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
	}

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
