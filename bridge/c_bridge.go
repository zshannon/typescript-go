package main

/*
#include <stdlib.h>
#include <string.h>

typedef struct {
    int code;
    char* category;
    char* message;
    char* file;
    int line;
    int column;
    int length;
} c_diagnostic;

typedef struct {
    int success;
    char* config_file;
    c_diagnostic* diagnostics;
    int diagnostic_count;
    char** emitted_files;
    int emitted_file_count;
    char** written_file_paths;
    char** written_file_contents;
    int written_file_count;
} c_build_result;

typedef struct {
    char* path;
    char* content;
} c_file_entry;

typedef struct {
    c_file_entry* files;
    int file_count;
    char** directories;
    int directory_count;
} c_file_resolver_data;

// Dynamic file resolver callback types (like ESBuild plugin callbacks)
typedef struct {
    char* path;
    int path_length;
} c_file_resolve_args;

typedef struct {
    char* content;
    int content_length;
    int exists;  // 0 = not found, 1 = file, 2 = directory
    char** directory_files;  // for directories
    int directory_files_count;
} c_file_resolve_result;

// Callback function pointer (like plugin callbacks)
typedef c_file_resolve_result* (*file_resolve_callback)(c_file_resolve_args*, void*);

// Resolver callbacks structure (like plugin system)
typedef struct {
    file_resolve_callback resolver;
    void* resolver_data;  // Swift callback context
} c_resolver_callbacks;

// Helper function to call function pointer (needed for CGO)
static inline c_file_resolve_result* call_file_resolve_callback(file_resolve_callback cb, c_file_resolve_args* args, void* data) {
    return cb(args, data);
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnosticwriter"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs"
	"github.com/microsoft/typescript-go/internal/vfs/osvfs"
)

// FileResolver interface for custom file resolution
type FileResolver interface {
	ResolveFile(path string) string
	FileExists(path string) bool
	DirectoryExists(path string) bool
	WriteFile(path string, content string) bool
	GetAllPaths(directory string) *PathList
}

// PathList represents a list of paths
type PathList struct {
	Paths []string
}

// SimpleFileResolver implements FileResolver for basic use cases
type SimpleFileResolver struct {
	files       map[string]string
	directories map[string]bool
	mu          sync.RWMutex
}

func NewSimpleFileResolver() *SimpleFileResolver {
	return &SimpleFileResolver{
		files:       make(map[string]string),
		directories: make(map[string]bool),
	}
}

func (r *SimpleFileResolver) AddFile(path, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[path] = content
}

func (r *SimpleFileResolver) AddDirectory(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.directories[path] = true
}

func (r *SimpleFileResolver) ResolveFile(path string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files[path]
}

func (r *SimpleFileResolver) FileExists(path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.files[path]
	return exists
}

func (r *SimpleFileResolver) DirectoryExists(path string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.directories[path]
}

func (r *SimpleFileResolver) WriteFile(path string, content string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[path] = content
	return true
}

func (r *SimpleFileResolver) GetAllPaths(directory string) *PathList {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var paths []string
	for path := range r.files {
		if strings.HasPrefix(path, directory) {
			paths = append(paths, path)
		}
	}
	for path := range r.directories {
		if strings.HasPrefix(path, directory) {
			paths = append(paths, path)
		}
	}
	return &PathList{Paths: paths}
}

// FileResolverC implements FileResolver interface for C bridge
type FileResolverC struct {
	data *C.c_file_resolver_data
}

func (f *FileResolverC) ResolveFile(path string) string {
	if f.data == nil {
		return ""
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	fileSlice := (*[1 << 28]C.c_file_entry)(unsafe.Pointer(f.data.files))[:f.data.file_count:f.data.file_count]
	for i := 0; i < int(f.data.file_count); i++ {
		if C.strcmp(fileSlice[i].path, cPath) == 0 {
			return C.GoString(fileSlice[i].content)
		}
	}
	return ""
}

func (f *FileResolverC) FileExists(path string) bool {
	return f.ResolveFile(path) != ""
}

func (f *FileResolverC) DirectoryExists(path string) bool {
	if f.data == nil {
		return false
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	dirSlice := (*[1 << 28]*C.char)(unsafe.Pointer(f.data.directories))[:f.data.directory_count:f.data.directory_count]
	for i := 0; i < int(f.data.directory_count); i++ {
		if C.strcmp(dirSlice[i], cPath) == 0 {
			return true
		}
	}
	return false
}

func (f *FileResolverC) WriteFile(path string, content string) bool {
	return true
}

func (f *FileResolverC) GetAllPaths(directory string) *PathList {
	if f.data == nil {
		return &PathList{Paths: []string{}}
	}

	// Normalize directory path to ensure consistent matching
	normalizedDir := directory
	if !strings.HasSuffix(normalizedDir, "/") {
		normalizedDir += "/"
	}

	childNames := make(map[string]bool) // Use map to avoid duplicates

	// Check files
	fileSlice := (*[1 << 28]C.c_file_entry)(unsafe.Pointer(f.data.files))[:f.data.file_count:f.data.file_count]
	for i := 0; i < int(f.data.file_count); i++ {
		filePath := C.GoString(fileSlice[i].path)
		if strings.HasPrefix(filePath, normalizedDir) {
			// Get the relative path after the directory
			relativePath := filePath[len(normalizedDir):]
			// Get only the immediate child (first segment)
			if relativePath != "" {
				parts := strings.Split(relativePath, "/")
				if len(parts) > 0 && parts[0] != "" {
					childNames[parts[0]] = true
				}
			}
		}
	}

	// Check directories
	dirSlice := (*[1 << 28]*C.char)(unsafe.Pointer(f.data.directories))[:f.data.directory_count:f.data.directory_count]
	for i := 0; i < int(f.data.directory_count); i++ {
		dirPath := C.GoString(dirSlice[i])
		if strings.HasPrefix(dirPath, normalizedDir) {
			// Get the relative path after the directory
			relativePath := dirPath[len(normalizedDir):]
			// Get only the immediate child (first segment)
			if relativePath != "" {
				// Remove trailing slash if present
				relativePath = strings.TrimSuffix(relativePath, "/")
				parts := strings.Split(relativePath, "/")
				if len(parts) > 0 && parts[0] != "" {
					childNames[parts[0]] = true
				}
			}
		}
	}

	// Convert map keys to slice
	paths := make([]string, 0, len(childNames))
	for name := range childNames {
		paths = append(paths, name)
	}

	return &PathList{Paths: paths}
}

// BridgeResult contains the result of a TypeScript compilation
type BridgeResult struct {
	Success      bool
	ConfigFile   string
	Diagnostics  []BridgeDiagnostic
	EmittedFiles []string
	WrittenFiles map[string]string
}

// BridgeDiagnostic contains diagnostic information
type BridgeDiagnostic struct {
	Code     int
	Category string
	Message  string
	File     string
	Line     int
	Column   int
	Length   int
}

// callbackVFS implements vfs.FS using a FileResolver
type callbackVFS struct {
	resolver     FileResolver
	osvfs        vfs.FS
	writtenFiles map[string]string
	mu           sync.RWMutex
}

func newCallbackVFS(resolver FileResolver) *callbackVFS {
	return &callbackVFS{
		resolver:     resolver,
		osvfs:        osvfs.FS(),
		writtenFiles: make(map[string]string),
	}
}

func (c *callbackVFS) UseCaseSensitiveFileNames() bool {
	return c.osvfs.UseCaseSensitiveFileNames()
}

func (c *callbackVFS) FileExists(path string) bool {
	return c.resolver.FileExists(path)
}

func (c *callbackVFS) ReadFile(path string) (contents string, ok bool) {
	c.mu.RLock()
	writtenContent, exists := c.writtenFiles[path]
	c.mu.RUnlock()

	if exists {
		return writtenContent, true
	}
	contents = c.resolver.ResolveFile(path)
	return contents, contents != ""
}

func (c *callbackVFS) WriteFile(path string, data string, writeByteOrderMark bool) error {
	if c.resolver.WriteFile(path, data) {
		c.mu.Lock()
		c.writtenFiles[path] = data
		c.mu.Unlock()
		return nil
	}
	return c.osvfs.WriteFile(path, data, writeByteOrderMark)
}

func (c *callbackVFS) Remove(path string) error {
	c.mu.Lock()
	delete(c.writtenFiles, path)
	c.mu.Unlock()
	return c.osvfs.Remove(path)
}

func (c *callbackVFS) DirectoryExists(path string) bool {
	return c.resolver.DirectoryExists(path)
}

func (c *callbackVFS) GetAccessibleEntries(path string) vfs.Entries {
	var files []string
	var directories []string

	pathList := c.resolver.GetAllPaths(path)
	if pathList != nil {
		for _, childName := range pathList.Paths {
			if childName == "" {
				continue
			}

			childPath := path
			if !strings.HasSuffix(path, "/") {
				childPath += "/"
			}
			childPath += childName

			if c.resolver.FileExists(childPath) {
				files = append(files, childName)
			} else if c.resolver.DirectoryExists(childPath) {
				directories = append(directories, childName)
			}
		}
	}

	return vfs.Entries{
		Files:       files,
		Directories: directories,
	}
}

func (c *callbackVFS) Stat(path string) vfs.FileInfo {
	return c.osvfs.Stat(path)
}

func (c *callbackVFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error {
	for filePath := range c.getAllKnownPaths(root) {
		if strings.HasPrefix(filePath, root) {
			if c.resolver.FileExists(filePath) {
				entry := &simpleDirEntry{
					name:  filepath.Base(filePath),
					isDir: false,
				}

				err := walkFn(filePath, entry, nil)
				if err != nil {
					if err == filepath.SkipDir {
						continue
					}
					return err
				}
			}
		}
	}
	return nil
}

func (c *callbackVFS) Realpath(path string) string {
	if c.resolver.FileExists(path) || c.resolver.DirectoryExists(path) {
		return path
	}
	return c.osvfs.Realpath(path)
}

func (c *callbackVFS) getAllKnownPaths(directory string) map[string]bool {
	paths := make(map[string]bool)

	for path := range c.writtenFiles {
		if strings.HasPrefix(path, directory) {
			paths[path] = true
		}
	}

	pathList := c.resolver.GetAllPaths(directory)
	if pathList != nil {
		for _, foundPath := range pathList.Paths {
			paths[foundPath] = true
		}
	}

	return paths
}

type simpleDirEntry struct {
	name  string
	isDir bool
}

func (e *simpleDirEntry) Name() string { return e.name }
func (e *simpleDirEntry) IsDir() bool  { return e.isDir }
func (e *simpleDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e *simpleDirEntry) Info() (fs.FileInfo, error) {
	return &simpleFileInfo{name: e.name, isDir: e.isDir}, nil
}

type simpleFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i *simpleFileInfo) Name() string       { return i.name }
func (i *simpleFileInfo) Size() int64        { return i.size }
func (i *simpleFileInfo) Mode() fs.FileMode  { return 0644 }
func (i *simpleFileInfo) ModTime() time.Time { return time.Time{} }
func (i *simpleFileInfo) IsDir() bool        { return i.isDir }
func (i *simpleFileInfo) Sys() interface{}   { return nil }

type bridgeSystem struct {
	writer             io.Writer
	fs                 vfs.FS
	defaultLibraryPath string
	newLine            string
	cwd                string
	start              time.Time
	customFS           vfs.FS
	callbackVFS        *callbackVFS
}

func (s *bridgeSystem) SinceStart() time.Duration { return time.Since(s.start) }
func (s *bridgeSystem) Now() time.Time            { return time.Now() }
func (s *bridgeSystem) FS() vfs.FS {
	if s.customFS != nil {
		return s.customFS
	}
	return s.fs
}
func (s *bridgeSystem) DefaultLibraryPath() string  { return s.defaultLibraryPath }
func (s *bridgeSystem) GetCurrentDirectory() string { return s.cwd }
func (s *bridgeSystem) NewLine() string             { return s.newLine }
func (s *bridgeSystem) Writer() io.Writer           { return s.writer }
func (s *bridgeSystem) EndWrite()                   {}

func newBridgeSystem() *bridgeSystem {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	libPath := bundled.LibPath()

	return &bridgeSystem{
		cwd:                tspath.NormalizePath(cwd),
		fs:                 bundled.WrapFS(osvfs.FS()),
		defaultLibraryPath: libPath,
		writer:             os.Stdout,
		newLine:            core.IfElse(runtime.GOOS == "windows", "\r\n", "\n"),
		start:              time.Now(),
	}
}

func newBridgeSystemWithResolver(resolver FileResolver) *bridgeSystem {
	sys := newBridgeSystem()
	if resolver != nil {
		sys.callbackVFS = newCallbackVFS(resolver)
		sys.customFS = bundled.WrapFS(sys.callbackVFS)
	}
	return sys
}

func buildWithConfig(projectPath string, printErrors bool, configFile string, resolver FileResolver) (*BridgeResult, error) {
	var sys *bridgeSystem
	if resolver != nil {
		sys = newBridgeSystemWithResolver(resolver)
	} else {
		sys = newBridgeSystem()
	}

	if printErrors {
		sys.writer = os.Stdout
	} else {
		sys.writer = io.Discard
	}

	configPath := configFile
	if configPath == "" {
		configPath = projectPath
	}

	commandLine := tsoptions.ParseCommandLine([]string{"-p", configPath}, sys)

	if len(commandLine.Errors) > 0 {
		return &BridgeResult{
			Success:     false,
			Diagnostics: convertASTDiagnostics(commandLine.Errors),
		}, nil
	}

	var configFileName string
	compilerOptions := commandLine.CompilerOptions()

	if compilerOptions.Project != "" {
		fileOrDirectory := tspath.NormalizePath(compilerOptions.Project)
		if sys.FS().DirectoryExists(fileOrDirectory) {
			configFileName = tspath.CombinePaths(fileOrDirectory, "tsconfig.json")
			if !sys.FS().FileExists(configFileName) {
				return &BridgeResult{
					Success: false,
					Diagnostics: []BridgeDiagnostic{{
						Code:     0,
						Category: "error",
						Message:  fmt.Sprintf("cannot find a tsconfig.json file at: %s", configFileName),
					}},
				}, nil
			}
		} else {
			configFileName = fileOrDirectory
			if !sys.FS().FileExists(configFileName) {
				return &BridgeResult{
					Success: false,
					Diagnostics: []BridgeDiagnostic{{
						Code:     0,
						Category: "error",
						Message:  fmt.Sprintf("the specified path does not exist: %s", fileOrDirectory),
					}},
				}, nil
			}
		}
	}

	if configFileName == "" {
		return &BridgeResult{
			Success: false,
			Diagnostics: []BridgeDiagnostic{{
				Code:     0,
				Category: "error",
				Message:  "no tsconfig.json file found",
			}},
		}, nil
	}

	extendedConfigCache := collections.SyncMap[tspath.Path, *tsoptions.ExtendedConfigCacheEntry]{}
	configParseResult, parseErrors := tsoptions.GetParsedCommandLineOfConfigFile(configFileName, compilerOptions, sys, &extendedConfigCache)

	if len(parseErrors) != 0 {
		return &BridgeResult{
			Success:     false,
			Diagnostics: convertASTDiagnostics(parseErrors),
			ConfigFile:  configFileName,
		}, nil
	}

	host := compiler.NewCachedFSCompilerHost(configParseResult.CompilerOptions(), sys.GetCurrentDirectory(), sys.FS(), sys.DefaultLibraryPath(), &extendedConfigCache)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           configParseResult,
		Host:             host,
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})

	ctx := context.Background()
	options := program.Options()
	allDiagnostics := slices.Clip(program.GetConfigFileParsingDiagnostics())
	configFileParsingDiagnosticsLength := len(allDiagnostics)

	allDiagnostics = append(allDiagnostics, program.GetSyntacticDiagnostics(ctx, nil)...)

	if len(allDiagnostics) == configFileParsingDiagnosticsLength {
		_ = program.GetBindDiagnostics(ctx, nil)
		allDiagnostics = append(allDiagnostics, program.GetOptionsDiagnostics(ctx)...)

		if options.ListFilesOnly.IsFalseOrUnknown() {
			allDiagnostics = append(allDiagnostics, program.GetGlobalDiagnostics(ctx)...)

			if len(allDiagnostics) == configFileParsingDiagnosticsLength {
				allDiagnostics = append(allDiagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
			}
		}

		if options.NoEmit.IsTrue() && options.GetEmitDeclarations() && len(allDiagnostics) == configFileParsingDiagnosticsLength {
			allDiagnostics = append(allDiagnostics, program.GetDeclarationDiagnostics(ctx, nil)...)
		}
	}

	var emitResult *compiler.EmitResult
	var emittedFiles []string
	if !options.ListFilesOnly.IsTrue() {
		emitResult = program.Emit(compiler.EmitOptions{})
		allDiagnostics = append(allDiagnostics, emitResult.Diagnostics...)
		emittedFiles = emitResult.EmittedFiles

		if sys.callbackVFS != nil {
			for path := range sys.callbackVFS.writtenFiles {
				found := false
				for _, existing := range emittedFiles {
					if existing == path {
						found = true
						break
					}
				}
				if !found {
					emittedFiles = append(emittedFiles, path)
				}
			}
		}
	}

	allDiagnostics = compiler.SortAndDeduplicateDiagnostics(allDiagnostics)
	diagnostics := convertASTDiagnostics(allDiagnostics)

	if printErrors && len(allDiagnostics) > 0 {
		for _, diag := range allDiagnostics {
			formatOpts := &diagnosticwriter.FormattingOptions{NewLine: "\n"}
			diagnosticwriter.WriteFormatDiagnostic(sys.Writer(), diag, formatOpts)
		}
	}

	success := true
	for _, diag := range diagnostics {
		if diag.Category == "error" {
			success = false
			break
		}
	}

	result := &BridgeResult{
		Success:      success,
		Diagnostics:  diagnostics,
		EmittedFiles: emittedFiles,
		ConfigFile:   configFileName,
	}

	if sys.callbackVFS != nil {
		result.WrittenFiles = make(map[string]string)
		sys.callbackVFS.mu.RLock()
		for path, content := range sys.callbackVFS.writtenFiles {
			result.WrittenFiles[path] = content
		}
		sys.callbackVFS.mu.RUnlock()
	}

	return result, nil
}

func convertASTDiagnostics(diagnostics []*ast.Diagnostic) []BridgeDiagnostic {
	result := make([]BridgeDiagnostic, len(diagnostics))
	for i, diag := range diagnostics {
		result[i] = BridgeDiagnostic{
			Code:     int(diag.Code()),
			Category: diag.Category().Name(),
			Message:  diag.Message(),
		}

		if diag.File() != nil {
			result[i].File = diag.File().FileName()
			if diag.Loc().Pos() >= 0 {
				line, column := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
				result[i].Line = line + 1
				result[i].Column = column + 1
				result[i].Length = diag.Loc().End() - diag.Loc().Pos()
			}
		}
	}
	return result
}

func calculateLineColumn(text string, pos int) (line, column int) {
	if pos < 0 || pos > len(text) {
		return 0, 0
	}

	line = 0
	column = 0
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

//export tsc_build_filesystem
func tsc_build_filesystem(projectPath *C.char, printErrors C.int, configFile *C.char) *C.c_build_result {
	goProjectPath := C.GoString(projectPath)
	goPrintErrors := printErrors != 0
	goConfigFile := C.GoString(configFile)

	result, err := buildWithConfig(goProjectPath, goPrintErrors, goConfigFile, nil)
	if err != nil {
		cResult := (*C.c_build_result)(C.malloc(C.sizeof_c_build_result))
		cResult.success = 0
		cResult.config_file = C.CString("error: " + err.Error())
		cResult.diagnostics = nil
		cResult.diagnostic_count = 0
		cResult.emitted_files = nil
		cResult.emitted_file_count = 0
		cResult.written_file_paths = nil
		cResult.written_file_contents = nil
		cResult.written_file_count = 0
		return cResult
	}

	return convertBridgeResultToC(result)
}

//export tsc_build_with_resolver
func tsc_build_with_resolver(projectPath *C.char, printErrors C.int, configFile *C.char, resolverData *C.c_file_resolver_data) *C.c_build_result {
	goProjectPath := C.GoString(projectPath)
	goPrintErrors := printErrors != 0
	goConfigFile := C.GoString(configFile)

	resolver := &FileResolverC{data: resolverData}

	result, err := buildWithConfig(goProjectPath, goPrintErrors, goConfigFile, resolver)
	if err != nil {
		cResult := (*C.c_build_result)(C.malloc(C.sizeof_c_build_result))
		cResult.success = 0
		cResult.config_file = C.CString("error: " + err.Error())
		cResult.diagnostics = nil
		cResult.diagnostic_count = 0
		cResult.emitted_files = nil
		cResult.emitted_file_count = 0
		cResult.written_file_paths = nil
		cResult.written_file_contents = nil
		cResult.written_file_count = 0
		return cResult
	}

	return convertBridgeResultToC(result)
}

// Dynamic FileResolver that uses callbacks (like ESBuild plugins)
type FileResolverDynamic struct {
	callbacks *C.c_resolver_callbacks
}

func (f *FileResolverDynamic) ResolveFile(path string) string {
	if f.callbacks == nil || f.callbacks.resolver == nil {
		return ""
	}
	
	// Create C args
	cArgs := (*C.c_file_resolve_args)(C.malloc(C.sizeof_c_file_resolve_args))
	defer C.free(unsafe.Pointer(cArgs))
	
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	
	cArgs.path = cPath
	cArgs.path_length = C.int(len(path))
	
	// Call the Swift callback (like plugin callbacks)
	cResult := C.call_file_resolve_callback(f.callbacks.resolver, cArgs, f.callbacks.resolver_data)
	if cResult == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cResult))
	
	if cResult.exists == 1 && cResult.content != nil { // file
		content := C.GoStringN(cResult.content, cResult.content_length)
		C.free(unsafe.Pointer(cResult.content))
		return content
	}
	
	return ""
}

func (f *FileResolverDynamic) FileExists(path string) bool {
	if f.callbacks == nil || f.callbacks.resolver == nil {
		return false
	}
	
	// Create C args
	cArgs := (*C.c_file_resolve_args)(C.malloc(C.sizeof_c_file_resolve_args))
	defer C.free(unsafe.Pointer(cArgs))
	
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	
	cArgs.path = cPath
	cArgs.path_length = C.int(len(path))
	
	// Call the Swift callback
	cResult := C.call_file_resolve_callback(f.callbacks.resolver, cArgs, f.callbacks.resolver_data)
	if cResult == nil {
		return false
	}
	defer C.free(unsafe.Pointer(cResult))
	
	return cResult.exists == 1 // 1 = file
}

func (f *FileResolverDynamic) DirectoryExists(path string) bool {
	if f.callbacks == nil || f.callbacks.resolver == nil {
		return false
	}
	
	// Create C args  
	cArgs := (*C.c_file_resolve_args)(C.malloc(C.sizeof_c_file_resolve_args))
	defer C.free(unsafe.Pointer(cArgs))
	
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	
	cArgs.path = cPath
	cArgs.path_length = C.int(len(path))
	
	// Call the Swift callback
	cResult := C.call_file_resolve_callback(f.callbacks.resolver, cArgs, f.callbacks.resolver_data)
	if cResult == nil {
		return false
	}
	defer C.free(unsafe.Pointer(cResult))
	
	return cResult.exists == 2 // 2 = directory
}

func (f *FileResolverDynamic) WriteFile(path string, content string) bool {
	// For in-memory builds, always capture writes instead of writing to filesystem
	// This allows tests to capture emitted content without filesystem errors
	return true
}

func (f *FileResolverDynamic) GetAllPaths(directory string) *PathList {
	if f.callbacks == nil || f.callbacks.resolver == nil {
		return &PathList{Paths: []string{}}
	}
	
	// Create C args
	cArgs := (*C.c_file_resolve_args)(C.malloc(C.sizeof_c_file_resolve_args))
	defer C.free(unsafe.Pointer(cArgs))
	
	cPath := C.CString(directory)
	defer C.free(unsafe.Pointer(cPath))
	
	cArgs.path = cPath
	cArgs.path_length = C.int(len(directory))
	
	// Call the Swift callback
	cResult := C.call_file_resolve_callback(f.callbacks.resolver, cArgs, f.callbacks.resolver_data)
	if cResult == nil || cResult.exists != 2 { // not a directory
		return &PathList{Paths: []string{}}
	}
	defer C.free(unsafe.Pointer(cResult))
	
	// Convert directory files array
	var paths []string
	if cResult.directory_files != nil && cResult.directory_files_count > 0 {
		filesPtrSlice := (*[1 << 28]*C.char)(unsafe.Pointer(cResult.directory_files))[:cResult.directory_files_count:cResult.directory_files_count]
		for i := 0; i < int(cResult.directory_files_count); i++ {
			if filesPtrSlice[i] != nil {
				paths = append(paths, C.GoString(filesPtrSlice[i]))
				C.free(unsafe.Pointer(filesPtrSlice[i]))
			}
		}
		C.free(unsafe.Pointer(cResult.directory_files))
	}
	
	return &PathList{Paths: paths}
}

//export tsc_build_with_dynamic_resolver
func tsc_build_with_dynamic_resolver(projectPath *C.char, printErrors C.int, configFile *C.char, callbacks *C.c_resolver_callbacks) *C.c_build_result {
	goProjectPath := C.GoString(projectPath)
	goPrintErrors := printErrors != 0
	goConfigFile := C.GoString(configFile)

	resolver := &FileResolverDynamic{callbacks: callbacks}

	result, err := buildWithConfig(goProjectPath, goPrintErrors, goConfigFile, resolver)
	if err != nil {
		cResult := (*C.c_build_result)(C.malloc(C.sizeof_c_build_result))
		cResult.success = 0
		cResult.config_file = C.CString("error: " + err.Error())
		cResult.diagnostics = nil
		cResult.diagnostic_count = 0
		cResult.emitted_files = nil
		cResult.emitted_file_count = 0
		cResult.written_file_paths = nil
		cResult.written_file_contents = nil
		cResult.written_file_count = 0
		return cResult
	}

	return convertBridgeResultToC(result)
}

//export tsc_validate_simple
func tsc_validate_simple(code *C.char) *C.char {
	goCode := C.GoString(code)

	resolver := NewSimpleFileResolver()
	resolver.AddFile("/project/main.ts", goCode)
	resolver.AddFile("/project/tsconfig.json", `{
		"compilerOptions": {
			"target": "es2015",
			"module": "commonjs",
			"strict": true,
			"noEmit": true
		},
		"files": ["main.ts"]
	}`)
	resolver.AddDirectory("/project")

	result, err := buildWithConfig("/project", false, "", resolver)

	response := map[string]interface{}{
		"success":     result != nil && result.Success,
		"diagnostics": []map[string]interface{}{},
	}

	if err != nil {
		response["success"] = false
		response["error"] = err.Error()
	} else if result != nil {
		diagnostics := []map[string]interface{}{}
		for _, diag := range result.Diagnostics {
			diagnostics = append(diagnostics, map[string]interface{}{
				"code":     diag.Code,
				"category": diag.Category,
				"message":  diag.Message,
				"file":     diag.File,
				"line":     diag.Line,
				"column":   diag.Column,
				"length":   diag.Length,
			})
		}
		response["diagnostics"] = diagnostics
	}

	jsonBytes, _ := json.Marshal(response)
	return C.CString(string(jsonBytes))
}

//export tsc_free_string
func tsc_free_string(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

//export tsc_free_result
func tsc_free_result(result *C.c_build_result) {
	if result == nil {
		return
	}

	if result.config_file != nil {
		C.free(unsafe.Pointer(result.config_file))
	}

	if result.diagnostics != nil {
		diagSlice := (*[1 << 28]C.c_diagnostic)(unsafe.Pointer(result.diagnostics))[:result.diagnostic_count:result.diagnostic_count]
		for i := 0; i < int(result.diagnostic_count); i++ {
			if diagSlice[i].category != nil {
				C.free(unsafe.Pointer(diagSlice[i].category))
			}
			if diagSlice[i].message != nil {
				C.free(unsafe.Pointer(diagSlice[i].message))
			}
			if diagSlice[i].file != nil {
				C.free(unsafe.Pointer(diagSlice[i].file))
			}
		}
		C.free(unsafe.Pointer(result.diagnostics))
	}

	if result.emitted_files != nil {
		fileSlice := (*[1 << 28]*C.char)(unsafe.Pointer(result.emitted_files))[:result.emitted_file_count:result.emitted_file_count]
		for i := 0; i < int(result.emitted_file_count); i++ {
			if fileSlice[i] != nil {
				C.free(unsafe.Pointer(fileSlice[i]))
			}
		}
		C.free(unsafe.Pointer(result.emitted_files))
	}

	if result.written_file_paths != nil {
		pathSlice := (*[1 << 28]*C.char)(unsafe.Pointer(result.written_file_paths))[:result.written_file_count:result.written_file_count]
		for i := 0; i < int(result.written_file_count); i++ {
			if pathSlice[i] != nil {
				C.free(unsafe.Pointer(pathSlice[i]))
			}
		}
		C.free(unsafe.Pointer(result.written_file_paths))
	}

	if result.written_file_contents != nil {
		contentSlice := (*[1 << 28]*C.char)(unsafe.Pointer(result.written_file_contents))[:result.written_file_count:result.written_file_count]
		for i := 0; i < int(result.written_file_count); i++ {
			if contentSlice[i] != nil {
				C.free(unsafe.Pointer(contentSlice[i]))
			}
		}
		C.free(unsafe.Pointer(result.written_file_contents))
	}

	C.free(unsafe.Pointer(result))
}

//export tsc_create_resolver_data
func tsc_create_resolver_data() *C.c_file_resolver_data {
	data := (*C.c_file_resolver_data)(C.malloc(C.sizeof_c_file_resolver_data))
	data.files = nil
	data.file_count = 0
	data.directories = nil
	data.directory_count = 0
	return data
}

//export tsc_add_file_to_resolver
func tsc_add_file_to_resolver(data *C.c_file_resolver_data, path *C.char, content *C.char) {
	if data == nil {
		return
	}

	newCount := data.file_count + 1
	newSize := C.size_t(newCount) * C.sizeof_c_file_entry

	if data.files == nil {
		data.files = (*C.c_file_entry)(C.malloc(newSize))
	} else {
		data.files = (*C.c_file_entry)(C.realloc(unsafe.Pointer(data.files), newSize))
	}

	fileSlice := (*[1 << 28]C.c_file_entry)(unsafe.Pointer(data.files))[:newCount:newCount]
	fileSlice[data.file_count].path = C.CString(C.GoString(path))
	fileSlice[data.file_count].content = C.CString(C.GoString(content))

	data.file_count = C.int(newCount)
}

//export tsc_add_directory_to_resolver
func tsc_add_directory_to_resolver(data *C.c_file_resolver_data, path *C.char) {
	if data == nil {
		return
	}

	newCount := data.directory_count + 1
	newSize := C.size_t(newCount) * C.size_t(unsafe.Sizeof(uintptr(0)))

	if data.directories == nil {
		data.directories = (**C.char)(C.malloc(newSize))
	} else {
		data.directories = (**C.char)(C.realloc(unsafe.Pointer(data.directories), newSize))
	}

	dirSlice := (*[1 << 28]*C.char)(unsafe.Pointer(data.directories))[:newCount:newCount]
	dirSlice[data.directory_count] = C.CString(C.GoString(path))

	data.directory_count = C.int(newCount)
}

//export tsc_free_resolver_data
func tsc_free_resolver_data(data *C.c_file_resolver_data) {
	if data == nil {
		return
	}

	if data.files != nil {
		fileSlice := (*[1 << 28]C.c_file_entry)(unsafe.Pointer(data.files))[:data.file_count:data.file_count]
		for i := 0; i < int(data.file_count); i++ {
			if fileSlice[i].path != nil {
				C.free(unsafe.Pointer(fileSlice[i].path))
			}
			if fileSlice[i].content != nil {
				C.free(unsafe.Pointer(fileSlice[i].content))
			}
		}
		C.free(unsafe.Pointer(data.files))
	}

	if data.directories != nil {
		dirSlice := (*[1 << 28]*C.char)(unsafe.Pointer(data.directories))[:data.directory_count:data.directory_count]
		for i := 0; i < int(data.directory_count); i++ {
			if dirSlice[i] != nil {
				C.free(unsafe.Pointer(dirSlice[i]))
			}
		}
		C.free(unsafe.Pointer(data.directories))
	}

	C.free(unsafe.Pointer(data))
}

func convertBridgeResultToC(result *BridgeResult) *C.c_build_result {
	cResult := (*C.c_build_result)(C.malloc(C.sizeof_c_build_result))

	if result.Success {
		cResult.success = 1
	} else {
		cResult.success = 0
	}
	cResult.config_file = C.CString(result.ConfigFile)

	cResult.diagnostic_count = C.int(len(result.Diagnostics))
	if len(result.Diagnostics) > 0 {
		diagSize := C.size_t(len(result.Diagnostics)) * C.sizeof_c_diagnostic
		cResult.diagnostics = (*C.c_diagnostic)(C.malloc(diagSize))

		diagSlice := (*[1 << 28]C.c_diagnostic)(unsafe.Pointer(cResult.diagnostics))[:len(result.Diagnostics):len(result.Diagnostics)]
		for i, diag := range result.Diagnostics {
			diagSlice[i].code = C.int(diag.Code)
			diagSlice[i].category = C.CString(diag.Category)
			diagSlice[i].message = C.CString(diag.Message)
			diagSlice[i].file = C.CString(diag.File)
			diagSlice[i].line = C.int(diag.Line)
			diagSlice[i].column = C.int(diag.Column)
			diagSlice[i].length = C.int(diag.Length)
		}
	} else {
		cResult.diagnostics = nil
	}

	cResult.emitted_file_count = C.int(len(result.EmittedFiles))
	if len(result.EmittedFiles) > 0 {
		fileSize := C.size_t(len(result.EmittedFiles)) * C.size_t(unsafe.Sizeof(uintptr(0)))
		cResult.emitted_files = (**C.char)(C.malloc(fileSize))

		fileSlice := (*[1 << 28]*C.char)(unsafe.Pointer(cResult.emitted_files))[:len(result.EmittedFiles):len(result.EmittedFiles)]
		for i, file := range result.EmittedFiles {
			fileSlice[i] = C.CString(file)
		}
	} else {
		cResult.emitted_files = nil
	}

	cResult.written_file_count = C.int(len(result.WrittenFiles))
	if len(result.WrittenFiles) > 0 {
		fileSize := C.size_t(len(result.WrittenFiles)) * C.size_t(unsafe.Sizeof(uintptr(0)))
		cResult.written_file_paths = (**C.char)(C.malloc(fileSize))
		cResult.written_file_contents = (**C.char)(C.malloc(fileSize))

		pathSlice := (*[1 << 28]*C.char)(unsafe.Pointer(cResult.written_file_paths))[:len(result.WrittenFiles):len(result.WrittenFiles)]
		contentSlice := (*[1 << 28]*C.char)(unsafe.Pointer(cResult.written_file_contents))[:len(result.WrittenFiles):len(result.WrittenFiles)]

		i := 0
		for path, content := range result.WrittenFiles {
			pathSlice[i] = C.CString(path)
			contentSlice[i] = C.CString(content)
			i++
		}
	} else {
		cResult.written_file_paths = nil
		cResult.written_file_contents = nil
	}

	return cResult
}

func main() {
	runtime.LockOSThread()
}
