# Workloadidentity package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package
`workloadidentity`. It integrates the archived implementation, the
Primitive-internal dependency audit, and all four required consumer
interviews. The archive and consumer repositories were read-only. No archived
or consumer source was copied or changed.

The reconstruction decision is deliberately two-part:

- Primitive 2026 has evidence for one product-neutral capability: obtain a
  bounded Google service-account ID token for an exact outbound audience and
  carry it as a redacted bearer capability.
- The July 27 package must not be admitted unchanged. It collapses an
  unverified inbound bearer and a metadata-acquired outbound token into the
  same type, calls a lexical three-segment check canonical JWT validation,
  advertises principal projection it never performs, and accepts and rejects
  the wrong Google service-account principal forms.

The archive has good transport closure and redaction work worth preserving.
Its trust semantics and proof suite are not yet an admissible 2026 contract.
The reconstruction decision and its conditions are recorded below.

## Evidence boundary


| Source | Exact revision and Primitive pin | Workloadidentity tree | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | `9655ea4c9b2558c4eda27c44bb156d018eb82111` | One unrelated pre-existing untracked file, `core/api_http_boundary_hostile_test.go`; no archive file changed during this interview. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` | Committed-pin tree `4dc9589db8e5d626e3cb3921973dad94aa2b2cf5`; dirty pin is the exact archive tree `9655ea4c9b2558c4eda27c44bb156d018eb82111`. | Materially dirty tree, including the Primitive pin and unrelated product work. Kernel has no production import of the package at either observed state. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` | `e80bdff27a0dbaebb609e0c28376484e3e995920` | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` | `105f94d132d2cef88ac90c653fb863d413f50b8a` | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` | `ac1530f8345994574631ef164f8cda3ea66511ec` | `.ledger_pending.md` is modified; production and test sources inspected here match HEAD. |

All four consumer pins contain a Workloadidentity package, but none of the
three production users is pinned to the final archived API. Kernel's dirty pin
has the exact archive tree but Kernel does not call it. The source drift is
material:

| Consumer baseline | Diff from its pin to archived HEAD |
| --- | --- |
| Kernel committed pin | 7 files, 247 insertions, 84 deletions |
| Witness pin | 7 files, 263 insertions, 100 deletions |
| Bug pin | 8 files, 265 insertions, 100 deletions |
| Peachfuzz pin | 7 files, 247 insertions, 88 deletions |

The archive history is short but reveals substantial API churn:

1. `627f293cc4a1ac6790c81ec6c12ee0a2a75feb08` introduced the package on
   2026-07-18 as part of generalized release custody and workload evidence.
2. `4a57cd1e808c843b4fe312386600bfba9bb37125`,
   `c11c22e53ab6c6cef1b4cd70c1e67620c7e58151`, and
   `8c20a20138919f725269dd5a4d820bdf7081b77e` evolved doctrine, Peachfuzz,
   and Exchange integration through 2026-07-22.
3. `ecb0da7340f97e8ab8a80362fab053ac9473c6b7`,
   `156d4399c10003f53378b7ccbfe4cf5369684fbb`, and
   `cf8fa882ee56bb263220a1466c6deae08e466018` consolidated the runtime and
   Exchange boundaries through 2026-07-24.
4. `d259789e87bcadb829c5ffac72c6c91ccc604098` moved package facts and the
   error identity into Core on 2026-07-25.
5. `40ded9c104a99cbc4b0b672cd7392901b468d1eb` added token formatting
   redaction and hostile disclosure tests on 2026-07-26.

The archive index records the package as product-neutral Google workload
identity (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:41-50`). The completion ledger only
records that a spec was written or the package should be retired
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:503-507`); it does not provide a package-specific
acceptance review.

## Capability ownership

The implementation owns four separable mechanics:

1. a private-representation `Token` that carries a bounded bearer string,
   revalidates on explicit disclosure, and redacts every generic formatting
   surface (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:12-15`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:29-64`);
2. a private-representation `Principal` parser for one email-shaped subset
   ending in `.iam.gserviceaccount.com`
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:17-27`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:66-81`);
3. a `Source` that performs one bounded Exchange request against Google's
   default-service-account metadata identity endpoint for a typed API audience
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:18-76`); and
4. a package-owned `TokenSource` interface that allows another package to
   provide tokens (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:13-16`).

The declared boundary is broader. The specification says the package acquires
a metadata-server identity token, validates its canonical JWT shape, and
projects an exact Google service-account principal
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/SPEC.md:3-5`). It excludes `gcloud`, caching,
refresh, project selection, authorization decisions, user credentials, and
OGS principal mapping (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/SPEC.md:7-11`). Its stable
surface includes `ParseAuthorization` and `ParsePrincipal` beside outbound
acquisition (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/SPEC.md:13-14`).

The honest 2026 capability is narrower:

> Obtain one bounded Google-issued service-account ID-token bearer from the
> metadata server for one exact outbound service audience, and disclose it
> only at an explicit authorization-header boundary.

That operation does **not** authenticate an inbound request. It does not prove
signature, issuer, audience, expiry, subject, email, or authorization. It does
not project a principal. It does not make an arbitrary three-segment bearer a
Google identity token.

Those distinctions must be compiler-visible, because the Kernel gate treats
verified workload identity and exact audience as a request-authenticity
guarantee (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_gate.go:43-46`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_gate.go:86-100`). A type that merely proves
three dot-separated lexical segments cannot satisfy that ingress meaning.

## Archive evidence

### Archived dependency direction

`go list` at archived HEAD reports only standard-library imports plus `core`,
`exchange`, and `temporal`:

```text
core -> workloadidentity
temporal -> exchange
core -> exchange
exchange -> workloadidentity
```

This direction is good:

- Core supplies universal validation, typed HTTP/audience values, byte counts,
  authorization-header vocabulary, redaction text, and the Primitive error
  root.
- Temporal constructs the typed five-second attempt budget.
- Exchange owns `net/http`, bounded response reads, redirect refusal, request
  semantics, retry mechanics, and typed status/transport errors.
- Workloadidentity owns only the Google-specific composition over those
  primitives.

The archive's capability architecture ratchet explicitly rejects a raw
`*http.Client` or `http.RoundTripper` in
`workloadidentity/source.go`, requiring Exchange ownership instead
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/capability_architecture_test.go:263-270`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/capability_architecture_test.go:434-441`).

No product, release, evidence, persistence, process, secret manager, Kernel,
Bug, Witness, or Peachfuzz package is imported by Workloadidentity. The
low-level DAG therefore has no product back-edge.

There is one Core-placement concern for 2026. Every Google-only header, query
name, suffix, endpoint, and package limit currently lives in Core
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/workloadidentity_constants.go:3-13`), even though the Primitive
production scan found no package other than Workloadidentity consuming those
facts. Compiler ownership does not require universal Core ownership.
Package-private Google facts should remain in the package unless a second
Primitive package genuinely shares them. Core should retain only universal
HTTP types and the Primitive error root.

### Archived architecture worth retaining

### Redacted bearer capability

`Token` has a private string representation and no ordinary string method.
Its `fmt.Formatter` always emits the Core redaction value, including for the
zero token. The only disclosure paths are explicit methods that first
revalidate the token (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:44-64`).

The hostile redaction helper exercises generic formatting surfaces and proves
the zero value does not disclose shape
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:10-18`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:48-55`).
The explicit crossing test independently proves `BearerValue` and `Assertion`
still return the intended capability
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:20-46`).

This is the archive's strongest reusable design. Bug, Witness, and Peachfuzz
all need `BearerValue`; no production consumer found in the four repositories
calls `Assertion` or `ParseAuthorization`. Those two broader surfaces can be
removed unless a separately justified contract appears.

### Typed, bounded metadata request

`Source.Token` validates the Exchange capability and audience, rejects a nil
context, constructs a typed five-second duration, and builds one GET request
with:

- the fixed Google metadata identity target;
- `Metadata-Flavor: Google`;
- exact typed audience query encoding;
- `format=full`;
- single-attempt replay semantics; and
- expected status 200.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:23-55`.

The policy adds a one-byte request ceiling, a 16 KiB response ceiling, default
retry mechanics constrained by single-attempt semantics, and redirect refusal
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:56-75`). This is meaningfully stronger
than the older raw-`net/http` copies still pinned by Witness and Bug.

Google's current primary documentation confirms that metadata-server ID
tokens are for the attached service account, the requester chooses the
audience, and requests use the metadata identity endpoint with
`Metadata-Flavor: Google`: [Get an ID token](https://docs.cloud.google.com/docs/authentication/get-id-token).

### Typed error preservation

The package has one stable Core-owned error identity,
`core.ErrWorkloadIdentityContract`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:175-186`). Local failures wrap it while
joining the underlying Exchange or nil-context identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:107-109`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:23-29`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:37-42`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:67-74`).

The source tests use `errors.Is` for the package and Exchange identities and
`errors.As` for typed HTTP status evidence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:65-92`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:121-128`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:161-177`).
That is aligned with the testing protocol's strongest-error ordering
(`foundation@working-tree:_docs/testing_protocol.md:644-659`).

### Production-path transport tests

The positive source test drives the real `Source.Token -> Exchange ->
http.RoundTripper` path. It independently checks the exact target, query, and
request header before returning a response
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:27-63`).

Separate tests prove:

- oversized response refusal and both stable error identities
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:65-93`);
- redirect refusal without reaching the redirected target
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:95-132`); and
- one attempt on HTTP 503 plus typed actual/expected status
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:134-181`).

These are real behavioral tests, not direct helper theater. They meet the
protocol's production-path honesty rule for the transport slice
(`foundation@working-tree:_docs/testing_protocol.md:862-891`).

### Primitive-internal dependents

The archived Primitive production scan found **zero direct importers** of
`workloadidentity`.

The only out-of-package Go references at archived HEAD are:

- Core error/constant/governance declarations; and
- synthetic capability-architecture fixtures that use the package path to
  prove raw HTTP capabilities are forbidden.

There is no archived release, submission, receipt, objectstore, evidence, or
composition-root path that calls `Source`, accepts `TokenSource`, or consumes
`Token` or `Principal`. The package is therefore an orphan support package
inside the final archive.

This does not prove the capability is unnecessary. It does mean the archived
package has no Primitive-local end-to-end consumer proof and cannot inherit
admission merely from existing in the archive. The actual demand evidence is
external and version-skewed.

## Consumer evidence

### Kernel: policy vocabulary, not implementation

Kernel has no direct production or test import of Workloadidentity. Its use is
conceptual:

- `GateWorkloadIdentity` declares that machine-to-machine ingress has already
  authenticated an exact workload assertion and exact audience before a
  handler touches protected material or mutates state
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_gate.go:43-46`);
- Kernel counts that gate as a request-authenticity guarantee
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/route_gate.go:86-100`); and
- Switchboard renders the boundary as `workload identity + exact audience`
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/switchboard_contracts.go:12-26`,
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:switchboard/switchboard.go:660-683`).

**Local gem:** Kernel has already separated route classification from handler
implementation. The closed gate enum and Switchboard projection are useful
policy vocabulary.

**Implication:** Kernel's declared guarantee is much stronger than archived
`ParseAuthorization`. The gate needs a verifier that checks signature,
issuer, exact audience, expiry, and permitted principal before classification.
The archive supplies no such verifier. Admitting its `Token` as evidence for
this gate would be a security-category error.

Kernel's own governance matrix independently requires signature, issuer,
audience, expiry, and nonce validation before trusting ID-token claims
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/governance/crud_flow.md:517-532`). Workloadidentity cannot be
described as the implementation of `GateWorkloadIdentity`.

### Witness: preserved release-broker composition, not approved live API

Witness's `protocol/release` copy imports Workloadidentity and composes:

```text
TokenSource.Token
  -> Token.BearerValue
  -> exact release-data endpoint
  -> typed idempotent request
  -> echoed typed response
```

The source obtains one token, puts it in a `ReleaseDataClient`, validates the
endpoint/token/product tuple, and sends it through Exchange
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/release_data_client.go:14-71`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/release_data_client.go:95-125`).
It rejects a response that does not echo the exact request
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/release_data_client.go:74-92`).

**Local gem:** the release broker composition binds identity acquisition,
typed product request, idempotency key, and exact response echo. That complete
handoff is stronger than treating possession of any bearer as sufficient.

**Qualification:** Witness explicitly labels the protocol directory a
preservation copy, not an approved final API
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/FOUNDATION_EXTRACTION.md:1-7`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/FOUNDATION_EXTRACTION.md:25-36`). Its compile boundary
is intentionally not green until Primitive is repinned and vendor is
regenerated (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/FOUNDATION_EXTRACTION.md:47-53`). This is
archaeological design evidence, not current production cutover proof.

### Bug: real metadata acquisition for release authority

Bug has the clearest production use. `newReleaseEnvironment` constructs a
Workloadidentity metadata source whose audience is the exact Primitive
release-data endpoint, then injects it into the release-data composition
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/release.go:35-55`). A focused test pins the identity source's
audience to the broker endpoint and product-specific endpoint set
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/release_test.go:23-46`).

The release authority document says Cloud Build obtains an exact-audience
Google identity token, submits a typed request, and accepts release secrets
only after the response validates and echoes that entire request
(`bug@39ce96242240d7174d562c90bb255860946595dc:build_and_release.md:32-45`). The build runs as the dedicated
`release-signer@offgridsoftware.iam.gserviceaccount.com` account and does not
receive the secrets through environment injection
(`bug@39ce96242240d7174d562c90bb255860946595dc:cloudbuild.release.yaml:1-12`, `bug@39ce96242240d7174d562c90bb255860946595dc:cloudbuild.release.yaml:18-41`).

**Local gem:** the exact audience is not a loose string at the composition
root; it is the same typed endpoint used by the release-data client. The
broker, not the build, remains the sole Secret Manager reader, and the response
echo binds returned custody material to the exact typed request.

**Version warning:** Bug's pin still has the old `Source{HTTP, Audience}`
surface and raw HTTP release client. The archive has
`Source{Exchange, Audience}`. Bug has not compiled or tested against the final
archive tree.

### Peachfuzz: local impersonation, cache, and evidence publication

Peachfuzz consumes the shared token and principal types more deeply than it
consumes the metadata source.

Its local `IdentitySource` intentionally owns the behavior excluded by the
archive specification:

- invoke `gcloud auth print-identity-token`;
- impersonate one parsed service-account principal;
- bind the exact audience and request inclusion of email;
- bound command output;
- cache the token;
- singleflight concurrent refresh;
- allow a waiter one retry when the refresh leader's context ends; and
- implement Workloadidentity's `TokenSource`.

See `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:20-47`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:124-177`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:179-235`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:345-356`.

The app parses the configured Peachfuzz service-account principal, builds that
identity source for the exact run-evidence upload endpoint, and injects it into
the publisher (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:173-195`). The publisher
obtains a token, explicitly creates the bearer value, and sends it on the
typed evidence-authorization request
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:21-41`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:76-112`).

Peachfuzz also reuses `Principal` to configure an impersonated access-token
source for GCS (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/configured.go:13-33`).

**Local gem:** Peachfuzz's typed refresh-ownership state and shared
`refreshFlight` prevent one failed refresh from fanning out into one `gcloud`
process per waiter. The product correctly owns process execution, cache policy,
and refresh concurrency while depending on a shared opaque token type.

**Version warning:** Peachfuzz's pin uses removed exported facts such as
`TokenMaxBytes` and `GoogleServiceAccountEmailSuffix`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:20-27`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/endpoint.go:8-15`). It has not migrated to the archive's
Core-owned names. `ParseAuthorization` and `Assertion` appear only in tests;
the production publisher needs `BearerValue`, not an inbound parser.

## Strong mechanics and proof

The four repositories demonstrate three distinct concerns:

| Concern | Current owner/evidence | 2026 disposition |
| --- | --- | --- |
| Outbound metadata token acquisition | Bug production; Witness preserved release composition | Shared Primitive mechanic after redesign and migration proof |
| Outbound local `gcloud` impersonation/cache | Peachfuzz production | Product-owned; it composes over the shared token type |
| Inbound verification and route authenticity | Kernel gate vocabulary only | Kernel-owned until a second real verifier consumer proves Primitive-level sharing |
| Product authorization and release/evidence workflow | Bug, Witness, Peachfuzz | Consumer-owned composition and policy |

This split validates the archive's exclusions around process, cache, refresh,
and authorization. It invalidates the archive's decision to place inbound
`ParseAuthorization`, outbound acquisition, and a purported Google principal
projection behind one undifferentiated `Token` contract.

### Testing-protocol proof reproduced

The following read-only gates were run against archived HEAD during this
interview:

```text
go test ./workloadidentity -count=1
    PASS
go test -race ./workloadidentity -count=1
    PASS
go test ./workloadidentity -shuffle=on -count=10
    PASS
go vet ./workloadidentity
    PASS
staticcheck ./workloadidentity
    PASS
go test -coverprofile=... ./workloadidentity -count=1
    PASS, 87.3% statement coverage
```

Coverage details:

| Surface | Coverage |
| --- | ---: |
| `ParsePrincipal`, `ParseToken`, `ParseAuthorization`, token formatting, lexical helpers | 100% |
| `Token.BearerValue`, `Token.Assertion` | 66.7% |
| `Source.Validate` | 60.0% |
| `Source.Token` | 73.7% |
| Package total | 87.3% |

This proves the current tests are deterministic under repetition, race-clean,
and connected to the real local transport path. It does not prove the
contract is semantically correct. Coverage reaches the lexical parser while
the parser itself accepts non-JWT material.

#### Protocol-aligned evidence present

- The tests assert behavior that can regress: target, query, request header,
  redirect, attempt count, body ceiling, typed status, redaction, and explicit
  disclosure (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:27-181`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:10-55`).
- Stable failures use `errors.Is` and `errors.As`, not substring matching
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity_test.go:13-100`,
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:65-181`).
- The source path uses the actual production implementation over an injected
  RoundTripper rather than retesting copied request-building logic
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:27-63`).
- The capability architecture fixtures are valid structural ratchets under the
  protocol's structural-invariant rule
  (`foundation@working-tree:_docs/testing_protocol.md:1008-1041`).

#### Protocol gaps

The three parser tables have only:

- Token: one expected-valid and six expected-invalid cases;
- Authorization: one expected-valid and five expected-invalid cases; and
- Principal: one expected-valid and seven expected-invalid cases.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity_test.go:13-100`.

These are serious trust-boundary parsers, but they do not meet the protocol's
10-valid/10-invalid/20-boundary default, exact boundary/one-below/one-above
requirements, or hostile parser expectations
(`foundation@working-tree:_docs/testing_protocol.md:382-429`, `foundation@working-tree:_docs/testing_protocol.md:466-516`).

Missing proof includes:

- exact 16 KiB token acceptance and one-below behavior;
- token segments whose lengths are impossible base64url encodings;
- segments that decode but are not JSON objects;
- canonical versus padded/non-canonical base64url;
- control characters other than ASCII space at Authorization ingress;
- tab, carriage return, line feed, repeated whitespace, and scheme variants;
- valid service-account and project IDs at exact length floors and ceilings;
- leading/trailing hyphens, numeric starts, restricted project strings, and
  alternate valid Google default-account domains;
- zero/invalid `Source`, nil context, already-cancelled context, deadline,
  transport error, empty body, malformed body, exact response ceiling, and
  metadata-response provenance behavior;
- `BearerValue` and `Assertion` refusal on zero tokens; and
- a live or hermetic authentic Google JWT fixture.

The positive token fixture is the literal
`header.payload.signature`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity_test.go:11`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity_test.go:20`). Its final segment has
length 9, which is impossible for unpadded base64url, and none of its segments
is proved to decode to a JWT JOSE header, claims object, or signature. The
test therefore ratchets the wrong green behavior.

The package owns an external protocol/trust boundary but has no data-flow
struct inventory for `Token`, `Principal`, `Source`, or its target capability.
The protocol requires such an inventory for trust-boundary packages
(`foundation@working-tree:_docs/testing_protocol.md:1046-1098`).

There is no fuzz target. Fuzzing is a `should`, not a `must`, but the lexical
token, Authorization, and principal parsers are suitable boundaries if they
have independent semantic oracles
(`foundation@working-tree:_docs/testing_protocol.md:1210-1249`). A panic-only target would not close
the semantic defects below.

## Defects and blockers

### B0: one type collapses unverified ingress and acquired outbound authority

`ParseAuthorization` strips `Bearer `, rejects only the presence of an ASCII
space in the remainder, and returns the same `Token` type produced by the
metadata `Source` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:29-44`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:67-75`).

`validJWT` checks only:

- exactly two dots;
- nonempty segments; and
- characters in `[A-Za-z0-9_-]`.

It does not base64url-decode a segment, require canonical encoding, parse JOSE
or claims JSON, verify a signature, restrict an algorithm or key, validate
issuer, compare audience, enforce expiry, bind a subject/email, or return a
principal (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:84-105`).

Google documents service-account ID tokens as JWTs with issuer, audience,
subject, expiry, and signature meaning: [Token types](https://cloud.google.com/docs/authentication/token-types).
Kernel separately requires full ID-token validation before trusting claims
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/governance/crud_flow.md:517-532`).

The compiler cannot distinguish:

```text
metadata-acquired bearer for outbound forwarding
```

from:

```text
attacker-supplied three-segment Authorization value
```

Both become `workloadidentity.Token`. That is an inadmissible trust-state
collapse. `ParseAuthorization` must be removed from the outbound package or
must return a distinct `UnverifiedAssertion` that cannot satisfy an outbound
or verified-identity contract.

### B0: the canonical JWT shape claim is false

The specification promises canonical JWT-shape validation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/SPEC.md:3-5`). The implementation does not even
prove valid base64url encoding. For example, it accepts `a.b.c`; an unpadded
base64url segment of length 1 cannot decode.

The package's own accepted fixture ends in a nine-character segment and is not
a valid unpadded base64url encoding. The green test proves lexical compact
shape, not a JWT (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity_test.go:11-26`).

The contract must choose one honest meaning:

- an opaque bounded bearer obtained from a trusted acquisition source, with no
  claim to JWT validation; or
- a syntactically decoded JWT structure, still explicitly unverified; or
- a cryptographically verified Google identity assertion with separate typed
  issuer/audience/time/principal evidence.

The outbound use only needs the first. Calling it canonical JWT validation
adds a false security signal without helping any production consumer.

### B0: advertised principal projection does not exist

The specification says the package projects an exact Google service-account
principal (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/SPEC.md:3-5`). No production function
accepts a token and returns a principal.

`Source.Token` returns only `Token`. `ParseToken` never decodes claims.
`ParsePrincipal` parses a caller-supplied string independently
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:17-44`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:33-75`).

Peachfuzz confirms the actual use: it parses a configured principal first,
then passes that principal to `gcloud --impersonate-service-account` and
separately parses command output as a token
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:192-223`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:173-195`).

The specification must stop claiming projection. If inbound verification is
later admitted, a verifier may return a distinct `VerifiedIdentity` containing
the verified principal and exact audience.

### B0: `Principal` is neither exact nor generic Google service-account syntax

`ParsePrincipal` requires the `.iam.gserviceaccount.com` suffix and permits any
nonempty lower-alphanumeric-or-hyphen local and project components up to 254
total bytes (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:17-22`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:66-81`).

That over-accepts impossible user-managed identities such as components that:

- are shorter than 6 or longer than 30 characters;
- begin with a digit or hyphen;
- end with a hyphen; or
- use a project ID that cannot exist.

Google's service-account creation contract requires a 6-to-30-character account
ID matching `[a-z]([-a-z0-9]*[a-z0-9])`:
[projects.serviceAccounts.create](https://docs.cloud.google.com/iam/docs/reference/rest/v1/projects.serviceAccounts/create).
Google's project contract requires 6 to 30 characters, a leading lowercase
letter, and no trailing hyphen:
[Create projects](https://docs.cloud.google.com/resource-manager/docs/creating-managing-projects).

It also under-accepts valid Google service-account principals. Google
documents App Engine default accounts as
`PROJECT_ID@appspot.gserviceaccount.com` and Compute Engine default accounts as
`PROJECT_NUMBER-compute@developer.gserviceaccount.com`:
[Types of service accounts](https://docs.cloud.google.com/iam/docs/service-account-types).

The contract must be renamed and narrowed to
`UserManagedServiceAccountEmail`, with exact component rules, or expanded into
a closed set of typed Google account forms. The current generic `Principal`
name and exact Google service-account principal claim are both false.

### B1: metadata response provenance is unexamined

The positive source test returns status 200, an empty response header map, and
the fake lexical token; `Source.Token` accepts it
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source_test.go:33-63`). Production checks the
request's `Metadata-Flavor` header but imposes no response-header or
environment provenance rule (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:44-75`).

Google requires the request header and now offers both HTTP and preview HTTPS
metadata endpoints:
[View and query VM metadata](https://docs.cloud.google.com/compute/docs/metadata/querying-metadata).

This interview does not assert that every successful identity response must
carry a specific response header. It records the unresolved security decision:
the 2026 package must document how it knows a plain-HTTP response came from the
intended metadata service, prove the chosen rule, and include a hostile
headerless/spoofed-response case. The current test silently chooses status plus
lexical body as sufficient.

### B1: the public audience domain is narrower than Google's

`Source.Audience` is `core.APIEndpoint`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:18-21`). This is a valuable exact typed
binding for Bug and Peachfuzz, whose audiences are API URLs. Google service
account ID-token audiences can be freely chosen identifiers, however
([Token types](https://cloud.google.com/docs/authentication/token-types)).

The 2026 package must either:

- explicitly scope itself to HTTP service audiences and name the type/surface
  accordingly; or
- define a package-owned validated Google `Audience` type.

It must not imply generic Google ID-token acquisition while rejecting legal
non-URL audiences by an undocumented borrowed type.

### B1: `TokenSource` ownership is inverted

The package owns `TokenSource` even though Workloadidentity's production source
does not consume the interface; consumers do
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:13-16`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:84-87`).

Witness's release source and Peachfuzz's evidence publisher are the actual
consumers (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/release_data_client.go:14-21`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/evidence_http.go:21-26`). Peachfuzz must import and
implement the provider-owned interface merely to inject its product-owned
`gcloud` source (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:30-40`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:356`).

For 2026, each consuming composition should own its narrow interface returning
the shared outbound token type. That lets Primitive add or remove acquisition
implementations without making product process adapters implement a
provider-owned abstraction.

### B1: final archive has no migration or effect-path proof

The final archive has zero internal production consumers. Bug and Peachfuzz
are real users but remain pinned to earlier, source-incompatible surfaces.
Witness is a preserved unapproved copy. Kernel does not call the package.

Therefore:

- the exact archive API has no downstream compile proof;
- the exact archive redaction change has no consumer test proof;
- the `HTTP -> Exchange` source migration has no product cutover proof; and
- the Core constant migration already breaks Peachfuzz's pinned names.

Admission needs clean consumer migrations, not compatibility aliases. The
consumers must update to the chosen 2026 contract and compile/test against the
same reviewed Primitive revision.

### B1: proof suite does not meet the trust-boundary protocol

The green gates and 87.3% coverage are useful but insufficient. The serious
parsers do not meet the hostile table floors, exact thresholds are absent,
the accepted token fixture is invalid, Source failure branches are largely
unproved, and the required data-flow inventory is missing.

The testing protocol says a test must prove behavior that can fail, reach the
real branch, and name incomplete proof honestly
(`foundation@working-tree:_docs/testing_protocol.md:149-226`). The current suite calls lexical material
a valid token and calls its shape JWT validation. That is not an uncovered
line problem; it is a wrong oracle.

## Primitive 2026 ownership and DAG

### Admit one narrow outbound leaf

The clean dependency direction should be:

```text
core -> google/workload token acquisition
temporal -> exchange
core -> exchange
exchange -> google/workload token acquisition
google/workload token acquisition -> Bug release composition
google/workload token acquisition -> Witness release composition
google/workload token acquisition -> Peachfuzz evidence composition
Peachfuzz evidence composition -> product-owned gcloud/cache
```

The package name may remain `workloadidentity` if its scope is explicit, but a
Google-specific name would be more honest if Primitive expects other identity
providers later.

The admitted leaf should own:

- a redacted, private-representation **outbound** ID-token bearer;
- one explicit authorization-header disclosure;
- one exact audience type or a deliberately documented API-endpoint audience;
- a metadata-source constructor over Exchange;
- fixed Google metadata request facts;
- a single bounded request, redirect refusal, and response ceiling;
- stable package-owned error identities; and
- hostile tests with an authentic token fixture or an honest opaque-token
  contract.

It should not own:

- `ParseAuthorization`;
- an inbound-authentication claim;
- product authorization;
- process execution;
- token caching or refresh scheduling;
- release/evidence workflow;
- generic Google principal claims it cannot model exactly; or
- a provider-owned interface used only by consumers.

### Keep ingress verification a separate trust state

If multiple products later need shared Google assertion verification, the
types must make the state transition visible:

```text
Authorization bytes
    -> UnverifiedAssertion
    -> Verify(signature, issuer, exact audience, time)
    -> VerifiedIdentity{Principal, Audience, Issuer, ExpiresAt}
    -> consumer authorization policy
```

`UnverifiedAssertion`, outbound `Token`, and `VerifiedIdentity` must not be
aliases or interchangeable structs. The verifier should use Google's supported
verification primitives or independently pin the full key-fetch, cache,
issuer, algorithm, audience, and temporal contract. Authorization remains with
the product.

Until a second real verifier consumer exists, Kernel should own the verifier
behind `GateWorkloadIdentity`; Primitive should not build an orphan ingress
framework merely because it owns outbound acquisition.

### Preserve the consumer gems at their owners

- Kernel retains the closed route gate and authenticity classification.
- Bug retains exact endpoint/product/request/response-echo release
  composition.
- Witness retains only its genuinely Witness-owned release workflow after its
  preservation copy is adjudicated.
- Peachfuzz retains typed process execution, impersonation flags, bounded
  output, cache TTL, singleflight refresh, waiter retry, and evidence-upload
  policy.
- Primitive supplies only the shared redacted token and approved outbound
  acquisition mechanics.

This produces a one-way DAG and avoids moving product policy or hidden process
capability into Primitive.

### Admission conditions

The capability can be admitted only after all of the following are satisfied:

1. Replace the false canonical JWT contract with an honest opaque outbound
   token contract, or implement and test the exact stronger semantics.
2. Remove `ParseAuthorization` from the outbound surface, or return a distinct
   unverified type that cannot satisfy verified or acquired-token contracts.
3. Remove the false principal-projection claim.
4. Narrow and rename `Principal` to exact user-managed service-account email
   syntax, expand it into a closed complete Google model, or remove it from
   this package.
5. Decide and document the audience domain.
6. Decide and prove the metadata response/environment provenance rule.
7. Move Google-only facts out of Core unless another Primitive owner truly
   shares them.
8. Move source interfaces to the consumers unless a provider-side use is
   demonstrated.
9. Replace the invalid JWT fixture and add hostile parser/source tables,
   exact boundaries, cancellation/transport/body cases, stable error proofs,
   and the required data-flow inventory.
10. Migrate Bug, Witness, and Peachfuzz without aliases or shims and run their
    focused production-path tests against the exact reviewed Primitive
    revision.
11. Prove Kernel's gate is backed by a real verifier or explicitly keep that
    implementation outside this package.
12. Rerun focused tests, race/shuffle, vet, staticcheck, package architecture
    ratchets, and consumer gates.

## Decision rationale and conditions

**Retain the capability. Reject the archived package as-is.**

The positive evidence is real:

- Bug has a production metadata-token use protecting release material.
- Peachfuzz has a production shared-token use with a strong product-owned
  impersonation/cache implementation.
- Witness preserves a second release-broker composition.
- Kernel establishes the receiving-side need for exact workload identity and
  audience verification.
- The archive closes raw HTTP behind Exchange, bounds effects, preserves typed
  errors, and strongly redacts bearer formatting.

The blockers are also real:

- unverified ingress and acquired outbound tokens are the same type;
- canonical JWT validation accepts values that are not valid JWT encodings;
- principal projection is absent;
- principal syntax is both over- and under-inclusive;
- the archive has no exact downstream migration proof; and
- the hostile proof suite has a wrong positive oracle and falls short of the
  current testing protocol.

Primitive 2026 should reconstruct the narrow outbound leaf from first
principles, preserve the redaction and bounded Exchange mechanics, and leave
verification, process/cache policy, and product authorization with their
proper owners until genuine cross-product evidence justifies another shared
primitive.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
