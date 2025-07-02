// swift-tools-version:6.1
import PackageDescription

let package = Package(
    name: "SwiftTSGo",
    platforms: [.iOS(.v18), .macOS(.v15)],
    products: [
        .library(
            name: "SwiftTSGo",
            targets: ["SwiftTSGo"]
        )
    ],
    targets: [
        .systemLibrary(
            name: "TSCBridge",
            path: "Sources/TSCBridge"
        ),
        .target(
            name: "SwiftTSGo",
            dependencies: [
                .target(name: "TSCBridge")
            ],
            linkerSettings: [
                .unsafeFlags(["-LSources/TSCBridge"]),
                .linkedLibrary("tsc_macos", .when(platforms: [.macOS])),
                .linkedLibrary("tsc_ios_arm64", .when(platforms: [.iOS])),
            ]
        ),
        .testTarget(
            name: "SwiftTSGoTests",
            dependencies: [
                .target(name: "SwiftTSGo"),
                .target(name: "TSCBridge"),
            ],
            resources: [
                .copy("Resources")
            ]
        ),
    ]
)
