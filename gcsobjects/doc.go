// Package gcsobjects performs the authenticated Google Cloud Storage object
// lifecycle and short-lived upload-capability issuance over official provider
// SDKs.
//
// Gcsobjects owns authenticated SDK client construction, create-only bucket
// provisioning, typed logical namespace composition, create-only object writes,
// digest-bound bounded reads, and generation-matched permanent deletion. It
// reuses Integrity and ExactReader from objectstore, so exact
// extent and streaming SHA-256 and CRC32C proof are shared with the
// issued-capability transfers rather than reimplemented. The official Cloud
// Storage SDK is confined here: no consumer imports it, and the base
// objectstore package stays SDK-free.
//
// IssueGCSUploadCapability uses the official IAM Credentials client to sign
// Objectstore's exact typed create-only and CRC32C request fields. It returns
// only an opaque Objectstore capability projection; product grants, users,
// media roles, and workflows have no representation here.
//
// UploadMedia and UploadFile are two compiler-selected entry points, not one
// call behind a mode flag. UploadMedia writes a served asset a browser or CDN
// fetches: it carries the content type that makes the bytes render and an
// optional cache directive. UploadFile writes a stored blob the system
// retrieves and verifies: application/octet-stream, no cache directive. Both
// are create-only under a generation-zero precondition, so an existing object
// is a conflict rather than an overwrite, and both bind the stream to a
// declared SHA-256 and CRC32C over an exact extent.
//
// ReadGCSObject binds provider extent and CRC32C metadata to the caller's
// SHA-256 and byte ceiling, then verifies the complete stream. DeleteGCSObject
// and DeleteGCSObjects delete exact observed generations, prove the name or
// confined prefix absent afterward, and refuse buckets whose soft-delete policy
// would make "deleted" mean retained.
//
// Products select Application Default Credentials or an explicit service
// account file through a closed typed configuration; the SDK type and
// credential-discovery mechanics never escape the package.
package gcsobjects
