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

// MARK: - C Bridge Helpers

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

private func processResult(_ cResult: UnsafeMutablePointer<c_build_result>?) -> InMemoryBuildResult
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
    let diagnostics = convertCDiagnostics(
        cResult.pointee.diagnostics, count: Int(cResult.pointee.diagnostic_count))
    let writtenFiles = convertCWrittenFiles(
        cResult.pointee.written_file_paths,
        cResult.pointee.written_file_contents,
        count: Int(cResult.pointee.written_file_count))
    let compiledFiles = convertCCompiledFiles(from: writtenFiles)

    return InMemoryBuildResult(
        success: success,
        diagnostics: diagnostics,
        compiledFiles: compiledFiles,
        configFile: configFile,
        writtenFiles: writtenFiles
    )
}

// MARK: - Public API

/// Validate a simple TypeScript code string
/// - Parameter code: TypeScript code to validate
/// - Returns: Build result with validation status and diagnostics
public func validateTypeScript(_ code: String) throws -> InMemoryBuildResult {
    let cResult = withMutableCString(code) { codePtr in
        tsc_validate_simple(codePtr)
    }
    defer { tsc_free_string(cResult) }

    guard let cResult = cResult else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Validation failed with no result"
                )
            ]
        )
    }

    let jsonString = String(cString: cResult)
    guard let jsonData = jsonString.data(using: String.Encoding.utf8),
        let response = try? JSONSerialization.jsonObject(with: jsonData) as? [String: Any]
    else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Failed to parse validation result"
                )
            ]
        )
    }

    let success = response["success"] as? Bool ?? false
    var diagnostics: [DiagnosticInfo] = []

    if let diagnosticsArray = response["diagnostics"] as? [[String: Any]] {
        diagnostics = diagnosticsArray.compactMap { diagDict in
            guard let code = diagDict["code"] as? Int,
                let category = diagDict["category"] as? String,
                let message = diagDict["message"] as? String
            else {
                return nil as DiagnosticInfo?
            }

            return DiagnosticInfo(
                code: code,
                category: category,
                message: message,
                file: diagDict["file"] as? String ?? "",
                line: diagDict["line"] as? Int ?? 0,
                column: diagDict["column"] as? Int ?? 0,
                length: diagDict["length"] as? Int ?? 0
            )
        }
    }

    if let error = response["error"] as? String {
        diagnostics.append(
            DiagnosticInfo(
                code: 0,
                category: "error",
                message: error
            ))
    }

    return InMemoryBuildResult(
        success: success,
        diagnostics: diagnostics
    )
}

/// Build TypeScript files from a known set of source files
/// - Parameters:
///   - sourceFiles: Array of source files to compile
///   - config: TypeScript configuration (optional, uses default if nil)
/// - Returns: Build result with compilation status and diagnostics
public func buildInMemory(
    _ sourceFiles: [Source],
    config: TSConfig? = nil
) throws -> InMemoryBuildResult {
    let projectPath = "/project"
    let srcPath = "/project/src"

    // Create resolver data
    let resolverData = tsc_create_resolver_data()
    defer { tsc_free_resolver_data(resolverData) }

    guard let resolverData = resolverData else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Failed to create file resolver"
                )
            ]
        )
    }

    // Add project directories
    withMutableCString(projectPath) { projectPathPtr in
        tsc_add_directory_to_resolver(resolverData, projectPathPtr)
    }
    withMutableCString(srcPath) { srcPathPtr in
        tsc_add_directory_to_resolver(resolverData, srcPathPtr)
    }

    // Create appropriate tsconfig for in-memory builds
    let tsConfig: TSConfig
    if let providedConfig = config {
        tsConfig = providedConfig
    } else {
        // Create a config that matches where we place files
        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.strict = true
        compilerOptions.esModuleInterop = true
        compilerOptions.skipLibCheck = true
        compilerOptions.forceConsistentCasingInFileNames = true
        compilerOptions.noEmit = false

        tsConfig = TSConfig(
            compilerOptions: compilerOptions,
            exclude: ["node_modules", "dist"],
            include: ["src/**/*"]
        )
    }

    // Add source files to src directory
    for source in sourceFiles {
        let filePath = "\(srcPath)/\(source.name)"
        withMutableCString(filePath) { filePathPtr in
            withMutableCString(source.content) { contentPtr in
                tsc_add_file_to_resolver(resolverData, filePathPtr, contentPtr)
            }
        }
    }

    // Add tsconfig.json
    do {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let configData = try encoder.encode(tsConfig)
        let configString = String(data: configData, encoding: .utf8) ?? "{}"
        let configPath = "\(projectPath)/tsconfig.json"
        withMutableCString(configPath) { configPathPtr in
            withMutableCString(configString) { configStringPtr in
                tsc_add_file_to_resolver(resolverData, configPathPtr, configStringPtr)
            }
        }
    } catch {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message:
                        "Failed to encode TypeScript configuration: \(error.localizedDescription)"
                )
            ]
        )
    }

    // Build with resolver
    let cResult = withMutableCString(projectPath) { projectPathPtr in
        withMutableCString("") { emptyStringPtr in
            tsc_build_with_resolver(projectPathPtr, 0, emptyStringPtr, resolverData)
        }
    }
    return processResult(cResult)
}

/// Build TypeScript files with a custom file resolver
/// - Parameters:
///   - config: TypeScript configuration (optional, uses default if nil)
///   - resolver: Function that resolves file paths to FileResolver cases or nil
/// - Returns: Build result with compilation status and diagnostics
public func build(
    config: TSConfig? = nil,
    resolver: @escaping @Sendable (String) async throws -> FileResolver?
) async throws -> InMemoryBuildResult {
    let projectPath = "/project"

    // Create resolver data
    let resolverData = tsc_create_resolver_data()
    defer { tsc_free_resolver_data(resolverData) }

    guard let resolverData = resolverData else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Failed to create file resolver"
                )
            ]
        )
    }

    // Add project directory
    withMutableCString(projectPath) { projectPathPtr in
        tsc_add_directory_to_resolver(resolverData, projectPathPtr)
    }

    // Add tsconfig.json
    let tsConfig = config ?? TSConfig.default
    do {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let configData = try encoder.encode(tsConfig)
        let configString = String(data: configData, encoding: .utf8) ?? "{}"
        let configPath = "\(projectPath)/tsconfig.json"
        withMutableCString(configPath) { configPathPtr in
            withMutableCString(configString) { configStringPtr in
                tsc_add_file_to_resolver(resolverData, configPathPtr, configStringPtr)
            }
        }
    } catch {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message:
                        "Failed to encode TypeScript configuration: \(error.localizedDescription)"
                )
            ]
        )
    }

    // We need to pre-populate the resolver data based on the resolver function
    // This is a limitation of the C bridge approach - we can't make dynamic callbacks
    // For now, we'll return an error suggesting to use buildInMemory instead
    return InMemoryBuildResult(
        success: false,
        diagnostics: [
            DiagnosticInfo(
                code: 0,
                category: "error",
                message:
                    "Dynamic file resolution not supported with C bridge. Use buildInMemory() with predefined source files instead."
            )
        ]
    )
}

/// Build TypeScript files with a simple resolver using predefined files
/// - Parameters:
///   - files: Dictionary mapping file paths to their contents
///   - directories: Array of directory paths that should exist
/// - Returns: Build result with compilation status and diagnostics
public func buildWithSimpleResolver(
    _ files: [String: String],
    directories: [String] = []
) throws -> InMemoryBuildResult {
    // Create resolver data
    let resolverData = tsc_create_resolver_data()
    defer { tsc_free_resolver_data(resolverData) }

    guard let resolverData = resolverData else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Failed to create file resolver"
                )
            ]
        )
    }

    // Add directories
    for directory in directories {
        withMutableCString(directory) { directoryPtr in
            tsc_add_directory_to_resolver(resolverData, directoryPtr)
        }
    }

    // Add files
    for (path, content) in files {
        withMutableCString(path) { pathPtr in
            withMutableCString(content) { contentPtr in
                tsc_add_file_to_resolver(resolverData, pathPtr, contentPtr)
            }
        }
    }

    // Determine project path from files
    let projectPath = directories.first ?? "/project"

    // Build with resolver
    let cResult = withMutableCString(projectPath) { projectPathPtr in
        withMutableCString("") { emptyStringPtr in
            tsc_build_with_resolver(projectPathPtr, 0, emptyStringPtr, resolverData)
        }
    }
    return processResult(cResult)
}
