# AGENTS.md

Oaklog is a standalone generalized Minecraft server log uploader CLI.

Current scope:
- Upload local log files.
- Initial provider: mclo.gs.
- Print shareable URL.
- Keep CLI script-friendly.

Do not add:
- mclo.gs get/delete/analyse/insights behavior unless explicitly requested.
- Varda-specific modpack logic.
- Minecraft server installer/updater behavior.
- CurseForge/Modrinth logic.
- GUI/TUI behavior.
- background daemons or file watchers unless explicitly requested.

Go:
- prefer standard library
- use gofmt
- keep tests deterministic
- do not hit real network in tests
- use httptest for provider client tests
- dev-only repo-root `.env` fallback exists only behind `-tags oaklog_dev`; release builds must not include it

Release:
- use the external `go-build-bin` tool for build, archive, and checksums
- use GitHub CLI (`gh release create` / `gh release upload`) for upload
- keep build and upload separate
- do not upload raw binaries
- release archives should contain only the platform binary
