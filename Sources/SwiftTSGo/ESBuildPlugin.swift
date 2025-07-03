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
        file: String,
        namespace: String,
        line: Int,
        column: Int,
        length: Int,
        lineText: String
    ) {
        self.file = file
        self.namespace = namespace
        self.line = line
        self.column = column
        self.length = length
        self.lineText = lineText
    }
}

// MARK: - Message

public struct ESBuildPluginMessage {
    public let text: String
    public let location: ESBuildPluginLocation?
    public let detail: Any?
    
    public init(
        text: String,
        location: ESBuildPluginLocation? = nil,
        detail: Any? = nil
    ) {
        self.text = text
        self.location = location
        self.detail = detail
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
        path: String,
        importer: String,
        namespace: String,
        resolveDir: String,
        kind: ESBuildPluginResolveKind,
        pluginData: Any? = nil,
        with: [String: String] = [:]
    ) {
        self.path = path
        self.importer = importer
        self.namespace = namespace
        self.resolveDir = resolveDir
        self.kind = kind
        self.pluginData = pluginData
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
        path: String,
        namespace: String,
        suffix: String = "",
        pluginData: Any? = nil,
        with: [String: String] = [:]
    ) {
        self.path = path
        self.namespace = namespace
        self.suffix = suffix
        self.pluginData = pluginData
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
        path: String? = nil,
        namespace: String? = nil,
        external: Bool? = nil,
        sideEffects: Bool? = nil,
        suffix: String? = nil,
        pluginData: Any? = nil,
        pluginName: String? = nil,
        errors: [ESBuildPluginMessage] = [],
        warnings: [ESBuildPluginMessage] = [],
        watchFiles: [String] = [],
        watchDirs: [String] = []
    ) {
        self.path = path
        self.namespace = namespace
        self.external = external
        self.sideEffects = sideEffects
        self.suffix = suffix
        self.pluginData = pluginData
        self.pluginName = pluginName
        self.errors = errors
        self.warnings = warnings
        self.watchFiles = watchFiles
        self.watchDirs = watchDirs
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
        loader: ESBuildLoader? = nil,
        resolveDir: String? = nil,
        pluginData: Any? = nil,
        pluginName: String? = nil,
        errors: [ESBuildPluginMessage] = [],
        warnings: [ESBuildPluginMessage] = [],
        watchFiles: [String] = [],
        watchDirs: [String] = []
    ) {
        self.contents = contents
        self.loader = loader
        self.resolveDir = resolveDir
        self.pluginData = pluginData
        self.pluginName = pluginName
        self.errors = errors
        self.warnings = warnings
        self.watchFiles = watchFiles
        self.watchDirs = watchDirs
    }
    
    public init(
        contents: String,
        loader: ESBuildLoader? = nil,
        resolveDir: String? = nil,
        pluginData: Any? = nil,
        pluginName: String? = nil,
        errors: [ESBuildPluginMessage] = [],
        warnings: [ESBuildPluginMessage] = [],
        watchFiles: [String] = [],
        watchDirs: [String] = []
    ) {
        self.init(
            contents: contents.data(using: .utf8),
            loader: loader,
            resolveDir: resolveDir,
            pluginData: pluginData,
            pluginName: pluginName,
            errors: errors,
            warnings: warnings,
            watchFiles: watchFiles,
            watchDirs: watchDirs
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
        namespace: String? = nil,
        resolveDir: String? = nil,
        kind: ESBuildPluginResolveKind? = nil,
        pluginData: Any? = nil
    ) {
        self.importer = importer
        self.namespace = namespace
        self.resolveDir = resolveDir
        self.kind = kind
        self.pluginData = pluginData
    }
}

// MARK: - Default Plugins

extension ESBuildPlugin {
    public static func reactGlobalTransform(globalName: String = "_FLICKCORE_$REACT") -> ESBuildPlugin {
        return ESBuildPlugin(name: "react-global-transform") { build in
            build.onResolve(filter: "^react$", namespace: nil) { args in
                return ESBuildOnResolveResult(
                    path: "react",
                    namespace: "use-flick-react-global"
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
        path: String,
        namespace: String = "file",
        suffix: String = "",
        external: Bool = false,
        sideEffects: Bool = false,
        pluginData: Any? = nil,
        errors: [ESBuildPluginMessage] = [],
        warnings: [ESBuildPluginMessage] = []
    ) {
        self.path = path
        self.namespace = namespace
        self.suffix = suffix
        self.external = external
        self.sideEffects = sideEffects
        self.pluginData = pluginData
        self.errors = errors
        self.warnings = warnings
    }
}