import Foundation
import TSGoBindings

// MARK: - Core Types

public struct Source {
    public let name: String
    public let content: String

    public init(name: String, content: String) {
        self.name = name
        self.content = content
    }
}

public struct InMemoryBuildResult {
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

public enum FileResolver {
    case file(String)
    case directory
}

// MARK: - Internal File Resolver Adapter

private class DynamicFileResolver: NSObject, BridgeFileResolverProtocol {
    private let resolver: @Sendable (String) async throws -> FileResolver?
    private var writtenFiles: [String: String] = [:]

    init(resolver: @escaping @Sendable (String) async throws -> FileResolver?) {
        self.resolver = resolver
        super.init()
    }

    func getWrittenFiles() -> [String: String] {
        return writtenFiles
    }

    func resolveFile(_ path: String?) -> String {
        guard let path = path else { return "" }

        let semaphore = DispatchSemaphore(value: 0)
        var result = ""

        Task { [resolver] in
            do {
                if let fileResolver = try await resolver(path) {
                    switch fileResolver {
                    case .file(let content):
                        result = content
                    case .directory:
                        result = ""
                    }
                }
            } catch {
                // On error, return empty string
                result = ""
            }
            semaphore.signal()
        }

        semaphore.wait()
        return result
    }

    func fileExists(_ path: String?) -> Bool {
        guard let path = path else { return false }

        let semaphore = DispatchSemaphore(value: 0)
        var result = false

        Task { [resolver] in
            do {
                if let fileResolver = try await resolver(path) {
                    switch fileResolver {
                    case .file:
                        result = true
                    case .directory:
                        result = false
                    }
                }
            } catch {
                // On error, return false
                result = false
            }
            semaphore.signal()
        }

        semaphore.wait()
        return result
    }

    func directoryExists(_ path: String?) -> Bool {
        guard let path = path else { return false }

        let semaphore = DispatchSemaphore(value: 0)
        var result = false

        Task { [resolver] in
            do {
                if let fileResolver = try await resolver(path) {
                    switch fileResolver {
                    case .directory:
                        result = true
                    case .file:
                        result = false
                    }
                }
            } catch {
                // On error, return false
                result = false
            }
            semaphore.signal()
        }

        semaphore.wait()
        return result
    }

    func writeFile(_ path: String?, content: String?) -> Bool {
        // Simply return true to allow Go-side file capture
        // Avoid Swift-side file handling due to gomobile interop issues
        return true
    }
}

// MARK: - Result Processing Helpers

private func extractDiagnostics(from bridgeResult: BridgeBridgeResult) -> [DiagnosticInfo] {
    var diagnostics: [DiagnosticInfo] = []
    let count = bridgeResult.getDiagnosticCount()

    for i in 0..<count {
        if let bridgeDiag = bridgeResult.getDiagnostic(i) {
            let diagnostic = DiagnosticInfo(
                code: Int(bridgeDiag.code),
                category: bridgeDiag.category,
                message: bridgeDiag.message,
                file: bridgeDiag.file,
                line: Int(bridgeDiag.line),
                column: Int(bridgeDiag.column),
                length: Int(bridgeDiag.length)
            )
            diagnostics.append(diagnostic)
        }
    }

    return diagnostics
}

private func extractWrittenFiles(from bridgeResult: BridgeBridgeResult) -> [String: String] {
    var writtenFiles: [String: String] = [:]
    let count = bridgeResult.getWrittenFileCount()

    for i in 0..<count {
        let path = bridgeResult.getWrittenFilePath(i)
        if !path.isEmpty {
            let content = bridgeResult.getWrittenFileContent(path)
            writtenFiles[path] = content
        }
    }

    return writtenFiles
}

private func extractCompiledFiles(
    from bridgeResult: BridgeBridgeResult, writtenFiles: [String: String]
) -> [Source] {
    var compiledFiles: [Source] = []

    // Convert written files to Source objects
    for (path, content) in writtenFiles {
        // Extract just the filename from the path
        let filename = (path as NSString).lastPathComponent
        compiledFiles.append(Source(name: filename, content: content))
    }

    return compiledFiles
}

// MARK: - Public API

/// Build TypeScript files with a custom file resolver
/// - Parameters:
///   - config: TypeScript configuration (optional, uses default if nil)
///   - resolver: Async function that resolves file paths to FileResolver cases or nil
/// - Returns: Build result with compilation status and diagnostics
/// - Throws: BridgeError if the build process fails
public func build(
    config: TSConfig? = nil,
    resolver: @escaping @Sendable (String) async throws -> FileResolver?
) async throws -> InMemoryBuildResult {
    let projectPath = "/project"
    let tsConfig = config ?? TSConfig.default

    let capturedConfig = tsConfig  // Capture config to avoid Sendable issues
    let fileResolver = DynamicFileResolver(resolver: { path in
        // Handle tsconfig.json specially
        if path == "\(projectPath)/tsconfig.json" {
            do {
                let encoder = JSONEncoder()
                encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
                let configData = try encoder.encode(capturedConfig)
                return .file(String(data: configData, encoding: .utf8) ?? "{}")
            } catch {
                return nil
            }
        }

        return try await resolver(path)
    })

    // Build with the new clean API
    var error: NSError?
    let bridgeResult = BridgeBuildWithFileResolver(
        projectPath,
        false,
        "",
        fileResolver,
        &error
    )

    // Handle system-level errors
    if let error = error {
        let diagnostic = DiagnosticInfo(
            code: 0,
            category: "error",
            message: error.localizedDescription
        )
        return InMemoryBuildResult(
            success: false,
            diagnostics: [diagnostic]
        )
    }

    guard let result = bridgeResult else {
        let diagnostic = DiagnosticInfo(
            code: 0,
            category: "error",
            message: "Build failed with no result"
        )
        return InMemoryBuildResult(
            success: false,
            diagnostics: [diagnostic]
        )
    }

    // Extract results using the new API
    let diagnostics = extractDiagnostics(from: result)
    let writtenFiles = extractWrittenFiles(from: result)
    let compiledFiles = extractCompiledFiles(from: result, writtenFiles: writtenFiles)

    return InMemoryBuildResult(
        success: result.success,
        diagnostics: diagnostics,
        compiledFiles: compiledFiles,
        configFile: result.configFile,
        writtenFiles: writtenFiles
    )
}

/// Build TypeScript files from a known set of source files
/// - Parameters:
///   - sourceFiles: Array of source files to compile
///   - config: TypeScript configuration (optional, uses default if nil)
/// - Returns: Build result with compilation status and diagnostics
/// - Throws: BridgeError if the build process fails
public func build(
    _ sourceFiles: [Source],
    config: TSConfig? = nil
) async throws -> InMemoryBuildResult {
    let projectPath = "/project"

    // Create a file map for quick lookup - use both absolute and relative paths
    let fileMap: [String: String] = {
        var map: [String: String] = [:]
        for source in sourceFiles {
            let absolutePath = "\(projectPath)/\(source.name)"
            let relativePath = source.name

            // Store with both absolute and relative paths
            map[absolutePath] = source.content
            map[relativePath] = source.content
            map["/\(source.name)"] = source.content
        }
        return map
    }()

    let knownDirectories: Set<String> = {
        var directories = Set<String>([projectPath, "/"])

        // Add source files and their directories
        for source in sourceFiles {
            let absolutePath = "\(projectPath)/\(source.name)"
            let relativePath = source.name

            // Track parent directories for absolute path
            let parentDir = (absolutePath as NSString).deletingLastPathComponent
            if parentDir != projectPath {
                directories.insert(parentDir)

                // Add intermediate directories
                var currentPath = parentDir
                while currentPath != projectPath && currentPath != "/" {
                    directories.insert(currentPath)
                    currentPath = (currentPath as NSString).deletingLastPathComponent
                }
            }

            // Track parent directories for relative path
            let relativeParentDir = (relativePath as NSString).deletingLastPathComponent
            if relativeParentDir != "." && relativeParentDir != "" {
                directories.insert(relativeParentDir)
                directories.insert("/\(relativeParentDir)")
                directories.insert("\(projectPath)/\(relativeParentDir)")
            }
        }
        return directories
    }()

    // Create resolver that uses the provided source files
    let resolver: @Sendable (String) async throws -> FileResolver? = { path in
        // Check for exact file match
        if let content = fileMap[path] {
            return .file(content)
        }

        // Check if it's a known directory
        if knownDirectories.contains(path) {
            return .directory
        }

        // Handle output directories (dist, etc.) as existing directories
        if path.contains("/dist") || path.hasSuffix("/dist") || path == "dist"
            || path.hasPrefix("\(projectPath)/dist")
        {
            return .directory
        }

        // Also handle root project directory
        if path == projectPath {
            return .directory
        }

        // Check if any files are under this directory path
        let isDirectory = fileMap.keys.contains { filePath in
            filePath.hasPrefix(path + "/")
                || filePath.hasPrefix(path.trimmingCharacters(in: CharacterSet(charactersIn: "/")))
        }

        return isDirectory ? .directory : nil
    }

    // Modify the config to explicitly include the files we have
    let tsConfig: TSConfig
    if let providedConfig = config {
        var modifiedConfig = providedConfig
        // If no files or include patterns are specified, add our files
        if modifiedConfig.files == nil && modifiedConfig.include == nil {
            modifiedConfig.files = sourceFiles.map { $0.name }
        }
        tsConfig = modifiedConfig
    } else {
        // Use default config with explicit file list
        var defaultConfig = TSConfig.default
        defaultConfig.files = sourceFiles.map { $0.name }
        if defaultConfig.compilerOptions == nil {
            defaultConfig.compilerOptions = CompilerOptions()
        }
        tsConfig = defaultConfig
    }

    return try await build(config: tsConfig, resolver: resolver)
}
