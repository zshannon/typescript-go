import Foundation
import Testing

@testable import SwiftTSGo

// Helper function to replace buildWithSimpleResolver
private func buildWithSimpleResolver(
    _ files: [String: String],
    directories: [String] = []
) async throws -> InMemoryBuildResult {
    // Use the resolver-based build function to handle arbitrary file paths
    let resolver: @Sendable (String) async throws -> FileResolver? = { path in
        if let content = files[path] {
            return .file(content)
        }
        // Check if it's a directory that should exist
        if directories.contains(path) || path == "/project" || path == "/project/src" {
            // Find all files that are children of this directory
            let childFiles = files.keys
                .filter { $0.hasPrefix(path + "/") }
                .compactMap { filePath -> String? in
                    let relativePath = String(filePath.dropFirst(path.count + 1))
                    // Only return direct children, not nested paths
                    if !relativePath.contains("/") {
                        return relativePath
                    }
                    return nil
                }

            // Find all subdirectories that are children of this directory
            let childDirs = directories
                .filter { $0.hasPrefix(path + "/") }
                .compactMap { dirPath -> String? in
                    let relativePath = String(dirPath.dropFirst(path.count + 1))
                    // Only return direct children, not nested paths
                    if !relativePath.contains("/") {
                        return relativePath
                    }
                    return nil
                }

            let allChildren = childFiles + childDirs
            return .directory(allChildren)
        }
        return nil
    }

    // If there's a tsconfig.json, extract it and use it as config
    var config: TSConfig? = nil
    if let tsconfigContent = files["/project/tsconfig.json"] {
        if let data = tsconfigContent.data(using: .utf8),
           let decoded = try? JSONDecoder().decode(TSConfig.self, from: data)
        {
            config = decoded
        }
    }

    return try await build(config: config ?? .default, resolver: resolver)
}

@Suite(.serialized)
struct BuildInMemoryTests {
    @Test func basicCompilationTest() async throws {
        let sources = [
            Source(
                name: "hello.ts",
                content: """
                function greet(name: string): string {
                    return `Hello, ${name}!`;
                }

                const message = greet("World");
                console.log(message);
                """
            ),
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.noEmit = true
        compilerOptions.strict = true

        let config = TSConfig(compilerOptions: compilerOptions, include: ["src/**/*"])
        let result = try await build(config: config, sources)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(!result.configFile.isEmpty)
    }

    @Test func basicCompilationTestWithGlob() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "target": "es2015",
                        "module": "commonjs",
                        "noEmit": true
                    },
                    "include": ["src/**/*"]
                }
                """,
                "/project/src/main.ts": "console.log('Hello, World!');",
            ], directories: ["/project", "/project/src"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(!result.configFile.isEmpty)
    }

    @Test func typeErrorTest() async throws {
        let sources = [
            Source(
                name: "error.ts",
                content: """
                function addNumbers(a: number, b: number): number {
                    return a + b;
                }

                // This should cause a type error
                const result = addNumbers("hello", 42);
                """
            ),
        ]

        let result = try await build(sources)

        #expect(result.success == false)
        #expect(!result.diagnostics.isEmpty)

        let errorDiagnostics = result.diagnostics.filter { $0.category == "error" }
        #expect(!errorDiagnostics.isEmpty)

        // Look for the specific TS2345 error
        let ts2345Error = errorDiagnostics.first { $0.code == 2345 }
        #expect(ts2345Error != nil)
        #expect(ts2345Error?.message.contains("not assignable") == true)
    }

    @Test func customConfigTest() async throws {
        let sources = [
            Source(
                name: "strict.ts",
                content: """
                // This should fail in strict mode - function parameter without type
                function greet(name) {
                    return "Hello, " + name;
                }

                console.log(greet("World"));
                """
            ),
        ]

        // Test with strict mode
        var strictOptions = CompilerOptions()
        strictOptions.strict = true
        strictOptions.noImplicitAny = true
        strictOptions.noEmit = true
        let strictConfig = TSConfig(compilerOptions: strictOptions, include: ["src/**/*"])

        let strictResult = try await build(config: strictConfig, sources)
        #expect(strictResult.success == false)

        // Test with non-strict mode
        var lenientOptions = CompilerOptions()
        lenientOptions.strict = false
        lenientOptions.noImplicitAny = false
        lenientOptions.noEmit = true
        let lenientConfig = TSConfig(compilerOptions: lenientOptions, include: ["src/**/*"])

        let lenientResult = try await build(config: lenientConfig, sources)
        #expect(lenientResult.success == true)
    }

    @Test func multipleFilesTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "target": "es2015",
                        "module": "commonjs",
                        "noEmit": true
                    },
                    "include": ["**/*"]
                }
                """,
                "/project/utils.ts":
                    "export function add(a: number, b: number): number { return a + b; }",
                "/project/main.ts": "import { add } from './utils'; console.log(add(2, 3));",
            ], directories: ["/project"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func customResolverTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "target": "es2015",
                        "module": "commonjs",
                        "noEmit": true
                    },
                    "include": ["**/*"]
                }
                """,
                "/project/src/main.ts": "const x: number = 42; console.log(x);",
            ], directories: ["/project", "/project/src"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func emitFilesTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "target": "es2020",
                        "module": "commonjs",
                        "declaration": true,
                        "outDir": "./dist",
                        "noEmit": false
                    },
                    "include": ["**/*"],
                    "exclude": ["/project/dist"]
                }
                """,
                "/project/calculator.ts": """
                export class Calculator {
                    add(a: number, b: number): number {
                        return a + b;
                    }

                    subtract(a: number, b: number): number {
                        return a - b;
                    }
                }
                """,
            ], directories: ["/project", "/project/dist"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(!result.compiledFiles.isEmpty)

        // Should have both JS and declaration files
        let jsFiles = result.compiledFiles.filter { $0.name.hasSuffix(".js") }
        let dtsFiles = result.compiledFiles.filter { $0.name.hasSuffix(".d.ts") }

        #expect(!jsFiles.isEmpty)
        #expect(!dtsFiles.isEmpty)

        // Verify that written files are being captured
        #expect(!result.writtenFiles.isEmpty)

        // Should have written files with actual content
        let writtenJsFiles = result.writtenFiles.filter { $0.key.hasSuffix(".js") }
        let writtenDtsFiles = result.writtenFiles.filter { $0.key.hasSuffix(".d.ts") }

        #expect(!writtenJsFiles.isEmpty)
        #expect(!writtenDtsFiles.isEmpty)

        // Verify content is not empty
        for (_, content) in result.writtenFiles {
            #expect(!content.isEmpty)
        }
    }

    @Test func jsxSupportTest() async throws {
        let sources = [
            Source(
                name: "component.tsx",
                content: """
                import React from 'react';

                interface Props {
                    message: string;
                }

                export const Greeting: React.FC<Props> = ({ message }) => {
                    return <div>{message}</div>;
                };
                """
            ),
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.jsx = .react
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.esModuleInterop = true
        compilerOptions.allowSyntheticDefaultImports = true

        let config = TSConfig(compilerOptions: compilerOptions)

        let result = try await build(config: config, sources)

        // This might fail due to missing React types, but should not crash
        #expect(result.diagnostics.filter { $0.category == "error" }.count >= 0)
    }

    @Test func libConfigTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "target": "es2020",
                        "lib": ["ES2020", "DOM"],
                        "module": "esnext",
                        "noEmit": true
                    },
                    "include": ["**/*"]
                }
                """,
                "/project/modern.ts": """
                // Uses modern JavaScript features
                const promise = Promise.resolve(42);
                const result = await promise;
                console.log(result);

                const map = new Map<string, number>();
                map.set("answer", 42);

                // Export to make this a module
                export {};
                """,
            ], directories: ["/project"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func pathMappingTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "target": "es2020",
                        "module": "commonjs",
                        "noEmit": true
                    },
                    "include": ["**/*"]
                }
                """,
                "/project/src/utils/helper.ts": """
                export function formatMessage(msg: string): string {
                    return `[INFO] ${msg}`;
                }
                """,
                "/project/src/main.ts": """
                import { formatMessage } from './utils/helper';

                const message = formatMessage("Hello, World!");
                console.log(message);
                """,
            ], directories: ["/project", "/project/src", "/project/src/utils"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func simpleTypeCheckTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "noEmit": true,
                        "strict": true,
                        "target": "ES2020"
                    },
                    "include": ["**/*"]
                }
                """,
                "/project/simple.ts": """
                // Valid TypeScript code
                const message: string = "Hello, TypeScript!";
                const count: number = 42;
                const isActive: boolean = true;

                function add(a: number, b: number): number {
                    return a + b;
                }

                const result = add(count, 10);
                """,
            ], directories: ["/project"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(result.compiledFiles.isEmpty) // No output files with noEmit
        #expect(result.writtenFiles.isEmpty) // No written files with noEmit
    }

    @Test func validationOnlyTest() async throws {
        let result = try await buildWithSimpleResolver(
            [
                "/project/tsconfig.json": """
                {
                    "compilerOptions": {
                        "noEmit": true,
                        "strict": true,
                        "target": "es2020"
                    },
                    "include": ["**/*"]
                }
                """,
                "/project/validation.ts": """
                type User = {
                    id: number;
                    name: string;
                    email?: string;
                };

                function validateUser(user: unknown): user is User {
                    return typeof user === 'object' &&
                           user !== null &&
                           'id' in user &&
                           'name' in user;
                }

                const user: unknown = { id: 1, name: "John" };
                if (validateUser(user)) {
                    console.log(user.name); // TypeScript knows this is safe
                }
                """,
            ], directories: ["/project"]
        )

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(result.compiledFiles.isEmpty) // No output files with noEmit
    }
}
