# SwiftTSGo

A Swift package that exposes TypeScript compilation capabilities to Swift applications. This package provides a native bridge to the TypeScript compiler, allowing you to compile TypeScript code directly from Swift with rich diagnostic information and structured results.

## Features

- ✅ **Native TypeScript Compilation**: Compile TypeScript code directly from Swift
- ✅ **Rich Diagnostics**: Detailed error information with file locations, error codes, and categories
- ✅ **Structured Results**: Type-safe compilation results with success status and emitted files
- ✅ **Flexible Configuration**: Customizable build options including error reporting and config file paths
- ✅ **Cross-Platform**: Works on iOS, macOS, and other Swift-supported platforms
- ✅ **Silent by Default**: No unwanted console output during compilation

## Installation

### Swift Package Manager

Add SwiftTSGo to your project using Swift Package Manager:

```swift
dependencies: [
    .package(url: "https://github.com/zshannon/typescript-go.git", from: "0.0.1")
]
```

Then add it to your target dependencies:

```swift
targets: [
    .target(
        name: "YourTarget",
        dependencies: ["SwiftTSGo"]
    )
]
```

## Usage

### Basic Example

```swift
import SwiftTSGo

// Configure the TypeScript compilation
let config = BuildConfig(
    projectPath: "/path/to/your/typescript/project",
    printErrors: false
)

// Compile the TypeScript project
let result = buildWithConfig(config)

// Check the result
if result.success {
    print("✅ Compilation successful!")
    print("Config used: \(result.configFile)")
    print("Files emitted: \(result.emittedFiles.count)")
} else {
    print("❌ Compilation failed with \(result.diagnostics.count) diagnostics")

    // Handle errors
    for diagnostic in result.diagnostics where diagnostic.category == "error" {
        print("Error TS\(diagnostic.code): \(diagnostic.message)")
        if !diagnostic.file.isEmpty {
            print("  at \(diagnostic.file):\(diagnostic.line):\(diagnostic.column)")
        }
    }
}
```

### Advanced Error Handling

```swift
import SwiftTSGo

func compileTypeScript(projectPath: String) -> CompilationResult {
    let config = BuildConfig(
        projectPath: projectPath,
        printErrors: false
    )

    let result = buildWithConfig(config)

    // Categorize diagnostics
    let errors = result.diagnostics.filter { $0.category == "error" }
    let warnings = result.diagnostics.filter { $0.category == "warning" }

    // Create detailed result
    return CompilationResult(
        success: result.success,
        errorCount: errors.count,
        warningCount: warnings.count,
        outputFiles: result.emittedFiles,
        detailedErrors: errors.map { diagnostic in
            ErrorDetail(
                code: diagnostic.code,
                message: diagnostic.message,
                location: diagnostic.file.isEmpty ? nil :
                    FileLocation(
                        file: diagnostic.file,
                        line: diagnostic.line,
                        column: diagnostic.column
                    )
            )
        }
    )
}

struct CompilationResult {
    let success: Bool
    let errorCount: Int
    let warningCount: Int
    let outputFiles: [String]
    let detailedErrors: [ErrorDetail]
}

struct ErrorDetail {
    let code: Int
    let message: String
    let location: FileLocation?
}

struct FileLocation {
    let file: String
    let line: Int
    let column: Int
}
```

### Custom Configuration

```swift
import SwiftTSGo

// Using custom tsconfig.json location
let config = BuildConfig(
    projectPath: "/path/to/project",
    printErrors: true,  // Enable console output for debugging
    configFile: "/path/to/custom/tsconfig.json"
)

let result = buildWithConfig(config)
```

### Batch Processing

```swift
import SwiftTSGo

func compileMultipleProjects(_ projectPaths: [String]) async -> [String: BuildResult] {
    var results: [String: BuildResult] = [:]

    // Process projects sequentially to avoid global state race conditions
    for projectPath in projectPaths {
        let config = BuildConfig(projectPath: projectPath)
        let result = buildWithConfig(config)
        results[projectPath] = result

        // Optional: Add delay to ensure clean state between compilations
        try? await Task.sleep(nanoseconds: 100_000_000) // 0.1 seconds
    }

    return results
}
```

## API Reference

### BuildConfig

Configuration options for TypeScript compilation.

```swift
public struct BuildConfig {
    public let projectPath: String      // Path to project directory or tsconfig.json
    public let printErrors: Bool        // Whether to print errors to console
    public let configFile: String?      // Custom config file path (optional)

    public init(
        projectPath: String = ".",
        printErrors: Bool = false,
        configFile: String? = nil
    )
}
```

### BuildResult

Result of a TypeScript compilation with detailed information.

```swift
public struct BuildResult {
    public let success: Bool            // Whether compilation succeeded
    public let diagnostics: [DiagnosticInfo]  // All diagnostics (errors, warnings, etc.)
    public let emittedFiles: [String]   // List of files that were generated
    public let configFile: String       // Resolved config file that was used
}
```

### DiagnosticInfo

Detailed information about a compilation diagnostic.

```swift
public struct DiagnosticInfo {
    public let code: Int                // Diagnostic code (e.g., 2345)
    public let category: String         // Category: "error", "warning", "info", etc.
    public let message: String          // Human-readable diagnostic message
    public let file: String             // Source file (empty if not available)
    public let line: Int                // Line number (1-based, 0 if not available)
    public let column: Int              // Column number (1-based, 0 if not available)
    public let length: Int              // Length of affected text (0 if not available)
}
```

### Functions

#### buildWithConfig

Main compilation function that returns structured results.

```swift
public func buildWithConfig(_ config: BuildConfig) -> BuildResult
```

**Parameters:**
- `config`: Configuration for the compilation

**Returns:**
- `BuildResult`: Detailed compilation results

## Error Handling

### Common TypeScript Error Codes

| Code | Description | Example |
|------|-------------|---------|
| 2345 | Type assignment error | `Argument of type 'string' is not assignable to parameter of type 'number'` |
| 2322 | Type mismatch | `Type 'string' is not assignable to type 'number'` |
| 2304 | Cannot find name | `Cannot find name 'variableName'` |
| 7006 | Implicit any type | `Parameter 'param' implicitly has an 'any' type` |
| 2339 | Property does not exist | `Property 'prop' does not exist on type 'Object'` |

### Filtering Diagnostics

```swift
let result = buildWithConfig(config)

// Get only errors
let errors = result.diagnostics.filter { $0.category == "error" }

// Get specific error codes
let typeErrors = result.diagnostics.filter { [2345, 2322].contains($0.code) }

// Get diagnostics with file locations
let locatedDiagnostics = result.diagnostics.filter { !$0.file.isEmpty }
```

## Project Structure Requirements

Your TypeScript project should have a `tsconfig.json` file:

```json
{
    "compilerOptions": {
        "target": "ES2020",
        "module": "commonjs",
        "outDir": "./dist",
        "rootDir": "./src",
        "strict": true,
        "esModuleInterop": true,
        "skipLibCheck": true,
        "forceConsistentCasingInFileNames": true
    },
    "include": [
        "src/**/*"
    ],
    "exclude": [
        "node_modules"
    ]
}
```

## Threading Considerations

**Important**: The C bridge implementation is thread-safe for compilation operations. However, avoid calling `buildWithConfig` concurrently from multiple threads as the underlying TypeScript compiler may have resource contention issues.

For concurrent compilation, process projects sequentially or add synchronization:

```swift
import SwiftTSGo

actor TypeScriptCompiler {
    func compile(config: BuildConfig) -> BuildResult {
        return buildWithConfig(config)
    }
}

// Usage
let compiler = TypeScriptCompiler()
let result = await compiler.compile(config: config)
```

## Examples

### iOS App Integration

```swift
import SwiftUI
import SwiftTSGo

struct ContentView: View {
    @State private var compilationResult: BuildResult?
    @State private var isCompiling = false

    var body: some View {
        VStack {
            Button("Compile TypeScript") {
                compileProject()
            }
            .disabled(isCompiling)

            if let result = compilationResult {
                CompilationResultView(result: result)
            }
        }
        .padding()
    }

    private func compileProject() {
        isCompiling = true

        Task {
            let config = BuildConfig(
                projectPath: Bundle.main.path(forResource: "typescript-project", ofType: nil) ?? ".",
                printErrors: false
            )

            let result = buildWithConfig(config)

            await MainActor.run {
                self.compilationResult = result
                self.isCompiling = false
            }
        }
    }
}

struct CompilationResultView: View {
    let result: BuildResult

    var body: some View {
        VStack(alignment: .leading) {
            Text(result.success ? "✅ Success" : "❌ Failed")
                .font(.headline)

            if !result.diagnostics.isEmpty {
                Text("Diagnostics:")
                    .font(.subheadline)
                    .padding(.top)

                ForEach(Array(result.diagnostics.enumerated()), id: \.offset) { _, diagnostic in
                    DiagnosticRowView(diagnostic: diagnostic)
                }
            }
        }
    }
}

struct DiagnosticRowView: View {
    let diagnostic: DiagnosticInfo

    var body: some View {
        VStack(alignment: .leading) {
            Text("TS\(diagnostic.code): \(diagnostic.message)")
                .foregroundColor(diagnostic.category == "error" ? .red : .orange)

            if !diagnostic.file.isEmpty {
                Text("\(diagnostic.file):\(diagnostic.line):\(diagnostic.column)")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
        .padding(.vertical, 2)
    }
}
```

### Command Line Tool

```swift
import Foundation
import SwiftTSGo

@main
struct TypeScriptCLI {
    static func main() {
        let arguments = CommandLine.arguments

        guard arguments.count > 1 else {
            print("Usage: typescript-cli <project-path>")
            exit(1)
        }

        let projectPath = arguments[1]
        let config = BuildConfig(
            projectPath: projectPath,
            printErrors: true
        )

        print("Compiling TypeScript project at: \(projectPath)")

        let result = buildWithConfig(config)

        if result.success {
            print("✅ Compilation successful!")
            print("Files emitted: \(result.emittedFiles.count)")
            for file in result.emittedFiles {
                print("  - \(file)")
            }
        } else {
            print("❌ Compilation failed!")
            let errors = result.diagnostics.filter { $0.category == "error" }
            print("Errors: \(errors.count)")
            exit(1)
        }
    }
}
```

## Requirements

- iOS 17.0+ / macOS 14.0+
- Swift 5.9+
- Xcode 15.0+

## Development

This package includes pre-built C bridge libraries for the TypeScript compiler. If you need to rebuild the bindings from source or contribute to the project, you'll need additional development tools.

### Development Requirements
### Prerequisites

- Go 1.19 or later
- CGO support
- Platform-specific build tools (Xcode for iOS/macOS)

### Setting up the build environment

```bash
make setup
```

### Building the Bindings

To rebuild the C bridge libraries after making changes to the Go bridge code:

```bash
make build
```

This will regenerate the static libraries in `Sources/TSCBridge/` with your changes.

For faster development (macOS only):

```bash
make build-bridge-macos
```

### Running Tests

Run the Go bridge tests:

```bash
make test-go
```

Run the Swift tests:

```bash
make test-swift
```

Run all tests:

```bash
make test
```

### Project Structure

- `bridge/` - Go bridge code that interfaces with the TypeScript compiler
- `Sources/SwiftTSGo/` - Swift API layer
- `Sources/TSCBridge/` - C bridge static libraries and headers (auto-generated)
- `Tests/SwiftTSGoTests/` - Swift test suite
