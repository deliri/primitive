# Primitive v2026.1.33

Currency preserves native Go JSON integer syntax/range failures and exact typed
overflow identities without changing its wire shape or admitted domain. Decimal
conversion delegates to strconv; formatting uses fixed buffers and one final
string allocation. Overflow diagnostics use Core's existing identity directly.

Hostile tables now exhaust nominal code bytes, distinguish syntax from range,
check refusal results and receiver preservation, and independently verify digit
widths and int64 extrema. Runtime architecture scans use compiler-embedded source.
No new production structs, dependencies, policy, pools or runtime state were added.

The reviewed Linux race run passed 1,188 test events with no failures/skips and
94.6% statement coverage. Scoped Vet, Staticcheck, Errcheck, Witness and complexity
checks passed. macOS arm64 and Windows amd64 binaries compiled; they were not run.
Five final fuzz campaigns passed 8,023,958 executions. Prior attempts are retained.

Original, pre-review and post-review benchmark runs retain CPU/memory profiles,
binaries and exact source evidence outside Git. Formatting uses 24 B/op and one
allocation, down from 72 B/op and three. Several latency samples worsened; there
is no overall speedup or statistical claim from these shared-host observations.

The user approved bump, commit, push and moving to ID. Reviewed package sources
are unchanged at this checkpoint; only release coordinate tests are repeated.
Full-module gates, native macOS/Windows execution and consumer updates were not run.

See [the reviewed report](currency_upgrade_20260909.md),
[package evidence](currency_upgrade_20260909_evidence.json), and
[release evidence](release_v2026.1.33_evidence.json).
