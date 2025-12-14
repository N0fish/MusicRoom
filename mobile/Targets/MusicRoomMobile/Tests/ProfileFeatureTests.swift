import ComposableArchitecture
import XCTest

@testable import AppFeature
@testable import AppSupportClients

@MainActor
final class ProfileFeatureTests: XCTestCase {
    func testOnAppear_LoadsProfile() async {
        let profile = UserProfile(
            id: "1",
            userId: "user1",
            username: "testuser",
            displayName: "Test User",
            avatarUrl: "http://example.com/avatar.jpg",
            hasCustomAvatar: false,
            email: "test@example.com",
            preferences: ["genres": "Rock"]
        )

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.user.me = { profile }
        }

        await store.send(.onAppear) {
            $0.isLoading = true
        }

        await store.receive(\.profileResponse.success) {
            $0.isLoading = false
            $0.userProfile = profile
            $0.editableDisplayName = "Test User"
            $0.editableUsername = "testuser"
            $0.editableEmail = "test@example.com"
            $0.editableMusicPreferences = "Rock"
        }
    }

    func testEditAndSave_UpdatesProfile() async {
        let initialProfile = UserProfile(
            id: "1",
            userId: "user1",
            username: "old",
            displayName: "Old Name",
            avatarUrl: "http://example.com/avatar.jpg",
            hasCustomAvatar: false,
            email: "old@example.com",
            preferences: ["genres": "Old"]
        )

        // Final profile after update
        let updatedProfile = UserProfile(
            id: "1",
            userId: "user1",
            username: "new",
            displayName: "New Name",
            avatarUrl: "http://example.com/avatar.jpg",
            hasCustomAvatar: false,
            email: "new@example.com",
            preferences: ["genres": "New"]
        )

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.user.me = { initialProfile }
            $0.user.updateProfile = { _ in updatedProfile }
        }

        // Load initial
        await store.send(.onAppear) {
            $0.isLoading = true
        }
        await store.receive(\.profileResponse.success) {
            $0.isLoading = false
            $0.userProfile = initialProfile
            $0.editableDisplayName = "Old Name"
            $0.editableUsername = "old"
            $0.editableEmail = "old@example.com"
            $0.editableMusicPreferences = "Old"
        }

        // Toggle Edit
        await store.send(.toggleEditMode) {
            $0.isEditing = true
        }

        // Edit fields
        await store.send(.binding(.set(\.editableDisplayName, "New Name"))) {
            $0.editableDisplayName = "New Name"
        }
        await store.send(.binding(.set(\.editableUsername, "new"))) {
            $0.editableUsername = "new"
        }
        await store.send(.binding(.set(\.editableEmail, "new@example.com"))) {
            $0.editableEmail = "new@example.com"
        }
        await store.send(.binding(.set(\.editableMusicPreferences, "New"))) {
            $0.editableMusicPreferences = "New"
        }

        // Save
        await store.send(.saveButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(\.updateProfileResponse.success) {
            $0.isLoading = false
            $0.isEditing = false
            $0.userProfile = updatedProfile
            $0.errorMessage = nil
            // Note: editable fields stay as they are (updated values)
        }
    }

    func testLogout() async {
        let logoutCalled = LockIsolated(false)

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.authentication.logout = { logoutCalled.setValue(true) }
        }

        await store.send(.logoutButtonTapped)

        XCTAssertTrue(logoutCalled.value)
    }

    func testLinkAccount_Success() async {

        let profileLinked = UserProfile(
            id: "1", userId: "user1", username: "u", displayName: "d", avatarUrl: "",
            hasCustomAvatar: false, linkedProviders: ["google"], email: nil, preferences: [:]
        )
        _ = profileLinked  // Suppress unused warning if closure is ignored

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.webAuthenticationSession.authenticate = { _, _ in
                URL(string: "musicroom://auth?accessToken=mockAccess&refreshToken=mockRefresh")!
            }
            $0.user.link = { provider, token in
                XCTAssertEqual(provider, "google")
                XCTAssertEqual(token, "mockAccess")
                return profileLinked
            }
        }

        // store.exhaustivity = .off

        await store.send(
            ProfileFeature.Action.linkAccount(
                AuthenticationClient.SocialHelper.SocialProvider.google)
        ) {
            $0.isLoading = true
        }

        await store.receive(\.linkAccountResponse.success) {
            $0.isLoading = false
            $0.userProfile = profileLinked
            $0.errorMessage = nil
        }
    }

    func testUnlinkAccount_Success() async {

        let profileUnlinked = UserProfile(
            id: "1", userId: "user1", username: "u", displayName: "d", avatarUrl: "",
            hasCustomAvatar: false, linkedProviders: [], email: nil, preferences: [:]
        )

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.user.unlink = { provider in
                XCTAssertEqual(provider, "google")
                return profileUnlinked
            }
        }

        await store.send(
            ProfileFeature.Action.unlinkAccount(
                AuthenticationClient.SocialHelper.SocialProvider.google)
        ) {
            $0.isLoading = true
        }

        await store.receive(\.linkAccountResponse.success) {
            $0.isLoading = false
            $0.userProfile = profileUnlinked
            $0.errorMessage = nil
        }
    }
}
