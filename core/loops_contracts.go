package core

// Loops' protocol facts are deliberately nominal. Equal values in another
// provider family do not create a shared contract.
//
// Sources: https://app.loops.so/openapi.json and https://loops.so/docs/webhooks.
const (
	LoopsAPIHost                          = "app.loops.so"
	LoopsIdempotencyKeyMaximumBytes       = 100
	LoopsCredentialMinimumBytes           = 1
	LoopsCredentialCustodyMaximumBytes    = 4 * 1024
	LoopsContactIdentifierMaximumBytes    = 500
	LoopsCursorMaximumBytes               = 500
	LoopsPageSizeMinimum                  = 10
	LoopsPageSizeMaximum                  = 50
	LoopsWebhookSecretMinimumKeyBytes     = 1
	LoopsWebhookSecretCustodyMaximumBytes = 4 * 1024
	LoopsWebhookIDHeaderName              = "webhook-id"
	LoopsWebhookTimestampHeaderName       = "webhook-timestamp"
	LoopsWebhookSignatureHeaderName       = "webhook-signature"
	LoopsWebhookIDMaximumBytes            = 256
	LoopsWebhookTimestampMaximumBytes     = 19
	LoopsWebhookSignatureMaximumBytes     = 8 * 1024
	LoopsWebhookToleranceSeconds          = 300
	LoopsWebhookSignatureVersion          = "v1"
	LoopsWebhookSignatureEntrySeparator   = " "
	LoopsWebhookSignatureVersionSeparator = ","
	LoopsWebhookSignedContentSeparator    = "."
	LoopsWebhookScratchFilename           = "body"
)
