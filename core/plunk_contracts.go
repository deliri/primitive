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
	PlunkWebhookCustodyMaximumBytes       = 1 << 20
)
