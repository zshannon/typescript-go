package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs"
)

type CompileRequest struct {
	Code string `json:"code"`
}

type CompileResponse struct {
	Code   string            `json:"code,omitempty"`
	Errors []DiagnosticError `json:"errors,omitempty"`
}

type DiagnosticError struct {
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type memoryFS struct {
	files map[string]string
}

func newMemoryFS() *memoryFS {
	return &memoryFS{
		files: map[string]string{
			"/input.ts": "",
		},
	}
}

func (m *memoryFS) UseCaseSensitiveFileNames() bool { return true }
func (m *memoryFS) FileExists(path string) bool {
	_, ok := m.files[path]
	return ok
}
func (m *memoryFS) ReadFile(path string) (string, bool) {
	content, ok := m.files[path]
	return content, ok
}
func (m *memoryFS) WriteFile(path string, data string, _ bool) error {
	m.files[path] = data
	return nil
}
func (m *memoryFS) Remove(path string) error {
	delete(m.files, path)
	return nil
}
func (m *memoryFS) DirectoryExists(path string) bool {
	return path == "/" || path == ""
}
func (m *memoryFS) GetAccessibleEntries(path string) vfs.Entries {
	return vfs.Entries{Files: []string{"input.ts"}}
}
func (m *memoryFS) Stat(path string) vfs.FileInfo { return nil }
func (m *memoryFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error { return nil }
func (m *memoryFS) Realpath(path string) string { return path }

func calculateLineColumn(text string, pos int) (int, int) {
	if pos < 0 || pos >= len(text) {
		return 0, 0
	}
	line, col := 0, 0
	for i := 0; i < pos; i++ {
		if text[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func compileTypeScript(code string) CompileResponse {
	memFS := newMemoryFS()
	memFS.files["/input.ts"] = code
	
	fs := bundled.WrapFS(memFS)
	
	// Create minimal compiler options
	compilerOptions := &core.CompilerOptions{
		Target: core.ScriptTargetESNext,
		Module: core.ModuleKindESNext,
		Strict: core.TSTrue,
		NoEmit: core.TSFalse,
	}
	
	// Create parsed options
	parsedOptions := &core.ParsedOptions{
		CompilerOptions: compilerOptions,
		FileNames:       []string{"/input.ts"},
	}
	
	// Create config
	config := &tsoptions.ParsedCommandLine{
		ParsedConfig: parsedOptions,
	}
	
	// Create cache
	extendedConfigCache := &collections.SyncMap[tspath.Path, *tsoptions.ExtendedConfigCacheEntry]{}
	
	// Create host
	host := compiler.NewCachedFSCompilerHost("/", fs, bundled.LibPath(), extendedConfigCache)
	
	// Create program
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           config,
		Host:             host,
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})
	
	ctx := context.Background()
	
	// Get diagnostics
	diagnostics := program.GetSyntacticDiagnostics(ctx, nil)
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
	}
	
	if len(diagnostics) > 0 {
		errors := make([]DiagnosticError, 0, len(diagnostics))
		for _, diag := range diagnostics {
			err := DiagnosticError{
				Message: diag.Message(),
			}
			if diag.File() != nil && diag.Loc().Pos() >= 0 {
				line, col := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
				err.Line = line + 1
				err.Column = col + 1
			}
			errors = append(errors, err)
		}
		return CompileResponse{Errors: errors}
	}
	
	// Emit
	program.Emit(ctx, compiler.EmitOptions{})
	
	if jsContent, ok := memFS.files["/input.js"]; ok {
		return CompileResponse{Code: jsContent}
	}
	
	return CompileResponse{Errors: []DiagnosticError{{Message: "No output generated"}}}
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "TypeScript Go Server\n")
}

func compile(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var compileReq CompileRequest
	if err := json.NewDecoder(req.Body).Decode(&compileReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if compileReq.Code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	response := compileTypeScript(compileReq.Code)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/compile", compile)
	fmt.Println("Listening on :8080...")
	http.ListenAndServe(":8080", nil)
}