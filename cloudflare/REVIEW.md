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

Server objects clone credential custody. Close them when their owner exits.
Upload and object grants are redacted when formatted and bind the provider
scheme and authority. R2 grants additionally bind account, jurisdiction, method,
object path, signing scope and signed content type. `ParseR2Grant` proves that
structural agreement; Cloudflare independently verifies the signature. It is
not a local authentication receipt.

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

Provider limits cite their official source beside each constant in
`core/cloudflare_contracts.go`. Explicit SDK custody budgets are labelled as
such; they are not presented as Cloudflare limits.

- [Images direct uploads](https://developers.cloudflare.com/images/storage/upload-images/direct-creator-upload/)
  and [custom paths](https://developers.cloudflare.com/images/storage/upload-images/upload-custom-path/).
- [Stream direct uploads](https://developers.cloudflare.com/stream/uploading-videos/direct-creator-uploads/)
  and [webhook authentication](https://developers.cloudflare.com/stream/manage-video-library/using-webhooks/).
- [R2 presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/),
  [jurisdictions](https://developers.cloudflare.com/r2/api/tokens/) and
  [limits](https://developers.cloudflare.com/r2/platform/limits/).

These doors cover Images direct uploads, Stream basic uploads, their incoming
webhook authentication, and R2 single-object GET/HEAD/PUT/DELETE. They do not
claim the entire Cloudflare API. Stream tus/resumable transfer, R2 multipart and
queue consumption, and the remaining provider administration endpoints require
their own named capabilities and conformance proof. Basic upload ceilings are
refused explicitly, with no hidden alternate transfer path.

## Behavioral proof

The provider integration fixtures use real local TLS servers and Exchange.
They inspect exact methods, authority, headers, multipart/JSON bodies and byte
extents. They do not contact a live Cloudflare account. Direct fuzz transport
fixtures are identified in their tests and complement those integration tests.

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

Each external representation door is bound to its semantic fuzz target in
`ingress_inventory_test.go`; Exchange's signer has its own ingress inventory
entry and independent oracle. Production structs have compiler-visible roles.
The architecture guard rejects direct HTTP, filesystem, clock, entropy and
foreign provider SDK imports in this package. No waiver was added.

Raw execution facts and the complete required gate result are retained separately.
Local proof is not the user's independent acceptance, and a passing focused run
does not close untested packages or unavailable live-provider checks.
