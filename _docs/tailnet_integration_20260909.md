# Tailnet integration — approved release slice, 2026-09-09

The requested path is Blink Kernel's `cmd/api/anvil_tailnet_boot.go` → Primitive
`tailnet` → Tailscale `tsnet` v1.102.3. `tailnetconfig` is the pure, typed
configuration agreement; importing it in product configuration does not import
or initialize the Tailscale SDK. The API boot call shape is preserved:
`tailnet.NewClient(configuration, tailnet.GoogleIdentity{Client: identity})`,
then `Exchange()` and owned `Close()`.

The source branch was fetched with Git and merged without committing:
`origin/dsrv-tailnet-20260908`, commit
`0b98e1d52988916a5567717b6c5c06c2aab81240`, over the approved Attest release
`5fb274bc319ef79488ed59e03e41e0b1006f80cb` (v2026.1.28).
The merge was conflict-free. Attest was bumped, committed, atomically pushed,
and the remote main and peeled tag were verified. The user approved this slice for bump, commit, and push as v2026.1.29.
Consumer dependency pins remain unchanged. The measurements below precede
publication and bind to the recorded dirty source, not a future commit.

## Ownership and implementation

- Product code selects destination, identity, hostname, tag, state directory,
  and startup budget. Primitive owns a lazy outbound connection to exactly one
  address and port. It does not know Anvil, worker policy, completion, or jobs.
- Application HTTP uses Exchange and Go's `net/http.Transport`. Enrollment HTTP
  also goes through Exchange, with a 64 KiB response ceiling and the caller's
  bounded Temporal context. Configuration accepts only the provider address
  ranges, an explicit nonzero port, and a positive startup budget up to 60 seconds.
- `tailscale.com@v1.102.3` owns the mesh implementation. The official API client
  `tailscale.com/client/tailscale/v2@v2.10.1` owns workload identity exchange and
  auth-key creation. No OAuth protocol, JWT verifier, VPN, or network runtime
  was reimplemented. Both provider types stay inside the capability.
- The returned auth key must match the requested ephemeral, single-use,
  non-preauthorized tag capability and pass bounded credential admission.
  `tsnet` receives the explicit key and the SDK's standard control URL. The
  imported blank registration of `feature/identityfederation` was removed;
  its ambient-authentication path is not part of this implementation.
- Credential enrollment holds one gate, not a worker pool or retry engine.
  Two destination connections are the explicit transport ceiling. A client
  closes its transport and SDK node once. Nil and zero clients refuse safely;
  typed-nil identity sources cannot become deferred panics; closed clients do
  not issue new Exchange capabilities.
- Core owns four new typed error identities, appended to the existing closed
  domain without changing earlier ordinals. New scalar Parse/Validate/String
  contracts preserve exact text. There are no compatibility aliases for the
  imported package-local sentinels.
- The imported Googleidentity additions remain: exact audience text encoding
  and explicit service-account credential-file acquisition through Filestore
  and the Google SDK over Exchange. This is integration coverage for those
  additions, not a claim that all of Googleidentity received a package sweep.

The SDK still owns its internal node state files, sockets, timers, and shutdown
mechanics. Primitive adds no parallel filesystem or network implementation.
The startup context bounds cooperative identity/API work and readiness waiting;
like ordinary Go OS operations, synchronous SDK initialization is not a promise
of hard preemption of a blocked kernel call.

## Regressions and tests

The complete local `_docs/testing_protocol.md` was read, including its final
waiver and automation sections. Its SHA-256 is
`dd83cd7f62c172092546dab6b7c7d5c59753b5e8ae784631a94cff4d6d48126c`.

Retained red runs demonstrate:

1. Typed-nil identity admission and the nil/zero/closed Exchange capability
   boundary failed their expected ownership contract on the imported code.
2. The official API client's empty access-token response could lead to an
   unauthenticated key-creation request. The transport now refuses that request
   before the underlying HTTP effect. The actual SDK/local-provider test proves
   zero key requests on this refusal.
3. An early SDK filesystem failure followed by `Close` panicked inside `tsnet`
   because the SDK system had not initialized. The real SDK regression uses
   Filestore-created regular files at the state path and state parent. Explicit
   `Start` now lets the SDK roll back failed initialization; only a successfully
   started server is later closed after readiness failure. Both rows preserve
   the native `ENOTDIR` and return no retained server.

4. The imported tag validator reused hostname rules. It admitted digit-leading
   tags rejected by Tailscale and refused provider-valid uppercase/trailing-hyphen
   tags. Named red/green rows and the SDK's own `tailcfg.CheckTag` now independently
   pin the provider grammar; a separate 63-byte suffix limit remains explicit.

Six deliberate one-fact mutations fail the tests for context inheritance,
empty access credentials, ephemeral-key binding, control-server ownership,
pinned destinations, and struct inventory. Mutation sources and results are
retained and the original files were restored. The later startup failure has
its own actual red/green evidence rather than an additional synthetic mutation.

Hostile tables cover scalar byte limits, names and authority separators,
IPv4/IPv6 prefix edges, mapped addresses and zones, port extremes, temporal
limits, capability absence and closure, SDK token/key failures, body ceilings,
provider capability contradictions, context propagation, unsent request-body
closure, and SDK startup filesystem failures. Production struct inventories
classify capabilities, the one-operation transport, and the configuration intent.
No producer-to-classifier evidence lattice is introduced, so the protocol's
50-case classifier matrix does not apply.

Fuzz inventories:

| Ingress | Semantic target |
| --- | --- |
| ClientID/Hostname/Tag Parse, Validate and exact String; address admission; Configuration.Validate; NewClient | FuzzConfigurationIngress |
| Provider auth-key JSON and key/capability projection through the actual official SDK and Exchange | FuzzProviderAuthKeyAdmission |
| Audience.Parse/UnmarshalText and canonical text/JSON behavior | FuzzAudienceTextSemanticClosure |
| Service-account credential file bytes, SDK parsing and token acquisition | FuzzServiceAccountCredentialAcquisition |

Configuration fuzzing uses independent regex and address-byte oracles and checks
that constructor refusals return no capability and make no identity call. Provider
fuzzing requires exact key preservation and independent checks of key grammar,
byte ceilings, ephemeral/single-use flags, and the authored tag. Googleidentity
fuzzing proves receiver preservation and exact opaque local-provider output.
No fuzz callback contacts a live external service.

## Scope and limitations

The authoritative run results, commands, toolchain, source hashes, artifacts,
failed attempts, filters, coverage, and benchmark samples are in the companion
manifest. The source is dirty over the exact release commit; the evidence does
not pretend to come from a future integration commit or constitute independent
acceptance. Benchmark and fuzz phases are serial and occur after checks.

No live Tailscale enrollment, Google credential issuance, real-tailnet throughput,
native macOS/Windows execution, consumer deployment, or full-module gates were
performed. Darwin arm64 and Windows amd64 are compile checks only. The SDK tests
exercise its real request construction against owned local HTTP providers and its
real early filesystem-failure path. They do not prove live node readiness or
live node shutdown. Product consumer builds remain a published-release adoption
step; v2026.1.28 is the Attest release and does not contain Tailnet.

Witness lint's remaining finding is `doctrine/http/server_timeouts` on
`tsnet.Server`: it incorrectly demands fields belonging to `net/http.Server`.
The named fields do not exist on `tsnet.Server`. No alias, source disguise,
configuration suppression, or broad waiver was added to hide this analyzer issue.
This lint run remains failed in the evidence; other scoped analyzer results are
reported separately.

Earlier measurements completed on the candidate before the SDK startup repair.
Their orchestrator was stopped before launching further phases after the repair
was identified; the in-flight recorder was allowed to finish without source edits.
Those attempts remain historical. The final measurements have new labels and
current source bindings. Initial test build failures (a nonexistent decoder helper
and duplicated test constants) are also retained as development failures, not
production regressions.

Raw profiles, matching binaries, stdout/stderr, source snapshots and mutation logs
remain outside Git under
`/home/d/engineering-evidence/primitive/tailnet-integration-20260909`.

## Upstream references

- https://pkg.go.dev/tailscale.com@v1.102.3/tsnet
- https://pkg.go.dev/tailscale.com/client/tailscale/v2@v2.10.1
- https://tailscale.com/docs/features/workload-identity-federation

Pinned source for both SDKs was inspected locally. In particular, `tsnet.Up`
synchronously invokes startup, automatic identity federation uses the SDK's
shutdown context rather than the caller's startup context, and the API client's
identity token source constructs background HTTP requests. The per-enrollment
transport binds those requests to the owned context without a replacement runtime.

## Final validation and measurements

Final Linux race run: 377 passing events, 0 failures, 0 skips. Cache bypassed with `-count=1`.

```text
ok  	github.com/deliri/primitive/v2026/tailnet	1.093s	coverage: 81.3% of statements
ok  	github.com/deliri/primitive/v2026/tailnetconfig	1.069s	coverage: 100.0% of statements
ok  	github.com/deliri/primitive/v2026/googleidentity	1.858s	coverage: 81.7% of statements
```

Core ErrorIdentity contract tests, vet, Staticcheck, errcheck, and production complexity checks pass. Darwin arm64 and Windows amd64 compile. Witness retains the one false-positive SDK server finding described above.

| Compared operation | Before ns/op | Final ns/op | Before B/op | Final B/op | Before allocs/op | Final allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| BenchmarkClientLifecycle-8 | 1737 | 1875 | 832 | 880 | 5 | 6 |
| BenchmarkConfigurationValidate-8 | 191.7 | 199.6 | 0 | 0 | 0 | 0 |

The two compared benchmark bodies are unchanged. The final runner uses exact names to exclude the newly added enrollment case. Linux fixture values are unchanged; later fixture source handles Windows path spelling explicitly. Each comparison has one 30-second sample per phase, on a shared server. No timing improvement or regression trend is inferred from a single pair.

New local-provider SDK enrollment benchmark (no before-equivalent): `BenchmarkAuthKeyLocalProvider-8   	   71536	    497604 ns/op	         2.000 requests/op	   26946 B/op	     279 allocs/op`. It checks exactly one token exchange and one key request per iteration, and exact returned-key preservation. It excludes node startup and real-provider latency.

| Semantic fuzz target | Authoritative run | Budget | Result |
| --- | --- | --- | --- |
| FuzzProviderAuthKeyAdmission | provider-final-fuzz-FuzzProviderAuthKeyAdmission | 30 seconds, 4 workers | exit 0 |
| FuzzConfigurationIngress | provider-final-fuzz-FuzzConfigurationIngress | 30 seconds, 4 workers | exit 0 |
| FuzzAudienceTextSemanticClosure | fuzz-FuzzAudienceTextSemanticClosure | 30 seconds, 4 workers | exit 0 |
| FuzzServiceAccountCredentialAcquisition | fuzz-FuzzServiceAccountCredentialAcquisition | 30 seconds, 4 workers | exit 0 |

All raw attempts are retained. The two unchanged Googleidentity campaigns keep their original source bindings rather than being relabeled as fresh executions after the Tailnet-only grammar correction. Large binary/profile artifacts remain outside Git; their hashes and byte counts are in the manifest.

## Profile interpretation and approval

The final lifecycle allocation profile attributes 94.58% of sampled allocation
space directly to NewClient and 5.37% to Exchange.NewStandardClient. The original
lifecycle profile attributes approximately 100% to NewClient. The measured
increase of 48 B/op and one allocation is consistent with the additional owned
enrollment Exchange client; it is retained and accepted, not called a speedup.
Configuration validation remains zero-allocation. Its CPU profile spends time
in standard-library address parsing and string/UTF-8 validation plus the local
bounded identifier checks; no replacement parser or cache was introduced.

The local-provider enrollment CPU profile is led by Linux syscall and runtime
synchronization work (27.52% Syscall6, 11.87% futex). Allocation samples are led
by Go HTTP header cloning/parsing and request cloning. This benchmark includes
both the local HTTP server and client, so these are whole-fixture costs, not
isolated production-client attribution. The post-run in-use sample is dominated
by runtime and HTTP initialization; it does not establish peak memory, a live
node memory bound, or leak freedom. No speculative pooling or custom networking
was added in response to these samples.

All fifteen recorded pprof summaries completed successfully. Their exact commands,
matching binaries, raw profiles, and summary artifacts are in the evidence
manifest. Profile totals cover different iteration counts; per-operation costs
come from the benchmark output, not comparison of total allocated gigabytes.

User approval: “that's minor and it's ok. bump commit and push”. This accepts
the reported constructor cost increase and authorizes this release checkpoint.
The Witness false positive remains visible and failed; it is not converted to
a passing gate. Live Tailnet operation remains unverified as described above.
