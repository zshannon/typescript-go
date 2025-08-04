// Proper test with Go runtime initialization
#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include "lib/libtsc-musl.h"

// Swift stub implementations
c_on_resolve_result* swift_plugin_on_resolve_callback(c_on_resolve_args* args, void* callbackData) {
    return NULL;
}

c_on_load_result* swift_plugin_on_load_callback(c_on_load_args* args, void* callbackData) {
    return NULL;
}

void swift_plugin_on_start_callback(void* callbackData) {
    // No-op
}

void swift_plugin_on_end_callback(void* callbackData) {
    // No-op
}

int main() {
    printf("Testing musl library with proper initialization...\n");
    
    // Create a simple resolver data structure
    c_file_resolver_data* resolver = tsc_create_resolver_data();
    if (!resolver) {
        printf("Failed to create resolver data\n");
        return 1;
    }
    
    // Add a simple TypeScript file
    tsc_add_file_to_resolver(resolver, "/test.ts", "const x: number = 42;");
    
    // Add directory
    tsc_add_directory_to_resolver(resolver, "/");
    
    // Try to build
    printf("Calling tsc_build_with_resolver...\n");
    c_build_result* result = tsc_build_with_resolver("/", 0, "", resolver);
    
    if (result) {
        printf("Build result: success=%d\n", result->success);
        printf("Diagnostic count: %d\n", result->diagnostic_count);
        printf("Written file count: %d\n", result->written_file_count);
        
        // Free the result
        tsc_free_result(result);
    } else {
        printf("Build returned NULL\n");
    }
    
    // Free resolver
    tsc_free_resolver_data(resolver);
    
    printf("Test completed successfully!\n");
    return 0;
}