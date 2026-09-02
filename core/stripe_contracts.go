package core

// Stripe's protocol facts are deliberately nominal. Equal values in another
// provider family do not create a shared contract.
const (
	StripeAPIHost                          = "api.stripe.com"
	StripeAPIVersion                       = "2026-08-26.dahlia"
	StripeVersionHeaderName                = "Stripe-Version"
	StripeIdempotencyKeyMaximumBytes       = 255
	StripeCredentialMinimumBytes           = 4
	StripeCredentialCustodyMaximumBytes    = 4 * 1024
	StripeWebhookSecretMinimumBytes        = 7
	StripeWebhookSecretCustodyMaximumBytes = 4 * 1024
	StripeWebhookCustodyMaximumBytes       = 1 << 20
	StripeWebhookSignatureHeaderName       = "Stripe-Signature"
	StripeWebhookSignatureMaximumBytes     = 8 * 1024
)
