package bridge

import (
	"strings"
	"testing"
)

// MockFileResolver implements FileResolver for testing
type MockFileResolver struct {
	files       map[string]string
	directories map[string]bool
	writes      map[string]string
}

func NewMockFileResolver() *MockFileResolver {
	return &MockFileResolver{
		files:       make(map[string]string),
		directories: make(map[string]bool),
		writes:      make(map[string]string),
	}
}

func (m *MockFileResolver) WithFile(path, content string) *MockFileResolver {
	m.files[path] = content
	return m
}

func (m *MockFileResolver) WithDirectory(path string) *MockFileResolver {
	m.directories[path] = true
	return m
}

func (m *MockFileResolver) ResolveFile(path string) string {
	// Check written files first (for output files)
	if content, exists := m.writes[path]; exists {
		return content
	}
	// Then check input files
	return m.files[path]
}

func (m *MockFileResolver) FileExists(path string) bool {
	_, existsInWrites := m.writes[path]
	_, existsInFiles := m.files[path]
	return existsInWrites || existsInFiles
}

func (m *MockFileResolver) DirectoryExists(path string) bool {
	return m.directories[path]
}

func (m *MockFileResolver) WriteFile(path string, content string) bool {
	m.writes[path] = content
	return true
}

func (m *MockFileResolver) GetAllPaths(directory string) *PathList {
	var paths []string

	// Add all file paths under the directory
	for path := range m.files {
		if strings.HasPrefix(path, directory) {
			paths = append(paths, path)
		}
	}

	// Add all directory paths under the directory
	for path := range m.directories {
		if strings.HasPrefix(path, directory) {
			paths = append(paths, path)
		}
	}

	// Add all written file paths under the directory
	for path := range m.writes {
		if strings.HasPrefix(path, directory) {
			paths = append(paths, path)
		}
	}

	return &PathList{Paths: paths}
}

func TestCleanAPI_FileSystem(t *testing.T) {
	// Test filesystem-only build (no custom resolver)
	result, err := BridgeBuildWithFileSystem("./testdata/simple", false, "")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should have some basic validation
	if len(result.Diagnostics) < 0 {
		t.Error("Expected diagnostics array to be initialized")
	}
}

func TestCleanAPI_WithResolver_Success(t *testing.T) {
	// Create a mock resolver with a valid TypeScript project
	resolver := NewMockFileResolver().
		WithDirectory("/project").
		WithFile("/project/tsconfig.json", `{
			"compilerOptions": {
				"target": "es2015",
				"module": "commonjs",
				"noEmit": true
			},
			"include": ["**/*.ts"]
		}`).
		WithFile("/project/main.ts", `
			function greet(name: string): string {
				return "Hello, " + name + "!";
			}
			console.log(greet("World"));
		`)

	// Build with the resolver
	result, err := BridgeBuildWithFileResolver("/project", false, "", resolver)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful build, got failure")
		for _, diag := range result.Diagnostics {
			t.Logf("Diagnostic: [%s] %s", diag.Category, diag.Message)
		}
	}

	if len(result.Diagnostics) > 0 {
		// Log diagnostics for debugging but don't fail - might be warnings
		for _, diag := range result.Diagnostics {
			t.Logf("Diagnostic: TS%d [%s] %s", diag.Code, diag.Category, diag.Message)
		}
	}
}

func TestCleanAPI_WithResolver_TypeErrors(t *testing.T) {
	// Create a mock resolver with TypeScript errors
	resolver := NewMockFileResolver().
		WithDirectory("/project").
		WithFile("/project/tsconfig.json", `{
			"compilerOptions": {
				"target": "es2015",
				"module": "commonjs",
				"strict": true,
				"noEmit": true
			},
			"include": ["**/*.ts"]
		}`).
		WithFile("/project/main.ts", `
			function greet(name) { // Missing type annotation
				return "Hello, " + name + "!";
			}
			let x: number = "hello"; // Type error
			console.log(greet("World"));
		`)

	// Build with the resolver
	result, err := BridgeBuildWithFileResolver("/project", false, "", resolver)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Success {
		t.Error("Expected build to fail due to type errors")
	}

	if len(result.Diagnostics) == 0 {
		t.Error("Expected diagnostics for type errors")
	}

	// Check for specific error types
	hasTypeError := false
	for _, diag := range result.Diagnostics {
		if diag.Category == "error" {
			hasTypeError = true
			t.Logf("Found error: TS%d - %s", diag.Code, diag.Message)
		}
	}

	if !hasTypeError {
		t.Error("Expected at least one type error")
	}
}

func TestCleanAPI_WithResolver_EmitFiles(t *testing.T) {
	// Create a mock resolver that allows file emission
	resolver := NewMockFileResolver().
		WithDirectory("/project").
		WithDirectory("/project/dist").
		WithFile("/project/tsconfig.json", `{
			"compilerOptions": {
				"target": "es2015",
				"module": "commonjs",
				"outDir": "./dist",
				"noEmit": false
			},
			"include": ["**/*.ts"]
		}`).
		WithFile("/project/main.ts", `
			function greet(name: string): string {
				return "Hello, " + name + "!";
			}
			console.log(greet("World"));
		`)

	// Build with the resolver
	result, err := BridgeBuildWithFileResolver("/project", false, "", resolver)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful build, got failure")
		for _, diag := range result.Diagnostics {
			t.Logf("Diagnostic: [%s] %s", diag.Category, diag.Message)
		}
	}

	// Check that files were written
	if len(resolver.writes) == 0 {
		t.Error("Expected files to be written during compilation")
	} else {
		t.Logf("Files written during compilation:")
		for path, content := range resolver.writes {
			t.Logf("  %s: %d bytes", path, len(content))
		}
	}

	// Check WrittenFiles in result
	if result.WrittenFiles == nil {
		t.Error("Expected WrittenFiles to be populated")
	} else if len(result.WrittenFiles) == 0 {
		t.Error("Expected WrittenFiles to contain emitted files")
	}
}

func TestCleanAPI_MultipleBuilds_NoState(t *testing.T) {
	// Test that multiple builds don't interfere with each other

	// First build - success
	resolver1 := NewMockFileResolver().
		WithDirectory("/project1").
		WithFile("/project1/tsconfig.json", `{"compilerOptions": {"noEmit": true}, "include": ["**/*.ts"]}`).
		WithFile("/project1/main.ts", `const x: number = 42;`)

	result1, err1 := BridgeBuildWithFileResolver("/project1", false, "", resolver1)
	if err1 != nil {
		t.Fatalf("First build failed: %v", err1)
	}

	// Second build - failure
	resolver2 := NewMockFileResolver().
		WithDirectory("/project2").
		WithFile("/project2/tsconfig.json", `{"compilerOptions": {"strict": true, "noEmit": true}, "include": ["**/*.ts"]}`).
		WithFile("/project2/main.ts", `let x: number = "hello";`)

	result2, err2 := BridgeBuildWithFileResolver("/project2", false, "", resolver2)
	if err2 != nil {
		t.Fatalf("Second build failed: %v", err2)
	}

	// First build should still be successful (no state pollution)
	if !result1.Success {
		t.Error("First build result was corrupted by second build")
		t.Logf("Result1 diagnostics:")
		for _, diag := range result1.Diagnostics {
			t.Logf("  TS%d [%s] %s", diag.Code, diag.Category, diag.Message)
		}
	}

	// Second build should have failed
	if result2.Success {
		t.Error("Second build should have failed due to type error")
	}

	// Results should be independent
	if len(result1.Diagnostics) > 0 {
		t.Error("First build should have no diagnostics")
		t.Logf("Result1 diagnostics count: %d", len(result1.Diagnostics))
		for _, diag := range result1.Diagnostics {
			t.Logf("  TS%d [%s] %s", diag.Code, diag.Category, diag.Message)
		}
	}
}

func TestCleanAPI_FileDiscovery_WithPatterns(t *testing.T) {
	// Test that TypeScript can discover files through include patterns
	resolver := NewMockFileResolver().
		WithDirectory("/project").
		WithDirectory("/project/src").
		WithDirectory("/project/src/utils").
		WithFile("/project/tsconfig.json", `{
				"compilerOptions": {
					"target": "es2015",
					"module": "commonjs",
					"noEmit": true
				},
				"include": ["src/**/*.ts"]
			}`).
		WithFile("/project/src/main.ts", `
				import { add } from './utils/math';
				console.log(add(5, 3));
			`).
		WithFile("/project/src/utils/math.ts", `
				export function add(a: number, b: number): number {
					return a + b;
				}
			`)

	// Build with the resolver
	result, err := BridgeBuildWithFileResolver("/project", false, "", resolver)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful build with file discovery, got failure")
		for _, diag := range result.Diagnostics {
			t.Logf("Diagnostic: [%s] %s", diag.Category, diag.Message)
		}
	}

	// Should have no errors since both files should be discovered
	errorCount := 0
	for _, diag := range result.Diagnostics {
		if diag.Category == "error" {
			errorCount++
			t.Logf("Error: TS%d - %s", diag.Code, diag.Message)
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors with proper file discovery, got %d errors", errorCount)
	}
}
