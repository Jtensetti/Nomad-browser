# Nomad browser core

Browser-engine-independent client components for the Nomad experiments.

The repository uses **package-level capability separation** rather than a single browser `Engine` object:

- `selector` may depend on local embedding/basin/reconstruction code and has no dependency on the network planner or fabric;
- `planner` may depend on public emission-planning code and has no dependency on semantic or reconstruction packages.

`architecture_test.go` checks those dependency graphs with `go list -deps` so an accidental cross-import fails CI.

## Current scope

Implemented:

- local ranking of already-available candidates,
- public emission-plan composition,
- dependency-boundary regression tests.

Not implemented:

- rendering,
- transport,
- browser storage or cache isolation,
- JavaScript/extensions/service-worker isolation,
- process or OS sandboxing,
- Firefox/Chromium adapters.

This package split reduces one class of accidental dependency. It does **not** establish runtime non-interference; global state, IPC, browser services and the OS remain separate audit targets.

```bash
go test -race ./...
go vet ./...
```

The repository currently uses local `replace` directives for sibling Nomad modules. CI needs explicit cross-repository checkout/authentication before remote builds are meaningful.
