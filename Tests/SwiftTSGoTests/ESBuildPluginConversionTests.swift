import Testing
@testable import SwiftTSGo
import TSCBridge

@Suite("ESBuild Plugin Conversion Tests")
struct ESBuildPluginConversionTests {
    
    // MARK: - Plugin Data Serialization Tests
    
    @Test("Plugin data serialization with dictionary")
    func testPluginDataSerializationDictionary() {
        let data: [String: Any] = ["key": "value", "number": 42, "bool": true]
        let json = ESBuildPlugin.serializePluginData(data)
        
        #expect(json != nil)
        #expect(json!.contains("\"key\":\"value\""))
        #expect(json!.contains("\"number\":42"))
        #expect(json!.contains("\"bool\":true"))
    }
    
    @Test("Plugin data serialization with array")
    func testPluginDataSerializationArray() {
        let data = ["item1", "item2", "item3"]
        let json = ESBuildPlugin.serializePluginData(data)
        
        #expect(json != nil)
        #expect(json == "[\"item1\",\"item2\",\"item3\"]")
    }
    
    @Test("Plugin data serialization with nil")
    func testPluginDataSerializationNil() {
        let json = ESBuildPlugin.serializePluginData(nil)
        #expect(json == nil)
    }
    
    @Test("Plugin data deserialization with dictionary")
    func testPluginDataDeserializationDictionary() {
        let json = "{\"key\":\"value\",\"number\":42}"
        let data = ESBuildPlugin.deserializePluginData(json)
        
        #expect(data != nil)
        if let dict = data as? [String: Any] {
            #expect(dict["key"] as? String == "value")
            #expect(dict["number"] as? Int == 42)
        } else {
            Issue.record("Failed to deserialize as dictionary")
        }
    }
    
    @Test("Plugin data deserialization with array")
    func testPluginDataDeserializationArray() {
        let json = "[\"item1\",\"item2\",\"item3\"]"
        let data = ESBuildPlugin.deserializePluginData(json)
        
        #expect(data != nil)
        if let array = data as? [String] {
            #expect(array == ["item1", "item2", "item3"])
        } else {
            Issue.record("Failed to deserialize as array")
        }
    }
    
    // MARK: - Location Conversion Tests
    
    @Test("Location converts to C representation")
    func testLocationToCConversion() {
        let location = ESBuildPluginLocation(
            column: 5,
            file: "/src/test.js",
            length: 7,
            line: 10,
            lineText: "const x = 42;",
            namespace: "file"
        )
        
        let cValue = location.cValue
        defer { 
            free(cValue.pointee.file)
            free(cValue.pointee.namespace)
            free(cValue.pointee.line_text)
            cValue.deallocate()
        }
        
        #expect(String(cString: cValue.pointee.file) == "/src/test.js")
        #expect(String(cString: cValue.pointee.namespace) == "file")
        #expect(cValue.pointee.line == 10)
        #expect(cValue.pointee.column == 5)
        #expect(cValue.pointee.length == 7)
        #expect(String(cString: cValue.pointee.line_text) == "const x = 42;")
    }
    
    @Test("Location creates from C representation")
    func testLocationFromCConversion() {
        var cLocation = c_location()
        cLocation.file = strdup("/src/test.js")
        cLocation.namespace = strdup("file")
        cLocation.line = 10
        cLocation.column = 5
        cLocation.length = 7
        cLocation.line_text = strdup("const x = 42;")
        cLocation.suggestion = nil
        
        defer {
            free(cLocation.file)
            free(cLocation.namespace)
            free(cLocation.line_text)
        }
        
        let location = ESBuildPluginLocation(cValue: &cLocation)
        #expect(location != nil)
        #expect(location?.file == "/src/test.js")
        #expect(location?.namespace == "file")
        #expect(location?.line == 10)
        #expect(location?.column == 5)
        #expect(location?.length == 7)
        #expect(location?.lineText == "const x = 42;")
    }
    
    // MARK: - Message Conversion Tests
    
    @Test("Message converts to C representation")
    func testMessageToCConversion() {
        let location = ESBuildPluginLocation(
            column: 0,
            file: "test.js",
            length: 5,
            line: 1,
            lineText: "error",
            namespace: "file"
        )
        let message = ESBuildPluginMessage(
            location: location,
            text: "Test error"
        )
        
        let cValue = message.cValue
        defer {
            free(cValue.pointee.text)
            if cValue.pointee.location != nil {
                free(cValue.pointee.location.pointee.file)
                free(cValue.pointee.location.pointee.namespace)
                free(cValue.pointee.location.pointee.line_text)
                cValue.pointee.location.deallocate()
            }
            cValue.deallocate()
        }
        
        #expect(String(cString: cValue.pointee.text) == "Test error")
        #expect(cValue.pointee.location != nil)
    }
    
    @Test("Message creates from C representation")
    func testMessageFromCConversion() {
        var cMessage = c_message()
        cMessage.text = strdup("Test error")
        cMessage.id = nil
        cMessage.plugin_name = nil
        cMessage.notes = nil
        cMessage.notes_count = 0
        
        // Create location
        let cLocation = UnsafeMutablePointer<c_location>.allocate(capacity: 1)
        cLocation.pointee.file = strdup("test.js")
        cLocation.pointee.namespace = strdup("file")
        cLocation.pointee.line = 1
        cLocation.pointee.column = 0
        cLocation.pointee.length = 5
        cLocation.pointee.line_text = strdup("error")
        cLocation.pointee.suggestion = nil
        cMessage.location = cLocation
        
        defer {
            free(cMessage.text)
            free(cLocation.pointee.file)
            free(cLocation.pointee.namespace)
            free(cLocation.pointee.line_text)
            cLocation.deallocate()
        }
        
        let message = ESBuildPluginMessage(cValue: &cMessage)
        #expect(message != nil)
        #expect(message?.text == "Test error")
        #expect(message?.location != nil)
        #expect(message?.location?.file == "test.js")
    }
    
    // MARK: - OnResolveArgs Conversion Tests
    
    @Test("OnResolveArgs creates from C representation")
    func testOnResolveArgsFromCConversion() {
        var cArgs = c_on_resolve_args()
        cArgs.path = strdup("module")
        cArgs.importer = strdup("/src/index.js")
        cArgs.namespace = strdup("file")
        cArgs.resolve_dir = strdup("/src")
        cArgs.kind = 1 // importStatement
        cArgs.plugin_data = strdup("{\"key\":\"value\"}")
        
        // Create with map
        cArgs.with_count = 1
        cArgs.with_keys = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: 1)
        cArgs.with_values = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>.allocate(capacity: 1)
        cArgs.with_keys[0] = strdup("type")
        cArgs.with_values[0] = strdup("json")
        
        defer {
            free(cArgs.path)
            free(cArgs.importer)
            free(cArgs.namespace)
            free(cArgs.resolve_dir)
            free(cArgs.plugin_data)
            free(cArgs.with_keys[0])
            free(cArgs.with_values[0])
            cArgs.with_keys.deallocate()
            cArgs.with_values.deallocate()
        }
        
        let args = ESBuildOnResolveArgs(cValue: &cArgs)
        #expect(args.path == "module")
        #expect(args.importer == "/src/index.js")
        #expect(args.namespace == "file")
        #expect(args.resolveDir == "/src")
        #expect(args.kind == .importStatement)
        #expect(args.with["type"] == "json")
        
        if let pluginData = args.pluginData as? [String: String] {
            #expect(pluginData["key"] == "value")
        }
    }
    
    // MARK: - OnLoadArgs Conversion Tests
    
    @Test("OnLoadArgs creates from C representation")
    func testOnLoadArgsFromCConversion() {
        var cArgs = c_on_load_args()
        cArgs.path = strdup("/src/module.js")
        cArgs.namespace = strdup("file")
        cArgs.suffix = strdup("?v=1.0")
        cArgs.plugin_data = strdup("{\"transformed\":true}")
        cArgs.with_count = 0
        cArgs.with_keys = nil
        cArgs.with_values = nil
        
        defer {
            free(cArgs.path)
            free(cArgs.namespace)
            free(cArgs.suffix)
            free(cArgs.plugin_data)
        }
        
        let args = ESBuildOnLoadArgs(cValue: &cArgs)
        #expect(args.path == "/src/module.js")
        #expect(args.namespace == "file")
        #expect(args.suffix == "?v=1.0")
        #expect(args.with.isEmpty)
        
        if let pluginData = args.pluginData as? [String: Bool] {
            #expect(pluginData["transformed"] == true)
        }
    }
    
    // MARK: - OnResolveResult Conversion Tests
    
    @Test("OnResolveResult converts to C representation")
    func testOnResolveResultToCConversion() {
        let errors = [ESBuildPluginMessage(text: "Error 1")]
        let warnings = [ESBuildPluginMessage(text: "Warning 1")]
        
        let result = ESBuildOnResolveResult(
            errors: errors,
            external: true,
            namespace: "custom",
            path: "/resolved/path.js",
            pluginData: ["resolved": true],
            pluginName: "test-plugin",
            sideEffects: false,
            suffix: "?v=1.0",
            warnings: warnings,
            watchDirs: ["/watch/dir"],
            watchFiles: ["/watch/file.js"]
        )
        
        let cValue = result.cValue
        defer { freePluginCStructures(cValue) }
        
        #expect(cValue.pointee.path != nil)
        #expect(String(cString: cValue.pointee.path) == "/resolved/path.js")
        #expect(cValue.pointee.namespace != nil)
        #expect(String(cString: cValue.pointee.namespace) == "custom")
        #expect(cValue.pointee.external == 1)
        #expect(cValue.pointee.side_effects == 0)
        #expect(cValue.pointee.suffix != nil)
        #expect(String(cString: cValue.pointee.suffix) == "?v=1.0")
        #expect(cValue.pointee.errors_count == 1)
        #expect(cValue.pointee.warnings_count == 1)
        #expect(cValue.pointee.watch_files_count == 1)
        #expect(cValue.pointee.watch_dirs_count == 1)
    }
    
    // MARK: - OnLoadResult Conversion Tests
    
    @Test("OnLoadResult converts to C representation with string contents")
    func testOnLoadResultToCConversionString() {
        let errors = [ESBuildPluginMessage(text: "Error 1")]
        let warnings = [ESBuildPluginMessage(text: "Warning 1")]
        
        let result = ESBuildOnLoadResult(
            contents: "console.log('test')",
            errors: errors,
            loader: .js,
            pluginData: ["loaded": true],
            pluginName: "test-plugin",
            resolveDir: "/src",
            warnings: warnings,
            watchDirs: ["/src"],
            watchFiles: ["/config.json"]
        )
        
        let cValue = result.cValue
        defer { freePluginCStructures(cValue) }
        
        #expect(cValue.pointee.contents != nil)
        #expect(cValue.pointee.contents_length == 19) // "console.log('test')".count
        #expect(cValue.pointee.loader == ESBuildLoader.js.cValue)
        #expect(cValue.pointee.resolve_dir != nil)
        #expect(String(cString: cValue.pointee.resolve_dir) == "/src")
        #expect(cValue.pointee.errors_count == 1)
        #expect(cValue.pointee.warnings_count == 1)
        #expect(cValue.pointee.watch_files_count == 1)
        #expect(cValue.pointee.watch_dirs_count == 1)
    }
    
    @Test("OnLoadResult converts to C representation with nil contents")
    func testOnLoadResultToCConversionNilContents() {
        let result = ESBuildOnLoadResult()
        
        let cValue = result.cValue
        defer { freePluginCStructures(cValue) }
        
        #expect(cValue.pointee.contents == nil)
        #expect(cValue.pointee.contents_length == 0)
        #expect(cValue.pointee.loader == -1)
        #expect(cValue.pointee.resolve_dir == nil)
        #expect(cValue.pointee.errors_count == 0)
        #expect(cValue.pointee.warnings_count == 0)
        #expect(cValue.pointee.watch_files_count == 0)
        #expect(cValue.pointee.watch_dirs_count == 0)
    }
}