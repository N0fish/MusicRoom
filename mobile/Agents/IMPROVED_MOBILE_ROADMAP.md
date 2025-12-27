# IMPROVED MOBILE ROADMAP (2025-12-14)

## Current Status: ALPHA / PARTIAL PHASE 2
The project has a solid architecture (Tuist + TCA) but is missing critical features required for defense.

## Phase 1: Critical Fixes (Blocking Delivery)
*Goal: Make the app runnable and testable by a peer evaluator.*

- [ ] **1.1. Configurable Backend URL (BLOCKER)**
    - [ ] `SettingsFeature`: Ensure URL is saved to `Role.userDefaults/Keychain`.
    - [ ] `MusicRoomAPIClient`: Remove hardcoded `http://localhost:8080`.
    - [ ] `MusicRoomAPIClient`: Inject `AppEnvironment/Settings` dependency to read the dynamic URL at runtime.
    - [ ] **Verification**: Launch app -> Change URL in Settings -> API requests go to new URL.

- [ ] **1.2. Social Authentication**
    - [ ] Add `GoogleSignIn` / `AuthenticationServices` (Apple) dependencies.
    - [ ] Implement `AuthenticationFeature` logic for OAuth tokens.
    - [ ] Bind "Sign in with Google" button to `auth-service` OAuth endpoints.

- [ ] **1.3. Mandatory Logging**
    - [ ] Create `interceptor` or `middleware` in `APIClient`.
    - [ ] Attach headers: `X-Platform: iOS`, `X-Device: ...`, `X-App-Version: ...`.
    - [ ] **Verification**: Backend logs show device info for every request.

## Phase 2: Core Services Implementation
*Goal: Implement the "2 out of 3" mandatory services.*

- [ ] **2.1. Events & Voting (Music Track Vote)**
    - [ ] Connect `EventListFeature` to real `/events` endpoint (replace MockData).
    - [ ] Implement "Create Event" screen with visibility/license options.
    - [ ] Implement "Vote" action (POST `/events/{id}/vote`).
    - [ ] Handle Geo-Location permission for `geo_time` license.

- [ ] **2.2. Playlist Editor (Real-time)**
    - [ ] Implement WebSocket client in `MusicRoomAPIClient`.
    - [ ] Handle `playlist.updated` events.
    - [ ] UI for "Add Track" / "Remove Track".

## Phase 3: Polish & Security
- [ ] **3.1. Error Handling**: Show user-friendly alerts for 401/403 errors (e.g., "Subscription expired" or "Outside allowed area").
- [ ] **3.2. Token Refresh**: Implement transparent 401 interception -> Refresh Token -> Retry Request flow.

## Phase 4: Testing & Bonuses
- [ ] **4.1. Unit Tests**: Write tests for `AuthenticationFeature` reducers.
- [ ] **4.2. UI Tests**: Snapshot testing for main screens.
- [ ] **4.3. Bonuses**: Offline mode, iPad support.
