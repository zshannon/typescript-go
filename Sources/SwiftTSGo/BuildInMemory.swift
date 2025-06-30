import Foundation
import TSGoBindings

// Represents a source file with name and content
public struct Source {
    public let name: String
    public let content: String

    public init(name: String, content: String) {
        self.name = name
        self.content = content
    }
}

// Result of compiling files from memory
public struct InMemoryBuildResult {
    public let success: Bool
    public let diagnostics: [DiagnosticInfo]
    public let compiledFiles: [Source]
    public let configFile: String

    public init(
        success: Bool, diagnostics: [DiagnosticInfo] = [], compiledFiles: [Source] = [],
        configFile: String = ""
    ) {
        self.success = success
        self.diagnostics = diagnostics
        self.compiledFiles = compiledFiles
        self.configFile = configFile
    }
}

// Compile TypeScript files from memory using temporary directory
public func build(_ sourceFiles: [Source], config: TSConfig? = nil) throws
    -> InMemoryBuildResult
{
    // Create temporary directory (iOS sandbox friendly)
    let tempDir = URL(fileURLWithPath: NSTemporaryDirectory())
        .appendingPathComponent("typescript-go-compile-\(UUID().uuidString)")

    try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)

    // Ensure cleanup even if compilation fails
    defer {
        try? FileManager.default.removeItem(at: tempDir)
    }

    // Create src directory if we have TypeScript files
    let srcDir = tempDir.appendingPathComponent("src")
    var needsSrcDir = false

    // Write source files to temporary directory
    for sourceFile in sourceFiles {
        let fileName = sourceFile.name
        let fileURL: URL

        // Determine if file goes in src/ or root based on extension
        if fileName.hasSuffix(".ts") || fileName.hasSuffix(".tsx") {
            if !needsSrcDir {
                try FileManager.default.createDirectory(
                    at: srcDir, withIntermediateDirectories: true)
                needsSrcDir = true
            }
            fileURL = srcDir.appendingPathComponent(fileName)
        } else {
            fileURL = tempDir.appendingPathComponent(fileName)
        }

        try sourceFile.content.write(to: fileURL, atomically: true, encoding: .utf8)
    }

    // Create tsconfig.json from config or use default if none provided and we have TypeScript files
    if let tsConfig = config {
        let tsconfigURL = tempDir.appendingPathComponent("tsconfig.json")
        let configJSON = try tsConfig.toJSONString()
        try configJSON.write(to: tsconfigURL, atomically: true, encoding: .utf8)
    } else if needsSrcDir {
        let defaultConfig = TSConfig.default
        let tsconfigURL = tempDir.appendingPathComponent("tsconfig.json")
        let configJSON = try defaultConfig.toJSONString()
        try configJSON.write(to: tsconfigURL, atomically: true, encoding: .utf8)
    }

    // Run build on temporary directory
    let buildConfig = FileSystemBuildConfig(projectPath: tempDir.path, printErrors: false)
    let result = build(buildConfig)

    // If build failed, throw error
    if !result.success {
        let errorMessages = result.diagnostics
            .filter { $0.category == "error" }
            .map { "\($0.file):\($0.line):\($0.column) - \($0.message)" }
            .joined(separator: "\n")

        throw NSError(
            domain: "TypeScriptCompilationError",
            code: 1,
            userInfo: [
                NSLocalizedDescriptionKey: "TypeScript compilation failed:\n\(errorMessages)"
            ]
        )
    }

    // Read compiled files
    var compiledFiles: [Source] = []
    let distDir = tempDir.appendingPathComponent("dist")

    if FileManager.default.fileExists(atPath: distDir.path) {
        let distContents = try FileManager.default.contentsOfDirectory(
            at: distDir, includingPropertiesForKeys: nil)

        for fileURL in distContents {
            // Skip directories, only process files
            var isDirectory: ObjCBool = false
            if FileManager.default.fileExists(atPath: fileURL.path, isDirectory: &isDirectory)
                && !isDirectory.boolValue
            {
                let content = try String(contentsOf: fileURL)
                let compiledFile = Source(name: fileURL.lastPathComponent, content: content)
                compiledFiles.append(compiledFile)
            }
        }
    }

    return InMemoryBuildResult(
        success: result.success,
        diagnostics: result.diagnostics,
        compiledFiles: compiledFiles,
        configFile: result.configFile
    )
}
