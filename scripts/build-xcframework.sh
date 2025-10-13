#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building TSCBridge XCFramework for release...${NC}"

# Configuration
FRAMEWORK_NAME="TSCBridge"
OUTPUT_DIR="Sources/TSCBridge"
RELEASE_DIR="release"

# Clean any previous release artifacts
echo -e "${YELLOW}Cleaning previous release artifacts...${NC}"
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

# Build the XCFramework using the existing Makefile
echo -e "${YELLOW}Building XCFramework using Makefile...${NC}"
make build-bridge

# Verify the XCFramework was built
if [ ! -d "${OUTPUT_DIR}/${FRAMEWORK_NAME}.xcframework" ]; then
    echo -e "${RED}Error: XCFramework not found at ${OUTPUT_DIR}/${FRAMEWORK_NAME}.xcframework${NC}"
    exit 1
fi

# Copy XCFramework to release directory
echo -e "${YELLOW}Copying XCFramework to release directory...${NC}"
cp -r "${OUTPUT_DIR}/${FRAMEWORK_NAME}.xcframework" "${RELEASE_DIR}/"

# Create zip file for distribution
echo -e "${YELLOW}Creating zip archive...${NC}"
cd "${RELEASE_DIR}"
zip -r "${FRAMEWORK_NAME}.xcframework.zip" "${FRAMEWORK_NAME}.xcframework"

# Generate checksum
echo -e "${YELLOW}Generating SHA256 checksum...${NC}"
CHECKSUM=$(shasum -a 256 "${FRAMEWORK_NAME}.xcframework.zip" | awk '{print $1}')

# Create checksum file
echo "${CHECKSUM}" > "${FRAMEWORK_NAME}.xcframework.zip.sha256"

echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${BLUE}Release artifacts:${NC}"
echo -e "  - ${RELEASE_DIR}/${FRAMEWORK_NAME}.xcframework.zip"
echo -e "  - ${RELEASE_DIR}/${FRAMEWORK_NAME}.xcframework.zip.sha256"
echo ""
echo -e "${BLUE}SHA256 Checksum:${NC}"
echo -e "  ${CHECKSUM}"
echo ""
echo -e "${YELLOW}Use this checksum in Package.swift for the binaryTarget${NC}"
