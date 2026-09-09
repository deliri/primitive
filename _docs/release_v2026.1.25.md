# Primitive v2026.1.25

This user-reviewed checkpoint publishes Objectstore stream and evidence hardening
on Go 1.27.1. The supplied external review lists no issues.

ExactReader delegates directly to the caller-owned Go reader, checks native count
and error pairs, preserves final EOF and failure identity, and avoids a redundant
32 KiB buffer. Inspection rechecks cancellation before issuing proof. Typed-nil
streams refuse before effects; Core owns writer presence and Exchange consumes it.
Scalar JSON documents and browser projections enforce their owned limits. Transfers
and received evidence share provider extent and integrity validation, including
exact empty-content digests. BLAKE3 remains explicitly approved by the user.

The final Objectstore race run passed 2,081 test/subtest events, zero failures or
skips, and 87.9% statement coverage. The earlier combined Objectstore/Core run
passed 4,026 events. Eleven timed fuzz campaigns and two separately labeled
minimization diagnostics passed. Six before/after workload pairs retain explicit
CPU/memory profiles, matching binaries, and source snapshots. Upload allocation
fell from 40,330 to 7,284 B/op at 1 KiB and from 40,157 to 7,265 B/op at 10 MiB;
timing samples are not statistical trends.

Scoped analyzers passed. Exchange Witness remains unavailable due to an internal
`invalid doctrine report` failure. Linux/Windows compiled test binaries only on the
Mac; full-module gates were not repeated locally. Fresh Compass/Version coordinate
tests passed 72 events. The new server is the requested next execution environment.

See the [review](../objectstore/upgrade_review.md),
[benchmark comparison](../objectstore/upgrade_benchmark_after.md),
[package manifest](../objectstore/upgrade_evidence.json), and
[publication evidence](release_v2026.1.25_evidence.json).
Large raw profiles, binaries, logs, and source snapshots remain ignored locally.
The user explicitly approved bump, commit, and push; consumer pins are unchanged.
