package core

// PayPal's protocol facts are deliberately nominal. Equal values in another
// provider family do not create a shared contract.
const (
	PayPalLiveAPIHost                       = "api-m.paypal.com"
	PayPalSandboxAPIHost                    = "api-m.sandbox.paypal.com"
	PayPalRequestIDHeaderName               = "PayPal-Request-Id"
	PayPalRequestIDMaximumBytes             = 38
	PayPalAccessTokenCustodyMaximumBytes    = 4 * 1024
	PayPalClientIDCustodyMaximumBytes       = 255
	PayPalClientSecretCustodyMaximumBytes   = 1024
	PayPalWebhookEventCustodyMaximumBytes   = JSONDocumentMaximumBytes - 64*1024
	PayPalWebhookIDMaximumBytes             = 50
	PayPalAuthAlgorithmMaximumBytes         = 100
	PayPalCertificateURLMaximumBytes        = 500
	PayPalTransmissionIDMaximumBytes        = 50
	PayPalTransmissionSignatureMaximumBytes = 500
	PayPalTransmissionTimeMaximumBytes      = 100
	PayPalAuthAlgorithmHeaderName           = "PAYPAL-AUTH-ALGO"
	PayPalCertificateURLHeaderName          = "PAYPAL-CERT-URL"
	PayPalTransmissionIDHeaderName          = "PAYPAL-TRANSMISSION-ID"
	PayPalTransmissionSignatureHeaderName   = "PAYPAL-TRANSMISSION-SIG"
	PayPalTransmissionTimeHeaderName        = "PAYPAL-TRANSMISSION-TIME"
	PayPalLiveCertificateHost               = "api.paypal.com"
	PayPalSandboxCertificateHost            = "api.sandbox.paypal.com"
)
