import ProjectDescription

let deploymentTargets: DeploymentTargets = .iOS("26.0")

let project = Project(
    name: "MusicRoomMobile",
    organizationName: "Music Room",
    packages: [
        .remote(
            url: "https://github.com/pointfreeco/swift-composable-architecture",
            requirement: .upToNextMajor(from: "1.23.0")),
        .remote(
            url: "https://github.com/pointfreeco/swift-dependencies",
            requirement: .upToNextMajor(from: "1.10.0")),
        .remote(
            url: "https://github.com/pointfreeco/swift-case-paths",
            requirement: .upToNextMajor(from: "1.7.2")),
        .remote(
            url: "https://github.com/pointfreeco/swift-navigation",
            requirement: .upToNextMajor(from: "2.6.0")),
        .remote(
            url: "https://github.com/pointfreeco/swift-concurrency-extras",
            requirement: .upToNextMajor(from: "1.3.2")),
        .remote(
            url: "https://github.com/pointfreeco/swift-clocks",
            requirement: .upToNextMajor(from: "1.0.0")),
    ],
    settings: .settings(base: [
        "SWIFT_VERSION": "6.0",
        "SWIFT_STRICT_CONCURRENCY": "complete",
        "SWIFT_TREAT_WARNINGS_AS_ERRORS": "YES",
    ]),
    targets: [
        Target.target(
            name: "MusicRoomUI",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.ui",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/MusicRoomUI/Sources/**"],
            resources: ["Targets/MusicRoomUI/Resources/**"],
            dependencies: []
        ),
        Target.target(
            name: "MusicRoomDomain",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.domain",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/MusicRoomDomain/Sources/**"],
            dependencies: []
        ),
        Target.target(
            name: "MusicRoomAPI",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.api",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/MusicRoomAPI/Sources/**"],
            dependencies: [
                .target(name: "MusicRoomDomain"),
                .target(name: "AppSettingsClient"),
                .package(product: "Dependencies"),
            ]
        ),
        Target.target(
            name: "PolicyEngine",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.policyengine",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/PolicyEngine/Sources/**"],
            dependencies: [
                .target(name: "MusicRoomDomain"),
                .package(product: "Dependencies"),
            ]
        ),
        Target.target(
            name: "RealtimeMocks",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.realtimemocks",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/RealtimeMocks/Sources/**"],
            dependencies: [
                .target(name: "MusicRoomDomain"),
                .package(product: "Dependencies"),
            ]
        ),
        Target.target(
            name: "MusicRoomMobile",
            destinations: .iOS,
            product: .app,
            bundleId: "com.musicroom.mobile",
            deploymentTargets: deploymentTargets,
            infoPlist: .extendingDefault(with: [
                "UILaunchScreen": [:],
                "CFBundleURLTypes": [
                    [
                        "CFBundleTypeRole": "Editor",
                        "CFBundleURLName": "com.musicroom.mobile",
                        "CFBundleURLSchemes": ["musicroom"],
                    ]
                ],
            ]),
            sources: ["Targets/MusicRoomMobile/Sources/**"],
            resources: ["Targets/MusicRoomMobile/Resources/**"],
            dependencies: [
                .target(name: "AppFeature"),
                .target(name: "EventFeature"),
                .package(product: "ComposableArchitecture"),
                .package(product: "Dependencies"),
                .package(product: "CasePaths"),
                .package(product: "SwiftNavigation"),
            ]
        ),
        Target.target(
            name: "AppFeature",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.appfeature",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/AppFeature/Sources/**"],
            dependencies: [
                .target(name: "SettingsFeature"),
                .target(name: "AuthenticationFeature"),
                .target(name: "EventFeature"),
                .target(name: "AppSettingsClient"),
                .target(name: "MusicRoomDomain"),
                .target(name: "MusicRoomAPI"),
                .target(name: "PolicyEngine"),
                .target(name: "RealtimeMocks"),
                .target(name: "AppSupportClients"),
                .package(product: "ComposableArchitecture"),
            ]
        ),
        Target.target(
            name: "SettingsFeature",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.settingsfeature",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/SettingsFeature/Sources/**"],
            dependencies: [
                .target(name: "AppSettingsClient"),
                .target(name: "AppSupportClients"),
                .package(product: "ComposableArchitecture"),
            ]
        ),
        Target.target(
            name: "AuthenticationFeature",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.authenticationfeature",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/AuthenticationFeature/Sources/**"],
            dependencies: [
                .target(name: "AppSupportClients"),
                .target(name: "AppSettingsClient"),
                .target(name: "MusicRoomUI"),
                .package(product: "ComposableArchitecture"),
            ]
        ),
        Target.target(
            name: "EventFeature",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.eventfeature",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/EventFeature/Sources/**"],
            dependencies: [
                .target(name: "AppSupportClients"),
                .target(name: "MusicRoomUI"),
                .target(name: "MusicRoomDomain"),
                .target(name: "MusicRoomAPI"),
                .package(product: "ComposableArchitecture"),
            ]
        ),
        Target.target(
            name: "AppSupportClients",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.appsupportclients",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/AppSupportClients/Sources/**"],
            dependencies: [
                .package(product: "Dependencies")
            ]
        ),
        Target.target(
            name: "AppSettingsClient",
            destinations: .iOS,
            product: .staticLibrary,
            bundleId: "com.musicroom.appsettingsclient",
            deploymentTargets: deploymentTargets,
            sources: ["Targets/AppSettingsClient/Sources/**"],
            dependencies: [
                .package(product: "Dependencies")
            ]
        ),
        Target.target(
            name: "MusicRoomMobileTests",
            destinations: .iOS,
            product: .unitTests,
            bundleId: "com.musicroom.mobileTests",
            deploymentTargets: deploymentTargets,
            infoPlist: .default,
            sources: [
                "Targets/MusicRoomMobile/Tests/**",
                "Targets/EventFeature/Tests/**",
                "Targets/MusicRoomAPI/Tests/**",
            ],
            dependencies: [
                .target(name: "MusicRoomMobile"),
                .target(name: "AppFeature"),
                .target(name: "SettingsFeature"),
                .target(name: "EventFeature"),
                .target(name: "MusicRoomAPI"),
                .target(name: "MusicRoomDomain"),
                .target(name: "AppSettingsClient"),
                .package(product: "ComposableArchitecture"),
                .package(product: "SwiftNavigation"),
                .package(product: "CasePaths"),
                .package(product: "ConcurrencyExtras"),
                .package(product: "Dependencies"),
                .package(product: "Clocks"),
            ]
        ),
    ]
)
