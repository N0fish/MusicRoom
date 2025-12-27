import AppSettingsClient
import Dependencies
import Foundation
import OSLog
import UIKit

public struct TelemetryClient: Sendable {
    public var log: @Sendable (_ action: String, _ metadata: [String: String]) async -> Void

    public init(
        log: @escaping @Sendable (_ action: String, _ metadata: [String: String]) async -> Void
    ) {
        self.log = log
    }
}

extension TelemetryClient: DependencyKey {
    public static let liveValue = TelemetryClient { action, metadata in
        let logger = Logger(
            subsystem: Bundle.main.bundleIdentifier ?? "com.musicroom.mobile", category: "Audit")

        // 1. Log to os.Logger
        let metadataString =
            metadata.isEmpty
            ? "none" : metadata.map { "\($0.key): \($0.value)" }.joined(separator: ", ")
        logger.info(
            "Action: \(action, privacy: .public) | Metadata: \(metadataString, privacy: .public)")

        // 2. Backend Audit Trail
        @Dependency(\.appSettings) var settings
        @Dependency(\.authentication) var authentication
        let executor = AuthenticatedRequestExecutor(urlSession: .shared, authentication: authentication)

        let backendURL = settings.load().backendURL
        let endpoint = backendURL.appendingPathComponent("audit/logs")  // Generic endpoint for logs

        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        // Headers
        request.setValue("iOS", forHTTPHeaderField: "X-Client-Platform")
        let deviceName = await MainActor.run { UIDevice.current.name }
        request.setValue(deviceName, forHTTPHeaderField: "X-Client-Device")
        if let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String {
            request.setValue(version, forHTTPHeaderField: "X-Client-App-Version")
        }

        let body: [String: Any] = [
            "action": action,
            "metadata": metadata,
            "timestamp": ISO8601DateFormatter().string(from: Date()),
        ]

        do {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
            let (_, httpResponse) = try await executor.data(for: request)
            if !(200...299).contains(httpResponse.statusCode) {
                logger.error("Telemetry failed: Server returned \(httpResponse.statusCode)")
            }
        } catch {
            logger.error("Telemetry failed: \(error.localizedDescription)")
        }
    }

    public static let previewValue = TelemetryClient { action, metadata in
        print("[Preview Telemetry] Action: \(action) | Metadata: \(metadata)")
    }

    public static let testValue = TelemetryClient { _, _ in }
}

extension DependencyValues {
    public var telemetry: TelemetryClient {
        get { self[TelemetryClient.self] }
        set { self[TelemetryClient.self] = newValue }
    }
}
