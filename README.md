# Primitive

Primitive is the product-neutral Go primitive layer used to build reliable
command-line tools and services.

Module: `github.com/deliri/primitive/v2026`

First release: `v2026.0.0`

Primitive is a clean rebuild with no compatibility obligation to the archived
implementation. It makes Go's standard library, operating-system primitives,
documented protocols, and official SDKs typed, validated, bounded, streaming,
and easier to compose. The real substrate still does the work.

- [LEDGER.md](LEDGER.md) owns project state.
- [\_docs/testing_protocol.md](_docs/testing_protocol.md) owns test doctrine.
- [\_docs/interviews/](_docs/interviews/) preserves the recon evidence.

## Live project truth

Run `forge project info refresh` from the repository root. Forge inspects the
current Go tree with compiler-backed source analysis, refreshes each package's
`projectstandard_test.go` functions, and emits the bounded project projection
under `.forge/`.

Primitive owns package identities, effect ownership, schemas, and validation.
It does not maintain a second handwritten list of current files, imports,
declarations, benchmarks, fuzz targets, or effect sites. Those are observations
and remain "not observed" until Forge produces them.

## License

Primitive is licensed under the [Mozilla Public License 2.0](LICENSE).

Copyright 2026 Deliri Software Inc., operating as Off Grid Software.
