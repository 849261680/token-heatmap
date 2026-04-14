# Token Heatmap Menu

Minimal macOS menu bar shell for `tokenheat`.

Current behavior:

- reads today usage via `tokenheat report today --json`
- runs `tokenheat run daily` for manual sync
- installs/removes daily `launchd` sync
- opens the project repo and GitHub profile

Build from the repo root:

```bash
./scripts/build-tokenheat-menu.sh
```
