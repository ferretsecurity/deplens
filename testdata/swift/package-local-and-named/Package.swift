// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "LocalAndNamed",
    dependencies: [
        .package(path: "../SharedUtilities"),
        .package(name: "InternalKit", path: "../../InternalKit"),
        .package(name: "Wire", url: "https://example.test/wire.git", from: "1.2.0"),
    ]
)
