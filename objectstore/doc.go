// Package objectstore performs one exact, bounded transfer through an
// already-issued Amazon S3, Google Cloud Storage, or Cloudflare Images HTTPS
// capability.
//
// Objectstore owns provider request shape, exact extent, streaming SHA-256 and
// CRC32C proof, create-only intent, and commitment classification. It does not
// create buckets, credentials, signed URLs, Cloudflare draft records, retries,
// resumable sessions, or multi-provider workflows. It can project an
// already-issued UploadTarget into the exact capability document its receiver
// consumes; signing and authorization remain the caller's responsibility.
//
// UploadS3, UploadGCS, and UploadCloudflareImages are separate compiler-selected
// operations. Replication is ordinary caller composition: reopen the source and
// call each required operation in sequence.
package objectstore
