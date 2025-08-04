#include "../../lib/tsc_bridge.h"
#include <stdlib.h>

// Stub implementations for Swift plugin callbacks
// These are not used in the Node.js bindings but are required by the Go library

c_on_resolve_result* swift_plugin_on_resolve_callback(c_on_resolve_args* args, void* callbackData) {
    // Return NULL to indicate no resolution
    return NULL;
}

c_on_load_result* swift_plugin_on_load_callback(c_on_load_args* args, void* callbackData) {
    // Return NULL to indicate no content
    return NULL;
}

void swift_plugin_on_start_callback(void* callbackData) {
    // No-op stub implementation
}

void swift_plugin_on_end_callback(void* callbackData) {
    // No-op stub implementation
}