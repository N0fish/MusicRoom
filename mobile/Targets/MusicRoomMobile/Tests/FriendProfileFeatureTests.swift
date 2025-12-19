import ComposableArchitecture
import XCTest

@testable import AppSupportClients
@testable import EventFeature

@MainActor
final class FriendProfileFeatureTests: XCTestCase {
    func testIsMeDetection() async {
        let store = TestStore(
            initialState: FriendProfileFeature.State(userId: "u1", isFriend: false)
        ) {
            FriendProfileFeature()
        } withDependencies: {
            $0.friendsClient.getProfile = { _ in
                PublicUserProfile(
                    userId: "u1",
                    username: "alice",
                    displayName: "Alice",
                    avatarUrl: nil,
                    bio: nil,
                    visibility: "public",
                    preferences: nil
                )
            }
            $0.user.me = {
                UserProfile(
                    id: "1",
                    userId: "u1",
                    username: "alice",
                    displayName: "Alice",
                    avatarUrl: nil,
                    hasCustomAvatar: false,
                    bio: nil,
                    visibility: "public",
                    preferences: UserPreferences(),
                    linkedProviders: [],
                    email: "alice@example.com"
                )
            }
        }

        await store.send(.view(.onAppear)) {
            $0.isLoading = true
        }

        await store.receive(\.profileLoaded.success) {
            $0.isLoading = false
            $0.profile = PublicUserProfile(
                userId: "u1",
                username: "alice",
                displayName: "Alice",
                avatarUrl: nil,
                bio: nil,
                visibility: "public",
                preferences: nil
            )
        }

        await store.receive(\.userLoaded.success) {
            $0.isMe = true
        }
    }

    func testIsNotMeDetection() async {
        let store = TestStore(
            initialState: FriendProfileFeature.State(userId: "u2", isFriend: false)
        ) {
            FriendProfileFeature()
        } withDependencies: {
            $0.friendsClient.getProfile = { _ in
                PublicUserProfile(
                    userId: "u2",
                    username: "bob",
                    displayName: "Bob",
                    avatarUrl: nil,
                    bio: nil,
                    visibility: "public",
                    preferences: nil
                )
            }
            $0.user.me = {
                UserProfile(
                    id: "1",
                    userId: "u1",
                    username: "alice",
                    displayName: "Alice",
                    avatarUrl: nil,
                    hasCustomAvatar: false,
                    bio: nil,
                    visibility: "public",
                    preferences: UserPreferences(),
                    linkedProviders: [],
                    email: "alice@example.com"
                )
            }
        }

        await store.send(.view(.onAppear)) {
            $0.isLoading = true
        }

        await store.receive(\.profileLoaded.success) {
            $0.isLoading = false
            $0.profile = PublicUserProfile(
                userId: "u2",
                username: "bob",
                displayName: "Bob",
                avatarUrl: nil,
                bio: nil,
                visibility: "public",
                preferences: nil
            )
        }

        await store.receive(\.userLoaded.success) {
            $0.isMe = false
        }
    }
}
