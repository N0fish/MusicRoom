import Foundation
import Dependencies
import MusicRoomDomain

public struct PolicyEngineClient: Sendable {
    public var evaluate: @Sendable (_ event: Event) async -> PolicyDecision

    public init(evaluate: @escaping @Sendable (_ event: Event) async -> PolicyDecision) {
        self.evaluate = evaluate
    }
}

extension PolicyEngineClient: DependencyKey {
    public static let liveValue = PolicyEngineClient { event in
        switch event.licenseTier {
        case .everyone:
            return PolicyDecision(isAllowed: true, reason: "Open to everyone")
        case .invitedOnly:
            return PolicyDecision(isAllowed: false, reason: "Requires an invitation")
        case .geoLocked:
            return PolicyDecision(isAllowed: false, reason: "Geo-locked for on-site participants")
        }
    }

    public static let previewValue = PolicyEngineClient { _ in
        PolicyDecision(isAllowed: true, reason: "Preview policy allows access")
    }

    public static let testValue = PolicyEngineClient { _ in
        PolicyDecision(isAllowed: true, reason: "")
    }
}

extension DependencyValues {
    public var policyEngine: PolicyEngineClient {
        get { self[PolicyEngineClient.self] }
        set { self[PolicyEngineClient.self] = newValue }
    }
}
