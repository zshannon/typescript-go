// bridge/tsgo.go
package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
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

// stripANSIColors removes ANSI color codes from a string
func stripANSIColors(input string) string {
	// This regex matches ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	return ansiRegex.ReplaceAllString(input, "")
}

func Build(configPath string) error {
	sys := newBridgeSystem()

	// Parse command line with project flag
	commandLine := tsoptions.ParseCommandLine([]string{"-p", configPath}, sys)

	if len(commandLine.Errors) > 0 {
		return buildDiagnosticsError(commandLine.Errors)
	}

	// Find config file
	var configFileName string
	compilerOptions := commandLine.CompilerOptions()

	if compilerOptions.Project != "" {
		fileOrDirectory := tspath.NormalizePath(compilerOptions.Project)
		if sys.FS().DirectoryExists(fileOrDirectory) {
			configFileName = tspath.CombinePaths(fileOrDirectory, "tsconfig.json")
			if !sys.FS().FileExists(configFileName) {
				return fmt.Errorf("cannot find a tsconfig.json file at: %s", configFileName)
			}
		} else {
			configFileName = fileOrDirectory
			if !sys.FS().FileExists(configFileName) {
				return fmt.Errorf("the specified path does not exist: %s", fileOrDirectory)
			}
		}
	}

	if configFileName == "" {
		return errors.New("no tsconfig.json file found")
	}

	// Parse config file
	extendedConfigCache := collections.SyncMap[tspath.Path, *tsoptions.ExtendedConfigCacheEntry]{}
	configParseResult, errors := tsoptions.GetParsedCommandLineOfConfigFile(configFileName, compilerOptions, sys, &extendedConfigCache)

	if len(errors) != 0 {
		return buildDiagnosticsError(errors)
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
	if !options.ListFilesOnly.IsTrue() {
		emitResult = program.Emit(compiler.EmitOptions{})
		allDiagnostics = append(allDiagnostics, emitResult.Diagnostics...)
	}

	// Sort and deduplicate diagnostics
	allDiagnostics = compiler.SortAndDeduplicateDiagnostics(allDiagnostics)

	// If there are errors, format them and return as an error
	if len(allDiagnostics) > 0 {
		return buildDiagnosticsError(allDiagnostics)
	}

	return nil
}

// buildDiagnosticsError formats diagnostics into a readable error message
func buildDiagnosticsError(diagnostics []*ast.Diagnostic) error {
	if len(diagnostics) == 0 {
		return errors.New("compilation failed")
	}

	var buf bytes.Buffer
	formatOpts := &diagnosticwriter.FormattingOptions{
		NewLine: "\n",
	}

	for _, diagnostic := range diagnostics {
		diagnosticwriter.WriteFormatDiagnostic(&buf, diagnostic, formatOpts)
	}

	output := stripANSIColors(buf.String())
	output = strings.TrimSpace(output)

	if output != "" {
		return fmt.Errorf("compilation failed:\n%s", output)
	}

	return errors.New("compilation failed")
}
