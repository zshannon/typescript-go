package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBridgeBuildWithConfig_Success(t *testing.T) {
	// Create a temporary directory with a simple TypeScript project
	tempDir := t.TempDir()

	// Create tsconfig.json
	tsconfigContent := `{
		"compilerOptions": {
			"target": "es2015",
			"module": "commonjs",
			"outDir": "./dist"
		},
		"include": ["src/**/*"]
	}`

	err := os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create tsconfig.json: %v", err)
	}

	// Create src directory and a simple TypeScript file
	srcDir := filepath.Join(tempDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	tsContent := `function greet(name: string): string {
		return "Hello, " + name + "!";
	}

	console.log(greet("World"));`

	err = os.WriteFile(filepath.Join(srcDir, "main.ts"), []byte(tsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create main.ts: %v", err)
	}

	// Test the bridge function
	result, err := BridgeBuildWithConfig(tempDir, false, "")
	if err != nil {
		t.Fatalf("Expected successful compilation, got error: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful compilation, got Success=false")
	}

	if result.ConfigFile == "" {
		t.Errorf("Expected ConfigFile to be set, got empty string")
	}

	// There should be no diagnostics for a successful build
	if result.DiagnosticCount > 0 {
		t.Errorf("Expected no diagnostics for successful build, got %d", result.DiagnosticCount)

		// Print diagnostics for debugging
		for i := 0; i < result.DiagnosticCount; i++ {
			diag := GetLastDiagnostic(i)
			if diag != nil {
				t.Logf("Diagnostic %d: %s - %s", i, diag.Category, diag.Message)
			}
		}
	}
}

func TestBridgeBuildWithConfig_Error(t *testing.T) {
	// Create a temporary directory with a TypeScript project that has errors
	tempDir := t.TempDir()

	// Create tsconfig.json
	tsconfigContent := `{
		"compilerOptions": {
			"target": "es2015",
			"module": "commonjs",
			"strict": true
		}
	}`

	err := os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create tsconfig.json: %v", err)
	}

	// Create a TypeScript file with an error
	tsContent := `function greet(name) { // Missing type annotation in strict mode
		return "Hello, " + name + "!";
	}

	let x: number = "hello"; // Type error
	console.log(greet("World"));`

	err = os.WriteFile(filepath.Join(tempDir, "main.ts"), []byte(tsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create main.ts: %v", err)
	}

	// Test the bridge function
	result, buildErr := BridgeBuildWithConfig(tempDir, false, "")

	// Should not return an error for compilation failures (only for system errors)
	if buildErr != nil {
		t.Errorf("Expected no error for compilation failure, got: %v", buildErr)
	}

	if result == nil {
		t.Fatalf("Expected result struct, got nil")
	}

	if result.Success {
		t.Errorf("Expected failed compilation, got Success=true")
	}

	if result.DiagnosticCount == 0 {
		t.Errorf("Expected diagnostics for failed build, got 0")
	}

	// Test diagnostic retrieval
	for i := 0; i < result.DiagnosticCount; i++ {
		diag := GetLastDiagnostic(i)
		if diag == nil {
			t.Errorf("Expected diagnostic at index %d, got nil", i)
			continue
		}

		if diag.Code == 0 {
			t.Errorf("Expected non-zero diagnostic code, got 0")
		}

		if diag.Message == "" {
			t.Errorf("Expected non-empty diagnostic message, got empty string")
		}

		if diag.Category == "" {
			t.Errorf("Expected non-empty diagnostic category, got empty string")
		}

		t.Logf("Diagnostic %d: TS%d [%s] %s", i, diag.Code, diag.Category, diag.Message)
	}
}

func TestGetLastDiagnostic_OutOfBounds(t *testing.T) {
	// Test behavior when accessing diagnostics out of bounds

	// Reset global state
	lastBuildDiagnostics = nil

	// Should return nil for any index when no diagnostics exist
	diag := GetLastDiagnostic(0)
	if diag != nil {
		t.Errorf("Expected nil diagnostic for empty state, got %+v", diag)
	}

	diag = GetLastDiagnostic(-1)
	if diag != nil {
		t.Errorf("Expected nil diagnostic for negative index, got %+v", diag)
	}

	// Set some test diagnostics
	lastBuildDiagnostics = []DiagnosticInfo{
		{Code: 1, Category: "error", Message: "Test error"},
	}

	// Valid index should work
	diag = GetLastDiagnostic(0)
	if diag == nil {
		t.Errorf("Expected diagnostic at valid index, got nil")
	}

	// Out of bounds should return nil
	diag = GetLastDiagnostic(1)
	if diag != nil {
		t.Errorf("Expected nil diagnostic for out-of-bounds index, got %+v", diag)
	}
}

func TestGetLastEmittedFile_OutOfBounds(t *testing.T) {
	// Test behavior when accessing emitted files out of bounds

	// Reset global state
	lastBuildEmittedFiles = nil

	// Should return empty string for any index when no files exist
	file := GetLastEmittedFile(0)
	if file != "" {
		t.Errorf("Expected empty string for empty state, got %q", file)
	}

	file = GetLastEmittedFile(-1)
	if file != "" {
		t.Errorf("Expected empty string for negative index, got %q", file)
	}

	// Set some test files
	lastBuildEmittedFiles = []string{"test.js"}

	// Valid index should work
	file = GetLastEmittedFile(0)
	if file != "test.js" {
		t.Errorf("Expected 'test.js' at valid index, got %q", file)
	}

	// Out of bounds should return empty string
	file = GetLastEmittedFile(1)
	if file != "" {
		t.Errorf("Expected empty string for out-of-bounds index, got %q", file)
	}
}
