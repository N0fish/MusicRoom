import ComposableArchitecture
import XCTest

@testable import AppSupportClients
@testable import EventFeature

@MainActor
final class FriendProfileFeatureTests: XCTestCase {
    func testIsMeDetection() async {
        let profile = PublicUserProfile(
            userId: "u1",
            username: "alice",
            displayName: "Alice",
            avatarUrl: nil,
            isPremium: false,
            bio: nil,
            visibility: "public",
            preferences: nil
        )
        let user = UserProfile(
            id: "1",
            userId: "u1",
            username: "alice",
            displayName: "Alice",
            avatarUrl: nil,
            hasCustomAvatar: false,
            bio: nil,
            visibility: "public",
            preferences: UserPreferences(),
            isPremium: false,
            linkedProviders: [],
            email: "alice@example.com"
        )

        let store = TestStore(
            initialState: FriendProfileFeature.State(userId: "u1", isFriend: false)
        ) {
            FriendProfileFeature()
        } withDependencies: {
            $0.friendsClient.getProfile = { _ in profile }
            $0.user.me = { user }
        }

        await store.send(FriendProfileFeature.Action.view(.onAppear)) {
            $0.isLoading = true
        }

        await store.receive(FriendProfileFeature.Action.profileLoaded(.success(profile))) {
            $0.isLoading = false
            $0.profile = profile
        }

        await store.receive(FriendProfileFeature.Action.userLoaded(.success(user))) {
            $0.isMe = true
        }
    }

    func testIsNotMeDetection() async {
        let profile = PublicUserProfile(
            userId: "u2",
            username: "bob",
            displayName: "Bob",
            avatarUrl: nil,
            isPremium: false,
            bio: nil,
            visibility: "public",
            preferences: nil
        )
        let me = UserProfile(
            id: "1",
            userId: "u1",
            username: "alice",
            displayName: "Alice",
            avatarUrl: nil,
            hasCustomAvatar: false,
            bio: nil,
            visibility: "public",
            preferences: UserPreferences(),
            isPremium: false,
            linkedProviders: [],
            email: "alice@example.com"
        )

        let store = TestStore(
            initialState: FriendProfileFeature.State(userId: "u2", isFriend: false)
        ) {
            FriendProfileFeature()
        } withDependencies: {
            $0.friendsClient.getProfile = { _ in profile }
            $0.user.me = { me }
        }

        await store.send(FriendProfileFeature.Action.view(.onAppear)) {
            $0.isLoading = true
        }

        await store.receive(FriendProfileFeature.Action.profileLoaded(.success(profile))) {
            $0.isLoading = false
            $0.profile = profile
        }

        await store.receive(FriendProfileFeature.Action.userLoaded(.success(me)))
    }
}
