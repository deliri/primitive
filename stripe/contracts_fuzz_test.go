package stripe

import "testing"

func FuzzParseCredentialSemanticClosure(f *testing.F) {
	f.Add([]byte("rk_test_123"))
	f.Add([]byte("sk_live_123"))
	f.Add([]byte("pk_public"))
	f.Fuzz(func(t *testing.T, value []byte) {
		credential, err := ParseCredential(value)
		if err != nil {
			if validationErr := credential.Validate(); validationErr == nil {
				t.Fatalf("ParseCredential(%q) = rejected %v with validation %v, want rejected invalid credential", value, err, validationErr)
			}
			return
		}
		validationErr := credential.Validate()
		kindErr := credential.Kind().Validate()
		if validationErr != nil || kindErr != nil {
			t.Fatalf("ParseCredential(%q) validation = (%v, kind %v), want both nil", value, validationErr, kindErr)
		}
		_ = credential.Close()
	})
}
