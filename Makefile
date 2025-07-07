# TypeScript-Go Makefile
# Build system for TypeScript compiler bridge

# Configuration
BRIDGE_DIR = bridge
OUTPUT_DIR = Sources/TSCBridge

# Colors for output
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[1;33m
BLUE = \033[0;34m
NC = \033[0m # No Color

.PHONY: all build build-bridge sign-bridge setup test clean clean-bridge help

# Default target
all: build

# Build C bridge
build: build-bridge

# Build C bridge for all platforms
build-bridge:
	@echo "$(GREEN)Building TypeScript Go C Bridge...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@echo "$(YELLOW)Cleaning previous builds...$(NC)"
	@rm -f $(OUTPUT_DIR)/*.a $(OUTPUT_DIR)/*.h $(BRIDGE_DIR)/*.a $(BRIDGE_DIR)/*.h
	@echo "$(YELLOW)Building for macOS x86_64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildmode=c-archive -o libtsc_darwin_amd64.a .
	@echo "$(YELLOW)Building for macOS arm64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -buildmode=c-archive -o libtsc_darwin_arm64.a .
	@echo "$(YELLOW)Creating macOS universal binary...$(NC)"
	@cd $(BRIDGE_DIR) && lipo -create libtsc_darwin_amd64.a libtsc_darwin_arm64.a -output libtsc_macos_universal.a
	@echo "$(YELLOW)Building for iOS arm64 (device)...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC=$(shell xcrun --sdk iphoneos -f clang) CXX=$(shell xcrun --sdk iphoneos -f clang++) CGO_CFLAGS="-isysroot $(shell xcrun --sdk iphoneos --show-sdk-path) -arch arm64 -miphoneos-version-min=13.0" CGO_LDFLAGS="-isysroot $(shell xcrun --sdk iphoneos --show-sdk-path) -arch arm64" go build -buildmode=c-archive -o libtsc_ios_arm64.a .
	@echo "$(YELLOW)Building for iOS Simulator x86_64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=ios GOARCH=amd64 CC=$(shell xcrun --sdk iphonesimulator -f clang) CXX=$(shell xcrun --sdk iphonesimulator -f clang++) CGO_CFLAGS="-isysroot $(shell xcrun --sdk iphonesimulator --show-sdk-path) -arch x86_64 -mios-simulator-version-min=13.0" CGO_LDFLAGS="-isysroot $(shell xcrun --sdk iphonesimulator --show-sdk-path) -arch x86_64" go build -buildmode=c-archive -o libtsc_ios_sim_amd64.a .
	@echo "$(YELLOW)Building for iOS Simulator arm64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC=$(shell xcrun --sdk iphonesimulator -f clang) CXX=$(shell xcrun --sdk iphonesimulator -f clang++) CGO_CFLAGS="-isysroot $(shell xcrun --sdk iphonesimulator --show-sdk-path) -arch arm64 -mios-simulator-version-min=13.0 -target arm64-apple-ios13.0-simulator" CGO_LDFLAGS="-isysroot $(shell xcrun --sdk iphonesimulator --show-sdk-path) -arch arm64" go build -buildmode=c-archive -o libtsc_ios_sim_arm64.a .
	@echo "$(YELLOW)Creating iOS Simulator universal binary...$(NC)"
	@cd $(BRIDGE_DIR) && lipo -create libtsc_ios_sim_amd64.a libtsc_ios_sim_arm64.a -output libtsc_ios_sim_universal.a
	@echo "$(YELLOW)Creating XCFramework...$(NC)"
	@cd $(BRIDGE_DIR) && rm -rf TSCBridge.xcframework
	@cd $(BRIDGE_DIR) && mkdir -p headers && cp libtsc_darwin_amd64.h headers/tsc_bridge.h
	@cd $(BRIDGE_DIR) && xcodebuild -create-xcframework \
		-library libtsc_macos_universal.a -headers headers \
		-library libtsc_ios_arm64.a -headers headers \
		-library libtsc_ios_sim_universal.a -headers headers \
		-output TSCBridge.xcframework
	@echo "$(YELLOW)Copying files to output directory...$(NC)"
	@cp -r $(BRIDGE_DIR)/TSCBridge.xcframework $(OUTPUT_DIR)/
	@cp $(BRIDGE_DIR)/libtsc_darwin_amd64.h $(OUTPUT_DIR)/tsc_bridge.h
	@echo "$(YELLOW)Creating module map...$(NC)"
	@echo 'module TSCBridge {\n    header "tsc_bridge.h"\n    export *\n}' > $(OUTPUT_DIR)/module.modulemap
	@echo "$(YELLOW)Cleaning up intermediate files...$(NC)"
	@cd $(BRIDGE_DIR) && rm -f libtsc_*.a *.h
	@cd $(BRIDGE_DIR) && rm -rf TSCBridge.xcframework headers
	@echo "$(GREEN)C Bridge build completed successfully!$(NC)"
	@$(MAKE) sign-bridge
	@$(MAKE) verify-bridge




# Sign the XCFramework
sign-bridge:
	@echo "$(BLUE)Signing XCFramework...$(NC)"
	@codesign --sign "9C96C2F5997CCF243699DB73D5A365C135BEE961" --timestamp --force $(OUTPUT_DIR)/TSCBridge.xcframework
	@echo "$(GREEN)XCFramework signed successfully!$(NC)"

# Verify bridge builds
verify-bridge:
	@echo "$(BLUE)Verifying builds...$(NC)"
	@if [ -d $(OUTPUT_DIR)/TSCBridge.xcframework ]; then \
		echo "Checking TSCBridge.xcframework:"; \
		find $(OUTPUT_DIR)/TSCBridge.xcframework -name "*.a" -exec file {} \; || true; \
		echo "Verifying code signature:"; \
		codesign --verify --verbose $(OUTPUT_DIR)/TSCBridge.xcframework || true; \
		echo ""; \
	fi

# Setup development environment
setup:
	@echo "$(GREEN)Setting up development environment...$(NC)"
	@echo "$(YELLOW)Installing Go dependencies...$(NC)"
	@cd $(BRIDGE_DIR) && go mod download
	@echo "$(GREEN)Setup completed!$(NC)"

# Run tests
test:
	@echo "$(GREEN)Running Go tests...$(NC)"
	@cd $(BRIDGE_DIR) && go test -v
	@echo "$(GREEN)Running Swift tests...$(NC)"
	@swift test

# Run only Go tests
test-go:
	@cd $(BRIDGE_DIR) && go test -v

# Run only Swift tests
test-swift:
	@swift test

# Clean all build artifacts
clean: clean-bridge
	@echo "$(GREEN)All build artifacts cleaned!$(NC)"

# Clean C bridge artifacts
clean-bridge:
	@echo "$(YELLOW)Cleaning C bridge artifacts...$(NC)"
	@rm -rf $(OUTPUT_DIR)
	@cd $(BRIDGE_DIR) && rm -f *.a *.h
	@cd $(BRIDGE_DIR) && rm -rf TSCBridge.xcframework headers


# Development helpers
dev-setup: setup build-bridge
	@echo "$(GREEN)Development environment ready!$(NC)"

# Quick build and test
quick: build-bridge test-swift
	@echo "$(GREEN)Quick build and test completed!$(NC)"

# Help target
help:
	@echo "$(GREEN)TypeScript-Go Build System$(NC)"
	@echo ""
	@echo "$(BLUE)Main targets:$(NC)"
	@echo "  build              Build C bridge XCFramework (default)"
	@echo "  build-bridge       Build C bridge XCFramework"
	@echo "  sign-bridge        Sign the XCFramework"
	@echo ""
	@echo "$(BLUE)Development:$(NC)"
	@echo "  setup              Setup development environment"
	@echo "  dev-setup          Setup environment and build XCFramework"
	@echo "  quick              Quick build and test cycle"
	@echo ""
	@echo "$(BLUE)Testing:$(NC)"
	@echo "  test               Run all tests (Go + Swift)"
	@echo "  test-go            Run Go tests only"
	@echo "  test-swift         Run Swift tests only"
	@echo ""
	@echo "$(BLUE)Maintenance:$(NC)"
	@echo "  clean              Clean all build artifacts"
	@echo "  clean-bridge       Clean C bridge artifacts"
	@echo "  verify-bridge      Verify bridge library symbols"
	@echo "  help               Show this help message"
	@echo ""
	@echo "$(BLUE)Examples:$(NC)"
	@echo "  make dev-setup     # Setup environment and build XCFramework"
	@echo "  make quick         # Quick build and test cycle"
	@echo "  make build test    # Full build and test"
