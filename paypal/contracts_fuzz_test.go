package paypal

import "testing"

func FuzzWebhookIDSemanticClosure(f *testing.F) {
	f.Add("WEBHOOK123")
	f.Add("")
	f.Add("bad-id")
	f.Fuzz(func(t *testing.T, value string) {
		identifier, err := ParsePayPalWebhookID(value)
		validationErr := identifier.Validate()
		if (err == nil) != (validationErr == nil) {
			t.Fatalf("ParsePayPalWebhookID(%q) errors = (%v, validation %v), want matching admission", value, err, validationErr)
		}
		if err == nil && identifier.String() != value {
			t.Fatalf("webhook ID round trip = %q, want %q", identifier.String(), value)
		}
	})
}
