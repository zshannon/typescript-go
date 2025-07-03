# Build Process Notes

## Critical Build Step

**IMPORTANT**: After making any changes to Go code in the `bridge/` directory, you MUST run `make` to properly rebuild the C bridge and update all necessary files.

The `make` command:
1. Rebuilds the Go C bridge archive (TSCBridge.a)
2. Updates the header files
3. Ensures Swift can properly link to the new symbols

**Always run `make` after modifying:**
- `bridge/c_bridge.go`
- `bridge/esbuild_c_bridge.go`
- Any other Go files in the bridge directory

**DO NOT** try to manually rebuild just the Go archive - use `make` for the complete build process.