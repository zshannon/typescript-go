import Testing
import TSCBridge

@testable import SwiftTSGo

@Suite("ESBuild Types Tests")
struct ESBuildTypesTests {
    @Test("Platform enum values match esbuild Go constants via C bridge")
    func testPlatformRawValues() {
        #expect(ESBuildPlatform.default.cValue == esbuild_platform_default())
        #expect(ESBuildPlatform.browser.cValue == esbuild_platform_browser())
        #expect(ESBuildPlatform.node.cValue == esbuild_platform_node())
        #expect(ESBuildPlatform.neutral.cValue == esbuild_platform_neutral())
    }

    @Test("Platform initialization from C bridge values")
    func testPlatformInitialization() {
        #expect(ESBuildPlatform.from(cValue: esbuild_platform_default()) == .default)
        #expect(ESBuildPlatform.from(cValue: esbuild_platform_browser()) == .browser)
        #expect(ESBuildPlatform.from(cValue: esbuild_platform_node()) == .node)
        #expect(ESBuildPlatform.from(cValue: esbuild_platform_neutral()) == .neutral)
    }

    @Test("Invalid C bridge values return nil")
    func testInvalidPlatformValues() {
        #expect(ESBuildPlatform.from(cValue: -1) == nil)
        #expect(ESBuildPlatform.from(cValue: 999) == nil)
    }

    @Test("All C bridge platform values are implemented in Swift enum")
    func testAllCPlatformValuesImplemented() {
        // Get all platform values from C bridge
        let cArrayPtr = esbuild_get_all_platform_values()
        defer { esbuild_free_int_array(cArrayPtr) }

        guard let cArrayPtr else {
            Issue.record("Failed to get platform values from C bridge")
            return
        }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)

        // Convert C array to Swift array
        var cPlatformValues: [Int32] = []
        for i in 0 ..< count {
            let value = cArray.values.advanced(by: i).pointee
            cPlatformValues.append(value)
        }

        #expect(cPlatformValues.count == ESBuildPlatform.allCases.count)
        for value in cPlatformValues.sorted() {
            #expect(value == ESBuildPlatform(rawValue: value)?.rawValue)
        }
    }

    // MARK: - Format Enum Tests

    @Test("Format enum values match esbuild Go constants via C bridge")
    func testFormatRawValues() {
        #expect(ESBuildFormat.default.cValue == esbuild_format_default())
        #expect(ESBuildFormat.iife.cValue == esbuild_format_iife())
        #expect(ESBuildFormat.commonjs.cValue == esbuild_format_commonjs())
        #expect(ESBuildFormat.esmodule.cValue == esbuild_format_esmodule())
    }

    @Test("Format initialization from C bridge values")
    func testFormatInitialization() {
        #expect(ESBuildFormat.from(cValue: esbuild_format_default()) == .default)
        #expect(ESBuildFormat.from(cValue: esbuild_format_iife()) == .iife)
        #expect(ESBuildFormat.from(cValue: esbuild_format_commonjs()) == .commonjs)
        #expect(ESBuildFormat.from(cValue: esbuild_format_esmodule()) == .esmodule)
    }

    @Test("All C bridge format values are implemented in Swift enum")
    func testAllCFormatValuesImplemented() {
        let cArrayPtr = esbuild_get_all_format_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildFormat.allCases.count)
        for value in cValues {
            #expect(ESBuildFormat.from(cValue: value) != nil)
        }
    }

    // MARK: - Target Enum Tests

    @Test("Target enum values match esbuild Go constants via C bridge")
    func testTargetRawValues() {
        #expect(ESBuildTarget.default.cValue == esbuild_target_default())
        #expect(ESBuildTarget.esnext.cValue == esbuild_target_esnext())
        #expect(ESBuildTarget.es5.cValue == esbuild_target_es5())
        #expect(ESBuildTarget.es2015.cValue == esbuild_target_es2015())
        #expect(ESBuildTarget.es2024.cValue == esbuild_target_es2024())
    }

    @Test("All C bridge target values are implemented in Swift enum")
    func testAllCTargetValuesImplemented() {
        let cArrayPtr = esbuild_get_all_target_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildTarget.allCases.count)
        for value in cValues {
            #expect(ESBuildTarget.from(cValue: value) != nil)
        }
    }

    // MARK: - Loader Enum Tests

    @Test("Loader enum values match esbuild Go constants via C bridge")
    func testLoaderRawValues() {
        #expect(ESBuildLoader.none.cValue == esbuild_loader_none())
        #expect(ESBuildLoader.js.cValue == esbuild_loader_js())
        #expect(ESBuildLoader.ts.cValue == esbuild_loader_ts())
        #expect(ESBuildLoader.jsx.cValue == esbuild_loader_jsx())
        #expect(ESBuildLoader.tsx.cValue == esbuild_loader_tsx())
    }

    @Test("All C bridge loader values are implemented in Swift enum")
    func testAllCLoaderValuesImplemented() {
        let cArrayPtr = esbuild_get_all_loader_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildLoader.allCases.count)
        for value in cValues {
            #expect(ESBuildLoader.from(cValue: value) != nil)
        }
    }

    // MARK: - SourceMap Enum Tests

    @Test("SourceMap enum values match esbuild Go constants via C bridge")
    func testSourceMapRawValues() {
        #expect(ESBuildSourceMap.none.cValue == esbuild_sourcemap_none())
        #expect(ESBuildSourceMap.inline.cValue == esbuild_sourcemap_inline())
        #expect(ESBuildSourceMap.external.cValue == esbuild_sourcemap_external())
    }

    @Test("All C bridge sourcemap values are implemented in Swift enum")
    func testAllCSourceMapValuesImplemented() {
        let cArrayPtr = esbuild_get_all_sourcemap_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildSourceMap.allCases.count)
        for value in cValues {
            #expect(ESBuildSourceMap.from(cValue: value) != nil)
        }
    }

    // MARK: - JSX Enum Tests

    @Test("JSX enum values match esbuild Go constants via C bridge")
    func testJSXRawValues() {
        #expect(ESBuildJSX.transform.cValue == esbuild_jsx_transform())
        #expect(ESBuildJSX.preserve.cValue == esbuild_jsx_preserve())
        #expect(ESBuildJSX.automatic.cValue == esbuild_jsx_automatic())
    }

    @Test("All C bridge JSX values are implemented in Swift enum")
    func testAllCJSXValuesImplemented() {
        let cArrayPtr = esbuild_get_all_jsx_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildJSX.allCases.count)
        for value in cValues {
            #expect(ESBuildJSX.from(cValue: value) != nil)
        }
    }

    // MARK: - Comprehensive Test for All Remaining Enums

    @Test("All remaining enum types have complete C bridge integration")
    func testAllRemainingEnums() {
        // Test LogLevel
        let logLevelPtr = esbuild_get_all_loglevel_values()
        defer { esbuild_free_int_array(logLevelPtr) }
        if let logLevelPtr {
            let count = Int(logLevelPtr.pointee.count)
            #expect(count == ESBuildLogLevel.allCases.count)
        }

        // Test LegalComments
        let legalPtr = esbuild_get_all_legalcomments_values()
        defer { esbuild_free_int_array(legalPtr) }
        if let legalPtr {
            let count = Int(legalPtr.pointee.count)
            #expect(count == ESBuildLegalComments.allCases.count)
        }

        // Test Charset
        let charsetPtr = esbuild_get_all_charset_values()
        defer { esbuild_free_int_array(charsetPtr) }
        if let charsetPtr {
            let count = Int(charsetPtr.pointee.count)
            #expect(count == ESBuildCharset.allCases.count)
        }

        // Test TreeShaking
        let treePtr = esbuild_get_all_treeshaking_values()
        defer { esbuild_free_int_array(treePtr) }
        if let treePtr {
            let count = Int(treePtr.pointee.count)
            #expect(count == ESBuildTreeShaking.allCases.count)
        }

        // Test Color
        let colorPtr = esbuild_get_all_color_values()
        defer { esbuild_free_int_array(colorPtr) }
        if let colorPtr {
            let count = Int(colorPtr.pointee.count)
            #expect(count == ESBuildColor.allCases.count)
        }

        // Test Packages
        let packagesPtr = esbuild_get_all_packages_values()
        defer { esbuild_free_int_array(packagesPtr) }
        if let packagesPtr {
            let count = Int(packagesPtr.pointee.count)
            #expect(count == ESBuildPackages.allCases.count)
        }

        // Test SourcesContent
        let sourcesPtr = esbuild_get_all_sourcescontent_values()
        defer { esbuild_free_int_array(sourcesPtr) }
        if let sourcesPtr {
            let count = Int(sourcesPtr.pointee.count)
            #expect(count == ESBuildSourcesContent.allCases.count)
        }

        // Test MangleQuoted
        let manglePtr = esbuild_get_all_manglequoted_values()
        defer { esbuild_free_int_array(manglePtr) }
        if let manglePtr {
            let count = Int(manglePtr.pointee.count)
            #expect(count == ESBuildMangleQuoted.allCases.count)
        }

        // Test Drop
        let dropPtr = esbuild_get_all_drop_values()
        defer { esbuild_free_int_array(dropPtr) }
        if let dropPtr {
            let count = Int(dropPtr.pointee.count)
            #expect(count == ESBuildDrop.allCases.count)
        }

        // Test Engine
        let enginePtr = esbuild_get_all_engine_values()
        defer { esbuild_free_int_array(enginePtr) }
        if let enginePtr {
            let count = Int(enginePtr.pointee.count)
            #expect(count == ESBuildEngine.allCases.count)
        }

        // Test SideEffects
        let sideEffectsPtr = esbuild_get_all_sideeffects_values()
        defer { esbuild_free_int_array(sideEffectsPtr) }
        if let sideEffectsPtr {
            let count = Int(sideEffectsPtr.pointee.count)
            #expect(count == ESBuildSideEffects.allCases.count)
        }

        // Test ResolveKind
        let resolveKindPtr = esbuild_get_all_resolvekind_values()
        defer { esbuild_free_int_array(resolveKindPtr) }
        if let resolveKindPtr {
            let count = Int(resolveKindPtr.pointee.count)
            #expect(count == ESBuildResolveKind.allCases.count)
        }

        // Test MessageKind
        let messageKindPtr = esbuild_get_all_messagekind_values()
        defer { esbuild_free_int_array(messageKindPtr) }
        if let messageKindPtr {
            let count = Int(messageKindPtr.pointee.count)
            #expect(count == ESBuildMessageKind.allCases.count)
        }
    }

    // MARK: - SideEffects Enum Tests

    @Test("SideEffects enum values match esbuild Go constants via C bridge")
    func testSideEffectsRawValues() {
        #expect(ESBuildSideEffects.true.cValue == esbuild_sideeffects_true())
        #expect(ESBuildSideEffects.false.cValue == esbuild_sideeffects_false())
    }

    @Test("All C bridge sideeffects values are implemented in Swift enum")
    func testAllCSideEffectsValuesImplemented() {
        let cArrayPtr = esbuild_get_all_sideeffects_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildSideEffects.allCases.count)
        for value in cValues {
            #expect(ESBuildSideEffects.from(cValue: value) != nil)
        }
    }

    // MARK: - ResolveKind Enum Tests

    @Test("ResolveKind enum values match esbuild Go constants via C bridge")
    func testResolveKindRawValues() {
        #expect(ESBuildResolveKind.none.cValue == esbuild_resolvekind_none())
        #expect(ESBuildResolveKind.entryPoint.cValue == esbuild_resolvekind_entrypoint())
        #expect(ESBuildResolveKind.jsImportStatement.cValue == esbuild_resolvekind_jsimportstatement())
        #expect(ESBuildResolveKind.cssURLToken.cValue == esbuild_resolvekind_cssurltoken())
    }

    @Test("All C bridge resolvekind values are implemented in Swift enum")
    func testAllCResolveKindValuesImplemented() {
        let cArrayPtr = esbuild_get_all_resolvekind_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildResolveKind.allCases.count)
        for value in cValues {
            #expect(ESBuildResolveKind.from(cValue: value) != nil)
        }
    }

    // MARK: - MessageKind Enum Tests

    @Test("MessageKind enum values match esbuild Go constants via C bridge")
    func testMessageKindRawValues() {
        #expect(ESBuildMessageKind.error.cValue == esbuild_messagekind_error())
        #expect(ESBuildMessageKind.warning.cValue == esbuild_messagekind_warning())
    }

    @Test("All C bridge messagekind values are implemented in Swift enum")
    func testAllCMessageKindValuesImplemented() {
        let cArrayPtr = esbuild_get_all_messagekind_values()
        defer { esbuild_free_int_array(cArrayPtr) }
        guard let cArrayPtr else { return }

        let cArray = cArrayPtr.pointee
        let count = Int(cArray.count)
        var cValues: [Int32] = []
        for i in 0 ..< count {
            cValues.append(cArray.values.advanced(by: i).pointee)
        }

        #expect(cValues.count == ESBuildMessageKind.allCases.count)
        for value in cValues {
            #expect(ESBuildMessageKind.from(cValue: value) != nil)
        }
    }
}
