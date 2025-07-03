import Foundation
import TSCBridge

// MARK: - Plugin Data Serialization

extension ESBuildPlugin {
    /// Serialize pluginData to JSON string for C bridge
    static func serializePluginData(_ data: Any?) -> String? {
        guard let data else { return nil }

        do {
            let jsonData = try JSONSerialization.data(withJSONObject: data, options: [])
            return String(data: jsonData, encoding: .utf8)
        } catch {
            print("Warning: Failed to serialize plugin data: \(error)")
            return nil
        }
    }

    /// Deserialize pluginData from JSON string
    static func deserializePluginData(_ json: String?) -> (any Sendable)? {
        guard let json, let data = json.data(using: .utf8) else { return nil }

        do {
            return try JSONSerialization.jsonObject(with: data, options: []) as! (any Sendable)
        } catch {
            print("Warning: Failed to deserialize plugin data: \(error)")
            return nil
        }
    }
}

// MARK: - Location Conversion

extension ESBuildPluginLocation {
    /// Convert to C representation
    var cValue: UnsafeMutablePointer<c_location> {
        let loc = UnsafeMutablePointer<c_location>.allocate(capacity: 1)
        loc.pointee.file = strdup(file)
        loc.pointee.namespace = strdup(namespace)
        loc.pointee.line = Int32(line)
        loc.pointee.column = Int32(column)
        loc.pointee.length = Int32(length)
        loc.pointee.line_text = strdup(lineText)
        loc.pointee.suggestion = nil
        return loc
    }

    /// Create from C representation
    init?(cValue: UnsafePointer<c_location>?) {
        guard let cValue else { return nil }

        self.init(
            column: Int(cValue.pointee.column),
            file: String(cString: cValue.pointee.file),
            length: Int(cValue.pointee.length),
            line: Int(cValue.pointee.line),
            lineText: String(cString: cValue.pointee.line_text),
            namespace: String(cString: cValue.pointee.namespace)
        )
    }
}

// MARK: - Message Conversion

extension ESBuildPluginMessage {
    /// Convert to C representation
    var cValue: UnsafeMutablePointer<c_message> {
        let msg = UnsafeMutablePointer<c_message>.allocate(capacity: 1)
        msg.pointee.id = nil
        msg.pointee.plugin_name = nil
        msg.pointee.text = strdup(text)
        msg.pointee.location = location?.cValue
        msg.pointee.notes = nil
        msg.pointee.notes_count = 0
        return msg
    }

    /// Create from C representation
    init?(cValue: UnsafePointer<c_message>?) {
        guard let cValue else { return nil }

        self.init(
            detail: nil, // C bridge doesn't support detail field
            location: ESBuildPluginLocation(cValue: cValue.pointee.location),
            text: String(cString: cValue.pointee.text)
        )
    }
}

// MARK: - OnResolveArgs Conversion

extension ESBuildOnResolveArgs {
    /// Create from C representation
    init(cValue: UnsafePointer<c_on_resolve_args>) {
        var withDict: [String: String] = [:]
        if cValue.pointee.with_count > 0 {
            for i in 0 ..< Int(cValue.pointee.with_count) {
                let key = String(cString: cValue.pointee.with_keys[i]!)
                let value = String(cString: cValue.pointee.with_values[i]!)
                withDict[key] = value
            }
        }

        self.init(
            importer: String(cString: cValue.pointee.importer),
            kind: ESBuildPluginResolveKind(cValue: cValue.pointee.kind) ?? .importStatement,
            namespace: String(cString: cValue.pointee.namespace),
            path: String(cString: cValue.pointee.path),
            pluginData: ESBuildPlugin.deserializePluginData(
                cValue.pointee.plugin_data != nil ? String(cString: cValue.pointee.plugin_data) : nil
            ),
            resolveDir: String(cString: cValue.pointee.resolve_dir),
            with: withDict
        )
    }
}

// MARK: - OnLoadArgs Conversion

extension ESBuildOnLoadArgs {
    /// Create from C representation
    init(cValue: UnsafePointer<c_on_load_args>) {
        var withDict: [String: String] = [:]
        if cValue.pointee.with_count > 0 {
            for i in 0 ..< Int(cValue.pointee.with_count) {
                let key = String(cString: cValue.pointee.with_keys[i]!)
                let value = String(cString: cValue.pointee.with_values[i]!)
                withDict[key] = value
            }
        }

        self.init(
            namespace: String(cString: cValue.pointee.namespace),
            path: String(cString: cValue.pointee.path),
            pluginData: ESBuildPlugin.deserializePluginData(
                cValue.pointee.plugin_data != nil ? String(cString: cValue.pointee.plugin_data) : nil
            ),
            suffix: cValue.pointee.suffix != nil ? String(cString: cValue.pointee.suffix) : "",
            with: withDict
        )
    }
}

// MARK: - OnResolveResult Conversion

extension ESBuildOnResolveResult {
    /// Convert to C representation
    var cValue: UnsafeMutablePointer<c_on_resolve_result> {
        let result = UnsafeMutablePointer<c_on_resolve_result>.allocate(capacity: 1)

        result.pointee.path = path != nil ? strdup(path!) : nil
        result.pointee.namespace = namespace != nil ? strdup(namespace!) : nil
        result.pointee.external = external != nil ? (external! ? 1 : 0) : -1
        result.pointee.side_effects = sideEffects != nil ? (sideEffects! ? 1 : 0) : -1
        result.pointee.suffix = suffix != nil ? strdup(suffix!) : nil
        result.pointee.plugin_data = ESBuildPlugin.serializePluginData(pluginData).map { strdup($0) }
        result.pointee.plugin_name = pluginName != nil ? strdup(pluginName!) : nil

        // Convert errors
        result.pointee.errors_count = Int32(errors.count)
        if !errors.isEmpty {
            result.pointee.errors = UnsafeMutablePointer<c_message>.allocate(capacity: errors.count)
            for (i, error) in errors.enumerated() {
                result.pointee.errors[i] = error.cValue.pointee
            }
        } else {
            result.pointee.errors = nil
        }

        // Convert warnings
        result.pointee.warnings_count = Int32(warnings.count)
        if !warnings.isEmpty {
            result.pointee.warnings = UnsafeMutablePointer<c_message>.allocate(capacity: warnings.count)
            for (i, warning) in warnings.enumerated() {
                result.pointee.warnings[i] = warning.cValue.pointee
            }
        } else {
            result.pointee.warnings = nil
        }

        // Convert watch files
        result.pointee.watch_files_count = Int32(watchFiles.count)
        if !watchFiles.isEmpty {
            result.pointee.watch_files = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: watchFiles.count)
            for (i, file) in watchFiles.enumerated() {
                result.pointee.watch_files[i] = strdup(file)
            }
        } else {
            result.pointee.watch_files = nil
        }

        // Convert watch dirs
        result.pointee.watch_dirs_count = Int32(watchDirs.count)
        if !watchDirs.isEmpty {
            result.pointee.watch_dirs = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: watchDirs.count)
            for (i, dir) in watchDirs.enumerated() {
                result.pointee.watch_dirs[i] = strdup(dir)
            }
        } else {
            result.pointee.watch_dirs = nil
        }

        return result
    }
}

// MARK: - OnLoadResult Conversion

extension ESBuildOnLoadResult {
    /// Convert to C representation
    var cValue: UnsafeMutablePointer<c_on_load_result> {
        let result = UnsafeMutablePointer<c_on_load_result>.allocate(capacity: 1)

        // Convert contents
        if let contents {
            result.pointee.contents_length = Int32(contents.count)
            result.pointee.contents = UnsafeMutablePointer<CChar>.allocate(capacity: contents.count)
            _ = contents.withUnsafeBytes { bytes in
                memcpy(result.pointee.contents, bytes.baseAddress, contents.count)
            }
        } else {
            result.pointee.contents = nil
            result.pointee.contents_length = 0
        }

        result.pointee.loader = loader?.cValue ?? -1
        result.pointee.resolve_dir = resolveDir != nil ? strdup(resolveDir!) : nil
        result.pointee.plugin_data = ESBuildPlugin.serializePluginData(pluginData).map { strdup($0) }
        result.pointee.plugin_name = pluginName != nil ? strdup(pluginName!) : nil

        // Convert errors
        result.pointee.errors_count = Int32(errors.count)
        if !errors.isEmpty {
            result.pointee.errors = UnsafeMutablePointer<c_message>.allocate(capacity: errors.count)
            for (i, error) in errors.enumerated() {
                result.pointee.errors[i] = error.cValue.pointee
            }
        } else {
            result.pointee.errors = nil
        }

        // Convert warnings
        result.pointee.warnings_count = Int32(warnings.count)
        if !warnings.isEmpty {
            result.pointee.warnings = UnsafeMutablePointer<c_message>.allocate(capacity: warnings.count)
            for (i, warning) in warnings.enumerated() {
                result.pointee.warnings[i] = warning.cValue.pointee
            }
        } else {
            result.pointee.warnings = nil
        }

        // Convert watch files
        result.pointee.watch_files_count = Int32(watchFiles.count)
        if !watchFiles.isEmpty {
            result.pointee.watch_files = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: watchFiles.count)
            for (i, file) in watchFiles.enumerated() {
                result.pointee.watch_files[i] = strdup(file)
            }
        } else {
            result.pointee.watch_files = nil
        }

        // Convert watch dirs
        result.pointee.watch_dirs_count = Int32(watchDirs.count)
        if !watchDirs.isEmpty {
            result.pointee.watch_dirs = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                .allocate(capacity: watchDirs.count)
            for (i, dir) in watchDirs.enumerated() {
                result.pointee.watch_dirs[i] = strdup(dir)
            }
        } else {
            result.pointee.watch_dirs = nil
        }

        return result
    }
}

// MARK: - Free Functions

/// Free C plugin structures
public func freePluginCStructures(_ result: UnsafeMutablePointer<c_on_resolve_result>) {
    if result.pointee.path != nil { free(result.pointee.path) }
    if result.pointee.namespace != nil { free(result.pointee.namespace) }
    if result.pointee.suffix != nil { free(result.pointee.suffix) }
    if result.pointee.plugin_data != nil { free(result.pointee.plugin_data) }
    if result.pointee.plugin_name != nil { free(result.pointee.plugin_name) }

    // Free errors
    if result.pointee.errors != nil {
        for i in 0 ..< Int(result.pointee.errors_count) {
            free(result.pointee.errors[i].text)
            if result.pointee.errors[i].location != nil {
                free(result.pointee.errors[i].location.pointee.file)
                free(result.pointee.errors[i].location.pointee.namespace)
                free(result.pointee.errors[i].location.pointee.line_text)
                result.pointee.errors[i].location.deallocate()
            }
        }
        result.pointee.errors.deallocate()
    }

    // Free warnings
    if result.pointee.warnings != nil {
        for i in 0 ..< Int(result.pointee.warnings_count) {
            free(result.pointee.warnings[i].text)
            if result.pointee.warnings[i].location != nil {
                free(result.pointee.warnings[i].location.pointee.file)
                free(result.pointee.warnings[i].location.pointee.namespace)
                free(result.pointee.warnings[i].location.pointee.line_text)
                result.pointee.warnings[i].location.deallocate()
            }
        }
        result.pointee.warnings.deallocate()
    }

    // Free watch files
    if result.pointee.watch_files != nil {
        for i in 0 ..< Int(result.pointee.watch_files_count) {
            free(result.pointee.watch_files[i])
        }
        result.pointee.watch_files.deallocate()
    }

    // Free watch dirs
    if result.pointee.watch_dirs != nil {
        for i in 0 ..< Int(result.pointee.watch_dirs_count) {
            free(result.pointee.watch_dirs[i])
        }
        result.pointee.watch_dirs.deallocate()
    }

    result.deallocate()
}

public func freePluginCStructures(_ result: UnsafeMutablePointer<c_on_load_result>) {
    if result.pointee.contents != nil { free(result.pointee.contents) }
    if result.pointee.resolve_dir != nil { free(result.pointee.resolve_dir) }
    if result.pointee.plugin_data != nil { free(result.pointee.plugin_data) }
    if result.pointee.plugin_name != nil { free(result.pointee.plugin_name) }

    // Free errors
    if result.pointee.errors != nil {
        for i in 0 ..< Int(result.pointee.errors_count) {
            free(result.pointee.errors[i].text)
            if result.pointee.errors[i].location != nil {
                free(result.pointee.errors[i].location.pointee.file)
                free(result.pointee.errors[i].location.pointee.namespace)
                free(result.pointee.errors[i].location.pointee.line_text)
                result.pointee.errors[i].location.deallocate()
            }
        }
        result.pointee.errors.deallocate()
    }

    // Free warnings
    if result.pointee.warnings != nil {
        for i in 0 ..< Int(result.pointee.warnings_count) {
            free(result.pointee.warnings[i].text)
            if result.pointee.warnings[i].location != nil {
                free(result.pointee.warnings[i].location.pointee.file)
                free(result.pointee.warnings[i].location.pointee.namespace)
                free(result.pointee.warnings[i].location.pointee.line_text)
                result.pointee.warnings[i].location.deallocate()
            }
        }
        result.pointee.warnings.deallocate()
    }

    // Free watch files
    if result.pointee.watch_files != nil {
        for i in 0 ..< Int(result.pointee.watch_files_count) {
            free(result.pointee.watch_files[i])
        }
        result.pointee.watch_files.deallocate()
    }

    // Free watch dirs
    if result.pointee.watch_dirs != nil {
        for i in 0 ..< Int(result.pointee.watch_dirs_count) {
            free(result.pointee.watch_dirs[i])
        }
        result.pointee.watch_dirs.deallocate()
    }

    result.deallocate()
}
