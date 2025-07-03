import Foundation

// MARK: - ResolveKind

public enum ESBuildPluginResolveKind: CaseIterable {
    case entryPoint
    case importStatement
    case requireCall
    case dynamicImport
    case requireResolve
    case importRule
    case composesFrom
    case urlToken
    
    var cValue: Int32 {
        switch self {
        case .entryPoint: return 0
        case .importStatement: return 1
        case .requireCall: return 2
        case .dynamicImport: return 3
        case .requireResolve: return 4
        case .importRule: return 5
        case .composesFrom: return 6
        case .urlToken: return 7
        }
    }
    
    init?(cValue: Int32) {
        switch cValue {
        case 0: self = .entryPoint
        case 1: self = .importStatement
        case 2: self = .requireCall
        case 3: self = .dynamicImport
        case 4: self = .requireResolve
        case 5: self = .importRule
        case 6: self = .composesFrom
        case 7: self = .urlToken
        default: return nil
        }
    }
}

// MARK: - Location

public struct ESBuildPluginLocation {
    public let file: String
    public let namespace: String
    public let line: Int // 1-based
    public let column: Int // 0-based
    public let length: Int
    public let lineText: String
    
    public init(
        column: Int,
        file: String,
        length: Int,
        line: Int,
        lineText: String,
        namespace: String
    ) {
        self.column = column
        self.file = file
        self.length = length
        self.line = line
        self.lineText = lineText
        self.namespace = namespace
    }
}

// MARK: - Message

public struct ESBuildPluginMessage {
    public let text: String
    public let location: ESBuildPluginLocation?
    public let detail: Any?
    
    public init(
        detail: Any? = nil,
        location: ESBuildPluginLocation? = nil,
        text: String
    ) {
        self.detail = detail
        self.location = location
        self.text = text
    }
}

// MARK: - OnResolveArgs

public struct ESBuildOnResolveArgs {
    public let path: String
    public let importer: String
    public let namespace: String
    public let resolveDir: String
    public let kind: ESBuildPluginResolveKind
    public let pluginData: Any?
    public let with: [String: String]
    
    public init(
        importer: String,
        kind: ESBuildPluginResolveKind,
        namespace: String,
        path: String,
        pluginData: Any? = nil,
        resolveDir: String,
        with: [String: String] = [:]
    ) {
        self.importer = importer
        self.kind = kind
        self.namespace = namespace
        self.path = path
        self.pluginData = pluginData
        self.resolveDir = resolveDir
        self.with = with
    }
}

// MARK: - OnLoadArgs

public struct ESBuildOnLoadArgs {
    public let path: String
    public let namespace: String
    public let suffix: String
    public let pluginData: Any?
    public let with: [String: String]
    
    public init(
        namespace: String,
        path: String,
        pluginData: Any? = nil,
        suffix: String = "",
        with: [String: String] = [:]
    ) {
        self.namespace = namespace
        self.path = path
        self.pluginData = pluginData
        self.suffix = suffix
        self.with = with
    }
}

// MARK: - OnResolveResult

public struct ESBuildOnResolveResult {
    public let path: String?
    public let namespace: String?
    public let external: Bool?
    public let sideEffects: Bool?
    public let suffix: String?
    public let pluginData: Any?
    public let pluginName: String?
    public let errors: [ESBuildPluginMessage]
    public let warnings: [ESBuildPluginMessage]
    public let watchFiles: [String]
    public let watchDirs: [String]
    
    public init(
        errors: [ESBuildPluginMessage] = [],
        external: Bool? = nil,
        namespace: String? = nil,
        path: String? = nil,
        pluginData: Any? = nil,
        pluginName: String? = nil,
        sideEffects: Bool? = nil,
        suffix: String? = nil,
        warnings: [ESBuildPluginMessage] = [],
        watchDirs: [String] = [],
        watchFiles: [String] = []
    ) {
        self.errors = errors
        self.external = external
        self.namespace = namespace
        self.path = path
        self.pluginData = pluginData
        self.pluginName = pluginName
        self.sideEffects = sideEffects
        self.suffix = suffix
        self.warnings = warnings
        self.watchDirs = watchDirs
        self.watchFiles = watchFiles
    }
}

// MARK: - OnLoadResult

public struct ESBuildOnLoadResult {
    public let contents: Data?
    public let loader: ESBuildLoader?
    public let resolveDir: String?
    public let pluginData: Any?
    public let pluginName: String?
    public let errors: [ESBuildPluginMessage]
    public let warnings: [ESBuildPluginMessage]
    public let watchFiles: [String]
    public let watchDirs: [String]
    
    public init(
        contents: Data? = nil,
        errors: [ESBuildPluginMessage] = [],
        loader: ESBuildLoader? = nil,
        pluginData: Any? = nil,
        pluginName: String? = nil,
        resolveDir: String? = nil,
        warnings: [ESBuildPluginMessage] = [],
        watchDirs: [String] = [],
        watchFiles: [String] = []
    ) {
        self.contents = contents
        self.errors = errors
        self.loader = loader
        self.pluginData = pluginData
        self.pluginName = pluginName
        self.resolveDir = resolveDir
        self.warnings = warnings
        self.watchDirs = watchDirs
        self.watchFiles = watchFiles
    }
    
    public init(
        contents: String,
        errors: [ESBuildPluginMessage] = [],
        loader: ESBuildLoader? = nil,
        pluginData: Any? = nil,
        pluginName: String? = nil,
        resolveDir: String? = nil,
        warnings: [ESBuildPluginMessage] = [],
        watchDirs: [String] = [],
        watchFiles: [String] = []
    ) {
        self.init(
            contents: contents.data(using: .utf8),
            errors: errors,
            loader: loader,
            pluginData: pluginData,
            pluginName: pluginName,
            resolveDir: resolveDir,
            warnings: warnings,
            watchDirs: watchDirs,
            watchFiles: watchFiles
        )
    }
}

// MARK: - Plugin

public struct ESBuildPlugin {
    public let name: String
    public let setup: (ESBuildPluginBuild) -> Void
    
    public init(name: String, setup: @escaping (ESBuildPluginBuild) -> Void) {
        self.name = name
        self.setup = setup
    }
}

// MARK: - PluginBuild

public protocol ESBuildPluginBuild {
    func onResolve(
        filter: String,
        namespace: String?,
        callback: @escaping (ESBuildOnResolveArgs) -> ESBuildOnResolveResult?
    )
    
    func onLoad(
        filter: String,
        namespace: String?,
        callback: @escaping (ESBuildOnLoadArgs) -> ESBuildOnLoadResult?
    )
    
    func onStart(callback: @escaping () -> Void)
    func onEnd(callback: @escaping () -> Void)
    func onDispose(callback: @escaping () -> Void)
    
    func resolve(path: String, options: ESBuildResolveOptions) -> ESBuildResolveResult
}

// MARK: - ResolveOptions

public struct ESBuildResolveOptions {
    public let importer: String?
    public let namespace: String?
    public let resolveDir: String?
    public let kind: ESBuildPluginResolveKind?
    public let pluginData: Any?
    
    public init(
        importer: String? = nil,
        kind: ESBuildPluginResolveKind? = nil,
        namespace: String? = nil,
        pluginData: Any? = nil,
        resolveDir: String? = nil
    ) {
        self.importer = importer
        self.kind = kind
        self.namespace = namespace
        self.pluginData = pluginData
        self.resolveDir = resolveDir
    }
}

// MARK: - Default Plugins

extension ESBuildPlugin {
    public static func reactGlobalTransform(globalName: String = "_FLICKCORE_$REACT") -> ESBuildPlugin {
        return ESBuildPlugin(name: "react-global-transform") { build in
            build.onResolve(filter: "^react$", namespace: nil) { args in
                return ESBuildOnResolveResult(
                    namespace: "use-flick-react-global",
                    path: "react"
                )
            }
            
            build.onLoad(filter: ".*", namespace: "use-flick-react-global") { args in
                return ESBuildOnLoadResult(
                    contents: "module.exports = \(globalName)",
                    loader: .js
                )
            }
        }
    }
}

// MARK: - ResolveResult

public struct ESBuildResolveResult {
    public let path: String
    public let namespace: String
    public let suffix: String
    public let external: Bool
    public let sideEffects: Bool
    public let pluginData: Any?
    public let errors: [ESBuildPluginMessage]
    public let warnings: [ESBuildPluginMessage]
    
    public init(
        errors: [ESBuildPluginMessage] = [],
        external: Bool = false,
        namespace: String = "file",
        path: String,
        pluginData: Any? = nil,
        sideEffects: Bool = false,
        suffix: String = "",
        warnings: [ESBuildPluginMessage] = []
    ) {
        self.errors = errors
        self.external = external
        self.namespace = namespace
        self.path = path
        self.pluginData = pluginData
        self.sideEffects = sideEffects
        self.suffix = suffix
        self.warnings = warnings
    }
}