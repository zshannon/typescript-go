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
