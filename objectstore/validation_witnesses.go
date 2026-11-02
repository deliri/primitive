package objectstore

import "github.com/deliri/primitive/v2026/core"

// Every JSON marshaler here also validates; the compiler holds the pair.
var (
	_ core.ValidatedJSONMarshaler = S3Bucket{}
	_ core.ValidatedJSONMarshaler = S3Key{}
	_ core.ValidatedJSONMarshaler = S3Region{}
	_ core.ValidatedJSONMarshaler = s3CredentialValue{}
)
