# Keygen library package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the exclusive reconstruction report for the archived **library**
package `keygen`. It does not interview `cmd/keygen`; the command appears only
as a downstream composition and persistence consumer.

The archive and all four consumer repositories were read-only. The only
source-tree write made by this interview is this report.

The provisional decision is deliberately two-part:

- Primitive 2026 has strong evidence for one small product-neutral capability:
  construct validated Ed25519 key pairs and bounded secret material from the
  operating-system CSPRNG without owning persistence, product derivation, or
  product policy.
- The July 27 package must not be admitted unchanged. Its exact-fill loop
  silently accepts a final full read carrying an arbitrary non-EOF error; its
  returned key-pair type leaks private bytes through generic formatting; its
  production wrappers use the mutable `crypto/rand.Reader` variable despite
  claiming both an OS-CSPRNG fact and no global mutable state; and the stated
  `sole cryptographic entropy boundary` is contradicted by Primitive and all
  four consumers.

The archive contains unusually good exact-fill, zeroization, canonical-value,
and public-surface proof worth carrying forward. Those strengths do not close
the verified defects or the unresolved 2026 ownership decision. An independent
gap/adjudication pass remains before this report can become `COMPLETE`.

## Evidence boundary


| Source | Exact revision and Primitive pin | Keygen tree | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | `d715fc165b1e73cd5ffeb6d29768c504a3624bca` | One unrelated pre-existing untracked file, `core/api_http_boundary_hostile_test.go`; no archive file changed during this interview. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` | Committed-pin tree `e7f702abb23f2fe87702c7543cb58895a9262a7b`; dirty pin is the exact archive tree `d715fc165b1e73cd5ffeb6d29768c504a3624bca`. | Materially dirty tree, including the Primitive pin and unrelated product work. The dirty pin has the final Keygen API while Kernel production still calls removed symbols. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` | `e7f702abb23f2fe87702c7543cb58895a9262a7b` | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` | `55a65b027eda79b0ca098f775d5315c68f65d65b` | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` | `e7f702abb23f2fe87702c7543cb58895a9262a7b` | `.ledger_pending.md` is modified; production and test sources inspected here otherwise match HEAD. Its source mixes old and final API names against the old pin. |

The final archive package contains six files and 849 lines:

| File | Lines | Role |
| --- | ---: | --- |
| `SPEC.md` | 200 | Draft package contract |
| `architecture_test.go` | 126 | Exact export/type/struct and token ratchet |
| `doctrine_contract.go` | 7 | Doctrine witness |
| `generate.go` | 133 | Private injected-source implementation |
| `generate_hostile_test.go` | 361 | Exact-fill, bounds, clearing, deterministic, and concurrency proof |
| `public.go` | 22 | Three production CSPRNG wrappers |

The common Kernel, Witness, and Peachfuzz pin exports:

```go
GenerateEd25519SigningKey() (core.GeneratedSigningKey, error)
GenerateGarbleCustodySeed() (core.GarbleCustodySeed, error)
GenerateSecretHex(int) (core.GeneratedSecret, error)
```

That implementation calls `ed25519.GenerateKey(rand.Reader)` and `rand.Read`
directly and returns encoding-bearing generated-value structs
(`archive@0df2954a2d911a5d7d775691d023d569affa2c20:keygen/keygen.go:17-71`).

The final archive removes those symbols and exports:

```go
GenerateSigningKey() (core.Ed25519KeyPair, error)
GenerateCustodySeed() (core.SecretMaterial, error)
GenerateSecret(core.ByteCount) (core.SecretMaterial, error)
```

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:32-50` and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/public.go:9-22`.

The source drift is substantial and API-incompatible:

| Consumer baseline | Diff from its Keygen tree to archived HEAD |
| --- | --- |
| Kernel committed pin | 7 files, 842 insertions, 163 deletions |
| Witness pin | 7 files, 842 insertions, 163 deletions |
| Bug pin | 8 files, 844 insertions, 163 deletions |
| Peachfuzz pin | 7 files, 842 insertions, 163 deletions |

No compatibility layer should be created. Primitive's global contract forbids
shims and aliases; each real call site must migrate to the admitted API and the
old generated structs must be removed.

The archive history shows the package was reconstructed rapidly:

1. `2afba21dc342156b554a83ff14843d87cae5fc68` hardened the first generation
   contract on 2026-07-18.
2. `ec86e85c75bca455e6606db2927db557be472f65` added custody/release semantics
   later that day.
3. `627f293cc4a1ac6790c81ec6c12ee0a2a75feb08`,
   `4a57cd1e808c843b4fe312386600bfba9bb37125`, and
   `c11c22e53ab6c6cef1b4cd70c1e67620c7e58151` generalized release and
   Peachfuzz contracts through 2026-07-21.
4. `cf8fa882ee56bb263220a1466c6deae08e466018` established the final primitive
   boundary on 2026-07-24.
5. `d259789e87bcadb829c5ffac72c6c91ccc604098` centralized constants and closed
   capabilities on 2026-07-25.

The final package itself did not change after July 25, but its consumers did
not receive a coherent final-API cutover.

## Capability ownership

The implementation owns three public operations over one private mechanism:

1. `GenerateSigningKey` fills exactly one 32-byte Ed25519 seed, derives the
   64-byte Go private-key representation, validates the private/public
   relationship through Core, and clears both owned temporaries
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:13-29`);
2. `GenerateSecret` validates a typed `core.ByteCount` against the 16 to 64 byte
   Core interval before allocation, fills the exact extent, constructs
   `core.SecretMaterial`, and clears the raw buffer
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:40-70`);
3. `GenerateCustodySeed` selects the maximum 64-byte generic-secret extent and
   delegates to the second operation
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:31-38`).

The private mechanism reads an `io.Reader` until the requested extent is full,
rejects nil/empty contracts, impossible counts, premature errors, 100
consecutive empty reads, and a completely all-zero result
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:73-133`).

The package correctly does **not** own:

- Ed25519 signing or signature-domain policy;
- product key derivation;
- Garble derivation;
- KMS, HSM, secret-manager, configuration, environment, or file storage;
- key rotation;
- product identity or token encodings;
- release or machine namespaces; or
- runtime algorithm selection.

Those exclusions are stated in `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:8-13`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:190-200` and are
mostly reflected in production imports.

The honest 2026 capability is:

> Construct a small closed set of validated key and secret material values from
> Go's production CSPRNG, with no exported entropy provider and no product,
> persistence, derivation, or signing policy.

That is narrower than `Primitive's sole cryptographic entropy boundary`.
Filestore needs random stage-name collision resistance, Exchange needs retry
jitter, and products need typed nonces and identifiers. Those operations do not
become key generation merely because their source is `crypto/rand`.

## Archive evidence

### Archived dependency direction

The final library production DAG is very small:

```text
Go crypto/rand -----+
Go crypto/ed25519 --+--> keygen --> core-owned values
Go io --------------+
```

In Go import direction, `keygen` imports only the standard library and `core`.
No Primitive sibling or product package is imported.

That direction is worth retaining:

- Core owns cross-package byte counts, shared Ed25519 and secret value types,
  canonical parsing/encoding, and stable error identities.
- Keygen owns entropy observation and construction of those values.
- Consumers own why the value exists, when it is persisted, and which product
  domain it enters.

The archive also makes the public package type-free: it declares no production
structs or package types, and the public surface is exactly three functions
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/architecture_test.go:14-31`). That prevents a second informal
value system from growing beside Core.

There are two placement corrections for 2026:

1. `core.KeygenEntropyEmptyReadMaximum` is private implementation policy used
   only by Keygen and its tests. It should be package-owned, not placed in Core
   merely to make it a constant (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen_constants.go:3-5`).
2. `core.KeygenErrFmtCustodySeed`, `KeygenErrFmtMaterial`, and
   `KeygenErrFmtSigningKey` are package-local diagnostic formats, not shared
   cross-package contracts (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:80-82`). Stable
   error identities may remain Core-owned under the repository's global rule;
   local wording should not.

### Archived mechanics worth retaining

### Exact seed derivation

The signing path reads `ed25519.SeedSize` bytes and calls
`ed25519.NewKeyFromSeed`, then delegates structural consistency to
`core.NewEd25519KeyPair`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:13-28`).

This is aligned with Go 1.26.5: Ed25519 uses a 32-byte RFC 8032 seed, while Go's
private-key representation is 64 bytes and includes the public-key suffix.
Go documents `GenerateKey` as equivalent to reading `SeedSize` bytes and
calling `NewKeyFromSeed`
([Go 1.26.5 `crypto/ed25519`](https://pkg.go.dev/crypto/ed25519)).

Core independently re-derives the full private and public state during
validation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:25-63`). The public key is not accepted as
an unrelated input.

### Bounded allocation before effect

`secretByteLength` validates `core.ByteCount`, checks the compiler-owned
minimum and maximum, and only then permits the `uint64 -> int` conversion and
allocation (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:40-70`).

This preserves the package's O(n) bound with `n <= 64`; hostile sizes cannot
trigger large allocation. The size table covers minimum, standard, maximum,
adjacent valid points, zero, one, one-below, one-above, `MaxInt64`,
`MaxInt64+1`, and `MaxUint64`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:136-178`).

### Private injection, not an exported entropy interface

Production calls the private functions with the real source, while tests inject
hostile readers directly (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/public.go:9-22`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:77-92`).

This is a sound shape. It avoids making arbitrary entropy providers part of the
public contract and lets tests deterministically exercise partial reads,
impossible counts, source panic, empty-read livelock, exact byte projection,
and clearing.

### Honest bounded zeroization claim

The archive does not claim perfect erasure from Go stacks, garbage collection,
crash dumps, swap, strings, caller copies, or compiler-created copies. It
promises only to clear each mutable temporary slice it owns on every return
path (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:132-144`).

Production uses `defer clear(...)` immediately after declaring or allocating
the Ed25519 seed, derived private slice, and generic raw material
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:13-20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:40-50`). The retaining-reader test proves the
actual destination passed into the generation path is zeroed after successful
signing, custody, and generic generation and after selected failure paths
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:180-238`).

### Typed error identity

The package distinguishes invalid/impossible requests from entropy-source
failure:

- `core.ErrKeygenContract`;
- `core.ErrKeygenEntropy`, which wraps the contract identity.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:22-24` and
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:146-154`.

The hostile tests use `errors.Is`, not diagnostic string matching
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:118-133`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:160-177`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:219-237`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:319-343`).
That follows the testing protocol's required error-ordering rule
(`foundation@working-tree:_docs/testing_protocol.md:644-677`).

### Core canonical closure

The generated values do not expose mutable backing storage. Core owns:

- strict padded-base64 Ed25519 private-key parsing;
- deterministic public-key derivation and validation;
- strict canonical JSON with private and public halves;
- bounded lowercase-hex secret parsing;
- copy-out access to secret bytes; and
- receiver preservation after rejected JSON.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:20-137`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:139-259`.

The corresponding hostile Core suite covers constructor/parser bounds,
malformed and noncanonical JSON, duplicate and unknown fields, oversized input,
invalid encodings, all-zero material, and receiver preservation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen_hostile_test.go:15-150`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen_hostile_test.go:162-355`). The
`SecretMaterial` JSON boundary also has an oracle-bearing fuzz target
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen_hostile_test.go:306-355`).

### Exact public-surface ratchet

The architecture test enumerates the three permitted public functions and
rejects every exported type, alias, or any production struct
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/architecture_test.go:14-104`).

This is a useful compiler-visible ratchet. The zero-struct inventory is real
rather than a placeholder.

### Primitive-internal dependents

The final archive has three production library dependents and one command
consumer.

### Timeproof

`timeproof.acquire` requests exactly `core.TimeProofNonceBytes`, projects the
secret material into its private typed nonce, and then sends the nonce through
the production acquisition path
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:timeproof/acquire.go:38-68`).

Keygen owns unpredictability; Timeproof owns nonce width, wire meaning, and
network lifecycle. That is the correct division.

### Callbudget

Callbudget uses compile-time array bounds to prove its admission identity fits
inside the Keygen secret interval, generates exactly the product-neutral
identity width, copies it into `AdmissionIdentity`, and clears the temporary
copy (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:19-22`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/values.go:37-76`).

Its architecture ratchet requires the production `keygen.GenerateSecret` call,
so Keygen cannot become a decorative dependency
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:callbudget/architecture_test.go:120-145`).

### Register

Register generates:

- a typed attempt identity from bounded `SecretMaterial`; and
- an Ed25519 key pair for the durable pending enrollment record.

It clears the temporary secret copy before retaining only the product type
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:907-928`). It persists the key pair inside the
canonical store record (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:187-240`).

This use is the most security-sensitive crossing and exposes the archive's
key-pair formatting defect discussed below.

### `cmd/keygen` as downstream composition

The command uses all three library functions, explicitly marshals their private
material, clears the returned encoded byte slice, and delegates create-only,
owner-mode persistence to Filestore
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:53-130`).

Only this command calls `GenerateCustodySeed`; no Primitive library or
external consumer inspected here does. The command does not justify a
duplicate library function by itself.

Rate imports Keygen only from tests to create valid signing fixtures. It is not
a production dependent.

## Consumer evidence

### Kernel: direct generation, product-owned provisioning

Kernel is the largest direct consumer. Its local `keygen` package manages six
environment values: one Ed25519 pair plus CSRF, pepper, HMAC, and encryption
secrets (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:1-12`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:82-118`).

At current source it still calls the old Primitive API:

- `GenerateEd25519SigningKey`, then converts the public hex to base64 and
  retains private base64 for Kernel's environment schema
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:482-508`);
- `GenerateSecretHex(SecretByteStandard)` for each symmetric value
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:510-520`).

Kernel's committed pin contains those symbols. Its dirty pin points at the
final archive where they no longer exist (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76`). A focused
`go test ./keygen -count=1` did not reach those calls because broader dirty-pin
Core removals fail compilation first (`MoneyPennies`, `UnixNanoTime`, and other
unrelated missing Primitive symbols). The line-level Keygen incompatibility is
nevertheless exact: the final public surface contains none of Kernel's called
symbols.

**Local gem to retain downstream.** Kernel's valuable behavior is not random
byte generation. It owns:

- linked private/public environment values;
- preservation of existing non-empty values unless force is explicit;
- detection of a corrupt persisted private half;
- atomic env-file replacement;
- a lock around multi-file provisioning; and
- deterministic reconciliation of `PEPPER_HEX` and `HMAC_KEY_HEX` across
  dev/stage/prod files that share one Firestore database, while keeping CSRF,
  encryption, and Ed25519 keys per file
  (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:120-184`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:201-318`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:347-360`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:keygen/keygen.go:584-626`).

Primitive Keygen should supply typed fresh values. Kernel should continue to
own the product names, encoding projection, preservation, cross-environment
grouping, force policy, and persistence lifecycle.

### Witness: no Primitive Keygen use; strong local custody lifecycle

Witness's pinned Primitive contains the old package, but no Witness production
or test source imports it.

Witness directly creates a project-local Ed25519 trust key with
`ed25519.GenerateKey(rand.Reader)`. It refuses to overwrite an existing private
key, derives a missing public key from the private key, rejects an orphan public
key, and publishes private and public files under different install policies
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/local_key.go:19-45`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/local_key.go:88-143`).

It also generates protocol nonces directly with `rand.Read`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/checkin_nonce.go:12-25`) and injects
`rand.Reader` into update composition for deterministic downstream logic
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/update.go:82-86`).

The package documentation explicitly claims direct `crypto/rand` is a
load-bearing exception and says no private material exists in Primitive
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/license/doc.go:4-10`). The latter sentence is already false
against the archived `core.Ed25519KeyPair` and Register durable record.

**Local gem to retain downstream.** Witness's no-overwrite local-trust-key
lifecycle, private-to-public repair, OpenSSH private-key encoding, and durable
publish policy are product custody semantics
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/local_key.go:99-170`). They should remain above
Primitive Keygen. Primitive may replace the raw key-generation call, but it
must not absorb Witness's paths, OpenSSH format, repair, or install policy.

### Bug: no Primitive Keygen use; typed product seed and identities

Bug's pinned Primitive contains the old package, but Bug has no direct Keygen
import.

Bug instead owns a `WriterPrivateSeed` as a private fixed 32-byte value. It
generates with `rand.Read`, rejects zero and malformed hex persistence, derives
the product writer identity, and signs in the Bug license domain
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_key.go:17-91`).

Bug also directly generates:

- repository identity entropy
  (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/repository_identity.go:197-210`);
- seat invitation tokens and seat identities
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/license/seat_assignment_scalars.go:224-243`, `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/license/seat_assignment_scalars.go:272-277`); and
- deploy/update/release entropy providers injected into higher-level
  environments (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/deploy.go:53-68`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:124-126`).

**Local gem to retain downstream.** `WriterPrivateSeed` separates the persisted
product seed from the derived public writer identity and signing operation.
Repository, invitation, and seat types likewise own their exact encodings and
domains. A 2026 migration may source their raw unpredictable material from
Primitive where appropriate, but the product types, prefixes, JSON, digest,
and signing policy stay in Bug.

### Peachfuzz: direct old API through a product crypto boundary

Peachfuzz routes product crypto through `internal/professor`. Its production
paths call:

- old `GenerateSecretHex`, after converting the required hex-text width into a
  byte count, to construct a product `RunID`
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/professor/run_identity.go:9-26`);
- old `GenerateEd25519SigningKey` to construct
  `MachineEvidenceIdentity`
  (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/professor/run_evidence.go:9-17`).

Retry jitter remains a deliberately local raw `crypto/rand` operation
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/professor/retry_jitter.go:13-23`).

The module-level architecture test requires all crypto imports to route through
Professor and explicitly allowlists only its SHA-256 and retry-jitter files
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/professor/crypto_boundary_test.go:19-66`).

Current Peachfuzz source is internally version-skewed: production and one
archive test call old symbols while `protocol/run_stats_test.go` calls final
`GenerateSigningKey`; the module still pins the old tree
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/run_stats_test.go:188-243`,
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence_test.go:136-138`). A focused test
attempt could not reach compilation because the remote bytes for the pinned
pseudo-version failed Go checksum verification against `go.sum`. This is not
Keygen behavioral proof; it is additional evidence that no reproducible,
coherent consumer cutover exists.

**Local gem to retain downstream.** Professor is Peachfuzz's product crypto
composition boundary. It maps product-neutral Primitive material into RunID
and MachineEvidenceIdentity and structurally prevents crypto from spreading
through the product. Primitive should not absorb that product composition or
the explicitly separate retry-jitter policy.

## Strong mechanics and proof

### Testing-protocol proof

The project-local `_docs/testing_protocol.md` was read completely before this
interview. The archive proof was assessed against it, not merely against green
test output.

| Protocol contract | Archived proof | Assessment |
| --- | --- | --- |
| `test/evidence`: assertions must catch a real production regression (`foundation@working-tree:_docs/testing_protocol.md:149-170`) | Deterministic reader projection, exact-fill failures, retained-buffer clearing, typed constructor closure (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:84-277`) | Strong for private generation helpers. |
| `test/determinism`: control randomness (`foundation@working-tree:_docs/testing_protocol.md:356-380`) | Private readers deterministically script every byte and failure (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:16-82`) | Strong. The public distinctness test is supporting evidence only, as the spec honestly says. |
| `test/table-shape` and `test/boundaries` (`foundation@working-tree:_docs/testing_protocol.md:382-429`, `foundation@working-tree:_docs/testing_protocol.md:466-516`) | 23 exact-fill cases and 14 size cases (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:84-178`) | Exact-fill table is strong. The serious size boundary has only 7 accepting and 7 rejecting cases, below the normal 10/10/20 floor; it needs expansion or an explicit argument that the relevant interval dimensions are exhausted. |
| `test/errors` (`foundation@working-tree:_docs/testing_protocol.md:644-677`) | `errors.Is` for contract and entropy roots throughout | Strong. |
| `test/production-path` (`foundation@working-tree:_docs/testing_protocol.md:862-891`) | `GenerateSigningKey` reaches the installed OS source in the concurrency test (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:279-312`) | Partial. No public success test calls `GenerateSecret` or `GenerateCustodySeed`; helper injection is not proof of those public wrappers. |
| `test/goroutines/owned` (`foundation@working-tree:_docs/testing_protocol.md:965-1005`) | 32 goroutines, a `WaitGroup`, and a buffered result channel (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:279-312`) | Fails protocol: there is no cancel path or timeout backstop. A wedged entropy read wedges the test indefinitely. |
| `test/structural-invariant` / package exit (`foundation@working-tree:_docs/testing_protocol.md:613-642`, `foundation@working-tree:_docs/testing_protocol.md:1008-1044`) | Exact export/type/struct ratchet and a forbidden-token scan (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/architecture_test.go:14-126`) | Useful but incomplete: the token list does not structurally forbid filesystem/process effects or arbitrary sibling imports promised by the spec. |
| `test/fuzz-boundary` (`foundation@working-tree:_docs/testing_protocol.md:1210-1288`) | No library-package fuzzer; Core fuzzes strict `SecretMaterial` JSON closure | Appropriate for the private, bounded reader loop if hostile tables become exhaustive. The external Core parser owns the actual fuzzable byte boundary. |
| Layer triad and evidence-path rules | Keygen has no evidence ledger, fold, or multi-layer policy pipeline | Truthfully `NOT_APPLICABLE` to this small generator. |

### Reproduced archive-local gates

At archived HEAD:

- `go test ./keygen -count=1`: PASS;
- `go test -race ./keygen -count=1`: PASS;
- `go test ./keygen -shuffle=on -count=10`: PASS;
- `go test -cover ./keygen -count=1`: PASS, 92.8% statements;
- `go vet ./keygen`: PASS;
- `staticcheck ./keygen`: PASS;
- `GOOS=linux GOARCH=amd64 go test -c ./keygen`: PASS;
- `GOOS=windows GOARCH=amd64 go test -c ./keygen`: PASS;
- `GOOS=darwin GOARCH=arm64 go test -c ./keygen`: PASS.

The cross-build outputs were directed outside the repositories and no
persistent artifact was retained.

These gates establish local compile, race, repetition, analysis, coverage, and
cross-build facts. They do not establish that the exact-fill semantics are
correct, that generic formatting is safe, or that consumers compile against the
final API.

## Defects and blockers

### B1: a final full read with an arbitrary error is accepted

The specification is explicit:

- final remaining bytes plus `io.EOF` are accepted;
- **any other read error** is rejected
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:81-89`).

`readEntropy` does the opposite for the exact-final-read case. It:

1. receives `(count, readErr)`;
2. validates `count`;
3. advances `offset`;
4. returns `nil` immediately when the destination is full; and only then
5. checks `readErr`.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:95-122`, especially lines 102-111.

Therefore a reader returning `(len(remaining), errEntropySource)` succeeds and
its bytes become key or secret material. The hostile table covers a partial
read plus arbitrary error and a final read plus EOF, but omits the decisive
final-full-read-plus-arbitrary-error case
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:94-116`).

This is a direct implementation/spec contradiction at the package's only trust
boundary. Admission requires the new red test and a control-flow fix that
accepts an error on the final fill only when `errors.Is(readErr, io.EOF)`.

### B2: the production CSPRNG fact is not established under Go 1.26

The spec says values returned by this package carry the operational fact that
the production CSPRNG filled them, production uses only `crypto/rand.Reader`,
and the package has no global mutable state
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:73-79`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:156-160`).

The public functions pass the exported package variable `rand.Reader` into the
private helpers on every call (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/public.go:3-22`). Go documents
`crypto/rand.Reader` as a global shared variable. The archive does not capture
or protect its identity, so application code can replace it; any constant
non-zero reader passes the package's all-zero check and produces predictable
material.

Go 1.26 also deliberately hardened the preferred public APIs:

- `crypto/rand.Read` now always fills the buffer and irrecoverably crashes on
  secure-source failure;
- `ed25519.GenerateKey(nil)` uses a secure source, with the former custom-global
  behavior only temporarily restorable through
  `GODEBUG=cryptocustomrand=1`.

See [Go 1.26.5 `crypto/rand`](https://pkg.go.dev/crypto/rand) and
[Go 1.26.5 `crypto/ed25519`](https://pkg.go.dev/crypto/ed25519).

By calling `rand.Reader.Read` through its own loop, the archive bypasses those
Go 1.26 production semantics while its source-basis section expressly claims
Go 1.26.5 (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:15-30`).

The package must make one truthful 2026 choice:

- use `ed25519.GenerateKey(nil)` for signing and `rand.Read` for generic
  material, retain separately injected private helpers for unit proof, and
  state explicitly that same-process reassignment of the exported
  `rand.Reader` lies outside the CSPRNG guarantee; or
- stop claiming that every public result proves the OS CSPRNG filled it and
  explicitly own the replaceable-source assumption.

Calling `rand.Read` would improve exact-fill/error behavior. It always fills or
irrecoverably fails in Go 1.26, but it still consults an overridden
`rand.Reader`. It is not by itself proof against later application-level
replacement. If Primitive requires an absolute guarantee against code in the
same process replacing that variable, the public Go API does not supply that
generic-byte guarantee and the source design needs separate adjudication.

It cannot simultaneously promise replaceable-reader error recovery, no global
mutable state, and Go 1.26's protected production CSPRNG fact.

### B3: `core.Ed25519KeyPair` leaks private material through `fmt`

`core.Ed25519KeyPair` stores the complete private key in an unexported
64-byte array and deliberately exposes it only through explicit text/JSON
persistence (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:20-23`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:78-100`).

Unlike `core.Ed25519SigningKey` and `core.SecretMaterial`, it does **not**
implement `fmt.Formatter`. Generic `%v`, `%+v`, `%#v`, and reflection-based
formatting can therefore print its unexported byte array.

The redaction suite covers the signing key and generic secret only; it omits
the key pair (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/secret_format_hostile_test.go:13-67`). The
repository architecture ratchet discovers only types that already declare a
`Format` method, so it cannot detect this missing declaration
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:22-25`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:37-78`).

This is not hypothetical. Register stores the pair in `storeRecord`, and its
canonical round-trip failure prints the entire record with `%v`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/store.go:187-195`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:register/json_boundary_hostile_test.go:89-103`). Keygen and Core tests
also format pairs with `%+v` on failure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:240-258`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:319-339`).

The package spec says diagnostics never contain private material
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:146-154`). Admission requires:

- redacting `Ed25519KeyPair` on every generic formatting surface;
- hostile redaction tests for raw, base64, and hex spellings plus the zero
  value; and
- an enclosing-Register-record test proving the private key does not appear.

Canonical text and JSON are explicit persistence crossings and may continue to
contain private material; generic diagnostics may not.

### B4: `sole cryptographic entropy boundary` is false

The archive claims Keygen is Primitive's sole cryptographic entropy boundary
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:6-13`).

Primitive production itself contains at least:

- Filestore stage-token generation through `io.ReadFull(rand.Reader, token)`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/write.go:85-100`);
- Exchange retry jitter through `rand.Read`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/client.go:807-815`).

The global capability architecture ratchet governs time, process, filesystem,
and network imports, but does not assign `crypto/rand` to Keygen
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/capability_architecture_test.go:892-906`).

Every consumer also has direct entropy:

- Kernel owns product orchestration over Primitive generation;
- Witness creates keys, nonces, device material, and profile seeds directly;
- Bug creates writer seeds, repository IDs, invitation/seat values, and
  release/update entropy directly;
- Peachfuzz deliberately allows raw retry jitter inside Professor.

This is an ownership-contract defect, not merely a missing call-site migration.
For 2026, narrow the claim to **key and secret material construction**. Keep
operational randomness with its effect owner: Filestore owns collision-resistant
stage names, Exchange owns retry jitter, and product packages own typed product
identity/nonce policy while delegating secret/key construction where useful.

Do not export a generic `io.Reader`, `Fill`, or entropy-provider interface from
Keygen just to make the `sole` sentence true. That would broaden the capability
and erase the typed result boundary.

### B5: `GenerateCustodySeed` is an untyped duplicate operation

The spec says the three operations stay distinct and calls custody a long-lived
derivation root (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:42-50`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:114-122`).

The compiler cannot see that distinction. `GenerateCustodySeed` merely calls
`GenerateSecret(maximum)` and returns the same `core.SecretMaterial` type
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate.go:31-38`). A caller cannot distinguish a custody
root from any other 64-byte generic secret after return.

Only `cmd/keygen` calls the function. No library dependent and no inspected
consumer uses it.

This conflicts with the global prohibition on dead wrappers and informal
contracts. Primitive 2026 should choose exactly one:

- remove `GenerateCustodySeed`; the command can explicitly call
  `GenerateSecret(core.NewByteCount(core.SecretMaterialByteMaximum))`; or
- introduce a genuinely distinct, product-neutral typed derivation-root value
  with its own validation and lifecycle evidence.

Retaining a duplicate function whose distinction exists only in its name is not
an admissible compiler-owned contract.

### B6: the architecture ratchet proves less than the spec claims

The spec requires structural proof of the absence of product tokens, runtime
generator enums, aliases, entropy exports, filesystem I/O, process execution,
and sibling imports (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:168-185`).

The test proves:

- exact exported functions;
- zero declared types and structs; and
- absence of ten hard-coded lowercase substrings
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/architecture_test.go:14-126`).

It does not AST-check:

- imports of `os`, `os/exec`, or arbitrary Primitive siblings;
- filesystem/process selectors;
- exported variables or constants;
- exported reader/provider values;
- runtime string kinds under names not in the token list.

The current source happens to be narrow, but the ratchet would allow future
violations the spec says must break the build. Admission requires an AST/type
ratchet over the actual forbidden dependency and export shapes, with
non-vacuity floors.

### B7: production-wrapper and goroutine proof is incomplete

The public production-source test calls only `GenerateSigningKey`, and does so
from 32 goroutines without a cancel path or timeout backstop
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/generate_hostile_test.go:279-312`).

This violates `foundation@working-tree:_docs/testing_protocol.md:965-1005`. It also leaves
`GenerateSecret` and `GenerateCustodySeed` without native public success proof,
despite the spec requiring native success tests for all three target behaviors
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:keygen/SPEC.md:162-166`).

The 2026 suite must:

- prove each retained public wrapper reaches the production source;
- avoid an unbounded goroutine wait, or give every test-started goroutine an
  owner, cancel path, wait path, and timeout backstop;
- add final-full-read-plus-arbitrary-error coverage;
- cover clearing for each retained operation on success, partial failure,
  all-zero failure, and derived-state failure where constructible; and
- expand or justify the serious size table against the protocol's 10/10/20
  adversarial floor.

### B8: consumer migration is neither coherent nor reproducible

No inspected consumer is cleanly on the final API:

- Kernel's committed pin and source agree on the old API; its dirty pin changes
  to the final tree without changing Keygen calls.
- Witness and Bug do not use Primitive Keygen.
- Peachfuzz source mixes old and final symbol names while pinning the old tree;
  its pinned module download also failed checksum verification against its
  committed `go.sum`.

Archive-local green tests therefore do not prove adoption. Admission needs one
clean migration slice per direct consumer, no aliases, and crossing tests that
prove product projection and private-material handling.

## Primitive 2026 ownership and DAG

The recommended package graph is:

```text
core --> keygen
core --> timeproof
core --> callbudget
core --> register

keygen --> timeproof / callbudget / register
keygen + filestore --> cmd/keygen composition

crypto/rand --> keygen      (key/secret material)
crypto/rand --> filestore   (stage-name collision resistance)
crypto/rand --> exchange    (retry jitter)
```

The arrows denote import/use direction, not lifecycle ownership.

### Core owns

- `ByteCount` and truly shared byte-width constants;
- a redaction-safe `Ed25519KeyPair` only if more than one package shares its
  canonical persistence shape;
- `Ed25519SigningKey`, public key, signature, and `SecretMaterial` value
  contracts;
- strict canonical parsing/JSON and receiver preservation;
- cross-package stable error identities required by the global instructions.

Core must not own package-private empty-read thresholds or Keygen-only
diagnostic wording.

### Keygen owns

- the closed public generation operations;
- production CSPRNG selection under one truthful Go 1.26 policy;
- exact-fill behavior only where a custom private test source remains useful;
- rejection of structurally unusable generated material;
- clearing of mutable temporary buffers it directly owns;
- package-specific context around Core error identities;
- an exact export/import/effect ratchet.

It does not own persistence encodings beyond the Core value contract, product
types, storage, rotation, derivation domains, retry jitter, stage naming, or a
generic exported entropy capability.

### Consumers own

- conversion from Primitive values to product IDs, machine identities, writer
  seeds, env names, and protocol nonces;
- persistence location, create/replace/no-overwrite policy, and recovery;
- key rotation and custody lifecycle;
- product signing domains and derivation inputs;
- explicit clearing of copies they request from Core;
- crossing tests proving Primitive material becomes the right product value
  without diagnostic disclosure.

### Public API recommendation

Retain, after defects are corrected:

```go
func GenerateSigningKey() (core.Ed25519KeyPair, error)
func GenerateSecret(size core.ByteCount) (core.SecretMaterial, error)
```

Retire `GenerateCustodySeed` unless adjudication produces a distinct
product-neutral derivation-root type. Do not restore
`GenerateEd25519SigningKey`, `GenerateGarbleCustodySeed`, or
`GenerateSecretHex`; update consumers directly.

Before implementation, adjudicate whether the persisted Ed25519 contract should
be the complete 64-byte Go private key or the canonical 32-byte RFC 8032 seed.
Witness and Bug demonstrate that consumers have legitimate product-specific
formats. Primitive should own one shared persistence form only if at least two
real consumers require the same form; it must not force OpenSSH or product seed
schemas into Keygen.

## Decision rationale and conditions

### Admission checklist

The Keygen implementation remains unready until:

1. the exact-final-read arbitrary-error case is red and then fixed;
2. production CSPRNG selection is truthful under Go 1.26 and does not rely on
   an unprotected mutable global while claiming otherwise;
3. `core.Ed25519KeyPair` and every enclosing diagnostic surface redact private
   bytes under generic formatting;
4. the package claim is narrowed from all entropy to typed key/secret
   construction, or a separately reviewed capability architecture proves a
   different owner;
5. `GenerateCustodySeed` is removed or returns a genuinely distinct typed
   contract;
6. the structural ratchet enforces the promised import/effect/export boundary;
7. public production-path and goroutine-ownership proof conforms to the local
   testing protocol;
8. size and clearing tables close their stated hostile dimensions;
9. Kernel and Peachfuzz receive clean, shim-free final-API migrations with
   product crossing tests;
10. Witness and Bug explicitly decide whether their key/secret creation moves
    to Primitive or remains a documented product-owned exception;
11. the final package gates, race, shuffle, coverage, vet, staticcheck, and
    three cross-builds are rerun on the corrected package; and
12. an independent gap/adjudication review accepts the corrected evidence.

### Recon implications

**Retain the capability; reject the July 27 archive as-is.**

The archive establishes a valuable small boundary: typed size validation before
allocation, deterministic Ed25519 seed derivation, private hostile-reader
injection, bounded exact-fill handling, honest owned-buffer clearing, typed
errors, Core canonical closure, and an exact public-surface inventory.

It is not promotion-ready. The final-read error bug violates the written
entropy contract; the returned key-pair can disclose private bytes through
ordinary diagnostics; Go 1.26 production-source semantics and the no-global
claim are inconsistent; the sole-entropy statement is false; custody is a
name-only wrapper; architecture and production-wrapper proof are incomplete;
and the consumer graph has no coherent final-API migration.

Primitive 2026 should rebuild the narrow two-function capability from the
verified mechanics, correct the blockers, preserve consumer-owned product
policy, and admit it only after independent review. No compatibility facade,
commit, or push is authorized by this interview.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
