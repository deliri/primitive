# Primitive v2026.1.29

Adds the requested typed Tailnet capability using tailscale.com v1.102.3 and
its official API client v2.10.1. Product configuration stays in tailnetconfig;
Primitive owns pinned outbound transport, bounded enrollment over Exchange,
Temporal startup context, and SDK lifecycle. The existing Blink boot API is
preserved. Explicit Google service-account acquisition and exact audience text
encoding are included from the source integration branch.

Hardening rejects typed-nil/closed capabilities, unauthenticated enrollment,
contradictory returned auth keys, and invalid provider tags. Early SDK startup
failure preserves its native filesystem error without calling unsafe cleanup.
Core owns the new stable error identities.

The scoped Linux race run records 377 passing events, no failures or skips.
Four 30-second semantic fuzz campaigns pass. Scoped vet, Staticcheck, errcheck,
complexity, and Core error-contract checks pass; Darwin/Windows compile.
Witness retains one false positive: HTTP-server timeout fields demanded on
Tailscale's tsnet.Server. Full-module gates, native Darwin/Windows execution,
and live Tailnet enrollment were not run.

Before/after CPU and memory profiles and matching binaries are retained outside
Git. Lifecycle cost increases from 832 to 880 B/op and five to six allocations;
configuration validation remains zero-allocation. One shared-server timing pair
does not establish a timing trend. The user accepted the reported cost increase
and explicitly approved bump, commit, and push. Consumer pins remain unchanged.

See [the approved slice](tailnet_integration_20260909.md),
[run evidence](tailnet_integration_20260909_evidence.json), and
[release evidence](release_v2026.1.29_evidence.json).
