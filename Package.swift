// swift-tools-version:6.1
import PackageDescription
import Foundation

// This Package.swift is for local development and testing.
// The distributed Swift Package lives in swift-package/ submodule.
// Set SWIFTTSGO_LOCAL=1 to test with a locally built XCFramework.

let binaryTarget: Target
if ProcessInfo.processInfo.environment["SWIFTTSGO_LOCAL"] != nil {
    binaryTarget = .binaryTarget(
        name: "TSCBridgeLib",
        path: "Sources/TSCBridge/TSCBridge.xcframework"
    )
} else {
    binaryTarget = .binaryTarget(
        name: "TSCBridgeLib",
        url: "https://github.com/zshannon/typescript-go-swift/releases/download/0.1.2/TSCBridge.xcframework.zip",
        checksum: "fbef72612b8819e62ecf86a5a29f5d2a09b6a2725433d5b58195ceede151743a"
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
            path: "swift-package/Sources/TSCBridge"
        ),
        binaryTarget,
        .target(
            name: "SwiftTSGo",
            path: "swift-package/Sources/SwiftTSGo",
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
