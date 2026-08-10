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
- two create-only object writes, served-media and stored-file;
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

`ObjectAddress` derives it from a validated bucket and object name, and
`GCSObjectMetadata.Address` derives it from an accepted result. Both are pure
value derivations. Neither contacts the provider, neither proves the object
exists, and neither confers access: whether a reader may fetch that address is
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
SHA-256 and CRC32C over an exact extent.

## What it deliberately does not do

- create buckets, mint or persist credentials, or create signed URLs;
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

The effect leaf is the official Cloud Storage SDK,
`cloud.google.com/go/storage`, reached through one authenticated client.
Product code selects Application Default Credentials or an explicit
service-account file through a closed typed configuration; the SDK type and
credential-discovery mechanics never escape the package.
