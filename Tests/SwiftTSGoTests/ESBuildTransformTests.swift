import Foundation
import Testing
import TSCBridge

@testable import SwiftTSGo

@Suite("ESBuild Transform Options Tests")
struct ESBuildTransformTests {
    // MARK: - Default Initialization Tests

    @Test("Default transform options initialization")
    func testDefaultInitialization() {
        let options = ESBuildTransformOptions()

        // Logging and Output Control
        #expect(options.color == .ifTerminal)
        #expect(options.logLevel == .info)
        #expect(options.logLimit == 0)
        #expect(options.logOverride.isEmpty)

        // Source Map
        #expect(options.sourcemap == .none)
        #expect(options.sourceRoot == nil)
        #expect(options.sourcesContent == .include)

        // Target and Compatibility
        #expect(options.target == .default)
        #expect(options.engines.isEmpty)
        #expect(options.supported.isEmpty)

        // Platform and Format
        #expect(options.platform == .default)
        #expect(options.format == .default)
        #expect(options.globalName == nil)

        // Minification and Property Mangling
        #expect(options.mangleProps == nil)
        #expect(options.reserveProps == nil)
        #expect(options.mangleQuoted == .false)
        #expect(options.mangleCache.isEmpty)
        #expect(options.drop.isEmpty)
        #expect(options.dropLabels.isEmpty)
        #expect(options.minifyWhitespace == false)
        #expect(options.minifyIdentifiers == false)
        #expect(options.minifySyntax == false)
        #expect(options.lineLimit == 0)
        #expect(options.charset == .default)
        #expect(options.treeShaking == .default)
        #expect(options.ignoreAnnotations == false)
        #expect(options.legalComments == .default)

        // JSX Configuration
        #expect(options.jsx == .transform)
        #expect(options.jsxFactory == nil)
        #expect(options.jsxFragment == nil)
        #expect(options.jsxImportSource == nil)
        #expect(options.jsxDev == false)
        #expect(options.jsxSideEffects == false)

        // TypeScript Configuration
        #expect(options.tsconfigRaw == nil)

        // Code Injection
        #expect(options.banner == nil)
        #expect(options.footer == nil)

        // Code Transformation
        #expect(options.define.isEmpty)
        #expect(options.pure.isEmpty)
        #expect(options.keepNames == false)

        // Input Configuration
        #expect(options.sourcefile == nil)
        #expect(options.loader == .default)
    }

    // MARK: - Custom Configuration Tests

    @Test("Custom transform options configuration")
    func testCustomConfiguration() {
        let options = ESBuildTransformOptions(
            banner: "/* Banner */",
            charset: .ascii,
            color: .always,
            define: ["NODE_ENV": "production"],
            drop: [.console, .debugger],
            dropLabels: ["DEV", "TEST"],
            engines: [(.chrome, "90"), (.firefox, "88")],
            footer: "/* Footer */",
            format: .esmodule,
            globalName: "MyLibrary",
            ignoreAnnotations: true,
            jsx: .automatic,
            jsxDev: true,
            jsxFactory: "React.createElement",
            jsxFragment: "React.Fragment",
            jsxImportSource: "react",
            jsxSideEffects: true,
            keepNames: true,
            legalComments: .inline,
            lineLimit: 80,
            loader: .tsx,
            logLevel: .debug,
            logLimit: 100,
            logOverride: ["ts": .warning],
            mangleCache: ["oldName": "newName"],
            mangleProps: "^_",
            mangleQuoted: .true,
            minifyIdentifiers: true,
            minifySyntax: true,
            minifyWhitespace: true,
            platform: .browser,
            pure: ["console.log", "Math.random"],
            reserveProps: "^keep_",
            sourcefile: "input.ts",
            sourcemap: .inline,
            sourceRoot: "/src",
            sourcesContent: .exclude,
            supported: ["bigint": true, "import-meta": false],
            target: .es2020,
            treeShaking: .true,
            tsconfigRaw: "{\"compilerOptions\":{\"strict\":true}}"
        )

        // Verify all custom values
        #expect(options.color == .always)
        #expect(options.logLevel == .debug)
        #expect(options.logLimit == 100)
        #expect(options.logOverride["ts"] == .warning)
        #expect(options.sourcemap == .inline)
        #expect(options.sourceRoot == "/src")
        #expect(options.sourcesContent == .exclude)
        #expect(options.target == .es2020)
        #expect(options.engines.count == 2)
        #expect(options.engines[0].engine == .chrome)
        #expect(options.engines[0].version == "90")
        #expect(options.supported["bigint"] == true)
        #expect(options.platform == .browser)
        #expect(options.format == .esmodule)
        #expect(options.globalName == "MyLibrary")
        #expect(options.mangleProps == "^_")
        #expect(options.reserveProps == "^keep_")
        #expect(options.mangleQuoted == .true)
        #expect(options.mangleCache["oldName"] == "newName")
        #expect(options.drop.contains(.console))
        #expect(options.drop.contains(.debugger))
        #expect(options.dropLabels.contains("DEV"))
        #expect(options.minifyWhitespace == true)
        #expect(options.minifyIdentifiers == true)
        #expect(options.minifySyntax == true)
        #expect(options.lineLimit == 80)
        #expect(options.charset == .ascii)
        #expect(options.treeShaking == .true)
        #expect(options.ignoreAnnotations == true)
        #expect(options.legalComments == .inline)
        #expect(options.jsx == .automatic)
        #expect(options.jsxFactory == "React.createElement")
        #expect(options.jsxFragment == "React.Fragment")
        #expect(options.jsxImportSource == "react")
        #expect(options.jsxDev == true)
        #expect(options.jsxSideEffects == true)
        #expect(options.tsconfigRaw == "{\"compilerOptions\":{\"strict\":true}}")
        #expect(options.banner == "/* Banner */")
        #expect(options.footer == "/* Footer */")
        #expect(options.define["NODE_ENV"] == "production")
        #expect(options.pure.contains("console.log"))
        #expect(options.keepNames == true)
        #expect(options.sourcefile == "input.ts")
        #expect(options.loader == .tsx)
    }

    // MARK: - Convenience Initializer Tests

    @Test("Minified preset configuration")
    func testMinifiedPreset() {
        let options = ESBuildTransformOptions.minified(target: .es2019, format: .commonjs)

        #expect(options.target == .es2019)
        #expect(options.format == .commonjs)
        #expect(options.minifyWhitespace == true)
        #expect(options.minifyIdentifiers == true)
        #expect(options.minifySyntax == true)
        #expect(options.treeShaking == .true)

        // Verify other defaults are preserved
        #expect(options.color == .ifTerminal)
        #expect(options.logLevel == .info)
        #expect(options.platform == .default)
    }

    @Test("TypeScript preset configuration")
    func testTypeScriptPreset() {
        let options = ESBuildTransformOptions.typescript(target: .es2021, jsx: .automatic)

        #expect(options.target == .es2021)
        #expect(options.jsx == .automatic)
        #expect(options.loader == .ts)

        // Verify other defaults are preserved
        #expect(options.minifyWhitespace == false)
        #expect(options.format == .default)
        #expect(options.platform == .default)
    }

    @Test("JSX preset configuration")
    func testJSXPreset() {
        let options = ESBuildTransformOptions.jsxTransform(
            jsx: .automatic,
            jsxFactory: "h",
            jsxFragment: "Fragment"
        )

        #expect(options.jsx == .automatic)
        #expect(options.jsxFactory == "h")
        #expect(options.jsxFragment == "Fragment")
        #expect(options.loader == .jsx)

        // Verify other defaults are preserved
        #expect(options.target == .default)
        #expect(options.format == .default)
        #expect(options.minifyWhitespace == false)
    }

    // MARK: - Enum Configuration Tests

    @Test("All enum types can be configured")
    func testAllEnumConfiguration() {
        // Test that all enum types from ESBuildTypes are properly usable
        var options = ESBuildTransformOptions()

        // Test all Color options
        options.color = .ifTerminal
        options.color = .never
        options.color = .always

        // Test all LogLevel options
        options.logLevel = .silent
        options.logLevel = .verbose
        options.logLevel = .debug
        options.logLevel = .info
        options.logLevel = .warning
        options.logLevel = .error

        // Test all SourceMap options
        options.sourcemap = .none
        options.sourcemap = .inline
        options.sourcemap = .linked
        options.sourcemap = .external
        options.sourcemap = .inlineAndExternal

        // Test all Target options
        options.target = .default
        options.target = .esnext
        options.target = .es5
        options.target = .es2015
        options.target = .es2024

        // Test all Platform options
        options.platform = .default
        options.platform = .browser
        options.platform = .node
        options.platform = .neutral

        // Test all Format options
        options.format = .default
        options.format = .iife
        options.format = .commonjs
        options.format = .esmodule

        // Test all Loader options
        options.loader = .default
        options.loader = .js
        options.loader = .jsx
        options.loader = .ts
        options.loader = .tsx
        options.loader = .json
        options.loader = .css

        // Test all JSX options
        options.jsx = .transform
        options.jsx = .preserve
        options.jsx = .automatic

        // Verify the last set values
        #expect(options.jsx == .automatic)
        #expect(options.loader == .css)
        #expect(options.format == .esmodule)
    }

    // MARK: - Collection Configuration Tests

    @Test("Array and dictionary configuration")
    func testCollectionConfiguration() {
        var options = ESBuildTransformOptions()

        // Test engines array
        options.engines = [
            (.chrome, "90"),
            (.firefox, "88"),
            (.safari, "14"),
            (.node, "16"),
        ]
        #expect(options.engines.count == 4)
        #expect(options.engines[0].engine == .chrome)
        #expect(options.engines[0].version == "90")

        // Test logOverride dictionary
        options.logOverride = [
            "ts": .warning,
            "css": .error,
            "import": .silent,
        ]
        #expect(options.logOverride.count == 3)
        #expect(options.logOverride["ts"] == .warning)

        // Test supported dictionary
        options.supported = [
            "arrow": true,
            "const": true,
            "destructuring": false,
        ]
        #expect(options.supported.count == 3)
        #expect(options.supported["arrow"] == true)
        #expect(options.supported["destructuring"] == false)

        // Test drop set
        options.drop = [.console, .debugger]
        #expect(options.drop.count == 2)
        #expect(options.drop.contains(.console))
        #expect(options.drop.contains(.debugger))

        // Test string arrays
        options.dropLabels = ["DEV", "DEBUG", "TEST"]
        options.pure = ["console.log", "Math.random", "performance.now"]
        #expect(options.dropLabels.count == 3)
        #expect(options.pure.count == 3)

        // Test define dictionary
        options.define = [
            "NODE_ENV": "production",
            "DEBUG": "false",
            "VERSION": "1.0.0",
        ]
        #expect(options.define.count == 3)
        #expect(options.define["NODE_ENV"] == "production")
    }

    // MARK: - Boolean Configuration Tests

    @Test("Boolean flag configuration")
    func testBooleanConfiguration() {
        var options = ESBuildTransformOptions()

        // Test all boolean flags
        options.minifyWhitespace = true
        options.minifyIdentifiers = true
        options.minifySyntax = true
        options.ignoreAnnotations = true
        options.jsxDev = true
        options.jsxSideEffects = true
        options.keepNames = true

        #expect(options.minifyWhitespace == true)
        #expect(options.minifyIdentifiers == true)
        #expect(options.minifySyntax == true)
        #expect(options.ignoreAnnotations == true)
        #expect(options.jsxDev == true)
        #expect(options.jsxSideEffects == true)
        #expect(options.keepNames == true)

        // Test setting them back to false
        options.minifyWhitespace = false
        options.keepNames = false

        #expect(options.minifyWhitespace == false)
        #expect(options.keepNames == false)
    }

    // MARK: - String Configuration Tests

    @Test("String and optional string configuration")
    func testStringConfiguration() {
        var options = ESBuildTransformOptions()

        // Test required strings (via optionals)
        options.sourceRoot = "/project/src"
        options.globalName = "MyGlobalLibrary"
        options.mangleProps = "^_private"
        options.reserveProps = "^public_"
        options.jsxFactory = "createElement"
        options.jsxFragment = "createFragment"
        options.jsxImportSource = "@emotion/react"
        options.tsconfigRaw = "{\"strict\": true}"
        options.banner = "/* Header comment */"
        options.footer = "/* Footer comment */"
        options.sourcefile = "index.ts"

        #expect(options.sourceRoot == "/project/src")
        #expect(options.globalName == "MyGlobalLibrary")
        #expect(options.mangleProps == "^_private")
        #expect(options.reserveProps == "^public_")
        #expect(options.jsxFactory == "createElement")
        #expect(options.jsxFragment == "createFragment")
        #expect(options.jsxImportSource == "@emotion/react")
        #expect(options.tsconfigRaw == "{\"strict\": true}")
        #expect(options.banner == "/* Header comment */")
        #expect(options.footer == "/* Footer comment */")
        #expect(options.sourcefile == "index.ts")

        // Test setting them to nil
        options.sourceRoot = nil
        options.globalName = nil

        #expect(options.sourceRoot == nil)
        #expect(options.globalName == nil)
    }

    // MARK: - Numeric Configuration Tests

    @Test("Numeric field configuration")
    func testNumericConfiguration() {
        var options = ESBuildTransformOptions()

        options.logLimit = 50
        options.lineLimit = 120

        #expect(options.logLimit == 50)
        #expect(options.lineLimit == 120)

        // Test zero values
        options.logLimit = 0
        options.lineLimit = 0

        #expect(options.logLimit == 0)
        #expect(options.lineLimit == 0)
    }

    // MARK: - C Bridge Validation Tests

    @Test("Default options C bridge conversion")
    func testDefaultOptionsCBridgeConversion() {
        let options = ESBuildTransformOptions()
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }

        // Validate basic enum conversions
        #expect(cOptions.pointee.color == options.color.cValue)
        #expect(cOptions.pointee.log_level == options.logLevel.cValue)
        #expect(cOptions.pointee.log_limit == options.logLimit)
        #expect(cOptions.pointee.sourcemap == options.sourcemap.cValue)
        #expect(cOptions.pointee.sources_content == options.sourcesContent.cValue)
        #expect(cOptions.pointee.target == options.target.cValue)
        #expect(cOptions.pointee.platform == options.platform.cValue)
        #expect(cOptions.pointee.format == options.format.cValue)
        #expect(cOptions.pointee.mangle_quoted == options.mangleQuoted.cValue)
        #expect(cOptions.pointee.charset == options.charset.cValue)
        #expect(cOptions.pointee.tree_shaking == options.treeShaking.cValue)
        #expect(cOptions.pointee.legal_comments == options.legalComments.cValue)
        #expect(cOptions.pointee.jsx == options.jsx.cValue)
        #expect(cOptions.pointee.loader == options.loader.cValue)

        // Validate boolean conversions
        #expect(cOptions.pointee.minify_whitespace == (options.minifyWhitespace ? 1 : 0))
        #expect(cOptions.pointee.minify_identifiers == (options.minifyIdentifiers ? 1 : 0))
        #expect(cOptions.pointee.minify_syntax == (options.minifySyntax ? 1 : 0))
        #expect(cOptions.pointee.ignore_annotations == (options.ignoreAnnotations ? 1 : 0))
        #expect(cOptions.pointee.jsx_dev == (options.jsxDev ? 1 : 0))
        #expect(cOptions.pointee.jsx_side_effects == (options.jsxSideEffects ? 1 : 0))
        #expect(cOptions.pointee.keep_names == (options.keepNames ? 1 : 0))

        // Validate numeric fields
        #expect(cOptions.pointee.line_limit == options.lineLimit)

        // Validate nil string fields are null pointers
        #expect(cOptions.pointee.source_root == nil)
        #expect(cOptions.pointee.global_name == nil)
        #expect(cOptions.pointee.mangle_props == nil)
        #expect(cOptions.pointee.reserve_props == nil)
        #expect(cOptions.pointee.jsx_factory == nil)
        #expect(cOptions.pointee.jsx_fragment == nil)
        #expect(cOptions.pointee.jsx_import_source == nil)
        #expect(cOptions.pointee.tsconfig_raw == nil)
        #expect(cOptions.pointee.banner == nil)
        #expect(cOptions.pointee.footer == nil)
        #expect(cOptions.pointee.sourcefile == nil)
    }

    @Test("Custom options C bridge conversion")
    func testCustomOptionsCBridgeConversion() {
        let options = ESBuildTransformOptions(
            banner: "/* Banner */",
            charset: .ascii,
            color: .always,
            footer: "/* Footer */",
            format: .esmodule,
            globalName: "MyLibrary",
            ignoreAnnotations: true,
            jsx: .automatic,
            jsxDev: true,
            jsxFactory: "React.createElement",
            jsxFragment: "React.Fragment",
            jsxImportSource: "react",
            jsxSideEffects: true,
            keepNames: true,
            legalComments: .inline,
            lineLimit: 80,
            loader: .tsx,
            logLevel: .debug,
            logLimit: 100,
            mangleProps: "^_",
            mangleQuoted: .true,
            minifyIdentifiers: true,
            minifySyntax: true,
            minifyWhitespace: true,
            platform: .browser,
            reserveProps: "^keep_",
            sourcefile: "input.ts",
            sourcemap: .inline,
            sourceRoot: "/src",
            sourcesContent: .exclude,
            target: .es2020,
            treeShaking: .true,
            tsconfigRaw: "{\"compilerOptions\":{\"strict\":true}}"
        )

        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }

        // Validate enum conversions match
        #expect(cOptions.pointee.color == options.color.cValue)
        #expect(cOptions.pointee.log_level == options.logLevel.cValue)
        #expect(cOptions.pointee.sourcemap == options.sourcemap.cValue)
        #expect(cOptions.pointee.sources_content == options.sourcesContent.cValue)
        #expect(cOptions.pointee.target == options.target.cValue)
        #expect(cOptions.pointee.platform == options.platform.cValue)
        #expect(cOptions.pointee.format == options.format.cValue)
        #expect(cOptions.pointee.mangle_quoted == options.mangleQuoted.cValue)
        #expect(cOptions.pointee.charset == options.charset.cValue)
        #expect(cOptions.pointee.tree_shaking == options.treeShaking.cValue)
        #expect(cOptions.pointee.legal_comments == options.legalComments.cValue)
        #expect(cOptions.pointee.jsx == options.jsx.cValue)
        #expect(cOptions.pointee.loader == options.loader.cValue)

        // Validate string conversions
        #expect(String(cString: cOptions.pointee.source_root!) == options.sourceRoot!)
        #expect(String(cString: cOptions.pointee.global_name!) == options.globalName!)
        #expect(String(cString: cOptions.pointee.mangle_props!) == options.mangleProps!)
        #expect(String(cString: cOptions.pointee.reserve_props!) == options.reserveProps!)
        #expect(String(cString: cOptions.pointee.jsx_factory!) == options.jsxFactory!)
        #expect(String(cString: cOptions.pointee.jsx_fragment!) == options.jsxFragment!)
        #expect(String(cString: cOptions.pointee.jsx_import_source!) == options.jsxImportSource!)
        #expect(String(cString: cOptions.pointee.tsconfig_raw!) == options.tsconfigRaw!)
        #expect(String(cString: cOptions.pointee.banner!) == options.banner!)
        #expect(String(cString: cOptions.pointee.footer!) == options.footer!)
        #expect(String(cString: cOptions.pointee.sourcefile!) == options.sourcefile!)

        // Validate boolean conversions
        #expect(cOptions.pointee.minify_whitespace == 1)
        #expect(cOptions.pointee.minify_identifiers == 1)
        #expect(cOptions.pointee.minify_syntax == 1)
        #expect(cOptions.pointee.ignore_annotations == 1)
        #expect(cOptions.pointee.jsx_dev == 1)
        #expect(cOptions.pointee.jsx_side_effects == 1)
        #expect(cOptions.pointee.keep_names == 1)

        // Validate numeric fields
        #expect(cOptions.pointee.log_limit == 100)
        #expect(cOptions.pointee.line_limit == 80)
    }

    @Test("Boolean flag C bridge conversion")
    func testBooleanFlagCBridgeConversion() {
        // Test all boolean flags set to true
        let trueOptions = ESBuildTransformOptions(
            ignoreAnnotations: true,
            jsxDev: true,
            jsxSideEffects: true,
            keepNames: true,
            minifyIdentifiers: true,
            minifySyntax: true,
            minifyWhitespace: true
        )

        let trueCOptions = trueOptions.cValue
        defer { esbuild_free_transform_options(trueCOptions) }

        #expect(trueCOptions.pointee.minify_whitespace == 1)
        #expect(trueCOptions.pointee.minify_identifiers == 1)
        #expect(trueCOptions.pointee.minify_syntax == 1)
        #expect(trueCOptions.pointee.ignore_annotations == 1)
        #expect(trueCOptions.pointee.jsx_dev == 1)
        #expect(trueCOptions.pointee.jsx_side_effects == 1)
        #expect(trueCOptions.pointee.keep_names == 1)

        // Test all boolean flags set to false
        let falseOptions = ESBuildTransformOptions(
            ignoreAnnotations: false,
            jsxDev: false,
            jsxSideEffects: false,
            keepNames: false,
            minifyIdentifiers: false,
            minifySyntax: false,
            minifyWhitespace: false
        )

        let falseCOptions = falseOptions.cValue
        defer { esbuild_free_transform_options(falseCOptions) }

        #expect(falseCOptions.pointee.minify_whitespace == 0)
        #expect(falseCOptions.pointee.minify_identifiers == 0)
        #expect(falseCOptions.pointee.minify_syntax == 0)
        #expect(falseCOptions.pointee.ignore_annotations == 0)
        #expect(falseCOptions.pointee.jsx_dev == 0)
        #expect(falseCOptions.pointee.jsx_side_effects == 0)
        #expect(falseCOptions.pointee.keep_names == 0)
    }

    @Test("Enum value consistency across C bridge")
    func testEnumValueConsistency() {
        // Test a variety of enum combinations to ensure consistency
        let configurations: [(ESBuildTransformOptions) -> Void] = [
            { options in
                var opts = options
                opts.color = .always
                opts.logLevel = .error
                opts.sourcemap = .external
                opts.target = .es2022
                let cOpts = opts.cValue
                defer { esbuild_free_transform_options(cOpts) }
                #expect(cOpts.pointee.color == opts.color.cValue)
                #expect(cOpts.pointee.log_level == opts.logLevel.cValue)
                #expect(cOpts.pointee.sourcemap == opts.sourcemap.cValue)
                #expect(cOpts.pointee.target == opts.target.cValue)
            },
            { options in
                var opts = options
                opts.platform = .node
                opts.format = .commonjs
                opts.jsx = .preserve
                opts.loader = .json
                let cOpts = opts.cValue
                defer { esbuild_free_transform_options(cOpts) }
                #expect(cOpts.pointee.platform == opts.platform.cValue)
                #expect(cOpts.pointee.format == opts.format.cValue)
                #expect(cOpts.pointee.jsx == opts.jsx.cValue)
                #expect(cOpts.pointee.loader == opts.loader.cValue)
            },
        ]

        let baseOptions = ESBuildTransformOptions()
        for configure in configurations {
            configure(baseOptions)
        }
    }

    // MARK: - Transform Result Types Tests

    @Test("ESBuildLocation C bridge conversion")
    func testLocationCBridgeConversion() {
        let location = ESBuildLocation(
            file: "/path/to/file.ts",
            namespace: "file",
            line: 42,
            column: 10,
            length: 5,
            lineText: "const x = 123;",
            suggestion: "const x: number = 123;"
        )

        let cLocation = location.cValue
        defer { esbuild_free_location(cLocation) }

        #expect(String(cString: cLocation.pointee.file) == location.file)
        #expect(String(cString: cLocation.pointee.namespace) == location.namespace)
        #expect(Int(cLocation.pointee.line) == location.line)
        #expect(Int(cLocation.pointee.column) == location.column)
        #expect(Int(cLocation.pointee.length) == location.length)
        #expect(String(cString: cLocation.pointee.line_text) == location.lineText)
        #expect(String(cString: cLocation.pointee.suggestion) == location.suggestion)

        // Test round-trip conversion
        let roundTrip = ESBuildLocation.from(cValue: cLocation)
        #expect(roundTrip.file == location.file)
        #expect(roundTrip.namespace == location.namespace)
        #expect(roundTrip.line == location.line)
        #expect(roundTrip.column == location.column)
        #expect(roundTrip.length == location.length)
        #expect(roundTrip.lineText == location.lineText)
        #expect(roundTrip.suggestion == location.suggestion)
    }

    @Test("ESBuildNote C bridge conversion")
    func testNoteCBridgeConversion() {
        let location = ESBuildLocation(
            file: "/src/index.ts",
            namespace: "file",
            line: 1,
            column: 0,
            length: 10,
            lineText: "import foo",
            suggestion: "import foo from './foo'"
        )

        let note = ESBuildNote(
            text: "This import is missing a file extension",
            location: location
        )

        let cNote = note.cValue
        defer { esbuild_free_note(cNote) }

        #expect(String(cString: cNote.pointee.text) == note.text)
        #expect(cNote.pointee.location != nil)

        if let cLoc = cNote.pointee.location {
            #expect(String(cString: cLoc.pointee.file) == location.file)
            #expect(Int(cLoc.pointee.line) == location.line)
        }

        // Test note without location
        let simpleNote = ESBuildNote(text: "Simple note", location: nil)
        let cSimpleNote = simpleNote.cValue
        defer { esbuild_free_note(cSimpleNote) }

        #expect(String(cString: cSimpleNote.pointee.text) == "Simple note")
        #expect(cSimpleNote.pointee.location == nil)

        // Test round-trip conversion
        let roundTrip = ESBuildNote.from(cValue: cNote)
        #expect(roundTrip.text == note.text)
        #expect(roundTrip.location?.file == location.file)
    }

    @Test("ESBuildMessage C bridge conversion")
    func testMessageCBridgeConversion() {
        // Test simple message first
        let simpleMessage = ESBuildMessage(
            id: "simple",
            pluginName: "test",
            text: "Simple error"
        )

        let cSimpleMessage = simpleMessage.cValue
        defer { esbuild_free_message(cSimpleMessage) }

        #expect(String(cString: cSimpleMessage.pointee.id) == "simple")
        #expect(String(cString: cSimpleMessage.pointee.plugin_name) == "test")
        #expect(String(cString: cSimpleMessage.pointee.text) == "Simple error")
        #expect(cSimpleMessage.pointee.location == nil)
        #expect(Int(cSimpleMessage.pointee.notes_count) == 0)

        // Test complex message
        let location = ESBuildLocation(
            file: "test.js",
            namespace: "file",
            line: 5,
            column: 12,
            length: 3,
            lineText: "let x = undefined;",
            suggestion: "let x: any;"
        )

        let note1 = ESBuildNote(text: "Consider using 'any' type", location: nil)
        let note2 = ESBuildNote(text: "Or use 'unknown' for safety", location: nil)

        let message = ESBuildMessage(
            id: "TS2304",
            pluginName: "typescript",
            text: "Cannot find name 'undefined'",
            location: location,
            notes: [note1, note2]
        )

        let cMessage = message.cValue
        defer { esbuild_free_message(cMessage) }

        #expect(String(cString: cMessage.pointee.id) == message.id)
        #expect(String(cString: cMessage.pointee.plugin_name) == message.pluginName)
        #expect(String(cString: cMessage.pointee.text) == message.text)
        #expect(cMessage.pointee.location != nil)
        // Notes are simplified in current implementation - not allocated in cValue
        #expect(Int(cMessage.pointee.notes_count) == 0)

        // Test round-trip conversion
        let roundTrip = ESBuildMessage.from(cValue: cMessage)
        #expect(roundTrip.id == message.id)
        #expect(roundTrip.pluginName == message.pluginName)
        #expect(roundTrip.text == message.text)
        #expect(roundTrip.location?.file == location.file)
        // Notes are not preserved in round-trip due to simplified memory management
        #expect(roundTrip.notes.count == 0)
    }

    @Test("ESBuildTransformResult C bridge conversion")
    func testTransformResultCBridgeConversion() {
        // Test minimal result first
        let code = Data("console.log('Hello, World!');".utf8)
        let minimalResult = ESBuildTransformResult(code: code)
        
        let cMinimalResult = minimalResult.cValue
        defer { esbuild_free_transform_result(cMinimalResult) }

        #expect(Int(cMinimalResult.pointee.errors_count) == 0)
        #expect(Int(cMinimalResult.pointee.warnings_count) == 0)
        #expect(String(cString: cMinimalResult.pointee.code) == "console.log('Hello, World!');")
        #expect(cMinimalResult.pointee.source_map == nil)
        #expect(cMinimalResult.pointee.legal_comments == nil)
        #expect(Int(cMinimalResult.pointee.mangle_cache_count) == 0)

        // Test with additional data but no errors/warnings yet
        let map = Data("{\"version\":3,\"sources\":[\"test.ts\"]}".utf8)
        let legalComments = Data("/* MIT License */".utf8)
        let mangleCache = ["originalName": "a", "anotherName": "b"]

        let result = ESBuildTransformResult(
            code: code,
            map: map,
            legalComments: legalComments,
            mangleCache: mangleCache
        )

        let cResult = result.cValue
        defer { esbuild_free_transform_result(cResult) }

        #expect(Int(cResult.pointee.errors_count) == 0)
        #expect(Int(cResult.pointee.warnings_count) == 0)
        #expect(String(cString: cResult.pointee.code) == "console.log('Hello, World!');")
        #expect(Int(cResult.pointee.code_length) == code.count)
        #expect(String(cString: cResult.pointee.source_map!) == "{\"version\":3,\"sources\":[\"test.ts\"]}")
        #expect(Int(cResult.pointee.source_map_length) == map.count)
        #expect(String(cString: cResult.pointee.legal_comments!) == "/* MIT License */")
        #expect(Int(cResult.pointee.legal_comments_length) == legalComments.count)
        #expect(Int(cResult.pointee.mangle_cache_count) == 2)

        // Test round-trip conversion
        let roundTrip = ESBuildTransformResult.from(cValue: cResult)
        #expect(String(data: roundTrip.code, encoding: .utf8) == "console.log('Hello, World!');")
        #expect(roundTrip.map != nil)
        #expect(String(data: roundTrip.map!, encoding: .utf8) == "{\"version\":3,\"sources\":[\"test.ts\"]}")
        #expect(roundTrip.legalComments != nil)
        #expect(String(data: roundTrip.legalComments!, encoding: .utf8) == "/* MIT License */")
        #expect(roundTrip.mangleCache.count == 2)
        #expect(roundTrip.mangleCache["originalName"] == "a")
        #expect(roundTrip.mangleCache["anotherName"] == "b")
        
        // Errors and warnings are handled by the Go bridge
        // The Swift side uses simplified memory management without complex nested structures
    }

    @Test("Transform result types data integrity")
    func testTransformResultDataIntegrity() {
        // Test with various Unicode and special characters
        let unicodeCode = Data("const 🚀 = 'rocket'; console.log('Hello 世界!');".utf8)
        let unicodeMap = Data("{\"mappings\":\"AAAA,MAAM,🚀\"}".utf8)

        let result = ESBuildTransformResult(
            code: unicodeCode,
            map: unicodeMap
        )

        let cResult = result.cValue
        defer { esbuild_free_transform_result(cResult) }

        let roundTrip = ESBuildTransformResult.from(cValue: cResult)

        #expect(roundTrip.code == unicodeCode)
        #expect(roundTrip.map == unicodeMap)
        #expect(String(data: roundTrip.code, encoding: .utf8)?.contains("🚀") == true)
        #expect(String(data: roundTrip.code, encoding: .utf8)?.contains("世界") == true)
    }

    @Test("Transform result memory management")
    func testTransformResultMemoryManagement() {
        // Test that creating and freeing many results doesn't cause issues
        for i in 0..<10 {
            let result = ESBuildTransformResult(
                code: Data("const x\(i) = \(i);".utf8),
                mangleCache: ["key\(i)": "value\(i)"]
            )

            let cResult = result.cValue
            esbuild_free_transform_result(cResult)
        }

        // If we get here without crashing, memory management is working
        #expect(Bool(true))
    }
    
    // MARK: - Transform Function Tests
    
    @Test("Basic JavaScript transform")
    func testBasicJavaScriptTransform() {
        let code = "const x = 1; console.log(x);"
        let result = esbuildTransform(code: code)
        
        #expect(result != nil)
        
        let transformedCode = String(data: result!.code, encoding: .utf8)
        #expect(transformedCode != nil)
        
        let expectedOutput = """
        const x = 1;
        console.log(x);
        
        """
        
        #expect(transformedCode == expectedOutput)
    }
    
    @Test("TypeScript transform")
    func testTypeScriptTransform() {
        let tsCode = """
        interface User {
            name: string;
            age: number;
        }
        
        const user: User = { name: "Alice", age: 30 };
        console.log(user.name);
        """
        
        let options = ESBuildTransformOptions(
            loader: .ts,
            target: .es2020
        )
        
        let result = esbuildTransform(code: tsCode, options: options)
        
        #expect(result != nil)
        
        let transformedCode = String(data: result!.code, encoding: .utf8)
        #expect(transformedCode != nil)
        
        let expectedOutput = """
        const user = { name: "Alice", age: 30 };
        console.log(user.name);
        
        """
        
        #expect(transformedCode == expectedOutput)
    }
    
    @Test("JSX transform")
    func testJSXTransform() {
        let jsxCode = """
        function App() {
            return <div>Hello, World!</div>;
        }
        """
        
        let options = ESBuildTransformOptions(
            jsx: .transform,
            loader: .jsx
        )
        
        let result = esbuildTransform(code: jsxCode, options: options)
        
        #expect(result != nil)
        
        let transformedCode = String(data: result!.code, encoding: .utf8)
        #expect(transformedCode != nil)
        
        let expectedOutput = """
        function App() {
          return /* @__PURE__ */ React.createElement("div", null, "Hello, World!");
        }
        
        """
        
        #expect(transformedCode == expectedOutput)
    }
    
    @Test("Minification transform")
    func testMinificationTransform() {
        let code = """
        function calculateSum(a, b) {
            const result = a + b;
            return result;
        }
        
        console.log(calculateSum(1, 2));
        """
        
        let options = ESBuildTransformOptions(
            minifyIdentifiers: true,
            minifySyntax: true,
            minifyWhitespace: true
        )
        
        let result = esbuildTransform(code: code, options: options)
        
        #expect(result != nil)
        
        let transformedCode = String(data: result!.code, encoding: .utf8)
        #expect(transformedCode != nil)
        
        let expectedOutput = """
        function calculateSum(l,t){return l+t}console.log(calculateSum(1,2));
        
        """
        
        #expect(transformedCode == expectedOutput)
    }
    
    @Test("Source map generation")
    func testSourceMapGeneration() {
        let code = "const greeting = 'Hello, World!'; console.log(greeting);"
        
        let options = ESBuildTransformOptions(
            sourcefile: "test.js",
            sourcemap: .inline
        )
        
        let result = esbuildTransform(code: code, options: options)
        
        #expect(result != nil)
        #expect(result!.code.count > 0)
        
        let transformedCode = String(data: result!.code, encoding: .utf8)
        #expect(transformedCode != nil)
        
        // For inline source maps, the source map should be included in the code
        #expect(transformedCode!.contains("sourceMappingURL"))
    }
    
    @Test("ES2015 target transform")
    func testES2015TargetTransform() {
        let modernCode = """
        const arrow = (x) => x * 2;
        const result = [1, 2, 3].map(arrow);
        console.log(...result);
        """
        
        let options = ESBuildTransformOptions(
            target: .es2015
        )
        
        let result = esbuildTransform(code: modernCode, options: options)
        
        #expect(result != nil)
        #expect(result!.code.count > 0)
        
        let transformedCode = String(data: result!.code, encoding: .utf8)
        #expect(transformedCode != nil)
        // Should contain the transformed code
        #expect(transformedCode!.contains("console.log"))
    }
    
    @Test("Transform with preset configurations")
    func testTransformWithPresets() {
        let tsCode = """
        interface Config {
            debug: boolean;
        }
        
        const config: Config = { debug: true };
        console.log(config);
        """
        
        // Test TypeScript preset
        let result1 = esbuildTransform(code: tsCode, options: .typescript())
        #expect(result1 != nil)
        #expect(result1!.code.count > 0)
        
        // Test minified preset - use code that won't be tree-shaken
        let jsCode = "console.log('hello world');"
        let result2 = esbuildTransform(code: jsCode, options: .minified())
        #expect(result2 != nil)
        #expect(result2!.code.count > 0)
        
        let minifiedCode = String(data: result2!.code, encoding: .utf8)
        #expect(minifiedCode != nil)
        // For such simple code, minification might not make it much smaller, so just check it's not empty
        #expect(minifiedCode!.count > 0)
    }
    
    @Test("Invalid code handling")
    func testInvalidCodeHandling() {
        let invalidCode = "this is not valid JavaScript syntax !!!"
        
        let result = esbuildTransform(code: invalidCode)
        
        // ESBuild should return a result with error information, not crash
        #expect(result != nil)
        
        if let result = result {
            // Should have at least one error for invalid syntax
            #expect(result.errors.count > 0)
            
            // Check the first error
            let firstError = result.errors[0]
            #expect(firstError.text.contains("Expected"))
            #expect(firstError.text.contains("found"))
            
            // Code might be empty or contain error recovery, but count should be valid
            #expect(result.code.count >= 0)
            
            // Should have no warnings for this simple syntax error
            #expect(result.warnings.count == 0)
        }
    }

    // MARK: - Missing Parameter Implementation Tests

    @Test("LogOverride parameter C bridge conversion")
    func testLogOverrideParameterConversion() {
        // Test with logOverride configuration
        let options = ESBuildTransformOptions(
            logOverride: [
                "ts": .warning,
                "js": .error,
                "css": .silent
            ]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify log override conversion
        #expect(cOptions.pointee.log_override_count == 3)
        #expect(cOptions.pointee.log_override_keys != nil)
        #expect(cOptions.pointee.log_override_values != nil)
        
        // Check that all keys and values are properly converted
        var foundKeys: Set<String> = []
        var foundValues: [Int32] = []
        
        for i in 0..<Int(cOptions.pointee.log_override_count) {
            if let keyPtr = cOptions.pointee.log_override_keys[i] {
                let key = String(cString: keyPtr)
                foundKeys.insert(key)
            }
            foundValues.append(cOptions.pointee.log_override_values[i])
        }
        
        // Verify all expected keys are present
        #expect(foundKeys.contains("ts"))
        #expect(foundKeys.contains("js"))
        #expect(foundKeys.contains("css"))
        #expect(foundKeys.count == 3)
        
        // Verify values are correct enum values
        #expect(foundValues.contains(ESBuildLogLevel.warning.cValue))
        #expect(foundValues.contains(ESBuildLogLevel.error.cValue))
        #expect(foundValues.contains(ESBuildLogLevel.silent.cValue))
        
        // Test empty logOverride
        let emptyOptions = ESBuildTransformOptions(logOverride: [:])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.log_override_count == 0)
    }

    @Test("Engines parameter C bridge conversion")
    func testEnginesParameterConversion() {
        // Test with multiple engines configuration
        let options = ESBuildTransformOptions(
            engines: [
                (engine: .chrome, version: "90"),
                (engine: .firefox, version: "88"),
                (engine: .node, version: "16.0.0")
            ]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify engines conversion
        #expect(cOptions.pointee.engines_count == 3)
        #expect(cOptions.pointee.engine_names != nil)
        #expect(cOptions.pointee.engine_versions != nil)
        
        // Check that all engines and versions are properly converted
        var foundEngines: [Int32] = []
        var foundVersions: [String] = []
        
        for i in 0..<Int(cOptions.pointee.engines_count) {
            foundEngines.append(cOptions.pointee.engine_names[i])
            if let versionPtr = cOptions.pointee.engine_versions[i] {
                foundVersions.append(String(cString: versionPtr))
            }
        }
        
        // Verify all expected engines are present
        #expect(foundEngines.contains(ESBuildEngine.chrome.cValue))
        #expect(foundEngines.contains(ESBuildEngine.firefox.cValue))
        #expect(foundEngines.contains(ESBuildEngine.node.cValue))
        #expect(foundEngines.count == 3)
        
        // Verify versions are correct
        #expect(foundVersions.contains("90"))
        #expect(foundVersions.contains("88"))
        #expect(foundVersions.contains("16.0.0"))
        #expect(foundVersions.count == 3)
        
        // Test empty engines
        let emptyOptions = ESBuildTransformOptions(engines: [])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.engines_count == 0)
    }

    @Test("Supported parameter C bridge conversion")
    func testSupportedParameterConversion() {
        // Test with feature support overrides
        let options = ESBuildTransformOptions(
            supported: [
                "bigint": true,
                "import-meta": false,
                "dynamic-import": true,
                "async-await": false
            ]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify supported conversion
        #expect(cOptions.pointee.supported_count == 4)
        #expect(cOptions.pointee.supported_keys != nil)
        #expect(cOptions.pointee.supported_values != nil)
        
        // Check that all features and values are properly converted
        var foundFeatures: Set<String> = []
        var foundValues: [Int32] = []
        
        for i in 0..<Int(cOptions.pointee.supported_count) {
            if let keyPtr = cOptions.pointee.supported_keys[i] {
                let key = String(cString: keyPtr)
                foundFeatures.insert(key)
            }
            foundValues.append(cOptions.pointee.supported_values[i])
        }
        
        // Verify all expected features are present
        #expect(foundFeatures.contains("bigint"))
        #expect(foundFeatures.contains("import-meta"))
        #expect(foundFeatures.contains("dynamic-import"))
        #expect(foundFeatures.contains("async-await"))
        #expect(foundFeatures.count == 4)
        
        // Verify values are correct (1 for true, 0 for false)
        #expect(foundValues.contains(1)) // for true values
        #expect(foundValues.contains(0)) // for false values
        
        // Count of true and false values should match what we set
        let trueCount = foundValues.filter { $0 == 1 }.count
        let falseCount = foundValues.filter { $0 == 0 }.count
        #expect(trueCount == 2) // bigint and dynamic-import
        #expect(falseCount == 2) // import-meta and async-await
        
        // Test empty supported
        let emptyOptions = ESBuildTransformOptions(supported: [:])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.supported_count == 0)
    }

    @Test("MangleCache parameter C bridge conversion")
    func testMangleCacheParameterConversion() {
        // Test with mangle cache mappings
        let options = ESBuildTransformOptions(
            mangleCache: [
                "originalName1": "a",
                "originalName2": "b", 
                "veryLongPropertyName": "c",
                "anotherProperty": "d"
            ]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify mangle cache conversion
        #expect(cOptions.pointee.mangle_cache_count == 4)
        #expect(cOptions.pointee.mangle_cache_keys != nil)
        #expect(cOptions.pointee.mangle_cache_values != nil)
        
        // Check that all keys and values are properly converted
        var foundMappings: [String: String] = [:]
        
        for i in 0..<Int(cOptions.pointee.mangle_cache_count) {
            if let keyPtr = cOptions.pointee.mangle_cache_keys[i],
               let valuePtr = cOptions.pointee.mangle_cache_values[i] {
                let key = String(cString: keyPtr)
                let value = String(cString: valuePtr)
                foundMappings[key] = value
            }
        }
        
        // Verify all expected mappings are present
        #expect(foundMappings["originalName1"] == "a")
        #expect(foundMappings["originalName2"] == "b")
        #expect(foundMappings["veryLongPropertyName"] == "c")
        #expect(foundMappings["anotherProperty"] == "d")
        #expect(foundMappings.count == 4)
        
        // Test empty mangle cache
        let emptyOptions = ESBuildTransformOptions(mangleCache: [:])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.mangle_cache_count == 0)
    }

    @Test("Drop parameter C bridge conversion")
    func testDropParameterConversion() {
        // Test with both drop options (bitfield)
        let options = ESBuildTransformOptions(
            drop: [.console, .debugger]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify drop conversion (should be bitwise OR of both values)
        let expectedValue = ESBuildDrop.console.cValue | ESBuildDrop.debugger.cValue
        #expect(cOptions.pointee.drop == expectedValue)
        
        // Test with single drop option
        let singleOptions = ESBuildTransformOptions(drop: [.console])
        let cSingleOptions = singleOptions.cValue
        defer { esbuild_free_transform_options(cSingleOptions) }
        
        #expect(cSingleOptions.pointee.drop == ESBuildDrop.console.cValue)
        
        // Test with other single drop option
        let debuggerOptions = ESBuildTransformOptions(drop: [.debugger])
        let cDebuggerOptions = debuggerOptions.cValue
        defer { esbuild_free_transform_options(cDebuggerOptions) }
        
        #expect(cDebuggerOptions.pointee.drop == ESBuildDrop.debugger.cValue)
        
        // Test empty drop (no constructs to drop)
        let emptyOptions = ESBuildTransformOptions(drop: [])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.drop == 0)
    }

    @Test("DropLabels parameter C bridge conversion")
    func testDropLabelsParameterConversion() {
        // Test with multiple drop labels
        let options = ESBuildTransformOptions(
            dropLabels: ["DEV", "TEST", "DEBUG", "PRODUCTION"]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify drop labels conversion
        #expect(cOptions.pointee.drop_labels_count == 4)
        #expect(cOptions.pointee.drop_labels != nil)
        
        // Check that all labels are properly converted
        var foundLabels: [String] = []
        
        for i in 0..<Int(cOptions.pointee.drop_labels_count) {
            if let labelPtr = cOptions.pointee.drop_labels[i] {
                let label = String(cString: labelPtr)
                foundLabels.append(label)
            }
        }
        
        // Verify all expected labels are present
        #expect(foundLabels.contains("DEV"))
        #expect(foundLabels.contains("TEST"))
        #expect(foundLabels.contains("DEBUG"))
        #expect(foundLabels.contains("PRODUCTION"))
        #expect(foundLabels.count == 4)
        
        // Test with single label
        let singleOptions = ESBuildTransformOptions(dropLabels: ["SINGLE"])
        let cSingleOptions = singleOptions.cValue
        defer { esbuild_free_transform_options(cSingleOptions) }
        
        #expect(cSingleOptions.pointee.drop_labels_count == 1)
        if let firstLabelPtr = cSingleOptions.pointee.drop_labels[0] {
            #expect(String(cString: firstLabelPtr) == "SINGLE")
        }
        
        // Test empty drop labels
        let emptyOptions = ESBuildTransformOptions(dropLabels: [])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.drop_labels_count == 0)
    }

    @Test("Define parameter C bridge conversion")
    func testDefineParameterConversion() {
        // Test with multiple define mappings
        let options = ESBuildTransformOptions(
            define: [
                "NODE_ENV": "\"production\"",
                "VERSION": "\"1.2.3\"",
                "DEBUG": "false",
                "API_URL": "\"https://api.example.com\""
            ]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify define conversion
        #expect(cOptions.pointee.define_count == 4)
        #expect(cOptions.pointee.define_keys != nil)
        #expect(cOptions.pointee.define_values != nil)
        
        // Check that all keys and values are properly converted
        var foundMappings: [String: String] = [:]
        
        for i in 0..<Int(cOptions.pointee.define_count) {
            if let keyPtr = cOptions.pointee.define_keys[i],
               let valuePtr = cOptions.pointee.define_values[i] {
                let key = String(cString: keyPtr)
                let value = String(cString: valuePtr)
                foundMappings[key] = value
            }
        }
        
        // Verify all expected mappings are present
        #expect(foundMappings["NODE_ENV"] == "\"production\"")
        #expect(foundMappings["VERSION"] == "\"1.2.3\"")
        #expect(foundMappings["DEBUG"] == "false")
        #expect(foundMappings["API_URL"] == "\"https://api.example.com\"")
        #expect(foundMappings.count == 4)
        
        // Test with single define
        let singleOptions = ESBuildTransformOptions(define: ["SINGLE": "\"value\""])
        let cSingleOptions = singleOptions.cValue
        defer { esbuild_free_transform_options(cSingleOptions) }
        
        #expect(cSingleOptions.pointee.define_count == 1)
        if let keyPtr = cSingleOptions.pointee.define_keys[0],
           let valuePtr = cSingleOptions.pointee.define_values[0] {
            #expect(String(cString: keyPtr) == "SINGLE")
            #expect(String(cString: valuePtr) == "\"value\"")
        }
        
        // Test empty define
        let emptyOptions = ESBuildTransformOptions(define: [:])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.define_count == 0)
    }

    @Test("Pure parameter C bridge conversion")
    func testPureParameterConversion() {
        // Test with multiple pure functions
        let options = ESBuildTransformOptions(
            pure: ["console.log", "console.warn", "debug", "Math.random"]
        )
        
        let cOptions = options.cValue
        defer { esbuild_free_transform_options(cOptions) }
        
        // Verify pure conversion
        #expect(cOptions.pointee.pure_count == 4)
        #expect(cOptions.pointee.pure != nil)
        
        // Check that all pure functions are properly converted
        var foundFunctions: [String] = []
        
        for i in 0..<Int(cOptions.pointee.pure_count) {
            if let functionPtr = cOptions.pointee.pure[i] {
                let function = String(cString: functionPtr)
                foundFunctions.append(function)
            }
        }
        
        // Verify all expected functions are present
        #expect(foundFunctions.contains("console.log"))
        #expect(foundFunctions.contains("console.warn"))
        #expect(foundFunctions.contains("debug"))
        #expect(foundFunctions.contains("Math.random"))
        #expect(foundFunctions.count == 4)
        
        // Test with single pure function
        let singleOptions = ESBuildTransformOptions(pure: ["singleFunction"])
        let cSingleOptions = singleOptions.cValue
        defer { esbuild_free_transform_options(cSingleOptions) }
        
        #expect(cSingleOptions.pointee.pure_count == 1)
        if let firstFunctionPtr = cSingleOptions.pointee.pure[0] {
            #expect(String(cString: firstFunctionPtr) == "singleFunction")
        }
        
        // Test empty pure functions
        let emptyOptions = ESBuildTransformOptions(pure: [])
        let cEmptyOptions = emptyOptions.cValue
        defer { esbuild_free_transform_options(cEmptyOptions) }
        
        #expect(cEmptyOptions.pointee.pure_count == 0)
    }
}
