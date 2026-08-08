// Package objectstore performs exact bounded object transfers through issued
// HTTPS capabilities and authenticated Google Cloud Storage operations through
// the official provider SDK.
//
// Objectstore owns provider request shape, exact extent, streaming SHA-256 and
// CRC32C proof, create-only intent, and commitment classification. It does not
// create buckets, mint credentials, signed URLs, Cloudflare draft records,
// retries, resumable sessions, lifecycle policy, or multi-provider workflows.
// It can project an
// already-issued UploadTarget into the exact capability document its receiver
// consumes; signing and authorization remain the caller's responsibility.
//
// UploadS3, UploadGCS, and UploadCloudflareImages are separate compiler-selected
// operations. Replication is ordinary caller composition: reopen the source and
// call each required operation in sequence.
//
// NewGCSClient keeps authenticated SDK construction inside Primitive.
// CreateGCSObject is create-only and exact. ReadGCSObject verifies the complete
// stream. DeleteGCSObjects requires a confined bounded prefix, deletes exact
// listed generations, proves the prefix absent, and refuses buckets whose
// soft-delete policy would make "deleted" mean retained.
package objectstore
