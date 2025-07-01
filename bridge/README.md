# TypeScript Go Bridge

This package provides a bridge between Go and TypeScript compilation, designed to be compatible with gomobile for use in iOS/Android applications.

## Overview

The bridge exposes TypeScript compilation functionality through a simple API that can be called from Swift/Objective-C (iOS) or Java/Kotlin (Android) via gomobile.

## Key Features

- **TypeScript Compilation**: Compile TypeScript projects with full diagnostic support
- **Custom File Resolution**: Provide TypeScript source files from memory instead of filesystem
- **Gomobile Compatible**: All APIs are designed to work with gomobile limitations
- **Diagnostic Details**: Get detailed error/warning information with line/column numbers
- **Flexible Configuration**: Support for custom tsconfig.json and compiler options

## Basic Usage

### Simple Compilation

```go
// Compile a TypeScript project from filesystem
result, err := BridgeBuildWithConfig("/path/to/project", true, "")
if err != nil {
    // Handle system errors
    log.Fatal(err)
}

if !result.Success {
    // Handle compilation errors
    for i := 0; i < result.DiagnosticCount; i++ {
        diag := GetLastDiagnostic(i)
        fmt.Printf("Error: %s\n", diag.Message)
    }
}
```

### Compilation with Custom File Resolution

```go
// Set up file resolver
resolver := NewSimpleFileResolver()
resolver.AddFile("/project/main.ts", `
    function greet(name: string): string {
        return "Hello, " + name + "!";
    }
    console.log(greet("World"));
`)
resolver.AddFile("/project/tsconfig.json", `{
    "compilerOptions": {
        "target": "es2015",
        "module": "commonjs",
        "noEmit": true
    }
}`)
resolver.AddDirectory("/project")

// Use the resolver
SetFileResolver(resolver)
defer ClearFileResolver()

result, err := BridgeBuildWithConfig("/project", false, "")
```

### Compilation with Custom File Resolution

The most powerful feature is the ability to provide TypeScript source files from memory without touching the filesystem:

```go
// Set up file resolver
resolver := NewSimpleFileResolver()
resolver.AddFile("/project/main.ts", `
    function greet(name: string): string {
        return "Hello, " + name + "!";
    }
    console.log(greet("World"));
`)
resolver.AddFile("/project/tsconfig.json", `{
    "compilerOptions": {
        "target": "es2015",
        "module": "commonjs",
        "noEmit": true
    }
}`)
resolver.AddDirectory("/project")

// Use the resolver
SetFileResolver(resolver)
defer ClearFileResolver()

result, err := BridgeBuildWithConfig("/project", false, "")
```

## API Reference

### Core Functions

#### `BridgeBuildWithConfig(projectPath string, printErrors bool, configFile string) (*BridgeResult, error)`

Compiles a TypeScript project.

**Parameters:**
- `projectPath`: Path to the project directory or tsconfig.json file
- `printErrors`: Whether to print compilation errors to stdout
- `configFile`: Optional custom config file path

**Returns:**
- `BridgeResult`: Compilation result with success status and diagnostic counts
- `error`: System-level errors (not compilation errors)

#### `GetLastDiagnostic(index int) *BridgeDiagnostic`

Retrieves diagnostic information by index from the last compilation.

**Parameters:**
- `index`: Zero-based index of the diagnostic

**Returns:**
- `BridgeDiagnostic`: Diagnostic details, or `nil` if index is out of bounds

#### `GetLastEmittedFile(index int) string`

Retrieves emitted file path by index from the last compilation.

**Parameters:**
- `index`: Zero-based index of the emitted file

**Returns:**
- `string`: File path, or empty string if index is out of bounds

### File Resolution

#### `SetFileResolver(resolver FileResolver)`

Sets a global file resolver for subsequent compilations.

#### `ClearFileResolver()`

Clears the global file resolver, reverting to filesystem-based resolution.

#### `FileResolver` Interface

Implement this interface to provide custom file resolution:

```go
type FileResolver interface {
    ResolveFile(path string) string        // Return file contents or empty string
    FileExists(path string) bool           // Check if file exists
    DirectoryExists(path string) bool      // Check if directory exists
}
```

### Data Structures

#### `BridgeResult`

```go
type BridgeResult struct {
    Success          bool   // Whether compilation succeeded
    ConfigFile       string // Resolved config file path
    DiagnosticCount  int    // Number of diagnostics (errors/warnings)
    EmittedFileCount int    // Number of files emitted
}
```

#### `BridgeDiagnostic`

```go
type BridgeDiagnostic struct {
    Code     int    // TypeScript diagnostic code (e.g., 2345)
    Category string // "error", "warning", "info", etc.
    Message  string // Human-readable diagnostic message
    File     string // Source file path (may be empty)
    Line     int    // Line number (1-based, 0 if not available)
    Column   int    // Column number (1-based, 0 if not available)
    Length   int    // Length of affected text (0 if not available)
}
```

## Usage from Swift (iOS)

After generating the framework with gomobile:

```swift
import TypescriptGo

// Set up file resolver
let resolver = TypescriptGoNewSimpleFileResolver()
resolver?.addFile("/project/main.ts", "console.log('Hello from Swift!');")
resolver?.addFile("/project/tsconfig.json", "{\"compilerOptions\":{\"noEmit\":true}}")
resolver?.addDirectory("/project")

// Compile
TypescriptGoSetFileResolver(resolver)
defer { TypescriptGoClearFileResolver() }

var error: NSError?
if let result = TypescriptGoBridgeBuildWithConfig("/project", false, "", &error) {
    if result.success {
        print("Compilation successful!")
    } else {
        print("Compilation failed with \(result.diagnosticCount) diagnostics")
        
        for i in 0..<result.diagnosticCount {
            if let diagnostic = TypescriptGoGetLastDiagnostic(i) {
                print("Error: \(diagnostic.message)")
            }
        }
    }
}
```

## Advanced Usage

### Custom File Resolver Implementation

For more complex scenarios, implement the `FileResolver` interface:

```go
type MyCustomResolver struct {
    files map[string]string
    // ... other fields
}

func (r *MyCustomResolver) ResolveFile(path string) string {
    // Custom logic to resolve files
    return r.files[path]
}

func (r *MyCustomResolver) FileExists(path string) bool {
    _, exists := r.files[path]
    return exists
}

func (r *MyCustomResolver) DirectoryExists(path string) bool {
    // Custom directory existence logic
    return true
}
```

### Working with TypeScript Configurations

The bridge supports all standard TypeScript compiler options through tsconfig.json:

```json
{
  "compilerOptions": {
    "target": "es2015",
    "module": "commonjs",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "noEmit": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

## Error Handling

The bridge distinguishes between system errors and compilation errors:

- **System Errors**: Returned as Go `error` from bridge functions (e.g., file system issues)
- **Compilation Errors**: Communicated through `BridgeResult.Success = false` and diagnostic information

Always check both the error return value and the success flag:

```go
result, err := BridgeBuildWithConfig("/project", false, "")
if err != nil {
    // Handle system errors (file not found, permissions, etc.)
    log.Fatal(err)
}

if !result.Success {
    // Handle TypeScript compilation errors
    for i := 0; i < result.DiagnosticCount; i++ {
        diag := GetLastDiagnostic(i)
        if diag.Category == "error" {
            fmt.Printf("TS%d: %s at %s:%d:%d\n", 
                diag.Code, diag.Message, diag.File, diag.Line, diag.Column)
        }
    }
}
```

## Building with gomobile

To generate iOS/Android bindings:

```bash
# Install gomobile
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

# Generate iOS framework
gomobile bind -target=ios github.com/microsoft/typescript-go/bridge

# Generate Android AAR
gomobile bind -target=android github.com/microsoft/typescript-go/bridge
```

## Limitations

- File resolution is currently optimized for the `SimpleFileResolver` implementation
- Directory walking is limited to known paths in the file resolver
- Some advanced TypeScript features may require filesystem access for full functionality
- Output file generation requires real filesystem paths (use `noEmit: true` for pure validation)

## Examples

See the test files for comprehensive examples:
- `bridge_test.go`: Basic compilation and error handling
- File resolver tests: Custom file resolution examples

### Swift Usage Examples

If you're using the Swift package wrapper:

```swift
import SwiftTSGo

// Simple validation
let code = """
    function greet(name: string): string {
        return "Hello, " + name + "!";
    }
    """
let result = validateTypeScript(code)

// Multi-file project
let sources = [
    Source(name: "math.ts", content: "export function add(a: number, b: number) { return a + b; }"),
    Source(name: "main.ts", content: "import { add } from './math'; console.log(add(5, 3));")
]
let result = buildInMemory(sources)

// Using SimpleFileResolver directly
let files = [
    "/project/tsconfig.json": "{ \"compilerOptions\": { \"noEmit\": true } }",
    "/project/test.ts": "const x: number = 42;"
]
let result = buildWithSimpleResolver(files, directories: ["/project"])
```