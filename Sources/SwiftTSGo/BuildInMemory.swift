import Foundation
import TSCBridge

// MARK: - Core Types

public struct Source: Sendable {
    public let name: String
    public let content: String

    public init(name: String, content: String) {
        self.name = name
        self.content = content
    }
}

public struct InMemoryBuildResult: Sendable {
    public let success: Bool
    public let diagnostics: [DiagnosticInfo]
    public let compiledFiles: [Source]
    public let configFile: String
    public let writtenFiles: [String: String]

    public init(
        success: Bool,
        diagnostics: [DiagnosticInfo] = [],
        compiledFiles: [Source] = [],
        configFile: String = "",
        writtenFiles: [String: String] = [:]
    ) {
        self.success = success
        self.diagnostics = diagnostics
        self.compiledFiles = compiledFiles
        self.configFile = configFile
        self.writtenFiles = writtenFiles
    }
}

public enum FileResolver: Sendable {
    case file(String)
    case directory([String])
}

// MARK: - Public API

public func build(
    config: TSConfig = .default,
    _ sourceFiles: [Source]
) async throws -> InMemoryBuildResult {
    try await build(config: config) { path in
        if path == "/project" {
            return .directory(["src"])
        }
        if path == "/project/src" {
            return .directory(sourceFiles.map(\.name))
        }
        if let source = sourceFiles.first(where: { ("/project/src/" + $0.name) == path }) {
            return .file(source.content)
        }
        return nil
    }
}

/// Build TypeScript files with a custom file resolver
/// - Parameters:
///   - config: TypeScript configuration (defaults to .default)
///   - resolver: Function that resolves file paths to FileResolver cases or nil
/// - Returns: Build result with compilation status and diagnostics
public func build(
    config: TSConfig = .default,
    resolver: @escaping @Sendable (String) async throws -> FileResolver?
) async throws -> InMemoryBuildResult {
    let projectPath = "/project"

    // Create a wrapper resolver that handles tsconfig.json generation and delegation
    let wrapperResolver: @Sendable (String) async throws -> FileResolver? = { path in

        // 1) Exact match for tsconfig.json - try resolver first, then fallback to config param
        if path == "/project/tsconfig.json" {
            // First try the provided resolver
            if let resolverResult = try await resolver(path) {
                return resolverResult
            }

            // Fallback: generate from config param
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
            let configData = try encoder.encode(config)
            let configString = String(data: configData, encoding: .utf8) ?? "{}"
            return .file(configString)
        }

        // 2) Root directory - call resolver and ensure tsconfig.json is included
        if path == "/project" {
            var files: [String] = []

            if let resolved = try await resolver(path) {
                if case let .directory(upstreamFiles) = resolved {
                    files = upstreamFiles
                }
            }

            if !files.contains("tsconfig.json") {
                files.append("tsconfig.json")
            }

            return .directory(files)
        }

        // 3) Everything else - delegate to the resolver param
        return try await resolver(path)
    }

    // Use dynamic resolver with callback-based C bridge (like ESBuild plugins)
    let cResult = buildWithDynamicResolver(
        projectPath: projectPath,
        printErrors: false,
        configFile: "",
        resolver: wrapperResolver
    )

    guard let cResult else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Failed to build with dynamic resolver"
                ),
            ]
        )
    }

    return processResult(cResult)
}
