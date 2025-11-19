# MOBILE TECHNOLOGY STACK · iOS 26 / Swift 6.2

## 1. Platform Targets & Toolchain
- **Primary OS:** iOS 26 only. Dropping legacy OSes lets us rely on Observation, SwiftData cloud sync, App Intents, and MetricKit streaming without guard rails.
- **Tooling:** Xcode 26.x + Swift 6.2 compiler. Enable “Approachable Concurrency” warnings now, then flip to full `-strict-concurrency=complete` before beta to satisfy Swift 6 isolation guarantees.
- **Project Layout:** Tuist (preferred) or XcodeGen generates a modular workspace (App target + feature frameworks). Swift Package Manager hosts reusable feature modules, shared domain, and infrastructure packages.

## 2. Language & Concurrency Settings
- **Structured Concurrency Everywhere:** `async/await`, `TaskGroup`, `AsyncSequence`, and `actors` power every network, persistence, and realtime flow. No GCD/Combine unless wrapping legacy SDKs.
- **Actor Isolation Defaults:** UI-bound types adopt `@MainActor`; long-lived services (API client, cache, policy engine) are `actor`s. When bridging older code, use `@preconcurrency` and `nonisolated(unsafe)` sparingly with documentation.
- **Sendable Hygiene:** All closures escaping feature reducers must be `@Sendable`. Build fails if types violate `Sendable`, preventing the heisenbugs Swift 5 tolerated.
- **Global Settings:** `SWIFT_STRICT_CONCURRENCY = complete` and `SWIFT_SERIALIZE_SYMBOL_GRAPH = YES` (so DocC stays accurate). Apply “Implicitly open actor classes” only when integrating 3rd-party frameworks.

## 3. Architectural Pillars
- **Remote-Control Posture:** Mobile never owns truth—every playlist edit, vote, or delegation command round-trips through the backend’s policy checks.
- **Feature Modularity:** Capabilities (Auth, Event Discovery, Voting, Playlist Editor, Delegation, Profile, Settings) each expose `State`, `Action`, `Reducer`, and `View` modules via Swift packages to keep compile times low.
- **The Composable Architecture (TCA) 1.23+:** Primary state-management pattern. We use `@ObservableState`, `@Dependency`, `StackState`, and `NavigationStackStore` for navigation, plus deterministic `TestStore` coverage.
- **Clean Layering:** Features depend on shared `Domain` (entities/use cases) and `Infrastructure` (API client, persistence, analytics). No feature imports another feature directly; coordination happens via parent reducers.
- **Tuist-Driven Modularity:** All app, feature, domain, and infra targets are declared in Tuist manifests so new modules/preview apps can be generated consistently and reused across environments.

## 4. State, Navigation & Configuration
- **Observation-First Views:** SwiftUI views consume `@Bindable` feature state. Global models (session, environment overrides, analytics consent) live in the environment via Observation.
- **Navigation:** Each flow owns a `NavigationStackStore`. Optional child reducers automatically deallocate state when dismissed, keeping memory stable during long sessions.
- **Configurable Backend:** Settings feature exposes editable backend URL, feature flags, and logging verbosity. Values persist in SwiftData, surface in debug UI, and sync to Keychain for secure storage.

## 5. Networking & Realtime
- **API Client Actor:** Thin `APIClient` actor wraps `URLSession` (with HTTP/2 multiplexing, `URLSessionConfiguration.metricKitReporting = true`). It injects device metadata headers (platform, device model, app version, license tier) for every request per subject requirements.
- **Error Normalization:** Server responses map into domain enums (`AuthError`, `LicenseError`, `ThrottleError`) so reducers can present consistent UI.
- **Realtime:** `PlaylistStreamActor` owns `URLSessionWebSocketTask`, publishes `AsyncThrowingStream<ServerEvent>`, and applies resume tokens/backoff. When WebSocket drops, fall back to Server-Sent Events or polling effect.
- **Background Refresh:** `BGAppRefreshTask` pulls new events/licensing info; `BGProcessingTask` flushes offline mutations.

## 6. Persistence, Sync & Offline
- **SwiftData Everywhere:** Cache snapshots and pending work in models such as `EventCache`, `PlaylistSnapshot`, `QueuedVote`, `DelegatedDevice`. Use `@Attribute(.unique)` plus cascades via `@Relationship(deleteRule: .cascade)` to prevent duplication.
- **CloudKit Sync:** Configure the `ModelContainer` with iCloud syncing for analytics-friendly data (e.g., user playlists) while sensitive tokens remain local. SwiftData handles conflict resolution; reducers reconcile deltas with authoritative API responses.
- **Offline Queue:** Votes/edits composed offline persist with timestamps + UUID correlation IDs. When connection returns, an actor replays them sequentially, and the reducer reconciles optimistic UI with server truth.
- **Schema Evolution:** Increment `ModelConfiguration` versions whenever fields change; add lightweight migration scripts + release notes for QA.

## 7. Telemetry, Instrumentation & Logging
- **Action Logging Pipeline:** Wrap `Store` with a middleware reducer that mirrors every action/state change to (a) the backend audit endpoint and (b) `os.Logger` categories (“Auth”, “Vote”, “Playlist”, “Delegation”).
- **Performance Instrumentation:** Adopt the new SwiftUI Instrument template to profile body invalidations and layout thrash. Enable it in CI smoke tests to prevent regressions.
- **MetricKit & Crash Reporting:** Register `MXMetricManager` observers; upload summaries on next cold launch. Integrate Crashlytics or Sentry for symbolicated crashes tied to action breadcrumbs.
- **Privacy Gates:** Production builds strip personally identifiable data from logs, while internal builds keep verbose traces guarded behind a feature flag.

## 8. Security & License Enforcement
- **Secrets & Keys:** OAuth credentials, API keys, and environment selectors live in `.xcconfig` + Keychain. App reads runtime secrets via secure storage; nothing lands in source.
- **Auth Stack:** Sign in with Apple + OAuth (Google/Facebook) through AuthenticationServices/ASWebAuthenticationSession. Tokens + refresh tokens stored in Keychain (Secure Enclave on supported devices).
- **Policy Engine Actor:** Enforces subject-mandated licenses (public, invite-only, geo/time-bound). Reducers consult it before enabling UI affordances and show messaging (not silent failures) when restrictions apply.
- **Device Metadata:** Every action logs OS version, device model, and app build to satisfy the “every action triggers server logs” clause.

## 9. Testing & CI
- **Reducer Tests:** Exhaustive `TestStore` suites for each feature (auth handshake, vote contention, playlist conflict resolution, delegation handoffs). Dependencies injected via `.withDependencies`.
- **Snapshot & UI Tests:** XCUITest for end-to-end flows; `SwiftSnapshotTesting` for critical UI states. Run on physical-device farms (Xcode Cloud / Firebase Test Lab) to verify realtime UI.
- **Concurrency Regression Tests:** Add targeted tests for actors (API client, stream actors, policy engine) that assert sendable behavior and ensure tasks cancel correctly.
- **Automation:** GitHub Actions or Xcode Cloud pipeline runs linting, formatting, DocC export, unit/UI tests, and uploads build artifacts for QA. Nightly load tests exercise backend ramp-up expectations via synthetic clients.

## 10. Tooling & External Dependencies
- **Required Packages:** Point-Free TCA 1.23+, Swift Identified Collections, Swift Collections, Swift Async Algorithms, AlamofireImage/SDWebImageSwiftUI (if we need advanced media handling), Bugsnag/Sentry SDK for crash reporting.
- **Dev Ergonomics:** SwiftLint + SwiftFormat pre-commit hooks, Danger for PR hygiene, Swift Package Index monitoring for dependency updates, and Renovate/Bundlerbot for automated version bumps.
- **Design System:** Build shared typography/color components as SwiftUI libraries so playlist and voting flows stay visually consistent.

This stack reflects 2025-era best practices for a pure-iOS 26 client: modern Swift concurrency, Observation-driven SwiftUI, TCA 1.23 navigation, SwiftData + CloudKit offline support, rigorous telemetry, and automated testing. It satisfies the subject’s requirements (remote-control architecture, configurable backend URL, realtime collaboration, license enforcement, detailed logging) while remaining maintainable for a multi-feature roadmap.
