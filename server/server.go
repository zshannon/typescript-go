package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs"
)

type TypecheckRequest struct {
	Code string `json:"code"`
}

type TypecheckResponse struct {
	Pass   bool              `json:"pass,omitempty"`
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
	memFS := &memoryFS{
		files: map[string]string{
			"/input.ts": "",
		},
	}
	
	// Load bundled type definitions
	loadTypeDefinitions(memFS)
	
	return memFS
}

func loadTypeDefinitions(memFS *memoryFS) {
	// Walk the node_modules directory and load .d.ts files
	nodeModulesDir := "/node_modules"
	if _, err := os.Stat(nodeModulesDir); os.IsNotExist(err) {
		// Try local development path
		nodeModulesDir = "./node_modules"
		if _, err := os.Stat(nodeModulesDir); os.IsNotExist(err) {
			return // No node_modules available
		}
	}
	
	loadedCount := 0
	filepath.WalkDir(nodeModulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		if !d.IsDir() && (strings.HasSuffix(path, ".d.ts") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, "package.json")) {
			// Convert file system path to virtual path
			// Example: ./node_modules/@crayonnow/core/index.d.ts -> /node_modules/@crayonnow/core/index.d.ts
			virtualPath := path
			if strings.HasPrefix(virtualPath, "./") {
				virtualPath = virtualPath[2:] // Remove ./
			}
			if !strings.HasPrefix(virtualPath, "/") {
				virtualPath = "/" + virtualPath
			}
			virtualPath = strings.ReplaceAll(virtualPath, "\\", "/")
			
			if content, err := os.ReadFile(path); err == nil {
				memFS.files[virtualPath] = string(content)
				loadedCount++
			}
		}
		
		return nil
	})
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
	if path == "/" || path == "" {
		return true
	}
	
	// Check if any file path starts with this directory
	normalizedPath := strings.TrimSuffix(path, "/") + "/"
	for filePath := range m.files {
		if strings.HasPrefix(filePath, normalizedPath) {
			return true
		}
	}
	
	return false
}

func (m *memoryFS) GetAccessibleEntries(path string) vfs.Entries {
	entries := vfs.Entries{Files: []string{}, Directories: []string{}}
	
	// Normalize the path
	searchPath := strings.TrimSuffix(path, "/")
	if searchPath == "" {
		searchPath = "/"
	} else if !strings.HasPrefix(searchPath, "/") {
		searchPath = "/" + searchPath
	}
	if searchPath != "/" {
		searchPath += "/"
	}
	
	seen := make(map[string]bool)
	
	for filePath := range m.files {
		if !strings.HasPrefix(filePath, searchPath) {
			continue
		}
		
		relativePath := strings.TrimPrefix(filePath, searchPath)
		if relativePath == "" {
			continue
		}
		
		// Get the first segment (file or directory name)
		segments := strings.Split(relativePath, "/")
		firstSegment := segments[0]
		
		if seen[firstSegment] {
			continue
		}
		seen[firstSegment] = true
		
		if len(segments) == 1 {
			// It's a file
			entries.Files = append(entries.Files, firstSegment)
		} else {
			// It's a directory
			entries.Directories = append(entries.Directories, firstSegment)
		}
	}
	
	return entries
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

func typecheckTypeScript(code string) TypecheckResponse {
	memFS := newMemoryFS()
	
	// Always use .tsx to support JSX
	fileName := "/input.tsx"
	
	memFS.files[fileName] = code
	
	fs := bundled.WrapFS(memFS)
	
	// Create minimal compiler options (matching CrayonDeveloper settings)
	jsxImportSource := "react"
	compilerOptions := &core.CompilerOptions{
		AllowJs:                          core.TSTrue,
		Declaration:                      core.TSTrue,
		ESModuleInterop:                  core.TSTrue,
		ForceConsistentCasingInFileNames: core.TSTrue,
		IsolatedModules:                  core.TSTrue,
		Jsx:                              core.JsxEmitReactJSX,
		JsxImportSource:                  jsxImportSource,
		Module:                           core.ModuleKindCommonJS,
		ModuleResolution:                 core.ModuleResolutionKindBundler,
		NoEmit:                           core.TSTrue,
		ResolveJsonModule:                core.TSTrue,
		SkipLibCheck:                     core.TSTrue,
		Strict:                           core.TSTrue,
		StrictNullChecks:                 core.TSTrue,
		Target:                           core.ScriptTargetES2022,
		Lib:                              []string{"ES2022"},
	}
	
	// Create parsed options
	parsedOptions := &core.ParsedOptions{
		CompilerOptions: compilerOptions,
		FileNames:       []string{fileName},
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
		return TypecheckResponse{Errors: errors}
	}
	
	return TypecheckResponse{Pass: true}
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "TypeScript Go Server\n")
}

func typecheck(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var typecheckReq TypecheckRequest
	if err := json.NewDecoder(req.Body).Decode(&typecheckReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if typecheckReq.Code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	response := typecheckTypeScript(typecheckReq.Code)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/typecheck", typecheck)
	fmt.Println("Listening on :8080...")
	http.ListenAndServe(":8080", nil)
}