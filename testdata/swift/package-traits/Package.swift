// swift-tools-version: 6.1
import PackageDescription

let package = Package(
    name: "Traits",
    dependencies: [
        .package(
            url: "https://github.com/apple/swift-configuration.git",
            from: "1.0.0",
            traits: [.defaults, "YAML"]
        ),
        .package(id: "acme.no-default-features", from: "2.0.0", traits: []),
        .package(path: "../LocalFeatures", traits: ["Experimental"]),
    ]
)
