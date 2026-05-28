#!/bin/sh
# build-appimage.sh — stages an AppDir from the Wails-built Linux binary and
# invokes appimagetool to produce ClaudePanel-x86_64.AppImage in ./dist.
#
# Assumes:
#   * build/bin/claudepanel exists (run `wails build -platform linux/amd64` first)
#   * appimagetool is on $PATH (download from https://github.com/AppImage/AppImageKit/releases)

set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
APPDIR="$OUT_DIR/ClaudePanel.AppDir"

mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"

cp "$ROOT/bin/claudepanel" "$APPDIR/usr/bin/claudepanel"
chmod +x "$APPDIR/usr/bin/claudepanel"

cp "$ROOT/build/linux/claudepanel.desktop" "$APPDIR/usr/share/applications/claudepanel.desktop"
cp "$ROOT/build/linux/claudepanel.desktop" "$APPDIR/claudepanel.desktop"

cp "$ROOT/build/linux/icon.png" "$APPDIR/usr/share/icons/hicolor/256x256/apps/claudepanel.png"
cp "$ROOT/build/linux/icon.png" "$APPDIR/claudepanel.png"
ln -sf claudepanel.png "$APPDIR/.DirIcon"

cp "$ROOT/build/linux/AppRun" "$APPDIR/AppRun"
chmod +x "$APPDIR/AppRun"

ARCH=x86_64 appimagetool "$APPDIR" "$OUT_DIR/ClaudePanel-x86_64.AppImage"

echo "Built $OUT_DIR/ClaudePanel-x86_64.AppImage"
