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
            color: .always,
            logLevel: .debug,
            logLimit: 100,
            logOverride: ["ts": .warning],
            sourcemap: .inline,
            sourceRoot: "/src",
            sourcesContent: .exclude,
            target: .es2020,
            engines: [(.chrome, "90"), (.firefox, "88")],
            supported: ["bigint": true, "import-meta": false],
            platform: .browser,
            format: .esmodule,
            globalName: "MyLibrary",
            mangleProps: "^_",
            reserveProps: "^keep_",
            mangleQuoted: .true,
            mangleCache: ["oldName": "newName"],
            drop: [.console, .debugger],
            dropLabels: ["DEV", "TEST"],
            minifyWhitespace: true,
            minifyIdentifiers: true,
            minifySyntax: true,
            lineLimit: 80,
            charset: .ascii,
            treeShaking: .true,
            ignoreAnnotations: true,
            legalComments: .inline,
            jsx: .automatic,
            jsxFactory: "React.createElement",
            jsxFragment: "React.Fragment",
            jsxImportSource: "react",
            jsxDev: true,
            jsxSideEffects: true,
            tsconfigRaw: "{\"compilerOptions\":{\"strict\":true}}",
            banner: "/* Banner */",
            footer: "/* Footer */",
            define: ["NODE_ENV": "production"],
            pure: ["console.log", "Math.random"],
            keepNames: true,
            sourcefile: "input.ts",
            loader: .tsx
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
}
