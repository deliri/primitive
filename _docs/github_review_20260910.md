# GitHub admission and download ownership — v2026.1.64

This slice closes three mechanical boundary defects; the GitHub package sweep remains open.

- `NewAppClient` now validates its required App credential before construction. An empty or closed credential cannot silently construct a public client. `NewClient` remains the explicit public constructor. The existing independent credential copy is preserved.
- `TreeRequest.Validate` rejects absent and typed-nil visitors before provider requests. The test exhausts the nil-capable Go receiver kinds and distinguishes present empty receivers from nil values.
- `ReadTree` owns a child download context. A decoder or visitor refusal cancels the provider request before joining its worker. The provider cannot keep that join blocked merely by withholding the next bytes. Caller context ownership is preserved. Nil and already-cancelled contexts produce typed refusals without network or visitor effects.

Primitive returns exact provider observations. It does not decide repository meaning, readiness, release selection, or product completion. No new production struct, retained graph, workflow, compatibility layer, or global state was introduced.

## Behavioral evidence

The unchanged baseline `3d2892f6666faf1d2d12d9082afd8372200890ac` passed all GitHub tests. New admission tests failed on the empty App credential and five typed-nil receiver kinds. Real local HTTP tests reproduced both decoder-refusal and visitor-refusal stalls against baseline production. Those tests now observe request cancellation, join completion, exact visitor counts, zero completed observation on failure, and a still-usable caller context. A ten-second timeout is only a deadlock backstop; completion is demonstrated by returned facts and closed handler channels. Empty successful trees remain valid zero-entry observations.

The test layer map is constructor/credential admission, request validation, and the public HTTP/visitor handoff. Existing positive streaming tests continue to cover file/archive/tree transfer. No durable artifact, manifest, ledger, reporter, or CLI layer is changed. Partial visitor delivery remains possible before a later tree failure; a zero final observation is not a rollback of already delivered entries. Callers own consumption of those entries.

The real HTTP fixtures use a loopback test authority through the existing internal fixture constructor and the standard Exchange client. They do not contact live GitHub. Existing exported-constructor tests exercise the actual public App and public-client constructors.

## Retained execution facts

The server evidence directory is `/work/engineering-evidence/primitive/github-upgrade-20260910`. Its manifest and external verification report bind all recorded commands, attempts, output digests and byte counts, source snapshots, profiles, test counts, tree status and toolchain identity. The exact final commit is in that report. Test cache reuse is bypassed with `-count=1`. Failed red runs and the first lint run remain failures; they were not overwritten.

The new one-entry tree benchmark performs a real local HTTP transfer and observes exact output bytes, entry count, total visits and validation. Baseline and candidate use the same benchmark source, payload, toolchain, machine, CPU setting and 30-second duration, with CPU/memory profiles retained. The baseline checkout contains only the added benchmark source beyond the stated production revision. Machine power posture was not measured, and no accepted speedup or regression claim is made.

`errcheck -blank -asserts ./github` has five existing findings, reproduced against baseline: two fixture writes, invalid-client credential cleanup, a formatter write that cannot return an error, and the intentionally interrupted large-tree fixture write. Their filename and source-expression identities are compared with the candidate. These findings remain an explicit baseline, not a clean errcheck claim. Other scoped analyzers and full-module compilation are recorded separately. Independent execution acceptance, live-provider coverage, consumer builds, and whole-repository test success are not claimed.

## Remaining proof surfaces

The package is still first in the queue. Its tree decoder uses the standard JSON token decoder, which can allocate proportionally to a single large string, value or whitespace run; record-count streaming does not establish fixed memory. The next slice must bound or stream each representation before unbounded allocation, preserve synchronous delivery and complete/truncated distinctions, and add exact byte-boundary and allocation evidence. It must not restore an arbitrary total repository-size or entry-count ceiling.

The ingress inventory also still needs reconciliation for public reference/user-agent and credential admission, pagination headers, and the public tree HTTP boundary versus its direct decoder fuzz target. Existing direct decoder targets do not by themselves prove the entire transport path. Existing goroutine/credential lifecycle concurrency contracts and the unchecked-error baseline remain visible follow-ups. This release does not declare the full GitHub package sweep complete.
