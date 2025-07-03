import Foundation
import TSCBridge

/// ESBuild build options for bundling projects
public struct ESBuildBuildOptions {
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
    
    /// Path to TypeScript config file
    public var tsconfig: String?
    
    /// Raw TypeScript config as JSON string
    public var tsconfigRaw: String?
    
    // MARK: - Code Injection
    
    /// Code to prepend to output by file type
    public var banner: [String: String]
    
    /// Code to append to output by file type
    public var footer: [String: String]
    
    // MARK: - Code Transformation
    
    /// Replace identifiers with constant expressions
    public var define: [String: String]
    
    /// Mark functions as having no side effects
    public var pure: [String]
    
    /// Preserve function and class names
    public var keepNames: Bool
    
    // MARK: - Build Configuration
    
    /// Enable bundling
    public var bundle: Bool
    
    /// Preserve symbolic links
    public var preserveSymlinks: Bool
    
    /// Enable code splitting
    public var splitting: Bool
    
    /// Single output file path
    public var outfile: String?
    
    /// Output directory
    public var outdir: String?
    
    /// Base directory for output
    public var outbase: String?
    
    /// Absolute working directory
    public var absWorkingDir: String?
    
    /// Generate metafile
    public var metafile: Bool
    
    /// Write files to disk
    public var write: Bool
    
    /// Allow overwriting files
    public var allowOverwrite: Bool
    
    // MARK: - Module Resolution
    
    /// External packages
    public var external: [String]
    
    /// Package handling strategy
    public var packages: ESBuildPackages
    
    /// Path aliases
    public var alias: [String: String]
    
    /// Package.json main fields
    public var mainFields: [String]
    
    /// Export conditions
    public var conditions: [String]
    
    /// File loaders by extension
    public var loader: [String: ESBuildLoader]
    
    /// Extensions to resolve
    public var resolveExtensions: [String]
    
    /// Output file extensions
    public var outExtension: [String: String]
    
    /// Public path for assets
    public var publicPath: String?
    
    /// Files to inject
    public var inject: [String]
    
    /// Node module paths
    public var nodePaths: [String]
    
    // MARK: - Naming Templates
    
    /// Entry point naming template
    public var entryNames: String?
    
    /// Code chunk naming template
    public var chunkNames: String?
    
    /// Asset naming template
    public var assetNames: String?
    
    // MARK: - Input Configuration
    
    /// Entry point files
    public var entryPoints: [String]
    
    /// Advanced entry points
    public var entryPointsAdvanced: [ESBuildEntryPoint]
    
    /// Stdin input configuration
    public var stdin: ESBuildStdinOptions?
    
    // MARK: - C Bridge
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<esbuild_build_options> {
        let options = esbuild_create_build_options()!
        
        // Logging and Output Control
        options.pointee.color = color.cValue
        options.pointee.log_level = logLevel.cValue
        options.pointee.log_limit = logLimit
        
        // Log Override
        if !logOverride.isEmpty {
            options.pointee.log_override_count = Int32(logOverride.count)
            options.pointee.log_override_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: logOverride.count)
            options.pointee.log_override_values = UnsafeMutablePointer<Int32>.allocate(capacity: logOverride.count)
            
            for (index, (key, value)) in logOverride.enumerated() {
                options.pointee.log_override_keys[index] = strdup(key)
                options.pointee.log_override_values[index] = value.cValue
            }
        }
        
        // Source Map
        options.pointee.sourcemap = sourcemap.cValue
        if let sourceRoot = sourceRoot {
            options.pointee.source_root = strdup(sourceRoot)
        }
        options.pointee.sources_content = sourcesContent.cValue
        
        // Target and Compatibility
        options.pointee.target = target.cValue
        
        // Engines
        if !engines.isEmpty {
            options.pointee.engines_count = Int32(engines.count)
            options.pointee.engine_names = UnsafeMutablePointer<Int32>.allocate(capacity: engines.count)
            options.pointee.engine_versions = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: engines.count)
            
            for (index, engineInfo) in engines.enumerated() {
                options.pointee.engine_names[index] = engineInfo.engine.cValue
                options.pointee.engine_versions[index] = strdup(engineInfo.version)
            }
        }
        
        // Supported features
        if !supported.isEmpty {
            options.pointee.supported_count = Int32(supported.count)
            options.pointee.supported_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: supported.count)
            options.pointee.supported_values = UnsafeMutablePointer<Int32>.allocate(capacity: supported.count)
            
            for (index, (key, value)) in supported.enumerated() {
                options.pointee.supported_keys[index] = strdup(key)
                options.pointee.supported_values[index] = value ? 1 : 0
            }
        }
        
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
        
        // Mangle cache
        if !mangleCache.isEmpty {
            options.pointee.mangle_cache_count = Int32(mangleCache.count)
            options.pointee.mangle_cache_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: mangleCache.count)
            options.pointee.mangle_cache_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: mangleCache.count)
            
            for (index, (key, value)) in mangleCache.enumerated() {
                options.pointee.mangle_cache_keys[index] = strdup(key)
                options.pointee.mangle_cache_values[index] = strdup(value)
            }
        }
        
        // Drop constructs (bitfield)
        var dropValue: Int32 = 0
        for dropOption in drop {
            dropValue |= dropOption.cValue
        }
        options.pointee.drop = dropValue
        
        // Drop labels
        if !dropLabels.isEmpty {
            options.pointee.drop_labels_count = Int32(dropLabels.count)
            options.pointee.drop_labels = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: dropLabels.count)
            
            for (index, label) in dropLabels.enumerated() {
                options.pointee.drop_labels[index] = strdup(label)
            }
        }
        
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
        if let tsconfig = tsconfig {
            options.pointee.tsconfig = strdup(tsconfig)
        }
        if let tsconfigRaw = tsconfigRaw {
            options.pointee.tsconfig_raw = strdup(tsconfigRaw)
        }
        
        // Code Injection - Banner
        if !banner.isEmpty {
            options.pointee.banner_count = Int32(banner.count)
            options.pointee.banner_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: banner.count)
            options.pointee.banner_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: banner.count)
            
            for (index, (key, value)) in banner.enumerated() {
                options.pointee.banner_keys[index] = strdup(key)
                options.pointee.banner_values[index] = strdup(value)
            }
        }
        
        // Code Injection - Footer
        if !footer.isEmpty {
            options.pointee.footer_count = Int32(footer.count)
            options.pointee.footer_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: footer.count)
            options.pointee.footer_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: footer.count)
            
            for (index, (key, value)) in footer.enumerated() {
                options.pointee.footer_keys[index] = strdup(key)
                options.pointee.footer_values[index] = strdup(value)
            }
        }
        
        // Code Transformation - Define
        if !define.isEmpty {
            options.pointee.define_count = Int32(define.count)
            options.pointee.define_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: define.count)
            options.pointee.define_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: define.count)
            
            for (index, (key, value)) in define.enumerated() {
                options.pointee.define_keys[index] = strdup(key)
                options.pointee.define_values[index] = strdup(value)
            }
        }
        
        // Pure functions
        if !pure.isEmpty {
            options.pointee.pure_count = Int32(pure.count)
            options.pointee.pure = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: pure.count)
            
            for (index, function) in pure.enumerated() {
                options.pointee.pure[index] = strdup(function)
            }
        }
        
        options.pointee.keep_names = keepNames ? 1 : 0
        
        // Build Configuration
        options.pointee.bundle = bundle ? 1 : 0
        options.pointee.preserve_symlinks = preserveSymlinks ? 1 : 0
        options.pointee.splitting = splitting ? 1 : 0
        if let outfile = outfile {
            options.pointee.outfile = strdup(outfile)
        }
        if let outdir = outdir {
            options.pointee.outdir = strdup(outdir)
        }
        if let outbase = outbase {
            options.pointee.outbase = strdup(outbase)
        }
        if let absWorkingDir = absWorkingDir {
            options.pointee.abs_working_dir = strdup(absWorkingDir)
        }
        options.pointee.metafile = metafile ? 1 : 0
        options.pointee.write = write ? 1 : 0
        options.pointee.allow_overwrite = allowOverwrite ? 1 : 0
        
        // Entry points
        if !entryPoints.isEmpty {
            options.pointee.entry_points_count = Int32(entryPoints.count)
            options.pointee.entry_points = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: entryPoints.count)
            for (index, entryPoint) in entryPoints.enumerated() {
                options.pointee.entry_points[index] = strdup(entryPoint)
            }
        }
        
        // Module Resolution
        // External packages
        if !external.isEmpty {
            options.pointee.external_count = Int32(external.count)
            options.pointee.external = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: external.count)
            
            for (index, ext) in external.enumerated() {
                options.pointee.external[index] = strdup(ext)
            }
        }
        
        options.pointee.packages = packages.cValue
        
        // Alias mappings
        if !alias.isEmpty {
            options.pointee.alias_count = Int32(alias.count)
            options.pointee.alias_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: alias.count)
            options.pointee.alias_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: alias.count)
            
            for (index, (key, value)) in alias.enumerated() {
                options.pointee.alias_keys[index] = strdup(key)
                options.pointee.alias_values[index] = strdup(value)
            }
        }
        
        // Main fields
        if !mainFields.isEmpty {
            options.pointee.main_fields_count = Int32(mainFields.count)
            options.pointee.main_fields = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: mainFields.count)
            
            for (index, field) in mainFields.enumerated() {
                options.pointee.main_fields[index] = strdup(field)
            }
        }
        
        // Conditions
        if !conditions.isEmpty {
            options.pointee.conditions_count = Int32(conditions.count)
            options.pointee.conditions = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: conditions.count)
            
            for (index, condition) in conditions.enumerated() {
                options.pointee.conditions[index] = strdup(condition)
            }
        }
        
        // Loader mappings
        if !loader.isEmpty {
            options.pointee.loader_count = Int32(loader.count)
            options.pointee.loader_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: loader.count)
            options.pointee.loader_values = UnsafeMutablePointer<Int32>.allocate(capacity: loader.count)
            
            for (index, (key, value)) in loader.enumerated() {
                options.pointee.loader_keys[index] = strdup(key)
                options.pointee.loader_values[index] = value.cValue
            }
        }
        
        // Resolve extensions
        if !resolveExtensions.isEmpty {
            options.pointee.resolve_extensions_count = Int32(resolveExtensions.count)
            options.pointee.resolve_extensions = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: resolveExtensions.count)
            
            for (index, ext) in resolveExtensions.enumerated() {
                options.pointee.resolve_extensions[index] = strdup(ext)
            }
        }
        
        // Out extension mappings
        if !outExtension.isEmpty {
            options.pointee.out_extension_count = Int32(outExtension.count)
            options.pointee.out_extension_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: outExtension.count)
            options.pointee.out_extension_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: outExtension.count)
            
            for (index, (key, value)) in outExtension.enumerated() {
                options.pointee.out_extension_keys[index] = strdup(key)
                options.pointee.out_extension_values[index] = strdup(value)
            }
        }
        
        // Inject files
        if !inject.isEmpty {
            options.pointee.inject_count = Int32(inject.count)
            options.pointee.inject = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: inject.count)
            
            for (index, injectFile) in inject.enumerated() {
                options.pointee.inject[index] = strdup(injectFile)
            }
        }
        
        // Node paths
        if !nodePaths.isEmpty {
            options.pointee.node_paths_count = Int32(nodePaths.count)
            options.pointee.node_paths = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: nodePaths.count)
            
            for (index, path) in nodePaths.enumerated() {
                options.pointee.node_paths[index] = strdup(path)
            }
        }
        if let publicPath = publicPath {
            options.pointee.public_path = strdup(publicPath)
        }
        
        // Naming Templates
        if let entryNames = entryNames {
            options.pointee.entry_names = strdup(entryNames)
        }
        if let chunkNames = chunkNames {
            options.pointee.chunk_names = strdup(chunkNames)
        }
        if let assetNames = assetNames {
            options.pointee.asset_names = strdup(assetNames)
        }
        
        // Advanced entry points
        if !entryPointsAdvanced.isEmpty {
            options.pointee.entry_points_advanced_count = Int32(entryPointsAdvanced.count)
            options.pointee.entry_points_advanced = UnsafeMutablePointer<esbuild_entry_point>.allocate(capacity: entryPointsAdvanced.count)
            
            for (index, entryPoint) in entryPointsAdvanced.enumerated() {
                let advancedEP = entryPoint.cValue
                options.pointee.entry_points_advanced[index] = advancedEP.pointee
            }
        }
        
        // Stdin configuration
        if let stdin = stdin {
            options.pointee.stdin = stdin.cValue
        }
        
        return options
    }
    
    // MARK: - Initialization
    
    /// Creates build options with default values
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
        tsconfig: String? = nil,
        tsconfigRaw: String? = nil,
        banner: [String: String] = [:],
        footer: [String: String] = [:],
        define: [String: String] = [:],
        pure: [String] = [],
        keepNames: Bool = false,
        bundle: Bool = false,
        preserveSymlinks: Bool = false,
        splitting: Bool = false,
        outfile: String? = nil,
        outdir: String? = nil,
        outbase: String? = nil,
        absWorkingDir: String? = nil,
        metafile: Bool = false,
        write: Bool = true,
        allowOverwrite: Bool = false,
        external: [String] = [],
        packages: ESBuildPackages = .default,
        alias: [String: String] = [:],
        mainFields: [String] = [],
        conditions: [String] = [],
        loader: [String: ESBuildLoader] = [:],
        resolveExtensions: [String] = [],
        outExtension: [String: String] = [:],
        publicPath: String? = nil,
        inject: [String] = [],
        nodePaths: [String] = [],
        entryNames: String? = nil,
        chunkNames: String? = nil,
        assetNames: String? = nil,
        entryPoints: [String] = [],
        entryPointsAdvanced: [ESBuildEntryPoint] = [],
        stdin: ESBuildStdinOptions? = nil
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
        self.tsconfig = tsconfig
        self.tsconfigRaw = tsconfigRaw
        self.banner = banner
        self.footer = footer
        self.define = define
        self.pure = pure
        self.keepNames = keepNames
        self.bundle = bundle
        self.preserveSymlinks = preserveSymlinks
        self.splitting = splitting
        self.outfile = outfile
        self.outdir = outdir
        self.outbase = outbase
        self.absWorkingDir = absWorkingDir
        self.metafile = metafile
        self.write = write
        self.allowOverwrite = allowOverwrite
        self.external = external
        self.packages = packages
        self.alias = alias
        self.mainFields = mainFields
        self.conditions = conditions
        self.loader = loader
        self.resolveExtensions = resolveExtensions
        self.outExtension = outExtension
        self.publicPath = publicPath
        self.inject = inject
        self.nodePaths = nodePaths
        self.entryNames = entryNames
        self.chunkNames = chunkNames
        self.assetNames = assetNames
        self.entryPoints = entryPoints
        self.entryPointsAdvanced = entryPointsAdvanced
        self.stdin = stdin
    }
}

// MARK: - Supporting Types

/// Entry point configuration for builds
public struct ESBuildEntryPoint: Sendable {
    /// Input file path
    public let inputPath: String
    
    /// Output file path
    public let outputPath: String
    
    public init(inputPath: String, outputPath: String) {
        self.inputPath = inputPath
        self.outputPath = outputPath
    }
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<esbuild_entry_point> {
        let entryPoint = esbuild_create_entry_point()!
        
        entryPoint.pointee.input_path = strdup(inputPath)
        entryPoint.pointee.output_path = strdup(outputPath)
        
        return entryPoint
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<esbuild_entry_point>) -> ESBuildEntryPoint {
        return ESBuildEntryPoint(
            inputPath: String(cString: cValue.pointee.input_path),
            outputPath: String(cString: cValue.pointee.output_path)
        )
    }
}

/// Stdin input configuration for builds
public struct ESBuildStdinOptions: Sendable {
    /// Stdin content
    public let contents: String
    
    /// Resolution directory
    public let resolveDir: String
    
    /// Virtual filename
    public let sourcefile: String
    
    /// Content loader
    public let loader: ESBuildLoader
    
    public init(
        contents: String,
        resolveDir: String,
        sourcefile: String,
        loader: ESBuildLoader
    ) {
        self.contents = contents
        self.resolveDir = resolveDir
        self.sourcefile = sourcefile
        self.loader = loader
    }
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<esbuild_stdin_options> {
        let stdin = esbuild_create_stdin_options()!
        
        stdin.pointee.contents = strdup(contents)
        stdin.pointee.resolve_dir = strdup(resolveDir)
        stdin.pointee.sourcefile = strdup(sourcefile)
        stdin.pointee.loader = loader.cValue
        
        return stdin
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<esbuild_stdin_options>) -> ESBuildStdinOptions {
        return ESBuildStdinOptions(
            contents: String(cString: cValue.pointee.contents),
            resolveDir: String(cString: cValue.pointee.resolve_dir),
            sourcefile: String(cString: cValue.pointee.sourcefile),
            loader: ESBuildLoader(rawValue: cValue.pointee.loader) ?? .default
        )
    }
}

/// Output file from build
public struct ESBuildOutputFile: Sendable {
    /// Output file path
    public let path: String
    
    /// File contents as data
    public let contents: Data
    
    /// Content hash
    public let hash: String
    
    public init(path: String, contents: Data, hash: String) {
        self.path = path
        self.contents = contents
        self.hash = hash
    }
    
    /// Convert to C bridge representation
    public var cValue: UnsafeMutablePointer<esbuild_output_file> {
        let file = esbuild_create_output_file()!
        
        file.pointee.path = strdup(path)
        let contentsString = String(data: contents, encoding: .utf8) ?? ""
        file.pointee.contents = strdup(contentsString)
        file.pointee.contents_length = Int32(contents.count)
        file.pointee.hash = strdup(hash)
        
        return file
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<esbuild_output_file>) -> ESBuildOutputFile {
        let contentsString = String(cString: cValue.pointee.contents)
        let contents = Data(contentsString.utf8)
        
        return ESBuildOutputFile(
            path: String(cString: cValue.pointee.path),
            contents: contents,
            hash: String(cString: cValue.pointee.hash)
        )
    }
}

/// Result of an ESBuild build operation
public struct ESBuildBuildResult: Sendable {
    /// Error messages from the build
    public let errors: [ESBuildMessage]
    
    /// Warning messages from the build
    public let warnings: [ESBuildMessage]
    
    /// Generated output files
    public let outputFiles: [ESBuildOutputFile]
    
    /// Build metadata as JSON string
    public let metafile: String?
    
    /// Mangle cache for consistent property renaming
    public let mangleCache: [String: String]
    
    public init(
        errors: [ESBuildMessage] = [],
        warnings: [ESBuildMessage] = [],
        outputFiles: [ESBuildOutputFile] = [],
        metafile: String? = nil,
        mangleCache: [String: String] = [:]
    ) {
        self.errors = errors
        self.warnings = warnings
        self.outputFiles = outputFiles
        self.metafile = metafile
        self.mangleCache = mangleCache
    }
    
    /// Initialize from C bridge value
    public static func from(cValue: UnsafePointer<esbuild_build_result>) -> ESBuildBuildResult {
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
        
        var outputFiles: [ESBuildOutputFile] = []
        if let filesPtr = cValue.pointee.output_files, cValue.pointee.output_files_count > 0 {
            for i in 0..<Int(cValue.pointee.output_files_count) {
                let filePtr = filesPtr.advanced(by: i)
                outputFiles.append(ESBuildOutputFile.from(cValue: filePtr))
            }
        }
        
        let metafile: String?
        if let metafilePtr = cValue.pointee.metafile {
            metafile = String(cString: metafilePtr)
        } else {
            metafile = nil
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
        
        return ESBuildBuildResult(
            errors: errors,
            warnings: warnings,
            outputFiles: outputFiles,
            metafile: metafile,
            mangleCache: mangleCache
        )
    }
}

// MARK: - Build Function

/// Build a project using ESBuild
/// - Parameters:
///   - options: Build options
/// - Returns: Build result containing output files and metadata
public func esbuildBuild(options: ESBuildBuildOptions = ESBuildBuildOptions()) -> ESBuildBuildResult? {
    // Create options with silent logging to capture errors in result instead of printing to console
    let silentOptions = ESBuildBuildOptions(
        color: options.color,
        logLevel: .silent, // Override to silent
        logLimit: options.logLimit,
        logOverride: options.logOverride,
        sourcemap: options.sourcemap,
        sourceRoot: options.sourceRoot,
        sourcesContent: options.sourcesContent,
        target: options.target,
        engines: options.engines,
        supported: options.supported,
        platform: options.platform,
        format: options.format,
        globalName: options.globalName,
        mangleProps: options.mangleProps,
        reserveProps: options.reserveProps,
        mangleQuoted: options.mangleQuoted,
        mangleCache: options.mangleCache,
        drop: options.drop,
        dropLabels: options.dropLabels,
        minifyWhitespace: options.minifyWhitespace,
        minifyIdentifiers: options.minifyIdentifiers,
        minifySyntax: options.minifySyntax,
        lineLimit: options.lineLimit,
        charset: options.charset,
        treeShaking: options.treeShaking,
        ignoreAnnotations: options.ignoreAnnotations,
        legalComments: options.legalComments,
        jsx: options.jsx,
        jsxFactory: options.jsxFactory,
        jsxFragment: options.jsxFragment,
        jsxImportSource: options.jsxImportSource,
        jsxDev: options.jsxDev,
        jsxSideEffects: options.jsxSideEffects,
        tsconfig: options.tsconfig,
        tsconfigRaw: options.tsconfigRaw,
        banner: options.banner,
        footer: options.footer,
        define: options.define,
        pure: options.pure,
        keepNames: options.keepNames,
        bundle: options.bundle,
        preserveSymlinks: options.preserveSymlinks,
        splitting: options.splitting,
        outfile: options.outfile,
        outdir: options.outdir,
        outbase: options.outbase,
        absWorkingDir: options.absWorkingDir,
        metafile: options.metafile,
        write: options.write,
        allowOverwrite: options.allowOverwrite,
        external: options.external,
        packages: options.packages,
        alias: options.alias,
        mainFields: options.mainFields,
        conditions: options.conditions,
        loader: options.loader,
        resolveExtensions: options.resolveExtensions,
        outExtension: options.outExtension,
        publicPath: options.publicPath,
        inject: options.inject,
        nodePaths: options.nodePaths,
        entryNames: options.entryNames,
        chunkNames: options.chunkNames,
        assetNames: options.assetNames,
        entryPoints: options.entryPoints,
        entryPointsAdvanced: options.entryPointsAdvanced,
        stdin: options.stdin
    )
    
    let cOptions = silentOptions.cValue
    defer { esbuild_free_build_options(cOptions) }
    
    let cResult = esbuild_build(cOptions)
    
    defer {
        if let result = cResult {
            esbuild_free_build_result(result)
        }
    }
    
    guard let result = cResult else {
        return nil
    }
    
    return ESBuildBuildResult.from(cValue: result)
}