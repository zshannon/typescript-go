#!/bin/bash
set -e

echo "Testing musl library in Alpine Linux..."

# Run Alpine container with our library mounted
docker run --rm --platform linux/amd64 -v "$(pwd)":/workspace -w /workspace alpine:latest sh -c '
    # Install build tools and debugging tools
    apk add --no-cache gcc g++ musl-dev gdb

    # Copy header file to match include path
    cp lib/libtsc-musl.h lib/tsc_bridge.h

    # Try to compile proper test program with debug symbols
    echo "Compiling test program..."
    gcc -g -o test-musl test-musl-lib-proper.c lib/libtsc-musl.a -lpthread -ldl -static
    
    # Run the test with gdb to see where it crashes
    echo "Running test with gdb..."
    echo "run" | gdb -batch -ex "run" -ex "bt" ./test-musl || true
    
    # Also try running normally
    echo "Running test normally..."
    ./test-musl || echo "Exit code: $?"
'