// bridge/tsgo.go
package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
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

// BuildConfig holds configuration options for the build process
type BuildConfig struct {
	// ProjectPath is the path to the project directory or tsconfig.json file
	ProjectPath string
	// PrintErrors controls whether errors should be printed to stdout during compilation
	PrintErrors bool
	// ConfigFile allows specifying a custom config file path (optional)
	ConfigFile string
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

// BridgeResult contains the result of a TypeScript compilation (gomobile-compatible)
type BridgeResult struct {
	Success          bool
	ConfigFile       string
	DiagnosticCount  int
	EmittedFileCount int
}

type bridgeSystem struct {
	writer             io.Writer
	fs                 vfs.FS
	defaultLibraryPath string
	newLine            string
	cwd                string
	start              time.Time
}

func (s *bridgeSystem) SinceStart() time.Duration {
	return time.Since(s.start)
}

func (s *bridgeSystem) Now() time.Time {
	return time.Now()
}

func (s *bridgeSystem) FS() vfs.FS {
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

func BuildWithConfig(config BuildConfig) BuildResult {
	sys := newBridgeSystem()

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

	return BuildResult{
		Success:      success,
		Diagnostics:  diagnostics,
		EmittedFiles: emittedFiles,
		ConfigFile:   configFileName,
	}
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

// Global state for storing detailed results (gomobile limitation workaround)
var (
	lastBuildDiagnostics  []DiagnosticInfo
	lastBuildEmittedFiles []string
)

// BridgeBuildWithConfig is the gomobile-compatible bridge function
func BridgeBuildWithConfig(projectPath string, printErrors bool, configFile string) (*BridgeResult, error) {
	config := BuildConfig{
		ProjectPath: projectPath,
		PrintErrors: printErrors,
		ConfigFile:  configFile,
	}

	result := BuildWithConfig(config)

	bridgeResult := &BridgeResult{
		Success:          result.Success,
		ConfigFile:       result.ConfigFile,
		DiagnosticCount:  len(result.Diagnostics),
		EmittedFileCount: len(result.EmittedFiles),
	}

	// Store diagnostics and files for retrieval
	lastBuildDiagnostics = result.Diagnostics
	lastBuildEmittedFiles = result.EmittedFiles

	// Don't return error for compilation failures - those are communicated through
	// the Success flag and diagnostics. Only return errors for system-level issues.
	return bridgeResult, nil
}

// GetLastDiagnostic returns diagnostic info by index
func GetLastDiagnostic(index int) *BridgeDiagnostic {
	if index < 0 || index >= len(lastBuildDiagnostics) {
		return nil
	}

	diag := lastBuildDiagnostics[index]
	return &BridgeDiagnostic{
		Code:     diag.Code,
		Category: diag.Category,
		Message:  diag.Message,
		File:     diag.File,
		Line:     diag.Line,
		Column:   diag.Column,
		Length:   diag.Length,
	}
}

// GetLastEmittedFile returns emitted file by index
func GetLastEmittedFile(index int) string {
	if index < 0 || index >= len(lastBuildEmittedFiles) {
		return ""
	}
	return lastBuildEmittedFiles[index]
}
