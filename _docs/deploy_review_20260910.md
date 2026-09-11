# Deploy review — v2026.1.62

Deploy now refuses Release extents beyond Objectstore's GCS upload maximum
while constructing or validating an upload item. Previously the larger uint64
Release extent domain could enter a prepared plan and fail only when the
individual transfer began. The gate uses the existing provider-owned limit;
it introduces no arbitrary stream quota and reads no source bytes.

Receipts.Validate now rejects duplicate capability commitments across role
slots. A confirmed provider transfer cannot certify two different objects in
the fixed publication prefix. This closes the same uniqueness invariant at the
receipt boundary that already applied to the prepared plan.

The baseline's failing zero-policy test was obsolete: Objectstore intentionally
allows zero timeouts to inherit the caller context. It now tests a genuinely
invalid timeout ordering; a positive public execution test pins zero-policy
inheritance. No compatibility branch or timeout policy was added to production.

## Primitive boundary

This is a narrow GCS upload socket: authenticated Release facts, exact
create-only capabilities and caller-owned readers enter; confirmed transfers
and typed failure facts leave. The caller decides what publication means.
There is no product state machine, general workflow engine, retry loop,
activation/Latest update, tenant state, hidden scheduler or whole-stream model.

The only production edits are two validation gates in release.go. Storage
remains fixed arrays sized by Release's closed publication domain. Upload bytes
stream through Objectstore and Exchange; Deploy never accumulates them. The
bounded receipt scan is independent of object extent. Provider-specific limits
remain owned by Objectstore.Spec.

## Boundary inventory

| Surface | Evidence |
|---|---|
| UploadItemRequest.Validate / NewUploadItem / UploadItem.Validate | Typed-nil/absent source, valid empty reader, capability/commitment substitution, missing integrity and unknown roles, provider limit below/exact/above/extreme, zero source reads during admission; typed admission fuzz |
| ReleasePlanRequest.Validate / PrepareRelease / ReleasePlan.Validate | Authenticated manifest and canonical document integrity, role order, duplicate capabilities, missing items, timeout ordering and inherited zero timeout; invalid plan/client/context perform no requests |
| ReleaseGCS → Objectstore upload → provider response → Receipt | Real TLS client/server tests independently hash and count received bytes, record destination and create-only precondition, compare receipt role/grant/version/length/SHA256/CRC32C; confirmed, rejected and indeterminate outcomes |
| Receipt.Validate / Role / Transfer / Commitment | Actual capability-backed provider transfer, foreign commitment and raw-provider transfer refusal, attempted/absent transfer refusal, exact confirmed facts |
| Receipts.Validate / Count / At | Exact prefix, empty prefix, count above storage and maximum uint8, missing declared receipt, hidden padding, wrong role, duplicate grant identity, absent/negative/out-of-prefix lookup |
| UploadError.Error / Unwrap / typed fields | Deploy and underlying typed error identities, exact failed role and transfer commitment, nil/cause-less behavior; missing confirmation cannot become successful publication |
| ProgressObserver | Exact monotonically increasing upload progress, optional observer absence, callback refusal preserved with no extra confirmed receipt and no retry |
| Struct/data-flow inventory | Existing seven production structures stay classified; checked AST type assertion; no new production struct |

The provider handoff has a small mechanical outcome space. One complete upload
proves every confirmed position. Each of the eight possible stopping positions
is tested with explicit provider conflict and with absent generation proof.
Existing transport-loss tests cover every position separately. These cases
assert exclusive rejected/indeterminate/confirmed commitment facts and exact
retained-prefix identities; repeating the same complete success for an inert
selected index was deliberately removed. No invented product-classification
matrix or padded 50-case claim is made.

The exact-provider tests use a real local TLS server with redirected dialing;
signed destinations remain the production GCS host. Other failure injectors
and fuzz targets use local RoundTrippers through the actual Objectstore and
Exchange upload path. They are not live GCS certification. Request bodies are
closed by the injectors. Fixture construction now checks errors and builds
capabilities through the typed production projection and marshaler.

No durable evidence-file, manifest-bundle, ledger, reporter or CLI layer exists
inside Deploy. Release owns the authenticated manifest, Objectstore owns stream
and provider-response interpretation, and the caller owns durable evidence
storage. The local TLS witness compares the manifest-bound source integrity,
received bytes and returned transfer without claiming a provider disk walk.

## Fuzz inventory

- FuzzUploadItemTypedAdmission: arbitrary extent and role, foreign commitment,
  exact accept/refuse domain and no source reads.
- FuzzReleaseGCSSourceAndPrefix: mutated executable/manifest/metadata source at
  any fixed role; only exact signed bytes may extend the confirmed prefix;
  typed source error, failed role, one attempt and exact earlier commitments.
- FuzzReleaseGCSProviderGeneration: canonical positive signed-size-domain
  generations, missing/duplicated/malformed/overflow values, through the real
  Objectstore producer; no receipt or confirmed evidence without generation
  proof. Accepted versions retain the exact provider value.

Deploy has no JSON decoder of its own. Objectstore and Release retain ownership
of their raw capability/manifest decoders and broader provider grammar fuzzing.
The new fuzz targets exercise Deploy's public typed and streaming doors and the
producer-to-receipt handoff, using bounded secondary oracle work.

## Execution evidence

Base revision: 568009f0ca4891a6fc5c3a3719a99846c3118c80.
Evidence: /work/engineering-evidence/primitive/deploy-upgrade-20260910.

baseline retains the obsolete timeout assertion failure. extent-red proves
original admission of over-limit and maximum uint64 extents. receipt-prefix-red
proves the original duplicate-grant acceptance. provider-refusal-mutation
changes an upload error return to nil and fails all sixteen provider-refusal
rows; the mutation is discarded after capture. admission-fuzz-mutation bypasses
the provider extent check and fails the typed admission oracle.
handoff-fuzz-mutation discards the upload error and fails the source and
generation fuzz seeds. All mutations are restored. All attempts remain retained.
The source fuzz run exposed an oracle error: equal-length foreign bytes have
ErrObjectStoreIntegrity, while extent mismatch has ErrObjectStoreSource. The
oracle now asserts the distinct identities. Go's minimized corpus input is
promoted unchanged. Its automatic creation changed the source tree during that
failed run; the original false stability fact is retained. A separate post-run
observation verified that all pre-existing source hashes still matched and only
the named corpus file was added. This failed attempt remains non-accepting.

The final committed phase runs cache-disabled race tests for Deploy and Compass,
witness-lint, strict errcheck, staticcheck, vet and full-module build. The
NewUploadItem benchmark runs once per before/after phase for 30 seconds on the
same machine/toolchain/CPU setting, retaining profiles and binaries. Each fuzz
target then runs once for ten seconds with one worker. A single benchmark
sample per phase is not a trend or independent performance acceptance.

The external record retains exact revisions, clean/dirty source facts,
commands, tool identities, stable source digests, output hashes/byte counts,
emitted counts and exit statuses. Historical not-run/unavailable denominators
remain explicitly unknown. Local manifest/disk verification is separate from
independent acceptance. Full-module build is not full-module test acceptance;
previously reported unrelated Core/AWSidentity failures remain separate.

Wiring is next. Independent acceptance and consumer migration remain separate.
The release tag is derived from compass/config.json.
