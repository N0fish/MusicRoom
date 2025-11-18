import Foundation
import Dependencies

public struct DiagnosticsSummary: Equatable, Sendable {
    public enum Status: Equatable, Sendable {
        case reachable
        case unreachable(reason: String)
    }

    public let testedURL: URL
    public let status: Status
    public let latencyMs: Double
    public let measuredAt: Date

    public init(
        testedURL: URL,
        status: Status,
        latencyMs: Double,
        measuredAt: Date = Date()
    ) {
        self.testedURL = testedURL
        self.status = status
        self.latencyMs = latencyMs
        self.measuredAt = measuredAt
    }
}

public struct DiagnosticsClient: Sendable {
    public var ping: @Sendable (_ url: URL) async throws -> DiagnosticsSummary

    public init(ping: @escaping @Sendable (_ url: URL) async throws -> DiagnosticsSummary) {
        self.ping = ping
    }
}

public enum DiagnosticsError: Error, Equatable {
    case invalidURL
    case unsupportedScheme
}

extension DiagnosticsClient: DependencyKey {
    public static let liveValue: DiagnosticsClient = DiagnosticsClient { url in
        guard url.scheme?.isEmpty == false else { throw DiagnosticsError.unsupportedScheme }
        let start = Date()
        try await Task.sleep(nanoseconds: 250_000_000)
        let latency = Date().timeIntervalSince(start) * 1_000
        let trustedHosts = ["localhost", "127.0.0.1", "dev.musicroom", "staging.api.musicroom.app"]
        let status: DiagnosticsSummary.Status
        if let host = url.host, trustedHosts.contains(where: { host.contains($0) }) {
            status = .reachable
        } else {
            status = .unreachable(reason: "Backend not running yet – simulated response.")
        }
        return DiagnosticsSummary(
            testedURL: url,
            status: status,
            latencyMs: latency,
            measuredAt: Date()
        )
    }

    public static let previewValue = DiagnosticsClient { url in
        DiagnosticsSummary(testedURL: url, status: .reachable, latencyMs: 42)
    }

    public static let testValue = DiagnosticsClient { _ in
        DiagnosticsSummary(
            testedURL: URL(string: "https://example.com")!,
            status: .reachable,
            latencyMs: 10
        )
    }
}

extension DependencyValues {
    public var diagnostics: DiagnosticsClient {
        get { self[DiagnosticsClient.self] }
        set { self[DiagnosticsClient.self] = newValue }
    }
}
