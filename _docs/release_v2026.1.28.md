# Primitive v2026.1.28

This approved Attest follow-up preserves body callback error identity alongside
stream byte-limit refusals. Signature decoding uses a fixed Ed25519-sized buffer;
envelope encoding references its existing value copy. Canonical wire contracts
remain unchanged. The identical-harness comparison measures encoding at 27 versus
30 allocations and decoding at 47 versus 48. One shared-server timing pair does
not establish a latency improvement; both JSON timing regressions are retained.

Final ordinary tests passed with 685 passing events. Final race tests passed
with 682 passing events and 93.0% coverage; the allocation parent and two rows
are explicitly excluded from race builds. The initial incorrect use of ordinary
allocation budgets under race instrumentation remains recorded as a failed run.
Ten pre-fix failing stream rows demonstrate one error-loss defect family. Two
allocation mutations fail their respective budgets. All eleven 30-second fuzz
campaigns passed. Scoped vet, Staticcheck, errcheck, Witness lint, and production
complexity checks passed; Darwin and Windows compiled. Native Darwin/Windows
execution and full-module gates were not run. Earlier analyzer and cross-build
runs retain their exact source binding before the test-only race exclusion.

Before/after benchmarks retain explicit CPU and memory profiles and matching
binaries outside Git. Fresh Compass/Version tests verify the release coordinate.
See [the reviewed slice](attest_upgrade_20260909.md),
[its evidence](attest_upgrade_20260909_evidence.json), and
[release evidence](release_v2026.1.28_evidence.json).

The user explicitly approved bump, commit, and push. Consumer pins are unchanged.
The remaining-work queue now excludes previously upgraded packages; the user's
next requested capability is Tailnet, followed by the unfinished Controlplane
slice. This release contains no Tailnet implementation.
