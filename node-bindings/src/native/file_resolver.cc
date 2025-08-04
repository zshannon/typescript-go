#include "file_resolver.h"
#include <cstring>
#include <thread>
#include <chrono>
#include <condition_variable>

using namespace Napi;

// Static member definitions
std::mutex FileResolver::mutex_;
std::unordered_map<void*, FunctionReference> FileResolver::resolvers_;
std::unordered_map<void*, ThreadSafeFunction> FileResolver::threadSafeResolvers_;
uint64_t FileResolver::nextId_ = 1;
Env FileResolver::env_ = nullptr;

void FileResolver::Initialize(Env env) {
    env_ = env;
}

void* FileResolver::RegisterResolver(Function resolver) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    void* id = reinterpret_cast<void*>(nextId_++);
    resolvers_[id] = Persistent(resolver);
    
    return id;
}

void* FileResolver::RegisterThreadSafeResolver(ThreadSafeFunction resolver) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    void* id = reinterpret_cast<void*>(nextId_++);
    threadSafeResolvers_[id] = resolver;
    
    return id;
}

void FileResolver::UnregisterResolver(void* callbackId) {
    std::lock_guard<std::mutex> lock(mutex_);
    resolvers_.erase(callbackId);
    threadSafeResolvers_.erase(callbackId);
}

// Structure to pass data between threads
struct ResolverCallData {
    std::string path;
    c_file_resolve_result* result = nullptr;
    std::condition_variable cv;
    std::mutex mutex;
    bool ready = false;
};

c_file_resolve_result* FileResolver::ResolveCallback(c_file_resolve_args* args, void* data) {
    if (!args || !data) {
        return nullptr;
    }
    
    // Check if we have a thread-safe resolver first
    ThreadSafeFunction tsResolver = nullptr;
    {
        std::lock_guard<std::mutex> lock(mutex_);
        auto tsIt = threadSafeResolvers_.find(data);
        if (tsIt != threadSafeResolvers_.end()) {
            tsResolver = tsIt->second;
        }
    }
    
    if (tsResolver) {
        // Use thread-safe function to call back to JS
        ResolverCallData* callData = new ResolverCallData();
        callData->path = std::string(args->path);
        
        // Call the JS resolver through the thread-safe function
        auto status = tsResolver.BlockingCall([callData](Env env, Function resolver) {
            HandleScope scope(env);
            
            try {
                Value jsResult = resolver.Call({String::New(env, callData->path)});
                
                // Check if result is a Promise - this shouldn't happen with our sync resolver
                if (jsResult.IsPromise()) {
                    // Return not found if we get a Promise (shouldn't happen)
                    callData->result = static_cast<c_file_resolve_result*>(calloc(1, sizeof(c_file_resolve_result)));
                    callData->result->exists = 0; // Not found
                    return;
                }
                
                callData->result = static_cast<c_file_resolve_result*>(calloc(1, sizeof(c_file_resolve_result)));
                
                if (jsResult.IsNull() || jsResult.IsUndefined()) {
                    // File not found
                    callData->result->exists = 0;
                } else if (jsResult.IsObject()) {
                    Object obj = jsResult.As<Object>();
                    
                    if (obj.Has("type")) {
                        std::string type = obj.Get("type").As<String>().Utf8Value();
                        
                        if (type == "file" && obj.Has("content")) {
                            // File result
                            std::string content = obj.Get("content").As<String>().Utf8Value();
                            callData->result->exists = 1;
                            callData->result->content = strdup(content.c_str());
                            callData->result->content_length = content.length();
                        } else if (type == "directory" && obj.Has("files")) {
                            // Directory result
                            Array files = obj.Get("files").As<Array>();
                            callData->result->exists = 2;
                            
                            if (files.Length() > 0) {
                                callData->result->directory_files_count = files.Length();
                                callData->result->directory_files = static_cast<char**>(calloc(files.Length(), sizeof(char*)));
                                
                                for (uint32_t i = 0; i < files.Length(); i++) {
                                    std::string file = files.Get(i).As<String>().Utf8Value();
                                    callData->result->directory_files[i] = strdup(file.c_str());
                                }
                            }
                        }
                    }
                }
            } catch (...) {
                // Error - return null result
                if (callData->result) {
                    free(callData->result);
                    callData->result = nullptr;
                }
            }
            
            // Signal completion
            {
                std::lock_guard<std::mutex> lock(callData->mutex);
                callData->ready = true;
            }
            callData->cv.notify_one();
        });
        
        if (status == napi_ok) {
            // Wait for the result
            std::unique_lock<std::mutex> lock(callData->mutex);
            callData->cv.wait(lock, [callData] { return callData->ready; });
            
            c_file_resolve_result* result = callData->result;
            delete callData;
            return result;
        } else {
            delete callData;
        }
    }
    
    // Fall back to regular resolver if available (not used in current implementation)
    {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = resolvers_.find(data);
        if (it == resolvers_.end()) {
            return nullptr;
        }
        // Regular resolver exists but sync fallback not implemented
    }
    
    // This path should not be reached in async context
    // Return empty result
    c_file_resolve_result* result = static_cast<c_file_resolve_result*>(calloc(1, sizeof(c_file_resolve_result)));
    result->exists = 0;
    return result;
}