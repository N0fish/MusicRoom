# Mobile Application – Final State Expectations

This document consolidates everything the mobile team must deliver for the Music Room project once development is considered complete. It captures both mandatory requirements and bonus expectations from `Agents/subject.txt`, and it focuses solely on the mobile experience.

## 1. Scope & Architecture Alignment
- The mobile client acts strictly as a **remote control** for the back-end. All authoritative data, playlist states, and service rules stay on the server.
- The **back-end base URL must be configurable** within the application so testers can point to staging or local servers without rebuilding the app.
- The mobile implementation may target Android or iOS (or both), but it must consume the shared API (REST + JSON or the approved alternative) exactly as documented.
- Any external SDK (social login, IoT, analytics, etc.) must complement—not replace—core work. Dependencies must be fetched automatically via the existing tooling.

## 2. Account Creation & Identity Management
- **First-run onboarding** forces the user to create an account using either email/password or OAuth via Facebook/Google.
- After onboarding, a user can **link/unlink social identities** to an existing account.
- Email-based accounts require **email verification** and a **password reset flow**.
- The profile screen manages four editable data scopes: public information, friends-only information, private information, and music preferences.

## 3. Core Service Coverage
The production app exposes every mandatory service; at minimum two must be fully operable from mobile.

### 3.1 Music Track Vote
- Users discover or create events (public by default) and can toggle visibility to private invitations.
- License tiers drive who can vote (everyone, invited-only, or geo/time-bound participants such as “in-venue between 16:00–18:00”).
- Voting updates reorder the current playlist in real time, with proper contention handling when multiple users vote simultaneously.

### 3.2 Music Control Delegation
- Owners can delegate playback control per device registered on their account.
- Licensing rules enforce who can take control on a given device, and delegation can be reassigned live among friends.

### 3.3 Music Playlist Editor
- Enables collaborative, real-time playlist editing with optimistic UI and conflict awareness when users move or edit the same tracks.
- Visibility (public/private) and license rules mirror the Track Vote service: anyone vs. invited-only editors.

## 4. Security, Logging & Observability
- Authenticated users only access their own data; the UI honors authorization responses from the API and never exposes other users’ private details.
- The app surfaces protective measures (e.g., rate-limit feedback, forced re-authentication, device revocation) aligned with the back-end’s anti-bruteforce and session-hardening strategies.
- **Every user action triggers a log entry on the server** that includes platform (Android, iOS, etc.), device model, and application version. The mobile client must therefore send this metadata with each request or logging call.
- Secrets (API keys, OAuth configs) live in environment files ignored by Git; the repository stays credential-free.

## 5. Ramp-up & Quality Expectations
- Ship instrumentation or toggles needed for the back-end team to run load tests against all three services, and document any mobile-side constraints that influence maximum user counts.
- Maintain agility by pairing each feature with targeted unit/UI tests and integrating mobile checks into the shared CI pipeline.
- Provide or contribute to the shared API reference (Swagger/OpenAPI, etc.) so mobile behavior stays aligned with server contracts.

## 6. Bonus Deliverables (Evaluated Only if Mandatory Scope Is Perfect)
- **Multi-platform support:** Extend the remote-control experience beyond the primary mobile target, e.g., responsive web or the complementary mobile platform, ensuring feature parity for the three services.
- **IoT integration:** Implement an iBeacon (or equivalent) flow where approaching a registered event automatically surfaces contextual information (location, schedule, music style) inside the app.
- **Subscription tiers:** Offer at least two plans (free vs. paid) with clear upgrade/downgrade paths. Restrict premium-only features such as collaborative playlist editing behind the paid tier.
- **Offline mode:** Define an offline experience (e.g., browsing cached playlists, queuing votes) and a robust synchronization strategy that resolves conflicts and purges stale data once connectivity returns.

Delivering the above ensures the mobile application matches the expected final state for Music Room, covering every mandatory requirement and making the bonus features attainable.
