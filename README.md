# typescript-go

A Go-based bridge to the TypeScript compiler, with XCFramework distribution for Swift applications.

This repository builds the native TypeScript compiler bridge and distributes it as an XCFramework. For using this in Swift projects, see [typescript-go-swift](https://github.com/zshannon/typescript-go-swift).

## Features

- ✅ **Native TypeScript Compilation**: Go bridge to TypeScript compiler with C API
- ✅ **XCFramework Distribution**: Pre-built binaries for iOS and macOS (arm64 + x86_64)
- ✅ **Rich Diagnostics**: Detailed error information with file locations and error codes
- ✅ **Cross-Platform**: Supports iOS (device + simulator) and macOS (Apple Silicon + Intel)
- ✅ **Swift Package**: Clean Swift API through [typescript-go-swift](https://github.com/zshannon/typescript-go-swift)

## For Swift Users

**Add the Swift package to your project:**

```swift
dependencies: [
    .package(url: "https://github.com/zshannon/typescript-go-swift.git", from: "0.1.2")
]
```

See [typescript-go-swift](https://github.com/zshannon/typescript-go-swift) for full Swift API documentation and usage examples.

## Architecture

```
typescript-go (this repo)
├── bridge/              # Go bridge to TypeScript compiler
├── Sources/TSCBridge/   # C headers and module map
├── scripts/             # Build scripts for XCFramework
└── swift-package/       # Submodule → typescript-go-swift
    └── Sources/
        ├── SwiftTSGo/   # Swift API wrapper
        └── TSCBridge/   # C headers (copy)
```

## Requirements

- **For using the Swift package**: iOS 18.0+ / macOS 15.0+, Swift 6.1+, Xcode 16.0+
- **For building from source**: Go 1.25+, CGO support, Xcode command line tools

## Development

### Building the XCFramework

To build the XCFramework from source:

```bash
./scripts/build-xcframework.sh
```

This creates `release/TSCBridge.xcframework.zip` with binaries for:
- iOS device (arm64)
- iOS simulator (arm64 + x86_64)
- macOS (arm64 + x86_64)

### Running Tests

Run the Go bridge tests:

```bash
make test-go
```

Run the Swift tests (uses submodule Swift sources):

```bash
make test-swift
```

### Project Structure

- `bridge/` - Go bridge code interfacing with TypeScript compiler
- `Sources/TSCBridge/` - C headers and module map for the bridge
- `scripts/` - Build scripts for creating the XCFramework
- `swift-package/` - Git submodule pointing to [typescript-go-swift](https://github.com/zshannon/typescript-go-swift)
- `Tests/SwiftTSGoTests/` - Swift test suite

### Release Process

1. Build XCFramework: Run the **Release XCFramework** workflow with version (e.g., `0.1.3`)
2. Review generated PR on typescript-go-swift repo
3. Merge PR to publish release

The workflow automatically:
- Builds XCFramework for all platforms
- Creates draft release on typescript-go-swift with binary
- Updates Package.swift with new version and checksum
- Creates PR for review

## License

Apache 2.0
