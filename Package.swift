// swift-tools-version:6.1
import PackageDescription

// Use local XCFramework when SWIFTTSGO_LOCAL environment variable is set
// Otherwise use the released version from GitHub
#if SWIFTTSGO_LOCAL
let binaryTarget: Target = .binaryTarget(
    name: "TSCBridgeLib",
    path: "Sources/TSCBridge/TSCBridge.xcframework"
)
#else
let binaryTarget: Target = .binaryTarget(
    name: "TSCBridgeLib",
    url: "https://github.com/zshannon/typescript-go/releases/download/0.1.0/TSCBridge.xcframework.zip",
    checksum: "0000000000000000000000000000000000000000000000000000000000000000"
)
#endif

let package = Package(
    name: "SwiftTSGo",
    platforms: [.iOS(.v18), .macOS(.v15)],
    products: [
        .library(
            name: "SwiftTSGo",
            targets: ["SwiftTSGo"]
        ),
    ],
    targets: [
        .systemLibrary(
            name: "TSCBridge",
            path: "Sources/TSCBridge"
        ),
        binaryTarget,
        .target(
            name: "SwiftTSGo",
            dependencies: [
                .target(name: "TSCBridge"),
                .target(name: "TSCBridgeLib"),
            ]
        ),
        .testTarget(
            name: "SwiftTSGoTests",
            dependencies: [
                .target(name: "SwiftTSGo"),
            ],
            resources: [
                .copy("Resources"),
            ]
        ),
    ]
)
