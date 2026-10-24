package core

// S3 query signing vocabulary and expiry bounds belong to the provider,
// rather than to products or the generic Signature V4 signer.
// Source: https://docs.aws.amazon.com/AmazonS3/latest/developerguide/sigv4-query-string-auth.html
const (
	S3SigningService              = "s3"
	S3UnsignedPayload             = "UNSIGNED-PAYLOAD"
	S3QueryExpires                = "X-Amz-Expires"
	S3EscapedPathSeparator        = "%2F"
	S3PresignMinimumSeconds int64 = 1
	S3PresignMaximumSeconds int64 = 604800
)
