import Foundation
import TSCBridge

/// ESBuild transformation options
public struct ESBuildTransformOptions {
    // MARK: - Logging and Output Control

    /// Controls colored output in terminal
    public var color: ESBuildColor

    /// Sets the verbosity level for logging
    public var logLevel: ESBuildLogLevel

    /// Maximum number of log messages to show
    public var logLimit: Int32

    /// Override log level for specific message types
    public var logOverride: [String: ESBuildLogLevel]

    // MARK: - Source Map

    /// Controls source map generation
    public var sourcemap: ESBuildSourceMap

    /// Sets the source root in generated source maps
    public var sourceRoot: String?

    /// Controls whether to include source content in source maps
    public var sourcesContent: ESBuildSourcesContent

    // MARK: - Target and Compatibility

    /// Sets the target ECMAScript version
    public var target: ESBuildTarget

    /// Specifies target engines with versions
    public var engines: [(engine: ESBuildEngine, version: String)]

    /// Override feature support detection
    public var supported: [String: Bool]

    // MARK: - Platform and Format

    /// Sets target platform
    public var platform: ESBuildPlatform

    /// Sets output format
    public var format: ESBuildFormat

    /// Global name for IIFE format
    public var globalName: String?

    // MARK: - Minification and Property Mangling

    /// Regex pattern for properties to mangle
    public var mangleProps: String?

    /// Regex pattern for properties NOT to mangle
    public var reserveProps: String?

    /// Whether to mangle quoted properties
    public var mangleQuoted: ESBuildMangleQuoted

    /// Cache for consistent property mangling
    public var mangleCache: [String: String]

    /// Drop specific constructs (console, debugger)
    public var drop: Set<ESBuildDrop>

    /// Array of labels to drop
    public var dropLabels: [String]

    /// Remove unnecessary whitespace
    public var minifyWhitespace: Bool

    /// Shorten variable names
    public var minifyIdentifiers: Bool

    /// Use shorter syntax where possible
    public var minifySyntax: Bool

    /// Maximum characters per line when minifying
    public var lineLimit: Int32

    /// Controls output character encoding
    public var charset: ESBuildCharset

    /// Controls dead code elimination
    public var treeShaking: ESBuildTreeShaking

    /// Ignore side-effect annotations
    public var ignoreAnnotations: Bool

    /// How to handle legal comments
    public var legalComments: ESBuildLegalComments

    // MARK: - JSX Configuration

    /// JSX transformation mode
    public var jsx: ESBuildJSX

    /// Function to call for JSX elements
    public var jsxFactory: String?

    /// Function to call for JSX fragments
    public var jsxFragment: String?

    /// Module to import JSX functions from (automatic mode)
    public var jsxImportSource: String?

    /// Enable JSX dev mode
    public var jsxDev: Bool

    /// Whether JSX has side effects
    public var jsxSideEffects: Bool

    // MARK: - TypeScript Configuration

    /// Raw TypeScript config as JSON string
    public var tsconfigRaw: String?

    // MARK: - Code Injection

    /// Code to prepend to output
    public var banner: String?

    /// Code to append to output
    public var footer: String?

    // MARK: - Code Transformation

    /// Replace identifiers with constant expressions
    public var define: [String: String]

    /// Mark functions as having no side effects
    public var pure: [String]

    /// Preserve function and class names
    public var keepNames: Bool

    // MARK: - Input Configuration

    /// Virtual filename for input (used in error messages)
    public var sourcefile: String?

    /// How to interpret the input
    public var loader: ESBuildLoader

    // MARK: - C Bridge

    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<c_transform_options> {
        let options = esbuild_create_transform_options()!
        
        // Logging and Output Control
        options.pointee.color = color.cValue
        options.pointee.log_level = logLevel.cValue
        options.pointee.log_limit = logLimit
        
        // Source Map
        options.pointee.sourcemap = sourcemap.cValue
        if let sourceRoot = sourceRoot {
            options.pointee.source_root = strdup(sourceRoot)
        }
        options.pointee.sources_content = sourcesContent.cValue
        
        // Target and Compatibility
        options.pointee.target = target.cValue
        options.pointee.platform = platform.cValue
        options.pointee.format = format.cValue
        if let globalName = globalName {
            options.pointee.global_name = strdup(globalName)
        }
        
        // Minification and Property Mangling
        if let mangleProps = mangleProps {
            options.pointee.mangle_props = strdup(mangleProps)
        }
        if let reserveProps = reserveProps {
            options.pointee.reserve_props = strdup(reserveProps)
        }
        options.pointee.mangle_quoted = mangleQuoted.cValue
        
        // Boolean flags
        options.pointee.minify_whitespace = minifyWhitespace ? 1 : 0
        options.pointee.minify_identifiers = minifyIdentifiers ? 1 : 0
        options.pointee.minify_syntax = minifySyntax ? 1 : 0
        options.pointee.line_limit = lineLimit
        options.pointee.charset = charset.cValue
        options.pointee.tree_shaking = treeShaking.cValue
        options.pointee.ignore_annotations = ignoreAnnotations ? 1 : 0
        options.pointee.legal_comments = legalComments.cValue
        
        // JSX Configuration
        options.pointee.jsx = jsx.cValue
        if let jsxFactory = jsxFactory {
            options.pointee.jsx_factory = strdup(jsxFactory)
        }
        if let jsxFragment = jsxFragment {
            options.pointee.jsx_fragment = strdup(jsxFragment)
        }
        if let jsxImportSource = jsxImportSource {
            options.pointee.jsx_import_source = strdup(jsxImportSource)
        }
        options.pointee.jsx_dev = jsxDev ? 1 : 0
        options.pointee.jsx_side_effects = jsxSideEffects ? 1 : 0
        
        // TypeScript Configuration
        if let tsconfigRaw = tsconfigRaw {
            options.pointee.tsconfig_raw = strdup(tsconfigRaw)
        }
        
        // Code Injection
        if let banner = banner {
            options.pointee.banner = strdup(banner)
        }
        if let footer = footer {
            options.pointee.footer = strdup(footer)
        }
        
        // Code Transformation
        options.pointee.keep_names = keepNames ? 1 : 0
        
        // Input Configuration
        if let sourcefile = sourcefile {
            options.pointee.sourcefile = strdup(sourcefile)
        }
        options.pointee.loader = loader.cValue
        
        return options
    }

    // MARK: - Initialization

    /// Creates transform options with default values
    public init(
        color: ESBuildColor = .ifTerminal,
        logLevel: ESBuildLogLevel = .info,
        logLimit: Int32 = 0,
        logOverride: [String: ESBuildLogLevel] = [:],
        sourcemap: ESBuildSourceMap = .none,
        sourceRoot: String? = nil,
        sourcesContent: ESBuildSourcesContent = .include,
        target: ESBuildTarget = .default,
        engines: [(engine: ESBuildEngine, version: String)] = [],
        supported: [String: Bool] = [:],
        platform: ESBuildPlatform = .default,
        format: ESBuildFormat = .default,
        globalName: String? = nil,
        mangleProps: String? = nil,
        reserveProps: String? = nil,
        mangleQuoted: ESBuildMangleQuoted = .false,
        mangleCache: [String: String] = [:],
        drop: Set<ESBuildDrop> = [],
        dropLabels: [String] = [],
        minifyWhitespace: Bool = false,
        minifyIdentifiers: Bool = false,
        minifySyntax: Bool = false,
        lineLimit: Int32 = 0,
        charset: ESBuildCharset = .default,
        treeShaking: ESBuildTreeShaking = .default,
        ignoreAnnotations: Bool = false,
        legalComments: ESBuildLegalComments = .default,
        jsx: ESBuildJSX = .transform,
        jsxFactory: String? = nil,
        jsxFragment: String? = nil,
        jsxImportSource: String? = nil,
        jsxDev: Bool = false,
        jsxSideEffects: Bool = false,
        tsconfigRaw: String? = nil,
        banner: String? = nil,
        footer: String? = nil,
        define: [String: String] = [:],
        pure: [String] = [],
        keepNames: Bool = false,
        sourcefile: String? = nil,
        loader: ESBuildLoader = .default
    ) {
        self.color = color
        self.logLevel = logLevel
        self.logLimit = logLimit
        self.logOverride = logOverride
        self.sourcemap = sourcemap
        self.sourceRoot = sourceRoot
        self.sourcesContent = sourcesContent
        self.target = target
        self.engines = engines
        self.supported = supported
        self.platform = platform
        self.format = format
        self.globalName = globalName
        self.mangleProps = mangleProps
        self.reserveProps = reserveProps
        self.mangleQuoted = mangleQuoted
        self.mangleCache = mangleCache
        self.drop = drop
        self.dropLabels = dropLabels
        self.minifyWhitespace = minifyWhitespace
        self.minifyIdentifiers = minifyIdentifiers
        self.minifySyntax = minifySyntax
        self.lineLimit = lineLimit
        self.charset = charset
        self.treeShaking = treeShaking
        self.ignoreAnnotations = ignoreAnnotations
        self.legalComments = legalComments
        self.jsx = jsx
        self.jsxFactory = jsxFactory
        self.jsxFragment = jsxFragment
        self.jsxImportSource = jsxImportSource
        self.jsxDev = jsxDev
        self.jsxSideEffects = jsxSideEffects
        self.tsconfigRaw = tsconfigRaw
        self.banner = banner
        self.footer = footer
        self.define = define
        self.pure = pure
        self.keepNames = keepNames
        self.sourcefile = sourcefile
        self.loader = loader
    }
}

// MARK: - Convenience Initializers

public extension ESBuildTransformOptions {
    /// Creates transform options optimized for minification
    static func minified(
        target: ESBuildTarget = .es2015,
        format: ESBuildFormat = .esmodule
    ) -> ESBuildTransformOptions {
        ESBuildTransformOptions(
            target: target,
            format: format,
            minifyWhitespace: true,
            minifyIdentifiers: true,
            minifySyntax: true,
            treeShaking: .true
        )
    }

    /// Creates transform options for TypeScript compilation
    static func typescript(
        target: ESBuildTarget = .es2020,
        jsx: ESBuildJSX = .transform
    ) -> ESBuildTransformOptions {
        ESBuildTransformOptions(
            target: target,
            jsx: jsx,
            loader: .ts
        )
    }

    /// Creates transform options for JSX transformation
    static func jsxTransform(
        jsx: ESBuildJSX = .transform,
        jsxFactory: String? = nil,
        jsxFragment: String? = nil
    ) -> ESBuildTransformOptions {
        ESBuildTransformOptions(
            jsx: jsx,
            jsxFactory: jsxFactory,
            jsxFragment: jsxFragment,
            loader: .jsx
        )
    }
}

// MARK: - Transform Result Types

/// Location information for ESBuild messages
public struct ESBuildLocation: Sendable {
    /// File path where the message occurred
    public let file: String
    
    /// Namespace for the file
    public let namespace: String
    
    /// Line number (1-based)
    public let line: Int
    
    /// Column number (0-based, in bytes)
    public let column: Int
    
    /// Length of the relevant text (in bytes)
    public let length: Int
    
    /// The text of the line containing the error
    public let lineText: String
    
    /// Suggested replacement text
    public let suggestion: String
    
    public init(
        file: String,
        namespace: String,
        line: Int,
        column: Int,
        length: Int,
        lineText: String,
        suggestion: String
    ) {
        self.file = file
        self.namespace = namespace
        self.line = line
        self.column = column
        self.length = length
        self.lineText = lineText
        self.suggestion = suggestion
    }
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<c_location> {
        let location = esbuild_create_location()!
        
        location.pointee.file = strdup(file)
        location.pointee.namespace = strdup(namespace)
        location.pointee.line = Int32(line)
        location.pointee.column = Int32(column)
        location.pointee.length = Int32(length)
        location.pointee.line_text = strdup(lineText)
        location.pointee.suggestion = strdup(suggestion)
        
        return location
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<c_location>) -> ESBuildLocation {
        return ESBuildLocation(
            file: String(cString: cValue.pointee.file),
            namespace: String(cString: cValue.pointee.namespace),
            line: Int(cValue.pointee.line),
            column: Int(cValue.pointee.column),
            length: Int(cValue.pointee.length),
            lineText: String(cString: cValue.pointee.line_text),
            suggestion: String(cString: cValue.pointee.suggestion)
        )
    }
}

/// Note with additional information for ESBuild messages
public struct ESBuildNote: Sendable {
    /// Note text
    public let text: String
    
    /// Optional location where the note applies
    public let location: ESBuildLocation?
    
    public init(text: String, location: ESBuildLocation? = nil) {
        self.text = text
        self.location = location
    }
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<c_note> {
        let note = esbuild_create_note()!
        
        note.pointee.text = strdup(text)
        if let location = location {
            note.pointee.location = location.cValue
        } else {
            note.pointee.location = nil
        }
        
        return note
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<c_note>) -> ESBuildNote {
        let location: ESBuildLocation?
        if let locPtr = cValue.pointee.location {
            location = ESBuildLocation.from(cValue: locPtr)
        } else {
            location = nil
        }
        
        return ESBuildNote(
            text: String(cString: cValue.pointee.text),
            location: location
        )
    }
}

/// ESBuild diagnostic message (error or warning)
public struct ESBuildMessage: Sendable {
    /// Message identifier
    public let id: String
    
    /// Name of the plugin that generated this message
    public let pluginName: String
    
    /// Message text
    public let text: String
    
    /// Optional location where the message occurred
    public let location: ESBuildLocation?
    
    /// Additional notes with more information
    public let notes: [ESBuildNote]
    
    public init(
        id: String,
        pluginName: String,
        text: String,
        location: ESBuildLocation? = nil,
        notes: [ESBuildNote] = []
    ) {
        self.id = id
        self.pluginName = pluginName
        self.text = text
        self.location = location
        self.notes = notes
    }
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<c_message> {
        let message = esbuild_create_message()!
        
        message.pointee.id = strdup(id)
        message.pointee.plugin_name = strdup(pluginName)
        message.pointee.text = strdup(text)
        message.pointee.location = nil
        message.pointee.notes = nil
        message.pointee.notes_count = 0
        
        // Only add location if it exists
        if let location = location {
            message.pointee.location = location.cValue
        }
        
        // Simplified notes handling - no nested allocation for now
        // Notes will be handled by the Go side implementation
        
        return message
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<c_message>) -> ESBuildMessage {
        let location: ESBuildLocation?
        if let locPtr = cValue.pointee.location {
            location = ESBuildLocation.from(cValue: locPtr)
        } else {
            location = nil
        }
        
        var notes: [ESBuildNote] = []
        if let notesPtr = cValue.pointee.notes, cValue.pointee.notes_count > 0 {
            for i in 0..<Int(cValue.pointee.notes_count) {
                let notePtr = notesPtr.advanced(by: i)
                notes.append(ESBuildNote.from(cValue: notePtr))
            }
        }
        
        return ESBuildMessage(
            id: String(cString: cValue.pointee.id),
            pluginName: String(cString: cValue.pointee.plugin_name),
            text: String(cString: cValue.pointee.text),
            location: location,
            notes: notes
        )
    }
}

/// Result of an ESBuild transform operation
public struct ESBuildTransformResult: Sendable {
    /// Error messages from the transform
    public let errors: [ESBuildMessage]
    
    /// Warning messages from the transform
    public let warnings: [ESBuildMessage]
    
    /// Transformed code output
    public let code: Data
    
    /// Source map data (if generated)
    public let map: Data?
    
    /// Legal comments extracted from the code
    public let legalComments: Data?
    
    /// Mangle cache for consistent property renaming
    public let mangleCache: [String: String]
    
    public init(
        errors: [ESBuildMessage] = [],
        warnings: [ESBuildMessage] = [],
        code: Data,
        map: Data? = nil,
        legalComments: Data? = nil,
        mangleCache: [String: String] = [:]
    ) {
        self.errors = errors
        self.warnings = warnings
        self.code = code
        self.map = map
        self.legalComments = legalComments
        self.mangleCache = mangleCache
    }
    
    /// Convert to C bridge representation  
    public var cValue: UnsafeMutablePointer<c_transform_result> {
        let result = esbuild_create_transform_result()!
        
        // Simplified approach - let Go side handle complex nested structures
        result.pointee.errors = nil
        result.pointee.errors_count = 0
        result.pointee.warnings = nil
        result.pointee.warnings_count = 0
        
        // Convert code
        let codeString = String(data: code, encoding: .utf8) ?? ""
        result.pointee.code = strdup(codeString)
        result.pointee.code_length = Int32(code.count)
        
        // Convert map
        if let map = map {
            let mapString = String(data: map, encoding: .utf8) ?? ""
            result.pointee.source_map = strdup(mapString)
            result.pointee.source_map_length = Int32(map.count)
        } else {
            result.pointee.source_map = nil
            result.pointee.source_map_length = 0
        }
        
        // Convert legal comments
        if let legalComments = legalComments {
            let commentsString = String(data: legalComments, encoding: .utf8) ?? ""
            result.pointee.legal_comments = strdup(commentsString)
            result.pointee.legal_comments_length = Int32(legalComments.count)
        } else {
            result.pointee.legal_comments = nil
            result.pointee.legal_comments_length = 0
        }
        
        // Convert mangle cache
        if !mangleCache.isEmpty {
            result.pointee.mangle_cache_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: mangleCache.count)
            result.pointee.mangle_cache_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: mangleCache.count)
            result.pointee.mangle_cache_count = Int32(mangleCache.count)
            
            for (index, (key, value)) in mangleCache.enumerated() {
                result.pointee.mangle_cache_keys[index] = strdup(key)
                result.pointee.mangle_cache_values[index] = strdup(value)
            }
        } else {
            result.pointee.mangle_cache_keys = nil
            result.pointee.mangle_cache_values = nil
            result.pointee.mangle_cache_count = 0
        }
        
        return result
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<c_transform_result>) -> ESBuildTransformResult {
        var errors: [ESBuildMessage] = []
        if let errorsPtr = cValue.pointee.errors, cValue.pointee.errors_count > 0 {
            for i in 0..<Int(cValue.pointee.errors_count) {
                let errorPtr = errorsPtr.advanced(by: i)
                errors.append(ESBuildMessage.from(cValue: errorPtr))
            }
        }
        
        var warnings: [ESBuildMessage] = []
        if let warningsPtr = cValue.pointee.warnings, cValue.pointee.warnings_count > 0 {
            for i in 0..<Int(cValue.pointee.warnings_count) {
                let warningPtr = warningsPtr.advanced(by: i)
                warnings.append(ESBuildMessage.from(cValue: warningPtr))
            }
        }
        
        let code: Data
        if let codePtr = cValue.pointee.code {
            code = Data(String(cString: codePtr).utf8)
        } else {
            code = Data()
        }
        
        let map: Data?
        if let mapPtr = cValue.pointee.source_map, cValue.pointee.source_map_length > 0 {
            map = Data(String(cString: mapPtr).utf8)
        } else {
            map = nil
        }
        
        let legalComments: Data?
        if let commentsPtr = cValue.pointee.legal_comments, cValue.pointee.legal_comments_length > 0 {
            legalComments = Data(String(cString: commentsPtr).utf8)
        } else {
            legalComments = nil
        }
        
        var mangleCache: [String: String] = [:]
        if let keysPtr = cValue.pointee.mangle_cache_keys,
           let valuesPtr = cValue.pointee.mangle_cache_values,
           cValue.pointee.mangle_cache_count > 0 {
            let keysBuffer = UnsafeBufferPointer(start: keysPtr, count: Int(cValue.pointee.mangle_cache_count))
            let valuesBuffer = UnsafeBufferPointer(start: valuesPtr, count: Int(cValue.pointee.mangle_cache_count))
            
            for (keyPtr, valuePtr) in zip(keysBuffer, valuesBuffer) {
                if let keyPtr = keyPtr, let valuePtr = valuePtr {
                    let key = String(cString: keyPtr)
                    let value = String(cString: valuePtr)
                    mangleCache[key] = value
                }
            }
        }
        
        return ESBuildTransformResult(
            errors: errors,
            warnings: warnings,
            code: code,
            map: map,
            legalComments: legalComments,
            mangleCache: mangleCache
        )
    }
}

// MARK: - Transform Function

/// Transform source code using ESBuild
/// - Parameters:
///   - code: Source code to transform
///   - options: Transform options
/// - Returns: Transform result containing transformed code and metadata
public func esbuildTransform(code: String, options: ESBuildTransformOptions = ESBuildTransformOptions()) -> ESBuildTransformResult? {
    let cOptions = options.cValue
    defer { esbuild_free_transform_options(cOptions) }
    
    let cResult = code.withCString { cCode in
        return esbuild_transform(UnsafeMutablePointer(mutating: cCode), cOptions)
    }
    
    defer { 
        if let result = cResult {
            esbuild_free_transform_result(result)
        }
    }
    
    guard let result = cResult else {
        return nil
    }
    
    return ESBuildTransformResult.from(cValue: result)
}
