import Foundation
import TSGoBindings

// Configuration options for TypeScript compilation
public struct FileSystemBuildConfig {
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
public struct FileSystemBuildResult {
    public let success: Bool
    public let diagnostics: [DiagnosticInfo]
    public let emittedFiles: [String]
    public let configFile: String

    public init(
        success: Bool, diagnostics: [DiagnosticInfo] = [], emittedFiles: [String] = [],
        configFile: String = ""
    ) {
        self.success = success
        self.diagnostics = diagnostics
        self.emittedFiles = emittedFiles
        self.configFile = configFile
    }
}

// Structured function that returns detailed results
public func build(_ config: FileSystemBuildConfig) -> FileSystemBuildResult {
    var error: NSError?
    let bridgeResult = BridgeBridgeBuildWithConfig(
        config.projectPath, config.printErrors, config.configFile ?? "", &error
    )

    var diagnostics: [DiagnosticInfo] = []
    var emittedFiles: [String] = []

    // Process result even if there's an error (compilation failures still return results with diagnostics)
    if let result = bridgeResult {
        // Retrieve diagnostics
        for i in 0..<result.diagnosticCount {
            if let bridgeDiag = BridgeGetLastDiagnostic(i) {
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

        // Retrieve emitted files
        for i in 0..<result.emittedFileCount {
            let file = BridgeGetLastEmittedFile(i)
            if !file.isEmpty {
                emittedFiles.append(file)
            }
        }

        return FileSystemBuildResult(
            success: result.success,
            diagnostics: diagnostics,
            emittedFiles: emittedFiles,
            configFile: result.configFile
        )
    }

    // Handle case where result is nil (should be rare)
    if let error = error {
        let diagnostic = DiagnosticInfo(
            code: 0,
            category: "error",
            message: error.localizedDescription
        )
        return FileSystemBuildResult(success: false, diagnostics: [diagnostic])
    }

    return FileSystemBuildResult(success: false)
}
