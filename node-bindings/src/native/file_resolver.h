#ifndef FILE_RESOLVER_H
#define FILE_RESOLVER_H

#include <napi.h>
#include <mutex>
#include <unordered_map>
#include "tsc_bridge_wrapper.h"

class FileResolver {
public:
    // Initialize with environment
    static void Initialize(Napi::Env env);
    
    // Register a JS resolver function and return a callback ID
    static void* RegisterResolver(Napi::Function resolver);
    
    // Register a thread-safe resolver function
    static void* RegisterThreadSafeResolver(Napi::ThreadSafeFunction resolver);
    
    // Unregister a resolver
    static void UnregisterResolver(void* callbackId);
    
    // C callback function that bridges to JS
    static c_file_resolve_result* ResolveCallback(c_file_resolve_args* args, void* data);
    
private:
    static std::mutex mutex_;
    static std::unordered_map<void*, Napi::FunctionReference> resolvers_;
    static std::unordered_map<void*, Napi::ThreadSafeFunction> threadSafeResolvers_;
    static uint64_t nextId_;
    static Napi::Env env_;
};

#endif // FILE_RESOLVER_H