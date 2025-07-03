import Foundation
import TSCBridge

// MARK: - Plugin Callback Management

/// Swift callback registry for plugins  
private nonisolated(unsafe) var pluginCallbackRegistry: [UnsafeRawPointer: PluginCallbackStorage] = [:]
/// Strong references to keep plugin builds alive
private nonisolated(unsafe) var pluginBuildRegistry: [UnsafeRawPointer: RealPluginBuild] = [:]
private let registryLock = NSLock()

/// Storage for plugin callbacks
private class PluginCallbackStorage {
    var resolveCallbacks: [(filter: String, namespace: String?, callback: (ESBuildOnResolveArgs) -> ESBuildOnResolveResult?)] = []
    var loadCallbacks: [(filter: String, namespace: String?, callback: (ESBuildOnLoadArgs) -> ESBuildOnLoadResult?)] = []
    var startCallbacks: [() -> Void] = []
    var endCallbacks: [() -> Void] = []
    var disposeCallbacks: [() -> Void] = []
}

// MARK: - Swift Implementation of PluginBuild

/// Real implementation of ESBuildPluginBuild that stores callbacks
public class RealPluginBuild: ESBuildPluginBuild {
    private let storage: PluginCallbackStorage
    private let pluginID: UnsafeRawPointer
    
    init(pluginID: UnsafeRawPointer) {
        self.pluginID = pluginID
        self.storage = PluginCallbackStorage()
        
        registryLock.lock()
        pluginCallbackRegistry[pluginID] = storage
        registryLock.unlock()
    }
    
    public func onResolve(
        filter: String,
        namespace: String?,
        callback: @escaping (ESBuildOnResolveArgs) -> ESBuildOnResolveResult?
    ) {
        storage.resolveCallbacks.append((filter: filter, namespace: namespace, callback: callback))
    }
    
    public func onLoad(
        filter: String,
        namespace: String?,
        callback: @escaping (ESBuildOnLoadArgs) -> ESBuildOnLoadResult?
    ) {
        storage.loadCallbacks.append((filter: filter, namespace: namespace, callback: callback))
    }
    
    public func onStart(callback: @escaping () -> Void) {
        storage.startCallbacks.append(callback)
    }
    
    public func onEnd(callback: @escaping () -> Void) {
        storage.endCallbacks.append(callback)
    }
    
    public func onDispose(callback: @escaping () -> Void) {
        storage.disposeCallbacks.append(callback)
    }
    
    public func resolve(path: String, options: ESBuildResolveOptions) -> ESBuildResolveResult {
        // For now, return a basic result
        // This would need to call back into esbuild's resolve function
        return ESBuildResolveResult(path: path)
    }
    
    deinit {
        // Clean up callbacks when plugin is destroyed
        for callback in storage.disposeCallbacks {
            callback()
        }
        
        registryLock.lock()
        pluginCallbackRegistry.removeValue(forKey: pluginID)
        pluginBuildRegistry.removeValue(forKey: pluginID)
        registryLock.unlock()
    }
}

// MARK: - C Callback Bridge Functions

/// C callback function for onResolve
@_cdecl("swift_plugin_on_resolve_callback")
public func swiftPluginOnResolveCallback(
    args: UnsafeMutablePointer<c_on_resolve_args>?,
    callbackData: UnsafeMutableRawPointer?
) -> UnsafeMutablePointer<c_on_resolve_result>? {
    registryLock.lock()
    defer { registryLock.unlock() }
    
    guard let callbackData = callbackData,
          let args = args,
          let storage = pluginCallbackRegistry[callbackData] else {
        return nil
    }
    
    // Convert C args to Swift
    let swiftArgs = ESBuildOnResolveArgs(cValue: args)
    
    // Try each resolve callback until one returns a result
    for (filter, namespace, callback) in storage.resolveCallbacks {
        // Check if path matches filter (simplified regex check)
        if swiftArgs.path.range(of: filter, options: .regularExpression) != nil {
            // Check namespace match
            if let namespace = namespace, swiftArgs.namespace != namespace {
                continue
            }
            
            // Call the Swift callback
            if let result = callback(swiftArgs) {
                // Convert result to C and return
                return result.cValue
            }
        }
    }
    
    return nil
}

/// C callback function for onLoad
@_cdecl("swift_plugin_on_load_callback")
public func swiftPluginOnLoadCallback(
    args: UnsafeMutablePointer<c_on_load_args>?,
    callbackData: UnsafeMutableRawPointer?
) -> UnsafeMutablePointer<c_on_load_result>? {
    
    registryLock.lock()
    defer { registryLock.unlock() }
    
    guard let callbackData = callbackData,
          let args = args,
          let storage = pluginCallbackRegistry[callbackData] else {
        return nil
    }
    
    // Convert C args to Swift
    let swiftArgs = ESBuildOnLoadArgs(cValue: args)
    
    // Try each load callback until one returns a result
    for (filter, namespace, callback) in storage.loadCallbacks {
        // Check if path matches filter (simplified regex check)
        if swiftArgs.path.range(of: filter, options: .regularExpression) != nil {
            // Check namespace match
            if let namespace = namespace, swiftArgs.namespace != namespace {
                continue
            }
            
            // Call the Swift callback
            if let result = callback(swiftArgs) {
                // Convert result to C and return
                return result.cValue
            }
        }
    }
    
    return nil
}

/// C callback function for onStart
@_cdecl("swift_plugin_on_start_callback")
public func swiftPluginOnStartCallback(callbackData: UnsafeMutableRawPointer?) {
    registryLock.lock()
    defer { registryLock.unlock() }
    
    guard let callbackData = callbackData,
          let storage = pluginCallbackRegistry[callbackData] else {
        return
    }
    
    for callback in storage.startCallbacks {
        callback()
    }
}

/// C callback function for onEnd
@_cdecl("swift_plugin_on_end_callback")
public func swiftPluginOnEndCallback(callbackData: UnsafeMutableRawPointer?) {
    registryLock.lock()
    defer { registryLock.unlock() }
    
    guard let callbackData = callbackData,
          let storage = pluginCallbackRegistry[callbackData] else {
        return
    }
    
    for callback in storage.endCallbacks {
        callback()
    }
}

// MARK: - Plugin Registration

/// Convert Swift plugin to C representation and register it
extension ESBuildPlugin {
    /// Convert to C plugin structure with registered callbacks
    func createCPlugin() -> UnsafeMutablePointer<c_plugin> {
        let cPlugin = UnsafeMutablePointer<c_plugin>.allocate(capacity: 1)
        cPlugin.pointee.name = strdup(name)
        
        // Create plugin ID for callback storage
        let pluginID = UnsafeRawPointer(cPlugin)
        // Create real plugin build object and run setup
        let realBuild = RealPluginBuild(pluginID: pluginID)
        setup(realBuild)
        
        // Store strong reference to prevent deallocation
        registryLock.lock()
        pluginBuildRegistry[pluginID] = realBuild
        registryLock.unlock()
        
        // Get the stored callbacks
        registryLock.lock()
        guard let storage = pluginCallbackRegistry[pluginID] else {
            registryLock.unlock()
            // If no storage, create empty plugin
            cPlugin.pointee.resolve_hooks = nil
            cPlugin.pointee.resolve_hooks_count = 0
            cPlugin.pointee.load_hooks = nil
            cPlugin.pointee.load_hooks_count = 0
            cPlugin.pointee.on_start = nil
            cPlugin.pointee.on_end = nil
            cPlugin.pointee.start_data = nil
            cPlugin.pointee.end_data = nil
            return cPlugin
        }
        registryLock.unlock()
        
        // Convert resolve callbacks to C hooks
        if !storage.resolveCallbacks.isEmpty {
            cPlugin.pointee.resolve_hooks_count = Int32(storage.resolveCallbacks.count)
            cPlugin.pointee.resolve_hooks = UnsafeMutablePointer<c_plugin_resolve_hook>.allocate(capacity: storage.resolveCallbacks.count)
            
            for (i, (filter, namespace, _)) in storage.resolveCallbacks.enumerated() {
                cPlugin.pointee.resolve_hooks[i].filter = strdup(filter)
                cPlugin.pointee.resolve_hooks[i].namespace = namespace.map { strdup($0) }
                cPlugin.pointee.resolve_hooks[i].callback = swiftPluginOnResolveCallback
                cPlugin.pointee.resolve_hooks[i].callback_data = UnsafeMutableRawPointer(mutating: pluginID)
            }
        } else {
            cPlugin.pointee.resolve_hooks = nil
            cPlugin.pointee.resolve_hooks_count = 0
        }
        
        // Convert load callbacks to C hooks
        if !storage.loadCallbacks.isEmpty {
            cPlugin.pointee.load_hooks_count = Int32(storage.loadCallbacks.count)
            cPlugin.pointee.load_hooks = UnsafeMutablePointer<c_plugin_load_hook>.allocate(capacity: storage.loadCallbacks.count)
            
            for (i, (filter, namespace, _)) in storage.loadCallbacks.enumerated() {
                cPlugin.pointee.load_hooks[i].filter = strdup(filter)
                cPlugin.pointee.load_hooks[i].namespace = namespace.map { strdup($0) }
                cPlugin.pointee.load_hooks[i].callback = swiftPluginOnLoadCallback
                cPlugin.pointee.load_hooks[i].callback_data = UnsafeMutableRawPointer(mutating: pluginID)
            }
        } else {
            cPlugin.pointee.load_hooks = nil
            cPlugin.pointee.load_hooks_count = 0
        }
        
        // Set start/end callbacks if they exist
        if !storage.startCallbacks.isEmpty {
            cPlugin.pointee.on_start = swiftPluginOnStartCallback
            cPlugin.pointee.start_data = UnsafeMutableRawPointer(mutating: pluginID)
        } else {
            cPlugin.pointee.on_start = nil
            cPlugin.pointee.start_data = nil
        }
        
        if !storage.endCallbacks.isEmpty {
            cPlugin.pointee.on_end = swiftPluginOnEndCallback
            cPlugin.pointee.end_data = UnsafeMutableRawPointer(mutating: pluginID)
        } else {
            cPlugin.pointee.on_end = nil
            cPlugin.pointee.end_data = nil
        }
        
        print("Plugin '\(name)' registered with \(storage.resolveCallbacks.count) resolve callbacks and \(storage.loadCallbacks.count) load callbacks")
        
        return cPlugin
    }
    
    /// Free C plugin structure
    static func freeCPlugin(_ cPlugin: UnsafeMutablePointer<c_plugin>) {
        if cPlugin.pointee.name != nil {
            free(cPlugin.pointee.name)
        }
        
        // Free resolve hooks
        if cPlugin.pointee.resolve_hooks != nil {
            for i in 0..<Int(cPlugin.pointee.resolve_hooks_count) {
                free(cPlugin.pointee.resolve_hooks[i].filter)
                if cPlugin.pointee.resolve_hooks[i].namespace != nil {
                    free(cPlugin.pointee.resolve_hooks[i].namespace)
                }
            }
            cPlugin.pointee.resolve_hooks.deallocate()
        }
        
        // Free load hooks
        if cPlugin.pointee.load_hooks != nil {
            for i in 0..<Int(cPlugin.pointee.load_hooks_count) {
                free(cPlugin.pointee.load_hooks[i].filter)
                if cPlugin.pointee.load_hooks[i].namespace != nil {
                    free(cPlugin.pointee.load_hooks[i].namespace)
                }
            }
            cPlugin.pointee.load_hooks.deallocate()
        }
        
        // Clean up registries
        let pluginID = UnsafeRawPointer(cPlugin)
        registryLock.lock()
        pluginCallbackRegistry.removeValue(forKey: pluginID)
        pluginBuildRegistry.removeValue(forKey: pluginID)
        registryLock.unlock()
        
        cPlugin.deallocate()
    }
}