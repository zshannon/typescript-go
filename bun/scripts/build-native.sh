#!/bin/bash
set -e

# Build tsgo native library for all supported platforms
# Usage: ./scripts/build-native.sh [platform]
#
# Platforms: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-x64, all, current
# If no platform specified, builds for current platform

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUN_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$BUN_DIR")"

cd "$ROOT_DIR"

build_platform() {
    local platform=$1
    local goos goarch ext

    case "$platform" in
        darwin-arm64)
            goos=darwin
            goarch=arm64
            ext=dylib
            ;;
        darwin-x64)
            goos=darwin
            goarch=amd64
            ext=dylib
            ;;
        linux-arm64)
            goos=linux
            goarch=arm64
            ext=so
            ;;
        linux-x64)
            goos=linux
            goarch=amd64
            ext=so
            ;;
        win32-x64)
            goos=windows
            goarch=amd64
            ext=dll
            ;;
        *)
            echo "Unknown platform: $platform"
            echo "Valid platforms: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-x64"
            exit 1
            ;;
    esac

    local output_dir="$BUN_DIR/binaries/$platform"
    local output_file="$output_dir/libtsgo.$ext"

    mkdir -p "$output_dir"

    echo "Building $platform..."
    CGO_ENABLED=1 GOOS=$goos GOARCH=$goarch go build -buildmode=c-shared -o "$output_file" ./bun

    echo "  -> $output_file ($(du -h "$output_file" | cut -f1))"
}

detect_current_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$os" in
        darwin) os="darwin" ;;
        linux) os="linux" ;;
        mingw*|msys*|cygwin*) os="win32" ;;
    esac

    case "$arch" in
        arm64|aarch64) arch="arm64" ;;
        x86_64|amd64) arch="x64" ;;
    esac

    echo "${os}-${arch}"
}

PLATFORM="${1:-current}"

echo "=== Building tsgo native library ==="
echo "Root: $ROOT_DIR"
echo "Output: $BUN_DIR/binaries"
echo ""

case "$PLATFORM" in
    all)
        build_platform darwin-arm64
        build_platform darwin-x64
        build_platform linux-arm64
        build_platform linux-x64
        build_platform win32-x64
        ;;
    current)
        CURRENT=$(detect_current_platform)
        echo "Detected platform: $CURRENT"
        build_platform "$CURRENT"
        ;;
    *)
        build_platform "$PLATFORM"
        ;;
esac

echo ""
echo "=== Build complete ==="
