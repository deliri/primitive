package core

// Twilio's protocol facts are deliberately nominal. Equal values in another
// provider family do not create a shared contract.
const (
	TwilioAPIHost                         = "api.twilio.com"
	TwilioAPIKeySecretCustodyMaximumBytes = 4 * 1024
	TwilioAuthTokenCustodyMaximumBytes    = 4 * 1024
	TwilioWebhookCustodyMaximumBytes      = 1 << 20
	TwilioWebhookSignatureHeaderName      = "X-Twilio-Signature"
	TwilioWebhookSignatureBytes           = 28
	TwilioWebhookBodySHA256QueryName      = "bodySHA256"
)
