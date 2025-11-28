# MOBILE ROADMAP · MOBILE REMOTE CONTROL APP

## Long-Term Roadmap (As Of 2025-11-27)
1. **Phase 1 – Platform & Modularization** (COMPLETED)
   - Restructure Tuist project into modular targets (App, Features, Domain, Infrastructure, Tests).
   - Wire base settings screen that edits backend URL, add SwiftLint/SwiftFormat, and ensure CI can build/test headlessly.
2. **Phase 2 – Infrastructure & Observability**
   - Implement `APIClient` actor (REST/WebSocket, auth refresh, retries) and PolicyEngine actor for license checks.
   - Add telemetry pipeline (backend audit logs + os.Logger), crash/MetricKit hooks, and diagnostics UI; cover with unit tests.
3. **Phase 3 – Authentication & Identity**
   - Ship onboarding (email/password, OAuth, verification, reset) plus backend URL chooser and rate-limit feedback.
   - Build profile editor covering public/friends/private/preferences scopes with SwiftData + Keychain storage.
4. **Phase 4 – Event Discovery & Track Vote**
   - Event browser for public/private/invite events, license UX, realtime playlist stream, and optimistic voting with contention handling.
   - Provide reducer + snapshot tests validating vote ordering, geo/time locks, and streaming recovery.
5. **Phase 5 – Control Delegation & Playlist Editor**
   - Device delegation dashboard with live reassignment respecting licensing tiers and conflict prompts.
   - Collaborative playlist editor with optimistic edits, conflict banners, visibility toggles, and offline queue replay.
6. **Phase 6 – Ramp-up, QA, and Bonus Hooks**
   - Automate load-test hooks, document supported concurrency, harden session/device revocation, finish CI/CD (DocC, UI smoke tests).
   - Stage optional bonuses (multi-platform, IoT beacons, subscriptions, offline enhancements) only after mandatory scope is green.

## Immediate Next Steps
- **Phase 2 Kickoff:** Implement `APIClient` actor in `MusicRoomAPI` target.
- **Telemetry:** Design and implement the telemetry pipeline for audit logs.
