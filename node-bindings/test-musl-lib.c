// Simple test to verify musl library linking
#include <stdio.h>
#include "lib/libtsc-musl.h"

int main() {
    printf("Testing musl library...\n");
    
    // Try to call a simple function from the library
    c_build_result* result = tsc_build_filesystem("/test", 0, "");
    
    if (result) {
        printf("Library call succeeded!\n");
        tsc_free_result(result);
    } else {
        printf("Library call returned NULL\n");
    }
    
    return 0;
}