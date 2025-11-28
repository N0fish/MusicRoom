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

        // 2. TODO: Send to backend audit endpoint
        // For now, we just log locally. In a real implementation, this would hit the API.
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
