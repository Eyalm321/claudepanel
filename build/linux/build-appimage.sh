#!/bin/sh
# build-appimage.sh — stages an AppDir from the Wails-built Linux binary and
# invokes appimagetool to produce ClaudeBar-x86_64.AppImage in ./dist.
#
# Assumes:
#   * build/bin/claudebar exists (run `wails build -platform linux/amd64` first)
#   * appimagetool is on $PATH (download from https://github.com/AppImage/AppImageKit/releases)

set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
APPDIR="$OUT_DIR/ClaudeBar.AppDir"

mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

cp "$ROOT/build/bin/claudebar" "$APPDIR/usr/bin/claudebar"
chmod +x "$APPDIR/usr/bin/claudebar"

cp "$ROOT/build/linux/claudebar.desktop" "$APPDIR/usr/share/applications/claudebar.desktop"
cp "$ROOT/build/linux/claudebar.desktop" "$APPDIR/claudebar.desktop"

cp "$ROOT/build/linux/icon.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/claudebar.png"
cp "$ROOT/build/linux/icon.png" "$APPDIR/claudebar.png"
ln -sf claudebar.png "$APPDIR/.DirIcon"

cp "$ROOT/build/linux/AppRun" "$APPDIR/AppRun"
chmod +x "$APPDIR/AppRun"

ARCH=x86_64 appimagetool "$APPDIR" "$OUT_DIR/ClaudeBar-x86_64.AppImage"

echo "Built $OUT_DIR/ClaudeBar-x86_64.AppImage"
