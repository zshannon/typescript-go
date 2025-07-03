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

typedef struct {
	char* input_path;
	char* output_path;
} esbuild_entry_point;

typedef struct {
	char* contents;
	char* resolve_dir;
	char* sourcefile;
	int loader;
} esbuild_stdin_options;

typedef struct {
	char* path;
	char* contents;
	int contents_length;
	char* hash;
} esbuild_output_file;

// Plugin API structures - moved here to be available for build options

typedef enum {
	RESOLVE_KIND_ENTRY_POINT = 0,
	RESOLVE_KIND_IMPORT_STATEMENT = 1,
	RESOLVE_KIND_REQUIRE_CALL = 2,
	RESOLVE_KIND_DYNAMIC_IMPORT = 3,
	RESOLVE_KIND_REQUIRE_RESOLVE = 4,
	RESOLVE_KIND_IMPORT_RULE = 5,
	RESOLVE_KIND_COMPOSES_FROM = 6,
	RESOLVE_KIND_URL_TOKEN = 7
} c_resolve_kind;

typedef struct {
	char* path;                  // string
	char* importer;              // string
	char* namespace;             // string
	char* resolve_dir;           // string
	int kind;                    // c_resolve_kind enum
	char* plugin_data;           // JSON string for pluginData
	char** with_keys;            // keys for with map
	char** with_values;          // values for with map
	int with_count;              // count of with entries
} c_on_resolve_args;

typedef struct {
	char* path;                  // string
	char* namespace;             // string
	char* suffix;                // string
	char* plugin_data;           // JSON string for pluginData
	char** with_keys;            // keys for with map
	char** with_values;          // values for with map
	int with_count;              // count of with entries
} c_on_load_args;

typedef struct {
	char* path;                  // optional string
	char* namespace;             // optional string
	int external;                // bool (0/1) (-1 for nil)
	int side_effects;            // bool (0/1) (-1 for nil)
	char* suffix;                // optional string
	char* plugin_data;           // JSON string for pluginData
	char* plugin_name;           // optional string
	c_message* errors;           // array of errors
	int errors_count;            // count of errors
	c_message* warnings;         // array of warnings
	int warnings_count;          // count of warnings
	char** watch_files;          // array of file paths
	int watch_files_count;       // count of watch files
	char** watch_dirs;           // array of directory paths
	int watch_dirs_count;        // count of watch dirs
} c_on_resolve_result;

typedef struct {
	char* contents;              // optional byte array
	int contents_length;         // length of contents (0 if nil)
	int loader;                  // Loader enum (-1 for nil)
	char* resolve_dir;           // optional string
	char* plugin_data;           // JSON string for pluginData
	char* plugin_name;           // optional string
	c_message* errors;           // array of errors
	int errors_count;            // count of errors
	c_message* warnings;         // array of warnings
	int warnings_count;          // count of warnings
	char** watch_files;          // array of file paths
	int watch_files_count;       // count of watch files
	char** watch_dirs;           // array of directory paths
	int watch_dirs_count;        // count of watch dirs
} c_on_load_result;

// Plugin callback function pointer types
typedef c_on_resolve_result* (*on_resolve_callback)(c_on_resolve_args*, void*);
typedef c_on_load_result* (*on_load_callback)(c_on_load_args*, void*);
typedef void (*on_start_callback)(void*);
typedef void (*on_end_callback)(void*);

typedef struct {
	char* filter;                // filter regex for the callback
	char* namespace;             // namespace filter (optional)
	on_resolve_callback callback; // callback function pointer
	void* callback_data;         // Swift closure context
} c_plugin_resolve_hook;

typedef struct {
	char* filter;                // filter regex for the callback
	char* namespace;             // namespace filter (optional) 
	on_load_callback callback;   // callback function pointer
	void* callback_data;         // Swift closure context
} c_plugin_load_hook;

typedef struct {
	char* name;                  // plugin name
	c_plugin_resolve_hook* resolve_hooks; // array of resolve hooks
	int resolve_hooks_count;     // count of resolve hooks
	c_plugin_load_hook* load_hooks; // array of load hooks
	int load_hooks_count;        // count of load hooks
	on_start_callback on_start;  // start callback (optional)
	on_end_callback on_end;      // end callback (optional)
	void* start_data;            // start callback context
	void* end_data;              // end callback context
} c_plugin;

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
	char* tsconfig;              // string (file path)
	char* tsconfig_raw;          // string (JSON)

	// Code Injection
	char** banner_keys;          // keys for map[string]string (file types)
	char** banner_values;        // values for map[string]string
	int banner_count;            // count of banner entries
	char** footer_keys;          // keys for map[string]string (file types)
	char** footer_values;        // values for map[string]string
	int footer_count;            // count of footer entries

	// Code Transformation
	char** define_keys;          // keys for map[string]string
	char** define_values;        // values for map[string]string
	int define_count;            // count of define entries
	char** pure;                 // string array
	int pure_count;              // count of pure functions
	int keep_names;              // bool (0/1)

	// Build Configuration
	int bundle;                  // bool (0/1)
	int preserve_symlinks;       // bool (0/1)
	int splitting;               // bool (0/1)
	char* outfile;               // string
	char* outdir;                // string
	char* outbase;               // string
	char* abs_working_dir;       // string
	int metafile;                // bool (0/1)
	int write;                   // bool (0/1)
	int allow_overwrite;         // bool (0/1)

	// Module Resolution
	char** external;             // string array
	int external_count;          // count of external entries
	int packages;                // Packages enum
	char** alias_keys;           // keys for map[string]string
	char** alias_values;         // values for map[string]string
	int alias_count;             // count of alias entries
	char** main_fields;          // string array
	int main_fields_count;       // count of main fields
	char** conditions;           // string array
	int conditions_count;        // count of conditions
	char** loader_keys;          // keys for map[string]Loader (file extensions)
	int* loader_values;          // values for map[string]Loader
	int loader_count;            // count of loader entries
	char** resolve_extensions;   // string array
	int resolve_extensions_count; // count of resolve extensions
	char** out_extension_keys;   // keys for map[string]string
	char** out_extension_values; // values for map[string]string
	int out_extension_count;     // count of out extension entries
	char* public_path;           // string
	char** inject;               // string array
	int inject_count;            // count of inject entries
	char** node_paths;           // string array
	int node_paths_count;        // count of node paths

	// Naming Templates
	char* entry_names;           // string
	char* chunk_names;           // string
	char* asset_names;           // string

	// Input Configuration
	char** entry_points;         // string array (simple entry points)
	int entry_points_count;      // count of entry points
	esbuild_entry_point* entry_points_advanced; // advanced entry points
	int entry_points_advanced_count;      // count of advanced entry points
	esbuild_stdin_options* stdin;      // stdin options (optional)

	// Plugin Configuration
	c_plugin* plugins;           // array of plugins
	int plugins_count;           // count of plugins
} esbuild_build_options;

typedef struct {
	c_message* errors;           // array of error messages
	int errors_count;            // count of errors
	c_message* warnings;         // array of warning messages
	int warnings_count;          // count of warnings
	esbuild_output_file* output_files; // array of output files
	int output_files_count;      // count of output files
	char* metafile;              // metafile JSON as string
	char** mangle_cache_keys;    // keys for mangle cache
	char** mangle_cache_values;  // values for mangle cache
	int mangle_cache_count;      // count of mangle cache entries
} esbuild_build_result;

// Forward declarations for Swift callback functions
c_on_resolve_result* swift_plugin_on_resolve_callback(c_on_resolve_args* args, void* callbackData);
c_on_load_result* swift_plugin_on_load_callback(c_on_load_args* args, void* callbackData);
void swift_plugin_on_start_callback(void* callbackData);
void swift_plugin_on_end_callback(void* callbackData);
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

// Build API Functions

//export esbuild_create_entry_point
func esbuild_create_entry_point() *C.esbuild_entry_point {
	return (*C.esbuild_entry_point)(C.malloc(C.size_t(unsafe.Sizeof(C.esbuild_entry_point{}))))
}

//export esbuild_create_stdin_options
func esbuild_create_stdin_options() *C.esbuild_stdin_options {
	return (*C.esbuild_stdin_options)(C.malloc(C.size_t(unsafe.Sizeof(C.esbuild_stdin_options{}))))
}

//export esbuild_create_output_file
func esbuild_create_output_file() *C.esbuild_output_file {
	return (*C.esbuild_output_file)(C.malloc(C.size_t(unsafe.Sizeof(C.esbuild_output_file{}))))
}

//export esbuild_create_build_options
func esbuild_create_build_options() *C.esbuild_build_options {
	options := (*C.esbuild_build_options)(C.malloc(C.size_t(unsafe.Sizeof(C.esbuild_build_options{}))))
	
	// Initialize all pointers to nil and counts to 0
	options.log_override_keys = nil
	options.log_override_values = nil
	options.log_override_count = 0
	options.source_root = nil
	options.engine_names = nil
	options.engine_versions = nil
	options.engines_count = 0
	options.supported_keys = nil
	options.supported_values = nil
	options.supported_count = 0
	options.global_name = nil
	options.mangle_props = nil
	options.reserve_props = nil
	options.mangle_cache_keys = nil
	options.mangle_cache_values = nil
	options.mangle_cache_count = 0
	options.drop_labels = nil
	options.drop_labels_count = 0
	options.jsx_factory = nil
	options.jsx_fragment = nil
	options.jsx_import_source = nil
	options.tsconfig = nil
	options.tsconfig_raw = nil
	options.banner_keys = nil
	options.banner_values = nil
	options.banner_count = 0
	options.footer_keys = nil
	options.footer_values = nil
	options.footer_count = 0
	options.define_keys = nil
	options.define_values = nil
	options.define_count = 0
	options.pure = nil
	options.pure_count = 0
	options.outfile = nil
	options.outdir = nil
	options.outbase = nil
	options.abs_working_dir = nil
	options.external = nil
	options.external_count = 0
	options.alias_keys = nil
	options.alias_values = nil
	options.alias_count = 0
	options.main_fields = nil
	options.main_fields_count = 0
	options.conditions = nil
	options.conditions_count = 0
	options.loader_keys = nil
	options.loader_values = nil
	options.loader_count = 0
	options.resolve_extensions = nil
	options.resolve_extensions_count = 0
	options.out_extension_keys = nil
	options.out_extension_values = nil
	options.out_extension_count = 0
	options.public_path = nil
	options.inject = nil
	options.inject_count = 0
	options.node_paths = nil
	options.node_paths_count = 0
	options.entry_names = nil
	options.chunk_names = nil
	options.asset_names = nil
	options.entry_points = nil
	options.entry_points_count = 0
	options.entry_points_advanced = nil
	options.entry_points_advanced_count = 0
	options.stdin = nil
	
	return options
}

//export esbuild_create_build_result
func esbuild_create_build_result() *C.esbuild_build_result {
	result := (*C.esbuild_build_result)(C.malloc(C.size_t(unsafe.Sizeof(C.esbuild_build_result{}))))
	
	// Initialize all pointers to nil and counts to 0
	result.errors = nil
	result.errors_count = 0
	result.warnings = nil
	result.warnings_count = 0
	result.output_files = nil
	result.output_files_count = 0
	result.metafile = nil
	result.mangle_cache_keys = nil
	result.mangle_cache_values = nil
	result.mangle_cache_count = 0
	
	return result
}

//export esbuild_free_entry_point
func esbuild_free_entry_point(ep *C.esbuild_entry_point) {
	if ep == nil {
		return
	}
	
	if ep.input_path != nil {
		C.free(unsafe.Pointer(ep.input_path))
	}
	if ep.output_path != nil {
		C.free(unsafe.Pointer(ep.output_path))
	}
	
	C.free(unsafe.Pointer(ep))
}

//export esbuild_free_stdin_options
func esbuild_free_stdin_options(stdin *C.esbuild_stdin_options) {
	if stdin == nil {
		return
	}
	
	if stdin.contents != nil {
		C.free(unsafe.Pointer(stdin.contents))
	}
	if stdin.resolve_dir != nil {
		C.free(unsafe.Pointer(stdin.resolve_dir))
	}
	if stdin.sourcefile != nil {
		C.free(unsafe.Pointer(stdin.sourcefile))
	}
	
	C.free(unsafe.Pointer(stdin))
}

//export esbuild_free_output_file
func esbuild_free_output_file(file *C.esbuild_output_file) {
	if file == nil {
		return
	}
	
	if file.path != nil {
		C.free(unsafe.Pointer(file.path))
	}
	if file.contents != nil {
		C.free(unsafe.Pointer(file.contents))
	}
	if file.hash != nil {
		C.free(unsafe.Pointer(file.hash))
	}
	
	C.free(unsafe.Pointer(file))
}

//export esbuild_free_build_options
func esbuild_free_build_options(opts *C.esbuild_build_options) {
	if opts == nil {
		return
	}
	
	// Free arrays and their string contents
	if opts.log_override_keys != nil {
		for i := 0; i < int(opts.log_override_count); i++ {
			if opts.log_override_keys != nil {
				key := (*[1000]*C.char)(unsafe.Pointer(opts.log_override_keys))[i]
				if key != nil {
					C.free(unsafe.Pointer(key))
				}
			}
		}
		C.free(unsafe.Pointer(opts.log_override_keys))
	}
	if opts.log_override_values != nil {
		C.free(unsafe.Pointer(opts.log_override_values))
	}
	
	// Free simple string fields
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
	if opts.tsconfig != nil {
		C.free(unsafe.Pointer(opts.tsconfig))
	}
	if opts.tsconfig_raw != nil {
		C.free(unsafe.Pointer(opts.tsconfig_raw))
	}
	if opts.outfile != nil {
		C.free(unsafe.Pointer(opts.outfile))
	}
	if opts.outdir != nil {
		C.free(unsafe.Pointer(opts.outdir))
	}
	if opts.outbase != nil {
		C.free(unsafe.Pointer(opts.outbase))
	}
	if opts.abs_working_dir != nil {
		C.free(unsafe.Pointer(opts.abs_working_dir))
	}
	if opts.public_path != nil {
		C.free(unsafe.Pointer(opts.public_path))
	}
	if opts.entry_names != nil {
		C.free(unsafe.Pointer(opts.entry_names))
	}
	if opts.chunk_names != nil {
		C.free(unsafe.Pointer(opts.chunk_names))
	}
	if opts.asset_names != nil {
		C.free(unsafe.Pointer(opts.asset_names))
	}
	
	// Free more complex arrays (simplified for now - full implementation would free all arrays)
	
	if opts.stdin != nil {
		esbuild_free_stdin_options(opts.stdin)
	}
	
	C.free(unsafe.Pointer(opts))
}

//export esbuild_free_build_result
func esbuild_free_build_result(result *C.esbuild_build_result) {
	if result == nil {
		return
	}
	
	// Free errors array
	if result.errors != nil {
		for i := 0; i < int(result.errors_count); i++ {
			errorPtr := (*C.c_message)(unsafe.Pointer(uintptr(unsafe.Pointer(result.errors)) + uintptr(i)*unsafe.Sizeof(C.c_message{})))
			esbuild_free_message_contents(errorPtr)
		}
		C.free(unsafe.Pointer(result.errors))
	}
	
	// Free warnings array
	if result.warnings != nil {
		for i := 0; i < int(result.warnings_count); i++ {
			warningPtr := (*C.c_message)(unsafe.Pointer(uintptr(unsafe.Pointer(result.warnings)) + uintptr(i)*unsafe.Sizeof(C.c_message{})))
			esbuild_free_message_contents(warningPtr)
		}
		C.free(unsafe.Pointer(result.warnings))
	}
	
	// Free output files array
	if result.output_files != nil {
		for i := 0; i < int(result.output_files_count); i++ {
			filePtr := (*C.esbuild_output_file)(unsafe.Pointer(uintptr(unsafe.Pointer(result.output_files)) + uintptr(i)*unsafe.Sizeof(C.esbuild_output_file{})))
			if filePtr.path != nil {
				C.free(unsafe.Pointer(filePtr.path))
			}
			if filePtr.contents != nil {
				C.free(unsafe.Pointer(filePtr.contents))
			}
			if filePtr.hash != nil {
				C.free(unsafe.Pointer(filePtr.hash))
			}
		}
		C.free(unsafe.Pointer(result.output_files))
	}
	
	// Free metafile
	if result.metafile != nil {
		C.free(unsafe.Pointer(result.metafile))
	}
	
	// Free mangle cache
	if result.mangle_cache_keys != nil {
		for i := 0; i < int(result.mangle_cache_count); i++ {
			key := (*[1000]*C.char)(unsafe.Pointer(result.mangle_cache_keys))[i]
			if key != nil {
				C.free(unsafe.Pointer(key))
			}
		}
		C.free(unsafe.Pointer(result.mangle_cache_keys))
	}
	if result.mangle_cache_values != nil {
		for i := 0; i < int(result.mangle_cache_count); i++ {
			value := (*[1000]*C.char)(unsafe.Pointer(result.mangle_cache_values))[i]
			if value != nil {
				C.free(unsafe.Pointer(value))
			}
		}
		C.free(unsafe.Pointer(result.mangle_cache_values))
	}
	
	C.free(unsafe.Pointer(result))
}

//export esbuild_build
func esbuild_build(opts *C.esbuild_build_options) *C.esbuild_build_result {
	// Convert C options to Go BuildOptions
	buildOpts := api.BuildOptions{}
	
	// Basic logging options
	buildOpts.Color = api.StderrColor(opts.color)
	buildOpts.LogLevel = api.LogLevel(opts.log_level)
	buildOpts.LogLimit = int(opts.log_limit)
	
	// Source map options
	buildOpts.Sourcemap = api.SourceMap(opts.sourcemap)
	if opts.source_root != nil {
		buildOpts.SourceRoot = C.GoString(opts.source_root)
	}
	buildOpts.SourcesContent = api.SourcesContent(opts.sources_content)
	
	// Target and compatibility
	buildOpts.Target = api.Target(opts.target)
	buildOpts.Platform = api.Platform(opts.platform)
	buildOpts.Format = api.Format(opts.format)
	if opts.global_name != nil {
		buildOpts.GlobalName = C.GoString(opts.global_name)
	}
	
	// Build configuration
	buildOpts.Bundle = opts.bundle != 0
	buildOpts.PreserveSymlinks = opts.preserve_symlinks != 0
	buildOpts.Splitting = opts.splitting != 0
	if opts.outfile != nil {
		buildOpts.Outfile = C.GoString(opts.outfile)
	}
	if opts.outdir != nil {
		buildOpts.Outdir = C.GoString(opts.outdir)
	}
	if opts.outbase != nil {
		buildOpts.Outbase = C.GoString(opts.outbase)
	}
	buildOpts.Metafile = opts.metafile != 0
	buildOpts.Write = opts.write != 0
	buildOpts.AllowOverwrite = opts.allow_overwrite != 0
	
	// Entry points
	if opts.entry_points_count > 0 && opts.entry_points != nil {
		entryPointsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.entry_points))[:opts.entry_points_count:opts.entry_points_count]
		for _, ep := range entryPointsSlice {
			if ep != nil {
				buildOpts.EntryPoints = append(buildOpts.EntryPoints, C.GoString(ep))
			}
		}
	}
	
	// Stdin configuration
	if opts.stdin != nil {
		stdinOpts := &api.StdinOptions{
			Contents:   C.GoString(opts.stdin.contents),
			ResolveDir: C.GoString(opts.stdin.resolve_dir),
			Sourcefile: C.GoString(opts.stdin.sourcefile),
			Loader:     api.Loader(opts.stdin.loader),
		}
		buildOpts.Stdin = stdinOpts
	}
	
	// Minification and Property Mangling
	if opts.mangle_props != nil {
		buildOpts.MangleProps = C.GoString(opts.mangle_props)
	}
	if opts.reserve_props != nil {
		buildOpts.ReserveProps = C.GoString(opts.reserve_props)
	}
	buildOpts.MangleQuoted = api.MangleQuoted(opts.mangle_quoted)
	
	// Convert mangle cache
	if opts.mangle_cache_count > 0 && opts.mangle_cache_keys != nil && opts.mangle_cache_values != nil {
		buildOpts.MangleCache = make(map[string]interface{})
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.mangle_cache_keys))[:opts.mangle_cache_count:opts.mangle_cache_count]
		valuesSlice := (*[1000]*C.char)(unsafe.Pointer(opts.mangle_cache_values))[:opts.mangle_cache_count:opts.mangle_cache_count]
		for i := 0; i < int(opts.mangle_cache_count); i++ {
			if keysSlice[i] != nil && valuesSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := C.GoString(valuesSlice[i])
				buildOpts.MangleCache[key] = value
			}
		}
	}
	
	buildOpts.Drop = api.Drop(opts.drop)
	
	// Convert drop labels
	if opts.drop_labels_count > 0 && opts.drop_labels != nil {
		labelsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.drop_labels))[:opts.drop_labels_count:opts.drop_labels_count]
		for _, label := range labelsSlice {
			if label != nil {
				buildOpts.DropLabels = append(buildOpts.DropLabels, C.GoString(label))
			}
		}
	}
	
	buildOpts.MinifyWhitespace = opts.minify_whitespace != 0
	buildOpts.MinifyIdentifiers = opts.minify_identifiers != 0
	buildOpts.MinifySyntax = opts.minify_syntax != 0
	buildOpts.LineLimit = int(opts.line_limit)
	buildOpts.Charset = api.Charset(opts.charset)
	buildOpts.TreeShaking = api.TreeShaking(opts.tree_shaking)
	buildOpts.IgnoreAnnotations = opts.ignore_annotations != 0
	buildOpts.LegalComments = api.LegalComments(opts.legal_comments)
	
	// JSX Configuration
	buildOpts.JSX = api.JSX(opts.jsx)
	if opts.jsx_factory != nil {
		buildOpts.JSXFactory = C.GoString(opts.jsx_factory)
	}
	if opts.jsx_fragment != nil {
		buildOpts.JSXFragment = C.GoString(opts.jsx_fragment)
	}
	if opts.jsx_import_source != nil {
		buildOpts.JSXImportSource = C.GoString(opts.jsx_import_source)
	}
	buildOpts.JSXDev = opts.jsx_dev != 0
	buildOpts.JSXSideEffects = opts.jsx_side_effects != 0
	
	// TypeScript Configuration
	if opts.tsconfig != nil {
		buildOpts.Tsconfig = C.GoString(opts.tsconfig)
	}
	if opts.tsconfig_raw != nil {
		buildOpts.TsconfigRaw = C.GoString(opts.tsconfig_raw)
	}
	
	// Code Injection - Banner
	if opts.banner_count > 0 && opts.banner_keys != nil && opts.banner_values != nil {
		buildOpts.Banner = make(map[string]string)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.banner_keys))[:opts.banner_count:opts.banner_count]
		valuesSlice := (*[1000]*C.char)(unsafe.Pointer(opts.banner_values))[:opts.banner_count:opts.banner_count]
		for i := 0; i < int(opts.banner_count); i++ {
			if keysSlice[i] != nil && valuesSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := C.GoString(valuesSlice[i])
				buildOpts.Banner[key] = value
			}
		}
	}
	
	// Code Injection - Footer
	if opts.footer_count > 0 && opts.footer_keys != nil && opts.footer_values != nil {
		buildOpts.Footer = make(map[string]string)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.footer_keys))[:opts.footer_count:opts.footer_count]
		valuesSlice := (*[1000]*C.char)(unsafe.Pointer(opts.footer_values))[:opts.footer_count:opts.footer_count]
		for i := 0; i < int(opts.footer_count); i++ {
			if keysSlice[i] != nil && valuesSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := C.GoString(valuesSlice[i])
				buildOpts.Footer[key] = value
			}
		}
	}
	
	// Code Transformation - Define
	if opts.define_count > 0 && opts.define_keys != nil && opts.define_values != nil {
		buildOpts.Define = make(map[string]string)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.define_keys))[:opts.define_count:opts.define_count]
		valuesSlice := (*[1000]*C.char)(unsafe.Pointer(opts.define_values))[:opts.define_count:opts.define_count]
		for i := 0; i < int(opts.define_count); i++ {
			if keysSlice[i] != nil && valuesSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := C.GoString(valuesSlice[i])
				buildOpts.Define[key] = value
			}
		}
	}
	
	// Code Transformation - Pure
	if opts.pure_count > 0 && opts.pure != nil {
		pureSlice := (*[1000]*C.char)(unsafe.Pointer(opts.pure))[:opts.pure_count:opts.pure_count]
		for _, pure := range pureSlice {
			if pure != nil {
				buildOpts.Pure = append(buildOpts.Pure, C.GoString(pure))
			}
		}
	}
	
	buildOpts.KeepNames = opts.keep_names != 0
	
	// Additional Build Configuration
	if opts.abs_working_dir != nil {
		buildOpts.AbsWorkingDir = C.GoString(opts.abs_working_dir)
	}
	
	// Module Resolution - External
	if opts.external_count > 0 && opts.external != nil {
		externalSlice := (*[1000]*C.char)(unsafe.Pointer(opts.external))[:opts.external_count:opts.external_count]
		for _, ext := range externalSlice {
			if ext != nil {
				buildOpts.External = append(buildOpts.External, C.GoString(ext))
			}
		}
	}
	
	buildOpts.Packages = api.Packages(opts.packages)
	
	// Module Resolution - Alias
	if opts.alias_count > 0 && opts.alias_keys != nil && opts.alias_values != nil {
		buildOpts.Alias = make(map[string]string)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.alias_keys))[:opts.alias_count:opts.alias_count]
		valuesSlice := (*[1000]*C.char)(unsafe.Pointer(opts.alias_values))[:opts.alias_count:opts.alias_count]
		for i := 0; i < int(opts.alias_count); i++ {
			if keysSlice[i] != nil && valuesSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := C.GoString(valuesSlice[i])
				buildOpts.Alias[key] = value
			}
		}
	}
	
	// Module Resolution - MainFields
	if opts.main_fields_count > 0 && opts.main_fields != nil {
		fieldsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.main_fields))[:opts.main_fields_count:opts.main_fields_count]
		for _, field := range fieldsSlice {
			if field != nil {
				buildOpts.MainFields = append(buildOpts.MainFields, C.GoString(field))
			}
		}
	}
	
	// Module Resolution - Conditions
	if opts.conditions_count > 0 && opts.conditions != nil {
		conditionsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.conditions))[:opts.conditions_count:opts.conditions_count]
		for _, condition := range conditionsSlice {
			if condition != nil {
				buildOpts.Conditions = append(buildOpts.Conditions, C.GoString(condition))
			}
		}
	}
	
	// Module Resolution - Loader (by extension)
	if opts.loader_count > 0 && opts.loader_keys != nil && opts.loader_values != nil {
		buildOpts.Loader = make(map[string]api.Loader)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.loader_keys))[:opts.loader_count:opts.loader_count]
		valuesSlice := (*[1000]C.int)(unsafe.Pointer(opts.loader_values))[:opts.loader_count:opts.loader_count]
		for i := 0; i < int(opts.loader_count); i++ {
			if keysSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := api.Loader(valuesSlice[i])
				buildOpts.Loader[key] = value
			}
		}
	}
	
	// Module Resolution - ResolveExtensions
	if opts.resolve_extensions_count > 0 && opts.resolve_extensions != nil {
		extensionsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.resolve_extensions))[:opts.resolve_extensions_count:opts.resolve_extensions_count]
		for _, ext := range extensionsSlice {
			if ext != nil {
				buildOpts.ResolveExtensions = append(buildOpts.ResolveExtensions, C.GoString(ext))
			}
		}
	}
	
	// Module Resolution - OutExtension
	if opts.out_extension_count > 0 && opts.out_extension_keys != nil && opts.out_extension_values != nil {
		buildOpts.OutExtension = make(map[string]string)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.out_extension_keys))[:opts.out_extension_count:opts.out_extension_count]
		valuesSlice := (*[1000]*C.char)(unsafe.Pointer(opts.out_extension_values))[:opts.out_extension_count:opts.out_extension_count]
		for i := 0; i < int(opts.out_extension_count); i++ {
			if keysSlice[i] != nil && valuesSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := C.GoString(valuesSlice[i])
				buildOpts.OutExtension[key] = value
			}
		}
	}
	
	// Module Resolution - PublicPath
	if opts.public_path != nil {
		buildOpts.PublicPath = C.GoString(opts.public_path)
	}
	
	// Module Resolution - Inject
	if opts.inject_count > 0 && opts.inject != nil {
		injectSlice := (*[1000]*C.char)(unsafe.Pointer(opts.inject))[:opts.inject_count:opts.inject_count]
		for _, inject := range injectSlice {
			if inject != nil {
				buildOpts.Inject = append(buildOpts.Inject, C.GoString(inject))
			}
		}
	}
	
	// Module Resolution - NodePaths
	if opts.node_paths_count > 0 && opts.node_paths != nil {
		pathsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.node_paths))[:opts.node_paths_count:opts.node_paths_count]
		for _, path := range pathsSlice {
			if path != nil {
				buildOpts.NodePaths = append(buildOpts.NodePaths, C.GoString(path))
			}
		}
	}
	
	// Naming Templates
	if opts.entry_names != nil {
		buildOpts.EntryNames = C.GoString(opts.entry_names)
	}
	if opts.chunk_names != nil {
		buildOpts.ChunkNames = C.GoString(opts.chunk_names)
	}
	if opts.asset_names != nil {
		buildOpts.AssetNames = C.GoString(opts.asset_names)
	}
	
	// Advanced Entry Points
	if opts.entry_points_advanced_count > 0 && opts.entry_points_advanced != nil {
		entryPointsSlice := (*[1000]C.esbuild_entry_point)(unsafe.Pointer(opts.entry_points_advanced))[:opts.entry_points_advanced_count:opts.entry_points_advanced_count]
		for _, ep := range entryPointsSlice {
			advancedEP := api.EntryPoint{
				InputPath:  C.GoString(ep.input_path),
				OutputPath: C.GoString(ep.output_path),
			}
			buildOpts.EntryPointsAdvanced = append(buildOpts.EntryPointsAdvanced, advancedEP)
		}
	}
	
	// Log Override
	if opts.log_override_count > 0 && opts.log_override_keys != nil && opts.log_override_values != nil {
		buildOpts.LogOverride = make(map[string]api.LogLevel)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.log_override_keys))[:opts.log_override_count:opts.log_override_count]
		valuesSlice := (*[1000]C.int)(unsafe.Pointer(opts.log_override_values))[:opts.log_override_count:opts.log_override_count]
		for i := 0; i < int(opts.log_override_count); i++ {
			if keysSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := api.LogLevel(valuesSlice[i])
				buildOpts.LogOverride[key] = value
			}
		}
	}
	
	// Engine/Compatibility - Engines
	if opts.engines_count > 0 && opts.engine_names != nil && opts.engine_versions != nil {
		engineNamesSlice := (*[1000]C.int)(unsafe.Pointer(opts.engine_names))[:opts.engines_count:opts.engines_count]
		engineVersionsSlice := (*[1000]*C.char)(unsafe.Pointer(opts.engine_versions))[:opts.engines_count:opts.engines_count]
		for i := 0; i < int(opts.engines_count); i++ {
			if engineVersionsSlice[i] != nil {
				engine := api.Engine{
					Name:    api.EngineName(engineNamesSlice[i]),
					Version: C.GoString(engineVersionsSlice[i]),
				}
				buildOpts.Engines = append(buildOpts.Engines, engine)
			}
		}
	}
	
	// Engine/Compatibility - Supported
	if opts.supported_count > 0 && opts.supported_keys != nil && opts.supported_values != nil {
		buildOpts.Supported = make(map[string]bool)
		keysSlice := (*[1000]*C.char)(unsafe.Pointer(opts.supported_keys))[:opts.supported_count:opts.supported_count]
		valuesSlice := (*[1000]C.int)(unsafe.Pointer(opts.supported_values))[:opts.supported_count:opts.supported_count]
		for i := 0; i < int(opts.supported_count); i++ {
			if keysSlice[i] != nil {
				key := C.GoString(keysSlice[i])
				value := valuesSlice[i] != 0
				buildOpts.Supported[key] = value
			}
		}
	}
	
	// Plugin configuration
	if opts.plugins_count > 0 && opts.plugins != nil {
		pluginsSlice := (*[1000]C.c_plugin)(unsafe.Pointer(opts.plugins))[:opts.plugins_count:opts.plugins_count]
		
		for _, cPlugin := range pluginsSlice {
			// Create a Go plugin from the C plugin
			goPlugin := api.Plugin{
				Name: C.GoString(cPlugin.name),
				Setup: func(build api.PluginBuild) {
					// Handle resolve hooks
					if cPlugin.resolve_hooks_count > 0 && cPlugin.resolve_hooks != nil {
						resolveHooksSlice := (*[1000]C.c_plugin_resolve_hook)(unsafe.Pointer(cPlugin.resolve_hooks))[:cPlugin.resolve_hooks_count:cPlugin.resolve_hooks_count]
						
						for _, hook := range resolveHooksSlice {
							filter := C.GoString(hook.filter)
							var namespace string
							if hook.namespace != nil {
								namespace = C.GoString(hook.namespace)
							}
							
							build.OnResolve(api.OnResolveOptions{
								Filter:    filter,
								Namespace: namespace,
							}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
								// Convert Go args to C args
								cArgs := (*C.c_on_resolve_args)(C.malloc(C.sizeof_c_on_resolve_args))
								defer C.free(unsafe.Pointer(cArgs))
								
								cArgs.path = C.CString(args.Path)
								defer C.free(unsafe.Pointer(cArgs.path))
								cArgs.importer = C.CString(args.Importer)
								defer C.free(unsafe.Pointer(cArgs.importer))
								cArgs.namespace = C.CString(args.Namespace)
								defer C.free(unsafe.Pointer(cArgs.namespace))
								cArgs.resolve_dir = C.CString(args.ResolveDir)
								defer C.free(unsafe.Pointer(cArgs.resolve_dir))
								cArgs.kind = C.int(args.Kind)
								
								// Call the Swift callback
								result := C.swift_plugin_on_resolve_callback(cArgs, hook.callback_data)
								if result != nil {
									defer C.free(unsafe.Pointer(result))
									
									// Convert C result to Go result
									goResult := api.OnResolveResult{}
									if result.path != nil {
										goResult.Path = C.GoString(result.path)
									}
									if result.namespace != nil {
										goResult.Namespace = C.GoString(result.namespace)
									}
									if result.external >= 0 {
										goResult.External = result.external != 0
									}
									if result.side_effects >= 0 {
										if result.side_effects != 0 {
											goResult.SideEffects = api.SideEffectsTrue
										} else {
											goResult.SideEffects = api.SideEffectsFalse
										}
									}
									
									// Handle errors
									if result.errors_count > 0 && result.errors != nil {
										errorsSlice := (*[1000]C.c_message)(unsafe.Pointer(result.errors))[:result.errors_count:result.errors_count]
										for _, cError := range errorsSlice {
											goError := api.Message{
												Text: C.GoString(cError.text),
											}
											if cError.plugin_name != nil {
												goError.PluginName = C.GoString(cError.plugin_name)
											}
											goResult.Errors = append(goResult.Errors, goError)
										}
									}
									
									// Handle warnings
									if result.warnings_count > 0 && result.warnings != nil {
										warningsSlice := (*[1000]C.c_message)(unsafe.Pointer(result.warnings))[:result.warnings_count:result.warnings_count]
										for _, cWarning := range warningsSlice {
											goWarning := api.Message{
												Text: C.GoString(cWarning.text),
											}
											if cWarning.plugin_name != nil {
												goWarning.PluginName = C.GoString(cWarning.plugin_name)
											}
											goResult.Warnings = append(goResult.Warnings, goWarning)
										}
									}
									
									return goResult, nil
								}
								
								return api.OnResolveResult{}, nil
							})
						}
					}
					
					// Handle load hooks
					if cPlugin.load_hooks_count > 0 && cPlugin.load_hooks != nil {
						loadHooksSlice := (*[1000]C.c_plugin_load_hook)(unsafe.Pointer(cPlugin.load_hooks))[:cPlugin.load_hooks_count:cPlugin.load_hooks_count]
						
						for _, hook := range loadHooksSlice {
							filter := C.GoString(hook.filter)
							var namespace string
							if hook.namespace != nil {
								namespace = C.GoString(hook.namespace)
							}
							
							build.OnLoad(api.OnLoadOptions{
								Filter:    filter,
								Namespace: namespace,
							}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
								// Convert Go args to C args
								cArgs := (*C.c_on_load_args)(C.malloc(C.sizeof_c_on_load_args))
								defer C.free(unsafe.Pointer(cArgs))
								
								cArgs.path = C.CString(args.Path)
								defer C.free(unsafe.Pointer(cArgs.path))
								cArgs.namespace = C.CString(args.Namespace)
								defer C.free(unsafe.Pointer(cArgs.namespace))
								cArgs.suffix = C.CString(args.Suffix)
								defer C.free(unsafe.Pointer(cArgs.suffix))
								
								// Call the Swift callback
								result := C.swift_plugin_on_load_callback(cArgs, hook.callback_data)
								if result != nil {
									defer C.free(unsafe.Pointer(result))
									
									// Convert C result to Go result
									goResult := api.OnLoadResult{}
									if result.contents != nil {
										contents := C.GoStringN(result.contents, result.contents_length)
										goResult.Contents = &contents
									}
									if result.loader >= 0 {
										goResult.Loader = api.Loader(result.loader)
									}
									if result.resolve_dir != nil {
										goResult.ResolveDir = C.GoString(result.resolve_dir)
									}
									
									// Handle errors
									if result.errors_count > 0 && result.errors != nil {
										errorsSlice := (*[1000]C.c_message)(unsafe.Pointer(result.errors))[:result.errors_count:result.errors_count]
										for _, cError := range errorsSlice {
											goError := api.Message{
												Text: C.GoString(cError.text),
											}
											if cError.plugin_name != nil {
												goError.PluginName = C.GoString(cError.plugin_name)
											}
											goResult.Errors = append(goResult.Errors, goError)
										}
									}
									
									// Handle warnings
									if result.warnings_count > 0 && result.warnings != nil {
										warningsSlice := (*[1000]C.c_message)(unsafe.Pointer(result.warnings))[:result.warnings_count:result.warnings_count]
										for _, cWarning := range warningsSlice {
											goWarning := api.Message{
												Text: C.GoString(cWarning.text),
											}
											if cWarning.plugin_name != nil {
												goWarning.PluginName = C.GoString(cWarning.plugin_name)
											}
											goResult.Warnings = append(goResult.Warnings, goWarning)
										}
									}
									
									return goResult, nil
								}
								
								return api.OnLoadResult{}, nil
							})
						}
					}
					
					// Handle start/end callbacks
					if cPlugin.on_start != nil {
						build.OnStart(func() (api.OnStartResult, error) {
							C.swift_plugin_on_start_callback(cPlugin.start_data)
							return api.OnStartResult{}, nil
						})
					}
					
					if cPlugin.on_end != nil {
						build.OnEnd(func(result *api.BuildResult) (api.OnEndResult, error) {
							C.swift_plugin_on_end_callback(cPlugin.end_data)
							return api.OnEndResult{}, nil
						})
					}
				},
			}
			
			buildOpts.Plugins = append(buildOpts.Plugins, goPlugin)
		}
	}
	
	// Perform the build
	result := api.Build(buildOpts)
	
	// Convert result to C structure
	cResult := esbuild_create_build_result()
	
	// Convert errors
	if len(result.Errors) > 0 {
		cResult.errors_count = C.int(len(result.Errors))
		cResult.errors = (*C.c_message)(C.malloc(C.size_t(uintptr(len(result.Errors)) * unsafe.Sizeof(C.c_message{}))))
		errorsSlice := (*[1000]C.c_message)(unsafe.Pointer(cResult.errors))[:len(result.Errors):len(result.Errors)]
		
		for i, err := range result.Errors {
			errorsSlice[i].id = C.CString(err.ID)
			errorsSlice[i].plugin_name = C.CString(err.PluginName)
			errorsSlice[i].text = C.CString(err.Text)
			errorsSlice[i].location = nil
			errorsSlice[i].notes = nil
			errorsSlice[i].notes_count = 0
			
			if err.Location != nil {
				loc := esbuild_create_location()
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
	
	// Convert warnings (similar to errors)
	if len(result.Warnings) > 0 {
		cResult.warnings_count = C.int(len(result.Warnings))
		cResult.warnings = (*C.c_message)(C.malloc(C.size_t(uintptr(len(result.Warnings)) * unsafe.Sizeof(C.c_message{}))))
		warningsSlice := (*[1000]C.c_message)(unsafe.Pointer(cResult.warnings))[:len(result.Warnings):len(result.Warnings)]
		
		for i, warn := range result.Warnings {
			warningsSlice[i].id = C.CString(warn.ID)
			warningsSlice[i].plugin_name = C.CString(warn.PluginName)
			warningsSlice[i].text = C.CString(warn.Text)
			warningsSlice[i].location = nil
			warningsSlice[i].notes = nil
			warningsSlice[i].notes_count = 0
			
			if warn.Location != nil {
				loc := esbuild_create_location()
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
	
	// Convert output files
	if len(result.OutputFiles) > 0 {
		cResult.output_files_count = C.int(len(result.OutputFiles))
		cResult.output_files = (*C.esbuild_output_file)(C.malloc(C.size_t(uintptr(len(result.OutputFiles)) * unsafe.Sizeof(C.esbuild_output_file{}))))
		filesSlice := (*[1000]C.esbuild_output_file)(unsafe.Pointer(cResult.output_files))[:len(result.OutputFiles):len(result.OutputFiles)]
		
		for i, file := range result.OutputFiles {
			filesSlice[i].path = C.CString(file.Path)
			filesSlice[i].contents = C.CString(string(file.Contents))
			filesSlice[i].contents_length = C.int(len(file.Contents))
			filesSlice[i].hash = C.CString(file.Hash)
		}
	} else {
		cResult.output_files = nil
		cResult.output_files_count = 0
	}
	
	// Convert metafile
	if result.Metafile != "" {
		cResult.metafile = C.CString(result.Metafile)
	} else {
		cResult.metafile = nil
	}
	
	// Convert mangle cache (simplified)
	cResult.mangle_cache_keys = nil
	cResult.mangle_cache_values = nil
	cResult.mangle_cache_count = 0
	
	return cResult
}

// Plugin callback bridge functions

// Note: swift_plugin_on_* functions are implemented in Swift
// and linked externally. No Go implementations needed.

// Plugin registry to store registered plugins
var pluginRegistry = make(map[unsafe.Pointer]*C.c_plugin)

//export register_plugin
func register_plugin(plugin *C.c_plugin) unsafe.Pointer {
	// Generate a unique ID for this plugin
	pluginID := unsafe.Pointer(plugin)
	pluginRegistry[pluginID] = plugin
	return pluginID
}

//export unregister_plugin
func unregister_plugin(pluginID unsafe.Pointer) {
	delete(pluginRegistry, pluginID)
}

// Main function is already defined in c_bridge.go
