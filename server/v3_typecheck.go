package main

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/execute/tsc"
	"github.com/microsoft/typescript-go/internal/locale"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"go.opentelemetry.io/otel/attribute"
)

// defaultCompilerOptions returns the default tsconfig options matching v2 hardcoded values.
func defaultCompilerOptions() *core.CompilerOptions {
	jsxImportSource := "@crayonnow/core"
	return &core.CompilerOptions{
		AllowJs:                          core.TSTrue,
		Declaration:                      core.TSTrue,
		ESModuleInterop:                  core.TSTrue,
		ForceConsistentCasingInFileNames: core.TSTrue,
		IsolatedModules:                  core.TSTrue,
		Jsx:                              core.JsxEmitReactJSX,
		JsxImportSource:                  jsxImportSource,
		Lib:                              []string{"ES2022"},
		Module:                           core.ModuleKindCommonJS,
		ModuleResolution:                 core.ModuleResolutionKindBundler,
		NoEmit:                           core.TSTrue,
		ResolveJsonModule:                core.TSTrue,
		SkipLibCheck:                     core.TSTrue,
		Strict:                           core.TSTrue,
		StrictNullChecks:                 core.TSTrue,
		Target:                           core.ScriptTargetES2022,
	}
}

// tsconfigCompilerOptions is the JSON structure for tsconfig.json compilerOptions.
type tsconfigCompilerOptions struct {
	Jsx              string   `json:"jsx"`
	JsxImportSource  string   `json:"jsxImportSource"`
	Lib              []string `json:"lib"`
	Module           string   `json:"module"`
	ModuleResolution string   `json:"moduleResolution"`
	SkipLibCheck     *bool    `json:"skipLibCheck"`
	Strict           *bool    `json:"strict"`
	Target           string   `json:"target"`
}

// tsconfigJSON is the JSON structure for tsconfig.json.
type tsconfigJSON struct {
	CompilerOptions tsconfigCompilerOptions `json:"compilerOptions"`
}

// parseTSConfig parses tsconfig.json and returns compiler options.
// If tsconfigRaw is nil, returns defaults.
func parseTSConfig(tsconfigRaw []byte) (*core.CompilerOptions, error) {
	opts := defaultCompilerOptions()

	if tsconfigRaw == nil {
		return opts, nil
	}

	var tsconfig tsconfigJSON
	if err := json.Unmarshal(tsconfigRaw, &tsconfig); err != nil {
		return nil, err
	}

	co := tsconfig.CompilerOptions

	// Jsx
	switch strings.ToLower(co.Jsx) {
	case "preserve":
		opts.Jsx = core.JsxEmitPreserve
	case "react":
		opts.Jsx = core.JsxEmitReact
	case "react-jsx":
		opts.Jsx = core.JsxEmitReactJSX
	case "react-jsxdev":
		opts.Jsx = core.JsxEmitReactJSXDev
	case "react-native":
		opts.Jsx = core.JsxEmitReactNative
	}

	// JsxImportSource
	if co.JsxImportSource != "" {
		opts.JsxImportSource = co.JsxImportSource
	}

	// Lib
	if len(co.Lib) > 0 {
		opts.Lib = co.Lib
	}

	// Module
	switch strings.ToLower(co.Module) {
	case "commonjs":
		opts.Module = core.ModuleKindCommonJS
	case "esnext":
		opts.Module = core.ModuleKindESNext
	case "es2015", "es6":
		opts.Module = core.ModuleKindES2015
	case "es2020":
		opts.Module = core.ModuleKindES2020
	case "es2022":
		opts.Module = core.ModuleKindES2022
	case "nodenext":
		opts.Module = core.ModuleKindNodeNext
	case "node16":
		opts.Module = core.ModuleKindNode16
	}

	// ModuleResolution
	switch strings.ToLower(co.ModuleResolution) {
	case "bundler":
		opts.ModuleResolution = core.ModuleResolutionKindBundler
	case "classic":
		opts.ModuleResolution = core.ModuleResolutionKindClassic
	case "node":
		opts.ModuleResolution = core.ModuleResolutionKindNode10
	case "node16":
		opts.ModuleResolution = core.ModuleResolutionKindNode16
	case "nodenext":
		opts.ModuleResolution = core.ModuleResolutionKindNodeNext
	}

	// SkipLibCheck
	if co.SkipLibCheck != nil {
		if *co.SkipLibCheck {
			opts.SkipLibCheck = core.TSTrue
		} else {
			opts.SkipLibCheck = core.TSFalse
		}
	}

	// Strict
	if co.Strict != nil {
		if *co.Strict {
			opts.Strict = core.TSTrue
		} else {
			opts.Strict = core.TSFalse
		}
	}

	// Target
	switch strings.ToLower(co.Target) {
	case "es2015", "es6":
		opts.Target = core.ScriptTargetES2015
	case "es2016":
		opts.Target = core.ScriptTargetES2016
	case "es2017":
		opts.Target = core.ScriptTargetES2017
	case "es2018":
		opts.Target = core.ScriptTargetES2018
	case "es2019":
		opts.Target = core.ScriptTargetES2019
	case "es2020":
		opts.Target = core.ScriptTargetES2020
	case "es2021":
		opts.Target = core.ScriptTargetES2021
	case "es2022":
		opts.Target = core.ScriptTargetES2022
	case "es2023":
		opts.Target = core.ScriptTargetES2023
	case "es2024":
		opts.Target = core.ScriptTargetES2024
	case "es2025":
		opts.Target = core.ScriptTargetES2025
	case "esnext":
		opts.Target = core.ScriptTargetESNext
	}

	return opts, nil
}

func typecheckV3(files map[string][]byte, tsconfigRaw []byte, lockContent []byte) TypecheckV2Response {
	return typecheckV3WithContext(context.Background(), files, tsconfigRaw, lockContent)
}

// typecheckV3WithContext performs typechecking for v3 requests.
// Config files (package.json, bun.lock, tsconfig.json) are excluded from the diskFS.
func typecheckV3WithContext(ctx context.Context, files map[string][]byte, tsconfigRaw []byte, lockContent []byte) (response TypecheckV2Response) {
	ctx, span := startSpan(ctx, "fly_tsgo.v3.typecheck",
		attribute.Int("fly_tsgo.files.count", len(files)),
	)
	typecheckStart := time.Now()
	defer func() {
		duration := time.Since(typecheckStart)
		typecheckDuration.Observe(duration.Seconds())
		log.Printf("[PERF] typecheckV3 total: %v (%d files)", duration, len(files))
		span.SetAttributes(
			attribute.Float64("fly_tsgo.typecheck.duration_ms", spanDurationMS(duration)),
			attribute.Int("fly_tsgo.typecheck.errors.count", len(response.Errors)),
			attribute.Bool("fly_tsgo.typecheck.success", len(response.Errors) == 0),
		)
		span.End()
	}()

	// Resolve deps
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)

	fs := newDiskFSFromDeps(depDir)
	fs.hasUserFiles = true

	// Populate with user files, skipping config files
	var fileNames []string
	for path, content := range files {
		// Skip config files
		if path == "/package.json" || path == "/bun.lock" || path == "/tsconfig.json" {
			continue
		}

		normalized, err := normalizeAndValidatePath(path)
		if err != nil {
			return TypecheckV2Response{
				Errors: []DiagnosticErrorV2{{
					File:    path,
					Message: err.Error(),
				}},
			}
		}

		fs.mu.Lock()
		fs.userFiles[normalized] = string(content)
		fs.mu.Unlock()

		// Collect .ts and .tsx files as entry points
		if strings.HasSuffix(normalized, ".ts") || strings.HasSuffix(normalized, ".tsx") {
			fileNames = append(fileNames, normalized)
		}
	}

	if len(fileNames) == 0 {
		return TypecheckV2Response{
			Errors: []DiagnosticErrorV2{{
				Message: "No TypeScript files found to check",
			}},
		}
	}

	// Parse compiler options from tsconfig
	compilerOptions, err := parseTSConfig(tsconfigRaw)
	if err != nil {
		recordSpanError(span, "err-v3-typecheck-parse-tsconfig", err)
		return TypecheckV2Response{
			Errors: []DiagnosticErrorV2{{
				Message: "failed to parse tsconfig.json: " + err.Error(),
			}},
		}
	}

	wrappedFS := bundled.WrapFS(fs)

	parsedOptions := &core.ParsedOptions{
		CompilerOptions: compilerOptions,
		FileNames:       fileNames,
	}

	config := &tsoptions.ParsedCommandLine{
		ParsedConfig: parsedOptions,
	}

	extendedConfigCache := &tsc.ExtendedConfigCache{}
	host := compiler.NewCachedFSCompilerHost("/", wrappedFS, bundled.LibPath(), extendedConfigCache, nil)

	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: config,
		Host:   host,
	})
	span.SetAttributes(attribute.Int("fly_tsgo.typecheck.entrypoints.count", len(fileNames)))

	// Get diagnostics
	diagnostics := program.GetSyntacticDiagnostics(ctx, nil)
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, program.GetSemanticDiagnostics(ctx, nil)...)
	}
	span.SetAttributes(attribute.Int("fly_tsgo.typecheck.diagnostics.count", len(diagnostics)))

	if len(diagnostics) > 0 {
		errors := make([]DiagnosticErrorV2, 0, len(diagnostics))
		for _, diag := range diagnostics {
			diagErr := DiagnosticErrorV2{
				Message: diag.Localize(locale.Default),
			}
			if diag.File() != nil {
				diagErr.File = diag.File().FileName()
				if diag.Loc().Pos() >= 0 {
					line, col := calculateLineColumn(diag.File().Text(), diag.Loc().Pos())
					diagErr.Line = line + 1
					diagErr.Column = col + 1
				}
			}
			errors = append(errors, diagErr)
		}
		typecheckResults.WithLabelValues("error").Inc()
		return TypecheckV2Response{Errors: errors}
	}

	typecheckResults.WithLabelValues("success").Inc()
	return TypecheckV2Response{Pass: true}
}
