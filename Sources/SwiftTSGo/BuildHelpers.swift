import Foundation
import TSCBridge

// MARK: - C Bridge Helpers

func convertCDiagnostics(_ cDiagnostics: UnsafeMutablePointer<c_diagnostic>?, count: Int)
    -> [DiagnosticInfo]
{
    guard let cDiagnostics, count > 0 else { return [] }

    var diagnostics: [DiagnosticInfo] = []
    for i in 0 ..< count {
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

func convertCWrittenFiles(
    _ paths: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>?,
    _ contents: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>?,
    count: Int
) -> [String: String] {
    guard let paths, let contents, count > 0 else { return [:] }

    var writtenFiles: [String: String] = [:]
    for i in 0 ..< count {
        if let pathPtr = paths[i], let contentPtr = contents[i] {
            let path = String(cString: pathPtr)
            let content = String(cString: contentPtr)
            writtenFiles[path] = content
        }
    }
    return writtenFiles
}

func convertCCompiledFiles(from writtenFiles: [String: String]) -> [Source] {
    writtenFiles.map { path, content in
        let filename = (path as NSString).lastPathComponent
        return Source(name: filename, content: content)
    }
}

func processResult(_ cResult: UnsafeMutablePointer<c_build_result>?) -> InMemoryBuildResult {
    guard let cResult else {
        return InMemoryBuildResult(
            success: false,
            diagnostics: [
                DiagnosticInfo(
                    code: 0,
                    category: "error",
                    message: "Build failed with no result"
                ),
            ]
        )
    }

    defer { tsc_free_result(cResult) }

    let success = cResult.pointee.success != 0
    let configFile =
        cResult.pointee.config_file != nil ? String(cString: cResult.pointee.config_file) : ""
    let diagnostics = convertCDiagnostics(
        cResult.pointee.diagnostics, count: Int(cResult.pointee.diagnostic_count)
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

// MARK: - C String Helpers

func withMutableCString<T>(_ string: String, _ body: (UnsafeMutablePointer<CChar>) -> T)
    -> T
{
    string.withCString { cString in
        let mutableCString = strdup(cString)!
        defer { free(mutableCString) }
        return body(mutableCString)
    }
}
