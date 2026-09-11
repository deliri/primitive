# Wiring removal — v2026.1.63

Primitive no longer exports the `wiring` package or `core.PackageWiring`. Runtime component graphs, connectivity, cycles, and command readiness belong to the product. The user explicitly directed removal of this API after review. This is a clean break, with no replacement graph framework, compatibility wrapper, alias, or readiness abstraction.

## Migration

Before updating their Primitive dependency, callers must remove imports of `github.com/deliri/primitive/v2026/wiring`, the `core.PackageWiring` catalog reference, and uses of `Definition`, `Describer`, `Request`, `Manifest`, `Derive`, and graph-specific error types. Construct components directly using ordinary Go. If a product actually requires a graph check, the product owns that check and its meaning; a successful graph check does not prove runtime readiness.

Read-only inspection found consumers in these server checkouts:

- Witness: `cmd/witness/runtime_wiring.go` and its hostile tests; related source fixtures in `standard_test.go`.
- Peachfuzz: `internal/cli/runtime_wiring.go` and its hostile tests.
- Bug: `cli/runtime_wiring.go` and its hostile tests.

Consumer repositories and dependency pins were not changed by this Primitive release. Their migrations remain required before adopting it. Package identities serialize as canonical names; callers must not persist an enum ordinal as a substitute for the shared marshaler.

## Removed surfaces and retained proof

The graph producer, snapshot, graph validator, error decoder, tests, fuzz target, and benchmark were removed together. The architecture catalog, compiler-visible export inventory, and authored package claim stream no longer advertise Wiring. Historical evidence remains in Git and in retained source snapshots; historical inventory tables are historical, not the current queue.

The new Core deletion ratchet proves that parsing the retired name returns a typed refusal and no identity, JSON refusal preserves a populated receiver, and the current catalog excludes the package. It failed against production revision `b71a7327c11b62becdd89ecdbce7a6be82bb59e7` before removal. Existing package-identity exhaustive tests cover the remaining domain. Existing external JSON/text inventory fuzz targets exercise the surviving public decoders. There is no remaining Wiring ingress to fuzz or operation to benchmark; inventing replacements would defeat the removal.

The earlier review also demonstrated that Derive copied oversized dependency slices before checking their bound: a rejected 1,048,576-element slice allocated about 2 MB per call. Deletion removes that path. This exploratory allocation probe is not accepted benchmark or performance evidence.

## Execution scope and limitations

Evidence resides on furnace under `/work/engineering-evidence/primitive/wiring-upgrade-20260910`; the separate verification report records its final commit and manifest digest. Runs retain exact arguments, source hashes, source-tree status, toolchain, output hashes and sizes, exit status, and separate event counts. Test runs use `-count=1`; filtered runs prove only their stated filter. Independent acceptance is not claimed.

The full Core suite and claims inventory were also run on the clean v2026.1.62 baseline in `/work/primitive-review-baselines/wiring-v62`. Baseline and candidate share these existing failures:

- `TestPrimitiveArchitectureCatalogMatchesEveryLandedPackage`
- `TestCleanUpgradeDebtIsAbsentFromLandedProduction`
- `TestRealWorldEffectOwnershipMatchesLandedProduction`
- `TestPrimitiveJSONMarshalersHaveCompleteValidationWitnesses`
- `TestCoreTopLevelExportsHaveTwoNamedPrimitiveConsumers`
- `TestPrimitiveClaimStreamCoversEveryCompiledPackageWithoutAClaimQuota` (missing Compass and Version claims)

These failures remain failures; this release does not claim a green full Core suite or repository-wide acceptance. Focused architecture/deletion tests, analyzers, full-module compilation, and selected fuzz execution are recorded separately. No consumer build is claimed.

Retained unsuccessful attempts include the initial allocation-probe syntax error, the intentional red test, an edit-script quoting failure that left production unchanged (the misleadingly named `removal-green` run remained red), and the compiler catching the export inventory count until it was reduced with the deleted entry. None was overwritten or reclassified as passing.
