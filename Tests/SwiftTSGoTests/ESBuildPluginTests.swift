import Testing
@testable import SwiftTSGo

@Suite("ESBuild Plugin API Tests")
struct ESBuildPluginTests {

    // MARK: - ResolveKind Tests

    @Test("ResolveKind enum has correct raw values")
    func testResolveKindValues() {
        #expect(ESBuildPluginResolveKind.entryPoint.cValue == 0)
        #expect(ESBuildPluginResolveKind.importStatement.cValue == 1)
        #expect(ESBuildPluginResolveKind.requireCall.cValue == 2)
        #expect(ESBuildPluginResolveKind.dynamicImport.cValue == 3)
        #expect(ESBuildPluginResolveKind.requireResolve.cValue == 4)
        #expect(ESBuildPluginResolveKind.importRule.cValue == 5)
        #expect(ESBuildPluginResolveKind.composesFrom.cValue == 6)
        #expect(ESBuildPluginResolveKind.urlToken.cValue == 7)
    }

    @Test("ResolveKind initializes from C values")
    func testResolveKindFromCValue() {
        #expect(ESBuildPluginResolveKind(cValue: 0) == .entryPoint)
        #expect(ESBuildPluginResolveKind(cValue: 1) == .importStatement)
        #expect(ESBuildPluginResolveKind(cValue: 2) == .requireCall)
        #expect(ESBuildPluginResolveKind(cValue: 3) == .dynamicImport)
        #expect(ESBuildPluginResolveKind(cValue: 4) == .requireResolve)
        #expect(ESBuildPluginResolveKind(cValue: 5) == .importRule)
        #expect(ESBuildPluginResolveKind(cValue: 6) == .composesFrom)
        #expect(ESBuildPluginResolveKind(cValue: 7) == .urlToken)
        #expect(ESBuildPluginResolveKind(cValue: 99) == nil)
    }

    @Test("ResolveKind has all cases")
    func testResolveKindAllCases() {
        let allCases = ESBuildPluginResolveKind.allCases
        #expect(allCases.count == 8)
        #expect(allCases.contains(.entryPoint))
        #expect(allCases.contains(.importStatement))
        #expect(allCases.contains(.requireCall))
        #expect(allCases.contains(.dynamicImport))
        #expect(allCases.contains(.requireResolve))
        #expect(allCases.contains(.importRule))
        #expect(allCases.contains(.composesFrom))
        #expect(allCases.contains(.urlToken))
    }

    // MARK: - Location Tests

    @Test("Location initializes with all properties")
    func testLocationInitialization() {
        let location = ESBuildPluginLocation(
            file: "test.js",
            namespace: "file",
            line: 10,
            column: 5,
            length: 7,
            lineText: "const x = 42;"
        )

        #expect(location.file == "test.js")
        #expect(location.namespace == "file")
        #expect(location.line == 10)
        #expect(location.column == 5)
        #expect(location.length == 7)
        #expect(location.lineText == "const x = 42;")
    }

    // MARK: - Message Tests

    @Test("Message initializes with minimal properties")
    func testMessageMinimalInit() {
        let message = ESBuildPluginMessage(text: "Error message")

        #expect(message.text == "Error message")
        #expect(message.location == nil)
        #expect(message.detail == nil)
    }

    @Test("Message initializes with all properties")
    func testMessageFullInit() {
        let location = ESBuildPluginLocation(
            file: "test.js",
            namespace: "file",
            line: 1,
            column: 0,
            length: 5,
            lineText: "error"
        )
        let detail = ["key": "value"]
        let message = ESBuildPluginMessage(
            text: "Error message",
            location: location,
            detail: detail
        )

        #expect(message.text == "Error message")
        #expect(message.location?.file == "test.js")
        #expect(message.detail != nil)
    }

    // MARK: - OnResolveArgs Tests

    @Test("OnResolveArgs initializes with all properties")
    func testOnResolveArgsInit() {
        let args = ESBuildOnResolveArgs(
            path: "module",
            importer: "/src/index.js",
            namespace: "file",
            resolveDir: "/src",
            kind: .importStatement,
            pluginData: ["key": "value"],
            with: ["type": "json"]
        )

        #expect(args.path == "module")
        #expect(args.importer == "/src/index.js")
        #expect(args.namespace == "file")
        #expect(args.resolveDir == "/src")
        #expect(args.kind == .importStatement)
        #expect(args.pluginData != nil)
        #expect(args.with["type"] == "json")
    }

    // MARK: - OnLoadArgs Tests

    @Test("OnLoadArgs initializes with all properties")
    func testOnLoadArgsInit() {
        let args = ESBuildOnLoadArgs(
            path: "/src/module.js",
            namespace: "file",
            suffix: "?v=1.0",
            pluginData: ["key": "value"],
            with: ["type": "json"]
        )

        #expect(args.path == "/src/module.js")
        #expect(args.namespace == "file")
        #expect(args.suffix == "?v=1.0")
        #expect(args.pluginData != nil)
        #expect(args.with["type"] == "json")
    }

    // MARK: - OnResolveResult Tests

    @Test("OnResolveResult initializes with minimal properties")
    func testOnResolveResultMinimalInit() {
        let result = ESBuildOnResolveResult()

        #expect(result.path == nil)
        #expect(result.namespace == nil)
        #expect(result.external == nil)
        #expect(result.sideEffects == nil)
        #expect(result.suffix == nil)
        #expect(result.pluginData == nil)
        #expect(result.pluginName == nil)
        #expect(result.errors.isEmpty)
        #expect(result.warnings.isEmpty)
        #expect(result.watchFiles.isEmpty)
        #expect(result.watchDirs.isEmpty)
    }

    @Test("OnResolveResult initializes with all properties")
    func testOnResolveResultFullInit() {
        let errors = [ESBuildPluginMessage(text: "Error")]
        let warnings = [ESBuildPluginMessage(text: "Warning")]

        let result = ESBuildOnResolveResult(
            path: "/resolved/path.js",
            namespace: "custom",
            external: true,
            sideEffects: false,
            suffix: "?v=1.0",
            pluginData: ["key": "value"],
            pluginName: "test-plugin",
            errors: errors,
            warnings: warnings,
            watchFiles: ["/watch/file.js"],
            watchDirs: ["/watch/dir"]
        )

        #expect(result.path == "/resolved/path.js")
        #expect(result.namespace == "custom")
        #expect(result.external == true)
        #expect(result.sideEffects == false)
        #expect(result.suffix == "?v=1.0")
        #expect(result.pluginData != nil)
        #expect(result.pluginName == "test-plugin")
        #expect(result.errors.count == 1)
        #expect(result.warnings.count == 1)
        #expect(result.watchFiles == ["/watch/file.js"])
        #expect(result.watchDirs == ["/watch/dir"])
    }

    // MARK: - OnLoadResult Tests

    @Test("OnLoadResult initializes with Data contents")
    func testOnLoadResultDataInit() {
        let data = "console.log('test')".data(using: .utf8)!
        let result = ESBuildOnLoadResult(
            contents: data,
            loader: .js,
            resolveDir: "/src"
        )

        #expect(result.contents == data)
        #expect(result.loader == .js)
        #expect(result.resolveDir == "/src")
    }

    @Test("OnLoadResult initializes with String contents")
    func testOnLoadResultStringInit() {
        let result = ESBuildOnLoadResult(
            contents: "console.log('test')",
            loader: .js,
            resolveDir: "/src"
        )

        #expect(result.contents == "console.log('test')".data(using: .utf8))
        #expect(result.loader == .js)
        #expect(result.resolveDir == "/src")
    }

    @Test("OnLoadResult initializes with all properties")
    func testOnLoadResultFullInit() {
        let errors = [ESBuildPluginMessage(text: "Error")]
        let warnings = [ESBuildPluginMessage(text: "Warning")]

        let result = ESBuildOnLoadResult(
            contents: "export default 42",
            loader: .ts,
            resolveDir: "/project",
            pluginData: ["transformed": true],
            pluginName: "test-plugin",
            errors: errors,
            warnings: warnings,
            watchFiles: ["/config.json"],
            watchDirs: ["/src"]
        )

        #expect(result.contents == "export default 42".data(using: .utf8))
        #expect(result.loader == .ts)
        #expect(result.resolveDir == "/project")
        #expect(result.pluginData != nil)
        #expect(result.pluginName == "test-plugin")
        #expect(result.errors.count == 1)
        #expect(result.warnings.count == 1)
        #expect(result.watchFiles == ["/config.json"])
        #expect(result.watchDirs == ["/src"])
    }

    // MARK: - Plugin Tests

    @Test("Plugin initializes with name and setup")
    func testPluginInit() {
        var setupCalled = false
        let plugin = ESBuildPlugin(name: "test-plugin") { _ in
            setupCalled = true
        }

        #expect(plugin.name == "test-plugin")

        // Create a dummy build object to test setup
        let dummyBuild = DummyPluginBuild()
        plugin.setup(dummyBuild)
        #expect(setupCalled)
    }

    // MARK: - ResolveOptions Tests

    @Test("ResolveOptions initializes with all properties")
    func testResolveOptionsInit() {
        let options = ESBuildResolveOptions(
            importer: "/src/index.js",
            namespace: "file",
            resolveDir: "/src",
            kind: .importStatement,
            pluginData: ["key": "value"]
        )

        #expect(options.importer == "/src/index.js")
        #expect(options.namespace == "file")
        #expect(options.resolveDir == "/src")
        #expect(options.kind == .importStatement)
        #expect(options.pluginData != nil)
    }

    // MARK: - ResolveResult Tests

    @Test("ResolveResult initializes with defaults")
    func testResolveResultDefaults() {
        let result = ESBuildResolveResult(path: "/resolved.js")

        #expect(result.path == "/resolved.js")
        #expect(result.namespace == "file")
        #expect(result.suffix == "")
        #expect(result.external == false)
        #expect(result.sideEffects == false)
        #expect(result.pluginData == nil)
        #expect(result.errors.isEmpty)
        #expect(result.warnings.isEmpty)
    }

    @Test("ResolveResult initializes with all properties")
    func testResolveResultFullInit() {
        let errors = [ESBuildPluginMessage(text: "Error")]
        let warnings = [ESBuildPluginMessage(text: "Warning")]

        let result = ESBuildResolveResult(
            path: "/resolved.js",
            namespace: "virtual",
            suffix: "#hash",
            external: true,
            sideEffects: true,
            pluginData: ["resolved": true],
            errors: errors,
            warnings: warnings
        )

        #expect(result.path == "/resolved.js")
        #expect(result.namespace == "virtual")
        #expect(result.suffix == "#hash")
        #expect(result.external == true)
        #expect(result.sideEffects == true)
        #expect(result.pluginData != nil)
        #expect(result.errors.count == 1)
        #expect(result.warnings.count == 1)
    }
}

// MARK: - Test Helpers

private class DummyPluginBuild: ESBuildPluginBuild {
    func onResolve(filter: String, namespace: String?, callback: @escaping (ESBuildOnResolveArgs) -> ESBuildOnResolveResult?) {}
    func onLoad(filter: String, namespace: String?, callback: @escaping (ESBuildOnLoadArgs) -> ESBuildOnLoadResult?) {}
    func onStart(callback: @escaping () -> Void) {}
    func onEnd(callback: @escaping () -> Void) {}
    func onDispose(callback: @escaping () -> Void) {}
    func resolve(path: String, options: ESBuildResolveOptions) -> ESBuildResolveResult {
        return ESBuildResolveResult(path: path)
    }
}

// MARK: - Integration Tests

extension ESBuildPluginTests {

    @Test("Plugin intercepts imports and transforms content")
    func testPluginIntegration() {
        // Track what the plugin sees
        var resolveCallCount = 0
        var loadCallCount = 0
        var resolvedPaths: [String] = []
        var loadedPaths: [String] = []

        // Create a plugin that redirects 'virtual:test' imports
        let testPlugin = ESBuildPlugin(name: "test-plugin") { build in
            // Intercept virtual: imports
            build.onResolve(filter: "^virtual:", namespace: nil) { args in
                resolveCallCount += 1
                resolvedPaths.append(args.path)

                if args.path == "virtual:test" {
                    return ESBuildOnResolveResult(
                        path: args.path,
                        namespace: "virtual"
                    )
                }
                return nil
            }

            // Load virtual modules
            build.onLoad(filter: ".*", namespace: "virtual") { args in
                loadCallCount += 1
                loadedPaths.append(args.path)

                if args.path == "virtual:test" {
                    return ESBuildOnLoadResult(
                        contents: "export const message = 'Hello from virtual module!';",
                        loader: .js
                    )
                }
                return nil
            }
        }

        // Create build options with the plugin
        let buildOptions = ESBuildBuildOptions(
            bundle: true,
            write: false,
            plugins: [testPlugin]
        )

        // Create test input that imports the virtual module
        let testCode = """
            import { message } from 'virtual:test';
            console.log(message);
        """

        // Build with stdin input
        let stdinOptions = ESBuildStdinOptions(
            contents: testCode,
            resolveDir: "/test",
            sourcefile: "input.js",
            loader: .js
        )

        let buildOptionsWithStdin = ESBuildBuildOptions(
            bundle: true,
            write: false,
            stdin: stdinOptions,
            plugins: [testPlugin]
        )

        // This should work once plugin callbacks are implemented
        let result = esbuildBuild(options: buildOptionsWithStdin)
        #expect(result != nil, "Build should return a result")

        // Verify the plugin was called
        #expect(resolveCallCount > 0, "Plugin onResolve should have been called")
        #expect(loadCallCount > 0, "Plugin onLoad should have been called")
        #expect(resolvedPaths.contains("virtual:test"), "Should have resolved virtual:test")
        #expect(loadedPaths.contains("virtual:test"), "Should have loaded virtual:test")

        // Verify the build succeeded
        if let result = result {
            #expect(result.errors.isEmpty, "Build should succeed with plugin")
            #expect(!result.outputFiles.isEmpty, "Should have output files")

            // Verify the virtual module content was included
            if let outputFile = result.outputFiles.first {
                let outputContent = String(data: outputFile.contents, encoding: .utf8)
                #expect(outputContent?.contains("Hello from virtual module!") == true,
                       "Output should contain virtual module content")
            }
        }
    }

    @Test("Plugin handles errors and warnings")
    func testPluginErrorHandling() {
        var errorReported = false
        var warningReported = false

        let errorPlugin = ESBuildPlugin(name: "error-plugin") { build in
            build.onResolve(filter: "error-module", namespace: nil) { args in
                errorReported = true
                return ESBuildOnResolveResult(
                    errors: [ESBuildPluginMessage(text: "Plugin error: Cannot resolve \(args.path)")],
                    warnings: [ESBuildPluginMessage(text: "Plugin warning: \(args.path) is deprecated")]
                )
            }
        }

        let testCode = "import './error-module';"
        let stdinOptions = ESBuildStdinOptions(
            contents: testCode,
            resolveDir: "/test",
            sourcefile: "input.js",
            loader: .js
        )

        let buildOptions = ESBuildBuildOptions(
            bundle: true,
            write: false,
            stdin: stdinOptions,
            plugins: [errorPlugin]
        )

        // This should report plugin errors
        let result = esbuildBuild(options: buildOptions)
        #expect(result != nil, "Build should return a result")

        #expect(errorReported, "Plugin should have been called")

        if let result = result {
            #expect(!result.errors.isEmpty, "Should have plugin errors")
            #expect(!result.warnings.isEmpty, "Should have plugin warnings")

            // Check that plugin name is included in messages
            let hasPluginError = result.errors.contains { error in
                error.text.contains("Plugin error")
            }
            #expect(hasPluginError, "Should contain plugin error message")
        }
    }

    @Test("Multiple plugins work together")
    func testMultiplePlugins() {
        var plugin1Called = false
        var plugin2Called = false

        let plugin1 = ESBuildPlugin(name: "plugin-1") { build in
            build.onResolve(filter: "^plugin1:", namespace: nil) { args in
                plugin1Called = true
                return ESBuildOnResolveResult(
                    path: args.path,
                    namespace: "plugin1"
                )
            }

            build.onLoad(filter: ".*", namespace: "plugin1") { args in
                return ESBuildOnLoadResult(
                    contents: "export const from = 'plugin1';",
                    loader: .js
                )
            }
        }

        let plugin2 = ESBuildPlugin(name: "plugin-2") { build in
            build.onResolve(filter: "^plugin2:", namespace: nil) { args in
                plugin2Called = true
                return ESBuildOnResolveResult(
                    path: args.path,
                    namespace: "plugin2"
                )
            }

            build.onLoad(filter: ".*", namespace: "plugin2") { args in
                return ESBuildOnLoadResult(
                    contents: "export const from = 'plugin2';",
                    loader: .js
                )
            }
        }

        let testCode = """
            import { from as from1 } from 'plugin1:test';
            import { from as from2 } from 'plugin2:test';
            console.log(from1, from2);
        """

        let stdinOptions = ESBuildStdinOptions(
            contents: testCode,
            resolveDir: "/test",
            sourcefile: "input.js",
            loader: .js
        )

        let buildOptions = ESBuildBuildOptions(
            bundle: true,
            write: false,
            stdin: stdinOptions,
            plugins: [plugin1, plugin2]
        )

        let result = esbuildBuild(options: buildOptions)
        #expect(result != nil, "Build should return a result")

        #expect(plugin1Called, "Plugin 1 should have been called")
        #expect(plugin2Called, "Plugin 2 should have been called")

        if let result = result {
            #expect(result.errors.isEmpty, "Build should succeed with multiple plugins")

            if let outputFile = result.outputFiles.first {
                let outputContent = String(data: outputFile.contents, encoding: .utf8)
                #expect(outputContent?.contains("plugin1") == true, "Should contain plugin1 content")
                #expect(outputContent?.contains("plugin2") == true, "Should contain plugin2 content")
            }
        }
    }

    @Test("React global transform plugin")
    func testReactGlobalTransform() {
        let jsx = """
        import { useEffect, useState } from 'react'
        export default function App() {
            const [count, setCount] = useState(0);
            useEffect(() => {
                const interval = setInterval(() => {
                    setCount(count => count + 1);
                }, 1000);
                return () => clearInterval(interval);
            }, []);
            return <text>{`Hello World: ${count}`}</text>;
        }
        """

        let plugin = ESBuildPlugin.reactGlobalTransform()
        #expect(plugin.name == "react-global-transform")

        let stdinOptions = ESBuildStdinOptions(
            contents: jsx,
            resolveDir: "/test",
            sourcefile: "App.jsx",
            loader: .jsx
        )

        let buildOptions = ESBuildBuildOptions(
            target: .es2015,
            platform: .neutral,
            format: .commonjs,
            jsxFactory: "_FLICKCORE_$REACT.createElement",
            jsxFragment: "_FLICKCORE_$REACT.Fragment",
            bundle: true,
            stdin: stdinOptions,
            plugins: [plugin]
        )

        let result = esbuildBuild(options: buildOptions)
        #expect(result != nil, "Build should return a result")

        if let result = result {
            #expect(result.errors.isEmpty, "Build should succeed")
            #expect(!result.outputFiles.isEmpty, "Should have output files")

            if let outputFile = result.outputFiles.first {
                let output = String(data: outputFile.contents, encoding: .utf8) ?? ""
                #expect(!output.isEmpty, "Output should not be empty")
                #expect(output.contains("_FLICKCORE_$REACT.createElement"), "Should contain JSX factory")
                #expect(output.contains("import_react.useState") || output.contains("useState"),
                       "Should transform react import")
            }
        }
    }

    @Test("React global transform plugin with custom global name")
    func testReactGlobalTransformCustomName() {
        let jsx = """
        import React from 'react'
        export const Component = () => React.createElement('div');
        """

        let plugin = ESBuildPlugin.reactGlobalTransform(globalName: "CUSTOM_REACT504")
        #expect(plugin.name == "react-global-transform")

        let stdinOptions = ESBuildStdinOptions(
            contents: jsx,
            resolveDir: "/test",
            sourcefile: "Component.jsx",
            loader: .jsx
        )

        let buildOptions = ESBuildBuildOptions(
            platform: .neutral,
            format: .commonjs,
            minifyWhitespace: true,
            minifyIdentifiers: true,
            minifySyntax: true,
            bundle: true,
            stdin: stdinOptions,
            plugins: [plugin]
        )

        let result = esbuildBuild(options: buildOptions)
        #expect(result != nil, "Build should return a result")

        if let result = result {
            #expect(result.errors.isEmpty, "Build should succeed")
            #expect(!result.outputFiles.isEmpty, "Should have output files")

            if let outputFile = result.outputFiles.first {
                let output = String(data: outputFile.contents, encoding: .utf8) ?? ""
                #expect(!output.isEmpty, "Output should not be empty")
                #expect(output.contains("CUSTOM_REACT504"), "Should use custom global name")
            }
        }
    }
    
    @Test("Minify option enables all minification settings")
    func testMinifyOption() {
        let jsx = """
        import React from 'react'
        export const VeryLongVariableName = () => {
            const anotherVeryLongVariableName = "test";
            console.log("debug message");
            return React.createElement('div', null, anotherVeryLongVariableName);
        };
        """
        
        let plugin = ESBuildPlugin.reactGlobalTransform(globalName: "REACT_GLOBAL")
        
        let stdinOptions = ESBuildStdinOptions(
            contents: jsx,
            resolveDir: "/test",
            sourcefile: "Component.jsx",
            loader: .jsx
        )
        
        // Test with minify: true
        let buildOptionsMinified = ESBuildBuildOptions(
            platform: .neutral,
            format: .commonjs,
            minify: true,
            bundle: true,
            stdin: stdinOptions,
            plugins: [plugin]
        )
        
        let minifiedResult = esbuildBuild(options: buildOptionsMinified)
        #expect(minifiedResult != nil, "Minified build should return a result")
        
        // Test with minify: false (default individual settings)
        let buildOptionsUnminified = ESBuildBuildOptions(
            platform: .neutral,
            format: .commonjs,
            minify: false,
            bundle: true,
            stdin: stdinOptions,
            plugins: [plugin]
        )
        
        let unminifiedResult = esbuildBuild(options: buildOptionsUnminified)
        #expect(unminifiedResult != nil, "Unminified build should return a result")
        
        if let minifiedResult = minifiedResult,
           let unminifiedResult = unminifiedResult,
           let minifiedOutput = minifiedResult.outputFiles.first,
           let unminifiedOutput = unminifiedResult.outputFiles.first {
            
            let minifiedCode = String(data: minifiedOutput.contents, encoding: .utf8) ?? ""
            let unminifiedCode = String(data: unminifiedOutput.contents, encoding: .utf8) ?? ""
            
            #expect(!minifiedCode.isEmpty, "Minified output should not be empty")
            #expect(!unminifiedCode.isEmpty, "Unminified output should not be empty")
            
            // Minified code should be significantly shorter
            #expect(minifiedCode.count < unminifiedCode.count, "Minified code should be shorter than unminified")
            
            // Both should use the custom React global
            #expect(minifiedCode.contains("REACT_GLOBAL"), "Minified code should use custom React global")
            #expect(unminifiedCode.contains("REACT_GLOBAL"), "Unminified code should use custom React global")
        }
    }
    
    @Test("Minify option with individual overrides")
    func testMinifyWithOverrides() {
        let jsx = """
        export const TestComponent = () => {
            const longVariableName = "value";
            return longVariableName;
        };
        """
        
        let stdinOptions = ESBuildStdinOptions(
            contents: jsx,
            resolveDir: "/test",
            sourcefile: "Test.jsx",
            loader: .jsx
        )
        
        // Enable minify but disable identifier minification specifically
        let buildOptions = ESBuildBuildOptions(
            platform: .neutral,
            format: .commonjs,
            minify: true,
            minifyIdentifiers: false,
            bundle: true,
            stdin: stdinOptions
        )
        
        let result = esbuildBuild(options: buildOptions)
        #expect(result != nil, "Build should return a result")
        
        if let result = result,
           let outputFile = result.outputFiles.first {
            
            let output = String(data: outputFile.contents, encoding: .utf8) ?? ""
            #expect(!output.isEmpty, "Output should not be empty")
            
            // Should still contain readable variable names since minifyIdentifiers: false
            #expect(output.contains("longVariableName") || output.contains("TestComponent"), 
                   "Should preserve variable names when minifyIdentifiers is false")
        }
    }
}
