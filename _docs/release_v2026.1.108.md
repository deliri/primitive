# Primitive v2026.1.108 — typed native cache prefix purge

Parent release: v2026.1.107, commit
`b73545bf01631c5c590c6974734160c315205434`.

## Contract

`CacheServer.PurgePrefix` executes one Cloudflare native prefix purge with a
validated `CachePrefixPurgeRequest`. `CachePurgePrefix` is constructed from a
typed HTTP endpoint; query-bearing inputs fail instead of silently becoming a
broader scope. The provider's 31-path-separator ceiling is owned by core.
Request and receipt preserve the typed prefix and zone. Provider acceptance
remains distinct from observation of public absence.

The operation invalidates cache variants without enumerating origins, query
strings or headers and without retaining a local cache model. Prefix matching
also includes longer resource paths sharing that prefix. The caller owns this
scope. Kernel selects a full unique object path and independently observes
public delivery; Primitive does not select application resources or declare
them deleted. The exact-file and prefix methods are different current provider
operations, with distinct typed request/receipt contracts.

All JSON uses the existing bounded, strict JSON v2 API boundary. The request
contains one prefix. Cancellation, provider refusal, response bounds, resource
cleanup and single-attempt transport remain owned by the shared API machinery.

Provider source:
[Cloudflare native prefix purge](https://developers.cloudflare.com/cache/how-to/purge-cache/purge_by_prefix/).

## Evidence

Local root: `/private/tmp/primitive-cache-evidence-20261005`.
Every run has the exact source coordinate, complete command, toolchain,
cache posture, exit status, counts, stdout/stderr and artifact digests.

- `cache-prefix-red-01`: missing typed capability, expected build failure.
- `cache-prefix-green-01`: 16 passing focused test events, no failures/skips.
- `cache-prefix-packages-01`: 2,543 passing events across cloudflare, core,
  compass, version and authored Hammer claims; no failures/skips.

Release verification also requires clean committed package/race execution and
serial `FuzzCachePurgePrefixClosure` / `FuzzCachePurgeResponseClosure` runs.
Those receipts identify their exact revision separately from this source note.
The response fuzz target directly exercises both provider operations; it does
not adapt one receipt shape into the other. Fuzz budgets are three seconds per
target under the current local protocol, with minimization capped likewise.

The earlier v2026.1.107 fuzz budgets were 30 seconds and exceeded the local
three-second instruction. Those receipts remain historical, and that execution
discrepancy is not folded into protocol compliance. The initial prefix test
also overlapped an active Kernel JSON fuzz phase; no fully serial
cross-repository verification claim applies to that initial run.

This release does not claim a live Cloudflare purge, a Kernel browser pass,
repository-wide gates, deployment, or independent acceptance.
