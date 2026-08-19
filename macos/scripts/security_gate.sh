#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
source_root="$repo_root/macos/Sources/NomadBrowser"
entitlements="$repo_root/macos/NomadBrowser.entitlements"

forbidden='import[[:space:]]+(WebKit|Network|FoundationNetworking|Darwin)|URLSession|WKWebView|WKURLSchemeHandler|NWConnection|NWBrow|CFNetwork|CFStream|NSStream|NSAppleScript|SFSafariApplication|NSWorkspace\.shared\.open|openURL\(|Process[[:space:]]*\(|(^|[^[:alnum:]_])(socket|connect|sendto|recvfrom|getaddrinfo|dlopen|dlsym)[[:space:]]*\('
if LC_ALL=C grep -ERnE "$forbidden" "$source_root"; then
    echo "forbidden ordinary-network or external-navigation capability in macOS client" >&2
    exit 1
fi

if ! /usr/libexec/PlistBuddy -c 'Print :com.apple.security.app-sandbox' "$entitlements" | grep -qx true; then
    echo "app sandbox entitlement is required" >&2
    exit 1
fi

if /usr/libexec/PlistBuddy -c 'Print' "$entitlements" | grep -Eq 'com\.apple\.security\.network\.(client|server)'; then
    echo "network client/server entitlements are forbidden" >&2
    exit 1
fi

swift test --package-path "$repo_root/macos"
