# MOBILE AGENT GUIDE · OPERATIONS NOTES

## Repository & CI Context
- The Music Room codebase is hosted on GitHub, so we can rely on GitHub Actions for continuous integration, linting, DocC export, and automated UI smoke tests.
- When defining workflows, ensure secrets stay in GitHub Action secrets or self-hosted runners; never embed credentials in the repo per global rules.

## Build & Tooling Requirements
- Local and CI builds must use the **XcodeBuildMCP** interface available in the Codex CLI (e.g., `mcp__XcodeBuildMCP__build_macos`, `...build_run_sim`, `...test_sim`). This MCP handles the modern Xcode 26 toolchain and device orchestration; do not bypass it with raw `xcodebuild` scripts unless noted here later.
- When scripting automation, surface the MCP command and parameters in documentation/PRs so other agents can reproduce builds or tests exactly.

## Living Document
- This guide will continue to aggregate mobile-agent-specific practices (workflow templates, simulator fleets, release rituals). Add dated sections as new decisions land.
