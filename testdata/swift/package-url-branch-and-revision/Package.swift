// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "URLBranchAndRevision",
    dependencies: [
        .package(url: "ssh://git@example.test/team/development-kit.git", branch: "next"),
        .package(url: "https://example.test/team/pinned-kit.git", revision: "aa681bd6c61e22df0fd808044a886fc4a7ed3a65"),
    ]
)
