// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "BeamAppCore",
    platforms: [
        .iOS(.v17),
        .macOS(.v14)
    ],
    products: [
        .library(name: "BeamAppCore", targets: ["BeamAppCore"])
    ],
    targets: [
        .target(name: "BeamAppCore"),
        .testTarget(name: "BeamAppCoreTests", dependencies: ["BeamAppCore"])
    ]
)
