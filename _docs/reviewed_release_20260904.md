# Primitive v2026.1.12 review record

The 2026-09-04 review authorizes fixing the reported defects, bumping the
version, committing and publishing this Primitive slice, then updating Hammer.
The Google verifier handoff is included in the reviewed slice.

## Contracts and removals

- `capabilities` owns standard function and declaring-receiver method
  classification. Promoted TCP/UDP/Unix methods resolve through `net.conn`.
  Effect, pure, contextual and unresolved are separate validated outcomes.
  Blanket package defaults are removed. Conflicting matching rules refuse.
- Callable replacements identify actual exported functions and request/result
  types. Tests bind those coordinates to compiled function signatures.
  `os.Exit` retains process ownership and explicitly has no offered operation.
- `sourceobservation` retains call references and unobserved/partial/observed
  counters. Contextual or unresolved calls cannot report classification closure.
  This is a call census, not proof of all runtime effects or product compliance.
- `exchange.BufferResponse` bounds buffered bytes, defers release to the product
  callback and returns exact committed headers/body-write outcomes. Ignored
  handler write errors remain failures. HEAD strips generated bodies; bodyless
  HEAD/304 responses may advertise representation length. Unsupported trailers
  and informational status protocols are refused by this buffered operation.
- `googleidentity` receives an admitted Exchange client and uses Google's SDK
  behind its transport/response controls. Strict RS256 header admission precedes
  certificate acquisition. The handoff documents real TLS/RSA evidence and
  explicitly unavailable live Google impersonation.
- `gotoolchain` overlays stream typed delete/replace mappings. The obsolete
  deletion-only request path is removed. The comment now describes both actions.
- `compass/config.json` advances to 2026.1.12, following the existing published
  v2026.1.11 tag; the prior config lagged that tag at 2026.1.10.

## Review corrections and evidence

The external review at `grok-review-1851dd8d.md` identified a missing JSON error
identity, missing HEAD coverage and the stale overlay comment. The JSON door
now uses the existing shared enum helpers instead of its duplicate decoder.
The exhaustive state domain and hostile decode cases check both error identities
and unchanged receivers. The response table checks body suppression, generated
length mismatch, bodyless representation lengths, malformed lengths and empty
responses through the actual buffer.

The new tests failed before those fixes. Raw red/green outputs are retained in
`/private/tmp/primitive-grok-review-{red,green}.log`. The initial red run was
filtered and lacked a separate full source snapshot; its output is supporting
development evidence, not independent acceptance.

The broader package runs and analyzers are retained under
`/private/tmp/primitive-review-20260904/`. Each recorded command has exact argv,
working directory, HEAD and dirty state, before/after source digests, toolchain,
selected environment, cache posture, exit status, stdout/stderr hashes and
emitted package/test outcomes. Earlier failures remain separate attempts.
The required scope is the complete `capabilities`, `sourceobservation`,
`exchange`, `googleidentity`, `gotoolchain` and `compass` packages. Global
`deadcode -test ./...` supplies all package test roots, so public operations used
outside a selected package are not incorrectly called dead.

Checks include go fix, uncached race tests, go vet, goconst with minimum length
four and tests excluded, errcheck, staticcheck, Witness lint and production
complexity at most ten. Semantic fuzz phases run serially with explicit budgets.
Errcheck also found and corrected a discarded response-body close error in the
handoff's Exchange test. The handoff's prior verifier fuzz and mutation evidence
remains separately described in `chunnel_verifier_handoff_20260904.md`.

## Limits and downstream work

These are local executions, not independent acceptance receipts. Emitted
test/subtest events are not a separately discovered independent-test denominator.
No benchmark or live provider acceptance is claimed. Committed-source validation
must identify the actual resulting commit.

The catalog's explicit scope includes tested exported method sets and named
function rules. Unlisted symbols remain unresolved during toolchain upgrades;
dynamic calls do not gain invented static targets. Four callable alternatives
are advertised; ownership alone does not imply a replacement exists.

Hammer owns compiler source inspection, findings, persisted reference/counter
verification and its dependency migration. Anvil owns when responses may be
released. Blink owns identity authorization and downstream acceptance. None of
those product policies moves into Primitive in this release.
