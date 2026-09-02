package twilio

import (
	"strings"
	"testing"
)

func FuzzParseSIDSemanticClosure(f *testing.F) {
	f.Add(true, "AC0123456789abcdefABCDEF0123456789")
	f.Add(false, apiKeySIDPrefix+strings.Repeat("0", sidHexBytes))
	f.Add(true, "ACbad")
	f.Fuzz(func(t *testing.T, account bool, value string) {
		if account {
			parsed, err := ParseAccountSID(value)
			validationErr := parsed.Validate()
			if (err == nil) != (validationErr == nil) {
				t.Fatalf("ParseAccountSID(%q) errors = (%v, validation %v), want matching admission", value, err, validationErr)
			}
			return
		}
		parsed, err := ParseAPIKeySID(value)
		validationErr := parsed.Validate()
		if (err == nil) != (validationErr == nil) {
			t.Fatalf("ParseAPIKeySID(%q) errors = (%v, validation %v), want matching admission", value, err, validationErr)
		}
	})
}
