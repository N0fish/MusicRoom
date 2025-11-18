import Foundation
import Dependencies
import UIKit

public struct AppMetadata: Equatable, Sendable {
    public let version: String
    public let build: String
    public let deviceModel: String
    public let systemVersion: String

    public init(version: String, build: String, deviceModel: String, systemVersion: String) {
        self.version = version
        self.build = build
        self.deviceModel = deviceModel
        self.systemVersion = systemVersion
    }

    public var summary: String {
        "v\(version) (\(build)) · \(deviceModel) · iOS \(systemVersion)"
    }
}

public struct AppMetadataClient: Sendable {
    public var load: @Sendable () async -> AppMetadata

    public init(load: @escaping @Sendable () async -> AppMetadata) {
        self.load = load
    }
}

extension AppMetadataClient: DependencyKey {
    public static let liveValue = AppMetadataClient {
        await MainActor.run {
            let bundle = Bundle.main
            let version = bundle.infoDictionary?["CFBundleShortVersionString"] as? String ?? "0.0"
            let build = bundle.infoDictionary?["CFBundleVersion"] as? String ?? "0"
            let device = UIDevice.current
            return AppMetadata(version: version, build: build, deviceModel: device.model, systemVersion: device.systemVersion)
        }
    }

    public static let previewValue = AppMetadataClient {
        AppMetadata(version: "0.1", build: "1", deviceModel: "iPhone 16 Pro", systemVersion: "18.1")
    }

    public static let testValue = AppMetadataClient {
        AppMetadata(version: "0.0", build: "0", deviceModel: "Test Device", systemVersion: "0")
    }
}

extension DependencyValues {
    public var appMetadata: AppMetadataClient {
        get { self[AppMetadataClient.self] }
        set { self[AppMetadataClient.self] = newValue }
    }
}
