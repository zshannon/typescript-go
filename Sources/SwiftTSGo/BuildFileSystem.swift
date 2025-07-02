import Foundation
import TSGoBindings

// Configuration options for TypeScript compilation
public struct FileSystemBuildConfig: Sendable {
    public let projectPath: String
    public let printErrors: Bool
    public let configFile: String?

    public init(projectPath: String = ".", printErrors: Bool = false, configFile: String? = nil) {
        self.projectPath = projectPath
        self.printErrors = printErrors
        self.configFile = configFile
    }
}

// Result of a TypeScript compilation
public struct FileSystemBuildResult: Sendable {
    public let success: Bool
    public let diagnostics: [DiagnosticInfo]
    public let emittedFiles: [String]
    public let configFile: String
    public let writtenFiles: [String: String]

    public init(
        success: Bool,
        diagnostics: [DiagnosticInfo] = [],
        emittedFiles: [String] = [],
        configFile: String = "",
        writtenFiles: [String: String] = [:]
    ) {
        self.success = success
        self.diagnostics = diagnostics
        self.emittedFiles = emittedFiles
        self.configFile = configFile
        self.writtenFiles = writtenFiles
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

private func extractEmittedFiles(from bridgeResult: BridgeBridgeResult) -> [String] {
    var emittedFiles: [String] = []
    let count = bridgeResult.getEmittedFileCount()

    for i in 0..<count {
        let file = bridgeResult.getEmittedFile(i)
        if !file.isEmpty {
            emittedFiles.append(file)
        }
    }

    return emittedFiles
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

// MARK: - Public API

/// Build TypeScript files using the filesystem
/// - Parameter config: Build configuration
/// - Returns: Build result with compilation status and diagnostics
public func build(_ config: FileSystemBuildConfig) -> FileSystemBuildResult {
    var error: NSError?
    let bridgeResult = BridgeBuildWithFileSystem(
        config.projectPath,
        config.printErrors,
        config.configFile ?? "",
        &error
    )

    // Handle system-level errors
    if let error = error {
        let diagnostic = DiagnosticInfo(
            code: 0,
            category: "error",
            message: error.localizedDescription
        )
        return FileSystemBuildResult(
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
        return FileSystemBuildResult(
            success: false,
            diagnostics: [diagnostic]
        )
    }

    // Extract results using the new API
    let diagnostics = extractDiagnostics(from: result)
    let emittedFiles = extractEmittedFiles(from: result)
    let writtenFiles = extractWrittenFiles(from: result)

    return FileSystemBuildResult(
        success: result.success,
        diagnostics: diagnostics,
        emittedFiles: emittedFiles,
        configFile: result.configFile,
        writtenFiles: writtenFiles
    )
}
