# Nomad browser core

Browser-engine-independent client contracts for Nomad v0.1.

## Implemented

- package-level Selection Firewall separation:
  - `planner` sees only public network configuration;
  - `selector` sees local embeddings, basins and candidates but no planner or
    fabric;
- an immutable `localcache` that accepts only objects whose exact bytes,
  SHA-256 commitment and Ed25519 signatures verify;
- a canonical signed-bundle payload that binds local renderer paths and MIME
  types to exact object hashes;
- an `adapter` API that serves GET/HEAD exclusively from the verified local
  cache, with no resolver, socket, HTTP client or network fallback;
- strict CSP, Permissions Policy, no-referrer and nosniff response defaults;
- a non-configurable `egress.Policy` that denies DNS, TCP, UDP, HTTP,
  WebSocket, WebRTC, preconnect, speculative fetch, service-worker updates,
  extension networking, telemetry, crash reporting and Safe Browsing calls;
- dependency-graph tests and real CI against commit- and SHA-256-pinned
  component snapshots.

MIME bindings live inside the signed canonical bundle bytes. A network-supplied
header cannot silently reinterpret an object as executable content.

## Engine integration contract

A Firefox or Chromium Nomad context must:

1. route renderer resource loads to `adapter.Handle`;
2. call `egress.Policy` before scheme dispatch and before every lower-level
   network capability;
3. disable or isolate every browser service listed by the policy;
4. keep ordinary profiles and Nomad profiles in separate process/storage
   boundaries;
5. pass engine-level tests proving that denied operations produce no DNS,
   socket or packet event.

## Honest status

The core API and fail-closed tests exist. The Firefox and Chromium forks do
**not** yet implement this contract, so Nomad browser-engine isolation is not a
v0.1 completion claim. Renderer process sandboxing, storage partitioning,
extensions, service workers and browser-vendor background services remain
engine-specific release gates.

The component repositories are private. `components/` is a minimal generated
integration snapshot; `COMPONENTS.lock` pins source commits and
`COMPONENTS.sha256` is checked in CI.

```bash
go test -race ./...
go vet ./...
```
