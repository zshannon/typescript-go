package main

/*
#include <stdlib.h>

typedef struct {
    int* values;
    int count;
} c_int_array;
*/
import "C"

import (
	"unsafe"

	"github.com/evanw/esbuild/pkg/api"
)

//export esbuild_platform_default
func esbuild_platform_default() C.int {
	return C.int(api.PlatformDefault)
}

//export esbuild_platform_browser
func esbuild_platform_browser() C.int {
	return C.int(api.PlatformBrowser)
}

//export esbuild_platform_node
func esbuild_platform_node() C.int {
	return C.int(api.PlatformNode)
}

//export esbuild_platform_neutral
func esbuild_platform_neutral() C.int {
	return C.int(api.PlatformNeutral)
}

//export esbuild_get_all_platform_values
func esbuild_get_all_platform_values() *C.c_int_array {
	// Get all platform values
	platforms := []C.int{
		C.int(api.PlatformDefault),
		C.int(api.PlatformBrowser),
		C.int(api.PlatformNode),
		C.int(api.PlatformNeutral),
	}

	// Allocate C memory for the array
	count := len(platforms)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))

	// Copy Go slice to C array
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, platform := range platforms {
		cSlice[i] = platform
	}

	// Create result struct
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)

	return result
}

//export esbuild_free_int_array
func esbuild_free_int_array(arr *C.c_int_array) {
	if arr == nil {
		return
	}

	if arr.values != nil {
		C.free(unsafe.Pointer(arr.values))
	}

	C.free(unsafe.Pointer(arr))
}