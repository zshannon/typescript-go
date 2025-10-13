// swift-tools-version:6.1
import PackageDescription
import Foundation

// Use local XCFramework when SWIFTTSGO_LOCAL environment variable is set
// Otherwise use the released version from GitHub
let binaryTarget: Target
if ProcessInfo.processInfo.environment["SWIFTTSGO_LOCAL"] != nil {
    binaryTarget = .binaryTarget(
        name: "TSCBridgeLib",
        path: "Sources/TSCBridge/TSCBridge.xcframework"
    )
} else {
    binaryTarget = .binaryTarget(
        name: "TSCBridgeLib",
        url: "https://github.com/zshannon/typescript-go/releases/download/0.1.2/TSCBridge.xcframework.zip",
        checksum: "787054ee839b9798af9d8be925b76c265d6df7cf2ee25d7d23bec01cffb8a41a"
    )
}

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
