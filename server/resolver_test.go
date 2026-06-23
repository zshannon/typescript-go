package main

import (
	"testing"
)

// Mock filesystem for testing
type MockFS struct {
	files map[string]string
}

func NewMockFS() *MockFS {
	return &MockFS{
		files: make(map[string]string),
	}
}

func (fs *MockFS) ReadFile(path string) (string, bool) {
	content, exists := fs.files[path]
	return content, exists
}

func (fs *MockFS) AddFile(path string, content string) {
	fs.files[path] = content
}

// TestCriticalBugFix tests the specific @react-spring/core issue where bare package
// importers couldn't resolve relative paths
func TestCriticalBugFix(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles map[string]string
		importPath string
		importer   string
		expected   string
	}{
		{
			name: "@react-spring/core NODE_ENV conditional pattern",
			setupFiles: map[string]string{
				"/node_modules/@react-spring/core/package.json": `{
					"name": "@react-spring/core",
					"exports": {
						".": {
							"require": {
								"default": "./dist/cjs/index.js"
							}
						}
					}
				}`,
				"/node_modules/@react-spring/core/dist/cjs/index.js": `
					if (process.env.NODE_ENV === 'production') {
						module.exports = require('./react-spring_core.production.min.cjs')
					} else {
						module.exports = require('./react-spring_core.development.cjs')
					}
				`,
				"/node_modules/@react-spring/core/dist/cjs/react-spring_core.development.cjs": `module.exports = "dev"`,
			},
			importPath: "./react-spring_core.development.cjs",
			importer:   "@react-spring/core", // Bare package name, not a file path!
			expected:   "/node_modules/@react-spring/core/dist/cjs/react-spring_core.development.cjs",
		},
		{
			name: "@use-gesture/react similar pattern",
			setupFiles: map[string]string{
				"/node_modules/@use-gesture/react/package.json": `{
					"name": "@use-gesture/react",
					"exports": {
						".": {
							"require": "./dist/use-gesture-react.cjs.js"
						}
					}
				}`,
				"/node_modules/@use-gesture/react/dist/use-gesture-react.cjs.js":             `module.exports = require('./use-gesture-react.cjs.development.js')`,
				"/node_modules/@use-gesture/react/dist/use-gesture-react.cjs.development.js": `module.exports = "gesture"`,
			},
			importPath: "./use-gesture-react.cjs.development.js",
			importer:   "@use-gesture/react",
			expected:   "/node_modules/@use-gesture/react/dist/use-gesture-react.cjs.development.js",
		},
		{
			name: "bare package with only main field",
			setupFiles: map[string]string{
				"/node_modules/simple-pkg/package.json":  `{"main": "lib/index.js"}`,
				"/node_modules/simple-pkg/lib/index.js":  `require("./helper")`,
				"/node_modules/simple-pkg/lib/helper.js": `module.exports = "helper"`,
			},
			importPath: "./helper",
			importer:   "simple-pkg",
			expected:   "/node_modules/simple-pkg/lib/helper.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewMockFS()
			for path, content := range tt.setupFiles {
				fs.AddFile(path, content)
			}

			result := resolveModule(fs, tt.importPath, tt.importer)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestModuleResolution tests core module resolution functionality
func TestModuleResolution(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles map[string]string
		importPath string
		importer   string
		expected   string
	}{
		{
			name: "ESM preferred with exports field for tree-shaking",
			setupFiles: map[string]string{
				"/node_modules/test-pkg/package.json": `{
					"exports": {
						".": {
							"require": "./dist/cjs/index.js",
							"import": "./dist/esm/index.js"
						}
					}
				}`,
				"/node_modules/test-pkg/dist/cjs/index.js": `module.exports = "cjs"`,
				"/node_modules/test-pkg/dist/esm/index.js": `export default "esm"`,
			},
			importPath: "test-pkg",
			expected:   "/node_modules/test-pkg/dist/esm/index.js", // Prefer import for tree-shaking
		},
		{
			name: "nested require field in exports",
			setupFiles: map[string]string{
				"/node_modules/test-pkg/package.json": `{
					"exports": {
						".": {
							"require": {
								"default": "./dist/cjs/index.js"
							}
						}
					}
				}`,
				"/node_modules/test-pkg/dist/cjs/index.js": `module.exports = "nested"`,
			},
			importPath: "test-pkg",
			expected:   "/node_modules/test-pkg/dist/cjs/index.js",
		},
		{
			name: "scoped package with subpath",
			setupFiles: map[string]string{
				"/node_modules/@org/pkg/package.json": `{
					"exports": {
						"./sub": "./lib/sub.js"
					}
				}`,
				"/node_modules/@org/pkg/lib/sub.js": `module.exports = "subpath"`,
			},
			importPath: "@org/pkg/sub",
			expected:   "/node_modules/@org/pkg/lib/sub.js",
		},
		{
			name: "fallback to main field",
			setupFiles: map[string]string{
				"/node_modules/old-pkg/package.json": `{"main": "lib/index.js"}`,
				"/node_modules/old-pkg/lib/index.js": `module.exports = "main"`,
			},
			importPath: "old-pkg",
			expected:   "/node_modules/old-pkg/lib/index.js",
		},
		{
			name: "fallback to index.js when no package.json",
			setupFiles: map[string]string{
				"/node_modules/no-json/index.js": `module.exports = "default"`,
			},
			importPath: "no-json",
			expected:   "/node_modules/no-json/index.js",
		},
		{
			name: "relative import from file",
			setupFiles: map[string]string{
				"/node_modules/pkg/main.js":  `require("./utils")`,
				"/node_modules/pkg/utils.js": `module.exports = "utils"`,
			},
			importPath: "./utils",
			importer:   "/node_modules/pkg/main.js",
			expected:   "/node_modules/pkg/utils.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewMockFS()
			for path, content := range tt.setupFiles {
				fs.AddFile(path, content)
			}

			result := resolveModule(fs, tt.importPath, tt.importer)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestPathSecurity tests path traversal protection
func TestPathSecurity(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		shouldPass bool
	}{
		{
			name:       "allow node_modules path",
			path:       "/node_modules/pkg/index.js",
			shouldPass: true,
		},
		{
			name:       "block absolute system path",
			path:       "/etc/passwd",
			shouldPass: false,
		},
		{
			name:       "block path traversal escape",
			path:       "/node_modules/../etc/passwd",
			shouldPass: false,
		},
		{
			name:       "block relative traversal to root",
			path:       "../../../etc/passwd",
			shouldPass: false,
		},
		{
			name:       "allow relative within package",
			path:       "./utils/helper",
			shouldPass: true,
		},
		{
			name:       "block null byte injection",
			path:       "/node_modules/pkg\x00/etc/passwd",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if tt.shouldPass && err != nil {
				t.Errorf("Expected path to be valid, but got error: %v", err)
			}
			if !tt.shouldPass && err == nil {
				t.Errorf("Expected path to be blocked, but validation passed")
			}
		})
	}

	// Test path traversal with module resolution
	t.Run("prevent traversal via bare package importer", func(t *testing.T) {
		fs := NewMockFS()
		fs.AddFile("/etc/passwd", "sensitive")
		fs.AddFile("/node_modules/evil/package.json", `{"main": "index.js"}`)

		result := resolveModule(fs, "../../etc/passwd", "evil")
		if result != "" {
			t.Errorf("Should block path traversal from bare package, got: %s", result)
		}
	})
}

// TestErrorHandling tests graceful error handling
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles map[string]string
		importPath string
		expected   string // Should fallback gracefully
	}{
		{
			name: "malformed package.json",
			setupFiles: map[string]string{
				"/node_modules/bad/package.json": `{invalid json}`,
				"/node_modules/bad/index.js":     `module.exports = {}`,
			},
			importPath: "bad",
			expected:   "/node_modules/bad/index.js", // Fallback to index.js
		},
		{
			name: "non-string main field",
			setupFiles: map[string]string{
				"/node_modules/bad-main/package.json": `{"main": {"not": "string"}}`,
				"/node_modules/bad-main/index.js":     `module.exports = {}`,
			},
			importPath: "bad-main",
			expected:   "/node_modules/bad-main/index.js",
		},
		{
			name: "empty package.json",
			setupFiles: map[string]string{
				"/node_modules/empty/package.json": ``,
				"/node_modules/empty/index.js":     `module.exports = {}`,
			},
			importPath: "empty",
			expected:   "/node_modules/empty/index.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewMockFS()
			for path, content := range tt.setupFiles {
				fs.AddFile(path, content)
			}

			result := resolveModule(fs, tt.importPath, "")
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestResolvePackageExports tests the exports field parser
func TestResolvePackageExports(t *testing.T) {
	tests := []struct {
		name     string
		exports  map[string]interface{}
		subpath  string
		expected []string
	}{
		{
			name:     "string export",
			exports:  map[string]interface{}{".": "./index.js"},
			subpath:  ".",
			expected: []string{"./index.js"},
		},
		{
			name: "conditional exports",
			exports: map[string]interface{}{
				".": map[string]interface{}{
					"require": "./cjs.js",
					"import":  "./esm.js",
				},
			},
			subpath:  ".",
			expected: []string{"./esm.js", "./cjs.js"}, // Both extracted, import first for tree-shaking
		},
		{
			name: "nested default in require",
			exports: map[string]interface{}{
				".": map[string]interface{}{
					"require": map[string]interface{}{
						"default": "./index.cjs",
					},
				},
			},
			subpath:  ".",
			expected: []string{"./index.cjs"},
		},
		{
			name: "subpath export",
			exports: map[string]interface{}{
				"./utils": "./lib/utils.js",
			},
			subpath:  "./utils",
			expected: []string{"./lib/utils.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := resolvePackageExports(tt.exports, tt.subpath)

			if len(results) != len(tt.expected) {
				t.Fatalf("Expected %d results, got %d: %v", len(tt.expected), len(results), results)
			}

			for i, exp := range tt.expected {
				if results[i] != exp {
					t.Errorf("Result %d: expected %s, got %s", i, exp, results[i])
				}
			}
		})
	}
}

// TestSubpathExportRelativeImports tests that relative imports from files
// loaded via subpath exports resolve correctly relative to the actual file location,
// not relative to the subpath name.
//
// Bug reproduction: zod-schema-faker/v4
//  1. zod-schema-faker/v4 resolves to ./dist/v4/zod-schema-faker.cjs via exports
//  2. That file contains require("../randexp-IynBq8em.cjs")
//  3. Should resolve to ./dist/randexp-IynBq8em.cjs (going up from dist/v4/)
//  4. Bug: was resolving to ./v4/randexp-IynBq8em.cjs (treating "v4" as the dir)
func TestSubpathExportRelativeImports(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles map[string]string
		importPath string
		importer   string
		expected   string
	}{
		{
			name: "relative import with ../ from subpath export",
			setupFiles: map[string]string{
				"/node_modules/zod-schema-faker/package.json": `{
					"name": "zod-schema-faker",
					"exports": {
						"./v4": {
							"require": "./dist/v4/zod-schema-faker.cjs"
						}
					}
				}`,
				"/node_modules/zod-schema-faker/dist/v4/zod-schema-faker.cjs": `require("../randexp-IynBq8em.cjs")`,
				"/node_modules/zod-schema-faker/dist/randexp-IynBq8em.cjs":    `module.exports = "randexp"`,
			},
			importPath: "../randexp-IynBq8em.cjs",
			// The importer should be the RESOLVED path, not the bare subpath
			importer: "/node_modules/zod-schema-faker/dist/v4/zod-schema-faker.cjs",
			expected: "/node_modules/zod-schema-faker/dist/randexp-IynBq8em.cjs",
		},
		{
			name: "relative import with ../ from subpath export - bare importer (current bug)",
			setupFiles: map[string]string{
				"/node_modules/zod-schema-faker/package.json": `{
					"name": "zod-schema-faker",
					"exports": {
						"./v4": {
							"require": "./dist/v4/zod-schema-faker.cjs"
						}
					}
				}`,
				"/node_modules/zod-schema-faker/dist/v4/zod-schema-faker.cjs": `require("../randexp-IynBq8em.cjs")`,
				"/node_modules/zod-schema-faker/dist/randexp-IynBq8em.cjs":    `module.exports = "randexp"`,
			},
			importPath: "../randexp-IynBq8em.cjs",
			// When importer is the bare subpath (what esbuild passes), resolution should
			// still find the correct file by resolving the subpath first
			importer: "zod-schema-faker/v4",
			expected: "/node_modules/zod-schema-faker/dist/randexp-IynBq8em.cjs",
		},
		{
			name: "relative import with ./ from subpath export - bare importer",
			setupFiles: map[string]string{
				"/node_modules/pkg/package.json": `{
					"exports": {
						"./sub": "./lib/deep/entry.js"
					}
				}`,
				"/node_modules/pkg/lib/deep/entry.js":  `require("./helper.js")`,
				"/node_modules/pkg/lib/deep/helper.js": `module.exports = "helper"`,
			},
			importPath: "./helper.js",
			importer:   "pkg/sub",
			expected:   "/node_modules/pkg/lib/deep/helper.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewMockFS()
			for path, content := range tt.setupFiles {
				fs.AddFile(path, content)
			}

			result := resolveModule(fs, tt.importPath, tt.importer)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestResolutionPriority verifies correct priority order
func TestResolutionPriority(t *testing.T) {
	t.Run("exports field takes precedence over main", func(t *testing.T) {
		fs := NewMockFS()
		fs.AddFile("/node_modules/pkg/package.json", `{
			"main": "old.js",
			"exports": {".": "./new.js"}
		}`)
		fs.AddFile("/node_modules/pkg/old.js", `module.exports = "old"`)
		fs.AddFile("/node_modules/pkg/new.js", `module.exports = "new"`)

		result := resolveModule(fs, "pkg", "")
		if result != "/node_modules/pkg/new.js" {
			t.Errorf("Should prefer exports over main, got: %s", result)
		}
	})

	t.Run("import takes precedence over require for tree-shaking", func(t *testing.T) {
		fs := NewMockFS()
		fs.AddFile("/node_modules/pkg/package.json", `{
			"exports": {
				".": {
					"import": "./esm.js",
					"require": "./cjs.js"
				}
			}
		}`)
		fs.AddFile("/node_modules/pkg/cjs.js", `module.exports = "cjs"`)
		fs.AddFile("/node_modules/pkg/esm.js", `export default "esm"`)

		result := resolveModule(fs, "pkg", "")
		if result != "/node_modules/pkg/esm.js" {
			t.Errorf("Should prefer import over require for tree-shaking, got: %s", result)
		}
	})
}
