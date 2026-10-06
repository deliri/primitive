# Cloudflare capability contract

Primitive owns these provider mechanics. Every network operation uses Exchange;
Temporal owns instants and durations. The Stream receiver borrows caller-owned
seekable scratch obtained through Filestore. It authenticates the complete body
before publishing any byte to the destination. Product code retains decisions
about permissions, retries, idempotency, media readiness and lifetime accounting.

## Paired doors

| Purpose | Server capability | Client capability |
| --- | --- | --- |
| Images direct creator upload | `ImagesServer.CreateDirectUpload` | `ImagesClient.Upload` |
| Stream basic direct creator upload | `StreamServer.CreateDirectUpload` | `StreamClient.UploadBasic` |
| R2 exact object operation | `R2Server.Presign` | `R2Client.Read`, `Put`, `Delete` |
| R2 exact metadata observation | `R2Server.Presign` with HEAD | `R2Client.Head` |
| Images lifecycle observation and retirement | `ImagesServer.Details`, `Delete` | Authenticated metadata only; no media body |
| Images custom delivery address | `ImageDetails.PublicAddress` | `ImageDeliveryRequest.Address`; exact origin/account/image/variant binding |
| Zone cache invalidation | `CacheServer.PurgeFile` | Exactly one URL; acceptance receipt, not absence proof |
| Cache variant invalidation | `CacheServer.PurgePrefix` | One typed host/path prefix; includes header/query variants; caller owns prefix scope |
| R2 multipart upload | `R2Server.PresignMultipart` | `R2Client.CreateMultipart`, `UploadPart`, `CompleteMultipart`, `AbortMultipart` |

Server objects clone credential custody. Close them when their owner exits.
Upload and object grants are redacted when formatted and bind the provider
scheme and authority. R2 grants additionally bind account, jurisdiction, method,
object path, signing scope and signed content type. `ParseR2Grant` proves that
structural agreement; Cloudflare independently verifies the signature. It is
not a local authentication receipt.

R2 write conditions can sign Content-MD5 and If-None-Match. The latter prevents
overwriting an existing object; it does not make a presigned URL single use or
revoke it after deletion. Applications must retain retirement state and reject
late attachment. R2 metadata exposes an opaque ETag, content type and byte
length; it never reinterprets a multipart ETag as a digest.

`R2CacheControl` signs an exact whole-second `max-age` on PUT or multipart
creation. Its zero value omits optional metadata; constructing it with a zero
duration requests immediate staleness explicitly. Other operations reject this
metadata. The SDK bounds its representation to 31 nonnegative integer bits;
Kernel and distros choose the actual lifetime. Grant header projection, client
execution and the independent signature tests use the same typed intent.

`CacheServer` clones a token and binds a zone. `PurgeFile` submits one complete
URL under the caller's response budget. Provider acceptance cannot establish
public absence. Product policy must observe delivery and decide completion;
custom cache-key header dimensions require their own explicit contract.
`PurgePrefix` uses Cloudflare's native prefix API to invalidate all variants
without enumerating headers or building a local cache model. Its request and
receipt carry `CachePurgePrefix`; query-bearing input is refused instead of
silently dropping the query. The native 31-separator ceiling lives in core.
Prefix scope can include longer paths sharing the prefix. A caller requiring
one object's variants must select that object's complete path and own the
resource naming policy. Neither purge operation proves public absence.

Images details bind the returned ID and delivery paths to the requested ID.
Draft creation is not upload completion. Image metadata does not verify an
application's declared SHA-256 or BLAKE3, nor expose an original byte extent.
Deletion requires the provider's explicit success envelope with no errors and
a present, valid opaque result. Missing or contradictory acceptance is refused.

Images delivery uses separate nominal types for the public account hash and a
predefined variant. A custom HTTPS origin belongs to the caller. PublicAddress
requires observed public, non-draft metadata containing that exact account,
image and variant before projecting the custom URL. URL projection proves no
DNS configuration or delivered bytes. The account hash has no invented length
quota; the variant's 99-character restriction is the native provider contract.

Multipart completion receives `R2CompletedParts`, a synchronous visitor over
the actual UploadPart receipts. It writes one typed part through encoding/xml
and Go's pipe directly to Exchange. It neither collects a manifest nor scans
it to calculate Content-Length. The previous ordinal is the only sequence
coordinate retained; actual provider ordinal rules remain validated. The
source owns its storage, honors cancellation and stops on a yielded error.
An owned Temporal deadline covers both source and HTTP work. Early peer refusal
closes the pipe, cancels the source and joins the producer before returning.

Control responses are decoded directly from a pipe with encoding/xml. The
closed flat schema rejects duplicate, unknown, nested and crossed fields.
There is no SDK response-document, ETag or upload-ID extent quota. Memory is
proportional to the current XML token and typed scalar fields, independent of
the number of part receipts. The SDK does not retain or reconstruct an upload
session, and it does not substitute ListParts for the caller's actual receipts.

R2 reads issue one GET or HEAD. There is no preliminary list or metadata query.
Signing is local and uses Exchange's validated SigV4 operation with the official
Go signer. Cloudflare owns its own R2 region, service, host and query constants.
It imports no S3, Plunk, Twilio or GCS SDK constants.

Images notification authentication uses the documented notification secret
header. Stream webhooks use the distinct timestamp/raw-body HMAC protocol.
A verified body is still opaque provider data, not an application transition.
Maximum age, future tolerance, body budget and destination lifetime are caller
policy. Old scratch tails are excluded by replaying only authenticated bytes.

## Documentation and scope

Native provider constraints cite their official source beside each constant in
`core/cloudflare_contracts.go`. Product acceptance budgets belong to callers.

- [Images direct uploads](https://developers.cloudflare.com/images/storage/upload-images/direct-creator-upload/)
  and [custom paths](https://developers.cloudflare.com/images/storage/upload-images/upload-custom-path/).
- [Stream direct uploads](https://developers.cloudflare.com/stream/uploading-videos/direct-creator-uploads/)
  and [webhook authentication](https://developers.cloudflare.com/stream/manage-video-library/using-webhooks/).
- [R2 presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/),
  [jurisdictions](https://developers.cloudflare.com/r2/api/tokens/) and
  [limits](https://developers.cloudflare.com/r2/platform/limits/).

These doors cover Images direct uploads, Stream basic uploads, their incoming
webhook authentication, R2 single-object GET/HEAD/PUT/DELETE and multipart,
and exact-URL cache purging. They do not claim the entire Cloudflare API.
Stream tus/resumable transfer, queue consumption and the remaining administration endpoints require
their own named capabilities and conformance proof. Basic upload ceilings are
refused explicitly, with no hidden alternate transfer path.

## Behavioral proof

The default provider integration fixtures use real local TLS servers and Exchange.
They inspect exact methods, authority, headers, multipart/JSON bodies and byte
extents. They do not contact a live Cloudflare account. Direct fuzz transport
fixtures are identified in their tests and complement those integration tests.
The separately tagged multipart live test uses a caller-configured credential
and disposable object, verifies the completed download hash and abort refusal,
then cleans up. Its execution is accounted for separately from local fixtures.

The API-envelope test exhausts the 12 combinations of success presence/value,
provider errors and empty/nonempty result; contradictory responses issue no
capability. Independent R2 HMAC verification proves exact method, path, query,
authority and signing scope. Mutation evidence removes provider-error rejection,
changes the R2 signing region, and introduces a direct HTTP import; each fails.

Regression evidence also captures wrapped EOF, UUID custom-ID refusal, opaque
webhook-key admission, scratch budget enforcement, signature member ordering,
and the transport/upload-completion data race. Atomic publication now protects
the final completion observation. The working-memory test streams a generated
33 MiB-plus-one-byte source through fixed windows; it makes no claim to have
transferred a terabyte or measured allocation distributions.

Furnace fuzzing found that an unescaped path byte changed an upload grant's
internal representation after serialization. Upload grants now store Core's
canonical endpoint projection. The minimized input is retained in the fuzz
corpus; regression cases prove nominal closure while preserving escaped slash,
query order, percent spelling and explicit empty-query meaning.

Each external representation door is bound to its semantic fuzz target in
`ingress_inventory_test.go`; Exchange's signer has its own ingress inventory
entry and independent oracle. Production structs have compiler-visible roles.
The architecture guard rejects direct HTTP, filesystem, clock, entropy and
foreign provider SDK imports in this package. No waiver was added.

Raw execution facts and the complete required gate result are retained separately.
Local proof is not the user's independent acceptance, and a passing focused run
does not close untested packages or unavailable live-provider checks.

The 2026-10-05 lifecycle increment has local TLS integration tests, semantic
fuzz targets for details/deletion/HEAD/write conditions, race and shuffled-repeat
proof, and a deliberate draft-readiness mutation that fails its regression.
A separately scoped live browser smoke uploaded a 3,842,345-byte image to
Images, a 5,534,429-byte video and a 142-byte PDF to R2. Metadata and public
delivery were observed; video/PDF downloads matched their source SHA-256;
same-grant overwrites returned 412; each delete was accepted. That probe did
not execute a product's workout form, authentication, CSRF or persistence flow.
The broader application smoke and repository-wide gates remain separate work.
