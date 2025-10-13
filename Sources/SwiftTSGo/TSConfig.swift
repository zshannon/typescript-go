import Foundation

// MARK: - Enums for TypeScript Configuration

public enum ECMAScriptTarget: String, Codable, CaseIterable, Sendable {
    case es3 = "ES3"
    case es5 = "ES5"
    case es2015 = "ES2015"
    case es2016 = "ES2016"
    case es2017 = "ES2017"
    case es2018 = "ES2018"
    case es2019 = "ES2019"
    case es2020 = "ES2020"
    case es2021 = "ES2021"
    case es2022 = "ES2022"
    case esnext = "ESNext"
}

public enum ModuleKind: String, Codable, CaseIterable, Sendable {
    case none
    case commonjs
    case amd
    case system
    case umd
    case es6
    case es2015
    case es2020
    case es2022
    case esnext
    case node16
    case nodenext
}

public enum ModuleResolutionKind: String, Codable, CaseIterable, Sendable {
    case classic
    case node
    case node16
    case nodenext
    case bundler
}

public enum JSXEmit: String, Codable, CaseIterable, Sendable {
    case none
    case preserve
    case react
    case reactNative = "react-native"
    case reactJSX = "react-jsx"
    case reactJSXDev = "react-jsxdev"
}

public enum NewLineKind: String, Codable, CaseIterable, Sendable {
    case crlf
    case lf
}

public enum ImportsNotUsedAsValues: String, Codable, CaseIterable, Sendable {
    case remove
    case preserve
    case error
}

// MARK: - Compiler Options

public struct CompilerOptions: Codable, Sendable {
    // Type Checking
    public var allowUnreachableCode: Bool?
    public var allowUnusedLabels: Bool?
    public var alwaysStrict: Bool?
    public var exactOptionalPropertyTypes: Bool?
    public var noFallthroughCasesInSwitch: Bool?
    public var noImplicitAny: Bool?
    public var noImplicitOverride: Bool?
    public var noImplicitReturns: Bool?
    public var noImplicitThis: Bool?
    public var noPropertyAccessFromIndexSignature: Bool?
    public var noUncheckedIndexedAccess: Bool?
    public var noUnusedLocals: Bool?
    public var noUnusedParameters: Bool?
    public var strict: Bool?
    public var strictBindCallApply: Bool?
    public var strictFunctionTypes: Bool?
    public var strictNullChecks: Bool?
    public var strictPropertyInitialization: Bool?
    public var useUnknownInCatchVariables: Bool?

    // Modules
    public var allowArbitraryExtensions: Bool?
    public var allowImportingTsExtensions: Bool?
    public var allowSyntheticDefaultImports: Bool?
    public var allowUmdGlobalAccess: Bool?
    public var baseUrl: String?
    public var customConditions: [String]?
    public var module: ModuleKind?
    public var moduleResolution: ModuleResolutionKind?
    public var moduleSuffixes: [String]?
    public var noResolve: Bool?
    public var paths: [String: [String]]?
    public var resolveJsonModule: Bool?
    public var resolvePackageJsonExports: Bool?
    public var resolvePackageJsonImports: Bool?
    public var rootDir: String?
    public var rootDirs: [String]?
    public var typeRoots: [String]?
    public var types: [String]?

    // Emit
    public var declaration: Bool?
    public var declarationDir: String?
    public var declarationMap: Bool?
    public var downlevelIteration: Bool?
    public var emitBOM: Bool?
    public var emitDeclarationOnly: Bool?
    public var emitDecoratorMetadata: Bool?
    public var experimentalDecorators: Bool?
    public var importHelpers: Bool?
    public var importsNotUsedAsValues: ImportsNotUsedAsValues?
    public var inlineSourceMap: Bool?
    public var inlineSources: Bool?
    public var mapRoot: String?
    public var newLine: NewLineKind?
    public var noEmit: Bool?
    public var noEmitHelpers: Bool?
    public var noEmitOnError: Bool?
    public var outDir: String?
    public var outFile: String?
    public var preserveConstEnums: Bool?
    public var preserveValueImports: Bool?
    public var removeComments: Bool?
    public var sourceMap: Bool?
    public var sourceRoot: String?
    public var stripInternal: Bool?

    // JavaScript Support
    public var allowJs: Bool?
    public var checkJs: Bool?
    public var maxNodeModuleJsDepth: Int?

    // Editor Support
    public var disableSizeLimit: Bool?
    public var plugins: [String]?

    // Interop Constraints
    public var allowSyntheticDefaultImports_legacy: Bool?
    public var esModuleInterop: Bool?
    public var forceConsistentCasingInFileNames: Bool?
    public var isolatedModules: Bool?
    public var preserveSymlinks: Bool?
    public var verbatimModuleSyntax: Bool?

    // Backwards Compatibility
    public var charset: String?
    public var keyofStringsOnly: Bool?
    public var noImplicitUseStrict: Bool?
    public var noStrictGenericChecks: Bool?
    public var out: String?
    public var suppressExcessPropertyErrors: Bool?
    public var suppressImplicitAnyIndexErrors: Bool?

    // Language and Environment
    public var emitDecoratorMetadata_legacy: Bool?
    public var experimentalDecorators_legacy: Bool?
    public var jsx: JSXEmit?
    public var jsxFactory: String?
    public var jsxFragmentFactory: String?
    public var jsxImportSource: String?
    public var lib: [String]?
    public var moduleDetection: String?
    public var noLib: Bool?
    public var reactNamespace: String?
    public var target: ECMAScriptTarget?
    public var useDefineForClassFields: Bool?

    // Compiler Diagnostics
    public var diagnostics: Bool?
    public var explainFiles: Bool?
    public var extendedDiagnostics: Bool?
    public var generateCpuProfile: String?
    public var listEmittedFiles: Bool?
    public var listFiles: Bool?
    public var traceResolution: Bool?

    // Projects
    public var composite: Bool?
    public var disableReferencedProjectLoad: Bool?
    public var disableSolutionSearching: Bool?
    public var disableSourceOfProjectReferenceRedirect: Bool?
    public var incremental: Bool?
    public var tsBuildInfoFile: String?

    // Output Formatting
    public var noErrorTruncation: Bool?
    public var preserveWatchOutput: Bool?
    public var pretty: Bool?

    // Completeness
    public var skipDefaultLibCheck: Bool?
    public var skipLibCheck: Bool?

    public init(
        allowArbitraryExtensions: Bool? = nil,
        allowImportingTsExtensions: Bool? = nil,
        allowJs: Bool? = nil,
        allowSyntheticDefaultImports: Bool? = nil,
        allowSyntheticDefaultImports_legacy: Bool? = nil,
        allowUmdGlobalAccess: Bool? = nil,
        allowUnreachableCode: Bool? = nil,
        allowUnusedLabels: Bool? = nil,
        alwaysStrict: Bool? = nil,
        baseUrl: String? = nil,
        charset: String? = nil,
        checkJs: Bool? = nil,
        composite: Bool? = nil,
        customConditions: [String]? = nil,
        declaration: Bool? = nil,
        declarationDir: String? = nil,
        declarationMap: Bool? = nil,
        diagnostics: Bool? = nil,
        disableReferencedProjectLoad: Bool? = nil,
        disableSizeLimit: Bool? = nil,
        disableSolutionSearching: Bool? = nil,
        disableSourceOfProjectReferenceRedirect: Bool? = nil,
        downlevelIteration: Bool? = nil,
        emitBOM: Bool? = nil,
        emitDeclarationOnly: Bool? = nil,
        emitDecoratorMetadata: Bool? = nil,
        emitDecoratorMetadata_legacy: Bool? = nil,
        esModuleInterop: Bool? = nil,
        exactOptionalPropertyTypes: Bool? = nil,
        experimentalDecorators: Bool? = nil,
        experimentalDecorators_legacy: Bool? = nil,
        explainFiles: Bool? = nil,
        extendedDiagnostics: Bool? = nil,
        forceConsistentCasingInFileNames: Bool? = nil,
        generateCpuProfile: String? = nil,
        importHelpers: Bool? = nil,
        importsNotUsedAsValues: ImportsNotUsedAsValues? = nil,
        incremental: Bool? = nil,
        inlineSourceMap: Bool? = nil,
        inlineSources: Bool? = nil,
        isolatedModules: Bool? = nil,
        jsx: JSXEmit? = nil,
        jsxFactory: String? = nil,
        jsxFragmentFactory: String? = nil,
        jsxImportSource: String? = nil,
        keyofStringsOnly: Bool? = nil,
        lib: [String]? = nil,
        listEmittedFiles: Bool? = nil,
        listFiles: Bool? = nil,
        mapRoot: String? = nil,
        maxNodeModuleJsDepth: Int? = nil,
        module: ModuleKind? = nil,
        moduleDetection: String? = nil,
        moduleResolution: ModuleResolutionKind? = nil,
        moduleSuffixes: [String]? = nil,
        newLine: NewLineKind? = nil,
        noEmit: Bool? = nil,
        noEmitHelpers: Bool? = nil,
        noEmitOnError: Bool? = nil,
        noErrorTruncation: Bool? = nil,
        noFallthroughCasesInSwitch: Bool? = nil,
        noImplicitAny: Bool? = nil,
        noImplicitOverride: Bool? = nil,
        noImplicitReturns: Bool? = nil,
        noImplicitThis: Bool? = nil,
        noImplicitUseStrict: Bool? = nil,
        noLib: Bool? = nil,
        noPropertyAccessFromIndexSignature: Bool? = nil,
        noResolve: Bool? = nil,
        noStrictGenericChecks: Bool? = nil,
        noUncheckedIndexedAccess: Bool? = nil,
        noUnusedLocals: Bool? = nil,
        noUnusedParameters: Bool? = nil,
        out: String? = nil,
        outDir: String? = nil,
        outFile: String? = nil,
        paths: [String: [String]]? = nil,
        plugins: [String]? = nil,
        preserveConstEnums: Bool? = nil,
        preserveSymlinks: Bool? = nil,
        preserveValueImports: Bool? = nil,
        preserveWatchOutput: Bool? = nil,
        pretty: Bool? = nil,
        reactNamespace: String? = nil,
        removeComments: Bool? = nil,
        resolveJsonModule: Bool? = nil,
        resolvePackageJsonExports: Bool? = nil,
        resolvePackageJsonImports: Bool? = nil,
        rootDir: String? = nil,
        rootDirs: [String]? = nil,
        skipDefaultLibCheck: Bool? = nil,
        skipLibCheck: Bool? = nil,
        sourceMap: Bool? = nil,
        sourceRoot: String? = nil,
        strict: Bool? = nil,
        strictBindCallApply: Bool? = nil,
        strictFunctionTypes: Bool? = nil,
        strictNullChecks: Bool? = nil,
        strictPropertyInitialization: Bool? = nil,
        stripInternal: Bool? = nil,
        suppressExcessPropertyErrors: Bool? = nil,
        suppressImplicitAnyIndexErrors: Bool? = nil,
        target: ECMAScriptTarget? = nil,
        traceResolution: Bool? = nil,
        tsBuildInfoFile: String? = nil,
        typeRoots: [String]? = nil,
        types: [String]? = nil,
        useDefineForClassFields: Bool? = nil,
        useUnknownInCatchVariables: Bool? = nil,
        verbatimModuleSyntax: Bool? = nil
    ) {
        // Type Checking
        self.allowUnreachableCode = allowUnreachableCode
        self.allowUnusedLabels = allowUnusedLabels
        self.alwaysStrict = alwaysStrict
        self.exactOptionalPropertyTypes = exactOptionalPropertyTypes
        self.noFallthroughCasesInSwitch = noFallthroughCasesInSwitch
        self.noImplicitAny = noImplicitAny
        self.noImplicitOverride = noImplicitOverride
        self.noImplicitReturns = noImplicitReturns
        self.noImplicitThis = noImplicitThis
        self.noPropertyAccessFromIndexSignature = noPropertyAccessFromIndexSignature
        self.noUncheckedIndexedAccess = noUncheckedIndexedAccess
        self.noUnusedLocals = noUnusedLocals
        self.noUnusedParameters = noUnusedParameters
        self.strict = strict
        self.strictBindCallApply = strictBindCallApply
        self.strictFunctionTypes = strictFunctionTypes
        self.strictNullChecks = strictNullChecks
        self.strictPropertyInitialization = strictPropertyInitialization
        self.useUnknownInCatchVariables = useUnknownInCatchVariables

        // Modules
        self.allowArbitraryExtensions = allowArbitraryExtensions
        self.allowImportingTsExtensions = allowImportingTsExtensions
        self.allowSyntheticDefaultImports = allowSyntheticDefaultImports
        self.allowUmdGlobalAccess = allowUmdGlobalAccess
        self.baseUrl = baseUrl
        self.customConditions = customConditions
        self.module = module
        self.moduleResolution = moduleResolution
        self.moduleSuffixes = moduleSuffixes
        self.noResolve = noResolve
        self.paths = paths
        self.resolveJsonModule = resolveJsonModule
        self.resolvePackageJsonExports = resolvePackageJsonExports
        self.resolvePackageJsonImports = resolvePackageJsonImports
        self.rootDir = rootDir
        self.rootDirs = rootDirs
        self.typeRoots = typeRoots
        self.types = types

        // Emit
        self.declaration = declaration
        self.declarationDir = declarationDir
        self.declarationMap = declarationMap
        self.downlevelIteration = downlevelIteration
        self.emitBOM = emitBOM
        self.emitDeclarationOnly = emitDeclarationOnly
        self.emitDecoratorMetadata = emitDecoratorMetadata
        self.experimentalDecorators = experimentalDecorators
        self.importHelpers = importHelpers
        self.importsNotUsedAsValues = importsNotUsedAsValues
        self.inlineSourceMap = inlineSourceMap
        self.inlineSources = inlineSources
        self.mapRoot = mapRoot
        self.newLine = newLine
        self.noEmit = noEmit
        self.noEmitHelpers = noEmitHelpers
        self.noEmitOnError = noEmitOnError
        self.outDir = outDir
        self.outFile = outFile
        self.preserveConstEnums = preserveConstEnums
        self.preserveValueImports = preserveValueImports
        self.removeComments = removeComments
        self.sourceMap = sourceMap
        self.sourceRoot = sourceRoot
        self.stripInternal = stripInternal

        // JavaScript Support
        self.allowJs = allowJs
        self.checkJs = checkJs
        self.maxNodeModuleJsDepth = maxNodeModuleJsDepth

        // Editor Support
        self.disableSizeLimit = disableSizeLimit
        self.plugins = plugins

        // Interop Constraints
        self.allowSyntheticDefaultImports_legacy = allowSyntheticDefaultImports_legacy
        self.esModuleInterop = esModuleInterop
        self.forceConsistentCasingInFileNames = forceConsistentCasingInFileNames
        self.isolatedModules = isolatedModules
        self.preserveSymlinks = preserveSymlinks
        self.verbatimModuleSyntax = verbatimModuleSyntax

        // Backwards Compatibility
        self.charset = charset
        self.keyofStringsOnly = keyofStringsOnly
        self.noImplicitUseStrict = noImplicitUseStrict
        self.noStrictGenericChecks = noStrictGenericChecks
        self.out = out
        self.suppressExcessPropertyErrors = suppressExcessPropertyErrors
        self.suppressImplicitAnyIndexErrors = suppressImplicitAnyIndexErrors

        // Language and Environment
        self.emitDecoratorMetadata_legacy = emitDecoratorMetadata_legacy
        self.experimentalDecorators_legacy = experimentalDecorators_legacy
        self.jsx = jsx
        self.jsxFactory = jsxFactory
        self.jsxFragmentFactory = jsxFragmentFactory
        self.jsxImportSource = jsxImportSource
        self.lib = lib
        self.moduleDetection = moduleDetection
        self.noLib = noLib
        self.reactNamespace = reactNamespace
        self.target = target
        self.useDefineForClassFields = useDefineForClassFields

        // Compiler Diagnostics
        self.diagnostics = diagnostics
        self.explainFiles = explainFiles
        self.extendedDiagnostics = extendedDiagnostics
        self.generateCpuProfile = generateCpuProfile
        self.listEmittedFiles = listEmittedFiles
        self.listFiles = listFiles
        self.traceResolution = traceResolution

        // Projects
        self.composite = composite
        self.disableReferencedProjectLoad = disableReferencedProjectLoad
        self.disableSolutionSearching = disableSolutionSearching
        self.disableSourceOfProjectReferenceRedirect = disableSourceOfProjectReferenceRedirect
        self.incremental = incremental
        self.tsBuildInfoFile = tsBuildInfoFile

        // Output Formatting
        self.noErrorTruncation = noErrorTruncation
        self.preserveWatchOutput = preserveWatchOutput
        self.pretty = pretty

        // Completeness
        self.skipDefaultLibCheck = skipDefaultLibCheck
        self.skipLibCheck = skipLibCheck
    }
}

// MARK: - Project Reference

public struct ProjectReference: Codable, Sendable {
    public var path: String
    public var prepend: Bool?
    public var circular: Bool?

    public init(circular: Bool? = nil, path: String, prepend: Bool? = nil) {
        self.circular = circular
        self.path = path
        self.prepend = prepend
    }
}

// MARK: - Type Acquisition

public struct TypeAcquisition: Codable, Sendable {
    public var enable: Bool?
    public var include: [String]?
    public var exclude: [String]?
    public var disableFilenameBasedTypeAcquisition: Bool?

    public init(
        disableFilenameBasedTypeAcquisition: Bool? = nil,
        enable: Bool? = nil,
        exclude: [String]? = nil,
        include: [String]? = nil
    ) {
        self.disableFilenameBasedTypeAcquisition = disableFilenameBasedTypeAcquisition
        self.enable = enable
        self.exclude = exclude
        self.include = include
    }
}

// MARK: - Watch Options

public struct WatchOptions: Codable, Sendable {
    public var watchFile: String?
    public var watchDirectory: String?
    public var fallbackPolling: String?
    public var synchronousWatchDirectory: Bool?
    public var excludeDirectories: [String]?
    public var excludeFiles: [String]?

    public init(
        excludeDirectories: [String]? = nil,
        excludeFiles: [String]? = nil,
        fallbackPolling: String? = nil,
        synchronousWatchDirectory: Bool? = nil,
        watchDirectory: String? = nil,
        watchFile: String? = nil
    ) {
        self.excludeDirectories = excludeDirectories
        self.excludeFiles = excludeFiles
        self.fallbackPolling = fallbackPolling
        self.synchronousWatchDirectory = synchronousWatchDirectory
        self.watchDirectory = watchDirectory
        self.watchFile = watchFile
    }
}

// MARK: - Main TSConfig Structure

public struct TSConfig: Codable, Sendable {
    public var compilerOptions: CompilerOptions?
    public var files: [String]?
    public var include: [String]?
    public var exclude: [String]?
    public var extends: String?
    public var references: [ProjectReference]?
    public var typeAcquisition: TypeAcquisition?
    public var watchOptions: WatchOptions?

    public init(
        compilerOptions: CompilerOptions? = nil,
        exclude: [String]? = nil,
        extends: String? = nil,
        files: [String]? = nil,
        include: [String]? = nil,
        references: [ProjectReference]? = nil,
        typeAcquisition: TypeAcquisition? = nil,
        watchOptions: WatchOptions? = nil
    ) {
        self.compilerOptions = compilerOptions
        self.exclude = exclude
        self.extends = extends
        self.files = files
        self.include = include
        self.references = references
        self.typeAcquisition = typeAcquisition
        self.watchOptions = watchOptions
    }
}

// MARK: - Convenience Extensions

public extension TSConfig {
    /// Creates a default TypeScript configuration suitable for most projects
    static var `default`: TSConfig {
        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.outDir = "./dist"
        compilerOptions.rootDir = "./src"
        compilerOptions.strict = true
        compilerOptions.esModuleInterop = true
        compilerOptions.skipLibCheck = true
        compilerOptions.forceConsistentCasingInFileNames = true

        return TSConfig(
            compilerOptions: compilerOptions,
            exclude: ["node_modules", "dist"],
            include: ["src/**/*"]
        )
    }

    /// Creates a configuration optimized for Node.js projects
    static var nodeProject: TSConfig {
        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.outDir = "./dist"
        compilerOptions.rootDir = "./src"
        compilerOptions.strict = true
        compilerOptions.esModuleInterop = true
        compilerOptions.skipLibCheck = true
        compilerOptions.forceConsistentCasingInFileNames = true
        compilerOptions.declaration = true
        compilerOptions.sourceMap = true
        compilerOptions.resolveJsonModule = true

        return TSConfig(
            compilerOptions: compilerOptions,
            exclude: ["node_modules", "dist", "**/*.test.ts", "**/*.spec.ts"],
            include: ["src/**/*"]
        )
    }

    /// Creates a configuration optimized for React projects
    static var reactProject: TSConfig {
        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.lib = ["dom", "dom.iterable", "es6"]
        compilerOptions.allowJs = true
        compilerOptions.skipLibCheck = true
        compilerOptions.esModuleInterop = true
        compilerOptions.allowSyntheticDefaultImports = true
        compilerOptions.strict = true
        compilerOptions.forceConsistentCasingInFileNames = true
        compilerOptions.noFallthroughCasesInSwitch = true
        compilerOptions.module = .esnext
        compilerOptions.moduleResolution = .node
        compilerOptions.resolveJsonModule = true
        compilerOptions.isolatedModules = true
        compilerOptions.noEmit = true
        compilerOptions.jsx = .reactJSX

        return TSConfig(
            compilerOptions: compilerOptions,
            exclude: ["node_modules"],
            include: ["src"]
        )
    }

    /// Converts the configuration to JSON string
    func toJSONString(prettyPrinted: Bool = true) throws -> String {
        let encoder = JSONEncoder()
        if prettyPrinted {
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        }
        let data = try encoder.encode(self)
        return String(data: data, encoding: .utf8) ?? ""
    }
}
