# Objectstore package interview

Status: `COMPLETE` | Decision: `REDESIGN`

## 2026-08-08 authenticated GCS scope amendment

The signed-capability reconstruction below remains the evidence for the first
three doors. Its exclusion of provider authentication, bucket clients, listing,
deletion, metadata, and cloud SDKs is superseded by the product-neutral fourth
door admitted on 2026-08-08.

The fourth door owns the official GCS SDK client and exactly these operations:

- Application Default Credentials or one typed service-account file;
- create-only exact whole-object streaming with SHA-256 and CRC32C;
- exact whole-object streaming read;
- structured content type, cache policy, and custom-time metadata; and
- bounded prefix listing, generation-matched deletion, and post-delete absence
  proof, with refusal when bucket soft-delete retention is enabled.

Products still own bucket selection, object namespaces, retention intent,
reconciliation, and retry policy. Primitive still does not create buckets,
mint or persist credentials, change lifecycle policy, copy, compose, expose SDK
types, or build product workflows. The provider proof uses real GCS: exact
create/read/delete and integrity-refusal paths run against a soft-delete-disabled
bucket, while a second real bucket proves that permanent deletion refuses a
seven-day soft-delete policy.

This is the sole reconstruction record for the proposed Primitive
`objectstore` package. The archive is evidence, not authority. No archived
source was copied.

## Evidence boundary


### Source revisions and consumer pins

| Source | Revision or pin | Objectstore availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | Present. First introduced by `a00668941af086e23a29f50bcb330e41ad10cfd8` on 2026-07-24. | One unrelated pre-existing untracked file, `core/api_http_boundary_hostile_test.go`; no archive file was changed during this interview. |
| Kernel committed HEAD | `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | Committed `go.mod` pins Primitive `0df2954a2d911a5d7d775691d023d569affa2c20`, which predates `objectstore`. | Kernel has a broad pre-existing dirty migration. Its dirty `kernel@working-tree:go.mod:76` advances Primitive to `e8b7172161a4994efcb7f092113e23c28928da43`, where `objectstore` exists. |
| Witness | `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins Primitive `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9`, which predates `objectstore`. | Only the pre-existing untracked ledger was observed. |
| Bug | `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins Primitive `388e593231a28434f6faae9f0ab9dffcf332dfc3`, which predates `objectstore`. | Only the pre-existing untracked ledger was observed. |
| Peachfuzz | `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins Primitive `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75`, which predates `objectstore`. | Only the pre-existing modified ledger was observed. |

No committed consumer pin contains the archived package. Kernel's dirty
Primitive frontier contains it, but no Kernel production file imports it.
Therefore the archived implementation has not received a real downstream
cutover proof from any of the four consumers.

The archived implementation evolved through the following material revisions:

- `a00668941af086e23a29f50bcb330e41ad10cfd8`: initial objectstore primitive;
- `d259789e87bcadb829c5ffac72c6c91ccc604098`: centralized constants and
  closed capabilities;
- `10b7fa7131d609cdecafe7212537e2d9898286b0`: completed absence projection;
- `89fb27ff6f328e478d71f266604de8af87fdecdc`: hardened signed-URL boundaries;
- `40ded9c104a99cbc4b0b672cd7392901b468d1eb`: comparative contract hardening.

## Capability ownership

### Admission question

The corrected `v2026.0.0` candidate needs one product-neutral primitive for
transferring a known finite object through an already-issued, expiring GCS
signed HTTPS capability.
It must prove exact extent, SHA-256, and CRC32C while streaming, enforce
create-only upload semantics, reject redirect and range drift, preserve typed
failure identity, and expose trustworthy transfer evidence.

That is a coherent Primitive responsibility. It occurs independently in
Witness custody, Bug release deployment, and higher-level archived Primitive
packages. Peachfuzz also demonstrates the importance of immutable-write
reconciliation and live-provider evidence, although its authenticated GCS JSON
API adapter is a different capability.

### Ownership boundary

### Objectstore owns

- the closed GCS provider identity;
- one opaque, expiring signed HTTPS bearer capability;
- whole-object `PUT` and `GET`;
- exact declared byte extent;
- streaming SHA-256 and CRC32C verification;
- create-only upload intent;
- the GCS signed-header lattice;
- redirect rejection and single-attempt stream execution;
- provider conflict and absence projection;
- provider generation/version capture when returned;
- a typed, immutable transfer result, including commitment certainty;
- product-neutral stable error projection;
- O(1) memory in object size.

This matches the archive's narrow statement of purpose
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:7-38`) and its dependency direction through
`core`, `contextstate`, `exchange`, and `temporal`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:40-55`).

### Objectstore does not own

- bucket names, object paths, tenant namespaces, or product retention;
- grants, receipts, manifests, release identity, upload-attempt identity, or
  content-addressed archive layout;
- signing credentials or signed-URL creation;
- provider authentication, bucket configuration, lifecycle policy, listing,
  deletion, copy, compose, or metadata mutation;
- local staging, durable activation, rollback, or cleanup;
- resumable upload, multipart upload, ranges, append, or unknown-length
  streams;
- product retry, reconciliation, or recovery workflows.

The archive makes the same exclusions at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:26-38` and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:823-840`.
Peachfuzz's bucket API and Witness/Bug's release bindings must therefore remain
downstream even though their mechanics inform the primitive.

## Archive evidence

### Archived architecture worth retaining

### Small public intent surface

The archive exposes only `NewClient`, `Upload`, `Download`, `ParseProvider`, and
`ParseSignedURL` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/public.go:10-42`). Runtime mechanics stay
private. `Provider`, `WriteMode`, and `Direction` are closed enums with invalid
zero values and canonical JSON tokens
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/enums.go:11-204`).

Retain the small intent surface. Do not retain compatibility names, provider
adapter interfaces, public request builders, public hashers, or public retry
machinery.

### Opaque bearer capability

`SignedURL` keeps its bytes private and redacts formatter output
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/values.go:18-27`). Validation requires HTTPS, no userinfo
or fragment, a non-root path, a non-empty query, visible ASCII, and exact
canonical round-trip (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/values.go:29-60`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/http_contracts.go:19-56`). Only the private Exchange target
projects the URL onto the request line, while its formatter remains redacted
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:22-50`).

This is materially stronger than the consumer implementations and should be
retained. A signed bearer type must not expose `String()`.

### Exact upload proof

Upload validates before source access, checks expiration, uses a fixed-buffer
exact-extent reader, calculates SHA-256 and CRC32C on the stream, declares an
exact content length, rejects replay, and closes the result only after extent
and digest proof (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:163-203`). The exact reader
withholds the final declared read until a one-byte lookahead proves EOF and
contains hostile reader behavior (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/exact.go:11-148`).

This corrects a common `io.LimitReader` failure mode: merely limiting a source
does not prove that the source ended at the declared extent.

### Exact download proof and honest partial progress

Download hashes bytes while sending them to the caller's writer, uses an exact
response bound, rejects partial-content responses, and verifies byte count,
SHA-256, and CRC32C before success
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:205-242`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:346-376`). It does not falsely claim
atomic destination behavior; the caller owns staging and rollback
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:471-485`).

The absence projection is deliberately narrow: only a typed provider `404`
with zero committed destination bytes is absence; partial progress becomes
integrity failure (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:244-266`).

### Provider lattice

The archive validates:

- GCS upload CRC32C and `x-goog-if-generation-match: 0`;
- S3 upload CRC32C and `If-None-Match: *`;
- GCS/S3 foreign-header exclusion;
- S3 download checksum mode;
- provider-specific maximum upload/download extents;
- GCS `412` and S3 `409`/`412` create-only conflicts;
- provider `404` download absence.

The implementation is at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/headers.go:70-167`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/headers.go:205-230`.
Compiler-visible size and wire ceilings are at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/objectstore_constants.go:4-33`.

Retain the behavior, but reconstruct its public header carrier as an immutable
typed contract rather than copying the archived raw-string DTO.

### Error disclosure and stable identity

The archive projects only a closed stable error set and a typed Exchange status
error, specifically to prevent signed URLs and arbitrary hostile stream errors
from escaping (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:268-309`). Core owns the
objectstore identity lattice
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:177-184`). Hostile disclosure tests cover
formatting and transport failures
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/signed_url_format_hostile_test.go:9-48`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/disclosure_hardening_test.go:48-103`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/disclosure_hardening_test.go:149-184`).

Retain stable `errors.Is`/`errors.As` identity and secret withholding. Rebuild
the graph walk so its bounded behavior cannot silently discard an applicable
stable identity.

### Proof shape

The deterministic archive suite covers:

- request-validation layer triads;
- upload/download production paths and scale boundaries;
- status, conflict, absence, cancellation, deadline, redirect, writer, source,
  and exact-extent failures;
- provider-version projection;
- source-size-independent allocation checks;
- enum, URL, header, JSON, and provider-size hostile tables;
- semantic fuzz targets for enums, signed URLs, digests, and extent.

The test entry points are enumerated at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/exact_hostile_test.go:1-198` and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/fuzz_test.go:1-279`;
representative production-path tests begin at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/transfer_hostile_test.go:140`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/transfer_hostile_test.go:449`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/transfer_hostile_test.go:674`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/transfer_hostile_test.go:1098`.

`go test ./objectstore` passed at archived HEAD during this interview. That is
ordinary deterministic package evidence only, not a release gate or
live-provider result.

### Archived Primitive dependents

A complete production import search found exactly two archived Primitive
packages importing `objectstore`: `submission` and `upgrade`.

### Submission

Submission carries `objectstore.Integrity`, an opaque create-only
`objectstore.UploadTarget`, and a caller policy inside a typed upload grant and
transfer request (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/grant.go:9-71`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:22-49`). It:

- binds the grant to the declaration before transfer;
- preflights the observed time against the grant expiry;
- invokes `objectstore.Upload`;
- requires provider version evidence for a confirmed result;
- separates confirmed from indeterminate completion;
- classifies expiration, verification, source/size contract failure, and
  ambiguous remote outcome by stable identities.

The execution and classification are at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:129-268`. The bearer target is deliberately
kept out of ordinary result persistence and cloned at ownership boundaries
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/grant.go:9-10`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/grant.go:82-85`).

Gem to retain: a higher-level protocol must bind the signed target to its own
declaration, authorization, object identity, and expiry. That product binding
does not belong in `objectstore`.

Lesson for Primitive 2026: Submission has to infer an indeterminate write from
a list of transport identities. Objectstore should instead expose a closed
compiler-owned commitment outcome derived from its own execution facts.

### Upgrade

Upgrade composes `objectstore.Download` with a `filestore.WriteStage`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:51-104`). It aborts the stage on transfer or
integrity failure, commits only verified bytes, requires durable activation and
temporary removal, and persists the recovery transition afterward
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/download.go:105-183`). Its request boundary directly owns an
objectstore client, download target, and policy
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:upgrade/types.go:155-171`).

Gem to retain: objectstore must remain non-durable and accept an `io.Writer`;
durable download is composition with Filestore, not a second filesystem
implementation.

## Consumer evidence

### Kernel

There is no object-transfer implementation and no direct import of archived
`objectstore` at committed HEAD or in the observed dirty tree.

Kernel has AWS/S3 configuration fields
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/compass.go:130-143`) and reports S3 as an enabled external when
credentials and a bucket are present
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/webapp/ceremony_wiring.go:590-604`). Its bridge explicitly states
that this is a ceremony reporter, not an HTTP client
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:bridge/bridge.go:45-49`).

Finding:

- no objectstore capability can be extracted from Kernel;
- Kernel's AWS credentials, endpoint, bucket, CloudFront, and ceremony facts
  remain Kernel-owned;
- the report-only S3 label is not proof of object storage behavior and must not
  be treated as a consumer cutover.

### Witness

Witness has no direct `objectstore` import because its Primitive pin predates
the package. It nevertheless contains two relevant protocol surfaces.

#### Custody upload and download

Custody defines typed artifact descriptors and signed upload targets
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:12-41`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:199-227`). Its upload:

- streams a known extent through a final-byte EOF proof and SHA-256;
- uses Exchange as a single-attempt signed `PUT`;
- captures the GCS generation;
- validates exact bytes and digest before producing an uploaded-object fact.

See `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/client.go:193-327` and the exact reader at
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/artifact_extent.go:11-85`.

Custody download verifies a server-signed grant, binds it to the receipt's
artifact/object/hash/size set, and offers an explicit time-window check
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/download.go:215-305`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/download.go:361-382`). Its transfer streams
through SHA-256 with an exact response limit
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/download.go:384-457`).

Useful product-owned mechanics:

- signed grant verification;
- grant-to-receipt and target-to-artifact binding;
- BLAKE3 plus SHA-256 content addressing;
- upload generation in the custody receipt;
- explicit clock-skew policy.

Verified lessons:

- `DownloadURL.String()` exposes the bearer
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/download.go:28-48`); Primitive must keep the URL
  opaque and redacted.
- The upload target has no target expiry and `UploadArtifact` does not enforce
  the session grant's expiry at execution
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:90-110`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:199-227`;
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/client.go:225-271`).
- `DownloadGrantBody.ValidateAt` is optional caller choreography rather than an
  execution-owned boundary (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/download.go:258-270`);
  `DownloadArtifact` accepts a detached target without the grant window.
- Custody proves SHA-256 and extent but does not locally calculate CRC32C on the
  transfer path. The archived primitive's dual digest and provider checksum
  contract is stronger.

No production caller outside `protocol/custody` was found for these client
methods. They are implemented and tested protocol capabilities, not yet
evidence of an end-to-end Witness command path.

#### Release upload binding

Witness Release derives a typed upload binding over release ID, manifest and
artifact digests, artifact size, product, provider, bucket, object, and a fresh
attempt ID (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/upload_binding.go:124-179`). It requires
provider-specific signed metadata for the attempt and binding plus a
create-only precondition
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/upload_binding.go:181-259`). Upload targets carry
that binding and expiry
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/release/models.go:235-294`).

This is the strongest downstream gem. It prevents a valid signed URL from being
replayed for the wrong release artifact or attempt. It remains Release-owned:
objectstore must carry additional validated signed headers unchanged, but must
not know release IDs, product names, buckets, object keys, or binding schemas.

### Bug

Bug has no direct `objectstore` import because its Primitive pin predates the
package. It consumes the same typed Release plan and upload-binding protocol as
Witness.

The candidate deployment lifecycle verifies a signed prepare response, uploads
each manifest artifact, constructs a typed finalize request, and verifies the
signed finalize response (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:143-235`). Before
upload it reopens the file beneath an owned root and verifies regular-file size
and manifest identity (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:278-348`).

The actual HTTP uploader uses a raw signed URL string, `http.MethodPut`,
`io.LimitReader`, copied raw headers, status `200`, and SHA-256
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:287-388`).

Useful product-owned mechanics:

- signed prepare/finalize handoff;
- release manifest-to-target binding;
- fresh upload-attempt identity;
- pre-upload file identity check;
- provider-neutral GCS/S3 release plan.

Verified weaknesses corrected by the proposed Primitive package:

- the raw URL projection is public to the caller and error surfaces;
- redirect rejection is not explicitly owned by the uploader;
- `io.LimitReader` does not independently prove exact source EOF;
- the uploader has no CRC32C transfer proof;
- it does not capture provider generation/version;
- it returns expected manifest/target facts as the uploaded result without
  provider version evidence.

Only tests call `ExecuteDeploy` or `HTTPArtifactUploader` in the observed tree
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy_test.go:64-272`); no command or other production
caller was found. This is candidate release machinery, not a completed live
deployment proof.

### Peachfuzz

Peachfuzz has no direct `objectstore` import. Its production archive is an
authenticated GCS JSON API client, not a signed-URL transfer consumer. It is
wired through `OpenConfiguredBlobstore`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/configured.go:13-33`) and the daemon archive
lifecycle (`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/archive.go:9-36`).

Its GCS implementation owns:

- injected workload-identity access tokens;
- bucket policy verification;
- create-only media upload using `ifGenerationMatch=0`;
- CRC32C request validation and metadata verification;
- bounded GET;
- page-by-page LIST;
- safe retry of a rewindable create-only source;
- immutable collision verification.

See `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:22-40`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:182-301`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:303-405`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:408-594`.
Blob descriptors calculate BLAKE3, extent, and CRC32C through bounded streaming
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/blob_upload.go:19-134`).

The key immutable-write gem is in
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/publish.go:40-63`: a create-only conflict is not
blindly treated as success; the existing remote bytes are streamed and compared
with the expected descriptor. The retry loop rewinds the source before every
attempt and relies on native create-only semantics
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:432-447`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:496-523`).

Peachfuzz also has real GCS acceptance evidence for policy verification,
write-once pack publication, paged listing, and hydration
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs_live_test.go:20-75`). This is stronger evidence
discipline than the archived objectstore package, but it proves authenticated
GCS JSON API behavior only; it does not prove GCS signed XML URLs or any S3
behavior.

Keep downstream:

- access-token acquisition and Authorization headers;
- bucket name and policy;
- archive namespace, pack/manifest layout, listing, hydration, lifecycle, and
  reclamation;
- bounded retry/reopen policy;
- BLAKE3 archive identity.

Potential future composition: a control plane may issue signed targets and use
Primitive objectstore for finite pack transfers, but Primitive must not absorb
Peachfuzz's bucket API to make that possible.

## Strong mechanics and proof

The archived architecture and consumer interviews above contain the located
mechanical and hostile proof for this package.

## Defects and blockers

### Verified archive defects and proof gaps

### Blockers before code admission

1. **No live-provider proof.** The archive explicitly marks objectstore as
   `live-provider proof pending`
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/specs/README.md:18`), and its pending ledger still requires
   disposable live GCS and S3 execution
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_pending.md:336-340`). The specification requires thirteen
   live cases per provider (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:799-821`) and makes a
   green live matrix part of completion
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:842-861`). No live objectstore test exists.

2. **Stringly public signed-header DTO.** `Header` exports `Name string` and
   `Value string` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/values.go:91-102`), and targets export
   `[]Header` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/values.go:142-176`). Validation is useful,
   but the compiler cannot distinguish a checksum header, create-only
   precondition, product metadata header, or forbidden framing header. Rebuild
   this as an immutable validated signed-header value/set with private
   representation and typed constructors/parsers.

3. **Hidden default projections.** `providerVersionHeader` maps every provider
   other than GCS to the S3 header, and `directionLabel` maps every direction
   other than upload to download
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:396-408`). Current callers validate first,
   so the invalid arms are intended to be unreachable, but these helpers
   encode future enum values as valid existing cases. Rebuild exhaustive,
   fail-closed projections returning an error.

4. **Structural test contradicts the package specification.** The spec says
   structural ratchets must not snapshot an exact declaration list
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/SPEC.md:782-797`). The implementation has an exact
   production-struct inventory that fails whenever any struct is added or
   removed (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/architecture_test.go:32-73`). The same file
   already contains a useful role/shape/import boundary ratchet at lines
   76-129; retain that proof.

   The testing protocol still requires every production struct to be
   classified by intentional data-flow role. Reconcile both requirements:
   preserve per-struct role classification, but remove the frozen
   `slices.Equal` exact-name snapshot. Do not drop inventory coverage and do
   not make implementation names the protocol.

5. **No downstream cutover proof.** All four committed consumer pins predate
   the package. The only observed pin containing objectstore is Kernel's dirty
   working-tree pin, and Kernel has no objectstore call site. Archived
   `submission` and `upgrade` tests are valuable local composition evidence,
   but they are not consumer integration evidence.

### Required design corrections or new proof

6. **Bounded error walk can silently lose identity.** The archive inspects at
   most 64 error nodes (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/errors.go:25-74`;
   `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/objectstore_constants.go:6`). Existing hostile tests prove
   spoof/panic resistance but do not prove exact-limit and over-limit identity
   behavior (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/disclosure_hardening_test.go:149-184`). A
   bounded hostile graph must fail closed with an explicit typed truncation
   identity or use a projection design whose required identities are never
   beyond an untrusted graph.

7. **Commit uncertainty is inferred downstream.** Submission reconstructs
   `indeterminate` from a list of objectstore and Exchange errors
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:submission/transfer.go:207-267`). That is duplicated protocol
   knowledge. Objectstore owns whether an upload was not attempted, rejected
   before commitment, confirmed, or left indeterminate after bytes crossed the
   boundary. Primitive 2026 should expose that as a closed result enum,
   validated with the result's direction/status/progress facts.

8. **Archive status is not release status.** The deterministic package test is
   green, and the completed ledger records local hostile and cross-build claims
   (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1902-1927`). This interview did not reproduce
   the complete race, fuzz, cross-build, lint, security, vulnerability, or
   witness-lint gate, and the missing live matrix independently prevents
   admission of the archived code unchanged.

## Primitive 2026 ownership and DAG

### `core`

Core owns only contracts shared across Primitive packages:

- stable objectstore error identities;
- `ByteLength`/`ByteCount`, `SHA256Hex`, `CRC32C`,
  `ProviderVersionIdentity`, and `HTTPStatusCode`;
- generic typed HTTP header name/value/set contracts and generic HTTP method,
  status, redirect, and replay values;
- shared JSON and URL ceilings;
- any value or invariant genuinely consumed by more than one Primitive
  package.

Provider-specific facts used solely to implement objectstore remain private to
objectstore. Product header names, object paths, schemas, retry values, and
error identities remain in each consumer's Core or owning product package.

### `exchange`

Exchange owns HTTP request construction, transport, exact response limits,
selected response headers, redirect enforcement, cancellation, attempt
timeouts, and typed transport/status/write failures. Objectstore composes it
and must not copy those mechanics.

### `temporal` and `contextstate`

Temporal owns validated instants and durations. Contextstate owns context
ingress and terminal-state classification. Objectstore performs one expiry
observation immediately before the attempt and does not persist wall-clock
arithmetic.

### `objectstore`

Objectstore owns:

- `Provider`, `WriteMode`, `Direction`, and a closed `Commitment`;
- opaque `SignedURL`;
- immutable `SignedHeader`/`SignedHeaders`;
- `Integrity`;
- upload/download targets and requests;
- explicit execution policy;
- immutable transfer evidence;
- provider lattice, extent proof, digest proof, expiry preflight, provider
  version projection, conflict/absence classification, and secret-safe error
  projection.

Every ingress DTO, package crossing, execution input, and external result calls
its owning `Validate()` method. JSON decoders validate a temporary and preserve
receiver state on failure. Public values are structs/enums, never maps or
string blobs.

### Consumers and higher Primitive packages

- grant issuers own signed-target creation and target-to-authorization binding;
- Release owns upload-attempt and artifact binding;
- consumers own commercial authorization, declaration, receipt, and recovery;
- Upgrade owns Filestore staging and durable activation;
- Peachfuzz owns bucket authentication, archive namespaces, listing, hydration,
  lifecycle, and replay/reconciliation policy;
- Kernel owns credentials/configuration and ceremony reporting.

### Proposed 2026 capability contract

The corrected candidate proposes one narrow package with:

- one immutable client built from an Exchange client;
- one opaque signed URL parser with no public string projection;
- closed GCS, upload/download, and commitment enums, with create-only upload;
- immutable typed signed-header values and sets;
- exact `Integrity{Length, SHA256, CRC32C}`;
- target-owned expiry and provider/write contracts;
- `Upload` and `Download` over `io.Reader`/`io.Writer`;
- one-attempt streaming and O(1) object-size memory;
- provider version evidence where supplied;
- honest partial download progress;
- typed result commitment:
  `NotAttempted`, `Rejected`, `Confirmed`, or `Indeterminate`;
- core-owned stable errors checked only with `errors.Is`/`errors.As`.

Do not add list, delete, bucket clients, cloud SDKs, signing, retries, resumable
sessions, multipart transfers, local files, compatibility wrappers, or
consumer-specific binding fields.

## Decision rationale and conditions

### Required admission proof

Before implementation is presented:

1. write and review the Primitive 2026 package specification;
2. prove enum exhaustion and fail-closed future values;
3. prove immutable header construction, JSON closure, duplicate rejection, and
   exact wire-size bounds;
4. prove signed URL redaction across formatting, error, JSON, and transport
   failures;
5. prove exact zero/short/exact/long upload behavior without releasing an
   unproved final chunk;
6. prove exact download, partial writer progress, overflow, range/content
   encoding rejection, and destination-error identity;
7. prove the GCS create-only and checksum lattice;
8. prove the commitment enum against preflight, early rejection, confirmed
   success, conflict, timeout, cancellation, and connection-loss boundaries;
9. prove error-graph bounds without silent stable-identity loss;
10. prove allocation independence from object size and `gocyclo <= 10`;
11. run ordinary, race/shuffle, fuzz, vet, static, security, vulnerability,
    nilness, error, constant, coupling, and witness-lint gates;
12. cross-build Linux and Windows;
13. run a disposable live signed-URL matrix for GCS, including
    zero/non-empty create-only upload, duplicate conflict, replace, exact
    download, checksum rejection, omitted signed header, expiry, wrong method,
    redirect, cancellation, and version capture;
14. compose proof with Upgrade and the named real consumers before any
    external consumer migration;
15. migrate each consumer only after the pushed `/v2026` dependency closure is
    complete.

### Recon implications

The earlier free-text verdict was admission with corrections.

The package boundary is real, reusable, and materially safer than the consumer
implementations. The archived streaming, dual-integrity, signed-capability,
provider-lattice, secret-withholding, and Filestore-composition mechanics are
the right foundation.

Do not copy the archived package wholesale. Reconstruct it from the reviewed
specification, replacing the raw header DTO, hidden enum defaults, declaration
snapshot ratchet, bounded-error ambiguity, and inferred commitment protocol.
Admission remains blocked until the full live GCS signed-provider matrix is
green. S3 and a Primitive Submission package are outside the corrected
`v2026.0.0` candidate. No commit or consumer migration is authorized by this
interview.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
