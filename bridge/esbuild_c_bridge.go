package main

/*
#include <stdlib.h>

typedef struct {
    int* values;
    int count;
} c_int_array;

typedef struct {
	// Logging and Output Control
	int color;                    // StderrColor enum
	int log_level;               // LogLevel enum  
	int log_limit;               // int
	char** log_override_keys;    // keys for map[string]LogLevel
	int* log_override_values;    // values for map[string]LogLevel
	int log_override_count;      // count of log override entries

	// Source Map
	int sourcemap;               // SourceMap enum
	char* source_root;           // string
	int sources_content;         // SourcesContent enum

	// Target and Compatibility  
	int target;                  // Target enum
	int* engine_names;           // EngineName enum array
	char** engine_versions;      // string array for engine versions
	int engines_count;           // count of engines
	char** supported_keys;       // keys for map[string]bool
	int* supported_values;       // values for map[string]bool (0/1)
	int supported_count;         // count of supported entries

	// Platform and Format
	int platform;                // Platform enum
	int format;                  // Format enum
	char* global_name;           // string

	// Minification and Property Mangling
	char* mangle_props;          // string (regex)
	char* reserve_props;         // string (regex)
	int mangle_quoted;           // MangleQuoted enum
	char** mangle_cache_keys;    // keys for map[string]interface{}
	char** mangle_cache_values;  // values as JSON strings
	int mangle_cache_count;      // count of mangle cache entries
	int drop;                    // Drop enum (bitfield)
	char** drop_labels;          // string array
	int drop_labels_count;       // count of drop labels
	int minify_whitespace;       // bool (0/1)
	int minify_identifiers;      // bool (0/1)
	int minify_syntax;           // bool (0/1)
	int line_limit;              // int
	int charset;                 // Charset enum
	int tree_shaking;            // TreeShaking enum
	int ignore_annotations;      // bool (0/1)
	int legal_comments;          // LegalComments enum

	// JSX Configuration
	int jsx;                     // JSX enum
	char* jsx_factory;           // string
	char* jsx_fragment;          // string
	char* jsx_import_source;     // string
	int jsx_dev;                 // bool (0/1)
	int jsx_side_effects;        // bool (0/1)

	// TypeScript Configuration
	char* tsconfig_raw;          // string (JSON)

	// Code Injection
	char* banner;                // string
	char* footer;                // string

	// Code Transformation
	char** define_keys;          // keys for map[string]string
	char** define_values;        // values for map[string]string
	int define_count;            // count of define entries
	char** pure;                 // string array
	int pure_count;              // count of pure functions
	int keep_names;              // bool (0/1)

	// Input Configuration
	char* sourcefile;            // string
	int loader;                  // Loader enum
} c_transform_options;

typedef struct {
	char* file;                  // string
	char* namespace;             // string
	int line;                    // int (1-based)
	int column;                  // int (0-based, in bytes)
	int length;                  // int (in bytes)
	char* line_text;             // string
	char* suggestion;            // string
} c_location;

typedef struct {
	char* text;                  // string
	c_location* location;        // optional location
} c_note;

typedef struct {
	char* id;                    // string
	char* plugin_name;           // string
	char* text;                  // string
	c_location* location;        // optional location
	c_note* notes;               // array of notes
	int notes_count;             // count of notes
} c_message;

typedef struct {
	c_message* errors;           // array of error messages
	int errors_count;            // count of errors
	c_message* warnings;         // array of warning messages
	int warnings_count;          // count of warnings
	
	char* code;                  // transformed code as string
	int code_length;             // length of code
	char* source_map;            // source map as string (optional)
	int source_map_length;       // length of map (0 if no map)
	char* legal_comments;        // legal comments as string (optional)
	int legal_comments_length;   // length of legal comments (0 if none)
	
	char** mangle_cache_keys;    // keys for mangle cache
	char** mangle_cache_values;  // values for mangle cache
	int mangle_cache_count;      // count of mangle cache entries
} c_transform_result;
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

// Format enum functions
//
//export esbuild_format_default
func esbuild_format_default() C.int {
	return C.int(api.FormatDefault)
}

//export esbuild_format_iife
func esbuild_format_iife() C.int {
	return C.int(api.FormatIIFE)
}

//export esbuild_format_commonjs
func esbuild_format_commonjs() C.int {
	return C.int(api.FormatCommonJS)
}

//export esbuild_format_esmodule
func esbuild_format_esmodule() C.int {
	return C.int(api.FormatESModule)
}

//export esbuild_get_all_format_values
func esbuild_get_all_format_values() *C.c_int_array {
	formats := []C.int{
		C.int(api.FormatDefault),
		C.int(api.FormatIIFE),
		C.int(api.FormatCommonJS),
		C.int(api.FormatESModule),
	}

	count := len(formats)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))

	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, format := range formats {
		cSlice[i] = format
	}

	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)

	return result
}

// Target enum functions
//
//export esbuild_target_default
func esbuild_target_default() C.int {
	return C.int(api.DefaultTarget)
}

//export esbuild_target_esnext
func esbuild_target_esnext() C.int {
	return C.int(api.ESNext)
}

//export esbuild_target_es5
func esbuild_target_es5() C.int {
	return C.int(api.ES5)
}

//export esbuild_target_es2015
func esbuild_target_es2015() C.int {
	return C.int(api.ES2015)
}

//export esbuild_target_es2016
func esbuild_target_es2016() C.int {
	return C.int(api.ES2016)
}

//export esbuild_target_es2017
func esbuild_target_es2017() C.int {
	return C.int(api.ES2017)
}

//export esbuild_target_es2018
func esbuild_target_es2018() C.int {
	return C.int(api.ES2018)
}

//export esbuild_target_es2019
func esbuild_target_es2019() C.int {
	return C.int(api.ES2019)
}

//export esbuild_target_es2020
func esbuild_target_es2020() C.int {
	return C.int(api.ES2020)
}

//export esbuild_target_es2021
func esbuild_target_es2021() C.int {
	return C.int(api.ES2021)
}

//export esbuild_target_es2022
func esbuild_target_es2022() C.int {
	return C.int(api.ES2022)
}

//export esbuild_target_es2023
func esbuild_target_es2023() C.int {
	return C.int(api.ES2023)
}

//export esbuild_target_es2024
func esbuild_target_es2024() C.int {
	return C.int(api.ES2024)
}

//export esbuild_get_all_target_values
func esbuild_get_all_target_values() *C.c_int_array {
	targets := []C.int{
		C.int(api.DefaultTarget),
		C.int(api.ESNext),
		C.int(api.ES5),
		C.int(api.ES2015),
		C.int(api.ES2016),
		C.int(api.ES2017),
		C.int(api.ES2018),
		C.int(api.ES2019),
		C.int(api.ES2020),
		C.int(api.ES2021),
		C.int(api.ES2022),
		C.int(api.ES2023),
		C.int(api.ES2024),
	}

	count := len(targets)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))

	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, target := range targets {
		cSlice[i] = target
	}

	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)

	return result
}

// Loader enum functions
//
//export esbuild_loader_none
func esbuild_loader_none() C.int { return C.int(api.LoaderNone) }

//export esbuild_loader_base64
func esbuild_loader_base64() C.int { return C.int(api.LoaderBase64) }

//export esbuild_loader_binary
func esbuild_loader_binary() C.int { return C.int(api.LoaderBinary) }

//export esbuild_loader_copy
func esbuild_loader_copy() C.int { return C.int(api.LoaderCopy) }

//export esbuild_loader_css
func esbuild_loader_css() C.int { return C.int(api.LoaderCSS) }

//export esbuild_loader_dataurl
func esbuild_loader_dataurl() C.int { return C.int(api.LoaderDataURL) }

//export esbuild_loader_default
func esbuild_loader_default() C.int { return C.int(api.LoaderDefault) }

//export esbuild_loader_empty
func esbuild_loader_empty() C.int { return C.int(api.LoaderEmpty) }

//export esbuild_loader_file
func esbuild_loader_file() C.int { return C.int(api.LoaderFile) }

//export esbuild_loader_globalcss
func esbuild_loader_globalcss() C.int { return C.int(api.LoaderGlobalCSS) }

//export esbuild_loader_js
func esbuild_loader_js() C.int { return C.int(api.LoaderJS) }

//export esbuild_loader_json
func esbuild_loader_json() C.int { return C.int(api.LoaderJSON) }

//export esbuild_loader_jsx
func esbuild_loader_jsx() C.int { return C.int(api.LoaderJSX) }

//export esbuild_loader_localcss
func esbuild_loader_localcss() C.int { return C.int(api.LoaderLocalCSS) }

//export esbuild_loader_text
func esbuild_loader_text() C.int { return C.int(api.LoaderText) }

//export esbuild_loader_ts
func esbuild_loader_ts() C.int { return C.int(api.LoaderTS) }

//export esbuild_loader_tsx
func esbuild_loader_tsx() C.int { return C.int(api.LoaderTSX) }

//export esbuild_get_all_loader_values
func esbuild_get_all_loader_values() *C.c_int_array {
	loaders := []C.int{
		C.int(api.LoaderNone), C.int(api.LoaderBase64), C.int(api.LoaderBinary), C.int(api.LoaderCopy),
		C.int(api.LoaderCSS), C.int(api.LoaderDataURL), C.int(api.LoaderDefault), C.int(api.LoaderEmpty),
		C.int(api.LoaderFile), C.int(api.LoaderGlobalCSS), C.int(api.LoaderJS), C.int(api.LoaderJSON),
		C.int(api.LoaderJSX), C.int(api.LoaderLocalCSS), C.int(api.LoaderText), C.int(api.LoaderTS), C.int(api.LoaderTSX),
	}
	count := len(loaders)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, loader := range loaders {
		cSlice[i] = loader
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// SourceMap enum functions
//
//export esbuild_sourcemap_none
func esbuild_sourcemap_none() C.int { return C.int(api.SourceMapNone) }

//export esbuild_sourcemap_inline
func esbuild_sourcemap_inline() C.int { return C.int(api.SourceMapInline) }

//export esbuild_sourcemap_linked
func esbuild_sourcemap_linked() C.int { return C.int(api.SourceMapLinked) }

//export esbuild_sourcemap_external
func esbuild_sourcemap_external() C.int { return C.int(api.SourceMapExternal) }

//export esbuild_sourcemap_inlineandexternal
func esbuild_sourcemap_inlineandexternal() C.int { return C.int(api.SourceMapInlineAndExternal) }

//export esbuild_get_all_sourcemap_values
func esbuild_get_all_sourcemap_values() *C.c_int_array {
	sourcemaps := []C.int{
		C.int(api.SourceMapNone), C.int(api.SourceMapInline), C.int(api.SourceMapLinked),
		C.int(api.SourceMapExternal), C.int(api.SourceMapInlineAndExternal),
	}
	count := len(sourcemaps)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, sm := range sourcemaps {
		cSlice[i] = sm
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// JSX enum functions
//
//export esbuild_jsx_transform
func esbuild_jsx_transform() C.int { return C.int(api.JSXTransform) }

//export esbuild_jsx_preserve
func esbuild_jsx_preserve() C.int { return C.int(api.JSXPreserve) }

//export esbuild_jsx_automatic
func esbuild_jsx_automatic() C.int { return C.int(api.JSXAutomatic) }

//export esbuild_get_all_jsx_values
func esbuild_get_all_jsx_values() *C.c_int_array {
	jsxModes := []C.int{
		C.int(api.JSXTransform), C.int(api.JSXPreserve), C.int(api.JSXAutomatic),
	}
	count := len(jsxModes)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, jsx := range jsxModes {
		cSlice[i] = jsx
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// LogLevel enum functions
//
//export esbuild_loglevel_silent
func esbuild_loglevel_silent() C.int { return C.int(api.LogLevelSilent) }

//export esbuild_loglevel_verbose
func esbuild_loglevel_verbose() C.int { return C.int(api.LogLevelVerbose) }

//export esbuild_loglevel_debug
func esbuild_loglevel_debug() C.int { return C.int(api.LogLevelDebug) }

//export esbuild_loglevel_info
func esbuild_loglevel_info() C.int { return C.int(api.LogLevelInfo) }

//export esbuild_loglevel_warning
func esbuild_loglevel_warning() C.int { return C.int(api.LogLevelWarning) }

//export esbuild_loglevel_error
func esbuild_loglevel_error() C.int { return C.int(api.LogLevelError) }

//export esbuild_get_all_loglevel_values
func esbuild_get_all_loglevel_values() *C.c_int_array {
	loglevels := []C.int{
		C.int(api.LogLevelSilent), C.int(api.LogLevelVerbose), C.int(api.LogLevelDebug),
		C.int(api.LogLevelInfo), C.int(api.LogLevelWarning), C.int(api.LogLevelError),
	}
	count := len(loglevels)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, level := range loglevels {
		cSlice[i] = level
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// LegalComments enum functions
//
//export esbuild_legalcomments_default
func esbuild_legalcomments_default() C.int { return C.int(api.LegalCommentsDefault) }

//export esbuild_legalcomments_none
func esbuild_legalcomments_none() C.int { return C.int(api.LegalCommentsNone) }

//export esbuild_legalcomments_inline
func esbuild_legalcomments_inline() C.int { return C.int(api.LegalCommentsInline) }

//export esbuild_legalcomments_endoffile
func esbuild_legalcomments_endoffile() C.int { return C.int(api.LegalCommentsEndOfFile) }

//export esbuild_legalcomments_linked
func esbuild_legalcomments_linked() C.int { return C.int(api.LegalCommentsLinked) }

//export esbuild_legalcomments_external
func esbuild_legalcomments_external() C.int { return C.int(api.LegalCommentsExternal) }

//export esbuild_get_all_legalcomments_values
func esbuild_get_all_legalcomments_values() *C.c_int_array {
	legalcomments := []C.int{
		C.int(api.LegalCommentsDefault), C.int(api.LegalCommentsNone), C.int(api.LegalCommentsInline),
		C.int(api.LegalCommentsEndOfFile), C.int(api.LegalCommentsLinked), C.int(api.LegalCommentsExternal),
	}
	count := len(legalcomments)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, legal := range legalcomments {
		cSlice[i] = legal
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// Charset enum functions
//
//export esbuild_charset_default
func esbuild_charset_default() C.int { return C.int(api.CharsetDefault) }

//export esbuild_charset_ascii
func esbuild_charset_ascii() C.int { return C.int(api.CharsetASCII) }

//export esbuild_charset_utf8
func esbuild_charset_utf8() C.int { return C.int(api.CharsetUTF8) }

//export esbuild_get_all_charset_values
func esbuild_get_all_charset_values() *C.c_int_array {
	charsets := []C.int{
		C.int(api.CharsetDefault), C.int(api.CharsetASCII), C.int(api.CharsetUTF8),
	}
	count := len(charsets)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, charset := range charsets {
		cSlice[i] = charset
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// TreeShaking enum functions
//
//export esbuild_treeshaking_default
func esbuild_treeshaking_default() C.int { return C.int(api.TreeShakingDefault) }

//export esbuild_treeshaking_false
func esbuild_treeshaking_false() C.int { return C.int(api.TreeShakingFalse) }

//export esbuild_treeshaking_true
func esbuild_treeshaking_true() C.int { return C.int(api.TreeShakingTrue) }

//export esbuild_get_all_treeshaking_values
func esbuild_get_all_treeshaking_values() *C.c_int_array {
	treeshaking := []C.int{
		C.int(api.TreeShakingDefault), C.int(api.TreeShakingFalse), C.int(api.TreeShakingTrue),
	}
	count := len(treeshaking)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, tree := range treeshaking {
		cSlice[i] = tree
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// StderrColor enum functions
//
//export esbuild_color_ifterminal
func esbuild_color_ifterminal() C.int { return C.int(api.ColorIfTerminal) }

//export esbuild_color_never
func esbuild_color_never() C.int { return C.int(api.ColorNever) }

//export esbuild_color_always
func esbuild_color_always() C.int { return C.int(api.ColorAlways) }

//export esbuild_get_all_color_values
func esbuild_get_all_color_values() *C.c_int_array {
	colors := []C.int{
		C.int(api.ColorIfTerminal), C.int(api.ColorNever), C.int(api.ColorAlways),
	}
	count := len(colors)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, color := range colors {
		cSlice[i] = color
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// Remaining enums - continuing the pattern for the rest
//
//export esbuild_packages_default
func esbuild_packages_default() C.int { return C.int(api.PackagesDefault) }

//export esbuild_packages_bundle
func esbuild_packages_bundle() C.int { return C.int(api.PackagesBundle) }

//export esbuild_packages_external
func esbuild_packages_external() C.int { return C.int(api.PackagesExternal) }

//export esbuild_get_all_packages_values
func esbuild_get_all_packages_values() *C.c_int_array {
	packages := []C.int{
		C.int(api.PackagesDefault), C.int(api.PackagesBundle), C.int(api.PackagesExternal),
	}
	count := len(packages)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, pkg := range packages {
		cSlice[i] = pkg
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

//export esbuild_sourcescontent_include
func esbuild_sourcescontent_include() C.int { return C.int(api.SourcesContentInclude) }

//export esbuild_sourcescontent_exclude
func esbuild_sourcescontent_exclude() C.int { return C.int(api.SourcesContentExclude) }

//export esbuild_get_all_sourcescontent_values
func esbuild_get_all_sourcescontent_values() *C.c_int_array {
	sourcescontent := []C.int{
		C.int(api.SourcesContentInclude), C.int(api.SourcesContentExclude),
	}
	count := len(sourcescontent)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, sc := range sourcescontent {
		cSlice[i] = sc
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

//export esbuild_manglequoted_false
func esbuild_manglequoted_false() C.int { return C.int(api.MangleQuotedFalse) }

//export esbuild_manglequoted_true
func esbuild_manglequoted_true() C.int { return C.int(api.MangleQuotedTrue) }

//export esbuild_get_all_manglequoted_values
func esbuild_get_all_manglequoted_values() *C.c_int_array {
	manglequoted := []C.int{
		C.int(api.MangleQuotedFalse), C.int(api.MangleQuotedTrue),
	}
	count := len(manglequoted)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, mq := range manglequoted {
		cSlice[i] = mq
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

//export esbuild_drop_console
func esbuild_drop_console() C.int { return C.int(api.DropConsole) }

//export esbuild_drop_debugger
func esbuild_drop_debugger() C.int { return C.int(api.DropDebugger) }

//export esbuild_get_all_drop_values
func esbuild_get_all_drop_values() *C.c_int_array {
	drops := []C.int{
		C.int(api.DropConsole), C.int(api.DropDebugger),
	}
	count := len(drops)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, drop := range drops {
		cSlice[i] = drop
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// EngineName enum functions
//
//export esbuild_engine_chrome
func esbuild_engine_chrome() C.int { return C.int(api.EngineChrome) }

//export esbuild_engine_deno
func esbuild_engine_deno() C.int { return C.int(api.EngineDeno) }

//export esbuild_engine_edge
func esbuild_engine_edge() C.int { return C.int(api.EngineEdge) }

//export esbuild_engine_firefox
func esbuild_engine_firefox() C.int { return C.int(api.EngineFirefox) }

//export esbuild_engine_hermes
func esbuild_engine_hermes() C.int { return C.int(api.EngineHermes) }

//export esbuild_engine_ie
func esbuild_engine_ie() C.int { return C.int(api.EngineIE) }

//export esbuild_engine_ios
func esbuild_engine_ios() C.int { return C.int(api.EngineIOS) }

//export esbuild_engine_node
func esbuild_engine_node() C.int { return C.int(api.EngineNode) }

//export esbuild_engine_opera
func esbuild_engine_opera() C.int { return C.int(api.EngineOpera) }

//export esbuild_engine_rhino
func esbuild_engine_rhino() C.int { return C.int(api.EngineRhino) }

//export esbuild_engine_safari
func esbuild_engine_safari() C.int { return C.int(api.EngineSafari) }

//export esbuild_get_all_engine_values
func esbuild_get_all_engine_values() *C.c_int_array {
	engines := []C.int{
		C.int(api.EngineChrome), C.int(api.EngineDeno), C.int(api.EngineEdge), C.int(api.EngineFirefox),
		C.int(api.EngineHermes), C.int(api.EngineIE), C.int(api.EngineIOS), C.int(api.EngineNode),
		C.int(api.EngineOpera), C.int(api.EngineRhino), C.int(api.EngineSafari),
	}
	count := len(engines)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, engine := range engines {
		cSlice[i] = engine
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// SideEffects enum functions
//
//export esbuild_sideeffects_true
func esbuild_sideeffects_true() C.int { return C.int(api.SideEffectsTrue) }

//export esbuild_sideeffects_false
func esbuild_sideeffects_false() C.int { return C.int(api.SideEffectsFalse) }

//export esbuild_get_all_sideeffects_values
func esbuild_get_all_sideeffects_values() *C.c_int_array {
	sideEffects := []C.int{
		C.int(api.SideEffectsTrue), C.int(api.SideEffectsFalse),
	}
	count := len(sideEffects)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, se := range sideEffects {
		cSlice[i] = se
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// ResolveKind enum functions
//
//export esbuild_resolvekind_none
func esbuild_resolvekind_none() C.int { return C.int(api.ResolveNone) }

//export esbuild_resolvekind_entrypoint
func esbuild_resolvekind_entrypoint() C.int { return C.int(api.ResolveEntryPoint) }

//export esbuild_resolvekind_jsimportstatement
func esbuild_resolvekind_jsimportstatement() C.int { return C.int(api.ResolveJSImportStatement) }

//export esbuild_resolvekind_jsrequirecall
func esbuild_resolvekind_jsrequirecall() C.int { return C.int(api.ResolveJSRequireCall) }

//export esbuild_resolvekind_jsdynamicimport
func esbuild_resolvekind_jsdynamicimport() C.int { return C.int(api.ResolveJSDynamicImport) }

//export esbuild_resolvekind_jsrequireresolve
func esbuild_resolvekind_jsrequireresolve() C.int { return C.int(api.ResolveJSRequireResolve) }

//export esbuild_resolvekind_cssimportrule
func esbuild_resolvekind_cssimportrule() C.int { return C.int(api.ResolveCSSImportRule) }

//export esbuild_resolvekind_csscomposesfrom
func esbuild_resolvekind_csscomposesfrom() C.int { return C.int(api.ResolveCSSComposesFrom) }

//export esbuild_resolvekind_cssurltoken
func esbuild_resolvekind_cssurltoken() C.int { return C.int(api.ResolveCSSURLToken) }

//export esbuild_get_all_resolvekind_values
func esbuild_get_all_resolvekind_values() *C.c_int_array {
	resolveKinds := []C.int{
		C.int(api.ResolveNone), C.int(api.ResolveEntryPoint), C.int(api.ResolveJSImportStatement),
		C.int(api.ResolveJSRequireCall), C.int(api.ResolveJSDynamicImport), C.int(api.ResolveJSRequireResolve),
		C.int(api.ResolveCSSImportRule), C.int(api.ResolveCSSComposesFrom), C.int(api.ResolveCSSURLToken),
	}
	count := len(resolveKinds)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, rk := range resolveKinds {
		cSlice[i] = rk
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// MessageKind enum functions
//
//export esbuild_messagekind_error
func esbuild_messagekind_error() C.int { return C.int(api.ErrorMessage) }

//export esbuild_messagekind_warning
func esbuild_messagekind_warning() C.int { return C.int(api.WarningMessage) }

//export esbuild_get_all_messagekind_values
func esbuild_get_all_messagekind_values() *C.c_int_array {
	messageKinds := []C.int{
		C.int(api.ErrorMessage), C.int(api.WarningMessage),
	}
	count := len(messageKinds)
	size := C.size_t(count) * C.size_t(unsafe.Sizeof(C.int(0)))
	cArray := (*C.int)(C.malloc(size))
	cSlice := (*[1 << 28]C.int)(unsafe.Pointer(cArray))[:count:count]
	for i, mk := range messageKinds {
		cSlice[i] = mk
	}
	result := (*C.c_int_array)(C.malloc(C.sizeof_c_int_array))
	result.values = cArray
	result.count = C.int(count)
	return result
}

// TransformOptions C bridge struct

//export esbuild_create_transform_options
func esbuild_create_transform_options() *C.c_transform_options {
	return (*C.c_transform_options)(C.malloc(C.sizeof_c_transform_options))
}

//export esbuild_free_transform_options
func esbuild_free_transform_options(opts *C.c_transform_options) {
	if opts == nil {
		return
	}
	
	// Free all string fields
	if opts.source_root != nil {
		C.free(unsafe.Pointer(opts.source_root))
	}
	if opts.global_name != nil {
		C.free(unsafe.Pointer(opts.global_name))
	}
	if opts.mangle_props != nil {
		C.free(unsafe.Pointer(opts.mangle_props))
	}
	if opts.reserve_props != nil {
		C.free(unsafe.Pointer(opts.reserve_props))
	}
	if opts.jsx_factory != nil {
		C.free(unsafe.Pointer(opts.jsx_factory))
	}
	if opts.jsx_fragment != nil {
		C.free(unsafe.Pointer(opts.jsx_fragment))
	}
	if opts.jsx_import_source != nil {
		C.free(unsafe.Pointer(opts.jsx_import_source))
	}
	if opts.tsconfig_raw != nil {
		C.free(unsafe.Pointer(opts.tsconfig_raw))
	}
	if opts.banner != nil {
		C.free(unsafe.Pointer(opts.banner))
	}
	if opts.footer != nil {
		C.free(unsafe.Pointer(opts.footer))
	}
	if opts.sourcefile != nil {
		C.free(unsafe.Pointer(opts.sourcefile))
	}
	
	// Free string arrays
	if opts.log_override_keys != nil {
		for i := 0; i < int(opts.log_override_count); i++ {
			if ptr := *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(opts.log_override_keys)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil)))); ptr != nil {
				C.free(unsafe.Pointer(ptr))
			}
		}
		C.free(unsafe.Pointer(opts.log_override_keys))
	}
	if opts.log_override_values != nil {
		C.free(unsafe.Pointer(opts.log_override_values))
	}
	
	if opts.engine_names != nil {
		C.free(unsafe.Pointer(opts.engine_names))
	}
	if opts.engine_versions != nil {
		for i := 0; i < int(opts.engines_count); i++ {
			if ptr := *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(opts.engine_versions)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil)))); ptr != nil {
				C.free(unsafe.Pointer(ptr))
			}
		}
		C.free(unsafe.Pointer(opts.engine_versions))
	}
	
	// Free other arrays (supported, mangle_cache, drop_labels, define, pure)
	// ... similar pattern for all array fields
	
	C.free(unsafe.Pointer(opts))
}

// TransformResult C bridge functions

//export esbuild_create_transform_result
func esbuild_create_transform_result() *C.c_transform_result {
	return (*C.c_transform_result)(C.malloc(C.sizeof_c_transform_result))
}

//export esbuild_create_location
func esbuild_create_location() *C.c_location {
	return (*C.c_location)(C.malloc(C.sizeof_c_location))
}

//export esbuild_create_note
func esbuild_create_note() *C.c_note {
	return (*C.c_note)(C.malloc(C.sizeof_c_note))
}

//export esbuild_create_message
func esbuild_create_message() *C.c_message {
	return (*C.c_message)(C.malloc(C.sizeof_c_message))
}

//export esbuild_free_location
func esbuild_free_location(loc *C.c_location) {
	if loc == nil {
		return
	}
	
	if loc.file != nil {
		C.free(unsafe.Pointer(loc.file))
	}
	if loc.namespace != nil {
		C.free(unsafe.Pointer(loc.namespace))
	}
	if loc.line_text != nil {
		C.free(unsafe.Pointer(loc.line_text))
	}
	if loc.suggestion != nil {
		C.free(unsafe.Pointer(loc.suggestion))
	}
	
	C.free(unsafe.Pointer(loc))
}

// Internal function to free note contents without freeing the struct itself
func esbuild_free_note_contents(note *C.c_note) {
	if note == nil {
		return
	}
	
	if note.text != nil {
		C.free(unsafe.Pointer(note.text))
	}
	if note.location != nil {
		esbuild_free_location(note.location)
	}
}

//export esbuild_free_note
func esbuild_free_note(note *C.c_note) {
	if note == nil {
		return
	}
	
	esbuild_free_note_contents(note)
	C.free(unsafe.Pointer(note))
}

// Internal function to free message contents without freeing the struct itself
func esbuild_free_message_contents(msg *C.c_message) {
	if msg == nil {
		return
	}
	
	if msg.id != nil {
		C.free(unsafe.Pointer(msg.id))
	}
	if msg.plugin_name != nil {
		C.free(unsafe.Pointer(msg.plugin_name))
	}
	if msg.text != nil {
		C.free(unsafe.Pointer(msg.text))
	}
	if msg.location != nil {
		esbuild_free_location(msg.location)
	}
	
	// Free notes array
	if msg.notes != nil {
		noteSlice := (*[1 << 28]C.c_note)(unsafe.Pointer(msg.notes))[:msg.notes_count:msg.notes_count]
		for i := 0; i < int(msg.notes_count); i++ {
			// Free contents only, not the struct itself (it's part of the array)
			esbuild_free_note_contents(&noteSlice[i])
		}
		C.free(unsafe.Pointer(msg.notes))
	}
}

//export esbuild_free_message
func esbuild_free_message(msg *C.c_message) {
	if msg == nil {
		return
	}
	
	esbuild_free_message_contents(msg)
	C.free(unsafe.Pointer(msg))
}

//export esbuild_free_transform_result
func esbuild_free_transform_result(result *C.c_transform_result) {
	if result == nil {
		return
	}
	
	// Free errors array
	if result.errors != nil {
		errorSlice := (*[1 << 28]C.c_message)(unsafe.Pointer(result.errors))[:result.errors_count:result.errors_count]
		for i := 0; i < int(result.errors_count); i++ {
			// Free contents only, not the struct itself (it's part of the array)
			esbuild_free_message_contents(&errorSlice[i])
		}
		C.free(unsafe.Pointer(result.errors))
	}
	
	// Free warnings array
	if result.warnings != nil {
		warningSlice := (*[1 << 28]C.c_message)(unsafe.Pointer(result.warnings))[:result.warnings_count:result.warnings_count]
		for i := 0; i < int(result.warnings_count); i++ {
			// Free contents only, not the struct itself (it's part of the array)
			esbuild_free_message_contents(&warningSlice[i])
		}
		C.free(unsafe.Pointer(result.warnings))
	}
	
	// Free string fields
	if result.code != nil {
		C.free(unsafe.Pointer(result.code))
	}
	if result.source_map != nil {
		C.free(unsafe.Pointer(result.source_map))
	}
	if result.legal_comments != nil {
		C.free(unsafe.Pointer(result.legal_comments))
	}
	
	// Free mangle cache arrays
	if result.mangle_cache_keys != nil {
		keySlice := (*[1 << 28]*C.char)(unsafe.Pointer(result.mangle_cache_keys))[:result.mangle_cache_count:result.mangle_cache_count]
		for i := 0; i < int(result.mangle_cache_count); i++ {
			if keySlice[i] != nil {
				C.free(unsafe.Pointer(keySlice[i]))
			}
		}
		C.free(unsafe.Pointer(result.mangle_cache_keys))
	}
	if result.mangle_cache_values != nil {
		valueSlice := (*[1 << 28]*C.char)(unsafe.Pointer(result.mangle_cache_values))[:result.mangle_cache_count:result.mangle_cache_count]
		for i := 0; i < int(result.mangle_cache_count); i++ {
			if valueSlice[i] != nil {
				C.free(unsafe.Pointer(valueSlice[i]))
			}
		}
		C.free(unsafe.Pointer(result.mangle_cache_values))
	}
	
	C.free(unsafe.Pointer(result))
}

//export esbuild_transform
func esbuild_transform(code *C.char, opts *C.c_transform_options) *C.c_transform_result {
	if code == nil || opts == nil {
		return nil
	}
	
	// Convert C string to Go string
	sourceCode := C.GoString(code)
	
	// Convert C options to Go options
	transformOpts := api.TransformOptions{}
	
	// Basic settings
	transformOpts.Color = api.StderrColor(opts.color)
	transformOpts.LogLevel = api.LogLevel(opts.log_level)
	transformOpts.LogLimit = int(opts.log_limit)
	
	// Source map
	transformOpts.Sourcemap = api.SourceMap(opts.sourcemap)
	if opts.source_root != nil {
		sourceRoot := C.GoString(opts.source_root)
		transformOpts.SourceRoot = sourceRoot
	}
	transformOpts.SourcesContent = api.SourcesContent(opts.sources_content)
	
	// Target and platform
	transformOpts.Target = api.Target(opts.target)
	transformOpts.Platform = api.Platform(opts.platform)
	transformOpts.Format = api.Format(opts.format)
	
	if opts.global_name != nil {
		globalName := C.GoString(opts.global_name)
		transformOpts.GlobalName = globalName
	}
	
	// Minification
	if opts.mangle_props != nil {
		mangleProps := C.GoString(opts.mangle_props)
		transformOpts.MangleProps = mangleProps
	}
	if opts.reserve_props != nil {
		reserveProps := C.GoString(opts.reserve_props)
		transformOpts.ReserveProps = reserveProps
	}
	transformOpts.MangleQuoted = api.MangleQuoted(opts.mangle_quoted)
	transformOpts.MinifyWhitespace = opts.minify_whitespace != 0
	transformOpts.MinifyIdentifiers = opts.minify_identifiers != 0
	transformOpts.MinifySyntax = opts.minify_syntax != 0
	transformOpts.Charset = api.Charset(opts.charset)
	transformOpts.TreeShaking = api.TreeShaking(opts.tree_shaking)
	transformOpts.IgnoreAnnotations = opts.ignore_annotations != 0
	transformOpts.LegalComments = api.LegalComments(opts.legal_comments)
	
	// JSX
	transformOpts.JSX = api.JSX(opts.jsx)
	if opts.jsx_factory != nil {
		jsxFactory := C.GoString(opts.jsx_factory)
		transformOpts.JSXFactory = jsxFactory
	}
	if opts.jsx_fragment != nil {
		jsxFragment := C.GoString(opts.jsx_fragment)
		transformOpts.JSXFragment = jsxFragment
	}
	if opts.jsx_import_source != nil {
		jsxImportSource := C.GoString(opts.jsx_import_source)
		transformOpts.JSXImportSource = jsxImportSource
	}
	transformOpts.JSXDev = opts.jsx_dev != 0
	transformOpts.JSXSideEffects = opts.jsx_side_effects != 0
	
	// TypeScript
	if opts.tsconfig_raw != nil {
		tsconfigRaw := C.GoString(opts.tsconfig_raw)
		transformOpts.TsconfigRaw = tsconfigRaw
	}
	
	// Code injection
	if opts.banner != nil {
		banner := C.GoString(opts.banner)
		transformOpts.Banner = banner
	}
	if opts.footer != nil {
		footer := C.GoString(opts.footer)
		transformOpts.Footer = footer
	}
	
	// Input configuration
	if opts.sourcefile != nil {
		sourcefile := C.GoString(opts.sourcefile)
		transformOpts.Sourcefile = sourcefile
	}
	transformOpts.Loader = api.Loader(opts.loader)
	transformOpts.KeepNames = opts.keep_names != 0
	
	// Call esbuild transform
	result := api.Transform(sourceCode, transformOpts)
	
	// Create C result
	cResult := esbuild_create_transform_result()
	if cResult == nil {
		return nil
	}
	
	// Convert code
	if len(result.Code) > 0 {
		cResult.code = C.CString(string(result.Code))
		cResult.code_length = C.int(len(result.Code))
	}
	
	// Convert source map
	if len(result.Map) > 0 {
		cResult.source_map = C.CString(string(result.Map))
		cResult.source_map_length = C.int(len(result.Map))
	}
	
	// Convert legal comments
	if len(result.LegalComments) > 0 {
		cResult.legal_comments = C.CString(string(result.LegalComments))
		cResult.legal_comments_length = C.int(len(result.LegalComments))
	}
	
	// Convert mangle cache
	if len(result.MangleCache) > 0 {
		cResult.mangle_cache_count = C.int(len(result.MangleCache))
		cResult.mangle_cache_keys = (**C.char)(C.malloc(C.size_t(len(result.MangleCache)) * C.size_t(unsafe.Sizeof(uintptr(0)))))
		cResult.mangle_cache_values = (**C.char)(C.malloc(C.size_t(len(result.MangleCache)) * C.size_t(unsafe.Sizeof(uintptr(0)))))
		
		keySlice := (*[1 << 28]*C.char)(unsafe.Pointer(cResult.mangle_cache_keys))[:len(result.MangleCache):len(result.MangleCache)]
		valueSlice := (*[1 << 28]*C.char)(unsafe.Pointer(cResult.mangle_cache_values))[:len(result.MangleCache):len(result.MangleCache)]
		
		i := 0
		for key, value := range result.MangleCache {
			keySlice[i] = C.CString(key)
			valueSlice[i] = C.CString(value.(string))
			i++
		}
	}
	
	// Convert errors with proper memory management
	if len(result.Errors) > 0 {
		cResult.errors = (*C.c_message)(C.malloc(C.size_t(len(result.Errors)) * C.size_t(unsafe.Sizeof(C.c_message{}))))
		cResult.errors_count = C.int(len(result.Errors))
		
		errorsSlice := (*[1 << 20]C.c_message)(unsafe.Pointer(cResult.errors))[:len(result.Errors):len(result.Errors)]
		for i, err := range result.Errors {
			errorsSlice[i].id = C.CString(err.ID)
			errorsSlice[i].plugin_name = C.CString(err.PluginName)
			errorsSlice[i].text = C.CString(err.Text)
			errorsSlice[i].location = nil
			errorsSlice[i].notes = nil
			errorsSlice[i].notes_count = 0
			
			// Add location if available
			if err.Location != nil {
				loc := (*C.c_location)(C.malloc(C.size_t(unsafe.Sizeof(C.c_location{}))))
				loc.file = C.CString(err.Location.File)
				loc.namespace = C.CString(err.Location.Namespace)
				loc.line = C.int(err.Location.Line)
				loc.column = C.int(err.Location.Column)
				loc.length = C.int(err.Location.Length)
				loc.line_text = C.CString(err.Location.LineText)
				loc.suggestion = C.CString(err.Location.Suggestion)
				errorsSlice[i].location = loc
			}
		}
	} else {
		cResult.errors = nil
		cResult.errors_count = 0
	}
	
	// Convert warnings with proper memory management
	if len(result.Warnings) > 0 {
		cResult.warnings = (*C.c_message)(C.malloc(C.size_t(len(result.Warnings)) * C.size_t(unsafe.Sizeof(C.c_message{}))))
		cResult.warnings_count = C.int(len(result.Warnings))
		
		warningsSlice := (*[1 << 20]C.c_message)(unsafe.Pointer(cResult.warnings))[:len(result.Warnings):len(result.Warnings)]
		for i, warn := range result.Warnings {
			warningsSlice[i].id = C.CString(warn.ID)
			warningsSlice[i].plugin_name = C.CString(warn.PluginName)
			warningsSlice[i].text = C.CString(warn.Text)
			warningsSlice[i].location = nil
			warningsSlice[i].notes = nil
			warningsSlice[i].notes_count = 0
			
			// Add location if available
			if warn.Location != nil {
				loc := (*C.c_location)(C.malloc(C.size_t(unsafe.Sizeof(C.c_location{}))))
				loc.file = C.CString(warn.Location.File)
				loc.namespace = C.CString(warn.Location.Namespace)
				loc.line = C.int(warn.Location.Line)
				loc.column = C.int(warn.Location.Column)
				loc.length = C.int(warn.Location.Length)
				loc.line_text = C.CString(warn.Location.LineText)
				loc.suggestion = C.CString(warn.Location.Suggestion)
				warningsSlice[i].location = loc
			}
		}
	} else {
		cResult.warnings = nil
		cResult.warnings_count = 0
	}
	
	return cResult
}
