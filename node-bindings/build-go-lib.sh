#!/bin/bash
set -e

# Script to build the Go TypeScript library for use with Node.js bindings on Alpine Linux

echo "Building Go TypeScript library for Alpine Linux..."

# Create lib directory
mkdir -p lib

# Navigate to the bridge directory
cd ../bridge

# Build the C archive with ios tag to avoid os.Executable() issues in library context
# The ios tag prevents filesystem-based initialization that doesn't work in Node.js addons
echo "Building C archive from bridge package..."  
CGO_ENABLED=1 go build -tags ios -buildmode=c-archive -o ../node-bindings/lib/libtsc.a .

# Check if build succeeded
if [ -f "../node-bindings/lib/libtsc.a" ]; then
    echo "Go library built successfully at node-bindings/lib/libtsc.a"
else
    echo "Failed to build Go library"
    exit 1
fi

# The header should be generated automatically
if [ -f "../node-bindings/lib/libtsc.h" ]; then
    echo "Header file generated at node-bindings/lib/libtsc.h"
    # Copy to match our expected name
    cp ../node-bindings/lib/libtsc.h ../node-bindings/lib/tsc_bridge.h
    
    # Fix namespace keyword for C++ compatibility
    echo "Fixing namespace keyword for C++ compatibility..."
    sed -i '' 's/char\* namespace;/char\* namespace_;/g' ../node-bindings/lib/tsc_bridge.h
fi

echo "Build complete!"