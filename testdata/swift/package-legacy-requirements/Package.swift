// swift-tools-version: 5.1
import PackageDescription

let package = Package(
    name: "LegacyRequirements",
    dependencies: [
        .package(url: "https://example.test/next-minor.git", .upToNextMinor(from: "1.2.0")),
        .package(url: "https://example.test/exact.git", .exact("2.3.4")),
        .package(url: "https://example.test/branch.git", .branch("develop")),
        .package(url: "https://example.test/revision.git", .revision("e74b07278b926c9ec6f9643455ea00d1ce04a021")),
    ]
)
