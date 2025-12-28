import Dependencies
import Foundation

public enum SessionEvent: Sendable, Equatable {
    case expired
}

public struct SessionEventsClient: Sendable {
    public var stream: @Sendable () -> AsyncStream<SessionEvent>
    public var send: @Sendable (SessionEvent) async -> Void

    public init(
        stream: @escaping @Sendable () -> AsyncStream<SessionEvent>,
        send: @escaping @Sendable (SessionEvent) async -> Void
    ) {
        self.stream = stream
        self.send = send
    }
}

extension SessionEventsClient: DependencyKey {
    public static let liveValue: SessionEventsClient = {
        let hub = SessionEventsHub()
        return SessionEventsClient(
            stream: {
                AsyncStream { continuation in
                    let id = UUID()
                    Task {
                        await hub.addContinuation(id, continuation)
                    }
                    continuation.onTermination = { _ in
                        Task { await hub.remove(id) }
                    }
                }
            },
            send: { event in
                await hub.send(event)
            }
        )
    }()

    public static let previewValue = SessionEventsClient(
        stream: {
            AsyncStream { $0.finish() }
        },
        send: { _ in }
    )

    public static let testValue = SessionEventsClient(
        stream: {
            AsyncStream { $0.finish() }
        },
        send: { _ in }
    )
}

extension DependencyValues {
    public var sessionEvents: SessionEventsClient {
        get { self[SessionEventsClient.self] }
        set { self[SessionEventsClient.self] = newValue }
    }
}

private actor SessionEventsHub {
    private var continuations: [UUID: AsyncStream<SessionEvent>.Continuation] = [:]
    private var pendingEvents: [SessionEvent] = []

    func addContinuation(
        _ id: UUID,
        _ continuation: AsyncStream<SessionEvent>.Continuation
    ) {
        continuations[id] = continuation
        if !pendingEvents.isEmpty {
            pendingEvents.forEach { continuation.yield($0) }
            pendingEvents.removeAll()
        }
    }

    func send(_ event: SessionEvent) {
        guard !continuations.isEmpty else {
            pendingEvents.append(event)
            return
        }
        continuations.values.forEach { $0.yield(event) }
    }

    func remove(_ id: UUID) {
        continuations[id] = nil
    }
}
