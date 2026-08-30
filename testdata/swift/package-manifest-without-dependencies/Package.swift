import PackageDescription

let package = Package(
    name: "AcaiaSDK",
    targets: [
        .binaryTarget(name: "AcaiaSDK", path: "AcaiaSDK.xcframework"),
    ]
)
