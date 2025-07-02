import TSCBridge
import Testing

@testable import SwiftTSGo

@Suite("ESBuild Types Tests")
struct ESBuildTypesTests {

    @Test("Platform enum values match esbuild Go constants via C bridge")
    func testPlatformRawValues() {
        #expect(ESBuildPlatform.default.actualRawValue == esbuild_platform_default())
        #expect(ESBuildPlatform.browser.actualRawValue == esbuild_platform_browser())
        #expect(ESBuildPlatform.node.actualRawValue == esbuild_platform_node())
        #expect(ESBuildPlatform.neutral.actualRawValue == esbuild_platform_neutral())
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
        for i in 0..<count {
            let value = cArray.values.advanced(by: i).pointee
            cPlatformValues.append(value)
        }

        #expect(cPlatformValues.count == ESBuildPlatform.allCases.count)
        for value in cPlatformValues.sorted() {
            #expect(value == ESBuildPlatform(rawValue: value)?.rawValue)
        }
    }
}
