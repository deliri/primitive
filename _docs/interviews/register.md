# Register package recon

Status: `COMPLETE` | Decision: `DEFER`

This report is the sole recon record for archived package `register`. Here,
`register` means local device enrollment for one commercial offering. It does
not mean Kernel account signup, WebAuthn passkey registration, Go callback
registration, process registries, or durable-file activation.

## Evidence boundary

The following repositories and exact revisions were inspected read-only:

| Repository | Revision or Primitive pin | Relevant state |
| --- | --- | --- |
| `foundation_back_up_july_27th_2026` | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` | Full archived package |
| archived Register tree | `2aa5ae5e13ad89bb243d4b1b14e7918e1d968611` | `HEAD:register` tree |
| initial Register specification | `87b55ba36b420b07e7037b8b2d56c96ebc03158c` | Device-registration lifecycle specified |
| Register implementation | `385669e7af53a252673ad38a6a061688aa67a900` | Hardened protocol implemented |
| recovery hardening | `6906facd7c6e67754455185891cfcb336101eb61` | Policy boundaries and lifecycle recovery hardened |
| latest package-affecting change | `40ded9c104a99cbc4b0b672cd7392901b468d1eb` | Comparative-contract hardening |
| archived comparison | `e8b7172161a4` | Same Register tree and Register-owned Core contracts as archive HEAD |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed pin `0df2954a2d91`; dirty worktree pin `e8b7172161a4` |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` | Pin `773add8ba0fc` |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` | Pin `388e593231a2` |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` | Pin `3f74d8fc35b4` |

The archive worktree has an unrelated untracked Core test. Register and its
Core-owned contracts are clean. No archived or consumer source was changed
during this interview.

The archived package is large: 12,531 lines across its specification,
production, tests, and `core/register_constants.go`. It contains 71 named tests
or fuzz targets.

The focused package gates were rerun:

- `go test ./register` completed successfully.
- `go test -race -shuffle=on -count=2 ./register` completed successfully.
- Production-only `gocyclo -over 10` reported no findings.

The green result proves the archived client package on the current Darwin host.
It does not prove an authority implementation, a consumer migration, Linux or
Windows terminal behavior, or suitability for Primitive 2026.

### Consumer pin and import facts

The committed Primitive pins used by Kernel, Witness, Bug, and Peachfuzz do not
contain a `register` directory. Kernel's dirty worktree pin `e8b7172161a4`
contains Register, but Kernel does not import or call it.

An exact import scan found:

- no `github.com/deliri/primitive/v2026/register` import in Kernel;
- no such import in Witness;
- no such import in Bug;
- no such import in Peachfuzz; and
- no such import from another archived Primitive package.

Therefore Register has zero current Primitive dependents and zero current
direct consumer dependents. Witness and Bug have independently implemented
commercial-device activation flows that overlap the intended capability. Those
flows are evidence of a real domain, but they are not integration evidence for
the archived API.

## Capability ownership

The archive says Register does one thing: enroll one locally generated device
identity for one offering and durably install the authenticated lifecycle state
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:6-21`).

It claims ownership of:

- bounded enrollment-secret ingress;
- a retry-stable attempt identity;
- local Ed25519 device-key generation;
- device identity derivation and proof of possession;
- initial enrollment and device recovery;
- a signed registration receipt;
- verification of the returned concrete Controlstate document;
- exact idempotency and reconciliation request construction; and
- crash-safe absent/pending/complete local state.

It explicitly excludes accounts, products, plans, pricing, payments, fraud
policy, recovery policy, route catalogs, rendering, and server persistence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:20-21`).

That boundary is directionally correct. Primitive can own the shared typed
enrollment protocol and local recovery transaction. The composition root must
own:

- how a secret is presented to the user;
- the concrete account authority and endpoints;
- the product/offering selection;
- authoritative secret authentication and rotation;
- recovery allowance and account policy;
- authoritative idempotency persistence;
- support workflow and presentation;
- native private-key custody policy; and
- installation of the returned Controlstate into downstream stores.

The archived package crosses several of those layers in one package. That is the
central admission concern.

## Archive evidence

### Closed secret ingress

`SecretSource` is a closed union for terminal, standard input, or inherited
descriptor. The package refuses arbitrary `io.Reader` and caller-selected path
ingress (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:95-122`).

The framing contract is exact and bounded: 32 through 256 ASCII graphic bytes,
followed by LF, CRLF, or EOF, with a derived overflow-probe ceiling
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:124-136`). `EnrollmentSecret` is a pointer capability backed
by shared fixed-capacity mutable state. Copies share one consumption right, and
`Destroy` is idempotent (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/secret.go:89-133`).

Terminal reads disable echo and join restore errors. Inherited descriptor
ownership is transferred and closed once; stdin remains borrowed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/secret.go:135-213`). These are useful security mechanisms.

The most important product-level consequence is that a correct composition
cannot pass the account secret in argv, retain it in a string, or persist it
after enrollment. Witness and Bug currently do all or part of that, so a
Register migration would be a security migration, not a package substitution.

### Nominal retry-stable identity

`Revision`, `EnrollmentKind`, `AttemptIdentity`, `ReceiptIdentity`, and issuer
generation are nominal typed values with private representation and exact
validation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/values.go:16-273`).

An empty transaction obtains exactly
`core.RegisterAttemptIdentityByteWidth` bytes from Keygen, converts them into
the attempt identity, clears the mutable copy, and generates one signing key
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:907-928`).

Device identity, receipt identity, and idempotency use domain-separated,
length-delimited canonical frames (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/values.go:274-374`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:442-452`). The caller cannot supply the device identity,
attempt identity, proof, or idempotency key independently.

This is a strong compiler-owned shape: one locally durable source fact derives
all replay identities.

### Proof of possession

The declaration binds revision, enrollment kind, attempt, offering, device, and
public key. `DeviceProof` signs the canonical declaration frame
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:203-224`).

The private key never crosses the network. It is durable before the first
connection, so restart re-derives the same public identity and deterministic
Ed25519 proof. That closes a serious gap in the Witness flow, whose device
fingerprint is derived from random entropy but does not prove possession of a
signing key during registration.

Bug already has a stronger local writer identity: its device fingerprint derives
from the writer public key and its grants bind a writer certificate. That
consumer design should inform a rebuilt enrollment domain.

### Signed receipt and Controlstate closure

`ReceiptFact` binds revision, receipt identity, attempt, kind, account, offering,
device, public key, positive signing generation, authoritative acceptance
instant, exact Controlstate-document digest, and recovery disposition
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:226-271`).

The signer exists only in the Attest envelope. The fact does not duplicate it.
`VerifyReceipt` returns a privately constructible proof-carrying
`VerifiedReceipt`.

Response verification independently verifies the receipt and Controlstate, then
requires account/offering/device binding, exact Controlstate digest, generation,
and allowed registration state to close
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:453-549`). A valid receipt paired with another valid
Controlstate cannot install.

This is a high-value mechanism. A 2026 design should preserve the single signer
truth, exact document digest, and cross-document binding.

### One-record durable state machine

The retained state is absent, pending, or complete. Pending stores revision,
kind, attempt, and one `core.Ed25519KeyPair`. Contrary to the specification's
store-one-half claim, that KeyPair's JSON persists both
`private_key_base64` and `public_key_hex` and reconciles them during decode
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:190`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:225`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:85-111`). Pending omits a separate Register-level device
identity, proof, secret, account identity, response body, endpoint, and retry
guidance, but it does not omit public-key bytes.

`storeRecord.Validate` enforces the pending/complete union
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:187-219`). Strict decode constructs a candidate and assigns
only after validation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:221-275`).

The public enrollment path:

1. validates context, store, client, request, and secret;
2. creates and durably installs pending;
3. releases exclusive ownership;
4. transmits and verifies;
5. reacquires ownership;
6. requires byte-identical pending identity and key;
7. atomically replaces pending with complete; and
8. returns installed only after durable completion
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:432-474`).

`installComplete` compares the current durable pending record before replacement
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:956-1005`). `writeRecord` performs one bounded
`filestore.InstallReplace` under the store's exclusive owner
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:1007-1024`).

This directly improves on Witness and Bug. Both consumers write grant, device,
and mutable state as separate atomic files under one lock. A crash between
renames can leave a cross-file partial commit. Register represents the
enrollment transition in one file.

### Real crash and contention proof

The archive does not merely unit-test helpers. It drives the public enrollment
operation through subprocess and real transport boundaries.

- A child exits inside request dispatch after pending activation; restart proves
  pending bytes and identity are reused (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store_process_hostile_test.go:580-638`).
- A parent kills a child after the authority receives the request; restart
  proves declaration, proof, and idempotency are identical
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store_process_hostile_test.go:666-753`).
- A child dies after receiving a successful response; restart reconciles,
  installs complete, and a later call performs no network operation
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store_process_hostile_test.go:846-973`).
- The process suite also covers two-process contention and post-verification
  activation contention.

The architecture ratchets pin the dependency allowlist, exact public operations,
exact Store methods, no duplicated derived device fields in persistence, no
maps, goroutines, direct clock, or direct network, and at most three declared
parameters (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/architecture_test.go:13-347`).

Five stable error identities are Core-owned and each has a producing-boundary
test. The removed, unproducible rollback identity is structurally forbidden
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/architecture_test.go:348-391`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/error_lifecycle_hostile_test.go:157-211`).

### Primitive dependency closure

Archived Register directly imports:

- `core`;
- `contextstate`;
- `temporal`;
- `keygen`;
- `attest`;
- `exchange`;
- `controlstate`;
- `filestore`;
- `golang.org/x/term`; and
- platform syscall support.

The dependency allowlist is explicit in the specification
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:23-42`) and structurally checked
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/architecture_test.go:13-59`).

This makes the archived package a late integration package, not a primitive.
It cannot be admitted before all of its Primitive dependencies have their own
2026 decisions. In particular, its `ControlstateTrustedKeys` directly embeds
trusted key sets for Lease, Gate, Callbudget, Status, Release Latest, Release
Manifest, and optional Rate (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:19-46`). Admitting Register as
written therefore freezes a broad commercial DAG even though no consumer
imports Register.

No other archived Primitive package imports Register. Controlstate is returned
by Register, but Controlstate does not depend on Register. The intended
direction is therefore:

`core -> temporal/keygen/attest/contextstate/filestore/exchange/controlstate -> register -> composition root`

The direct `register -> controlstate` edge expands transitively through the
whole lifecycle document.

## Consumer evidence

### Kernel

### Actual use

Kernel has no commercial Register import or call site. Its textual uses of
`register` are primarily account signup and WebAuthn passkey enrollment, a
distinct identity and authority domain.

WebAuthn register finish accepts typed session and credential fields with a
`Validate` boundary (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/webauthn_public_types.go:53-61`). The
authority verifies attestation, burns a single-use ceremony, and performs the
mutation in a store transaction
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/webauthn_public.go:59-96`).

At the transaction boundary it re-reads the user, rechecks active status and the
credential cap, prepares the ledger fact, and writes user, credential, and
ledger together (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/webauthn_public.go:99-133`).

### Local gems

Kernel contributes three useful rules:

- authorize before enrollment and re-authorize inside the mutation transaction;
- burn the single-use challenge before persistent success; and
- derive any summary flag from the authoritative credential set rather than
  maintaining an independent truth.

The last rule comes from a real defect. `HasWebAuthnCredential` previously
drifted from actual credentials and poisoned JWT and ledger facts
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/webauthn_flag_drift_red_probe_test.go:14-28`). The hostile table
pins both stale-false and stale-true repair
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/starter/webauthn_flag_drift_red_probe_test.go:108-172`).

For Primitive Register, the analogous rule is not to persist registration
state, public key, device identity, or proof as independent booleans or copied
fields. The durable signed receipt and key-derived identity are the truths.

### Admission implication

Kernel is not a Register consumer and should not be used to justify the archived
API. Its mutation-boundary and derived-state lessons should be carried into the
authority implementation when that implementation exists.

### Witness

### Actual use

Witness has the closest live duplicate of archived Register:
`witness activate <ACCOUNT-TOKEN>` ensures a local device, persists the account
token, and performs the first check-in (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:120-145`).

The device is minted once and retained (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:220-236`).
However, its identity is a SHA-256 fingerprint of locally generated entropy,
not a signing public key (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/device.go:17-86`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/device.go:191-205`). The activation request therefore
identifies a device but does not carry archived Register's device-key proof of
possession.

The check-in payload binds account token, device fingerprint, binary version and
digest, prior lease progression, nonce, and platform
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:285-324`).

The transport verifies the signed response and nonce binding. Store acceptance
re-verifies grant signature, device binding, exact time commitment, and monotonic
lease/server-time progression under the exclusive lock
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:335-376`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/store.go:163-246`).

### Local gems

Witness contributes:

- release builds pin the authority key at link time and reject a malformed
  stamp instead of falling back (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:64-85`);
- signed grant and signed time commitment bind to the same authority key;
- request nonce and device fingerprint close the response to the request;
- lease generation and server time cannot regress or equal-diverge
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/store.go:249-319`);
- concurrent local clock observations are merged under the store lock; and
- deactivation removes account token and grant while preserving device identity
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:197-209`).

These capabilities belong downstream of enrollment. A rebuilt Register should
return the first verified lifecycle document without absorbing Witness's gate,
clock, or standing behavior.

### Verified defects and migration value

The account token crosses argv as `args[0]` and is parsed there
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:124-130`). It is then persisted as a line of text
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/store.go:95-120`). That exposes a long-lived enrollment
credential to process invocation and durable storage, exactly the surfaces the
archived secret capability forbids.

Activation persists device and token before check-in
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:136-144`). There is no one-record pending enrollment
transaction binding attempt, signing key, proof, and eventual completion.

Accepted check-in writes grant, device, then state as three separate replace
operations (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/store.go:322-329`). The directory lock serializes
writers but does not make three renames one crash-atomic transaction. A crash
after the grant rename can expose a grant with stale device/state.

Archived Register supplies valuable answers to these defects: non-argv bounded
secret ingress, proof-carrying device identity, retry-stable pending state, and
one-file pending-to-complete replacement.

### Bug

### Actual use

Bug has another independent activation implementation. `bug license activate
<DEV-KEY>` parses the developer key from argv, ensures a device, persists the
key, and performs an online activation check-in
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:827-902`).

Unlike Witness, Bug's device owns a writer private seed. The device fingerprint
derives from the writer public key, and `Validate` recomputes the binding
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:16-24`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:64-93`).
`NewDevice` mints the seed, public writer identity, fingerprint, and typed label
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:112-159`).

The online request binds developer key, device fingerprint and label, writer
identity, binary identity, prior lease progression, nonce, platform, and exact
usage window (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:276-310`).

On response, Bug verifies the server-signed grant, exact device/writer binding,
retained writer revocations, signed time commitment, nonce, lease progression,
and usage consumption before accepting the grant
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:321-365`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:311-391`).

Bug also implements an air-gap request/response flow. It persists a typed
pending nonce and device fingerprint, verifies the signed response and request
nonce, ratchets revocations, and installs only a device-bound non-revoked grant
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:26-64`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:401-483`).

### Local gems

Bug contributes capabilities absent from the archived Register design:

- the device identity and evidence-writer identity are one key-derived truth;
- an authority-signed writer certificate binds the local writer to the device;
- retained signed revocation cutoffs fail closed if deleted after a grant;
- online and air-gap activation share the same typed grant validation;
- pending offline challenge is device-bound and single-purpose;
- usage is reset only after the verified grant commits; and
- rollback-resistant certified operation time is coupled to the writer
  credential, not merely to wall time.

The writer certificate and revocation design are especially important if the
registered device will later sign evidence. A 2026 enrollment protocol should
allow the authority response to bind a consumer-owned public capability without
duplicating Bug's writer state machine inside Register.

### Verified defects and migration value

The developer key enters through argv (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:880-892`) and is persisted
as a plaintext line (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:171-195`). This has the same
secret-ingress and retention problem as Witness.

Bug's accepted grant is written before device and state by design
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:345-391`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/store.go:580-590`). The comment makes the tradeoff explicit:
a later failure causes re-reporting rather than silent usage loss. That is
reasonable for usage accounting, but it is not an atomic enrollment completion.

Revocations are ratcheted before a refusal/grant outcome is handled
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:233-248`). This is valuable monotonic authority state, but it
also means activation is a multi-record workflow. A rebuilt package must not
pretend one generic registration-complete boolean captures it.

Archived Register's one-record pending credential is useful for the device
enrollment portion. Bug's writer certificate, revocation set, time authority,
usage accounting, and air-gap workflow remain separate downstream owners.

### Peachfuzz

### Actual use

Peachfuzz has no commercial registration or activation flow. It does have a
local machine-evidence identity that is relevant to device identity design.

`MachineEvidenceIdentity` stores one machine ID and one Ed25519 keypair. Its
constructor derives the machine ID from the public key, and `Validate`
recomputes the binding (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/machine_evidence_identity.go:10-41`).

Signed run evidence verifies that its machine namespace derives from the signing
key ID before verifying the signature
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/signed_run_evidence.go:10-57`). The derivation has one owner:
`MachineIDFromSigningPublicKey`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/signed_run_evidence.go:60-79`).

The application loads one bounded strict identity record or creates it through
Primitive durability with create-only installation and owner-only mode
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon_identity.go:19-84`). It explicitly rejects the retired
unsigned `machine_id` representation instead of carrying a compatibility shim
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon_identity.go:55-62`).

### Local gems

Peachfuzz contributes:

- identity namespace derives from the signing public key;
- a persisted identity stores the source keypair and validates the derived ID;
- signed evidence refuses a machine/key mismatch before signature acceptance;
- identity creation is bounded, create-only, durable, and owner-only; and
- a retired unsigned identity is rejected rather than migrated implicitly.

These reinforce the best parts of archived Register: one key-derived device
truth and clean upgrade boundaries.

### Admission implication

Peachfuzz is not a Register consumer. Its machine identity is evidence
provenance, not commercial enrollment. Primitive should share primitives such
as keypair validation, typed identity derivation, and durable create-only
installation, not force Peachfuzz through Register.

## Strong mechanics and proof

The archive establishes strong proof-carrying registration, strict response
binding, and durable re-verification mechanics. The consumer interviews above
also establish that registration is not one universal workflow. Those facts
support preserving the mechanics while withholding the package from the
current import graph.

## Defects and blockers

### No production authority and no authority replay oracle

Register is a client/local-state package. The server is required to authenticate
the account, atomically consume recovery allowance, supersede devices, persist
the response, and recover byte-identical committed responses
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:307-335`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:476-489`).

The required proof explicitly demands a real authority idempotency oracle:
same-account rotated or consumed secret must recover the committed response;
another account, altered declaration, altered public key, forged proof, and
key-only lookup must fail without disclosure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:681-686`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:709-717`).

No Register test contains the required consumed-key, rotated-key, same-account,
or key-only authority cases. The process tests prove client replay stability
against `httptest` handlers, but those handlers do not implement the
authoritative credential/idempotency database contract.

This is the largest proof gap. The archive can prove it repeats the same request;
it cannot prove the authority safely recognizes and recovers it.

### Pending keypair contradicts the store-one-half contract

The specification says Pending contains exactly the private key bytes and
claims public identity is re-derived so two persisted halves cannot disagree
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:392`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:407`). Production instead persists
the full `core.Ed25519KeyPair`, whose canonical JSON includes both private and
public material, and relies on an unmarshal reconciliation check
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:190`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:85-111`).

The Register struct ratchet misses the duplication because the public key is
nested inside a Core type rather than declared as a direct Register field.
Primitive 2026 must choose a typed custody record whose serialized shape
matches the reviewed derivation contract.

### Native terminal proof exists only for Darwin

The specification requires real subprocess terminal, pipe, and inherited
descriptor tests on every supported native platform
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:635-639`).

The only real terminal echo/restoration hostile test is
`terminal_darwin_hostile_test.go`, guarded by `//go:build darwin`. Windows has a
descriptor-duplication helper and non-Windows has a descriptor helper, but
neither Linux nor Windows has an equivalent native terminal echo/restoration
subprocess test.

The current host's green tests therefore do not satisfy the package's own
cross-platform approval condition.

### Archived monolith freezes unrelated dependency policy

The package combines:

- terminal and descriptor secret ingress;
- key generation and local key custody;
- declaration and proof protocol;
- signed receipt protocol;
- Controlstate trust closure;
- Exchange transport;
- retry scheduling;
- recovery dispositions; and
- Filestore transaction state.

This is why it needs eight Primitive packages plus native terminal support.
A consumer that only needs secret ingress inherits Controlstate and Exchange. An
authority that only needs declaration/proof/receipt types inherits client Store
machinery. A verifier inherits terminal behavior.

The package split is not cosmetic. It is required to keep coupling coefficient
at zero and prevent a late integration package from becoming a universal
commercial dependency.

### Core contains package-private implementation inventory

`core/register_constants.go` is 220 lines. It contains legitimate shared wire
facts, but also Register-only tokens, private Store states, source kinds, file
names, AST type names, method names, source file names, operation counts, field
counts, and test-owner-use counts
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/register_constants.go:3-73`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/register_constants.go:193-210`).

Core also owns 21 Register diagnostic labels
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:147-167`).

Shared error identities, JSON field names, shared account/offering/device
identities, and genuinely cross-package protocol values belong in Core. Private
source kind, Store state, file name, public-file name, method inventory, and AST
ratchet metadata belong to the package that owns them. Moving every literal to
Core does not create a single source of truth; it turns Core into a registry of
another package's internals.

### Secret and key custody policy are fused to enrollment

The archive correctly excludes arbitrary secret readers, but the public package
itself owns native terminal behavior. That makes a security-sensitive protocol
also own UI-adjacent OS mechanics.

The pending record persists the complete Ed25519 keypair in the local JSON
record (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:187-228`). Owner-only Filestore custody is explicit,
and platform keychain integration is a non-goal, but the choice becomes part of
the enrollment package instead of a typed key-custody boundary.

Witness, Bug, and Peachfuzz all need durable local signing identity, but their
capabilities differ. A 2026 design should use one typed custody contract whose
owner selects file-backed or native-protected custody. Register must not expose
a loose callback or generic byte store to achieve that split.

### Trust-key fan-out duplicates Controlstate knowledge

`ControlstateTrustedKeys` repeats the complete nested document signer inventory
inside Register (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/client.go:19-46`). Any Controlstate capability
addition forces Register source changes even though Register's real rule is
simply verify one concrete document under an admitted trust policy.

The Controlstate owner should expose one typed verified-ingress request or one
validated trust bundle. Register should depend on that compiler-owned contract,
not mirror every nested signer field.

### Result and error ownership needs one more review

The package uses a closed `EnrollResult` for installed, already installed,
recovery exhausted, temporarily unavailable, and reconciliation required
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:355-373`). It also returns errors for context, contract,
verification, conflict, and persistence failures.

This division is mostly coherent: authoritative business dispositions are
values; local/protocol failures are errors. Before admission, the authority
response union and local result union must be reviewed together so denial,
recovery exhaustion, reconciliation, and transient failure each have exactly one
owner and no state can be represented both as a disposition and an error.

### Fuzz proof is production-oracle-heavy

The fuzzers prove canonical closure, typed rejection, receiver preservation, and
deterministic re-derivation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/fuzz_test.go:12-220`).

The receipt fuzzer uses production `VerifyReceipt` as the signature/binding
oracle (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/fuzz_test.go:97-121`). The specification asks for independent
binding/signature oracles (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/SPEC.md:739-741`). Admission needs at least
one independent cryptographic oracle or cross-implementation vector suite, not
only production encode/decode/verify closure.

### No explicit no-alias/no-shim architecture ratchet

The exact public-operation and method inventory is pinned, but the architecture
suite does not explicitly reject exported type aliases, compatibility
constructors, forwarding wrappers, or shim packages. The 2026 testing protocol
requires clean upgrades and no compatibility debt, so the rebuilt package needs
an explicit structural ratchet.

## Primitive 2026 ownership and DAG

### Candidate enrollment domain

The smallest reusable domain layer owns:

- `Revision`;
- `EnrollmentKind`;
- nominal `AttemptIdentity`;
- device identity derivation;
- typed declaration;
- `DeviceProof`;
- typed recovery disposition;
- receipt fact/document;
- receipt identity derivation;
- receipt issuance/verification contracts; and
- package-local canonical domains and bounds.

It depends on:

- Core shared identities and stable errors;
- Keygen/key types;
- Temporal;
- Attest; and
- the standard library.

It does not depend on Exchange, Filestore, Controlstate, terminal APIs, or a
product package.

### Candidate secret-ingress capability

Secret ingress owns:

- closed source kind;
- typed inherited descriptor;
- exact framing;
- single-use mutable secret capability;
- destruction and redaction;
- platform no-echo mechanics; and
- native subprocess proof.

It should be a focused package or focused internal subdomain admitted only if
Witness and Bug migrate away from argv. The public contract must remain closed:
no arbitrary reader, path, string, env token, map, or callback escape hatch.

### Candidate enrollment transport

Transport owns:

- initial, recovery, and reconcile endpoints;
- canonical secret-bearing request encoding;
- idempotency derivation;
- Exchange policy;
- typed response union; and
- projection of temporary failure to an exact retry instant.

It depends on the enrollment domain, Core, Contextstate, Exchange, and Temporal.
It does not own durable pending state or terminal input.

### Candidate local enrollment transaction

Local transaction ownership includes:

- injective typed path projection;
- absent/pending/complete record;
- generate-once attempt and device key;
- durable pending before network;
- identical replay;
- compare-before-complete;
- complete only after receipt and lifecycle verification; and
- one-file crash-atomic replacement.

It depends on domain, typed key custody, Filestore, and Contextstate. The
composition root supplies transport and Controlstate verification through typed
structures, not loose callbacks.

### Candidate lifecycle installation

Controlstate remains its own owner. It should accept:

- one concrete document;
- one Controlstate-owned validated trust bundle; and
- one expected account/offering/device binding.

It returns one proof-carrying verified value. Register should not mirror every
nested trust key. Installation of Lease, Gate, Callbudget, Status, Release, and
other nested state remains a composition-root transaction with each owning
package.

### Candidate authority contract

The shared protocol can define a typed authority transition, but the real
authority adapter must prove:

- secret authentication before disclosure;
- account/offering derivation from authenticated authority state;
- device-proof verification;
- unique idempotency record keyed by the complete authenticated match set;
- byte-identical replay across rotated or consumed same-account secrets;
- no key-only or cross-account disclosure;
- atomic recovery allowance consumption and prior-device supersession;
- receipt and lifecycle signing only after authoritative commit; and
- crash/concurrency behavior against the production persistence technology.

An in-memory helper or `httptest` handler is not that proof.

### Resulting DAG

Admitting the archived monolith requires:

`core -> contextstate/temporal/keygen/attest/filestore/exchange/controlstate -> register -> Witness/Bug`

That path is too broad for an initial rebuild and currently has no importing
consumer.

A split, consumer-driven path can be:

`core -> temporal/keygen/attest -> enrollment-domain`

then independently:

- `core -> secret-ingress`;
- `core/contextstate/exchange/temporal/enrollment-domain -> enrollment-transport`;
- `core/contextstate/filestore/key-custody/enrollment-domain -> enrollment-store`;
- `core/attest/... -> controlstate`; and
- a Witness or Bug composition root joins them.

The authority adapter is a separate downstream implementation that shares the
domain types but not the client terminal or local Store.

## Decision rationale and conditions

Do not copy the archived Register package into the initial Primitive 2026
scaffold.

The archive contains exceptional mechanisms worth retaining as design evidence:

- closed non-argv secret ingress;
- single-use mutable secret capability and redaction;
- key-derived device identity and proof of possession;
- retry-stable attempt and compiler-derived idempotency;
- signed receipt bound to exact lifecycle-document digest;
- one-record absent/pending/complete transaction;
- a key-derived device/proof model, while correcting the archived pending
  record's duplicated persisted public key;
- real subprocess crash and contention proof;
- strict bounded canonical encoding;
- stable Core-owned error identity; and
- exact retry instant without a package clock.

The real Witness and Bug flows make this domain worth rebuilding later. They
also show that the archived API cannot simply be copied:

- both must migrate secrets out of argv and durable plaintext retention;
- Witness needs proof-carrying device identity;
- Bug needs writer certificate, revocation, and air-gap capabilities to remain
  separately owned;
- both need a deliberate migration from multi-file partial commits;
- the production authority must implement and prove safe exact replay; and
- lifecycle installation must use the 2026 Controlstate owner contract.

Admission requires all of the following:

1. Witness or Bug is named as the first migration consumer.
2. The secret UX is specified using terminal, stdin, or inherited descriptor,
   with no argv/env/string compatibility path.
3. The package is split by domain, ingress, transport, local transaction, and
   lifecycle ownership, or an equally strict zero-coupling design is proven.
4. Register-only implementation constants and AST inventory move out of Core;
   genuinely shared errors, identities, JSON fields, and protocol facts remain
   Core-owned.
5. Controlstate exposes one typed trust/verification boundary so Register does
   not duplicate nested signer inventory.
6. A typed key-custody boundary is chosen without raw-byte or callback escape
   hatches.
7. A real authority adapter passes consumed-key, rotated-key, same-account,
   cross-account, altered-declaration, altered-public-key, forged-proof, and
   key-only replay tests.
8. Native terminal echo/restoration subprocess proof exists on Darwin, Linux,
   and Windows.
9. Independent cryptographic receipt/proof vectors supplement production
   encode/decode fuzz closure.
10. Explicit no-alias, no-wrapper, no-shim, and no-compatibility ratchets exist.
11. The chosen consumer proves crash recovery and migration from its current
   multi-file state without accepting a silently mixed old/new representation.
12. Primitive, authority, and consumer pin/source/test changes land atomically
   only after explicit review.

Until those conditions are true, the archived package remains high-quality
design evidence, not an admitted Primitive 2026 dependency.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
