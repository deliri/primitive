# Compiler environment review — 2026-09-05

Status: user reviewed and approved for v2026.1.13 publication on 2026-09-05.

Published base: v2026.1.12, commit
`35a6a1093487b41ebd2a6cd6b1c3728c972c94df`.

Hammer's real offline probe let cmd/go request a toolchain ZIP because the
compiler capability always captured ambient process settings. The change lets
callers supply an exact typed `process.Environment` in gotoolchain.Configuration.
Open validates it and copies its variables. Omitted input explicitly retains
ambient capture. Primitive performs the execution; Hammer chooses its offline
settings in Hammer core. No Hammer policy or product meaning enters Primitive.

The same review fixes two demonstrated mechanical validation defects:
PackageAnalysis must reject foreign, duplicate and unordered compiler units;
ToolchainVersion must reject spellings Go itself does not recognize. The latter
now uses standard-library go/version in addition to the existing bounded,
stable-release domain. New semantic fuzz seeds preserve the malformed versions.

The unused SourceOverlay API and implementation are deleted, along with their
inventory entries and tests. No production caller was found in Hammer, Witness,
Bug, Anvil, Blink Kernel or PeachFuzz. Consumers outside that checked set using
the removed API will receive compiler errors and must update their real calls.
There is no compatibility layer.

The sequential compiler checker remains. Examining Go's unitchecker confirms
the selected-files/export-data approach. An attempted direct LoadSyntax change
reproduced the already recorded race inside Go 1.27.1/x-tools v0.49.0 and was
discarded. The new comment records why this code must not be removed on an
unverified assumption. Compiler type graphs are package-scoped aggregates,
not O(1)-memory streams or a persisted product model.

Production files changed: gotoolchain/contracts.go, toolchain.go, analysis.go
(reason comment), data_flow_inventory.go; source_overlay.go is deleted.
Tests added: environment_external_test.go, analysis_ownership_external_test.go,
scalars_fuzz_test.go. source_overlay_external_test.go is deleted with its API.
No other production package is changed; googleidentity remains at its published
reviewed state.

Evidence is retained under `/private/tmp/primitive-review-20260904/`:

- Typed-environment and Hammer offline semantic mutations fail; actual Go
  execution, a real HTTP proxy, and caller-slice mutation exercise production.
- Foreign/duplicate analysis and malformed version seeds fail before fixes.
- `scalar-final-race`: `go test -count=1 -race -json ./gotoolchain`, 40 emitted
  pass events, no emitted failure or skip, stable before/after source manifest.
- `scalar-final-*`: go fix, go vet, witness-lint, staticcheck, errcheck,
  `goconst -min-length 4 -ignore-tests`, and `gocyclo -over 10 -ignore '_test.go$'`
  pass over gotoolchain. `scalar-final-deadcode` passes `deadcode -test ./...`.
- `scalar-semantic-fuzz`: `go test -run '^$' -fuzz
  '^FuzzCompilerScalarSemanticClosure$' -fuzztime=15s -parallel=2 ./gotoolchain`,
  321,340 executions, passed. No benchmark claim is made.

These are dirty-tree development executions tied to recorded source manifests,
not independent acceptance or a completed protocol-wide package audit. All
earlier failures, including the rejected concurrent loader, remain retained.
Hammer's 13 non-local packages and focused local offline test pass with an
explicit review workspace. Its full local suite intentionally requires a
published dependency and remains pending integration after this review.

The final candidate binary also inspected all six private generations
successfully. Its binary digest, Go build information, exact inspection commands
and artifact hashes are in
`/private/tmp/hammer-release-review-20260905/final-inspection.json`; both source
trees have separate recorded file manifests in that directory.

Approved release action: bump compass/config.json to v2026.1.13,
validate the version-bearing package, commit this slice,
publish branch/tag, update Hammer's normal dependency, then run the complete
Hammer build and uncached test suite without a local replacement. No deployment,
external coordination message, or independent acceptance is part of that action.

The reviewer recorded three non-blocking follow-ups: construct reversed matching
analysis units in the ownership test; add the malformed version seeds to the
named deterministic table; and place the sequential-checker warning beside the
package-load configuration. These do not change the approved production slice
or gate this publication. Hammer's remaining gate is published-dependency
integration and the complete uncached race suite.
