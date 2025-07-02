import Foundation
import TSCBridge

// MARK: - C String Helpers

private func withMutableCString<T>(_ string: String, _ body: (UnsafeMutablePointer<CChar>) -> T)
    -> T
{
    return string.withCString { cString in
        let mutableCString = strdup(cString)!
        defer { free(mutableCString) }
        return body(mutableCString)
    }
}

// MARK: - File System Build Functions

/// Build TypeScript project from filesystem
/// - Parameters:
///   - projectPath: Path to the TypeScript project directory or tsconfig.json file
///   - printErrors: Whether to print compilation errors to stdout
///   - configFile: Optional custom config file path
/// - Returns: Build result with compilation status and diagnostics
/// - Throws: BuildError if the build process fails at system level
public func buildFromFileSystem(
    projectPath: String,
    printErrors: Bool = false,
    configFile: String = ""
) throws -> InMemoryBuildResult {
    let cResult = withMutableCString(projectPath) { projectPathPtr in
        withMutableCString(configFile) { configFilePtr in
            tsc_build_filesystem(projectPathPtr, printErrors ? 1 : 0, configFilePtr)
        }
    }
    return processFileSystemResult(cResult)
}

/// Build TypeScript project from filesystem with detailed configuration
/// - Parameters:
///   - projectPath: Path to the TypeScript project directory or tsconfig.json file
///   - options: Build options including error printing and config file
/// - Returns: Build result with compilation status and diagnostics
/// - Throws: BuildError if the build process fails at system level
public func buildFromFileSystem(
    projectPath: String,
    options: BuildOptions
) throws -> InMemoryBuildResult {
    let cResult = withMutableCString(projectPath) { projectPathPtr in
        withMutableCString(options.configFile ?? "") { configFilePtr in
            tsc_build_filesystem(projectPathPtr, options.printErrors ? 1 : 0, configFilePtr)
        }
    }
    return processFileSystemResult(cResult)
}

// MARK: - Build Options

public struct BuildOptions: Sendable {
    public let printErrors: Bool
    public let configFile: String?
    public let workingDirectory: String?

    public init(
        printErrors: Bool = false,
        configFile: String? = nil,
        workingDirectory: String? = nil
    ) {
        self.printErrors = printErrors
        self.configFile = configFile
        self.workingDirectory = workingDirectory
    }

    public static let `default` = BuildOptions()
}

// MARK: - Build Errors

public enum BuildError: LocalizedError {
    case systemError(String)
    case configurationError(String)
    case fileNotFound(String)
    case invalidPath(String)

    public var errorDescription: String? {
        switch self {
        case .systemError(let message):
            return "System error: \(message)"
        case .configurationError(let message):
            return "Configuration error: \(message)"
        case .fileNotFound(let path):
            return "File not found: \(path)"
        case .invalidPath(let path):
            return "Invalid path: \(path)"
        }
    }
}

// MARK: - Helper Functions

private func processFileSystemResult(_ cResult: UnsafeMutablePointer<c_build_result>?)
    -> InMemoryBuildResult
{
    guard let cResult = cResult else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Build failed with no result"
                )
            ]
        )
    }

    defer { tsc_free_result(cResult) }

    let success = cResult.pointee.success != 0
    let configFile =
        cResult.pointee.config_file != nil ? String(cString: cResult.pointee.config_file) : ""

    // Handle error cases where config_file contains error message
    if configFile.hasPrefix("error: ") {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: String(configFile.dropFirst(7))  // Remove "error: " prefix
                )
            ]
        )
    }

    let diagnostics = convertCDiagnostics(
        cResult.pointee.diagnostics,
        count: Int(cResult.pointee.diagnostic_count)
    )

    let _ = convertCEmittedFiles(
        cResult.pointee.emitted_files,
        count: Int(cResult.pointee.emitted_file_count)
    )

    let writtenFiles = convertCWrittenFiles(
        cResult.pointee.written_file_paths,
        cResult.pointee.written_file_contents,
        count: Int(cResult.pointee.written_file_count)
    )

    let compiledFiles = convertCCompiledFiles(from: writtenFiles)

    return InMemoryBuildResult(
        success: success,
        diagnostics: diagnostics,
        compiledFiles: compiledFiles,
        configFile: configFile,
        writtenFiles: writtenFiles
    )
}

private func convertCDiagnostics(_ cDiagnostics: UnsafeMutablePointer<c_diagnostic>?, count: Int)
    -> [DiagnosticInfo]
{
    guard let cDiagnostics = cDiagnostics, count > 0 else { return [] }

    var diagnostics: [DiagnosticInfo] = []
    for i in 0..<count {
        let cDiag = cDiagnostics[i]
        let diagnostic = DiagnosticInfo(
            code: Int(cDiag.code),
            category: String(cString: cDiag.category),
            message: String(cString: cDiag.message),
            file: String(cString: cDiag.file),
            line: Int(cDiag.line),
            column: Int(cDiag.column),
            length: Int(cDiag.length)
        )
        diagnostics.append(diagnostic)
    }
    return diagnostics
}

private func convertCEmittedFiles(
    _ files: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>?,
    count: Int
) -> [String] {
    guard let files = files, count > 0 else { return [] }

    var emittedFiles: [String] = []
    for i in 0..<count {
        if let filePtr = files[i] {
            let filePath = String(cString: filePtr)
            emittedFiles.append(filePath)
        }
    }
    return emittedFiles
}

private func convertCWrittenFiles(
    _ paths: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>?,
    _ contents: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>?,
    count: Int
) -> [String: String] {
    guard let paths = paths, let contents = contents, count > 0 else { return [:] }

    var writtenFiles: [String: String] = [:]
    for i in 0..<count {
        if let pathPtr = paths[i], let contentPtr = contents[i] {
            let path = String(cString: pathPtr)
            let content = String(cString: contentPtr)
            writtenFiles[path] = content
        }
    }
    return writtenFiles
}

private func convertCCompiledFiles(from writtenFiles: [String: String]) -> [Source] {
    return writtenFiles.map { (path, content) in
        let filename = (path as NSString).lastPathComponent
        return Source(name: filename, content: content)
    }
}

// MARK: - Convenience Functions

/// Validate TypeScript project configuration without compilation
/// - Parameter projectPath: Path to the TypeScript project directory or tsconfig.json file
/// - Returns: Build result with validation status and configuration diagnostics only
/// - Throws: BuildError if validation fails at system level
public func validateProject(projectPath: String) throws -> InMemoryBuildResult {
    // Use noEmit option to validate without generating output
    let tempConfigContent = """
        {
            "extends": "./tsconfig.json",
            "compilerOptions": {
                "noEmit": true
            }
        }
        """

    // Create temporary config file path
    let tempDir = NSTemporaryDirectory()
    let tempConfigPath = "\(tempDir)/tsconfig.validate.json"

    // Write temporary config
    try tempConfigContent.write(toFile: tempConfigPath, atomically: true, encoding: .utf8)
    defer {
        try? FileManager.default.removeItem(atPath: tempConfigPath)
    }

    return try buildFromFileSystem(projectPath: projectPath, configFile: tempConfigPath)
}

/// Check if a TypeScript project has compilation errors
/// - Parameter projectPath: Path to the TypeScript project directory or tsconfig.json file
/// - Returns: True if the project compiles without errors, false otherwise
public func hasCompilationErrors(projectPath: String) -> Bool {
    do {
        let result = try buildFromFileSystem(projectPath: projectPath)
        return !result.success || result.diagnostics.contains { $0.category == "error" }
    } catch {
        return true
    }
}

/// Get compilation diagnostics for a TypeScript project
/// - Parameter projectPath: Path to the TypeScript project directory or tsconfig.json file
/// - Returns: Array of diagnostics, or empty array if compilation fails
public func getCompilationDiagnostics(projectPath: String) -> [DiagnosticInfo] {
    do {
        let result = try buildFromFileSystem(projectPath: projectPath)
        return result.diagnostics
    } catch {
        return [
            DiagnosticInfo(
                code: 0,
                category: "error",
                message: "Failed to get diagnostics: \(error.localizedDescription)"
            )
        ]
    }
}
