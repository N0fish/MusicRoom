# MOBILE CHECKLIST (iOS) - MUSIC ROOM

Based on `Agents/subject.txt` (Version 5.3).

## I. General & Technical Requirements
- [ ] **Configurable Backend URL**: The application MUST allow changing the backend IP/Base URL. This is critical for evaluation (e.g., a settings screen on launch or inside specific developer settings).
- [ ] **Remote Control Philosophy**: The app behaves as a client ("remote control") for the backend. No heavy local business logic that contradicts the server "truth".
- [ ] **Platform**: Native iOS application (Swift/SwiftUI/Obj-C).
- [ ] **Dependencies**: Managed via standard tools (CocoaPods, Carthage, Swift Package Manager). No unmanaged binaries committed.

## II. Authentication (User)
- [ ] **First Launch**: User must create an account.
- [ ] **Registration Methods**:
    - [ ] Email / Password.
    - [ ] Social Network (Google OR Facebook).
- [ ] **Login Methods**:
    - [ ] Email / Password.
    - [ ] Social Network (Google OR Facebook).
- [ ] **Link Account**: Ability to link Social Account (FB/Google) to an existing Email account.
- [ ] **Password Management**:
    - [ ] "Forgot Password" flow (trigger backend email).
    - [ ] Change password.

## III. User Profile
- [ ] **View Profile**: Display user details.
- [ ] **Edit Profile**:
    - [ ] Public Information (Avatar, Nickname).
    - [ ] Private Information (Email, Real Name).
    - [ ] Music Preferences.
- [ ] **Privacy UI**: Visual indication of what is Public / Friends-Only / Private.
- [ ] **Friends System**: UI to add/remove friends (if implemented in backend).

## IV. Functional Services (Core UX)
*App must implement UI for at least 2 of the 3 services.*

### 1. Music Track Vote (Live Event)
- [ ] **Event Creation**: UI to create an event with Visibility (Public/Private) and License (Geo/Time/Invited) settings.
- [ ] **Event Discovery**: List of available events.
    - [ ] Handle "Public" events.
    - [ ] Handle "Invites" logic (view private events if invited).
- [ ] **Voting Interface**:
    - [ ] Suggest a track.
    - [ ] Vote for a track (Upvote/Downvote).
    - [ ] Visual feedback for errors (e.g., "Outside Geo Radius", "Vote Ended").
- [ ] **Geo-Location**: App must ask for Location Permissions to send coords for Geo-License voting.

### 2. Music Playlist Editor
- [ ] **Collaboration UI**: Real-time updates of the playlist (WebSocket integration).
- [ ] **Editing**: Add / Remove / Reorder tracks.
- [ ] **Invite UI**: Invite friends to collaborate on a private playlist.

### 3. Music Control Delegation
- [ ] **Delegation UI**: Select a friend to "control" the music on this device.
- [ ] **Remote Player**: UI to control playback (Play/Pause/Skip) based on delegation rights.

## V. Security & Logging
- [ ] **Logging**: All actions must generate logs sent to the backend.
    - [ ] Payload must include: `Platform` (iOS), `Device` (e.g., iPhone 14), `App Version`.
- [ ] **Session Security**: Handle token expiration/refresh transparently (or redirect to login).
- [ ] **Data Protection**: User cannot see data they are not authorized to see (UI handles 403/404 gracefully).

## VI. Bonus (Optional)
- [ ] **Multi-platform support**: (e.g., iPad layout or MacOS Catalyst).
- [ ] **IoT / IBeacon**: Auto-discovery of events when physically near.
- [ ] **Free vs Paid**: UI to switch subscription plans.
- [ ] **Offline Mode**:
    - [ ] App works without internet.
    - [ ] Sync mechanism when connection returns.
