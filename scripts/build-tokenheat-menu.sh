#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="Token Heatmap"
APP_DIR="$ROOT_DIR/dist/$APP_NAME.app"
MACOS_DIR="$APP_DIR/Contents/MacOS"
RESOURCES_DIR="$APP_DIR/Contents/Resources"
INFO_PLIST="$APP_DIR/Contents/Info.plist"
CLI_BINARY="$ROOT_DIR/tokenheat"
PROFILE_REPO_DIR_DEFAULT="$ROOT_DIR/../849261680"
PROFILE_REPO_DIR="${PROFILE_REPO_DIR:-$PROFILE_REPO_DIR_DEFAULT}"
PROJECT_URL="${PROJECT_URL:-https://github.com/849261680/token-heatmap}"
PROFILE_URL="${PROFILE_URL:-https://github.com/849261680}"

if ! xcodebuild -version >/dev/null 2>&1; then
  echo "Full Xcode is required to build the macOS menu app." >&2
  echo "Install Xcode, then run this script again." >&2
  exit 1
fi

if [[ ! -x "$CLI_BINARY" ]]; then
  echo "Missing CLI binary at $CLI_BINARY" >&2
  echo "Run: go build -o tokenheat ./cmd/gitoken" >&2
  exit 1
fi

rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR" "$RESOURCES_DIR"

python3 - <<'PY' "$ROOT_DIR/apps/macos/TokenHeatMenu/Info.plist.template" "$INFO_PLIST" "$RESOURCES_DIR/tokenheat" "$ROOT_DIR" "$PROFILE_REPO_DIR" "$PROJECT_URL" "$PROFILE_URL"
from pathlib import Path
import sys

template_path, output_path, cli_path, repo_dir, profile_repo_dir, project_url, profile_url = sys.argv[1:8]
content = Path(template_path).read_text()
content = content.replace("__CLI_PATH__", cli_path)
content = content.replace("__REPO_DIR__", repo_dir)
content = content.replace("__PROFILE_REPO_DIR__", profile_repo_dir)
content = content.replace("__PROJECT_URL__", project_url)
content = content.replace("__PROFILE_URL__", profile_url)
Path(output_path).write_text(content)
PY

cp "$CLI_BINARY" "$RESOURCES_DIR/tokenheat"
chmod +x "$RESOURCES_DIR/tokenheat"

swiftc \
  -parse-as-library \
  -target "$(uname -m)-apple-macos13.0" \
  -framework SwiftUI \
  -framework AppKit \
  "$ROOT_DIR/apps/macos/TokenHeatMenu/Sources/TokenHeatMenu/TokenHeatCLI.swift" \
  "$ROOT_DIR/apps/macos/TokenHeatMenu/Sources/TokenHeatMenu/MenuBarViewModel.swift" \
  "$ROOT_DIR/apps/macos/TokenHeatMenu/Sources/TokenHeatMenu/MenuBarContentView.swift" \
  "$ROOT_DIR/apps/macos/TokenHeatMenu/Sources/TokenHeatMenu/TokenHeatMenuApp.swift" \
  -o "$MACOS_DIR/TokenHeatMenu"

codesign --force --deep --sign - "$APP_DIR" >/dev/null 2>&1 || true

echo "Built: $APP_DIR"
