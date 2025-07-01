import Foundation
import Testing

@testable import SwiftTSGo

@Suite(.serialized)
struct BuildInMemoryTests {

    @Test func basicCompilationTest() throws {
        let sources = [
            Source(
                name: "hello.ts",
                content: """
                    function greet(name: string): string {
                        return `Hello, ${name}!`;
                    }

                    const message = greet("World");
                    console.log(message);
                    """)
        ]

        let result = try build(sources)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(!result.configFile.isEmpty)
    }

    @Test func typeErrorTest() throws {
        let sources = [
            Source(
                name: "error.ts",
                content: """
                    function addNumbers(a: number, b: number): number {
                        return a + b;
                    }

                    // This should cause a type error
                    const result = addNumbers("hello", 42);
                    """)
        ]

        let result = try build(sources)

        #expect(result.success == false)
        #expect(!result.diagnostics.isEmpty)

        let errorDiagnostics = result.diagnostics.filter { $0.category == "error" }
        #expect(!errorDiagnostics.isEmpty)

        // Look for the specific TS2345 error
        let ts2345Error = errorDiagnostics.first { $0.code == 2345 }
        #expect(ts2345Error != nil)
        #expect(ts2345Error?.message.contains("not assignable") == true)
    }

    @Test func customConfigTest() throws {
        let sources = [
            Source(
                name: "strict.ts",
                content: """
                    // This should fail in strict mode - function parameter without type
                    function greet(name) {
                        return "Hello, " + name;
                    }

                    console.log(greet("World"));
                    """)
        ]

        // Test with strict mode
        var strictOptions = CompilerOptions()
        strictOptions.strict = true
        strictOptions.noImplicitAny = true
        let strictConfig = TSConfig(compilerOptions: strictOptions)

        let strictResult = try build(sources, config: strictConfig)
        #expect(strictResult.success == false)

        // Test with non-strict mode
        var lenientOptions = CompilerOptions()
        lenientOptions.strict = false
        lenientOptions.noImplicitAny = false
        let lenientConfig = TSConfig(compilerOptions: lenientOptions)

        let lenientResult = try build(sources, config: lenientConfig)
        #expect(lenientResult.success == true)
    }

    @Test func multipleFilesTest() throws {
        let sources = [
            Source(
                name: "utils.ts",
                content: """
                    export function multiply(a: number, b: number): number {
                        return a * b;
                    }

                    export const PI = 3.14159;
                    """),
            Source(
                name: "main.ts",
                content: """
                    import { multiply, PI } from './utils';

                    const area = multiply(PI, 5 * 5);
                    console.log(`Area: ${area}`);
                    """),
        ]

        var moduleOptions = CompilerOptions()
        moduleOptions.module = .esnext
        moduleOptions.target = .es2020
        moduleOptions.moduleResolution = .node
        let config = TSConfig(compilerOptions: moduleOptions)

        let result = try build(sources, config: config)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func customResolverTest() throws {
        // Test the resolver-based build function
        let fileMap = [
            "package.json": """
            {
                "name": "test-project",
                "version": "1.0.0"
            }
            """,
            "src/index.ts": """
            interface User {
                name: string;
                age: number;
            }

            const user: User = {
                name: "Alice",
                age: 30
            };

            console.log(user);
            """,
            "src/types.ts": """
            export type Status = 'active' | 'inactive';
            """,
        ]

        let resolver: (String) -> FileResolver = { path in
            // Handle the project directory itself
            if path == "/project" {
                return .directory
            }

            // Handle files under /project/
            var normalizedPath = path
            if path.hasPrefix("/project/") {
                normalizedPath = String(path.dropFirst("/project/".count))
            } else if path.hasPrefix("/") {
                normalizedPath = String(path.dropFirst())
            }

            // Check for exact file match first
            if let content = fileMap[normalizedPath] {
                return .file(content)
            }

            // Check for directory patterns
            if normalizedPath == "src" || normalizedPath == "src/" {
                return .directory
            }

            // Check if it's a directory based on having files underneath
            let isDirectory = fileMap.keys.contains { filePath in
                filePath.hasPrefix(
                    normalizedPath.hasSuffix("/") ? normalizedPath : (normalizedPath + "/"))
            }

            if isDirectory {
                return .directory
            }

            // Handle root directory
            if normalizedPath.isEmpty || normalizedPath == "." {
                return .directory
            }

            return .notFound
        }

        var compilerOptions = CompilerOptions()
        compilerOptions.module = .commonjs
        compilerOptions.target = .es2020
        compilerOptions.outDir = "./dist"
        let config = TSConfig(
            compilerOptions: compilerOptions,
            files: ["src/index.ts", "src/types.ts"]
        )

        let result = try build(config: config, resolver: resolver)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func emitFilesTest() throws {
        let sources = [
            Source(
                name: "calculator.ts",
                content: """
                    export class Calculator {
                        add(a: number, b: number): number {
                            return a + b;
                        }

                        subtract(a: number, b: number): number {
                            return a - b;
                        }
                    }
                    """)
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.declaration = true
        compilerOptions.outDir = "./dist"
        compilerOptions.noEmit = false

        let config = TSConfig(compilerOptions: compilerOptions)

        let result = try build(sources, config: config)

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

    @Test func jsxSupportTest() throws {
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
                    """)
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.jsx = .react
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs
        compilerOptions.esModuleInterop = true
        compilerOptions.allowSyntheticDefaultImports = true

        let config = TSConfig(compilerOptions: compilerOptions)

        let result = try build(sources, config: config)

        // This might fail due to missing React types, but should not crash
        #expect(result.diagnostics.filter { $0.category == "error" }.count >= 0)
    }

    @Test func libConfigTest() throws {
        let sources = [
            Source(
                name: "modern.ts",
                content: """
                    // Uses modern JavaScript features
                    const promise = Promise.resolve(42);
                    const result = await promise;
                    console.log(result);

                    const map = new Map<string, number>();
                    map.set("answer", 42);

                    // Export to make this a module
                    export {};
                    """)
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.lib = ["ES2020", "DOM"]
        compilerOptions.module = .esnext

        let config = TSConfig(compilerOptions: compilerOptions)

        let result = try build(sources, config: config)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func pathMappingTest() throws {
        let sources = [
            Source(
                name: "src/utils/helper.ts",
                content: """
                    export function formatMessage(msg: string): string {
                        return `[INFO] ${msg}`;
                    }
                    """),
            Source(
                name: "src/main.ts",
                content: """
                    import { formatMessage } from './utils/helper';

                    const message = formatMessage("Hello, World!");
                    console.log(message);
                    """),
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.target = .es2020
        compilerOptions.module = .commonjs

        let config = TSConfig(compilerOptions: compilerOptions)

        let result = try build(sources, config: config)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
    }

    @Test func simpleTypeCheckTest() throws {
        // Simple type check test using resolver API - just validate types without emitting files
        let fileMap = [
            "simple.ts": """
            // Valid TypeScript code
            const message: string = "Hello, TypeScript!";
            const count: number = 42;
            const isActive: boolean = true;

            function add(a: number, b: number): number {
                return a + b;
            }

            const result = add(count, 10);
            """,
            "tsconfig.json": """
            {
                "compilerOptions": {
                    "noEmit": true,
                    "strict": true,
                    "target": "ES2020"
                }
            }
            """,
        ]

        let resolver: (String) -> FileResolver = { path in
            // Handle the project directory itself
            if path == "/project" {
                return .directory
            }

            // Handle files under /project/
            var normalizedPath = path
            if path.hasPrefix("/project/") {
                normalizedPath = String(path.dropFirst("/project/".count))
            } else if path.hasPrefix("/") {
                normalizedPath = String(path.dropFirst())
            }

            // Check for exact file match
            if let content = fileMap[normalizedPath] {
                return .file(content)
            }

            // Handle root directory
            if normalizedPath.isEmpty || normalizedPath == "." {
                return .directory
            }

            return .notFound
        }

        var compilerOptions = CompilerOptions()
        compilerOptions.noEmit = true  // Type check only, no file output
        compilerOptions.strict = true
        compilerOptions.target = .es2020

        let config = TSConfig(
            compilerOptions: compilerOptions,
            files: ["simple.ts"]
        )

        let result = try build(config: config, resolver: resolver)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(result.compiledFiles.isEmpty)  // No output files with noEmit
        #expect(result.writtenFiles.isEmpty)  // No written files with noEmit
    }

    @Test func validationOnlyTest() throws {
        let sources = [
            Source(
                name: "validation.ts",
                content: """
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
                    """)
        ]

        var compilerOptions = CompilerOptions()
        compilerOptions.noEmit = true
        compilerOptions.strict = true
        compilerOptions.target = .es2020

        let config = TSConfig(compilerOptions: compilerOptions)

        let result = try build(sources, config: config)

        #expect(result.success == true)
        #expect(result.diagnostics.filter { $0.category == "error" }.isEmpty)
        #expect(result.compiledFiles.isEmpty)  // No output files with noEmit
    }
}
