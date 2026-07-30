// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "Registry",
    dependencies: [
        .package(id: "mona.logging", from: "2.1.0"),
        .package(id: "acme.half-open-range", "1.0.0"..<"2.0.0"),
        .package(id: "acme.closed-range", "4.0.0"..."4.2.0"),
        .package(id: "acme.telemetry", exact: "3.0.1"),
    ]
)
