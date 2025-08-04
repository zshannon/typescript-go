{
  "targets": [
    {
      "target_name": "tscgo",
      "cflags!": [ "-fno-exceptions" ],
      "cflags_cc!": [ "-fno-exceptions" ],
      "sources": [
        "src/native/tscgo_binding.cc",
        "src/native/file_resolver.cc",
        "src/native/utils.cc",
        "src/native/swift_stubs.c"
      ],
      "include_dirs": [
        "<!@(node -p \"require('node-addon-api').include\")",
        "src/native",
        "lib"
      ],
      "libraries": [
        "<(module_root_dir)/lib/libtsc.a"
      ],
      "dependencies": ["<!(node -p \"require('node-addon-api').targets\"):node_addon_api_except"],
      "defines": [ 
        "NAPI_CPP_EXCEPTIONS"
      ],
      "conditions": [
        ["OS=='linux'", {
          "cflags": ["-fPIC", "-std=c++17"],
          "cflags_cc": ["-fPIC", "-std=c++17", "-fexceptions"],
          "ldflags": ["-Wl,-z,now"]
        }],
        ["OS=='mac'", {
          "xcode_settings": {
            "GCC_ENABLE_CPP_EXCEPTIONS": "YES",
            "CLANG_CXX_LIBRARY": "libc++",
            "CLANG_CXX_LANGUAGE_STANDARD": "c++17",
            "MACOSX_DEPLOYMENT_TARGET": "10.15",
            "OTHER_CFLAGS": ["-fexceptions"]
          }
        }]
      ]
    }
  ]
}