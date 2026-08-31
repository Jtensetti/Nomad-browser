# Nomad browser core

Browser-engine-independent client contracts for Nomad v0.1.

Two native clients ship from this repository, over the same portable Go core.

`macos/` holds a SwiftUI alpha. It has no address field or general-purpose web
renderer. It searches only verified local Nomad objects, renders signed plain
text, runs inside the macOS App Sandbox and is built as a universal
downloadable DMG by GitHub Actions. Its explicit, evidence-based release
boundary is documented in [`macos/SECURITY.md`](macos/SECURITY.md).

`cmd/nomad-browser` is the Linux client, documented in
[`linux/README.md`](linux/README.md). It does the same job through the Go
packages directly, and it is where the networkless claim is strongest: its
dependency graph contains no networking package at all, and it runs in a
network namespace with no interfaces, so the claim is enforced by the kernel
rather than by the program. `scripts/verify-networkless.sh` proves the
namespace is genuinely empty using a probe that must succeed outside it, and
runs in CI.

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
- periodic discovery of newly materialized `.nomadobject` files from the local
  sandbox cache, on a public five-second cadence that never depends on a query
  or selected result; malformed entries are isolated and rejected per file;
- an `objectstore` package that verifies signed objects in Go, independently of
  the Swift implementation of the same boundary, checked against the corpus
  both clients share;
- a `search` index that embeds and tokenizes each object when it is added, so
  one search costs exactly one embedding call however large the corpus is, and
  that names the embedder behind every ranking.

MIME bindings live inside the signed canonical bundle bytes. A network-supplied
header cannot silently reinterpret an object as executable content.

## The semantic side, and what it costs

Ranking runs through `basin.Embedder`. The only implementation in the tree is
`basin.LexicalHashEmbedder`, which its own documentation calls a lexical
baseline and not a semantic model, and every result names the embedder that
produced it so a lexical ranking is never presented as a semantic one. A real
model attaches either in-process through that interface or, keeping this
process socket-free, through the sealed loopback service in
`components/nomad-semantic-basins/basin/loopback`. Nothing in the Nomad tree
builds that model; see `EXTERNAL_BLOCKERS.md` in nomad-protocol.

Inference latency is a privacy property here, not just a responsiveness one.
How long an embedder takes depends on the query, the object and how many
candidates matched, all of which are private. If that latency could reach an
externally observable event it would modulate one with private state, which is
the single thing Nomad must never do. It cannot: the packages that hold query
text sit on the private side of the Selection Firewall and cannot reach the
emission planner, and the fabric emits on a fixed cadence regardless. A slow
model costs the reader a wait and costs the wire nothing.

What that leaves is ordinary cost, and the index is built so a model can be
slow without being felt. Each object is embedded and tokenized when it is
added, not when a query arrives, because objects materialize rarely and queries
are interactive. One search is therefore one embedding call and a set of
lookups, however large the corpus --
`TestASearchCostsExactlyOneEmbeddingCall` pins that. Every embedding call is
bounded by a required budget, and an embedder that hangs is abandoned at it
rather than waited on.

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

The native macOS alpha deliberately avoids that engine boundary by not using a
web engine at all. The live Nomad materializer can now populate its verified
object directory while the browser is running. A protected manual release
workflow now imports an ephemeral Developer ID identity, enables the hardened
runtime and secure timestamp, submits the DMG to Apple, requires an
`Accepted` result, staples the ticket and runs Gatekeeper checks before
publishing. Credential setup is documented in
[`macos/NOTARIZATION.md`](macos/NOTARIZATION.md). A credentialed successful
run, provisioned cross-application storage, independent operators and external
review remain production evidence gates.

The component repositories are private. `components/` is a minimal generated
integration snapshot; `COMPONENTS.lock` pins source commits and
`COMPONENTS.sha256` is checked in CI.

```bash
go test -race ./...
go vet ./...
```
