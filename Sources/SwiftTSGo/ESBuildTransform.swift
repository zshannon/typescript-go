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

extension ESBuildTransformOptions {
    
    /// Creates transform options optimized for minification
    public static func minified(
        target: ESBuildTarget = .es2015,
        format: ESBuildFormat = .esmodule
    ) -> ESBuildTransformOptions {
        return ESBuildTransformOptions(
            target: target,
            format: format,
            minifyWhitespace: true,
            minifyIdentifiers: true,
            minifySyntax: true,
            treeShaking: .true
        )
    }
    
    /// Creates transform options for TypeScript compilation
    public static func typescript(
        target: ESBuildTarget = .es2020,
        jsx: ESBuildJSX = .transform
    ) -> ESBuildTransformOptions {
        return ESBuildTransformOptions(
            target: target,
            jsx: jsx,
            loader: .ts
        )
    }
    
    /// Creates transform options for JSX transformation
    public static func jsxTransform(
        jsx: ESBuildJSX = .transform,
        jsxFactory: String? = nil,
        jsxFragment: String? = nil
    ) -> ESBuildTransformOptions {
        return ESBuildTransformOptions(
            jsx: jsx,
            jsxFactory: jsxFactory,
            jsxFragment: jsxFragment,
            loader: .jsx
        )
    }
}