package core

// Plunk's protocol facts are deliberately nominal. Equal values in another
// provider family do not create a shared contract.
const (
	PlunkAPIHost                          = "next-api.useplunk.com"
	PlunkIdempotencyKeyMaximumBytes       = 255
	PlunkCredentialMinimumBytes           = 4
	PlunkCredentialCustodyMaximumBytes    = 4 * 1024
	PlunkWebhookSecretMinimumBytes        = 1
	PlunkWebhookSecretCustodyMaximumBytes = 4 * 1024
	// Attachments follow the documented /v1/send contract: a bounded array of
	// base64 objects. The published guide admits five objects and the concept
	// page ten megabytes per email; the stricter of each bound applies here.
	// https://docs.useplunk.com/guides/send-a-transactional-email-with-attachments
	PlunkAttachmentMaximumCount          = 5
	PlunkAttachmentsMaximumBytes         = 10 * 1024 * 1024
	PlunkAttachmentFilenameMaximumBytes  = 255
	PlunkAttachmentMediaTypeMaximumBytes = 127
)
