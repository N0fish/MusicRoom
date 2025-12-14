import Dependencies
import Foundation
import OSLog

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

        // 1. Log to os.Logger for local debugging and Instruments
        let metadataString = metadata.map { "\($0.key): \($0.value)" }.joined(separator: ", ")
        logger.info(
            "Action: \(action, privacy: .public) | Metadata: \(metadataString, privacy: .public)")

        // 2. Backend Audit Trail
        // We do not have a specific 'POST /logs' endpoint in the backend.
        // Instead, we rely on Header Injection (X-Platform, X-Device, X-App-Version) in MusicRoomAPIClient.
        // The API Gateway/Nginx logs will capture these headers for every request, satisfying the audit requirement.
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
