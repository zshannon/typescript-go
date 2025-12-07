#!/bin/bash
set -e

# Build tsgo native library for all supported platforms
# This script should be run from the typescript-go/ffi directory

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FFI_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$FFI_DIR")"
OUTPUT_DIR="$FFI_DIR/binaries"

echo "=== Building tsgo native libraries ==="
echo "Root: $ROOT_DIR"
echo "FFI:  $FFI_DIR"
echo "Output: $OUTPUT_DIR"
echo ""

# Ensure we're in the right directory
cd "$ROOT_DIR"

# Create output directories
mkdir -p "$OUTPUT_DIR"/{darwin-arm64,darwin-x64,linux-arm64,linux-x64,win32-x64}

# Build function
build_platform() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT_NAME=$3
    local OUTPUT_SUBDIR=$4

    echo "Building for $GOOS/$GOARCH..."

    local EXT=".so"
    if [ "$GOOS" = "darwin" ]; then
        EXT=".dylib"
    elif [ "$GOOS" = "windows" ]; then
        EXT=".dll"
    fi

    CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH \
        go build -buildmode=c-shared \
        -o "$OUTPUT_DIR/$OUTPUT_SUBDIR/libtsgo$EXT" \
        ./ffi

    echo "  -> $OUTPUT_DIR/$OUTPUT_SUBDIR/libtsgo$EXT"
}

# Check if cross-compilation toolchains are available
check_cross_compile() {
    local target=$1
    case $target in
        "darwin-arm64")
            # Can only build natively on macOS ARM64
            if [ "$(uname -s)" = "Darwin" ] && [ "$(uname -m)" = "arm64" ]; then
                return 0
            fi
            ;;
        "darwin-x64")
            # Can only build natively on macOS x64 or with cross-compile on ARM64 Mac
            if [ "$(uname -s)" = "Darwin" ]; then
                return 0
            fi
            ;;
        "linux-x64")
            # Can build on Linux x64 natively, or with cross-compile toolchain
            if [ "$(uname -s)" = "Linux" ] && [ "$(uname -m)" = "x86_64" ]; then
                return 0
            fi
            # Check for cross-compiler
            if command -v x86_64-linux-gnu-gcc &> /dev/null; then
                return 0
            fi
            ;;
        "linux-arm64")
            # Can build on Linux ARM64 natively, or with cross-compile toolchain
            if [ "$(uname -s)" = "Linux" ] && [ "$(uname -m)" = "aarch64" ]; then
                return 0
            fi
            # Check for cross-compiler
            if command -v aarch64-linux-gnu-gcc &> /dev/null; then
                return 0
            fi
            ;;
        "win32-x64")
            # Can build on Windows x64 natively, or with mingw
            if [ "$(uname -s)" = "MINGW"* ] || [ "$(uname -s)" = "MSYS"* ]; then
                return 0
            fi
            # Check for mingw cross-compiler
            if command -v x86_64-w64-mingw32-gcc &> /dev/null; then
                return 0
            fi
            ;;
    esac
    return 1
}

# Build for current platform
build_current_platform() {
    local OS=$(uname -s)
    local ARCH=$(uname -m)

    case "$OS-$ARCH" in
        "Darwin-arm64")
            build_platform darwin arm64 libtsgo darwin-arm64
            # Also build x64 on ARM Mac (Rosetta compatible)
            build_platform darwin amd64 libtsgo darwin-x64
            ;;
        "Darwin-x86_64")
            build_platform darwin amd64 libtsgo darwin-x64
            ;;
        "Linux-x86_64")
            build_platform linux amd64 libtsgo linux-x64
            # Try ARM64 cross-compile if toolchain available
            if command -v aarch64-linux-gnu-gcc &> /dev/null; then
                CC=aarch64-linux-gnu-gcc build_platform linux arm64 libtsgo linux-arm64
            fi
            ;;
        "Linux-aarch64")
            build_platform linux arm64 libtsgo linux-arm64
            # Try x64 cross-compile if toolchain available
            if command -v x86_64-linux-gnu-gcc &> /dev/null; then
                CC=x86_64-linux-gnu-gcc build_platform linux amd64 libtsgo linux-x64
            fi
            ;;
        *)
            echo "Unsupported platform: $OS-$ARCH"
            exit 1
            ;;
    esac
}

# Windows cross-compile (if mingw available)
build_windows_cross() {
    if command -v x86_64-w64-mingw32-gcc &> /dev/null; then
        echo "Building Windows x64 (cross-compile with mingw)..."
        CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
            go build -buildmode=c-shared \
            -o "$OUTPUT_DIR/win32-x64/libtsgo.dll" \
            ./ffi
        echo "  -> $OUTPUT_DIR/win32-x64/libtsgo.dll"
    else
        echo "Skipping Windows build (x86_64-w64-mingw32-gcc not found)"
    fi
}

# Main
echo "Detected platform: $(uname -s)-$(uname -m)"
echo ""

build_current_platform
build_windows_cross

echo ""
echo "=== Build complete ==="
echo ""
echo "Built binaries:"
find "$OUTPUT_DIR" -name "libtsgo*" -type f | while read f; do
    echo "  $f ($(du -h "$f" | cut -f1))"
done
