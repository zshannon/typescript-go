package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
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
	case strings.HasSuffix(path, ".mjs"):
		return api.LoaderJS
	default:
		return api.LoaderDefault
	}
}

// compileV3 bundles TypeScript files using esbuild for v3 requests.
func compileV3(files map[string][]byte, pkg *v3PackageJSON, tsconfigRaw []byte, lockContent []byte) BuildV2Response {
	compileStart := time.Now()
	defer func() {
		duration := time.Since(compileStart)
		compileDuration.Observe(duration.Seconds())
		log.Printf("[PERF] compileV3 total: %v (%d files)", duration, len(files))
	}()

	// Resolve deps
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)

	fs := newDiskFSFromDeps(depDir)
	fs.hasUserFiles = true

	// Populate with user files, skipping config files
	for path, content := range files {
		if path == "/package.json" || path == "/bun.lock" || path == "/tsconfig.json" {
			continue
		}

		normalized, err := normalizeAndValidatePath(path)
		if err != nil {
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

	// Create virtual file resolver for esbuild
	resolverCalls := 0
	resolver := func(path string) (api.OnLoadResult, error) {
		resolverCalls++
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

	// Build with esbuild using options from package.json
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
		Write:             false,
		Plugins: []api.Plugin{{
			Name: "virtual-fs-v3",
			Setup: func(pb api.PluginBuild) {
				pb.OnResolve(api.OnResolveOptions{Filter: ".*"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
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

					// Check if bare import maps to a global variable
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

					// Handle subpath imports
					if args.Importer != "" && strings.Contains(args.Importer, "/node_modules/") {
						parts := strings.Split(args.Importer, "/node_modules/")
						if len(parts) >= 2 {
							remainingPath := parts[1]
							packageParts := strings.Split(remainingPath, "/")
							packageName := packageParts[0]
							if strings.HasPrefix(packageName, "@") && len(packageParts) > 1 {
								packageName = packageParts[0] + "/" + packageParts[1]
							}
							resolvedPath := "/node_modules/" + packageName + "/" + args.Path
							return api.OnResolveResult{Path: resolvedPath, Namespace: "virtual"}, nil
						}
					}

					return api.OnResolveResult{Path: args.Path, Namespace: "virtual"}, nil
				})

				pb.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "globals"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
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
	log.Printf("[PERF] esbuild.Build V3: %v (resolver called %d times)", time.Since(esbuildStart), resolverCalls)

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
		compileResults.WithLabelValues("error").Inc()
		return BuildV2Response{Errors: errors}
	}

	if len(result.OutputFiles) == 0 {
		compileResults.WithLabelValues("error").Inc()
		return BuildV2Response{Errors: []DiagnosticErrorV2{{Message: "No output generated"}}}
	}

	outputCode := string(result.OutputFiles[0].Contents)
	compileResults.WithLabelValues("success").Inc()
	return BuildV2Response{Code: outputCode}
}
