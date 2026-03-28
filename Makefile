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

.PHONY: all build build-bridge sign-bridge setup test clean clean-bridge help bench bench-local bench-prod bench-test bench-gen-fixtures

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
	@echo "$(YELLOW)Applying library compatibility patch...$(NC)"
	@if git apply --check internal-osvfs-library-fix.patch 2>/dev/null; then \
		git apply internal-osvfs-library-fix.patch; \
	else \
		echo "$(BLUE)Patch already applied or not needed$(NC)"; \
	fi
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
	@echo "$(YELLOW)Reverting library compatibility patch...$(NC)"
	@git checkout -- internal/vfs/osvfs/os.go 2>/dev/null || true
	@echo "$(GREEN)C Bridge build completed successfully!$(NC)"
	@$(MAKE) sign-bridge
	@$(MAKE) verify-bridge




# Sign the XCFramework
sign-bridge:
ifndef CI
	@echo "$(BLUE)Signing XCFramework...$(NC)"
	@codesign --sign "9C96C2F5997CCF243699DB73D5A365C135BEE961" --timestamp --force $(OUTPUT_DIR)/TSCBridge.xcframework
	@echo "$(GREEN)XCFramework signed successfully!$(NC)"
else
	@echo "$(YELLOW)Skipping code signing in CI environment$(NC)"
endif

# Verify bridge builds
verify-bridge:
	@echo "$(BLUE)Verifying builds...$(NC)"
	@if [ -d $(OUTPUT_DIR)/TSCBridge.xcframework ]; then \
		echo "Checking TSCBridge.xcframework:"; \
		find $(OUTPUT_DIR)/TSCBridge.xcframework -name "*.a" -exec file {} \; || true; \
		if [ -z "$$CI" ]; then \
			echo "Verifying code signature:"; \
			codesign --verify --verbose $(OUTPUT_DIR)/TSCBridge.xcframework || true; \
		fi; \
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

# Benchmarks
BENCH_DIR = server/benchmark-results
BENCH_PROD_URL = https://server-wild-sea-9370.fly.dev
BENCH_TIMESTAMP = $(shell date +%Y%m%d-%H%M%S)
BENCH_RUNS = 30
BENCH_WARMUP = 5

# Generate JSON fixture files from Go test constants
bench-gen-fixtures:
	@echo "$(YELLOW)Generating v2 benchmark fixtures...$(NC)"
	@cd server && go test -run=TestGenerateV2Fixtures -v ./... 2>&1 | grep "Wrote"
	@echo "$(GREEN)Fixtures generated$(NC)"

# Local Go benchmarks (pure compiler, no network)
bench-local:
	@echo "$(BLUE)=== Local V2 Benchmarks ===$(NC)"
	@echo "$(YELLOW)Running 3 rounds, 5s each...$(NC)"
	@mkdir -p $(BENCH_DIR)
	@cd server && go test -bench=BenchmarkV2 -benchmem -benchtime=5s -count=3 -run='^$$' ./... 2>/dev/null | tee ../$(BENCH_DIR)/$(BENCH_TIMESTAMP)-local.txt
	@echo ""
	@echo "$(GREEN)Results saved to $(BENCH_DIR)/$(BENCH_TIMESTAMP)-local.txt$(NC)"

# Production benchmarks via hyperfine
bench-prod: bench-gen-fixtures
	@echo "$(BLUE)=== Production V2 Benchmarks ===$(NC)"
	@echo "Endpoint: $(BENCH_PROD_URL)"
	@echo "Runs: $(BENCH_RUNS), Warmup: $(BENCH_WARMUP)"
	@echo ""
	@mkdir -p $(BENCH_DIR)
	@# Verify server is reachable
	@curl -sf $(BENCH_PROD_URL)/health > /dev/null 2>&1 || (echo "$(RED)Server unreachable at $(BENCH_PROD_URL)$(NC)" && exit 1)
	@hyperfine \
		--warmup $(BENCH_WARMUP) \
		--runs $(BENCH_RUNS) \
		--style full \
		--export-markdown $(BENCH_DIR)/$(BENCH_TIMESTAMP)-prod.md \
		--export-json $(BENCH_DIR)/$(BENCH_TIMESTAMP)-prod.json \
		-n "Health Check" \
			"curl -s $(BENCH_PROD_URL)/health > /dev/null" \
		-n "V2 Typecheck: Trivial" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/typecheck -H 'Content-Type: application/json' -d @server/fixtures/v2/typecheck-trivial.json > /dev/null" \
		-n "V2 Typecheck: Small Component" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/typecheck -H 'Content-Type: application/json' -d @server/fixtures/v2/typecheck-small.json > /dev/null" \
		-n "V2 Typecheck: Medium Component" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/typecheck -H 'Content-Type: application/json' -d @server/fixtures/v2/typecheck-medium.json > /dev/null" \
		-n "V2 Typecheck: Multi-File (5 files)" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/typecheck -H 'Content-Type: application/json' -d @server/fixtures/v2/typecheck-multifile.json > /dev/null" \
		-n "V2 Build: Trivial" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/build -H 'Content-Type: application/json' -d @server/fixtures/v2/build-trivial.json > /dev/null" \
		-n "V2 Build: Small Component" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/build -H 'Content-Type: application/json' -d @server/fixtures/v2/build-small.json > /dev/null" \
		-n "V2 Build: Medium Component" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/build -H 'Content-Type: application/json' -d @server/fixtures/v2/build-medium.json > /dev/null" \
		-n "V2 Build: Multi-File (5 files)" \
			"curl -s -X POST $(BENCH_PROD_URL)/v2/build -H 'Content-Type: application/json' -d @server/fixtures/v2/build-multifile.json > /dev/null"
	@echo ""
	@echo "$(GREEN)Results saved to:$(NC)"
	@echo "  $(BENCH_DIR)/$(BENCH_TIMESTAMP)-prod.md"
	@echo "  $(BENCH_DIR)/$(BENCH_TIMESTAMP)-prod.json"

# Quick smoke test: verify v2 endpoints work against production
bench-test: bench-gen-fixtures
	@echo "$(BLUE)=== V2 Endpoint Smoke Test ===$(NC)"
	@echo "Endpoint: $(BENCH_PROD_URL)"
	@echo ""
	@PASS=0; FAIL=0; \
	curl -sf $(BENCH_PROD_URL)/health > /dev/null 2>&1 || { echo "$(RED)Server unreachable$(NC)"; exit 1; }; \
	echo "$(GREEN)✓$(NC) Health check"; PASS=$$((PASS+1)); \
	\
	for f in server/fixtures/v2/typecheck-*.json; do \
		name=$$(basename "$$f" .json); \
		resp=$$(curl -sf -X POST $(BENCH_PROD_URL)/v2/typecheck -H 'Content-Type: application/json' -d @"$$f" 2>&1); \
		if [ $$? -eq 0 ] && echo "$$resp" | grep -qE '"pass"|"errors"'; then \
			echo "$(GREEN)✓$(NC) $$name"; PASS=$$((PASS+1)); \
		else \
			echo "$(RED)✗$(NC) $$name: $$resp"; FAIL=$$((FAIL+1)); \
		fi; \
	done; \
	\
	for f in server/fixtures/v2/build-*.json; do \
		name=$$(basename "$$f" .json); \
		resp=$$(curl -sf -X POST $(BENCH_PROD_URL)/v2/build -H 'Content-Type: application/json' -d @"$$f" 2>&1); \
		if [ $$? -eq 0 ] && echo "$$resp" | grep -q '"code"'; then \
			echo "$(GREEN)✓$(NC) $$name"; PASS=$$((PASS+1)); \
		else \
			echo "$(RED)✗$(NC) $$name: $${resp:0:200}"; FAIL=$$((FAIL+1)); \
		fi; \
	done; \
	\
	echo ""; \
	echo "Passed: $$PASS  Failed: $$FAIL"; \
	if [ $$FAIL -gt 0 ]; then exit 1; fi

# Run both local and production benchmarks
bench: bench-local bench-prod

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
	@echo "$(BLUE)Benchmarks:$(NC)"
	@echo "  bench              Run local + production benchmarks"
	@echo "  bench-local        Local Go benchmarks (no network)"
	@echo "  bench-prod         Production benchmarks via hyperfine"
	@echo "  bench-test         Smoke test v2 endpoints against prod"
	@echo "  bench-gen-fixtures Generate JSON fixture files"
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
