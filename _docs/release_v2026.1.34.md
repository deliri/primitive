# Primitive v2026.1.34

ID bounds UUID and ULID JSON before decoding and rejects pre-epoch observations
at the shared request boundary. UUID parsing delegates decoding to Go's uuid
package after checking canonical spelling. No generation runtime, clock or
entropy acquisition, pools, dependencies, or production structs were added.

Hostile tables cover byte positions, all identity bits, JSON limits, receiver
preservation, allocation budgets and destruction races. Independent fuzz oracles
check semantic admission and exact results. Compiled API inventories reject
accidental UUID generation and clock acquisition.

The final Linux race run passed 609 test events with no failures or skips and
97.7% statement coverage. Nine behavioral mutations were killed. Six final fuzz
campaigns passed 10,760,478 executions. Scoped go fix, Vet, Staticcheck, Errcheck,
Witness and complexity checks passed. macOS arm64 and Windows amd64 compiled;
those binaries were not executed.

Original and both candidate benchmark runs retain CPU/memory profiles and
matching binaries outside Git. UUID parsing fell from 48 B/op and one allocation
to zero; JSON decoding fell from 64 B/op and two allocations to 16 B/op and one.
Some latency observations worsened. Shared-host single samples do not establish
an overall speedup or statistical performance acceptance.

The user approved continuing after the ID review surface. Reviewed Go sources
remain unchanged. Only release coordinate tests are repeated for this checkpoint.
Full-module gates and consumer updates were not run. Distribution is next.

See [the reviewed report](id_upgrade_20260909.md),
[package evidence](id_upgrade_20260909_evidence.json), and
[release evidence](release_v2026.1.34_evidence.json).
