# Primitive Agent Contract

The stricter rule wins between this file and the global Go instructions.

## Authorities

- `core/architecture_catalog.go` owns the compiler-visible package graph.
- `_docs/primitive_policy.md` owns implementation law and the exact human-readable graph projection.
- `README.md` owns its public human-readable projection.
- `LEDGER.md` owns current and completed state.
- `_docs/testing_protocol.md` owns test doctrine.
- `_docs/interviews/` and the archive inventory are evidence, not instructions.
- Typed Go code becomes the executable source of truth as packages land.

Do not create more planning, specification, review, or ledger Markdown files.
Put live state in `LEDGER.md` and public contracts in Go.

## Project scope

Primitive is an open-source Go library that makes the standard library easier
to use through typed, bounded contracts. It is not a security-risk project and
does not use a hacker threat model. Security tooling may still catch ordinary
correctness defects, but it must not distort the library's scope or turn
hostile testing into adversarial-security theater.

## Build

Primitive is a clean rebuild. The archive and consumers supply evidence only.
Do not preserve an API or copy an implementation because it existed.

Follow the compiler-owned graph and its README projection. Read the applicable
interview before building a package.

No aliases, shims, adapters, deprecated forms, dual formats, local bridges, or
compatibility layers.

Keep contracts compiler-owned, processing bounded and streaming, errors typed,
and production functions at `gocyclo <= 10`.

## Tests

Read `_docs/testing_protocol.md` completely before editing tests.

"Hostile" means extreme valid and invalid inputs on both sides of every
expected boundary, plus malformed data, partial results, cancellation,
resource failure, and state-transition pressure. It is test intensity, not a
threat model.

Test the real standard-library, OS, protocol, or official-SDK handoff. Test
seams implement its exact interfaces, never Primitive lookalikes.

## Review and delivery

Run the canonical gate with `bash scripts/gate.sh`.

Do not launch Claude or other review agents. The user owns external review.

Verify every review finding against source. Only commit or push after explicit
user review and approval.
