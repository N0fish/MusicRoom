# BACKEND CHECKLIST - MUSIC ROOM

Based on `Agents/subject.txt` (Version 5.3).

## I. Architecture & General Instructions
- [ ] **Language/Strategy**: Justified choice of technology (Node, Go, etc.).
- [ ] **Dependencies**: Managed via `Makefile` or similar (e.g., `docker-compose`, `package.json`). No committed `node_modules`.
- [ ] **Data Storage**: Database implemented (SQL/NoSQL).
- [ ] **Environment**: Credentials/Keys in `.env` (not committed).
- [ ] **Logging**: Mobile actions logged on backend (Platform, Device, Version).

## II. API
- [ ] **Documentation**: Swagger/OpenAPI available. Introduces methods, inputs, outputs.
- [ ] **Structure**: RESTful or justified alternative (e.g., GraphQL).
- [ ] **Format**: JSON or justified alternative.
- [ ] **Access**: API is the single entry point for all clients.

## III. Authentication & Security
- [ ] **Registration**: Email/Password OR Social (FB/Google).
- [ ] **Login**: Email/Password OR Social (FB/Google).
- [ ] **Account Linking**: Link Social account to existing account.
- [ ] **Email Validation**: Required for Email/Password accounts.
- [ ] **Password Reset**: "Forgot password" flow.
- [ ] **Privacy/Security**:
    - [ ] User cannot access others' private data.
    - [ ] Protection against brute-force/session theft.

## IV. User Profile
- [ ] **Fields**:
    - [ ] Public info.
    - [ ] Friends-only info.
    - [ ] Private info.
    - [ ] Music preferences.
- [ ] **Updates**: User can update all above fields.

## V. Functional Services (At least 2 of 3)

### 1. Music Track Vote (Live Music Chain)
- [ ] **Core**: Suggest tracks, Vote for tracks.
- [ ] **Ranking**: Tracks with more votes played earlier.
- [ ] **Visibility Management**:
    - [ ] Public (default): Everyone finds/votes.
    - [ ] Private: Only invited can find/vote.
- [ ] **License Management**:
    - [ ] Default: Everyone votes.
    - [ ] Restricted: Only invited vote.
    - [ ] Geo/Time: Voting only in specific place/time.
- [ ] **Concurrency**: Logic for handling simultaneous votes/suggestions.

### 2. Music Playlist Editor (Collaborative)
- [ ] **Core**: Real-time collaborative editing.
- [ ] **Visibility Management**:
    - [ ] Public (default): Everyone accesses.
    - [ ] Private: Only invited access.
- [ ] **License Management**:
    - [ ] Default: Everyone edits.
    - [ ] Restricted: Only invited edit.
- [ ] **Concurrency**: Handle simultaneous moves/edits.

### 3. Music Control Delegation
- [ ] **License**: Specific per device.
- [ ] **Delegation**: Give control to friends.

## VI. Ramp-up & Quality
- [ ] **Load Testing**: Evidence of load testing (AB, Gatling, etc.).
- [ ] **Capacity**: Justified max users (CPU/RAM specs).
- [ ] **Tests**: Unit/Integration tests for each layer.
- [ ] **CI/CD**: Agility and continuous integration practices.
