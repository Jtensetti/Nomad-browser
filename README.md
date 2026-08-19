# Nomad Browser Core

Browser-engine-independent client core for the Nomad research architecture.

This repository exists to keep the privacy-critical client logic out of Firefox- and Chromium-specific patches.

## Current invariant

**Private selection must not influence externally observable network planning.**

`SearchLocal` accepts user intent and locally-known opaque candidates. `EmissionPlan` is derived only from public protocol configuration and epoch. There is deliberately no API path from private query state to the network plan.

## Why a separate core?

The eventual Firefox and Chromium integrations should be thin adapters around the same audited core. Duplicating privacy logic in two browser engines would create two independent places for leaks.

## Current status

Research implementation. It provides:

- local intent → semantic basin conversion
- local candidate ranking
- a hard API separation from Selection Firewall emission planning
- tests proving different private queries produce identical network plans

It does **not** yet provide HTML rendering, JS sandboxing, cache storage, browser UI, or a production network transport.

## Run

Clone the Nomad repositories side-by-side, then:

```bash
go test -race ./...
go vet ./...
go run ./cmd/nomad-browser
```
