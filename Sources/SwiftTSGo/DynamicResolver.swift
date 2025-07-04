import Foundation
import TSCBridge

// MARK: - Dynamic File Resolver Callback Management

/// Swift callback registry for dynamic file resolvers (like plugin registry)
private nonisolated(unsafe) var resolverCallbackRegistry: [UnsafeRawPointer: @Sendable (String) async throws
    -> FileResolver?] = [:]
private let resolverRegistryLock = NSLock()

// MARK: - Swift Bridge Functions (like plugin bridge functions)

/// Swift bridge function for dynamic file resolution (like plugin bridge functions)
@_cdecl("swift_file_resolve_callback")
public func swiftFileResolveCallback(
    args: UnsafeMutablePointer<c_file_resolve_args>?,
    callbackData: UnsafeMutableRawPointer?
) -> UnsafeMutablePointer<c_file_resolve_result>? {
    resolverRegistryLock.lock()
    defer { resolverRegistryLock.unlock() }

    guard let callbackData,
          let args,
          let resolverCallback = resolverCallbackRegistry[callbackData]
    else {
        return nil
    }

    // Convert C args to Swift
    let path = String(cString: args.pointee.path)

    // Execute async callback with semaphore blocking (like plugin callbacks)
    let semaphore = DispatchSemaphore(value: 0)
    nonisolated(unsafe) var result: FileResolver?

    Task { @Sendable in
        defer { semaphore.signal() }
        do {
            result = try await resolverCallback(path)
        } catch {
            result = nil
        }
    }

    semaphore.wait()

    // Convert result to C and return
    return result?.cValue
}

// MARK: - FileResolver C Conversion

extension FileResolver {
    /// Convert to C representation
    var cValue: UnsafeMutablePointer<c_file_resolve_result> {
        let cResult = UnsafeMutablePointer<c_file_resolve_result>.allocate(capacity: 1)

        switch self {
        case let .file(content):
            // File case
            cResult.pointee.exists = 1
            cResult.pointee.content = strdup(content)
            cResult.pointee.content_length = Int32(content.utf8.count)
            cResult.pointee.directory_files = nil
            cResult.pointee.directory_files_count = 0

        case let .directory(files):
            // Directory case
            cResult.pointee.exists = 2
            cResult.pointee.content = nil
            cResult.pointee.content_length = 0

            if !files.isEmpty {
                cResult.pointee.directory_files_count = Int32(files.count)
                cResult.pointee.directory_files = UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
                    .allocate(capacity: files.count)

                for (index, file) in files.enumerated() {
                    cResult.pointee.directory_files[index] = strdup(file)
                }
            } else {
                cResult.pointee.directory_files = nil
                cResult.pointee.directory_files_count = 0
            }
        }

        return cResult
    }
}

// MARK: - Callback Registration (like plugin registration)

/// Register a Swift resolver callback and return a callback ID (like plugin registration)
public func registerDynamicResolver(_ resolver: @escaping @Sendable (String) async throws -> FileResolver?)
    -> UnsafeRawPointer
{
    let callbackID = UnsafeRawPointer(Unmanaged.passUnretained(NSObject()).toOpaque())

    resolverRegistryLock.lock()
    resolverCallbackRegistry[callbackID] = resolver
    resolverRegistryLock.unlock()

    return callbackID
}

/// Unregister a resolver callback (like plugin cleanup)
public func unregisterDynamicResolver(_ callbackID: UnsafeRawPointer) {
    resolverRegistryLock.lock()
    resolverCallbackRegistry.removeValue(forKey: callbackID)
    resolverRegistryLock.unlock()
}

// MARK: - Dynamic Build Function

/// Build TypeScript with dynamic file resolution
public func buildWithDynamicResolver(
    projectPath: String,
    printErrors: Bool = false,
    configFile: String = "",
    resolver: @escaping @Sendable (String) async throws -> FileResolver?
) -> UnsafeMutablePointer<c_build_result>? {
    // Register the resolver callback (like plugin registration)
    let callbackID = registerDynamicResolver(resolver)
    defer { unregisterDynamicResolver(callbackID) }

    // Create resolver callbacks structure (like plugin structure)
    let cCallbacks = UnsafeMutablePointer<c_resolver_callbacks>.allocate(capacity: 1)
    defer { cCallbacks.deallocate() }

    cCallbacks.pointee.resolver = swiftFileResolveCallback
    cCallbacks.pointee.resolver_data = UnsafeMutableRawPointer(mutating: callbackID)

    // Call the dynamic build function
    return projectPath.withCString { projectPathPtr in
        configFile.withCString { configFilePtr in
            tsc_build_with_dynamic_resolver(
                UnsafeMutablePointer(mutating: projectPathPtr),
                printErrors ? 1 : 0,
                UnsafeMutablePointer(mutating: configFilePtr),
                cCallbacks
            )
        }
    }
}

// MARK: - C String Helper (using existing helper from BuildInMemory.swift)
