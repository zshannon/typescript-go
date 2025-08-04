#include <napi.h>
#include <string>
#include <vector>
#include <memory>
#include <mutex>
#include <unordered_map>
#include "tsc_bridge_wrapper.h"
#include "file_resolver.h"
#include "utils.h"

using namespace Napi;

// Forward declarations for our main functions
Value BuildFileSystem(const CallbackInfo& info);
Value BuildWithResolver(const CallbackInfo& info);
Value BuildWithDynamicResolver(const CallbackInfo& info);

// Initialize the module
Object Init(Env env, Object exports) {
    exports.Set(String::New(env, "buildFileSystem"), Function::New(env, BuildFileSystem));
    exports.Set(String::New(env, "buildWithResolver"), Function::New(env, BuildWithResolver));
    exports.Set(String::New(env, "buildWithDynamicResolver"), Function::New(env, BuildWithDynamicResolver));
    
    // Initialize the file resolver with the environment
    FileResolver::Initialize(env);
    
    return exports;
}

// Convert C diagnostic to JS object
Object DiagnosticToObject(Env env, const c_diagnostic& diag) {
    Object obj = Object::New(env);
    obj.Set("code", Number::New(env, diag.code));
    obj.Set("category", String::New(env, diag.category ? diag.category : ""));
    obj.Set("message", String::New(env, diag.message ? diag.message : ""));
    obj.Set("file", String::New(env, diag.file ? diag.file : ""));
    obj.Set("line", Number::New(env, diag.line));
    obj.Set("column", Number::New(env, diag.column));
    obj.Set("length", Number::New(env, diag.length));
    return obj;
}

// Convert C build result to JS object
Object BuildResultToObject(Env env, c_build_result* result) {
    Object obj = Object::New(env);
    
    if (!result) {
        obj.Set("success", Boolean::New(env, false));
        obj.Set("diagnostics", Array::New(env));
        obj.Set("compiledFiles", Array::New(env));
        obj.Set("configFile", String::New(env, ""));
        obj.Set("writtenFiles", Object::New(env));
        return obj;
    }
    
    // Basic fields
    obj.Set("success", Boolean::New(env, result->success != 0));
    obj.Set("configFile", String::New(env, result->config_file ? result->config_file : ""));
    
    // Diagnostics
    Array diagnostics = Array::New(env);
    for (int i = 0; i < result->diagnostic_count; i++) {
        diagnostics.Set(i, DiagnosticToObject(env, result->diagnostics[i]));
    }
    obj.Set("diagnostics", diagnostics);
    
    // Written files
    Object writtenFiles = Object::New(env);
    for (int i = 0; i < result->written_file_count; i++) {
        if (result->written_file_paths[i] && result->written_file_contents[i]) {
            writtenFiles.Set(
                String::New(env, result->written_file_paths[i]),
                String::New(env, result->written_file_contents[i])
            );
        }
    }
    obj.Set("writtenFiles", writtenFiles);
    
    // Compiled files (derived from written files)
    Array compiledFiles = Array::New(env);
    int compiledIndex = 0;
    for (int i = 0; i < result->written_file_count; i++) {
        if (result->written_file_paths[i] && result->written_file_contents[i]) {
            Object source = Object::New(env);
            std::string path(result->written_file_paths[i]);
            size_t lastSlash = path.find_last_of("/\\");
            std::string filename = (lastSlash != std::string::npos) ? path.substr(lastSlash + 1) : path;
            
            source.Set("name", String::New(env, filename));
            source.Set("content", String::New(env, result->written_file_contents[i]));
            compiledFiles.Set(compiledIndex++, source);
        }
    }
    obj.Set("compiledFiles", compiledFiles);
    
    return obj;
}

// Build from filesystem
Value BuildFileSystem(const CallbackInfo& info) {
    Env env = info.Env();
    
    if (info.Length() < 1 || !info[0].IsString()) {
        TypeError::New(env, "Project path must be a string").ThrowAsJavaScriptException();
        return env.Null();
    }
    
    std::string projectPath = info[0].As<String>().Utf8Value();
    bool printErrors = info.Length() > 1 && info[1].IsBoolean() ? info[1].As<Boolean>().Value() : false;
    std::string configFile = info.Length() > 2 && info[2].IsString() ? info[2].As<String>().Utf8Value() : "";
    
    c_build_result* result = tsc_build_filesystem(
        const_cast<char*>(projectPath.c_str()),
        printErrors ? 1 : 0,
        const_cast<char*>(configFile.c_str())
    );
    
    Object jsResult = BuildResultToObject(env, result);
    
    if (result) {
        tsc_free_result(result);
    }
    
    return jsResult;
}

// Build with static resolver
Value BuildWithResolver(const CallbackInfo& info) {
    Env env = info.Env();
    
    if (info.Length() < 4) {
        TypeError::New(env, "Expected projectPath, printErrors, configFile, and resolverData").ThrowAsJavaScriptException();
        return env.Null();
    }
    
    std::string projectPath = info[0].As<String>().Utf8Value();
    bool printErrors = info[1].As<Boolean>().Value();
    std::string configFile = info[2].As<String>().Utf8Value();
    Object resolverData = info[3].As<Object>();
    
    // Create C resolver data
    c_file_resolver_data* cResolverData = tsc_create_resolver_data();
    
    // Add files
    if (resolverData.Has("files")) {
        Object files = resolverData.Get("files").As<Object>();
        Array fileKeys = files.GetPropertyNames();
        
        for (uint32_t i = 0; i < fileKeys.Length(); i++) {
            Value keyValue = fileKeys.Get(i);
            if (!keyValue.IsString()) continue;
            
            std::string path = keyValue.As<String>().Utf8Value();
            Value contentValue = files.Get(path);
            if (!contentValue.IsString()) continue;
            
            std::string content = contentValue.As<String>().Utf8Value();
            
            tsc_add_file_to_resolver(
                cResolverData,
                const_cast<char*>(path.c_str()),
                const_cast<char*>(content.c_str())
            );
        }
    }
    
    // Add directories
    if (resolverData.Has("directories")) {
        Array directories = resolverData.Get("directories").As<Array>();
        
        for (uint32_t i = 0; i < directories.Length(); i++) {
            std::string dir = directories.Get(i).As<String>().Utf8Value();
            tsc_add_directory_to_resolver(
                cResolverData,
                const_cast<char*>(dir.c_str())
            );
        }
    }
    
    // Build
    c_build_result* result = tsc_build_with_resolver(
        const_cast<char*>(projectPath.c_str()),
        printErrors ? 1 : 0,
        const_cast<char*>(configFile.c_str()),
        cResolverData
    );
    
    Object jsResult = BuildResultToObject(env, result);
    
    // Cleanup
    if (cResolverData) {
        tsc_free_resolver_data(cResolverData);
    }
    if (result) {
        tsc_free_result(result);
    }
    
    return jsResult;
}

// Build with dynamic resolver (async) - using ThreadSafeFunction
class BuildWorker : public AsyncWorker {
public:
    BuildWorker(const Function& callback, const std::string& projectPath, bool printErrors, 
                const std::string& configFile, const Function& resolver)
        : AsyncWorker(callback), projectPath_(projectPath), printErrors_(printErrors),
          configFile_(configFile) {
        // Store the resolver function in a thread-safe way
        resolverRef_ = ThreadSafeFunction::New(
            callback.Env(),
            resolver,
            "TSCGoResolver",
            0,  // Unlimited queue
            1   // Initial thread count
        );
    }
    
    ~BuildWorker() {
        // Release the thread-safe function
        resolverRef_.Release();
    }
    
    void Execute() override {
        // Register the resolver with thread-safe function
        callbackId_ = FileResolver::RegisterThreadSafeResolver(resolverRef_);
        
        // Create C resolver callbacks
        c_resolver_callbacks callbacks;
        callbacks.resolver = FileResolver::ResolveCallback;
        callbacks.resolver_data = callbackId_;
        
        // Build with dynamic resolver
        result_ = tsc_build_with_dynamic_resolver(
            const_cast<char*>(projectPath_.c_str()),
            printErrors_ ? 1 : 0,
            const_cast<char*>(configFile_.c_str()),
            &callbacks
        );
    }
    
    void OnOK() override {
        HandleScope scope(Env());
        
        // Convert result to JS
        Object jsResult = BuildResultToObject(Env(), result_);
        
        // Cleanup
        FileResolver::UnregisterResolver(callbackId_);
        if (result_) {
            tsc_free_result(result_);
        }
        
        // Call the callback
        Callback().Call({Env().Null(), jsResult});
    }
    
    void OnError(const Error& error) override {
        HandleScope scope(Env());
        
        // Cleanup on error
        FileResolver::UnregisterResolver(callbackId_);
        if (result_) {
            tsc_free_result(result_);
        }
        
        Callback().Call({error.Value(), Env().Null()});
    }
    
private:
    std::string projectPath_;
    bool printErrors_;
    std::string configFile_;
    ThreadSafeFunction resolverRef_;
    void* callbackId_;
    c_build_result* result_ = nullptr;
};

Value BuildWithDynamicResolver(const CallbackInfo& info) {
    Env env = info.Env();
    
    if (info.Length() < 5) {
        TypeError::New(env, "Expected projectPath, printErrors, configFile, resolver, and callback").ThrowAsJavaScriptException();
        return env.Undefined();
    }
    
    std::string projectPath = info[0].As<String>().Utf8Value();
    bool printErrors = info[1].As<Boolean>().Value();
    std::string configFile = info[2].As<String>().Utf8Value();
    Function resolver = info[3].As<Function>();
    Function callback = info[4].As<Function>();
    
    BuildWorker* worker = new BuildWorker(callback, projectPath, printErrors, configFile, resolver);
    worker->Queue();
    
    return env.Undefined();
}


NODE_API_MODULE(tscgo, Init)