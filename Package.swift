// swift-tools-version:6.1
import PackageDescription

let package = Package(
    name: "SwiftTSGo",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(
            name: "SwiftTSGo",
            targets: ["SwiftTSGo"]
        )
    ],
    targets: [
        .binaryTarget(
            name: "SwiftTSGoMobile",
            path: "./Sources/TSGoBindings.xcframework"
        ),
        .target(
            name: "SwiftTSGo",
            dependencies: [
                .target(name: "SwiftTSGoMobile")
                // .byName(name: "ESBuildMobile")
            ]
        ),
        .testTarget(
            name: "SwiftTSGoTests",
            dependencies: [
                .target(name: "SwiftTSGo"),
                .target(name: "SwiftTSGoMobile"),
            ],
            resources: [
                .copy("Resources")
            ]
        ),
    ]
)
