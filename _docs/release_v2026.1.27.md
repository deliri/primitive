# Primitive v2026.1.27

This reviewed release hardens Chit's typed signing and manifest boundaries.
Canonical writers refuse typed-nil destinations; catalog issuance validates and
copies entries before signer callbacks; all twelve scalar JSON decoders enforce
Core's byte ceiling before parsing; authenticated nonempty manifests may contain
only empty objects. Entry-name validation uses Go's `strings.SplitSeq`, removing
the temporary component allocation without replacing Go's parsing machinery.

The final Chit Linux race run passed with 2,564 passing test events, no failures,
and no skips. Scoped vet, Staticcheck, errcheck, and Witness lint passed. Twelve
intentional production mutations failed their intended tests, including each
numeric decoder's early byte guard. Review 1944eb47 found zero bugs and two
suggestions; both are resolved.

The earlier slice evidence retains fifteen 30-second fuzz campaigns, two stronger
inventory reruns, Darwin/Windows compilation, production complexity checks, and
an identical-harness benchmark pair with CPU/memory profiles and matching binaries.
Entry-name boundary benchmarks dropped from one allocation to zero. Latency
samples on the shared server are not claimed as a proven general speedup.
The review follow-up changed only a test file among Go sources, so those earlier
runs retain their original source bindings and were not silently reclassified
as new runs. Full-module gates and native macOS/Windows execution were not run
for this slice. Fresh Compass/Version tests check the release coordinate.

See the [review and execution notes](chit_upgrade_20260909.md),
[original evidence](chit_upgrade_20260909_evidence.json),
[review follow-up evidence](chit_upgrade_20260909_review_followup.json), and
[release evidence](release_v2026.1.27_evidence.json).

The user explicitly approved the bump, commit, and push after review closure.
Raw profiles, binaries, logs, and source snapshots remain outside Git on the
server. Consumer dependency pins are unchanged by this release.
