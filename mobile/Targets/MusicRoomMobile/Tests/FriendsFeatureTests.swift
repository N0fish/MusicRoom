import ComposableArchitecture
import XCTest

@testable import AppFeature
@testable import AppSettingsClient
@testable import AppSupportClients

@MainActor
final class FriendsFeatureTests: XCTestCase {
    func testSegmentChangedToRequestsTriggersLoadWhenEmpty() async {
        let me = UserProfile(
            id: "1",
            userId: "u1",
            username: "alice",
            displayName: "Alice",
            avatarUrl: nil,
            hasCustomAvatar: false
        )

        let store = TestStore(initialState: FriendsFeature.State()) {
            FriendsFeature()
        } withDependencies: {
            $0.appSettings = .testValue
            $0.friendsClient = .testValue
            $0.friendsClient.listFriends = { [] }
            $0.friendsClient.incomingRequests = { [] }
            $0.user.me = { me }
        }
        store.exhaustivity = .off

        await store.send(.segmentChanged(.requests)) {
            $0.selectedSegment = .requests
        }

        await store.receive(.loadData) {
            $0.isLoading = true
            $0.errorMessage = nil
        }

        await store.finish()
    }

    func testClearsSearchResultsWhenSearchQueryEmptied() async {
        let friend = Friend(
            id: "1",
            userId: "u1",
            username: "alice",
            displayName: "Alice",
            avatarUrl: nil,
            isPremium: false
        )

        var state = FriendsFeature.State()
        state.searchQuery = "alice"
        state.searchResults = [friend]

        let store = TestStore(initialState: state) {
            FriendsFeature()
        }

        await store.send(.binding(.set(\.searchQuery, ""))) {
            $0.searchQuery = ""
            $0.searchResults = []
        }
    }
}
