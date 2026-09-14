#!/usr/bin/env bash
# Bump the app version across all three KrankyBear InstallerBear binaries and packaging metadata.
# Canonical default lives in build-config.sh (KB_VERSION_DEFAULT); this script keeps every
# consumer file in lock-step so the CLI and main app ship
# with one version.
#
# Usage: ./setver.sh 0.3.1
#
# macOS/BSD sed: sed -i '' …   Linux GNU sed: sed -i …

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

# shellcheck disable=SC1091
source "${ROOT}/build-config.sh"

if [[ "$(uname -s)" == Darwin ]]; then
  sed_i() { sed -i '' "$@"; }
else
  sed_i() { sed -i "$@"; }
fi

# Pull the current appVersion out of a Go file, for the interactive prompt.
go_ver() { grep -E '^[[:space:]]*appVersion[[:space:]]*=' "$1" 2>/dev/null | head -1 | sed -E 's/.*"([^"]+)".*/\1/'; }

if [[ $# -ge 1 ]]; then
  ver=$1
else
  echo "Enter a version number."
  echo "    packaging default (build-config.sh): ${KB_VERSION_DEFAULT}"
  echo "    Main App (main.go):                   $(go_ver main.go)"
  read -r ver
  if [[ -z "${ver}" ]]; then
    echo "No version change; exiting."
    exit 0
  fi
fi

echo "Setting version: ${ver}"
echo ""

echo "build-config.sh (KB_VERSION_DEFAULT)"
sed_i "s/^export KB_VERSION_DEFAULT=.*/export KB_VERSION_DEFAULT=\"${ver}\"/" ./build-config.sh

echo "installerbear.yaml (identity.version)"
sed_i "s/version: .*/version: \"${ver}\"/" ./installerbear.yaml

# appVersion = "..." in each app's main.go
for gofile in main.go; do
  if [[ -f "$gofile" ]]; then
    echo "${gofile} (appVersion)"
    sed_i "s/^\([[:space:]]*appVersion[[:space:]]*=[[:space:]]*\"\)[^\"]*\(\".*\)/\1${ver}\2/" "$gofile"
  fi
done

# FyneApp.toml files (root = CLI/GUI metadata, main app has its own)
for toml in FyneApp.toml; do
  if [[ -f "$toml" ]]; then
    echo "${toml} (Version)"
    sed_i "s/^[[:space:]]*Version = \".*\"/  Version = \"${ver}\"/" "$toml"
  fi
done

# Windows Inno Setup
if [[ -f "./${KB_INNO_ISS}" ]]; then
  echo "${KB_INNO_ISS} (MyAppVersion)"
  sed_i "s/#define MyAppVersion \".*\"/#define MyAppVersion \"${ver}\"/" "./${KB_INNO_ISS}"
fi

# Windows version resources (go-winres) — GUI is the only binary with embedded version info
if [[ -f "winres/winres.json" ]]; then
  echo "winres/winres.json"
  sed_i "s/\"file_version\": \"[^\"]*\"/\"file_version\": \"${ver}\"/" winres/winres.json
  sed_i "s/\"product_version\": \"[^\"]*\"/\"product_version\": \"${ver}\"/" winres/winres.json
  sed_i "s/\"FileVersion\": \"[^\"]*\"/\"FileVersion\": \"${ver}\"/" winres/winres.json
  sed_i "s/\"ProductVersion\": \"[^\"]*\"/\"ProductVersion\": \"${ver}\"/" winres/winres.json
fi

# macOS Info.plist sample (CFBundleShortVersionString + CFBundleVersion — this
# template doesn't track a separate build number, so both mirror the same
# version string)
if [[ -f "Info-plist.txt" ]]; then
  echo "Info-plist.txt (CFBundleShortVersionString, CFBundleVersion)"
  sed_i "/<key>CFBundleShortVersionString<\\/key>/,/<string>/ s/<string>[^<]*<\\/string>/<string>${ver}<\\/string>/" Info-plist.txt
  sed_i "/<key>CFBundleVersion<\\/key>/,/<string>/ s/<string>[^<]*<\\/string>/<string>${ver}<\\/string>/" Info-plist.txt
fi

# ReleaseNotes.md: insert a header for this version right above the
# previous most-recent "## Version ..." entry - i.e. after the "# Release
# Notes" title and "## Future Ideas" block at the top of the file, not
# before them. (Earlier versions of this script prepended at the very top
# of the file instead, which pushed the title/Future-ideas section down
# below a stack of version headers - fixed 2026-09-12. Switched from a
# bare "Version ..." + decorative "━" underline to a real Markdown "##"
# heading, with no underline, when ReleaseNotes.md itself switched to
# proper Markdown - 2026-09-14.)
echo "ReleaseNotes.md"
if ! grep -q "^## Version ${ver} " ReleaseNotes.md; then
  echo "  adding '## Version ${ver}' header"
  insert_line=$(grep -n '^## Version ' ReleaseNotes.md | head -1 | cut -d: -f1)
  if [[ -z "${insert_line}" ]]; then
    # No existing "## Version ..." line at all - append at the end rather
    # than guess a position.
    {
      cat ReleaseNotes.md
      echo ""
      echo "## Version ${ver} - $(date '+%B %d, %Y')"
      echo ""
    } > ReleaseNotes.md.new
  else
    {
      head -n "$((insert_line - 1))" ReleaseNotes.md
      echo "## Version ${ver} - $(date '+%B %d, %Y')"
      echo ""
      tail -n "+${insert_line}" ReleaseNotes.md
    } > ReleaseNotes.md.new
  fi
  mv ReleaseNotes.md.new ReleaseNotes.md
fi

echo ""
echo "Version updated to: ${ver}"
echo ""
echo "Files updated:"
echo "  - build-config.sh (KB_VERSION_DEFAULT)"
echo "  - main.go (appVersion)"
echo "  - installerbear.yaml"
echo "  - FyneApp.toml"
echo "  - ${KB_INNO_ISS}"
echo "  - winres/winres.json"
echo "  - Info-plist.txt"
echo "  - ReleaseNotes.md"
echo ""
echo "Don't forget to flesh out ReleaseNotes.md with the actual changes!"

# "Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning." Winston Churchill, November 10, 1942
