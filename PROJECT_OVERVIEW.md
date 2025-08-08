# TypeScript-Go Fork Overview

This fork of microsoft/typescript-go provides two distinct but related features for running TypeScript and ESBuild in different environments:

## Architecture Overview

### 1. Swift Package - Native Cross-Compilation
**Purpose**: Provides TypeScript compiler and ESBuild functionality as a native Swift package for iOS/macOS applications.

**Key Components**:
- **Bridge Layer** (`bridge/`): Go code compiled to C archives using CGO
  - `c_bridge.go`: Main bridge implementation wrapping TypeScript compiler
  - `esbuild_c_bridge.go`: ESBuild functionality exposed via C interface
  
- **XCFramework** (`Sources/TSCBridge/`): Universal binary framework
  - Supports macOS (x86_64, arm64)
  - Supports iOS device (arm64)
  - Supports iOS Simulator (x86_64, arm64)
  
- **Swift Wrapper** (`Sources/SwiftTSGo/`): High-level Swift API
  - `BuildInMemory.swift`: In-memory compilation
  - `ESBuildPlugin*.swift`: ESBuild plugin system
  - `TSConfig.swift`: TypeScript configuration handling

**Build Process**:
```bash
make build-bridge  # Builds the C bridge and XCFramework
swift build        # Builds the Swift package
```

**Use Case**: Native iOS/macOS apps that need TypeScript compilation or bundling without network dependencies.

### 2. Go HTTP Server - Cloud Service
**Purpose**: Provides TypeScript and ESBuild as an HTTP API service, deployable as a Docker container.

**Key Components**:
- **Server** (`server.go`): HTTP server exposing compilation endpoints
  - `/build` endpoint: TypeScript compilation with optional typechecking
  - ESBuild transformation capabilities
  - In-memory file system support for compilation
  
- **Deployment**:
  - Dockerized for containerization
  - Deployed to Fly.io for global edge computing
  - Stateless design for horizontal scaling

**Build Process**:
```bash
# Build server binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-linux server.go

# Docker build (various approaches in npm scripts)
npm run build:docker:local   # Local testing
npm run build:docker:amd64   # Production AMD64
npm run build:docker:arm64   # Production ARM64
```

**Use Case**: Web services, CI/CD pipelines, or any environment that needs TypeScript compilation via HTTP API.

## Shared Foundation

Both features share the same underlying TypeScript-Go compiler implementation from Microsoft:
- Full TypeScript type checking
- JavaScript emission
- Declaration file generation
- Source map support
- ESBuild integration for bundling

## Key Differences

| Aspect | Swift Package | HTTP Server |
|--------|--------------|-------------|
| **Target Platform** | iOS/macOS native | Cloud/Docker |
| **Integration** | Direct library import | HTTP API |
| **Performance** | Native speed, no network | Network latency, but scalable |
| **Deployment** | Embedded in app | Fly.io container |
| **Use Case** | Mobile/desktop apps | Web services, CI/CD |

## Development Workflow

1. **Core TypeScript Updates**: Update the `_submodules/TypeScript` submodule
2. **Bridge Updates**: Modify `bridge/*.go` files for new TypeScript APIs
3. **Rebuild Bridge**: `make build-bridge` to regenerate XCFramework
4. **Test Both Paths**:
   - Swift: `swift test`
   - Server: `go run server.go` and test endpoints

## Important Files

- `Makefile`: Orchestrates bridge building for all platforms
- `Package.swift`: Swift package definition
- `server.go`: HTTP server implementation
- `Dockerfile`: Container definition for server deployment
- `fly.toml`: Fly.io deployment configuration

## Common Issues & Fixes

1. **Bridge compilation errors**: Usually due to TypeScript API changes
   - Check `compiler.NewCachedFSCompilerHost` signature
   - Verify `program.Emit` requires context parameter

2. **Go mod issues**: Run `cd bridge && go mod tidy`

3. **XCFramework signing**: Requires valid Apple Developer certificate
   - See line 69 in Makefile for certificate ID

## Testing

- **Swift Package**: `swift test`
- **Go Bridge**: `cd bridge && go test -v`
- **Server**: Manual HTTP testing or integration tests