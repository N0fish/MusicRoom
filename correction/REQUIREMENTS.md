# Requirements Checklist

This checklist is derived from `Agents/subject.txt` (Version 5.3) and the grading scale `Agents/Intra Projects music-room Edit.txt`. It outlines the mandatory obligations for the Backend, Server, and Mobile components with extreme detail.

## Backend Obligations (Logic & Services)

### 1. User Management & Authentication (IV.1)
- [ ] **Account Creation**: Support registration via Email/Password.
- [ ] **Social Registration**: Support registration via Social Network (Facebook or Google).
- [ ] **Account Linking**: Allow linking a Social Network account (Facebook/Google) to an existing account.
- [ ] **Profile Management**:
    - [ ] Manage Public Information.
    - [ ] Manage Friend-only Information.
    - [ ] Manage Private Information.
    - [ ] Manage Music Preferences.
    - [ ] **[Grading Check]** User must be able to update: Personal info, Preferences/Settings, Friends, Playlists, Devices, and Permissions.
- [ ] **Security Flows**:
    - [ ] Email validation for Email/Password accounts.
    - [ ] Password reset flow (for forgotten passwords).

### 2. Core Music Services (IV.2)
*Must implement at least 2 of the following 3 services. Grading requires validation of implemented services.*

#### A. Music Track Vote (IV.2.1)
- [ ] **Live Playlist**: Maintain a live queue/chain of music tracks.
- [ ] **Voting System**: Implement voting logic where tracks with more votes move up the queue.
    - [ ] **[Grading Check]** Tracks receiving most votes must be played in order of vote count.
- [ ] **Visibility Management**:
    - [ ] **Public**: Visible and votable by everyone.
    - [ ] **Private**: Visible and votable only by invited guests.
- [ ] **License/Permission Management**:
    - [ ] "Everyone" can vote.
    - [ ] "Guests only" can vote.
    - [ ] "Location/Time specific" (e.g., GPS radius or time window required).
- [ ] **Concurrency**: Handle concurrent votes and playlist reordering correctly.

#### B. Music Control Delegation (IV.2.2)
- [ ] **Device Linking**: Link several devices to a single user account.
- [ ] **Delegation Logic**: Allow a user to delegate music control to different friends (users).

#### C. Music Playlist Editor (IV.2.3)
- [ ] **Real-time Collaboration**: Support real-time multi-user playlist editing.
    - [ ] **[Grading Check]** Playlist can be played *while* it is being modified.
- [ ] **Visibility Management**:
    - [ ] **Public**: Accessible by everyone.
    - [ ] **Private**: Accessible only by invited guests.
- [ ] **Permission Management**:
    - [ ] "Everyone" can edit.
    - [ ] "Invited only" (Guests) can edit.
- [ ] **Concurrency**: Handle concurrent edits without conflicts.

### 3. API (IV.4)
- [ ] **Central Access Point**: API is the *only* exposed access point for clients to communicate with the server.
- [ ] **Documentation**:
    - [ ] **[Grading Check]** Documentation must be available at a defined URL.
    - [ ] Must cover *all* functionalities used by the mobile application.
- [ ] **Consistency**:
    - [ ] **[Grading Check]** API functionalities must follow a common logic in naming and structure.
    - [ ] API must be designed as a coherent whole (simplicity, consistency, clarity).
- [ ] **Architecture**: Follow REST principles (or valid alternative like GraphQL if justified).
- [ ] **Format**: Use JSON for data exchange (or valid alternative).

### 4. Security & Logging (IV.6)
- [ ] **Data Isolation**: Ensure authenticated users can only access their own or authorized data.
- [ ] **Storage Security**:
    - [ ] **[Grading Check]** Storage system (DB) must *only* be accessible from the server (and admin tools), never directly exposed.
- [ ] **Input Control**: Set up information control mechanisms for data sent to the API.
- [ ] **Attack Protection**: Implement mechanisms against bruteforce and session theft.
- [ ] **Activity Logging**: Log *all* actions on the mobile app with Platform, Device, and App Version.

---

## Server Obligations (Infrastructure & Operations)

### 1. Architecture & Infrastructure (III.1, IV.3)
- [ ] **Technology Choice**: Justify choice (e.g., Go, Node.js) based on effectiveness and scalability.
- [ ] **Scalability**:
    - [ ] **[Grading Check]** Selected storage solution must be capable of increasing in capacity.
    - [ ] **[Grading Check]** Selected server technology must be capable of increasing in capacity.
- [ ] **Dependency Management**: Use standard package managers (`go.mod`) and ensure all dependencies are downloadable via `Makefile`.

### 2. Performance & Ramp-up (IV.7)
- [ ] **Capacity Measurement**:
    - [ ] **[Grading Check]** You must *measure* the ramp-up capacity according to allocated physical resources.
    - [ ] **[Grading Check]** You must use a measuring tool (AB, Gatling, etc.) to simulate simultaneous users.
- [ ] **Load Management**: Define supported maximum users.

### 3. Deployment & Config (IV.8)
- [ ] **Environment Security**: Usage of `.env` file for ALL credentials/keys.
    - [ ] **[Automatic Fail]** Any credential found in git or outside `.env` results in grade 0.
- [ ] **Setup Scripts**: `Makefile` or similar must set up the solution successfully.
    - [ ] **[Automatic Fail]** If solution doesn't start or code error prevents startup, grade is 0.

---

## Team & Quality (Methodology)

- [ ] **Role Definition**:
    - [ ] Coordinator/Supervisor.
    - [ ] Backend Referent.
    - [ ] API Referent.
    - [ ] Mobile Experience Referent.
    - [ ] Security Referent.
- [ ] **Testing Strategy**:
    - [ ] **[Grading Check]** Consistent one-off tests created from the start to limit regressions.
    - [ ] **[Grading Check]** Specific tests for *each layer*: Server, API, Application, Services.
- [ ] **Methodology**: Documented process for questioning decisions and priorities.

---

## Mobile Application (Grading Specifics)

- [ ] **Technology**: Must be Native (Swift/Kotlin) or Compiled Native (Xamarin). **No Hybrid/Web-wrappers** (Cordova, Ionic, PhoneGap).
- [ ] **UX Quality**: Interface decisions must be questioned and tested with users for feedback.
- [ ] **SDK Integration**:
    - [ ] Social SDKs (Facebook/Google) for auth/linking.
    - [ ] External Music SDKs/APIs (Deezer, Spotify, etc.) for enhanced music experience.

---

## Bonus Capabilities (V & Grading)

- [ ] **Multi-platform**: Web responsive support.
- [ ] **IoT**: iBeacon or similar mechanism for event discovery.
- [ ] **Subscription Model**: Free vs Premium.
- [ ] **Offline Mode**: Offline usage with synchronization.
- [ ] **Additional OAuth**: More providers beyond mandatory.
- [ ] **Statistics**: User listening stats or event stats.
