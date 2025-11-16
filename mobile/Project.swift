import ProjectDescription

let deploymentTargets: DeploymentTargets = .iOS("26.0")

let project = Project(
    name: "MusicRoomMobile",
    organizationName: "Music Room",
    packages: [
        .remote(url: "https://github.com/pointfreeco/swift-composable-architecture", requirement: .upToNextMajor(from: "1.23.0"))
    ],
    settings: .settings(base: [
        "SWIFT_VERSION": "6.0",
        "SWIFT_STRICT_CONCURRENCY": "complete"
    ]),
    targets: [
        Target.target(
            name: "MusicRoomMobile",
            destinations: .iOS,
            product: .app,
            bundleId: "com.musicroom.mobile",
            deploymentTargets: deploymentTargets,
            infoPlist: .extendingDefault(with: [
                "UILaunchScreen": [:]
            ]),
            sources: ["Targets/MusicRoomMobile/Sources/**"],
            resources: ["Targets/MusicRoomMobile/Resources/**"],
            dependencies: [
                .package(product: "ComposableArchitecture")
            ]
        ),
        Target.target(
            name: "MusicRoomMobileTests",
            destinations: .iOS,
            product: .unitTests,
            bundleId: "com.musicroom.mobileTests",
            deploymentTargets: deploymentTargets,
            infoPlist: .default,
            sources: ["Targets/MusicRoomMobile/Tests/**"],
            dependencies: [
                .target(name: "MusicRoomMobile")
            ]
        )
    ]
)
