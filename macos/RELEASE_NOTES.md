# Nomad Browser for macOS — alpha

This build has no address field and no ordinary web renderer. It searches only
the verified local Nomad object cache and renders signed plain-text documents
with native SwiftUI controls.

Security controls in this artifact:

- macOS App Sandbox is enabled;
- network client and server entitlements are absent;
- the release binary is rejected if it links WebKit, CFNetwork or Network;
- no URLSession, socket, external-URL or web-view API is permitted in source;
- every displayed object must pass SHA-256 and Ed25519 verification over
  `nomad-object-v1 || SHA256(payload)`;
- only `text/plain; charset=utf-8` is rendered;
- malformed, oversized, mutated or unsigned objects fail closed;
- search is bounded and entirely local.

This prerelease is ad-hoc signed unless the build is supplied with an Apple
Developer ID. It is not notarized, so macOS Gatekeeper can require an explicit
user override. Independent review and the multi-operator Nomad network remain
production gates.
