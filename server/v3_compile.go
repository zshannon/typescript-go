package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"go.opentelemetry.io/otel/attribute"
)

// isExternal determines if an import path should be treated as external.
// Only bare imports can be external (not relative or absolute paths).
// Supports "*" (all bare imports), exact match, and scoped prefix match.
func isExternal(path string, externals []string) bool {
	// Relative and absolute paths are never external
	if strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
		return false
	}

	for _, ext := range externals {
		if ext == "*" {
			return true
		}
		if path == ext {
			return true
		}
		// Scoped prefix match: "zod" matches "zod/lib", "@scope/pkg" matches "@scope/pkg/sub"
		if strings.HasPrefix(path, ext+"/") {
			return true
		}
	}

	return false
}

// loaderForPath returns the appropriate esbuild loader based on file extension.
func loaderForPath(path string) api.Loader {
	switch {
	case strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx"):
		return api.LoaderTSX
	case strings.HasSuffix(path, ".jsx"):
		return api.LoaderJSX
	case strings.HasSuffix(path, ".json"):
		return api.LoaderJSON
	case strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".mjs"):
		return api.LoaderJS
	default:
		return api.LoaderDefault
	}
}

func compileV3(files map[string][]byte, pkg *v3PackageJSON, tsconfigRaw []byte, lockContent []byte) BuildV2Response {
	return compileV3WithContext(context.Background(), files, pkg, tsconfigRaw, lockContent)
}

// compileV3WithContext bundles TypeScript files using esbuild for v3 requests.
func compileV3WithContext(ctx context.Context, files map[string][]byte, pkg *v3PackageJSON, tsconfigRaw []byte, lockContent []byte) BuildV2Response {
	return compileV3WithHost(ctx, files, pkg, tsconfigRaw, lockContent, nil)
}

func compileV3WithHost(ctx context.Context, files map[string][]byte, pkg *v3PackageJSON, tsconfigRaw []byte, lockContent []byte, contract *hostContract) (response BuildV2Response) {
	ctx, span := startSpan(ctx, "fly_tsgo.v3.compile",
		attribute.Int("fly_tsgo.files.count", len(files)),
		attribute.Bool("fly_tsgo.host_contract.present", contract != nil),
	)
	compileStart := time.Now()
	defer func() {
		duration := time.Since(compileStart)
		compileDuration.Observe(duration.Seconds())
		log.Printf("[PERF] compileV3 total: %v (%d files)", duration, len(files))
		span.SetAttributes(
			attribute.Float64("fly_tsgo.compile.duration_ms", spanDurationMS(duration)),
			attribute.Int("fly_tsgo.compile.errors.count", len(response.Errors)),
			attribute.Bool("fly_tsgo.compile.success", len(response.Errors) == 0),
		)
		if response.Code != "" {
			span.SetAttributes(attribute.Int("fly_tsgo.compile.output.bytes", len(response.Code)))
		}
		span.End()
	}()

	_, prepareSpan := startSpan(ctx, "esbuild.compile.prepare")
	// Resolve deps
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)

	fs := newDiskFSFromDeps(depDir)
	fs.hasUserFiles = true

	// Populate with user files, skipping config files
	for path, content := range files {
		if path == "/package.json" || path == "/bun.lock" || path == "/tsconfig.json" || isFlickCompilationFile(path) {
			continue
		}

		normalized, err := normalizeAndValidatePath(path)
		if err != nil {
			prepareSpan.End()
			return BuildV2Response{
				Errors: []DiagnosticErrorV2{{
					File:    path,
					Message: err.Error(),
				}},
			}
		}

		fs.mu.Lock()
		fs.userFiles[normalized] = string(content)
		fs.mu.Unlock()
	}

	if contract != nil {
		contract.install(fs)
	}

	// Normalize entry point: "./src/index.ts" -> "/src/index.ts"
	entryPoint := pkg.Main
	if strings.HasPrefix(entryPoint, "./") {
		entryPoint = entryPoint[1:] // "./src/index.ts" -> "/src/index.ts"
	} else if !strings.HasPrefix(entryPoint, "/") {
		entryPoint = "/" + entryPoint
	}

	// Verify entry point exists in provided files
	fs.mu.RLock()
	_, entryExists := fs.userFiles[entryPoint]
	fs.mu.RUnlock()
	if !entryExists {
		prepareSpan.End()
		return BuildV2Response{
			Errors: []DiagnosticErrorV2{{
				File:    pkg.Main,
				Message: fmt.Sprintf("Entry point not found in provided files: %s", pkg.Main),
			}},
		}
	}

	// Get esbuild options from package.json
	opts := pkg.Esbuild.esbuildOptions()

	// Get externals and globals for plugin-based handling
	externals := pkg.Esbuild.External
	globals := pkg.Esbuild.Globals
	prepareSpan.End()

	// Create virtual file resolver for esbuild.
	loadTimings := durationAccumulator{}
	resolveTimings := durationAccumulator{}
	resolver := func(path string) (api.OnLoadResult, error) {
		startedAt := time.Now()
		defer func() { loadTimings.observe(time.Since(startedAt)) }()
		trackPackageResolution(path)

		if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
			if content, exists := fs.ReadFile(path); exists {
				return api.OnLoadResult{
					Contents: &content,
					Loader:   loaderForPath(path),
				}, nil
			}
		}

		// Try with common extensions
		if strings.HasPrefix(path, "/") {
			extensions := []string{".js", ".jsx", ".mjs", ".json", ".ts", ".tsx"}
			for _, ext := range extensions {
				testPath := path + ext
				if content, exists := fs.ReadFile(testPath); exists {
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loaderForPath(testPath),
					}, nil
				}
			}
		}

		// Try to resolve using module resolver
		if !strings.HasPrefix(path, "/") {
			resolvedPath := resolveModule(fs, path, "")
			if resolvedPath != "" {
				if content, exists := fs.ReadFile(resolvedPath); exists {
					return api.OnLoadResult{
						Contents: &content,
						Loader:   loaderForPath(resolvedPath),
					}, nil
				}
			}
		}

		return api.OnLoadResult{}, fmt.Errorf("file not found: %s", path)
	}

	// Pass tsconfig to esbuild so it picks up JSX settings (jsx, jsxImportSource, etc.)
	var tsconfigForEsbuild string
	if tsconfigRaw != nil {
		tsconfigForEsbuild = string(tsconfigRaw)
	}

	// Build with esbuild using options from package.json
	esbuildCtx, esbuildSpan := startSpan(ctx, "esbuild.build",
		attribute.Bool("esbuild.bundle", opts.Bundle),
		attribute.String("esbuild.format", esbuildFormatName(opts.Format)),
	)
	_, loadSpan := startSpan(esbuildCtx, "esbuild.load")
	_, resolveSpan := startSpan(esbuildCtx, "esbuild.resolve")
	esbuildStart := time.Now()
	result := api.Build(api.BuildOptions{
		Bundle:            opts.Bundle,
		EntryPoints:       []string{entryPoint},
		Format:            opts.Format,
		MinifyIdentifiers: opts.MinifyIdentifiers,
		MinifySyntax:      opts.MinifySyntax,
		MinifyWhitespace:  opts.MinifyWhitespace,
		Platform:          opts.Platform,
		Target:            opts.Target,
		TsconfigRaw:       tsconfigForEsbuild,
		Write:             false,
		Plugins: []api.Plugin{{
			Name: "virtual-fs-v3",
			Setup: func(pb api.PluginBuild) {
				pb.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					startedAt := time.Now()
					defer func() { resolveTimings.observe(time.Since(startedAt)) }()
					trackPackageResolution(args.Path)

					// Handle absolute imports
					if strings.HasPrefix(args.Path, "/") {
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}

					// Handle relative imports
					if strings.HasPrefix(args.Path, "./") || strings.HasPrefix(args.Path, "../") {
						resolvedPath := resolveModule(fs, args.Path, args.Importer)
						if resolvedPath == "" {
							importerPath := args.Importer
							if !strings.HasPrefix(importerPath, "/") {
								importerPath = resolveBarePackageImporter(fs, importerPath)
							}
							importerDir := filepath.Dir(importerPath)
							resolvedPath = filepath.Join(importerDir, args.Path)
							resolvedPath = strings.ReplaceAll(resolvedPath, "\\", "/")
						}
						return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
					}

					// Check if bare import maps to a global variable (exact match only)
					if _, ok := globals[args.Path]; ok {
						return api.OnResolveResult{Path: args.Path, Namespace: "globals"}, nil
					}

					// Check if bare import is external (produces require())
					if isExternal(args.Path, externals) {
						return api.OnResolveResult{External: true}, nil
					}

					// Handle bare imports
					if !strings.Contains(args.Path, "/") || strings.HasPrefix(args.Path, "@") {
						return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
					}

					// Handle subpath imports from within node_modules.
					// Only resolve relative to the importer's package if the import
					// targets the SAME package. Cross-package bare specifiers like
					// "react/jsx-runtime" imported from "@flickfyi/photon" must resolve
					// from /node_modules/ root, not inside the importer's package.
					if args.Importer != "" && strings.Contains(args.Importer, "/node_modules/") {
						// Extract the importing package's name
						parts := strings.Split(args.Importer, "/node_modules/")
						if len(parts) >= 2 {
							remainingPath := parts[1]
							packageParts := strings.Split(remainingPath, "/")
							importerPkg := packageParts[0]
							if strings.HasPrefix(importerPkg, "@") && len(packageParts) > 1 {
								importerPkg = packageParts[0] + "/" + packageParts[1]
							}

							// Extract the imported specifier's package name
							importParts := strings.Split(args.Path, "/")
							importPkg := importParts[0]
							if strings.HasPrefix(importPkg, "@") && len(importParts) > 1 {
								importPkg = importParts[0] + "/" + importParts[1]
							}

							if importPkg == importerPkg {
								// Same package: resolve as subpath within the package
								resolvedPath := "/node_modules/" + importerPkg + "/" + args.Path
								return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
							}
						}
					}

					return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
				})

				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "globals"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					startedAt := time.Now()
					defer func() { loadTimings.observe(time.Since(startedAt)) }()
					globalVar := globals[args.Path]
					contents := "module.exports = " + globalVar
					return api.OnLoadResult{Contents: &contents, Loader: api.LoaderJS}, nil
				})

				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "virtual"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
					return resolver(args.Path)
				})
			},
		}},
	})
	esbuildDuration := time.Since(esbuildStart)
	waitForDiskMemoryCache()
	cacheStats := fs.cacheSnapshot()
	loadSpan.SetAttributes(
		attribute.Int64("esbuild.load.calls.count", loadTimings.count()),
		attribute.Float64("esbuild.load.duration_ms.max", spanDurationMS(loadTimings.max())),
		attribute.Float64("esbuild.load.duration_ms.sum", spanDurationMS(loadTimings.total())),
		attribute.Int64("fly_tsgo.memory_cache.file.hit.count", cacheStats.fileHits),
		attribute.Int64("fly_tsgo.memory_cache.file.miss.count", cacheStats.fileMisses),
	)
	loadSpan.End()
	resolveSpan.SetAttributes(
		attribute.Int64("esbuild.resolve.calls.count", resolveTimings.count()),
		attribute.Float64("esbuild.resolve.duration_ms.max", spanDurationMS(resolveTimings.max())),
		attribute.Float64("esbuild.resolve.duration_ms.sum", spanDurationMS(resolveTimings.total())),
		attribute.Int64("fly_tsgo.memory_cache.resolution.hit.count", cacheStats.resolutionHits),
		attribute.Int64("fly_tsgo.memory_cache.resolution.miss.count", cacheStats.resolutionMisses),
	)
	resolveSpan.End()
	esbuildSpan.SetAttributes(
		attribute.Bool("fly_tsgo.esbuild.bundle", opts.Bundle),
		attribute.Float64("fly_tsgo.esbuild.duration_ms", spanDurationMS(esbuildDuration)),
		attribute.Int("fly_tsgo.esbuild.errors.count", len(result.Errors)),
		attribute.String("fly_tsgo.esbuild.format", esbuildFormatName(opts.Format)),
		attribute.Int("fly_tsgo.esbuild.output_files.count", len(result.OutputFiles)),
		attribute.Int64("fly_tsgo.esbuild.resolver_calls.count", loadTimings.count()),
	)
	esbuildSpan.End()
	span.SetAttributes(attribute.String("fly_tsgo.compile.entry_point", entryPoint))
	log.Printf("[PERF] esbuild.Build V3: %v (resolver called %d times)", esbuildDuration, loadTimings.count())

	_, resultSpan := startSpan(ctx, "esbuild.result.process")
	if len(result.Errors) > 0 {
		errors := make([]DiagnosticErrorV2, 0, len(result.Errors))
		for _, err := range result.Errors {
			diagErr := DiagnosticErrorV2{
				Message: err.Text,
			}
			if err.Location != nil {
				diagErr.File = err.Location.File
				diagErr.Line = err.Location.Line
				diagErr.Column = err.Location.Column
			}
			errors = append(errors, diagErr)
		}
		resultSpan.End()
		compileResults.WithLabelValues("error").Inc()
		return BuildV2Response{Errors: errors}
	}

	if len(result.OutputFiles) == 0 {
		resultSpan.End()
		compileResults.WithLabelValues("error").Inc()
		return BuildV2Response{Errors: []DiagnosticErrorV2{{Message: "No output generated"}}}
	}

	outputCode := string(result.OutputFiles[0].Contents)
	resultSpan.End()
	compileResults.WithLabelValues("success").Inc()
	return BuildV2Response{Code: outputCode}
}

type durationAccumulator struct {
	calls      atomic.Int64
	maxNanos   atomic.Int64
	totalNanos atomic.Int64
}

func (accumulator *durationAccumulator) count() int64 {
	return accumulator.calls.Load()
}

func (accumulator *durationAccumulator) max() time.Duration {
	return time.Duration(accumulator.maxNanos.Load())
}

func (accumulator *durationAccumulator) observe(duration time.Duration) {
	nanoseconds := duration.Nanoseconds()
	accumulator.calls.Add(1)
	accumulator.totalNanos.Add(nanoseconds)

	for maximum := accumulator.maxNanos.Load(); nanoseconds > maximum; maximum = accumulator.maxNanos.Load() {
		if accumulator.maxNanos.CompareAndSwap(maximum, nanoseconds) {
			break
		}
	}
}

func (accumulator *durationAccumulator) total() time.Duration {
	return time.Duration(accumulator.totalNanos.Load())
}

func esbuildFormatName(format api.Format) string {
	switch format {
	case api.FormatIIFE:
		return "iife"
	case api.FormatESModule:
		return "esm"
	case api.FormatCommonJS:
		return "cjs"
	default:
		return "unknown"
	}
}
