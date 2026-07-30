// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "URLRangeAndExact",
    dependencies: [
        .package(url: "https://github.com/onevcat/Kingfisher.git", "7.0.0"..<"8.0.0"),
        .package(url: "https://example.test/team/inclusive-range.git", "2.0.0"..."2.5.0"),
        .package(url: "https://github.com/pointfreeco/swift-composable-architecture.git", exact: "1.10.0"),
    ]
)
