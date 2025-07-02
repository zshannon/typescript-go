// bridge/tsgo.go
package bridge

import (
	"context"
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

// FileResolver is the interface that Swift can implement to provide custom file resolution
type FileResolver interface {
	// ResolveFile returns the contents of the file at the given path
	// Returns empty string if the file doesn't exist or can't be read
	ResolveFile(path string) string
	// FileExists returns true if the file exists at the given path
	FileExists(path string) bool
	// DirectoryExists returns true if the directory exists at the given path
	DirectoryExists(path string) bool
	// WriteFile writes content to the given path
	// Returns true if the write was successful
	WriteFile(path string, content string) bool

	GetAllPaths(directory string) *PathList
}

// PathList represents a list of paths for gomobile compatibility
type PathList struct {
	Paths []string
}

// CreatePathList creates a new PathList instance with the given paths
func CreatePathList() *PathList {
	return &PathList{Paths: []string{}}
}

// Add adds a string to the PathList's Paths slice
func (p *PathList) Add(path string) {
	p.Paths = append(p.Paths, path)
}

// GetCount returns the number of paths in the list
func (p *PathList) GetCount() int {
	return len(p.Paths)
}

// GetPath returns the path at the given index
func (p *PathList) GetPath(index int) string {
	if index < 0 || index >= len(p.Paths) {
		return ""
	}
	return p.Paths[index]
}

// Clear removes all paths from the list
func (p *PathList) Clear() {
	p.Paths = p.Paths[:0]
}

// pathEnumerator is an optional interface that FileResolver can implement
// to enable TypeScript file discovery through patterns like **/*.ts
type pathEnumerator interface {
	// GetAllPaths returns all known file and directory paths under the given directory
	GetAllPaths(directory string) *PathList
}

// callbackVFS implements vfs.FS using a FileResolver callback
type callbackVFS struct {
	resolver     FileResolver
	osvfs        vfs.FS            // fallback to OS filesystem for unsupported operations
	writtenFiles map[string]string // Track files written during compilation
	mu           sync.RWMutex      // Protects writtenFiles from concurrent access
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
	// Check if this file was written during compilation first
	c.mu.RLock()
	writtenContent, exists := c.writtenFiles[path]
	c.mu.RUnlock()

	if exists {
		return writtenContent, true
	}
	// Otherwise, resolve through the resolver
	contents = c.resolver.ResolveFile(path)
	return contents, contents != ""
}

func (c *callbackVFS) WriteFile(path string, data string, writeByteOrderMark bool) error {
	// Try to write using the resolver first
	if c.resolver.WriteFile(path, data) {
		// Store the written file content for retrieval
		c.mu.Lock()
		c.writtenFiles[path] = data
		c.mu.Unlock()
		return nil
	}
	// Fallback to OS filesystem for write operations
	return c.osvfs.WriteFile(path, data, writeByteOrderMark)
}

func (c *callbackVFS) Remove(path string) error {
	// Remove from written files if it exists there
	c.mu.Lock()
	delete(c.writtenFiles, path)
	c.mu.Unlock()
	// Delegate to OS filesystem for remove operations
	return c.osvfs.Remove(path)
}

func (c *callbackVFS) DirectoryExists(path string) bool {
	return c.resolver.DirectoryExists(path)
}

func (c *callbackVFS) GetAccessibleEntries(path string) vfs.Entries {
	var files []string
	var directories []string

	// First, try to get direct children from the resolver's GetAllPaths method
	pathList := c.resolver.GetAllPaths(path)
	if pathList != nil {
		for _, childName := range pathList.Paths {
			if childName == "" {
				continue
			}

			// Construct the full path for this child
			childPath := path
			if !strings.HasSuffix(path, "/") {
				childPath += "/"
			}
			childPath += childName

			// Check if it's a file or directory
			if c.resolver.FileExists(childPath) {
				files = append(files, childName)
			} else if c.resolver.DirectoryExists(childPath) {
				directories = append(directories, childName)
			}
		}
	}

	// Fallback: Get all files and directories from known paths
	if len(files) == 0 && len(directories) == 0 {
		for filePath := range c.getAllKnownPaths(path) {
			if strings.HasPrefix(filePath, path) {
				relativePath := strings.TrimPrefix(filePath, path)
				if relativePath != "" && relativePath[0] == '/' {
					relativePath = relativePath[1:]
				}

				// Skip if this is the exact path or empty relative path
				if relativePath == "" {
					continue
				}

				// Check if this is a direct child (no additional path separators)
				if !strings.Contains(relativePath, "/") {
					if c.resolver.FileExists(filePath) {
						files = append(files, relativePath)
					}
				} else {
					// This is a subdirectory - extract the first segment
					segments := strings.Split(relativePath, "/")
					if len(segments) > 0 {
						dirName := segments[0]
						dirPath := path + "/" + dirName
						if c.resolver.DirectoryExists(dirPath) {
							// Check if we already added this directory
							found := false
							for _, existing := range directories {
								if existing == dirName {
									found = true
									break
								}
							}
							if !found {
								directories = append(directories, dirName)
							}
						}
					}
				}
			}
		}
	}

	return vfs.Entries{
		Files:       files,
		Directories: directories,
	}
}

func (c *callbackVFS) Stat(path string) vfs.FileInfo {
	// Delegate to OS filesystem for stat operations
	return c.osvfs.Stat(path)
}

func (c *callbackVFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error {
	// Walk through all known paths that start with root
	for filePath := range c.getAllKnownPaths(root) {
		if strings.HasPrefix(filePath, root) {
			if c.resolver.FileExists(filePath) {
				// Create a minimal DirEntry for the file
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
	// For virtual files, just return the path as-is
	if c.resolver.FileExists(path) || c.resolver.DirectoryExists(path) {
		return path
	}
	// Delegate to OS filesystem for realpath
	return c.osvfs.Realpath(path)
}

// getAllKnownPaths returns all paths known to the resolver under a specific directory
// This is a helper method to iterate over virtual files
func (c *callbackVFS) getAllKnownPaths(directory string) map[string]bool {
	paths := make(map[string]bool)

	// Add written files under the directory
	for path := range c.writtenFiles {
		if strings.HasPrefix(path, directory) {
			paths[path] = true
		}
	}

	// Get all paths from the resolver if it supports enumeration
	if enumerator, ok := c.resolver.(pathEnumerator); ok {
		pathList := enumerator.GetAllPaths(directory)
		if pathList != nil {
			for _, foundPath := range pathList.Paths {
				paths[foundPath] = true
			}
		}
	}

	return paths
}

// simpleDirEntry implements fs.DirEntry for virtual files
type simpleDirEntry struct {
	name  string
	isDir bool
}

func (e *simpleDirEntry) Name() string {
	return e.name
}

func (e *simpleDirEntry) IsDir() bool {
	return e.isDir
}

func (e *simpleDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}

func (e *simpleDirEntry) Info() (fs.FileInfo, error) {
	return &simpleFileInfo{
		name:  e.name,
		size:  0,
		isDir: e.isDir,
	}, nil
}

// simpleFileInfo implements fs.FileInfo for virtual files
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

// BuildConfig holds configuration options for the build process
type BuildConfig struct {
	// ProjectPath is the path to the project directory or tsconfig.json file
	ProjectPath string
	// PrintErrors controls whether errors should be printed to stdout during compilation
	PrintErrors bool
	// ConfigFile allows specifying a custom config file path (optional)
	ConfigFile string
	// FileResolver allows providing custom file resolution (optional)
	FileResolver FileResolver
}

// DiagnosticInfo contains detailed information about a TypeScript diagnostic
type DiagnosticInfo struct {
	// Code is the diagnostic code (e.g., 2345)
	Code int
	// Category is the diagnostic category (error, warning, info, etc.)
	Category string
	// Message is the diagnostic message
	Message string
	// File is the source file where the diagnostic occurred (may be empty)
	File string
	// Line is the line number (1-based, 0 if not available)
	Line int
	// Column is the column number (1-based, 0 if not available)
	Column int
	// Length is the length of the affected text (0 if not available)
	Length int
}

// BuildResult contains the result of a TypeScript compilation
type BuildResult struct {
	// Success indicates whether the compilation succeeded
	Success bool
	// Diagnostics contains all diagnostics (errors, warnings, etc.)
	Diagnostics []DiagnosticInfo
	// EmittedFiles contains the list of files that were emitted
	EmittedFiles []string
	// ConfigFile is the resolved config file path that was used
	ConfigFile string
	// WrittenFiles contains the content of files written during compilation (when using FileResolver)
	WrittenFiles map[string]string
}

// BridgeDiagnostic contains detailed information about a TypeScript diagnostic (gomobile-compatible)
type BridgeDiagnostic struct {
	Code     int
	Category string
	Message  string
	File     string
	Line     int
	Column   int
	Length   int
}

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

func (s *bridgeSystem) SinceStart() time.Duration {
	return time.Since(s.start)
}

func (s *bridgeSystem) Now() time.Time {
	return time.Now()
}

func (s *bridgeSystem) FS() vfs.FS {
	if s.customFS != nil {
		return s.customFS
	}
	return s.fs
}

func (s *bridgeSystem) DefaultLibraryPath() string {
	return s.defaultLibraryPath
}

func (s *bridgeSystem) GetCurrentDirectory() string {
	return s.cwd
}

func (s *bridgeSystem) NewLine() string {
	return s.newLine
}

func (s *bridgeSystem) Writer() io.Writer {
	return s.writer
}

func (s *bridgeSystem) EndWrite() {
	// do nothing, this is needed in the interface for testing
}

func newBridgeSystem() *bridgeSystem {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	return &bridgeSystem{
		cwd:                tspath.NormalizePath(cwd),
		fs:                 bundled.WrapFS(osvfs.FS()),
		defaultLibraryPath: bundled.LibPath(),
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

func BuildWithConfig(config BuildConfig) BuildResult {
	var sys *bridgeSystem
	if config.FileResolver != nil {
		sys = newBridgeSystemWithResolver(config.FileResolver)
	} else {
		sys = newBridgeSystem()
	}

	// Determine which writer to use based on PrintErrors setting
	if config.PrintErrors {
		sys.writer = os.Stdout
	} else {
		sys.writer = io.Discard
	}

	// Use custom config file if provided, otherwise use project path
	projectPath := config.ProjectPath
	if config.ConfigFile != "" {
		projectPath = config.ConfigFile
	}

	// Parse command line with project flag
	commandLine := tsoptions.ParseCommandLine([]string{"-p", projectPath}, sys)

	if len(commandLine.Errors) > 0 {
		return BuildResult{
			Success:     false,
			Diagnostics: convertASTDiagnostics(commandLine.Errors),
		}
	}

	// Find config file
	var configFileName string
	compilerOptions := commandLine.CompilerOptions()

	if compilerOptions.Project != "" {
		fileOrDirectory := tspath.NormalizePath(compilerOptions.Project)
		if sys.FS().DirectoryExists(fileOrDirectory) {
			configFileName = tspath.CombinePaths(fileOrDirectory, "tsconfig.json")
			if !sys.FS().FileExists(configFileName) {
				return BuildResult{
					Success: false,
					Diagnostics: []DiagnosticInfo{{
						Code:     0,
						Category: "error",
						Message:  fmt.Sprintf("cannot find a tsconfig.json file at: %s", configFileName),
					}},
				}
			}
		} else {
			configFileName = fileOrDirectory
			if !sys.FS().FileExists(configFileName) {
				return BuildResult{
					Success: false,
					Diagnostics: []DiagnosticInfo{{
						Code:     0,
						Category: "error",
						Message:  fmt.Sprintf("the specified path does not exist: %s", fileOrDirectory),
					}},
				}
			}
		}
	}

	if configFileName == "" {
		return BuildResult{
			Success: false,
			Diagnostics: []DiagnosticInfo{{
				Code:     0,
				Category: "error",
				Message:  "no tsconfig.json file found",
			}},
		}
	}

	// Parse config file
	extendedConfigCache := collections.SyncMap[tspath.Path, *tsoptions.ExtendedConfigCacheEntry]{}
	configParseResult, parseErrors := tsoptions.GetParsedCommandLineOfConfigFile(configFileName, compilerOptions, sys, &extendedConfigCache)

	if len(parseErrors) != 0 {
		return BuildResult{
			Success:     false,
			Diagnostics: convertASTDiagnostics(parseErrors),
			ConfigFile:  configFileName,
		}
	}

	// Perform compilation
	host := compiler.NewCachedFSCompilerHost(configParseResult.CompilerOptions(), sys.GetCurrentDirectory(), sys.FS(), sys.DefaultLibraryPath(), &extendedConfigCache)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config:           configParseResult,
		Host:             host,
		JSDocParsingMode: ast.JSDocParsingModeParseForTypeErrors,
	})

	// Collect all diagnostics (following the same pattern as emitFilesAndReportErrors)
	ctx := context.Background()
	options := program.Options()
	allDiagnostics := slices.Clip(program.GetConfigFileParsingDiagnostics())
	configFileParsingDiagnosticsLength := len(allDiagnostics)

	allDiagnostics = append(allDiagnostics, program.GetSyntacticDiagnostics(ctx, nil)...)

	if len(allDiagnostics) == configFileParsingDiagnosticsLength {
		// Bind diagnostics early to ensure proper initialization
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

	// Emit files if not in noEmit mode
	var emitResult *compiler.EmitResult
	var emittedFiles []string
	if !options.ListFilesOnly.IsTrue() {
		emitResult = program.Emit(compiler.EmitOptions{})
		allDiagnostics = append(allDiagnostics, emitResult.Diagnostics...)
		emittedFiles = emitResult.EmittedFiles

		// If we have a callback VFS, collect the written files
		if sys.callbackVFS != nil {
			for path := range sys.callbackVFS.writtenFiles {
				// Only add if not already in emittedFiles
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

	// Sort and deduplicate diagnostics
	allDiagnostics = compiler.SortAndDeduplicateDiagnostics(allDiagnostics)

	// Convert diagnostics to our format
	diagnostics := convertASTDiagnostics(allDiagnostics)

	// Print errors if requested
	if config.PrintErrors && len(allDiagnostics) > 0 {
		for _, diag := range allDiagnostics {
			formatOpts := &diagnosticwriter.FormattingOptions{
				NewLine: "\n",
			}
			diagnosticwriter.WriteFormatDiagnostic(sys.Writer(), diag, formatOpts)
		}
	}

	// Determine success - typically errors prevent success, but warnings don't
	success := true
	for _, diag := range diagnostics {
		if diag.Category == "error" {
			success = false
			break
		}
	}

	result := BuildResult{
		Success:      success,
		Diagnostics:  diagnostics,
		EmittedFiles: emittedFiles,
		ConfigFile:   configFileName,
	}

	// Add written file contents if we have a callback VFS
	if sys.callbackVFS != nil {
		result.WrittenFiles = make(map[string]string)
		sys.callbackVFS.mu.RLock()
		for path, content := range sys.callbackVFS.writtenFiles {
			result.WrittenFiles[path] = content
		}
		sys.callbackVFS.mu.RUnlock()
	}

	return result
}

// convertASTDiagnostics converts AST diagnostics to our DiagnosticInfo format
func convertASTDiagnostics(diagnostics []*ast.Diagnostic) []DiagnosticInfo {
	result := make([]DiagnosticInfo, len(diagnostics))
	for i, diag := range diagnostics {
		result[i] = DiagnosticInfo{
			Code:     int(diag.Code()),
			Category: diag.Category().Name(),
			Message:  diag.Message(),
		}

		// Add file information if available
		if diag.File() != nil {
			result[i].File = diag.File().FileName()
			if diag.Loc().Pos() >= 0 {
				// Calculate line and column from position
				line, column := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
				result[i].Line = line + 1     // Convert to 1-based
				result[i].Column = column + 1 // Convert to 1-based
				result[i].Length = diag.Loc().End() - diag.Loc().Pos()
			}
		}
	}
	return result
}

// calculateLineColumn calculates line and column from text position
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

// BridgeResult contains the full result of a TypeScript compilation (gomobile-compatible)
type BridgeResult struct {
	Success      bool
	ConfigFile   string
	Diagnostics  []BridgeDiagnostic
	EmittedFiles []string
	WrittenFiles map[string]string
}

// GetDiagnosticCount returns the number of diagnostics
func (r *BridgeResult) GetDiagnosticCount() int {
	return len(r.Diagnostics)
}

// GetDiagnostic returns a diagnostic by index
func (r *BridgeResult) GetDiagnostic(index int) *BridgeDiagnostic {
	if index < 0 || index >= len(r.Diagnostics) {
		return nil
	}
	return &r.Diagnostics[index]
}

// GetEmittedFileCount returns the number of emitted files
func (r *BridgeResult) GetEmittedFileCount() int {
	return len(r.EmittedFiles)
}

// GetEmittedFile returns an emitted file path by index
func (r *BridgeResult) GetEmittedFile(index int) string {
	if index < 0 || index >= len(r.EmittedFiles) {
		return ""
	}
	return r.EmittedFiles[index]
}

// GetWrittenFileContent returns the content of a written file by path
func (r *BridgeResult) GetWrittenFileContent(path string) string {
	if r.WrittenFiles == nil {
		return ""
	}
	return r.WrittenFiles[path]
}

// GetWrittenFileCount returns the number of written files
func (r *BridgeResult) GetWrittenFileCount() int {
	if r.WrittenFiles == nil {
		return 0
	}
	return len(r.WrittenFiles)
}

// GetWrittenFilePath returns a written file path by index
func (r *BridgeResult) GetWrittenFilePath(index int) string {
	if r.WrittenFiles == nil {
		return ""
	}
	if index < 0 || index >= len(r.WrittenFiles) {
		return ""
	}
	// Convert map to sorted slice for consistent ordering
	paths := make([]string, 0, len(r.WrittenFiles))
	for path := range r.WrittenFiles {
		paths = append(paths, path)
	}
	// Sort for deterministic ordering
	for i := 0; i < len(paths)-1; i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[i] > paths[j] {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
	return paths[index]
}

// GetWrittenFilePaths returns all written file paths as a PathList
func (r *BridgeResult) GetWrittenFilePaths() *PathList {
	if r.WrittenFiles == nil {
		return &PathList{Paths: []string{}}
	}
	paths := make([]string, 0, len(r.WrittenFiles))
	for path := range r.WrittenFiles {
		paths = append(paths, path)
	}
	return &PathList{Paths: paths}
}

// BuildWithFileSystem builds using only the filesystem (no custom resolver)
func BuildWithFileSystem(projectPath string, printErrors bool, configFile string) (*BridgeResult, error) {
	config := BuildConfig{
		ProjectPath:  projectPath,
		PrintErrors:  printErrors,
		ConfigFile:   configFile,
		FileResolver: nil, // No custom resolver, use filesystem
	}

	result := BuildWithConfig(config)

	// Convert diagnostics to bridge format
	bridgeDiagnostics := make([]BridgeDiagnostic, len(result.Diagnostics))
	for i, diag := range result.Diagnostics {
		bridgeDiagnostics[i] = BridgeDiagnostic{
			Code:     diag.Code,
			Category: diag.Category,
			Message:  diag.Message,
			File:     diag.File,
			Line:     diag.Line,
			Column:   diag.Column,
			Length:   diag.Length,
		}
	}

	bridgeResult := &BridgeResult{
		Success:      result.Success,
		ConfigFile:   result.ConfigFile,
		Diagnostics:  bridgeDiagnostics,
		EmittedFiles: result.EmittedFiles,
		WrittenFiles: result.WrittenFiles,
	}

	return bridgeResult, nil
}

// BuildWithFileResolver builds with a dynamic callback-based file resolver
func BuildWithFileResolver(projectPath string, printErrors bool, configFile string, resolver FileResolver) (*BridgeResult, error) {
	config := BuildConfig{
		ProjectPath:  projectPath,
		PrintErrors:  printErrors,
		ConfigFile:   configFile,
		FileResolver: resolver,
	}

	result := BuildWithConfig(config)

	// Convert diagnostics to bridge format
	bridgeDiagnostics := make([]BridgeDiagnostic, len(result.Diagnostics))
	for i, diag := range result.Diagnostics {
		bridgeDiagnostics[i] = BridgeDiagnostic{
			Code:     diag.Code,
			Category: diag.Category,
			Message:  diag.Message,
			File:     diag.File,
			Line:     diag.Line,
			Column:   diag.Column,
			Length:   diag.Length,
		}
	}

	bridgeResult := &BridgeResult{
		Success:      result.Success,
		ConfigFile:   result.ConfigFile,
		Diagnostics:  bridgeDiagnostics,
		EmittedFiles: result.EmittedFiles,
		WrittenFiles: result.WrittenFiles,
	}

	return bridgeResult, nil
}
