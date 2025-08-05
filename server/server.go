package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs"
)

var (
	serverVersion = "1.0.0"
	startTime     = time.Now()
	moduleStats   = ModuleStats{}
	globalMemFS   *memoryFS
)

type ModuleStats struct {
	TotalFiles      int
	TypeDefinitions int
	JavaScriptFiles int
	PackageFiles    int
	LoadErrors      int
}

type TypecheckRequest struct {
	Code string `json:"code"`
}

type TypecheckResponse struct {
	Pass   bool              `json:"pass,omitempty"`
	Errors []DiagnosticError `json:"errors,omitempty"`
}

type BuildRequest struct {
	Code string `json:"code"`
}

type BuildResponse struct {
	Code   string            `json:"code,omitempty"`
	Errors []DiagnosticError `json:"errors,omitempty"`
}

type DiagnosticError struct {
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type HealthResponse struct {
	Status         string                 `json:"status"`
	Version        string                 `json:"version"`
	Uptime         string                 `json:"uptime"`
	Modules        ModuleStats            `json:"modules"`
	Dependencies   map[string]string      `json:"dependencies"`
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
	log.Println("Loading type definitions...")
	
	// Walk the node_modules directory and load files
	nodeModulesDir := "/node_modules"
	if _, err := os.Stat(nodeModulesDir); os.IsNotExist(err) {
		log.Printf("Node modules not found at %s, trying local path...", nodeModulesDir)
		// Try local development path
		nodeModulesDir = "./node_modules"
		if _, err := os.Stat(nodeModulesDir); os.IsNotExist(err) {
			log.Printf("ERROR: No node_modules found at %s either", nodeModulesDir)
			return
		}
	}
	
	log.Printf("Loading modules from: %s", nodeModulesDir)
	
	stats := ModuleStats{}
	
	err := filepath.WalkDir(nodeModulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			stats.LoadErrors++
			log.Printf("WARNING: Error walking %s: %v", path, err)
			return nil // Skip errors
		}
		
		if d.IsDir() {
			return nil
		}
		
		// Load .d.ts files, .js files, and package.json files
		if strings.HasSuffix(path, ".d.ts") || 
		   strings.HasSuffix(path, ".js") || 
		   strings.HasSuffix(path, ".jsx") ||
		   strings.HasSuffix(path, ".mjs") ||
		   strings.HasSuffix(path, "package.json") {
			
			// Skip TypeScript compiler files to save space
			if strings.Contains(path, "/typescript/lib/") && strings.HasSuffix(path, ".js") {
				log.Printf("Skipping TypeScript compiler file: %s", path)
				return nil
			}
			
			// Convert file system path to virtual path
			virtualPath := path
			if strings.HasPrefix(virtualPath, "./") {
				virtualPath = virtualPath[2:] // Remove ./
			}
			if !strings.HasPrefix(virtualPath, "/") {
				virtualPath = "/" + virtualPath
			}
			virtualPath = strings.ReplaceAll(virtualPath, "\\", "/")
			
			content, err := os.ReadFile(path)
			if err != nil {
				stats.LoadErrors++
				log.Printf("WARNING: Failed to read %s: %v", path, err)
				return nil
			}
			
			memFS.files[virtualPath] = string(content)
			stats.TotalFiles++
			
			if strings.HasSuffix(path, ".d.ts") {
				stats.TypeDefinitions++
			} else if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx") || strings.HasSuffix(path, ".mjs") {
				stats.JavaScriptFiles++
			} else if strings.HasSuffix(path, "package.json") {
				stats.PackageFiles++
			}
			
			// Log every 100 files to show progress
			if stats.TotalFiles%100 == 0 {
				log.Printf("Progress: loaded %d files...", stats.TotalFiles)
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("ERROR: Failed to walk node_modules: %v", err)
	}
	
	moduleStats = stats
	log.Printf("Module loading complete: %d total files (%d .d.ts, %d .js, %d package.json), %d errors", 
		stats.TotalFiles, stats.TypeDefinitions, stats.JavaScriptFiles, stats.PackageFiles, stats.LoadErrors)
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
	// Clone the global memFS to avoid concurrent modification issues
	memFS := &memoryFS{
		files: make(map[string]string, len(globalMemFS.files)),
	}
	for k, v := range globalMemFS.files {
		memFS.files[k] = v
	}
	
	// Always use .tsx to support JSX
	fileName := "/input.tsx"
	
	memFS.files[fileName] = code
	
	fs := bundled.WrapFS(memFS)
	
	// Create minimal compiler options (matching CrayonDeveloper settings)
	jsxImportSource := "@crayonnow/core"
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

func buildTypeScript(code string) BuildResponse {
	// Clone the global memFS to avoid concurrent modification issues
	memFS := &memoryFS{
		files: make(map[string]string, len(globalMemFS.files)),
	}
	for k, v := range globalMemFS.files {
		memFS.files[k] = v
	}
	
	// Always use .tsx to support JSX
	fileName := "/input.tsx"
	
	memFS.files[fileName] = code
	
	// Create virtual file resolver for esbuild
	resolver := func(path string) (api.OnLoadResult, error) {
		log.Printf("Resolving: %s", path)
		
		// First try exact path
		if content, exists := memFS.files[path]; exists {
			loader := api.LoaderDefault
			if strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts") {
				loader = api.LoaderTSX
			} else if strings.HasSuffix(path, ".jsx") {
				loader = api.LoaderJSX
			} else if strings.HasSuffix(path, ".json") {
				loader = api.LoaderJSON
			}
			log.Printf("Found exact path: %s", path)
			return api.OnLoadResult{
				Contents: &content,
				Loader:   loader,
			}, nil
		}
		
		// Handle absolute paths that may be missing extension
		if strings.HasPrefix(path, "/") {
			extensions := []string{".js", ".jsx", ".mjs", ".json", ".ts", ".tsx"}
			for _, ext := range extensions {
				testPath := path + ext
				if content, exists := memFS.files[testPath]; exists {
					loader := api.LoaderDefault
					if ext == ".tsx" || ext == ".ts" {
						loader = api.LoaderTSX
					} else if ext == ".jsx" {
						loader = api.LoaderJSX
					} else if ext == ".json" {
						loader = api.LoaderJSON
					}
					log.Printf("Found with extension: %s", testPath)
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loader,
					}, nil
				}
			}
		}
		
		// Try to resolve as node_modules
		if !strings.HasPrefix(path, "/") {
			// Split the path to handle scoped packages and subpaths
			parts := strings.Split(path, "/")
			packageName := parts[0]
			if strings.HasPrefix(packageName, "@") && len(parts) > 1 {
				// Scoped package like @crayonnow/core
				packageName = parts[0] + "/" + parts[1]
			}
			
			// Try various module resolution patterns
			patterns := []string{}
			
			// Check package.json for main/exports field
			pkgPath := "/node_modules/" + packageName + "/package.json"
			if pkgContent, exists := memFS.files[pkgPath]; exists {
				var pkg map[string]interface{}
				if err := json.Unmarshal([]byte(pkgContent), &pkg); err == nil {
					// Handle exports field
					if exports, ok := pkg["exports"].(map[string]interface{}); ok {
						// Look for the exact export path or default
						subpath := "./" + strings.Join(parts[len(strings.Split(packageName, "/")):], "/")
						if subpath == "./" {
							subpath = "."
						}
						
						if exportPath, ok := exports[subpath]; ok {
							if str, ok := exportPath.(string); ok {
								// Handle both string exports and object exports
								patterns = append(patterns, "/node_modules/" + packageName + "/" + strings.TrimPrefix(str, "./"))
							} else if obj, ok := exportPath.(map[string]interface{}); ok {
								// Handle conditional exports like { "import": "./dist/index.js" }
								if imp, ok := obj["import"].(string); ok {
									patterns = append(patterns, "/node_modules/" + packageName + "/" + strings.TrimPrefix(imp, "./"))
								}
								if def, ok := obj["default"].(string); ok {
									patterns = append(patterns, "/node_modules/" + packageName + "/" + strings.TrimPrefix(def, "./"))
								}
							}
						}
						
						// Try default export
						if def, ok := exports["."].(string); ok && subpath == "." {
							patterns = append(patterns, "/node_modules/" + packageName + "/" + strings.TrimPrefix(def, "./"))
						} else if defObj, ok := exports["."].(map[string]interface{}); ok && subpath == "." {
							// Handle conditional exports for default
							if imp, ok := defObj["import"].(string); ok {
								patterns = append(patterns, "/node_modules/" + packageName + "/" + strings.TrimPrefix(imp, "./"))
							}
							if def, ok := defObj["default"].(string); ok {
								patterns = append(patterns, "/node_modules/" + packageName + "/" + strings.TrimPrefix(def, "./"))
							}
						}
					}
					
					// Handle main field
					if main, ok := pkg["main"].(string); ok {
						mainPath := "/node_modules/" + packageName + "/" + main
						if len(parts) > len(strings.Split(packageName, "/")) {
							// Subpath import
							subpath := strings.Join(parts[len(strings.Split(packageName, "/")):], "/")
							mainPath = "/node_modules/" + packageName + "/" + subpath
						}
						patterns = append(patterns, mainPath)
					}
				}
			}
			
			// Add default patterns
			if len(parts) > len(strings.Split(packageName, "/")) {
				// Subpath import like @crayonnow/core/jsx-runtime
				subpath := strings.Join(parts[len(strings.Split(packageName, "/")):], "/")
				patterns = append(patterns,
					"/node_modules/"+packageName+"/"+subpath,
					"/node_modules/"+packageName+"/"+subpath+".js",
					"/node_modules/"+packageName+"/"+subpath+".jsx",
					"/node_modules/"+packageName+"/"+subpath+".mjs",
					"/node_modules/"+packageName+"/"+subpath+"/index.js",
				)
			} else {
				// Main package import
				patterns = append(patterns,
					"/node_modules/"+path,
					"/node_modules/"+path+"/index.js",
					"/node_modules/"+path+"/index.jsx",
					"/node_modules/"+path+"/index.mjs",
				)
			}
			
			for _, pattern := range patterns {
				if content, exists := memFS.files[pattern]; exists {
					loader := api.LoaderDefault
					if strings.HasSuffix(pattern, ".jsx") {
						loader = api.LoaderJSX
					} else if strings.HasSuffix(pattern, ".json") {
						loader = api.LoaderJSON
					}
					log.Printf("Found at: %s", pattern)
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loader,
					}, nil
				}
			}
			
			log.Printf("Tried patterns: %v", patterns)
		}
		
		return api.OnLoadResult{}, fmt.Errorf("file not found: %s", path)
	}
	
	// Build with esbuild (matching Swift configuration)
	result := api.Build(api.BuildOptions{
		EntryPoints:        []string{fileName},
		Bundle:             true,
		Format:             api.FormatCommonJS,
		JSXFactory:         "_CRAYONCORE_$REACT.createElement",
		JSXFragment:        "_CRAYONCORE_$REACT.Fragment",
		MinifyWhitespace:   true,
		MinifyIdentifiers:  true,
		MinifySyntax:       true,
		Platform:           api.PlatformBrowser,
		Target:             api.ES2022,
		Write:              false,
		External:           []string{"*"},
		Plugins: []api.Plugin{{
			Name: "virtual-fs",
			Setup: func(pb api.PluginBuild) {
				pb.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					// Transform react imports to use global variable
					if args.Path == "react" {
						return api.OnResolveResult{
							Path:      "react",
							Namespace: "use-crayon-react-global",
						}, nil
					}
					
					// Handle absolute imports
					if strings.HasPrefix(args.Path, "/") {
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}
					
					// Handle relative imports
					if strings.HasPrefix(args.Path, "./") || strings.HasPrefix(args.Path, "../") {
						importerDir := filepath.Dir(args.Importer)
						resolvedPath := filepath.Join(importerDir, args.Path)
						resolvedPath = strings.ReplaceAll(resolvedPath, "\\", "/")
						return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
					}
					
					// Handle bare imports (no relative path)
					if !strings.Contains(args.Path, "/") || strings.HasPrefix(args.Path, "@") {
						// This is a node_modules import
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}
					
					// Handle subpath imports that aren't relative (like "cjs/react.production.js")
					// These are relative to the importer's package
					if args.Importer != "" && strings.Contains(args.Importer, "/node_modules/") {
						// Extract the package path from the importer
						parts := strings.Split(args.Importer, "/node_modules/")
						if len(parts) >= 2 {
							// Find the package name
							remainingPath := parts[1]
							packageParts := strings.Split(remainingPath, "/")
							packageName := packageParts[0]
							if strings.HasPrefix(packageName, "@") && len(packageParts) > 1 {
								packageName = packageParts[0] + "/" + packageParts[1]
							}
							// Resolve relative to package root
							resolvedPath := "/node_modules/" + packageName + "/" + args.Path
							return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
						}
					}
					
					// Default: treat as node_modules import
					return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
				})
				
				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "virtual"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					return resolver(args.Path)
				})
				
				// Handle react global transform
				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "use-crayon-react-global"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					contents := "module.exports = _CRAYONCORE_$REACT"
					return api.OnLoadResult{
						Contents: &contents,
						Loader:   api.LoaderJS,
					}, nil
				})
			},
		}},
	})
	
	if len(result.Errors) > 0 {
		errors := make([]DiagnosticError, 0, len(result.Errors))
		for _, err := range result.Errors {
			diagErr := DiagnosticError{
				Message: err.Text,
			}
			if err.Location != nil {
				diagErr.Line = err.Location.Line
				diagErr.Column = err.Location.Column
			}
			errors = append(errors, diagErr)
		}
		return BuildResponse{Errors: errors}
	}
	
	if len(result.OutputFiles) == 0 {
		return BuildResponse{Errors: []DiagnosticError{{Message: "No output generated"}}}
	}
	
	return BuildResponse{Code: string(result.OutputFiles[0].Contents)}
}

// Middleware for request logging
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a custom response writer to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Call the next handler
		next(lrw, r)
		
		// Log the request
		duration := time.Since(start)
		log.Printf("%s %s - %d - %v", r.Method, r.URL.Path, lrw.statusCode, duration)
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func getPackageVersions() map[string]string {
	versions := make(map[string]string)
	
	// Read package versions from globalMemFS
	if globalMemFS != nil {
		// Check @crayonnow/core
		if pkgContent, exists := globalMemFS.files["/node_modules/@crayonnow/core/package.json"]; exists {
			var pkg map[string]interface{}
			if err := json.Unmarshal([]byte(pkgContent), &pkg); err == nil {
				if version, ok := pkg["version"].(string); ok {
					versions["@crayonnow/core"] = version
				}
			}
		}
		
		// Check react
		if pkgContent, exists := globalMemFS.files["/node_modules/react/package.json"]; exists {
			var pkg map[string]interface{}
			if err := json.Unmarshal([]byte(pkgContent), &pkg); err == nil {
				if version, ok := pkg["version"].(string); ok {
					versions["react"] = version
				}
			}
		}
		
		// Check typescript
		if pkgContent, exists := globalMemFS.files["/node_modules/typescript/package.json"]; exists {
			var pkg map[string]interface{}
			if err := json.Unmarshal([]byte(pkgContent), &pkg); err == nil {
				if version, ok := pkg["version"].(string); ok {
					versions["typescript"] = version
				}
			}
		}
	}
	
	return versions
}

func health(w http.ResponseWriter, req *http.Request) {
	uptime := time.Since(startTime)
	
	response := HealthResponse{
		Status:       "healthy",
		Version:      serverVersion,
		Uptime:       fmt.Sprintf("%v", uptime.Round(time.Second)),
		Modules:      moduleStats,
		Dependencies: getPackageVersions(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func hello(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	fmt.Fprintf(w, "TypeScript Go Server v%s\nUptime: %v\nModules loaded: %d\n", 
		serverVersion, time.Since(startTime).Round(time.Second), moduleStats.TotalFiles)
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

func build(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var buildReq BuildRequest
	if err := json.NewDecoder(req.Body).Decode(&buildReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if buildReq.Code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	response := buildTypeScript(buildReq.Code)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	log.Printf("TypeScript Go Server v%s starting...", serverVersion)
	
	// Initialize module loading before serving requests
	log.Println("Initializing server...")
	globalMemFS = newMemoryFS() // Load modules once at startup
	
	// Set up routes with logging middleware
	http.HandleFunc("/health", loggingMiddleware(health))
	http.HandleFunc("/typecheck", loggingMiddleware(typecheck))
	http.HandleFunc("/build", loggingMiddleware(build))
	http.HandleFunc("/", loggingMiddleware(hello))
	
	log.Printf("Server ready! Listening on :8080...")
	log.Printf("Endpoints: /, /health, /typecheck, /build")
	
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}