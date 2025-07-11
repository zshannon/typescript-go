import Foundation
import Testing

@testable import SwiftTSGo

@Suite(.serialized)
struct BuildFileSystemTests {
    @Test func helloWorldTest() throws {
        // Get the path to the test project directory
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-hello", ofType: nil)!

        // Configure build with the test project path
        // Build using the file system build function
        let result = try buildFromFileSystem(projectPath: testProjectPath, printErrors: false)

        // Verify successful compilation
        #expect(result.success == true)
        #expect(result.diagnostics.isEmpty)
        #expect(!result.configFile.isEmpty)
        #expect(result.configFile.contains("tsconfig.json"))

        // Verify output file exists
        let outputFile = testProjectPath + "/dist/hello.js"
        #expect(FileManager.default.fileExists(atPath: outputFile))
        let outputContents = try String(contentsOfFile: outputFile, encoding: .utf8)
        #expect(outputContents.contains("console.log(message)"))
    }

    @Test func typeCheckFailureTest() throws {
        // Get the path to the test project directory with type errors
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-error", ofType: nil)!

        // Build using the file system build function
        let result = try buildFromFileSystem(projectPath: testProjectPath, printErrors: false)

        // Verify compilation failed
        #expect(result.success == false)
        #expect(!result.configFile.isEmpty)
        #expect(!result.diagnostics.isEmpty)

        // Check that we have the expected TypeScript error
        let errorDiagnostics = result.diagnostics.filter { $0.category == "error" }
        #expect(!errorDiagnostics.isEmpty)

        // Look for the specific TS2345 error
        let ts2345Error = errorDiagnostics.first { $0.code == 2345 }
        #expect(ts2345Error != nil)
        #expect(
            ts2345Error?.message.contains(
                "Argument of type 'string' is not assignable to parameter of type 'number'"
            ) == true
        )

        // Verify the error has proper location information
        if let error = ts2345Error {
            #expect(!error.file.isEmpty)
            #expect(error.file.contains("error.ts"))
            #expect(error.line > 0)
            #expect(error.column > 0)
        }
    }

    @Test func detailedDiagnosticsTest() throws {
        // Test that we get detailed diagnostic information
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-error", ofType: nil)!

        let result = try buildFromFileSystem(projectPath: testProjectPath, printErrors: true)

        // Should fail compilation
        #expect(result.success == false)
        #expect(!result.diagnostics.isEmpty)

        // Check diagnostic details
        for diagnostic in result.diagnostics {
            #expect(!diagnostic.message.isEmpty)
            #expect(!diagnostic.category.isEmpty)

            if diagnostic.category == "error" {
                #expect(diagnostic.code > 0)
                #expect(!diagnostic.file.isEmpty)
                #expect(diagnostic.line >= 0)
                #expect(diagnostic.column >= 0)
            }
        }
    }

    @Test func emittedFilesTest() throws {
        // Test with a configuration that emits files
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-hello", ofType: nil)!

        // Create a temporary directory for output
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(
            UUID().uuidString
        )
        try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)

        defer {
            try? FileManager.default.removeItem(at: tempDir)
        }

        // Copy test project to temp directory so we can modify tsconfig
        let tempProjectDir = tempDir.appendingPathComponent("test-project")
        try FileManager.default.copyItem(atPath: testProjectPath, toPath: tempProjectDir.path)

        // Modify tsconfig to enable emission
        let tsconfigPath = tempProjectDir.appendingPathComponent("tsconfig.json")
        let tsconfigContent = try String(contentsOf: tsconfigPath, encoding: .utf8)

        // Simple JSON string replacement to add noEmit: false
        let modifiedContent = tsconfigContent.replacingOccurrences(
            of: "\"skipLibCheck\": true",
            with: "\"skipLibCheck\": true,\n        \"noEmit\": false"
        )
        try modifiedContent.write(to: tsconfigPath, atomically: true, encoding: .utf8)

        let result = try buildFromFileSystem(projectPath: tempProjectDir.path, printErrors: false)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)

        // Check that output files were actually created in the dist directory
        let distDir = tempProjectDir.appendingPathComponent("dist")
        let distExists = FileManager.default.fileExists(atPath: distDir.path)
        #expect(distExists == true)

        if distExists {
            let distContents = try FileManager.default.contentsOfDirectory(atPath: distDir.path)
            #expect(!distContents.isEmpty)

            // Should have at least one .js file
            let jsFiles = distContents.filter { $0.hasSuffix(".js") }
            #expect(!jsFiles.isEmpty)
        }
    }

    @Test func temporaryDirectoryBuildTest() throws {
        // Test building in a temporary directory
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-hello", ofType: nil)!

        // Create a temporary directory
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(
            UUID().uuidString
        )
        try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)

        defer {
            try? FileManager.default.removeItem(at: tempDir)
        }

        // Copy the test project to the temporary directory
        let tempProjectDir = tempDir.appendingPathComponent("test-project")
        try FileManager.default.copyItem(atPath: testProjectPath, toPath: tempProjectDir.path)

        let result = try buildFromFileSystem(projectPath: tempProjectDir.path, printErrors: false)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(!result.configFile.isEmpty)
    }

    @Test func explicitConfigFileTest() throws {
        // Test with explicit tsconfig file path
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-hello", ofType: nil)!

        let result = try buildFromFileSystem(
            projectPath: testProjectPath,
            printErrors: false,
            configFile: "\(testProjectPath)/custom.config.json"
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(!result.configFile.isEmpty)
        #expect(result.configFile.contains("custom.config.json"))
    }

    @Test func printErrorsConfigTest() throws {
        // Test with printErrors enabled
        let testBundle = Bundle.module
        let testProjectPath = testBundle.path(forResource: "test-error", ofType: nil)!

        let result = try buildFromFileSystem(projectPath: testProjectPath, printErrors: true)

        // Should still fail compilation but with error printing enabled
        #expect(result.success == false)
        #expect(!result.diagnostics.isEmpty)

        // Verify diagnostic information is still available
        let errorDiagnostics = result.diagnostics.filter { $0.category == "error" }
        #expect(!errorDiagnostics.isEmpty)
    }

    @Test func nonExistentProjectTest() throws {
        // Test with a non-existent project path
        let nonExistentPath = "/path/that/does/not/exist"

        let result = try buildFromFileSystem(projectPath: nonExistentPath, printErrors: false)

        // Should fail
        #expect(result.success == false)
        #expect(!result.diagnostics.isEmpty)

        // Should have some kind of error diagnostic
        let errorDiagnostics = result.diagnostics.filter { $0.category == "error" }
        #expect(!errorDiagnostics.isEmpty)
    }
}
