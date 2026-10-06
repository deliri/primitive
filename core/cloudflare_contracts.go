package core

// Cloudflare protocol facts have their own owner even where another provider
// currently uses the same spelling or quantity.
const (
	// CloudflareAPIHost is the bearer-authenticated API authority.
	// https://developers.cloudflare.com/fundamentals/api/how-to/make-api-calls/
	CloudflareAPIHost         = "api.cloudflare.com"
	CloudflareAPIAccountsPath = "/client/v4/accounts/"
	CloudflareAPIZonesPath    = "/client/v4/zones/"
	CloudflareCachePurgePath  = "/purge_cache"
	// Optional operation ID returned by the cache purge API.
	// https://developers.cloudflare.com/api/resources/cache/methods/purge/
	CloudflareCachePurgeIDMaximumCharacters = 32
	// Native prefix-purge path depth. Provider contract, not a product budget.
	// https://developers.cloudflare.com/cache/how-to/purge-cache/purge_by_prefix/
	CloudflareCachePrefixMaximumSeparators = 31
	// CloudflareImagesUploadHost receives direct creator multipart uploads.
	// https://developers.cloudflare.com/images/storage/upload-images/direct-creator-upload/
	CloudflareImagesUploadHost = "upload.imagedelivery.net"
	// CloudflareImagesUploadMaximumBytes is the Images 10 MB source-file limit.
	// https://developers.cloudflare.com/images/platform/limits/
	CloudflareImagesUploadMaximumBytes uint64 = 10_000_000
	// CloudflareStreamUploadHost receives direct creator uploads.
	// https://developers.cloudflare.com/stream/uploading-videos/direct-creator-uploads/
	CloudflareStreamUploadHost = "upload.videodelivery.net"
	// CloudflareStreamBasicUploadMaximumBytes applies only to basic POST uploads;
	// larger videos use tus, whose chunk window does not cap the total upload.
	// https://developers.cloudflare.com/stream/uploading-videos/direct-creator-uploads/
	CloudflareStreamBasicUploadMaximumBytes uint64 = 200_000_000
	// CloudflareNotificationAuthenticationHeader authenticates generic notification
	// webhooks, including Images. It is not Stream's signature protocol.
	// https://developers.cloudflare.com/notifications/get-started/configure-webhooks/
	CloudflareNotificationAuthenticationHeader = "cf-webhook-auth"
	// CloudflareStreamSignatureHeader carries time=<seconds>,sig1=<HMAC hex>.
	// https://developers.cloudflare.com/stream/manage-video-library/using-webhooks/
	CloudflareStreamSignatureHeader = "Webhook-Signature"
	// CloudflareR2HostSuffix and CloudflareR2SigningRegion identify R2's own
	// SigV4 authority and region, not an Amazon endpoint or region default.
	// https://developers.cloudflare.com/r2/api/s3/api/
	CloudflareR2HostSuffix    = ".r2.cloudflarestorage.com"
	CloudflareR2SigningRegion = "auto"
	// CloudflareR2SigningService is the service name required by R2's SigV4 API.
	// https://developers.cloudflare.com/r2/api/s3/presigned-urls/
	CloudflareR2SigningService = "s3"
	// CloudflareSecretCustodyMaximumBytes is a Primitive custody budget, NOT a
	// published Cloudflare token-length restriction. Tokens are opaque:
	// https://developers.cloudflare.com/fundamentals/api/get-started/create-token/
	CloudflareSecretCustodyMaximumBytes = 4096
)

// The grammar contains two names, a signed 64-bit decimal timestamp and a
// SHA-256 hex digest. This parser custody bound is not a body-size limit.
// https://developers.cloudflare.com/stream/manage-video-library/using-webhooks/
const CloudflareStreamSignatureMaximumBytes = len("time=") + 19 + len(",sig1=") + 64

// https://developers.cloudflare.com/api/resources/stream/subresources/direct_upload/methods/create/
const (
	CloudflareStreamCreatorMaximumCharacters = 64
	CloudflareStreamDurationMaximumSeconds   = 36000
)

// These are Images-specific creation constraints.
// https://developers.cloudflare.com/api/resources/images/subresources/v2/subresources/direct_uploads/methods/create/
const (
	CloudflareImageCreatorMaximumCharacters = 1024
	CloudflareImageExpiryMinimumSeconds     = 120
	CloudflareImageExpiryMaximumSeconds     = 21600
)

// Cloudflare account and generated Stream identifiers use 32 hexadecimal
// characters. https://developers.cloudflare.com/api/resources/stream/
const CloudflareIdentityCharacters = 32

// https://developers.cloudflare.com/images/storage/upload-images/upload-custom-path/
const CloudflareImageIDMaximumCharacters = 1024

// These constants belong to R2, even when SigV4 has equal wire spellings.
// https://developers.cloudflare.com/r2/api/s3/presigned-urls/
const (
	CloudflareR2QueryExpires          = "X-Amz-Expires"
	CloudflareR2UnsignedPayload       = "UNSIGNED-PAYLOAD"
	CloudflareR2PresignMaximumSeconds = 604800
)

// https://developers.cloudflare.com/r2/platform/limits/
const (
	CloudflareR2ObjectKeyMaximumBytes              = 1024
	CloudflareR2SingleUploadMaximumBytes    uint64 = 5 * 1024 * 1024 * 1024
	CloudflareR2MultipartMaximumParts              = 10000
	CloudflareR2MultipartMinimumPartBytes   uint64 = 5 * 1024 * 1024
	CloudflareR2MultipartMaximumPartBytes   uint64 = (5*1024 - 5) * 1024 * 1024
	CloudflareR2MultipartMaximumObjectBytes uint64 = (5*1024 - 5) * 1024 * 1024 * 1024
)

// R2 multipart protocol names. Transfer extents remain owned by the caller
// and provider; these names do not impose metadata or object-size quotas.
const (
	CloudflareR2QueryUploads               = "uploads"
	CloudflareR2QueryUploadID              = "uploadId"
	CloudflareR2QueryPartNumber            = "partNumber"
	CloudflareR2XMLNamespace               = "http://s3.amazonaws.com/doc/2006-03-01/"
	CloudflareR2XMLMediaType               = "application/xml"
	CloudflareR2MultipartCompletionElement = "CompleteMultipartUpload"
	CloudflareR2MultipartPartElement       = "Part"
)

// https://developers.cloudflare.com/r2/buckets/create-buckets/
const (
	CloudflareR2BucketMinimumBytes = 3
	CloudflareR2BucketMaximumBytes = 63
)

// R2's presigned URL wire protocol. These names are not imported from an S3
// capability: R2's supported domain remains closed independently.
// https://developers.cloudflare.com/r2/api/s3/presigned-urls/
const (
	CloudflareR2QueryAlgorithm       = "X-Amz-Algorithm"
	CloudflareR2QuerySigningIdentity = "X-Amz-Credential"
	CloudflareR2QueryDate            = "X-Amz-Date"
	CloudflareR2QuerySignature       = "X-Amz-Signature"
	CloudflareR2QuerySignedHeaders   = "X-Amz-SignedHeaders"
	CloudflareR2Algorithm            = "AWS4-HMAC-SHA256"
	CloudflareR2CredentialTerminator = "aws4_request"
	CloudflareR2SignatureHexBytes    = 64
)

// CloudflareR2QueryMaximumBytes bounds the representation emitted by this SDK:
// its credential custody, fixed signing scope, and six query fields. Three
// bytes per input byte admit percent-escaping; eleven separators frame six
// assignments. This is an SDK representation budget, not a provider object limit.
// https://developers.cloudflare.com/r2/api/s3/presigned-urls/
const CloudflareR2QueryMaximumBytes = 3*(CloudflareSecretCustodyMaximumBytes+
	len("/20060102/")+len(CloudflareR2SigningRegion)+len("/")+
	len(CloudflareR2SigningService)+len("/")+len(CloudflareR2CredentialTerminator)+
	len(CloudflareR2Algorithm)+len("20060102T150405Z")+len("604800")+
	CloudflareR2SignatureHexBytes+len("cache-control;content-md5;content-type;host;if-none-match")+
	len(CloudflareR2QueryAlgorithm)+len(CloudflareR2QuerySigningIdentity)+
	len(CloudflareR2QueryDate)+len(CloudflareR2QueryExpires)+
	len(CloudflareR2QuerySignature)+len(CloudflareR2QuerySignedHeaders)) + 11

// CloudflareMultipartFilenameMaximumBytes is an SDK framing-custody budget,
// not a provider file-size limit. Images documents a 255-character filename;
// four bytes per rune retain that entire UTF-8 domain in bounded framing.
// https://developers.cloudflare.com/api/resources/images/subresources/v1/methods/get/
const CloudflareMultipartFilenameMaximumBytes = 4 * 255

// CloudflareMultipartFileField identifies the source file in both media APIs.
// https://developers.cloudflare.com/images/storage/upload-images/direct-creator-upload/
// https://developers.cloudflare.com/stream/uploading-videos/direct-creator-uploads/
const CloudflareMultipartFileField = "file"

// Images management and R2 conditions are owned by Cloudflare's documented
// APIs, independently of other providers' coincidentally equal spellings.
const (
	CloudflareImagesDeliveryHost = "imagedelivery.net"
	// Native custom-domain delivery route; caller owns domain/account binding.
	// https://developers.cloudflare.com/images/optimization/hosted-images/serve-from-custom-domains/
	CloudflareImagesCustomDeliveryPath = "/cdn-cgi/imagedelivery/"
	CloudflareImagesV1Path             = "/images/v1/"
	CloudflareR2ContentMD5Header       = "Content-MD5"
	CloudflareR2IfNoneMatchHeader      = "If-None-Match"
	CloudflareR2ETagHeader             = "ETag"
	CloudflareR2CreateOnlyValue        = "*"
	CloudflareR2CacheMaxAgePrefix      = "max-age="
)

// https://developers.cloudflare.com/api/resources/images/subresources/v1/subresources/variants/methods/create/
const CloudflareImageVariantMaximumCharacters = 99

// This SDK admits exact delta-seconds within 31 nonnegative integer bits,
// below the RFC's infinity sentinel. This is a custody bound, not product policy.
// https://www.rfc-editor.org/rfc/rfc9111.html#section-1.2.2
const CloudflareR2CacheMaxAgeMaximumSeconds = 2147483647

// Documented watermark position domain; not another provider's enum.
// https://developers.cloudflare.com/api/resources/stream/methods/get/
const (
	CloudflareWatermarkUpperRight = "upperRight"
	CloudflareWatermarkUpperLeft  = "upperLeft"
	CloudflareWatermarkLowerRight = "lowerRight"
	CloudflareWatermarkLowerLeft  = "lowerLeft"
	CloudflareWatermarkCenter     = "center"
)
