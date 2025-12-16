import Dependencies
import Foundation

public struct DiagnosticsSummary: Equatable, Sendable {
    public enum Status: Equatable, Sendable {
        case reachable
        case unreachable(reason: String)
    }

    public let testedURL: URL
    public let status: Status
    public let latencyMs: Double

    public let wsStatus: Status
    public let wsLatencyMs: Double

    public let measuredAt: Date

    public init(
        testedURL: URL,
        status: Status,
        latencyMs: Double,
        wsStatus: Status,
        wsLatencyMs: Double,
        measuredAt: Date = Date()
    ) {
        self.testedURL = testedURL
        self.status = status
        self.latencyMs = latencyMs
        self.wsStatus = wsStatus
        self.wsLatencyMs = wsLatencyMs
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

        async let httpResult = checkHTTP(url: url)
        async let wsResult = checkWebSocket(url: url)

        let (status, latency) = await httpResult
        let (wsStatus, wsLatency) = await wsResult

        return DiagnosticsSummary(
            testedURL: url,
            status: status,
            latencyMs: latency,
            wsStatus: wsStatus,
            wsLatencyMs: wsLatency,
            measuredAt: Date()
        )
    }

    private static func checkHTTP(url: URL) async -> (DiagnosticsSummary.Status, Double) {
        let healthURL = url.appendingPathComponent("health")
        let start = Date()

        do {
            var request = URLRequest(url: healthURL)
            request.httpMethod = "GET"
            request.timeoutInterval = 5

            let (_, response) = try await URLSession.shared.data(for: request)
            let latency = Date().timeIntervalSince(start) * 1_000

            if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200 {
                return (.reachable, latency)
            } else {
                let statusCode = (response as? HTTPURLResponse)?.statusCode ?? -1
                return (.unreachable(reason: "Status code: \(statusCode)"), latency)
            }
        } catch {
            let latency = Date().timeIntervalSince(start) * 1_000
            return (.unreachable(reason: error.localizedDescription), latency)
        }
    }

    private static func checkWebSocket(url: URL) async -> (DiagnosticsSummary.Status, Double) {
        guard let host = url.host, let port = url.port else {
            return (.unreachable(reason: "Invalid URL format"), 0)
        }

        var components = URLComponents()
        components.scheme = url.scheme == "https" ? "wss" : "ws"
        components.host = host == "localhost" ? "127.0.0.1" : host
        components.port = port
        components.path = "/ws"

        guard let wsURL = components.url else {
            return (.unreachable(reason: "Failed to construct WS URL"), 0)
        }

        let start = Date()
        let task = URLSession.shared.webSocketTask(with: wsURL)
        task.resume()

        return await withCheckedContinuation { continuation in
            // Set a timeout
            Task {
                try? await Task.sleep(nanoseconds: 5_000_000_000)  // 5 seconds
                if task.state == .running {
                    task.cancel(with: .normalClosure, reason: nil)
                    // If we haven't resumed by now, it's a timeout.
                    // However, we rely on the receive callback to fire with error on cancel.
                }
            }

            task.receive { result in
                let latency = Date().timeIntervalSince(start) * 1_000
                switch result {
                case .success:
                    continuation.resume(returning: (.reachable, latency))
                case .failure(let error):
                    continuation.resume(
                        returning: (.unreachable(reason: error.localizedDescription), latency))
                }
                task.cancel(with: .normalClosure, reason: nil)
            }
        }
    }

    public static let previewValue = DiagnosticsClient { url in
        DiagnosticsSummary(
            testedURL: url,
            status: .reachable,
            latencyMs: 42,
            wsStatus: .reachable,
            wsLatencyMs: 35
        )
    }

    public static let testValue = DiagnosticsClient { _ in
        DiagnosticsSummary(
            testedURL: URL(string: "https://example.com")!,
            status: .reachable,
            latencyMs: 10,
            wsStatus: .reachable,
            wsLatencyMs: 12
        )
    }
}

extension DependencyValues {
    public var diagnostics: DiagnosticsClient {
        get { self[DiagnosticsClient.self] }
        set { self[DiagnosticsClient.self] = newValue }
    }
}
