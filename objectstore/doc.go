// Package objectstore performs exact bounded whole-object transfers through
// already-issued HTTPS capabilities.
//
// Objectstore owns provider request shape, exact extent, streaming SHA-256 and
// CRC32C proof, create-only intent, and commitment classification. It does not
// create buckets, mint credentials, signed URLs, Cloudflare draft records,
// retries, resumable sessions, lifecycle policy, or multi-provider workflows.
// It can project an already-issued UploadTarget into the exact capability
// document its receiver consumes. For raw-object providers it can also project
// that same bearer plus one exact Integrity and content type into the complete
// browser-spendable HTTP request, including provider-owned checksum and
// create-only fields. Signing and authorization remain the caller's
// responsibility.
//
// UploadS3, UploadGCS, and UploadCloudflareImages are separate compiler-selected
// operations. Replication is ordinary caller composition: reopen the source and
// call each required operation in sequence.
//
// The authenticated Google Cloud Storage lifecycle over the official provider
// SDK is a separate package, gcsobjects, which reuses Integrity and ExactReader
// from here. Objectstore itself imports no cloud SDK.
package objectstore
