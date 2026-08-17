package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// FileSystem interface for reading files
type FileSystem interface {
	ReadFile(path string) (string, bool)
}

// validatePath ensures the path doesn't escape node_modules or access system files
func validatePath(path string) error {
	// Check for null bytes (basic security check)
	if strings.Contains(path, "\x00") {
		return errors.New("path contains null bytes")
	}

	// S3 has a 1024 character key limit - check early
	if len(path) > 1024 {
		return fmt.Errorf("path too long: %d characters", len(path))
	}

	// Prevent excessive path segments (DoS prevention)
	if strings.Count(path, "/") > 20 {
		return fmt.Errorf("path has too many segments: %d", strings.Count(path, "/"))
	}

	// Normalize the path to resolve .. and .
	cleanPath := filepath.Clean(path)

	// Convert to forward slashes for consistent checking
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	// Allow paths within node_modules
	if strings.HasPrefix(cleanPath, "/node_modules/") {
		return nil
	}

	// Allow relative paths that will be resolved within node_modules
	if !strings.HasPrefix(cleanPath, "/") && !strings.Contains(cleanPath, "..") {
		return nil
	}

	// Block absolute paths outside node_modules
	if strings.HasPrefix(cleanPath, "/") && !strings.HasPrefix(cleanPath, "/node_modules/") {
		return fmt.Errorf("absolute path outside node_modules not allowed: %s", cleanPath)
	}

	// Block any path that tries to escape using ..
	if strings.Contains(cleanPath, "..") {
		// Check if it escapes node_modules
		if !strings.Contains(cleanPath, "node_modules") || strings.HasPrefix(cleanPath, "../") {
			return fmt.Errorf("path traversal detected: %s", cleanPath)
		}
	}

	return nil
}

// resolvePackageExports extracts paths from package.json exports field
func resolvePackageExports(exports map[string]interface{}, subpath string) []string {
	var paths []string

	exportPath, ok := exports[subpath]
	if !ok && subpath != "." {
		// Try with ./ prefix
		exportPath, ok = exports["./"+subpath]
	}
	if !ok && subpath == "." {
		// Try default export
		exportPath, ok = exports["."]
	}

	if !ok {
		return paths
	}

	// Handle string export
	if str, ok := exportPath.(string); ok {
		paths = append(paths, str)
		return paths
	}

	// Handle object export (conditional exports)
	if obj, ok := exportPath.(map[string]interface{}); ok {
		// Prioritize import for ESM so esbuild can tree-shake
		if imp, exists := obj["import"]; exists {
			paths = append(paths, extractPathsFromExport(imp)...)
		}
		if req, exists := obj["require"]; exists {
			paths = append(paths, extractPathsFromExport(req)...)
		}
		if def, exists := obj["default"]; exists {
			if defStr, ok := def.(string); ok {
				paths = append(paths, defStr)
			}
		}
	}

	return paths
}

// extractPathsFromExport handles nested export structures
func extractPathsFromExport(export interface{}) []string {
	var paths []string

	if str, ok := export.(string); ok {
		paths = append(paths, str)
	} else if obj, ok := export.(map[string]interface{}); ok {
		// Only handle the default field for nested objects
		// (Real packages use { "default": "./path" } structure)
		if def, exists := obj["default"]; exists {
			if defStr, ok := def.(string); ok {
				paths = append(paths, defStr)
			}
		}
	}

	return paths
}

// resolveBarePackageImporter resolves the actual file path when importer is just a package name
func resolveBarePackageImporter(fs FileSystem, packageName string) string {
	pkgJsonPath := "/node_modules/" + packageName + "/package.json"
	pkgContent, exists := fs.ReadFile(pkgJsonPath)
	if !exists {
		// Default to index.js if no package.json
		return "/node_modules/" + packageName + "/index.js"
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal([]byte(pkgContent), &pkg); err != nil {
		// Malformed JSON, fallback to index.js
		return "/node_modules/" + packageName + "/index.js"
	}

	// Try exports field, preferring ESM for tree-shaking
	if exports, ok := pkg["exports"].(map[string]interface{}); ok {
		paths := resolvePackageExports(exports, ".")
		// resolvePackageExports already prioritizes import over require
		if len(paths) > 0 {
			return "/node_modules/" + packageName + "/" + strings.TrimPrefix(paths[0], "./")
		}
	}

	// Fall back to main field
	if main, ok := pkg["main"].(string); ok {
		return "/node_modules/" + packageName + "/" + main
	}

	// Default to index.js
	return "/node_modules/" + packageName + "/index.js"
}

// resolveModule is the main module resolution function
func resolveModule(fs FileSystem, importPath string, importer string) string {
	if disk, ok := fs.(*diskFS); ok {
		if resolvedPath, exists := disk.resolveUserFile(importPath, importer); exists {
			return resolvedPath
		}
		if resolvedPath, cached := disk.cachedResolution(importPath, importer); cached {
			return resolvedPath
		}
		resolvedPath := resolveModuleUncached(fs, importPath, importer)
		disk.cacheResolution(importPath, importer, resolvedPath)
		return resolvedPath
	}
	return resolveModuleUncached(fs, importPath, importer)
}

func (fs *diskFS) resolveUserFile(importPath string, importer string) (string, bool) {
	if (!strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "../")) ||
		!strings.HasPrefix(importer, "/") || strings.HasPrefix(importer, "/node_modules/") {
		return "", false
	}

	resolvedPath, err := normalizeAndValidatePath(filepath.Join(filepath.Dir(importer), importPath))
	if err != nil {
		return "", false
	}

	fs.mu.RLock()
	defer fs.mu.RUnlock()
	for _, suffix := range []string{"", ".js", ".jsx", ".mjs", ".json", ".ts", ".tsx"} {
		candidate := resolvedPath + suffix
		if _, exists := fs.userFiles[candidate]; exists {
			return candidate, true
		}
	}
	return "", false
}

func resolveModuleUncached(fs FileSystem, importPath string, importer string) string {
	// Security: validate paths (skip for relative imports - they're validated after resolution)
	if !strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "../") {
		if err := validatePath(importPath); err != nil {
			return ""
		}
	}

	// Handle absolute imports
	if strings.HasPrefix(importPath, "/") {
		if err := validatePath(importPath); err != nil {
			return ""
		}
		// Only allow /node_modules paths
		if !strings.HasPrefix(importPath, "/node_modules/") {
			return ""
		}
		// Try with common extensions
		for _, ext := range []string{"", ".js", ".jsx", ".mjs", "/index.js"} {
			testPath := importPath + ext
			if _, exists := fs.ReadFile(testPath); exists {
				return testPath
			}
		}
		return ""
	}

	// Handle relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		var importerPath string

		// If importer is already an absolute path, use it directly
		if strings.HasPrefix(importer, "/") {
			importerPath = importer
		} else if importer != "" {
			// Importer is a bare package name or subpath export (e.g., "zod-schema-faker/v4")
			// Use resolveModule to properly resolve it via exports field
			importerPath = resolveModule(fs, importer, "")
		}

		if importerPath == "" {
			return ""
		}

		importerDir := filepath.Dir(importerPath)
		resolvedPath := filepath.Join(importerDir, importPath)
		resolvedPath = strings.ReplaceAll(resolvedPath, "\\", "/")

		// Security: validate the resolved path
		if err := validatePath(resolvedPath); err != nil {
			return ""
		}

		// Try with common extensions
		for _, ext := range []string{"", ".js", ".jsx", ".mjs", ".cjs", "/index.js"} {
			testPath := resolvedPath + ext
			if _, exists := fs.ReadFile(testPath); exists {
				return testPath
			}
		}

		// Try exact path
		if _, exists := fs.ReadFile(resolvedPath); exists {
			return resolvedPath
		}

		return ""
	}

	// Handle bare imports (package imports)
	parts := strings.Split(importPath, "/")
	packageName := parts[0]
	if strings.HasPrefix(packageName, "@") && len(parts) > 1 {
		// Scoped package
		packageName = parts[0] + "/" + parts[1]
	}

	isMainPackageImport := len(parts) == len(strings.Split(packageName, "/"))

	// Check package.json for exports/main
	pkgJsonPath := "/node_modules/" + packageName + "/package.json"
	if pkgContent, exists := fs.ReadFile(pkgJsonPath); exists {
		var pkg map[string]interface{}
		if err := json.Unmarshal([]byte(pkgContent), &pkg); err == nil {
			// Check exports field first (highest priority)
			if exports, ok := pkg["exports"].(map[string]interface{}); ok {
				// Calculate subpath
				subpath := "./" + strings.Join(parts[len(strings.Split(packageName, "/")):], "/")
				if subpath == "./" {
					subpath = "."
				}

				paths := resolvePackageExports(exports, subpath)
				for _, path := range paths {
					testPath := "/node_modules/" + packageName + "/" + strings.TrimPrefix(path, "./")
					if _, exists := fs.ReadFile(testPath); exists {
						return testPath
					}
					// Try with .js extension
					if !strings.HasSuffix(testPath, ".js") && !strings.HasSuffix(testPath, ".cjs") {
						testPath = testPath + ".js"
						if _, exists := fs.ReadFile(testPath); exists {
							return testPath
						}
					}
				}
			}

			// Try main field (second priority, only for main package import)
			if isMainPackageImport {
				if main, ok := pkg["main"].(string); ok {
					mainPath := "/node_modules/" + packageName + "/" + strings.TrimPrefix(main, "./")
					if _, exists := fs.ReadFile(mainPath); exists {
						return mainPath
					}
					// Try with extension
					if !strings.HasSuffix(mainPath, ".js") {
						mainPath = mainPath + ".js"
						if _, exists := fs.ReadFile(mainPath); exists {
							return mainPath
						}
					}
				}
			}
		}
	}

	// Try default patterns (lowest priority)
	if !isMainPackageImport {
		// Subpath import
		subpath := strings.Join(parts[len(strings.Split(packageName, "/")):], "/")
		patterns := []string{
			"/node_modules/" + packageName + "/" + subpath,
			"/node_modules/" + packageName + "/" + subpath + ".js",
			"/node_modules/" + packageName + "/" + subpath + "/index.js",
		}
		for _, pattern := range patterns {
			if _, exists := fs.ReadFile(pattern); exists {
				return pattern
			}
		}
	} else {
		// Main package import - only try index.js as last resort
		patterns := []string{
			"/node_modules/" + packageName + "/index.js",
			"/node_modules/" + packageName + "/index.jsx",
			"/node_modules/" + packageName + "/index.mjs",
		}
		for _, pattern := range patterns {
			if _, exists := fs.ReadFile(pattern); exists {
				return pattern
			}
		}
	}

	return ""
}
