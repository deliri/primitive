# Timeproof package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `timeproof`.
Primary archive, Primitive-internal, and all four consumer interviews are
integrated. The archive is evidence, not authority. No archived or consumer
source was copied or changed.

The reconstruction decision is deliberately two-part:

- Primitive 2026 needs one product-neutral RFC 3161 acquisition and
  verification capability; Witness has a real production use and archived
  Submission supplies a second distinct composition use.
- The July 27 implementation must not be admitted unchanged. A verified CRL
  transport-classification defect, a completely unproved ordinary acquisition
  path, and an undocumented public constructor that permits caller-authored
  outage evidence are blockers.

## Evidence boundary


| Source | Exact revision and pin | Timeproof availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | Exact package tree `b73dbe5bc5ab25b1e667a605a840aacf8758729d` | One unrelated pre-existing untracked file, `core/api_http_boundary_hostile_test.go`; no archive file changed during this interview. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` | The committed pin contains no `timeproof` tree. The dirty pin contains exact tree `b73dbe5bc5ab25b1e667a605a840aacf8758729d`. Current Kernel production has no direct import. | Materially dirty tree, including the Primitive pin and unrelated product work. Dirty-pin presence is not released adoption evidence. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` | Pin contains no `timeproof` tree. Current production retains local RFC 3161 implementations and no Primitive `timeproof` import. | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` | Pin contains no `timeproof` tree. Current production has no direct import. | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` | Pin contains no `timeproof` tree. Current production has no direct import. | `.ledger_pending.md` is modified; production and test sources inspected here match HEAD. |

The four committed consumer pins all predate Timeproof. Kernel's dirty pin is
the only consumer checkout that contains the exact archived package, and
Kernel does not call it. There is therefore no downstream cutover proof for
this API. That absence does not make the capability obsolete: Witness has a
large live duplicate and a concrete release-manifest timestamp operation.

The material archive history is short and reconstructable:

1. `ab799782d03e602b344fac85ef3d477bfb348374` introduced the authoritative
   time-proof primitive on 2026-07-25.
2. `d259789e87bcadb829c5ffac72c6c91ccc604098` centralized constants and closed
   capabilities later that day.
3. `8e7aa9a170d5b63e0f6a0664ab6670b97cd253c8` added the hardened Submission
   protocol on 2026-07-26.
4. `89fb27ff6f328e478d71f266604de8af87fdecdc` hardened text, timestamp, and
   signed-URL boundaries on 2026-07-26.

The archive index calls Timeproof a reviewed implementation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:26`). Its completion ledger also claims the
live smoke, race/shuffle, static analysis, cross-builds, and user approval were
green (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1135-1189`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:2192-2267`). Those are historical claims. This interview records the gates
actually reproduced below and does not convert a prior ledger checkbox into
current proof.

## Capability ownership

Timeproof owns one narrow fact:

> obtain or verify one bounded RFC 3161 timestamp proof for one typed SHA-256
> digest.

That exact boundary appears at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:6-23`. Within it,
Timeproof owns:

- RFC 3161 request construction;
- RFC 3161 and CMS response parsing;
- RFC 5816 ESSCertID/ESSCertIDv2 signer-certificate binding;
- timestamp-signing EKU, certificate-chain, and signature verification;
- message-imprint, nonce, policy, serial, and generation-time verification;
- captured and replayed revocation evidence;
- a closed authority registry and deterministic authority-order policy;
- one total acquisition budget composed over Exchange attempt policy;
- a proof-carrying authoritative result;
- an explicitly non-authoritative local fallback with attempt/exhaustion
  evidence; and
- caller-owned skew assessment beside, not inside, the cryptographic proof.

It does not own what the digest means. Evidence records, receipts, leases,
gates, products, accounts, offerings, routes, persistence, object publication,
billing, display, retention, or control-plane policy remain downstream
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:17-19`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:37-44`).

An RFC 3161 timestamp proves that an authority attested to one message imprint
at its `genTime` under the verified policy. It does not prove:

- who created the object;
- that the object was accepted, paid for, published, or retained;
- that a product server's signed commercial clock advanced monotonically;
- that a local wall clock is trustworthy;
- that a release is latest or permitted;
- or that a run, submission, or mutation satisfied consumer policy.

Those exclusions matter for Bug and Witness. Their signed server-time,
high-water, lease, writer-certificate, and receipt semantics cannot be
collapsed into a generic RFC 3161 result.

## Archive evidence

The production import graph matches the intended low-level ownership:

```text
core -> temporal
core -> contextstate
core -> keygen
core -> exchange
temporal -> exchange
contextstate -> exchange
temporal + contextstate + keygen + exchange -> timeproof
```

`go list` at archived HEAD shows Timeproof imports only the standard library
plus `core`, `temporal`, `contextstate`, `exchange`, and `keygen`. It imports no
consumer, persistence, object store, lifecycle, attestation, receipt, or
product package. The direct-import ratchet enforces this allow-list
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/architecture_test.go:14-38`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/architecture_test.go:124-139`).

This is the right direction:

- Core owns universal SHA-256, HTTP, media, limit, JSON, and stable-error
  contracts.
- Temporal owns typed instants, durations, timeout construction, comparison,
  and checked arithmetic.
- Contextstate owns trusted cancellation/deadline classification.
- Exchange owns bounded HTTP, retry, backoff, jitter, redirects, media, status,
  and response limits.
- Keygen owns CSPRNG material.
- Timeproof owns the RFC/CMS/PKI meaning assembled over those primitives.

Persistence belongs to a composition root over Filestore. Publication belongs
to a composition root over Objectstore. Attest and Timeproof remain peers:
Ed25519 proves signer possession and domain binding; RFC 3161 proves
authority-attested time for a digest.

### Archived architecture worth retaining

### Closed authority, policy, trust, and endpoint facts

The archive does not expose a mutable URL/root plug-in. `Authority` is a closed
enum, initially FreeTSA. Policy identity, trust identity, and acquisition
endpoints are separate compiler-owned values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:101-140`).

That separation is excellent:

- offline replay does not parse a URL;
- changing an endpoint cannot retroactively invalidate stored evidence;
- policy JSON and the ASN.1 policy OID derive from one token;
- root bytes are embedded;
- root SHA-256, exact DER serial width, validity interval, and self-signature
  are checked; and
- endpoint lookup occurs only in acquisition.

The implementation is at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/registry.go:18-36`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/registry.go:38-100`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/registry.go:103-197`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/registry.go:257-336`. The architecture ratchet requires
`endpointsForAuthority` to be called only from `acquire.go`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/architecture_test.go:40-71`).

FreeTSA's root contract includes a pinned 32-byte fingerprint, exact
nine-octet positive DER serial, validity from 2016-03-13 through 2041-03-07,
and self-signature verification
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/registry.go:265-283`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/registry.go:298-335`). A future provider requires
new enum, policy, trust, endpoint, authentic-fixture, and hostile-proof facts;
it cannot arrive as a configuration string.

### Request construction and nonce binding

`Acquire` validates the context, client, digest, and caller observation, obtains
fresh fixed-width secret material through Keygen, converts it to a nonzero
nonce, and builds one request before retries
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:28-56`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/nonce.go:13-69`).

The request is:

- RFC 3161 version 1;
- SHA-256 message imprint over the caller's typed digest;
- one exact nonce;
- `certReq=true`;
- canonical DER with explicit ceilings; and
- transmitted as `application/timestamp-query`, expecting
  `application/timestamp-reply`.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/request.go:12-144` and
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:186-205`. The same encoded request and nonce
survive Exchange retries, while a new top-level call gets new material. That is
the correct replay boundary.

### Sequential bounded acquisition and explicit degradation

The client closes an ordered authority policy and an Exchange-backed network
policy. `AttemptTimeout` cannot exceed `TotalTimeout`; retry policy is typed;
the CRL cache is bounded to the closed authority count
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/policy.go:11-87`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/policy.go:103-167`).

Acquisition places one total timeout around the complete ordered attempt loop.
It stops on the first verified authority, returns no fallback on terminal
context failure, and otherwise returns a closed unofficial result only after
policy exhaustion (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:59-106`).

The attempt lattice preserves:

- unavailable transport;
- unavailable revocation source;
- HTTP refusal;
- RFC status refusal with typed PKI failure bits;
- invalid response media or size;
- malformed response;
- signer, signature, chain, digest, nonce, policy, generation-time, and
  revocation-invalid failures; and
- verified authority.

Attempt sets reject duplicate authorities and any attempt after a verified
attempt. Exhaustion is derived as unavailable-only, refused-only,
invalid-proof-observed, or mixed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/attempt.go:9-176`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/attempt.go:178-263`).

The model is worth retaining even though the current acquisition proof and one
classifier are defective. Invalid cryptography must never be mislabeled as an
ordinary outage, and an outage must not be inflated into evidence of a hostile
authority.

### Full verifier and proof-carrying evidence

The verifier is the archive's strongest part. It checks the complete token
again from stored bytes, not merely persisted projections:

- response/status/token closure;
- CMS content types and one signer;
- canonical signed attributes;
- message-digest and content binding;
- ESSCertID or ESSCertIDv2 signer binding;
- signature algorithm and signature;
- timestamp-only critical EKU;
- leaf/intermediate/root chain at `genTime`;
- policy, imprint, nonce, serial, and generation time;
- CRL issuer, signature, update window, and revocation timing; and
- separately supplied expected digest.

The authoritative result carries the raw bounded response, raw bounded CRL,
authority, digest, nonce, derived `genTime`, serial, policy, and signer SHA-256.
All byte accessors clone
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/evidence.go:11-82`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/timestamp.go:10-74`).

`Verify` reconstructs from the raw evidence and separately checks the expected
digest (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/verify.go:10-74`). Authoritative JSON decode invokes
that verifier, compares every derived projection, requires canonical bytes,
and mutates the receiver only after success
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/timestamp.go:84-137`).

This is materially stronger than accepting a timestamp-shaped blob or trusting
stored `genTime`, signer, and policy fields.

### Closed result and strict persistence reconstruction

`Result` is a private-representation closed sum with exactly one authoritative
or unofficial variant. Validation rejects zero, cross-variant contamination,
attempt/result disagreement, verified attempts without authoritative evidence,
and unofficial results without derived exhaustion
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/result.go:10-102`).

Canonical JSON contains an explicit kind. Strict decode rejects unknown,
duplicate, missing, null, trailing, noncanonical, and cross-variant inputs
before assignment. Authoritative reconstruction re-runs cryptographic
verification (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/result.go:123-231`).

Authority evidence itself has exact base64 canonicalization, bounded raw
response and CRL bytes, cloned ownership, and receiver preservation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/evidence.go:92-160`).

### Skew is a use-time projection

The archive correctly removed caller observation and acceptance threshold from
the proof. `AuthoritativeTimestamp.AssessSkew` compares one caller observation
to verified `genTime` under a caller-owned maximum. It yields typed state,
direction, delta, and maximum without mutating or re-encoding the proof
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/timestamp.go:66-74`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/skew.go:8-117`).

This preserves a crucial distinction:

- proof fact: what time the TSA cryptographically asserted;
- local observation: what the caller's clock said;
- policy: how much difference one consumer tolerates.

An unofficial timestamp has no skew assessment and cannot advance trusted time,
extend a lease, defeat rollback detection, or satisfy an authority-required
receipt (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:577-626`).

### Architecture and semantic-crypto ratchets

The archive inventories every production struct and classifies protocol facts,
sealed projections, internal flow, capabilities, state, and typed errors.
Adding or removing a struct breaks the inventory
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/architecture_test.go:191-320`).

It also bans raw ASN.1, X.509, HTTP, URL, maps, and interfaces from exported
struct fields (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/architecture_test.go:141-189`).

The authentic FreeTSA fixture verifies exact authority, policy, signer,
generation time, response, and CRL. Mutations hit outer DER, CMS content,
signature tail, CRL envelope, and CRL signature
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/authentic_fixture_test.go:14-143`).

The verifier has a real LayerTriad:

1. authentic signed evidence projects one authoritative proof;
2. a signature-region mutation projects no authority and a typed invalid
   identity; and
3. absent evidence creates no timestamp claim
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/authentic_fixture_test.go:145-190`).

Semantic fuzzing begins from authentic evidence rather than hoping arbitrary
bytes reach a cryptographic success path. It attacks response bytes, expected
digest, expected nonce, truncation, and trailing data, and constrains any
accepted mutation to the exact verified projection
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/tamper_fuzz_test.go:8-227`).

### Archived Primitive dependents

A complete production import search at archived HEAD found exactly one direct
Primitive dependent: `submission`.

`submission.Declaration` embeds:

- exact Objectstore integrity;
- exact `timeproof.Result`;
- submission/account/offering/device/evidence-kind/revision facts; and
- a distinct digest over the complete canonical declaration.

Construction validates the Timeproof result. If it is authoritative, its
message-imprint digest must equal the evidence object's Objectstore SHA-256.
The declaration digest then binds the exact Timeproof JSON together with every
other declaration fact
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/declaration.go:12-115`).

Strict decode reconstructs and revalidates the complete declaration before
assignment (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/declaration.go:146-212`). The specification
explicitly keeps an unofficial proof unofficial through declaration, upload,
and receipt issuance
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/SPEC.md:158-203`).

No archived production package imports Submission, so this is one internal
composition consumer, not an end-to-end product path.

Release deliberately does not import Timeproof. A composition root may verify
a Timeproof document and place the resulting time reference into a release
fact, while Release remains transitively free of Timeproof and Exchange. The
archive ledger records that intended boundary
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:2390-2436`). Primitive 2026 should retain it:
pure release selection must not acquire time or perform HTTP.

## Consumer evidence

### Kernel

Kernel has no committed or dirty production import of Primitive Timeproof.
The dirty module pin merely makes the exact archived package available.
Kernel's current needs do not justify making RFC 3161 a dependency of Core,
Temporal, authentication, ledgers, or ordinary HTTP.

The strongest adjacent candidate is Scout's scan ledger. A ledger entry includes
the prior hash, findings, source/dependency fingerprint, local timestamp, and a
SHA-256 chain hash. Append holds an exclusive lock, links to the prior entry,
writes one bounded JSONL record, and syncs it
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/ledger.go:17-116`). The fingerprint binds Git and dependency
identity but stamps ambient local time
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:scout/fingerprint.go:20-72`).

That is a useful possible composition point: a product decision could timestamp
a selected Scout chain head or release artifact digest. It is not present
behavior and must not be invented as a Primitive requirement. Kernel's local
gems are the content-linked chain, bounded append, explicit source identity,
and canonical ledger precision, not a claim that its wall-clock field is
authoritative.

Kernel's broader ledger accepts `Now` as a required input rather than reading
it inside the pure constructor
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/ledger/event.go:464-495`). That reinforces the boundary:
Kernel owns the event and acceptance policy; Timeproof can supply one optional
independent fact at a composition root.

**Kernel conclusion:** no current admission-driving use. Preserve optional
digest-proof composition without forcing Timeproof into Kernel Core or general
Temporal.

### Witness

Witness is the real first consumer and the strongest comparison source. Its
local `internal/notary` owns a mature RFC 3161 implementation, and
`cmd/witness-release` timestamps the release manifest hash through it
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:405-422`).

The result is not decorative. Bundle verification:

- strictly decodes the typed TSA artifact;
- validates its sealed fields;
- opens the exact token sidecar;
- reads exactly the declared byte count with an extra-byte check; and
- calls stored-token RFC 3161 verification
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/verify/verify.go:1201-1275`).

Witness's local implementation supplies important gems and failure history:

- request/result/policy facts are Core-owned typed structs
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:220-246`);
- nonce and verification time are created per request on a private client copy,
  after a bug showed shared policy mutation reused a nonce and raced
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:399-445`);
- endpoint probes can race to first success while attempt evidence is merged
  in configured order (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:526-555`);
- retry uses deterministic injectable sleeping, exponential backoff, jitter,
  and `Retry-After`
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:716-794`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:869-880`);
- chain verification occurs at TSA `genTime`
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary.go:2028-2059`);
- endpoint and bundled root are one registry fact, after a provider was
  advertised without a usable trust anchor
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/trust_anchors.go:17-84`);
- deterministic tests cover acquisition success, all-endpoint attempt evidence,
  fallback/exhaustion, permanent status, context deadline, parallel first-win,
  backoff, `Retry-After`, stored-token replay, and authentic recorded TSR
  provider binding
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:343`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:396`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:961`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:998`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:1025`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:1928`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:2974`,
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:3806`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:3880`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:4098`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:7025`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/notary/notary_test.go:7207`).

The archive is stronger in several ways: closed typed authority identity instead
of public URL strings, one transport owner through Exchange, RFC 5816 binding,
captured CRL bytes, canonical proof reconstruction, proof/unofficial sum
closure, and typed bounded errors. Witness is stronger in deterministic
acquisition-path proof and operational failure history.

Witness also retains a weaker custody timestamp client that verifies only
structural correspondence and stamps local observation. It must be retired, not
normalized into the new capability
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:43-46`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:79-101`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp_client.go:143-159`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/timestamp.go:176-225`).

Witness-owned policy remains downstream:

- which manifest, chain head, or evidence object is timestamped;
- whether an authoritative proof is mandatory;
- release/custody failure behavior;
- receipt and artifact paths;
- retry/provider deployment choices that are truly product policy;
- ledger ordering, retention, and human projection.

**Witness conclusion:** first migration target. Replace both duplicate RFC 3161
paths with one Primitive Timeproof capability only after the archive blockers
are corrected and the complete Witness receipt/replay semantics are preserved.

### Bug

Bug has no RFC 3161 client or direct Timeproof import. Its strongest adjacent
mechanic is not a TSA proof: it is a product-signed, rollback-resistant
operation time.

Bug:

- verifies monotonic signed server-time commitments and rejects conflicting
  facts at one server instant
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:394-411`);
- selects a durable trusted floor from local, state, device, and signed-server
  facts, then verifies the writer certificate at exactly that selected instant
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:611-644`,
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:301-312`);
- caps accepted clock progression at the authenticated lease boundary and
  merges concurrent floors
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:209-267`); and
- signs repository, operation digest, device, writer, and certified occurrence
  time into a writer attestation
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_attestation.go:44-114`).

Writer proof claims separately bind commit, proof SHA-256, bug identity,
operation, and occurrence time, then derive a typed digest
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/writer_proof.go:74-145`).

These are excellent local gems:

- durable maximum authentic floor;
- conflict rejection at equal authority time;
- bounded repair of a poisoned future floor;
- certificate validity checked at the exact selected instant; and
- subject/domain binding around a signed operation digest.

They are not Timeproof internals. RFC 3161 cannot replace the product server's
lease progression or the writer's signature. A composition root could
optionally request a TSA proof for the final writer-proof digest, but current
Bug code does not require or implement that ceremony.

**Bug conclusion:** retain Bug's signed progression and certified-operation
policy in Bug/Lease. Timeproof may later provide an independent digest-time
fact, never the durable commercial clock itself.

### Peachfuzz

Peachfuzz has no direct Timeproof integration and supplies no current
authority-time requirement.

Its adjacent evidence path is nevertheless relevant. Peachfuzz seals one
validated run-evidence value with Ed25519, canonical-encodes it, computes its
content digest, and verifies signature, bytes, and digest at hostile archive
ingress (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence.go:15-88`).

Its durable watermark explicitly distinguishes complete-history adoption from
retained-history adoption and forbids regression
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence_watermark.go:18-135`). Its pack
manifest binds sorted content digests to exact contiguous offsets and one pack
digest (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/pack.go:71-142`). Publication uses
content-addressed pack and manifest paths so crash/retry converges without a
mutable upload ledger
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/publish.go:104-140`).

Those are Peachfuzz's local gems:

- signed canonical run evidence;
- content-addressed immutable identity;
- explicit historical-origin truth;
- monotonic archive watermark;
- deterministic pack layout; and
- retry-convergent publication.

A future evidence service could timestamp a run-evidence digest, pack-manifest
digest, or ledger checkpoint. That would prove external occurrence time only.
It would not decide run validity, archive completeness, lifecycle retention,
reclamation eligibility, or watermark advancement.

**Peachfuzz conclusion:** no current admission-driving use. Keep Timeproof
optional and above immutable evidence projection; do not put it inside archive
packing, scheduling, finding ownership, or retention.

## Strong mechanics and proof

| Use | Existing owner | What Timeproof could contribute | What must remain outside |
| --- | --- | --- | --- |
| Witness release manifest and evidence receipt | Witness Notary, release command, artifact verifier | Authoritative RFC 3161 acquisition, captured proof, replay verification, typed outage result | Manifest identity, receipt requirement, custody path, failure policy, publication |
| Archived Submission declaration | Primitive Submission composition | Proof for exact Objectstore SHA-256; explicit unofficial result when permitted | Account/offering/device/evidence kind, acceptance, quota, storage, receipt |
| Kernel Scout or release chain head | No current Timeproof use | Optional external time proof for a selected final digest | Scan semantics, ledger hash, cache policy, release policy |
| Bug writer proof | Bug license and run | Optional independent proof for the already-derived writer-proof digest | Signed server-time progression, writer certificate, repository/operation binding, commercial gate |
| Peachfuzz evidence or pack checkpoint | Peachfuzz archive | Optional external time proof for immutable signed evidence or manifest digest | Run validity, archive completeness, watermark, lifecycle, retention |

Only Witness and archived Submission are present concrete uses. Kernel, Bug, and
Peachfuzz are possible composition seams, not evidence for mandatory adoption.

## Defects and blockers

### 1. CRL transport outages are classified as invalid cryptographic proof

This is a concrete production defect.

`revocationForToken` returns Exchange errors directly when CRL fetch fails
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:207-230`). Exchange classifies HTTP transport
failure as `core.ErrExchangeTransport`, and exhausted retry wraps
`core.ErrExchangeRetryExhausted`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:244-255`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:435-447`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:701-709`).

Timeproof then classifies a revocation failure as unavailable **only** when
`errors.Is(err, core.ErrExchangeContract)`. Every other error becomes
`AttemptStateInvalidProof/AttemptFailureRevocationInvalid`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:291-299`).

The branches are semantically reversed for the important cases:

- a network outage, timeout/retry exhaustion, or unavailable CRL endpoint is
  labeled invalid proof;
- a local Exchange contract defect is labeled revocation unavailable.

That wrong attempt state then derives
`ExhaustionInvalidProofObserved` instead of `ExhaustionUnavailableOnly`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/attempt.go:238-260`).

It contradicts the specification's requirement to keep revocation transport
unavailability distinct from malformed or cryptographically invalid CRL
evidence (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:721-729`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:746-755`) and the completion
ledger's claim that invalid proof never collapses into ordinary availability
state (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:2221-2223`).

This is not covered by any test. The only tests mentioning
`AttemptFailureRevocationUnavailable` or `AttemptFailureRevocationInvalid`
manually construct lattice values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/result_hostile_test.go:20-87`).

Primitive 2026 needs a typed classifier that distinguishes:

- terminal caller context;
- Exchange transport/retry/status unavailability;
- media/body-limit protocol rejection;
- malformed CRL;
- wrong issuer/signature/window;
- and revoked signer.

Each branch needs a deterministic production-path test and exact
`errors.Is`/typed-result assertion.

### 2. Ordinary tests execute none of the acquisition implementation

The only test call to exported `Acquire` is the opt-in live smoke
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/live_smoke_test.go:16-87`). With
`FOUNDATION_TIMEPROOF_LIVE` unset, ordinary runs skip it at line 19.

Coverage gathered during this interview reports 65.5% statements overall, but
every function in `acquire.go` from `AcquireRequest.Validate` through
`invalidAttempt` is 0.0%, except the parser-adjacent
`classifyVerificationFailure` at 66.7%.

Consequently ordinary deterministic tests do not execute:

- client/request ingress;
- fresh top-level nonce acquisition;
- total timeout;
- the authority loop;
- first-success stop;
- cancellation before/during/between attempts;
- Exchange submission;
- CRL fetch;
- CRL cache hit, invalidation, copy, or concurrency;
- status/media/body/transport classification;
- authoritative result construction; or
- unofficial fallback from real attempt outcomes.

The archive has a verifier LayerTriad, not an acquisition LayerTriad. Its
state/failure lattice tests show that manually assembled values validate; they
do not prove that production effects project into the correct values.

This fails the project testing protocol's production-path requirement
(`foundation@working-tree:_docs/testing_protocol.md:862-891`) and leaves most of the
specification's failover/fallback matrix unproved
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:746-755`).

Witness demonstrates that deterministic HTTP acquisition proof is practical.
Primitive 2026 needs an Exchange-backed local server/harness that exercises
the real production path, plus a true LayerTriad for:

1. valid authority response plus CRL -> authoritative result;
2. transport/CRL/crypto mutation -> exact non-authoritative attempt and no
   authority; and
3. no/terminal context -> no timestamp claim and no fallback where forbidden.

### 3. The public surface contradicts the specification and permits fabricated outage evidence

The stable-surface section says the intent operations are exactly:

- `NewAuthorityPolicy`;
- `NewClient`;
- `Acquire`; and
- `Verify`.

It says `public.go` contains these functions
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:78-99`).

`public.go` also exports `NewUnofficialResult`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/public.go:20-25`), and `result.go` exports
`UnofficialResultRequest` with caller-supplied `[]Attempt` and local instant
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/result.go:17-49`).

An external caller can therefore fabricate a syntactically valid statement
that FreeTSA was unavailable, refused, or returned invalid proof. The result
cannot claim authoritative time, which limits the security impact, but its
attempt and exhaustion diagnostics can be false and can be sealed into an
archived Submission declaration. Submission tests already use this constructor
to manufacture fallback evidence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/test_helpers_test.go:35-49`).

Primitive 2026 must choose and specify one truthful contract:

- remove the constructor and require unofficial acquisition results to come
  only from `Acquire`;
- expose a distinct manually observed local-time value with no authority
  attempt claims; or
- explicitly define trusted composition-root reconstruction and give it
  provenance that cannot be confused with observed acquisition.

Keeping an undocumented constructor because tests need a shortcut is not
acceptable.

### 4. The advertised stable Timeproof error set is not the actual acquisition boundary

Core declares `ErrTimeProofAuthorityUnavailable`,
`ErrTimeProofAuthorityRefused`, `ErrTimeProofInvalid`,
`ErrTimeProofEvidenceLimit`, `ErrTimeProofRevocation`, and
`ErrTimeProofExhausted`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:45-51`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:628-649`).

Production search shows:

- `ErrTimeProofAuthorityUnavailable` has no Timeproof producer;
- `ErrTimeProofExhausted` has no Timeproof producer;
- `ErrTimeProofAuthorityRefused` is used only by one parser rejection path;
- acquisition outage/refusal is normally represented only in `Result`; and
- a terminal context path returns the raw Exchange error directly
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:108-120`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:159-168`).

The specification permits exhaustion to be a result rather than top-level
error, but also says raw Exchange errors are projected onto the closed
Timeproof identity set. The implementation does not consistently satisfy that
claim.

Primitive 2026 should either remove dormant stable identities or give each a
documented producer/consumer purpose. Cancellation may retain standard context
identity, but the package boundary must also expose a Timeproof-owned typed
classification without leaking unrelated Exchange identity.

### 5. A one-authority registry makes failover architectural, not proved behavior

`core.TimeProofAuthorityMaximumCount` is one
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/timeproof_constants.go:17`), and `Authority` contains only
FreeTSA. The ordered policy shape is useful, but no current valid policy can
contain a second authority. Therefore:

- invalid-then-valid authority behavior;
- no authority attempted twice across multiple authorities;
- between-attempt cancellation;
- deterministic multi-authority evidence order; and
- mixed real acquisition exhaustion

cannot be demonstrated through the registered production policy.

Do not add a fake production authority merely to satisfy a test. Before
claiming failover in 2026, add a second real compiler-owned provider with
primary-source review, endpoint/root/policy binding, authentic evidence, and a
deterministic test transport that exercises both registry identities.

Until then, describe the current behavior as one-authority acquisition with an
ordered policy architecture, not operational provider redundancy.

### 6. Public skew validation is unproved

The result-producing skew table tests `assessSkew` output state and direction,
but does not call `SkewAssessment.Validate`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/value_hostile_test.go:142-189`). Interview coverage reports
0.0% for `SkewAssessment.Validate` and all three lattice validators.

The production logic appears coherent, but the public value boundary lacks
hostile proof for:

- unknown/out-of-range state and direction;
- zero/nonzero delta disagreements;
- equality with nonzero delta;
- within-tolerance at exact maximum;
- exceeded at exact maximum versus one nanosecond over;
- zero/invalid maximum; and
- malformed direct JSON through its exported fields.

That is a proof gap under the protocol's hostile validation matrix, not a
confirmed production behavior defect.

### 7. Current release and live-provider proof was not reproduced

This interview reproduced deterministic unit, race, shuffle/repeat, vet, and
semantic fuzz gates. It intentionally did not contact a public service.

The opt-in live command reported:

```text
set FOUNDATION_TIMEPROOF_LIVE=1 to exercise the real authority
--- SKIP: TestLiveFreeTSAAcquisition
```

The archive completion ledger says a prior live FreeTSA acquisition and
cross-platform/static gate set passed. Those claims are useful provenance, but
the specification itself says a skipped live smoke is not executed and is
never passing evidence (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/SPEC.md:757-771`).

Before a 2026 release:

- re-record and review an authentic current provider response/CRL;
- run the explicit live gate;
- run all declared cross-build/static/security gates;
- retain exact command output and revisions; and
- distinguish provider outage from implementation failure.

### 8. There is no downstream cutover proof

No committed consumer pin contains Timeproof. Kernel's dirty pin has the exact
tree but no call site. Witness's completion ledger says it will retire local
implementations, but current Witness still owns them. Archived Submission is
not consumed by another archived production package.

This does not reject the capability. It does mean the archived API has not yet
survived:

- Witness receipt/artifact migration;
- stored-proof compatibility review;
- deployment/provider policy review;
- a real consumer build at the new module pin; or
- removal of duplicate RFC 3161 implementations.

Admission must include a clean-cut migration plan, not merely a package-local
green gate.

### Testing-protocol assessment

The project-local testing protocol was read completely before this interview.
No tests were written or modified.

#### Evidence reproduced at the archive revision

| Command | Result |
| --- | --- |
| `go test -count=1 ./timeproof` | PASS |
| `go test -race -count=1 ./timeproof` | PASS |
| `go test -shuffle=on -count=3 ./timeproof` | PASS |
| `go vet ./timeproof` | PASS |
| `go test -count=1 -coverprofile=/tmp/timeproof-cover.out ./timeproof` | PASS; 65.5% statements |
| `go test -run '^$' -fuzz='^FuzzAuthorityEvidenceJSONSemanticClosure$' -fuzztime=3s ./timeproof` | PASS; 38,947 executions |
| `go test -run '^$' -fuzz='^FuzzTimestampResponseCryptographicClosure$' -fuzztime=3s ./timeproof` | PASS; 33,695 executions |
| `go test -run '^$' -fuzz='^FuzzRevocationEvidenceCryptographicClosure$' -fuzztime=3s ./timeproof` | PASS; 43,563 executions |
| `go test -run '^$' -fuzz='^FuzzTimestampTokenTamperRejection$' -fuzztime=3s ./timeproof` | PASS; 14,839 executions |
| `go test -count=1 -run '^TestLiveFreeTSAAcquisition$' -v ./timeproof` | PASS with the sole test SKIPPED; not live evidence |

Temporary coverage/fuzz artifacts were outside the repository.

#### Strong conformance

The archive satisfies important parts of the protocol:

- genuine positive/negative/absent LayerTriad for the verifier
  (`foundation@working-tree:_docs/testing_protocol.md:518-642`);
- hostile tables far beyond ordinary happy-path testing;
- authentic cryptographic fixtures rather than mocked signatures;
- semantic fuzz oracles with accepted-value invariants
  (`foundation@working-tree:_docs/testing_protocol.md:1210-1288`);
- strict typed error assertions through `errors.Is`;
- structural architecture and data-flow inventories
  (`foundation@working-tree:_docs/testing_protocol.md:1010-1098`);
- receiver non-mutation and canonical round trips; and
- repeat, shuffle, race, and deterministic local execution.

#### Missing conformance

The decisive protocol failure is that the verifier's strength masks a missing
effect-path proof. The protocol requires tests to execute the production path
and prove behavior, not merely an analogous helper or manually assembled result
(`foundation@working-tree:_docs/testing_protocol.md:862-891`).

The archive needs:

- a deterministic acquisition LayerTriad;
- hostile 20-invalid/10-valid style coverage at the acquisition, classifier,
  cache, and public-skew boundaries
  (`foundation@working-tree:_docs/testing_protocol.md:382-516`);
- explicit red evidence before each defect fix and focused green evidence
  afterward (`foundation@working-tree:_docs/testing_protocol.md:149-226`);
- exact stable error/result assertions for every effect failure;
- concurrency proof for shared-client CRL cache;
- cancellation and total-budget proof without wall-clock guesses; and
- a separate explicit live-provider gate.

Coverage percentage is not an admission metric. The useful coverage fact is
structural: an entire security-relevant production file is absent from ordinary
execution.

## Primitive 2026 ownership and DAG

### Ownership split

Primitive 2026 Timeproof should own:

- closed authority identity;
- authority-to-policy/trust/endpoint registry;
- RFC 3161 request and response semantics;
- CMS and ESS certificate binding;
- timestamp signer/chain/revocation verification;
- digest, nonce, policy, serial, and `genTime` binding;
- bounded exact evidence;
- deterministic attempt projection;
- authoritative/unofficial closed result;
- replay verification; and
- caller-input skew projection.

Primitive primitives retain:

- `core`: universal digest/media/limit/error contracts;
- `temporal`: instant, duration, checked comparison, and timeout mechanics;
- `contextstate`: terminal context observation;
- `exchange`: HTTP, total/attempt budget, retry, backoff, jitter, status,
  redirect, and bounded body mechanics;
- `keygen`: fresh secret/nonce material;
- `filestore`: durable local proof persistence; and
- `objectstore`: proof/evidence publication.

Consumers and higher protocols retain:

- which digest is timestamped;
- whether authority is mandatory;
- degraded-operation consequences;
- receipt, release, submission, writer, run, and artifact semantics;
- signed server-time progression and durable high-water;
- lease/gate/commercial interpretation;
- provider rollout and operational monitoring;
- retention, publication, and human display.

### Minimum implementation DAG

```text
core -> temporal
core -> contextstate
core -> keygen
core -> exchange
temporal + contextstate + keygen + exchange -> timeproof

timeproof -> composition-root persistence/publication
timeproof -> Witness receipt/release/custody composition
timeproof -> submission declaration composition

attest + objectstore + filestore -> submission/composition

lease / gate / release / consumer state
    consume verified time facts only where their own policy requires them;
    they do not acquire time inside pure evaluation.
```

Timeproof must follow Core, Temporal, Contextstate, Exchange, and Keygen
admission. It should precede Submission integration and Witness notary cutover.
It must not become a prerequisite of pure Release fact verification, Lease,
Gate, general Temporal, or consumer archive logic.

### Migration order

1. Resolve the Primitive 2026 Core/Temporal/Contextstate/Exchange/Keygen
   contracts Timeproof actually needs.
2. Write the new Timeproof specification from this report, not by copying the
   archive.
3. Decide the manual-unofficial-result contract before fixing any API.
4. Implement the acquisition LayerTriad and classifier tests first, recording
   red evidence for the archived CRL defect.
5. Reconstruct the verifier, evidence, strict JSON, and skew mechanics while
   preserving authentic-fixture and semantic-fuzz strengths.
6. Add current provider root/policy/endpoint evidence and run explicit live and
   cross-platform gates.
7. Migrate Witness's release/custody producer and bundle verifier as the first
   real consumer; remove both superseded local RFC 3161 paths in the same clean
   cut.
8. Integrate Submission only after Objectstore and its declaration/receipt
   composition are admitted.
9. Add Kernel, Bug, or Peachfuzz use only when each product names a concrete
   authority-required digest contract. Do not create ceremonial imports.

No compatibility wrapper, alias, old-result adapter, raw-URL plug-in, or second
transport loop should survive migration.

## Decision rationale and conditions

**Retain and later admit the capability. Do not admit or copy the archived
implementation unchanged.**

The archive contains unusually strong reusable design evidence:

- narrow ownership;
- closed authority/trust/policy/endpoint contracts;
- exact RFC 3161/CMS/RFC 5816 verification;
- embedded pinned trust;
- captured CRL evidence;
- proof-carrying replay;
- authoritative versus unofficial type closure;
- caller-owned skew;
- strict canonical reconstruction;
- authentic fixtures;
- a real verifier LayerTriad;
- semantic cryptographic fuzzing; and
- explicit data-flow architecture.

The following are admission blockers:

1. correct the reversed CRL transport/invalid-proof classification;
2. execute the complete acquisition path deterministically in ordinary tests;
3. resolve or remove `NewUnofficialResult` and caller-authored attempt evidence;
4. align the stable Timeproof error/result boundary with actual producers;
5. prove public skew validation and shared CRL-cache behavior;
6. describe one-authority behavior honestly until a second real provider is
   registered;
7. re-run explicit live, cross-build, and declared static/security gates; and
8. complete one clean downstream cutover, beginning with Witness.

Kernel, Bug, and Peachfuzz do not currently require Timeproof, and their local
clock, license, ledger, writer, and archive rules remain product-owned.
Witness's actual release/custody use and archived Submission's digest-binding
use are sufficient to justify the product-neutral capability without inflating
it into a universal trusted-clock system.

This recon report authorizes no implementation, consumer migration, commit, or
push.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
