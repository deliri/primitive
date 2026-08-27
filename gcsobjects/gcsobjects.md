# gcsobjects

## What this package is for

gcsobjects owns the authenticated Google Cloud Storage object lifecycle: the
operations a product performs against a bucket it holds credentials for, as
opposed to the issued-capability transfers `objectstore` performs against a
signed URL it was handed. It exists so the official
`cloud.google.com/go/storage` SDK is confined to one package instead of being
dragged into every `objectstore` consumer, including local tools that only use
the signed-capability surface.

It owns exactly:

- authenticated SDK client construction from a closed typed configuration;
- create-only bucket provisioning in one typed provider location with a closed
  flat or hierarchical namespace choice;
- logical directory and object-name composition from validated segments,
  without fake placeholder objects or caller-owned slash conventions;
- two create-only object writes, served-media and stored-file;
- exact-generation upload observation that binds authenticated client evidence
  to the official provider metadata without downloading object bytes;
- official IAM Credentials signing of short-lived V4 create-only upload and
  whole-object retrieval capabilities from Objectstore's exact typed fields;
- one digest-bound bounded read;
- the canonical public address of one object; and
- generation-matched permanent deletion of one exact object or one confined
  prefix, each proved absent afterward.

## The address is a name, not a grant

`UploadMedia` writes an object a browser or CDN will fetch, so the package that
publishes it owns the address it was published at. Without that, every consumer
rebuilds the provider's URL from a copied host and two slashes, which is the
projection rule broken by omission: the owner declines the address, so the
address grows copies outside the owner.

`ObjectAddress` derives it from a validated bucket and object name,
`ParseObjectAddress` reverses only that exact unsigned canonical form into a
validated bucket and object identity, and
`GCSObjectMetadata.Address` derives it from an accepted result. All three are
pure value derivations. None contacts the provider, proves the object exists,
or confers access: whether a reader may fetch that address is
the bucket's policy. A stored file in a private bucket therefore has a perfectly
correct address that answers 403, which is the truth rather than a trap. A
consumer serving through its own CDN composes its own origin and does not use
this.

## Served versus stored

The write surface is two compiler-selected entry points, never one call behind a
mode flag. The caller makes the decision by naming the function:

- **UploadMedia** writes an object a browser or CDN will fetch. The caller
  chooses the content type that makes the bytes render; the cache directive is
  optional, and the edge applies its own default when it is absent.
- **UploadFile** writes a stored blob the system retrieves and verifies. It is
  written application/octet-stream with no cache directive, because nothing
  serves it to a browser.

Both are create-only under a generation-zero precondition, so an existing object
is a conflict rather than an overwrite, and both bind the stream to a declared
SHA-256 and CRC32C over an exact extent. `ObserveGCSUpload` then reads only the
exact generation named by authenticated transfer evidence and releases sealed
proof only when bucket, name, generation, extent, and CRC32C agree. Its
`ProviderObservation` projection then hands Objectstore's provider-neutral
sealed facts to an authority without teaching that authority about GCS.

## Buckets and logical directories

`CreateBucket` accepts one `GCSBucketCreateRequest` containing a nominal Google
project ID, validated bucket name, provider location, and closed namespace
choice. It calls the official SDK once and returns sealed provisioning evidence
only after the provider accepts creation. Existing buckets are conflicts; this
surface never updates bucket policy in place.

Flat object storage has no directory resource to create. `ComposeGCSRootPrefix`,
`ComposeGCSChildPrefix`, and `ComposeGCSObjectName` therefore build validated
logical prefixes and exact names without uploading zero-byte slash objects that
pretend to be directories. A hierarchical bucket enables the provider's real
hierarchical namespace at bucket creation; managed-folder IAM administration
remains outside this object lifecycle.

## What it deliberately does not do

- mutate existing buckets or mint or persist credentials;
- create placeholder objects to imitate directories;
- administer managed-folder IAM policy;
- overwrite, copy, compose, or mutate arbitrary object metadata;
- mutate bucket lifecycle, retention, or namespace policy;
- implement resumable or multipart upload protocols;
- reimplement exact-extent streaming or integrity proof: `Integrity` and
  `ExactReader` belong to `objectstore` and are reused; or
- own product retention or naming policy, which stays downstream.

## When it refuses

Deletion is deliberately narrower than general object administration. It
requires an exact name or a nonempty slash-terminated prefix, bounds the prefix
object count, pins every delete to the observed generation, observes the name or
prefix again to prove absence, and refuses a bucket with soft-delete retention
enabled, because there success would not mean permanent deletion.

## Where it meets the real world

The effect leaves are the official Cloud Storage SDK,
`cloud.google.com/go/storage`, and the official IAM Credentials API client.
The latter signs exactly one Objectstore-owned V4 upload or retrieval request
with an explicit service-account principal and request context; it does not
create or store signing keys.
Product code selects Application Default Credentials or an explicit
service-account file through a closed typed configuration; the SDK type and
credential-discovery mechanics never escape the package.
