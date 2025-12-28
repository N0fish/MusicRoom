import Dependencies
import Foundation
import Network

public enum NetworkStatus: Sendable, Equatable {
    case satisfied
    case unsatisfied
    case requiresConnection
    case unknown
}

public struct NetworkMonitorClient: Sendable {
    public var start: @Sendable () -> AsyncStream<NetworkStatus>
}

extension DependencyValues {
    public var networkMonitor: NetworkMonitorClient {
        get { self[NetworkMonitorClient.self] }
        set { self[NetworkMonitorClient.self] = newValue }
    }
}

extension NetworkMonitorClient: DependencyKey {
    public static var liveValue: NetworkMonitorClient {
        NetworkMonitorClient(
            start: {
                AsyncStream { continuation in
                    let monitor = NWPathMonitor()
                    let queue = DispatchQueue(label: "NetworkMonitorClient")

                    monitor.pathUpdateHandler = { path in
                        let status: NetworkStatus
                        switch path.status {
                        case .satisfied: status = .satisfied
                        case .unsatisfied: status = .unsatisfied
                        case .requiresConnection: status = .requiresConnection
                        @unknown default: status = .unknown
                        }
                        continuation.yield(status)
                    }

                    monitor.start(queue: queue)

                    continuation.onTermination = { _ in
                        monitor.cancel()
                    }
                }
            }
        )
    }

    public static var testValue: NetworkMonitorClient {
        NetworkMonitorClient(
            start: {
                AsyncStream {
                    $0.yield(.satisfied)
                    $0.finish()
                }
            }
        )
    }
}
