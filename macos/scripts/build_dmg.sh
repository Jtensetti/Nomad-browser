#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
macos_root="$repo_root/macos"
dist="$repo_root/dist"
version="${NOMAD_VERSION:-0.1.0-alpha.1}"
build_number="${NOMAD_BUILD_NUMBER:-1}"
identity="${CODESIGN_IDENTITY:--}"
app="$dist/Nomad Browser.app"

rm -rf "$dist"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"

"$macos_root/scripts/security_gate.sh"
swift build --package-path "$macos_root" -c release --arch arm64 --arch x86_64

binary="$(find "$macos_root/.build" -type f -name NomadBrowser -perm -111 | rg '/(release|Release)/NomadBrowser$' | head -n 1)"
if [[ -z "$binary" ]]; then
    echo "universal NomadBrowser executable was not produced" >&2
    exit 1
fi
binary_description="$(file "$binary")"
if [[ "$binary_description" != *arm64* || "$binary_description" != *x86_64* ]]; then
    echo "release executable is not universal: $binary_description" >&2
    exit 1
fi
cp "$binary" "$app/Contents/MacOS/NomadBrowser"

resource_bundle="$(find "$macos_root/.build" -type d -name 'NomadBrowser_NomadBrowser.bundle' | head -n 1)"
if [[ -z "$resource_bundle" ]]; then
    echo "SwiftPM resource bundle was not produced" >&2
    exit 1
fi
cp -R "$resource_bundle" "$app/Contents/Resources/"

cp "$macos_root/Info.plist" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $build_number" "$app/Contents/Info.plist"

icon_png="$dist/AppIcon-1024.png"
swift "$macos_root/scripts/make_icon.swift" "$icon_png"
iconset="$dist/AppIcon.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" "$icon_png" --out "$iconset/icon_${size}x${size}.png" >/dev/null
    doubled=$((size * 2))
    sips -z "$doubled" "$doubled" "$icon_png" --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/AppIcon.icns"

codesign --force --deep --strict --options runtime --entitlements "$macos_root/NomadBrowser.entitlements" --sign "$identity" "$app"
codesign --verify --deep --strict --verbose=2 "$app"
codesign -d --entitlements :- "$app" >"$dist/effective-entitlements.plist" 2>&1
if ! rg -q 'com\.apple\.security\.app-sandbox' "$dist/effective-entitlements.plist"; then
    echo "signed application lost its sandbox entitlement" >&2
    exit 1
fi
if rg -q 'com\.apple\.security\.network\.(client|server)' "$dist/effective-entitlements.plist"; then
    echo "signed application unexpectedly has network capability" >&2
    exit 1
fi

linked="$(otool -L "$app/Contents/MacOS/NomadBrowser")"
if printf '%s\n' "$linked" | rg -q '/(WebKit|CFNetwork|Network)\.framework/'; then
    echo "release binary links an ordinary-network or web engine framework" >&2
    printf '%s\n' "$linked" >&2
    exit 1
fi

dmg_root="$dist/dmg-root"
mkdir -p "$dmg_root"
cp -R "$app" "$dmg_root/"
ln -s /Applications "$dmg_root/Applications"
dmg="$dist/Nomad-Browser-${version}-macOS-universal.dmg"
hdiutil create -volname "Nomad Browser" -srcfolder "$dmg_root" -ov -format UDZO "$dmg" >/dev/null
if [[ "$identity" != "-" ]]; then
    codesign --force --sign "$identity" "$dmg"
fi
hdiutil verify "$dmg"
shasum -a 256 "$dmg" >"$dmg.sha256"
printf '%s\n' "$dmg"
