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

    /// Enable all minification options
    public var minify: Bool

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

    /// Plugins to use during the build
    public var plugins: [ESBuildPlugin]

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
            options.pointee.log_override_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: logOverride.count)
            options.pointee.log_override_values = UnsafeMutablePointer<Int32>.allocate(capacity: logOverride.count)

            for (index, (key, value)) in logOverride.enumerated() {
                options.pointee.log_override_keys[index] = strdup(key)
                options.pointee.log_override_values[index] = value.cValue
            }
        }

        // Source Map
        options.pointee.sourcemap = sourcemap.cValue
        if let sourceRoot {
            options.pointee.source_root = strdup(sourceRoot)
        }
        options.pointee.sources_content = sourcesContent.cValue

        // Target and Compatibility
        options.pointee.target = target.cValue

        // Engines
        if !engines.isEmpty {
            options.pointee.engines_count = Int32(engines.count)
            options.pointee.engine_names = UnsafeMutablePointer<Int32>.allocate(capacity: engines.count)
            options.pointee.engine_versions = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: engines.count)

            for (index, engineInfo) in engines.enumerated() {
                options.pointee.engine_names[index] = engineInfo.engine.cValue
                options.pointee.engine_versions[index] = strdup(engineInfo.version)
            }
        }

        // Supported features
        if !supported.isEmpty {
            options.pointee.supported_count = Int32(supported.count)
            options.pointee.supported_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: supported.count)
            options.pointee.supported_values = UnsafeMutablePointer<Int32>.allocate(capacity: supported.count)

            for (index, (key, value)) in supported.enumerated() {
                options.pointee.supported_keys[index] = strdup(key)
                options.pointee.supported_values[index] = value ? 1 : 0
            }
        }

        options.pointee.platform = platform.cValue
        options.pointee.format = format.cValue
        if let globalName {
            options.pointee.global_name = strdup(globalName)
        }

        // Minification and Property Mangling
        if let mangleProps {
            options.pointee.mangle_props = strdup(mangleProps)
        }
        if let reserveProps {
            options.pointee.reserve_props = strdup(reserveProps)
        }
        options.pointee.mangle_quoted = mangleQuoted.cValue

        // Mangle cache
        if !mangleCache.isEmpty {
            options.pointee.mangle_cache_count = Int32(mangleCache.count)
            options.pointee.mangle_cache_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: mangleCache.count)
            options.pointee.mangle_cache_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: mangleCache.count)

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
            options.pointee.drop_labels = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: dropLabels.count)

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
        if let jsxFactory {
            options.pointee.jsx_factory = strdup(jsxFactory)
        }
        if let jsxFragment {
            options.pointee.jsx_fragment = strdup(jsxFragment)
        }
        if let jsxImportSource {
            options.pointee.jsx_import_source = strdup(jsxImportSource)
        }
        options.pointee.jsx_dev = jsxDev ? 1 : 0
        options.pointee.jsx_side_effects = jsxSideEffects ? 1 : 0

        // TypeScript Configuration
        if let tsconfig {
            options.pointee.tsconfig = strdup(tsconfig)
        }
        if let tsconfigRaw {
            options.pointee.tsconfig_raw = strdup(tsconfigRaw)
        }

        // Code Injection - Banner
        if !banner.isEmpty {
            options.pointee.banner_count = Int32(banner.count)
            options.pointee.banner_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: banner.count)
            options.pointee.banner_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: banner.count)

            for (index, (key, value)) in banner.enumerated() {
                options.pointee.banner_keys[index] = strdup(key)
                options.pointee.banner_values[index] = strdup(value)
            }
        }

        // Code Injection - Footer
        if !footer.isEmpty {
            options.pointee.footer_count = Int32(footer.count)
            options.pointee.footer_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: footer.count)
            options.pointee.footer_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: footer.count)

            for (index, (key, value)) in footer.enumerated() {
                options.pointee.footer_keys[index] = strdup(key)
                options.pointee.footer_values[index] = strdup(value)
            }
        }

        // Code Transformation - Define
        if !define.isEmpty {
            options.pointee.define_count = Int32(define.count)
            options.pointee.define_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: define.count)
            options.pointee.define_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: define.count)

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
        if let outfile {
            options.pointee.outfile = strdup(outfile)
        }
        if let outdir {
            options.pointee.outdir = strdup(outdir)
        }
        if let outbase {
            options.pointee.outbase = strdup(outbase)
        }
        if let absWorkingDir {
            options.pointee.abs_working_dir = strdup(absWorkingDir)
        }
        options.pointee.metafile = metafile ? 1 : 0
        options.pointee.write = write ? 1 : 0
        options.pointee.allow_overwrite = allowOverwrite ? 1 : 0

        // Entry points
        if !entryPoints.isEmpty {
            options.pointee.entry_points_count = Int32(entryPoints.count)
            options.pointee.entry_points = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: entryPoints.count)
            for (index, entryPoint) in entryPoints.enumerated() {
                options.pointee.entry_points[index] = strdup(entryPoint)
            }
        }

        // Module Resolution
        // External packages
        if !external.isEmpty {
            options.pointee.external_count = Int32(external.count)
            options.pointee.external = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: external.count)

            for (index, ext) in external.enumerated() {
                options.pointee.external[index] = strdup(ext)
            }
        }

        options.pointee.packages = packages.cValue

        // Alias mappings
        if !alias.isEmpty {
            options.pointee.alias_count = Int32(alias.count)
            options.pointee.alias_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: alias.count)
            options.pointee.alias_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: alias.count)

            for (index, (key, value)) in alias.enumerated() {
                options.pointee.alias_keys[index] = strdup(key)
                options.pointee.alias_values[index] = strdup(value)
            }
        }

        // Main fields
        if !mainFields.isEmpty {
            options.pointee.main_fields_count = Int32(mainFields.count)
            options.pointee.main_fields = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: mainFields.count)

            for (index, field) in mainFields.enumerated() {
                options.pointee.main_fields[index] = strdup(field)
            }
        }

        // Conditions
        if !conditions.isEmpty {
            options.pointee.conditions_count = Int32(conditions.count)
            options.pointee.conditions = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: conditions.count)

            for (index, condition) in conditions.enumerated() {
                options.pointee.conditions[index] = strdup(condition)
            }
        }

        // Loader mappings
        if !loader.isEmpty {
            options.pointee.loader_count = Int32(loader.count)
            options.pointee.loader_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: loader.count)
            options.pointee.loader_values = UnsafeMutablePointer<Int32>.allocate(capacity: loader.count)

            for (index, (key, value)) in loader.enumerated() {
                options.pointee.loader_keys[index] = strdup(key)
                options.pointee.loader_values[index] = value.cValue
            }
        }

        // Resolve extensions
        if !resolveExtensions.isEmpty {
            options.pointee.resolve_extensions_count = Int32(resolveExtensions.count)
            options.pointee.resolve_extensions = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: resolveExtensions.count)

            for (index, ext) in resolveExtensions.enumerated() {
                options.pointee.resolve_extensions[index] = strdup(ext)
            }
        }

        // Out extension mappings
        if !outExtension.isEmpty {
            options.pointee.out_extension_count = Int32(outExtension.count)
            options.pointee.out_extension_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: outExtension.count)
            options.pointee.out_extension_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: outExtension.count)

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
            options.pointee.node_paths = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: nodePaths.count)

            for (index, path) in nodePaths.enumerated() {
                options.pointee.node_paths[index] = strdup(path)
            }
        }
        if let publicPath {
            options.pointee.public_path = strdup(publicPath)
        }

        // Naming Templates
        if let entryNames {
            options.pointee.entry_names = strdup(entryNames)
        }
        if let chunkNames {
            options.pointee.chunk_names = strdup(chunkNames)
        }
        if let assetNames {
            options.pointee.asset_names = strdup(assetNames)
        }

        // Advanced entry points
        if !entryPointsAdvanced.isEmpty {
            options.pointee.entry_points_advanced_count = Int32(entryPointsAdvanced.count)
            options.pointee.entry_points_advanced = UnsafeMutablePointer<esbuild_entry_point>
                .allocate(capacity: entryPointsAdvanced.count)

            for (index, entryPoint) in entryPointsAdvanced.enumerated() {
                let advancedEP = entryPoint.cValue
                options.pointee.entry_points_advanced[index] = advancedEP.pointee
            }
        }

        // Stdin configuration
        if let stdin {
            options.pointee.stdin = stdin.cValue
        }

        // Plugins configuration
        if !plugins.isEmpty {
            // print("Build has \(plugins.count) plugins configured")

            // Create C plugins array
            options.pointee.plugins_count = Int32(plugins.count)
            options.pointee.plugins = UnsafeMutablePointer<c_plugin>.allocate(capacity: plugins.count)

            // Convert each Swift plugin to C plugin
            for (index, plugin) in plugins.enumerated() {
                let cPlugin = plugin.createCPlugin()
                // Store the plugin pointer directly in the array
                options.pointee.plugins[index] = cPlugin.pointee

                // Register the plugin for callback management using the array element
                let arrayElementPtr = options.pointee.plugins.advanced(by: index)
                let _ = register_plugin(arrayElementPtr)

                // The plugin memory is now managed by the build options
            }
        } else {
            options.pointee.plugins = nil
            options.pointee.plugins_count = 0
        }

        return options
    }

    // MARK: - Initialization

    /// Creates build options with default values
    public init(
        absWorkingDir: String? = nil,
        alias: [String: String] = [:],
        allowOverwrite: Bool = false,
        assetNames: String? = nil,
        banner: [String: String] = [:],
        bundle: Bool = false,
        charset: ESBuildCharset = .default,
        chunkNames: String? = nil,
        color: ESBuildColor = .ifTerminal,
        conditions: [String] = [],
        define: [String: String] = [:],
        drop: Set<ESBuildDrop> = [],
        dropLabels: [String] = [],
        engines: [(engine: ESBuildEngine, version: String)] = [],
        entryNames: String? = nil,
        entryPoints: [String] = [],
        entryPointsAdvanced: [ESBuildEntryPoint] = [],
        external: [String] = [],
        footer: [String: String] = [:],
        format: ESBuildFormat = .default,
        globalName: String? = nil,
        ignoreAnnotations: Bool = false,
        inject: [String] = [],
        jsx: ESBuildJSX = .transform,
        jsxDev: Bool = false,
        jsxFactory: String? = nil,
        jsxFragment: String? = nil,
        jsxImportSource: String? = nil,
        jsxSideEffects: Bool = false,
        keepNames: Bool = false,
        legalComments: ESBuildLegalComments = .default,
        lineLimit: Int32 = 0,
        loader: [String: ESBuildLoader] = [:],
        logLevel: ESBuildLogLevel = .info,
        logLimit: Int32 = 0,
        logOverride: [String: ESBuildLogLevel] = [:],
        mainFields: [String] = [],
        mangleCache: [String: String] = [:],
        mangleProps: String? = nil,
        mangleQuoted: ESBuildMangleQuoted = .false,
        metafile: Bool = false,
        minify: Bool = false,
        minifyIdentifiers: Bool? = nil,
        minifySyntax: Bool? = nil,
        minifyWhitespace: Bool? = nil,
        nodePaths: [String] = [],
        outbase: String? = nil,
        outdir: String? = nil,
        outExtension: [String: String] = [:],
        outfile: String? = nil,
        packages: ESBuildPackages = .default,
        platform: ESBuildPlatform = .default,
        plugins: [ESBuildPlugin] = [],
        preserveSymlinks: Bool = false,
        publicPath: String? = nil,
        pure: [String] = [],
        reserveProps: String? = nil,
        resolveExtensions: [String] = [],
        sourcemap: ESBuildSourceMap = .none,
        sourceRoot: String? = nil,
        sourcesContent: ESBuildSourcesContent = .include,
        splitting: Bool = false,
        stdin: ESBuildStdinOptions? = nil,
        supported: [String: Bool] = [:],
        target: ESBuildTarget = .default,
        treeShaking: ESBuildTreeShaking = .default,
        tsconfig: String? = nil,
        tsconfigRaw: String? = nil,
        write: Bool = true
    ) {
        self.absWorkingDir = absWorkingDir
        self.alias = alias
        self.allowOverwrite = allowOverwrite
        self.assetNames = assetNames
        self.banner = banner
        self.bundle = bundle
        self.charset = charset
        self.chunkNames = chunkNames
        self.color = color
        self.conditions = conditions
        self.define = define
        self.drop = drop
        self.dropLabels = dropLabels
        self.engines = engines
        self.entryNames = entryNames
        self.entryPoints = entryPoints
        self.entryPointsAdvanced = entryPointsAdvanced
        self.external = external
        self.footer = footer
        self.format = format
        self.globalName = globalName
        self.ignoreAnnotations = ignoreAnnotations
        self.inject = inject
        self.jsx = jsx
        self.jsxDev = jsxDev
        self.jsxFactory = jsxFactory
        self.jsxFragment = jsxFragment
        self.jsxImportSource = jsxImportSource
        self.jsxSideEffects = jsxSideEffects
        self.keepNames = keepNames
        self.legalComments = legalComments
        self.lineLimit = lineLimit
        self.loader = loader
        self.logLevel = logLevel
        self.logLimit = logLimit
        self.logOverride = logOverride
        self.mainFields = mainFields
        self.mangleCache = mangleCache
        self.mangleProps = mangleProps
        self.mangleQuoted = mangleQuoted
        self.metafile = metafile
        self.minify = minify
        self.minifyIdentifiers = minifyIdentifiers ?? minify
        self.minifySyntax = minifySyntax ?? minify
        self.minifyWhitespace = minifyWhitespace ?? minify
        self.nodePaths = nodePaths
        self.outbase = outbase
        self.outdir = outdir
        self.outExtension = outExtension
        self.outfile = outfile
        self.packages = packages
        self.platform = platform
        self.plugins = plugins
        self.preserveSymlinks = preserveSymlinks
        self.publicPath = publicPath
        self.pure = pure
        self.reserveProps = reserveProps
        self.resolveExtensions = resolveExtensions
        self.sourcemap = sourcemap
        self.sourceRoot = sourceRoot
        self.sourcesContent = sourcesContent
        self.splitting = splitting
        self.stdin = stdin
        self.supported = supported
        self.target = target
        self.treeShaking = treeShaking
        self.tsconfig = tsconfig
        self.tsconfigRaw = tsconfigRaw
        self.write = write
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
        ESBuildEntryPoint(
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
        loader: ESBuildLoader,
        resolveDir: String,
        sourcefile: String
    ) {
        self.contents = contents
        self.loader = loader
        self.resolveDir = resolveDir
        self.sourcefile = sourcefile
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
        ESBuildStdinOptions(
            contents: String(cString: cValue.pointee.contents),
            loader: ESBuildLoader(rawValue: cValue.pointee.loader) ?? .default,
            resolveDir: String(cString: cValue.pointee.resolve_dir),
            sourcefile: String(cString: cValue.pointee.sourcefile)
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

    public init(contents: Data, hash: String, path: String) {
        self.contents = contents
        self.hash = hash
        self.path = path
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
            contents: contents,
            hash: String(cString: cValue.pointee.hash),
            path: String(cString: cValue.pointee.path)
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
            for i in 0 ..< Int(cValue.pointee.errors_count) {
                let errorPtr = errorsPtr.advanced(by: i)
                errors.append(ESBuildMessage.from(cValue: errorPtr))
            }
        }

        var warnings: [ESBuildMessage] = []
        if let warningsPtr = cValue.pointee.warnings, cValue.pointee.warnings_count > 0 {
            for i in 0 ..< Int(cValue.pointee.warnings_count) {
                let warningPtr = warningsPtr.advanced(by: i)
                warnings.append(ESBuildMessage.from(cValue: warningPtr))
            }
        }

        var outputFiles: [ESBuildOutputFile] = []
        if let filesPtr = cValue.pointee.output_files, cValue.pointee.output_files_count > 0 {
            for i in 0 ..< Int(cValue.pointee.output_files_count) {
                let filePtr = filesPtr.advanced(by: i)
                outputFiles.append(ESBuildOutputFile.from(cValue: filePtr))
            }
        }

        let metafile: String? = if let metafilePtr = cValue.pointee.metafile {
            String(cString: metafilePtr)
        } else {
            nil
        }

        var mangleCache: [String: String] = [:]
        if let keysPtr = cValue.pointee.mangle_cache_keys,
           let valuesPtr = cValue.pointee.mangle_cache_values,
           cValue.pointee.mangle_cache_count > 0
        {
            let keysBuffer = UnsafeBufferPointer(start: keysPtr, count: Int(cValue.pointee.mangle_cache_count))
            let valuesBuffer = UnsafeBufferPointer(start: valuesPtr, count: Int(cValue.pointee.mangle_cache_count))

            for (keyPtr, valuePtr) in zip(keysBuffer, valuesBuffer) {
                if let keyPtr, let valuePtr {
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
/// Build with ESBuild using a custom file resolver
/// - Parameters:
///   - options: ESBuild build options
///   - resolver: Function that resolves file paths to FileResolver cases or nil
/// - Returns: Build result with compilation status and output files
public func esbuildBuild(
    options: ESBuildBuildOptions = ESBuildBuildOptions(),
    resolver: @escaping @Sendable (String) async throws -> FileResolver?
) async throws -> ESBuildBuildResult? {
    // Create a resolver plugin that uses the provided callback
    let resolverPlugin = ESBuildPlugin(name: "dynamic-resolver") { build in
        let namespace = "swifttsgo"

        // Handle all file resolution
        build.onResolve(filter: ".*", namespace: nil) { args in
            do {
                // Try to resolve the import path
                let resolvedPath = try await resolveImportPath(args.path, from: args.importer, resolver: resolver)
                if let path = resolvedPath {
                    return ESBuildOnResolveResult(namespace: namespace, path: path)
                }
            } catch {
                return ESBuildOnResolveResult(
                    errors: [
                        ESBuildPluginMessage(text: "Failed to resolve '\(args.path)': \(error.localizedDescription)"),
                    ]
                )
            }
            return nil
        }

        // Handle all file loading
        build.onLoad(filter: ".*", namespace: namespace) { args in
            do {
                if let fileResolver = try await resolver(args.path) {
                    if case let .file(content) = fileResolver {
                        return ESBuildOnLoadResult(
                            contents: content,
                            loader: detectLoader(for: args.path)
                        )
                    }
                }
            } catch {
                return ESBuildOnLoadResult(
                    errors: [ESBuildPluginMessage(text: "Failed to load '\(args.path)': \(error.localizedDescription)")]
                )
            }
            return nil
        }
    }

    // Add the resolver plugin to the build options
    var buildOptions = options
    buildOptions.plugins = buildOptions.plugins + [resolverPlugin]

    return esbuildBuild(options: buildOptions)
}

/// Helper function to resolve import paths relative to the importer
private func resolveImportPath(
    _ importPath: String,
    from importer: String?,
    resolver: @Sendable (String) async throws -> FileResolver?
) async throws -> String? {
    // Handle absolute paths
    if importPath.hasPrefix("/") {
        return try await resolver(importPath) != nil ? importPath : nil
    }

    // Handle relative paths
    if importPath.hasPrefix("./") || importPath.hasPrefix("../") {
        guard let importer else { return nil }

        let importerDir = (importer as NSString).deletingLastPathComponent
        let resolvedPath = (importerDir as NSString).appendingPathComponent(importPath)
        let normalizedPath = (resolvedPath as NSString).standardizingPath

        return try await resolver(normalizedPath) != nil ? normalizedPath : nil
    }

    // Handle node_modules style imports (simplified)
    // For now, just check if the path exists as-is
    return try await resolver(importPath) != nil ? importPath : nil
}

/// Detect the appropriate loader based on file extension
private func detectLoader(for path: String) -> ESBuildLoader {
    let ext = (path as NSString).pathExtension.lowercased()

    switch ext {
    case "js": return .js
    case "jsx": return .jsx
    case "ts": return .ts
    case "tsx": return .tsx
    case "json": return .json
    case "css": return .css
    case "txt": return .text
    default: return .js // Default to JS
    }
}

/// Build with ESBuild using in-memory files
/// - Parameters:
///   - files: Dictionary of file paths to content
///   - options: ESBuild build options
/// - Returns: Build result with compilation status and output files
public func esbuildBuild(
    files: [String: String],
    options: ESBuildBuildOptions = ESBuildBuildOptions()
) async throws -> ESBuildBuildResult? {
    try await esbuildBuild(options: options) { path in
        if let content = files[path] {
            return .file(content)
        }

        // Check if it's a directory by looking for child files
        let childFiles = files.keys
            .filter { $0.hasPrefix(path + "/") }
            .compactMap { filePath -> String? in
                let relativePath = String(filePath.dropFirst(path.count + 1))
                // Only return direct children, not nested paths
                if !relativePath.contains("/") {
                    return relativePath
                }
                return nil
            }

        if !childFiles.isEmpty {
            return .directory(childFiles)
        }

        return nil
    }
}

public func esbuildBuild(options: ESBuildBuildOptions = ESBuildBuildOptions()) -> ESBuildBuildResult? {
    // Create options with silent logging to capture errors in result instead of printing to console
    let silentOptions = ESBuildBuildOptions(
        absWorkingDir: options.absWorkingDir,
        alias: options.alias,
        allowOverwrite: options.allowOverwrite,
        assetNames: options.assetNames,
        banner: options.banner,
        bundle: options.bundle,
        charset: options.charset,
        chunkNames: options.chunkNames,
        color: options.color,
        conditions: options.conditions,
        define: options.define,
        drop: options.drop,
        dropLabels: options.dropLabels,
        engines: options.engines,
        entryNames: options.entryNames,
        entryPoints: options.entryPoints,
        entryPointsAdvanced: options.entryPointsAdvanced,
        external: options.external,
        footer: options.footer,
        format: options.format,
        globalName: options.globalName,
        ignoreAnnotations: options.ignoreAnnotations,
        inject: options.inject,
        jsx: options.jsx,
        jsxDev: options.jsxDev,
        jsxFactory: options.jsxFactory,
        jsxFragment: options.jsxFragment,
        jsxImportSource: options.jsxImportSource,
        jsxSideEffects: options.jsxSideEffects,
        keepNames: options.keepNames,
        legalComments: options.legalComments,
        lineLimit: options.lineLimit,
        loader: options.loader,
        logLevel: .silent, // Override to silent
        logLimit: options.logLimit,
        logOverride: options.logOverride,
        mainFields: options.mainFields,
        mangleCache: options.mangleCache,
        mangleProps: options.mangleProps,
        mangleQuoted: options.mangleQuoted,
        metafile: options.metafile,
        minify: options.minify,
        minifyIdentifiers: options.minifyIdentifiers,
        minifySyntax: options.minifySyntax,
        minifyWhitespace: options.minifyWhitespace,
        nodePaths: options.nodePaths,
        outbase: options.outbase,
        outdir: options.outdir,
        outExtension: options.outExtension,
        outfile: options.outfile,
        packages: options.packages,
        platform: options.platform,
        plugins: options.plugins,
        preserveSymlinks: options.preserveSymlinks,
        publicPath: options.publicPath,
        pure: options.pure,
        reserveProps: options.reserveProps,
        resolveExtensions: options.resolveExtensions,
        sourcemap: options.sourcemap,
        sourceRoot: options.sourceRoot,
        sourcesContent: options.sourcesContent,
        splitting: options.splitting,
        stdin: options.stdin,
        supported: options.supported,
        target: options.target,
        treeShaking: options.treeShaking,
        tsconfig: options.tsconfig,
        tsconfigRaw: options.tsconfigRaw,
        write: options.write
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
