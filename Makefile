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

.PHONY: all build build-bridge setup test clean clean-bridge help

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
	@echo "$(YELLOW)Building for iOS arm64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=ios GOARCH=arm64 go build -buildmode=c-archive -o libtsc_ios_arm64.a .
	@echo "$(YELLOW)Building for iOS Simulator x86_64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildmode=c-archive -tags ios -o libtsc_ios_sim_amd64.a .
	@echo "$(YELLOW)Building for iOS Simulator arm64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -buildmode=c-archive -tags ios -o libtsc_ios_sim_arm64.a .
	@echo "$(YELLOW)Creating universal binary for macOS...$(NC)"
	@cd $(BRIDGE_DIR) && lipo -create libtsc_darwin_amd64.a libtsc_darwin_arm64.a -output libtsc_macos.a
	@echo "$(YELLOW)Creating universal binary for iOS Simulator...$(NC)"
	@cd $(BRIDGE_DIR) && lipo -create libtsc_ios_sim_amd64.a libtsc_ios_sim_arm64.a -output libtsc_ios_simulator.a
	@echo "$(YELLOW)Copying files to output directory...$(NC)"
	@cp $(BRIDGE_DIR)/libtsc_macos.a $(OUTPUT_DIR)/
	@cp $(BRIDGE_DIR)/libtsc_ios_arm64.a $(OUTPUT_DIR)/
	@cp $(BRIDGE_DIR)/libtsc_ios_simulator.a $(OUTPUT_DIR)/
	@cp $(BRIDGE_DIR)/libtsc_darwin_amd64.h $(OUTPUT_DIR)/tsc_bridge.h
	@echo "$(YELLOW)Creating module map...$(NC)"
	@echo 'module TSCBridge {\n    header "tsc_bridge.h"\n    export *\n}' > $(OUTPUT_DIR)/module.modulemap
	@echo "$(YELLOW)Cleaning up intermediate files...$(NC)"
	@cd $(BRIDGE_DIR) && rm -f libtsc_darwin_*.a libtsc_ios_*.a *.h
	@echo "$(GREEN)C Bridge build completed successfully!$(NC)"
	@$(MAKE) verify-bridge

# Build only for macOS (faster for development)
build-bridge-macos:
	@echo "$(GREEN)Building TypeScript Go C Bridge for macOS only...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@echo "$(YELLOW)Cleaning previous builds...$(NC)"
	@rm -f $(OUTPUT_DIR)/libtsc_macos.a $(OUTPUT_DIR)/tsc_bridge.h $(BRIDGE_DIR)/libtsc_*.a $(BRIDGE_DIR)/*.h
	@echo "$(YELLOW)Building for macOS x86_64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildmode=c-archive -o libtsc_darwin_amd64.a .
	@echo "$(YELLOW)Building for macOS arm64...$(NC)"
	@cd $(BRIDGE_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -buildmode=c-archive -o libtsc_darwin_arm64.a .
	@echo "$(YELLOW)Creating universal binary for macOS...$(NC)"
	@cd $(BRIDGE_DIR) && lipo -create libtsc_darwin_amd64.a libtsc_darwin_arm64.a -output libtsc_macos.a
	@echo "$(YELLOW)Copying files to output directory...$(NC)"
	@cp $(BRIDGE_DIR)/libtsc_macos.a $(OUTPUT_DIR)/
	@cp $(BRIDGE_DIR)/libtsc_darwin_amd64.h $(OUTPUT_DIR)/tsc_bridge.h
	@echo "$(YELLOW)Creating module map...$(NC)"
	@echo 'module TSCBridge {\n    header "tsc_bridge.h"\n    export *\n}' > $(OUTPUT_DIR)/module.modulemap
	@echo "$(YELLOW)Cleaning up intermediate files...$(NC)"
	@cd $(BRIDGE_DIR) && rm -f libtsc_darwin_*.a *.h
	@echo "$(GREEN)C Bridge macOS build completed successfully!$(NC)"



# Verify bridge builds
verify-bridge:
	@echo "$(BLUE)Verifying builds...$(NC)"
	@for lib in $(OUTPUT_DIR)/*.a; do \
		echo "Checking $$(basename $$lib):"; \
		file "$$lib"; \
		nm "$$lib" | grep -E "(tsc_build_|tsc_validate_|tsc_free_)" | head -5 || true; \
		echo ""; \
	done

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

# Development helpers
dev-setup: setup build-bridge-macos
	@echo "$(GREEN)Development environment ready!$(NC)"

# Quick build and test
quick: build-bridge-macos test-swift
	@echo "$(GREEN)Quick build and test completed!$(NC)"

# Help target
help:
	@echo "$(GREEN)TypeScript-Go Build System$(NC)"
	@echo ""
	@echo "$(BLUE)Main targets:$(NC)"
	@echo "  build              Build C bridge for all platforms (default)"
	@echo "  build-bridge       Build C bridge for all platforms"
	@echo "  build-bridge-macos Build C bridge for macOS only (faster)"
	@echo ""
	@echo "$(BLUE)Development:$(NC)"
	@echo "  setup              Setup development environment"
	@echo "  dev-setup          Setup environment and build for macOS"
	@echo "  quick              Quick macOS build and Swift test"
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
	@echo "  make dev-setup     # Setup environment and build for development"
	@echo "  make quick         # Fast build and test cycle"
	@echo "  make build test    # Full build and test"
