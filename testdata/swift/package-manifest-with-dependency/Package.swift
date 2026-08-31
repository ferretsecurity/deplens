import PackageDescription

let package = Package(
    name: "TreeSitterNu",
    dependencies: [
        .package(name: "SwiftTreeSitter", url: "https://github.com/tree-sitter/swift-tree-sitter", from: "0.9.0"),
    ]
)
