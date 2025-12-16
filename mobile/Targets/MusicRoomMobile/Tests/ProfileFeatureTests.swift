import AppSettingsClient
import ComposableArchitecture
import XCTest

@testable import AppFeature
@testable import AppSupportClients

@MainActor
final class ProfileFeatureTests: XCTestCase {
    // ... existing tests ...

    func testOnAppear_LoadsProfile() async {
        let profile = UserProfile(
            id: "1",
            userId: "user1",
            username: "testuser",
            displayName: "Test User",
            avatarUrl: "http://example.com/avatar.jpg",
            hasCustomAvatar: false,
            preferences: UserPreferences(genres: ["Rock"]),
            linkedProviders: [],
            email: "test@example.com"
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
            preferences: UserPreferences(genres: ["Old"]),
            linkedProviders: [],
            email: "old@example.com"
        )

        // Final profile after update
        let updatedProfile = UserProfile(
            id: "1",
            userId: "user1",
            username: "new",
            displayName: "New Name",
            avatarUrl: "http://example.com/avatar.jpg",
            hasCustomAvatar: false,
            preferences: UserPreferences(genres: ["New"]),
            linkedProviders: [],
            email: "new@example.com"
        )

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.user.me = { initialProfile }
            $0.user.updateProfile = { _ in updatedProfile }
            $0.telemetry.log = { action, _ in
                XCTAssertEqual(action, "user.profile.update.success")
            }
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
            hasCustomAvatar: false, preferences: UserPreferences(), linkedProviders: ["google"],
            email: nil
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
            hasCustomAvatar: false, preferences: UserPreferences(), linkedProviders: [], email: nil
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

    func testChangePassword_Success() async {
        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.user.changePassword = { current, new in
                XCTAssertEqual(current, "oldPass")
                XCTAssertEqual(new, "newPass")
            }
        }

        await store.send(.toggleChangePasswordMode) {
            $0.isChangingPassword = true
        }

        await store.send(.binding(.set(\.currentPassword, "oldPass"))) {
            $0.currentPassword = "oldPass"
        }
        await store.send(.binding(.set(\.newPassword, "newPass"))) {
            $0.newPassword = "newPass"
        }
        await store.send(.binding(.set(\.confirmNewPassword, "newPass"))) {
            $0.confirmNewPassword = "newPass"
        }

        await store.send(.changePasswordButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(\.changePasswordResponse.success) {
            $0.isLoading = false
            $0.isChangingPassword = false
            $0.passwordChangeSuccessMessage = "Password changed successfully."
            $0.currentPassword = ""
            $0.newPassword = ""
            $0.confirmNewPassword = ""
        }
    }

    func testChangePassword_ValidationMismatch() async {
        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        }

        await store.send(.toggleChangePasswordMode) {
            $0.isChangingPassword = true
        }

        await store.send(.binding(.set(\.currentPassword, "oldPass"))) {
            $0.currentPassword = "oldPass"
        }
        await store.send(.binding(.set(\.newPassword, "new1"))) {
            $0.newPassword = "new1"
        }
        await store.send(.binding(.set(\.confirmNewPassword, "new2"))) {
            $0.confirmNewPassword = "new2"
        }

        await store.send(.changePasswordButtonTapped) {
            $0.errorMessage = "New passwords do not match."
        }
    }
    func testRandomizeAvatar() async {
        let backendURL = URL(string: "http://test.backend")!

        // The random avatar returns a relative URL
        let randomProfile = UserProfile(
            id: "1", userId: "u1", username: "u", displayName: "d",
            avatarUrl: "/avatars/random.svg",
            hasCustomAvatar: false, preferences: UserPreferences(), linkedProviders: [], email: nil
        )

        // Expected normalized profile
        let normalizedProfile = UserProfile(
            id: "1", userId: "u1", username: "u", displayName: "d",
            avatarUrl: "http://test.backend/avatars/random.svg",
            hasCustomAvatar: false, preferences: UserPreferences(), linkedProviders: [], email: nil
        )

        let store = TestStore(initialState: ProfileFeature.State()) {
            ProfileFeature()
        } withDependencies: {
            $0.user.generateRandomAvatar = { randomProfile }
            $0.appSettings.load = { AppSettings(backendURL: backendURL) }
        }

        await store.send(.generateRandomAvatarTapped) {
            $0.isAvatarLoading = true
        }

        await store.receive(.generateRandomAvatarResponse(.success(randomProfile))) {
            $0.isAvatarLoading = false
            $0.userProfile = normalizedProfile
        }
    }
}
