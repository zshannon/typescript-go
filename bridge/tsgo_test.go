package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuild_CapturesCompilationErrors(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a TypeScript project with an error
	srcDir := filepath.Join(tempDir, "src")
	err := os.MkdirAll(srcDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	// Create a TypeScript file with a type error
	tsFile := filepath.Join(srcDir, "error.ts")
	tsContent := `function add(a: number, b: number): number {
    return a + b;
}

// This should cause a type error
const result = add("hello", 5);
console.log(result);
`
	err = os.WriteFile(tsFile, []byte(tsContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write TypeScript file: %v", err)
	}

	// Create a tsconfig.json
	tsconfigFile := filepath.Join(tempDir, "tsconfig.json")
	tsconfigContent := `{
  "compilerOptions": {
    "target": "es2020",
    "module": "commonjs",
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src/**/*"]
}
`
	err = os.WriteFile(tsconfigFile, []byte(tsconfigContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write tsconfig.json: %v", err)
	}

	// Call Build function - it should return an error with compilation details
	err = Build(tempDir)

	// Verify that we got an error
	if err == nil {
		t.Fatal("Expected Build to return an error due to TypeScript compilation error, but got nil")
	}

	// Verify that the error message contains expected compilation error details
	errorMsg := err.Error()
	expectedSubstrings := []string{
		"compilation failed:",
		"TS2345",
		"Argument of type 'string' is not assignable to parameter of type 'number'",
		"error.ts",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(errorMsg, expected) {
			t.Errorf("Expected error message to contain '%s', but got: %s", expected, errorMsg)
		}
	}
}

func TestBuild_SuccessfulCompilation(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a TypeScript project without errors
	srcDir := filepath.Join(tempDir, "src")
	err := os.MkdirAll(srcDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	// Create a valid TypeScript file
	tsFile := filepath.Join(srcDir, "index.ts")
	tsContent := `function add(a: number, b: number): number {
    return a + b;
}

const result = add(2, 3);
console.log(result);
`
	err = os.WriteFile(tsFile, []byte(tsContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write TypeScript file: %v", err)
	}

	// Create a tsconfig.json
	tsconfigFile := filepath.Join(tempDir, "tsconfig.json")
	tsconfigContent := `{
  "compilerOptions": {
    "target": "es2020",
    "module": "commonjs",
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src/**/*"]
}
`
	err = os.WriteFile(tsconfigFile, []byte(tsconfigContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write tsconfig.json: %v", err)
	}

	// Call Build function - it should succeed
	err = Build(tempDir)
	// Verify that we didn't get an error
	if err != nil {
		t.Fatalf("Expected Build to succeed, but got error: %v", err)
	}

	// Verify that the dist directory was created
	distDir := filepath.Join(tempDir, "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		t.Error("Expected dist directory to be created, but it doesn't exist")
	}
}
