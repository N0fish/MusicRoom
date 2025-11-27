# MOBILE AGENT GUIDE · OPERATIONS NOTES

## Repository & CI Context
- The Music Room codebase is hosted on GitHub, so we can rely on GitHub Actions for continuous integration, linting, DocC export, and automated UI smoke tests.
- When defining workflows, ensure secrets stay in GitHub Action secrets or self-hosted runners; never embed credentials in the repo per global rules.

## Build & Tooling Requirements
- **Primary Build Method:** Use `tuist generate` followed by `xcodebuild` or `tuist build`.
- **MCP Note:** The `XcodeBuildMCP` mentioned in previous versions of this guide is currently unavailable. Agents should fallback to standard shell commands (`xcodebuild`, `xcrun simctl`) for building and running the app.
- **Simulator:** Ensure you target a simulator that matches the deployment target (iOS 26.0+). `iPhone 17` (iOS 26.1) is a verified working target.

## Living Document
- This guide will continue to aggregate mobile-agent-specific practices (workflow templates, simulator fleets, release rituals). Add dated sections as new decisions land.
