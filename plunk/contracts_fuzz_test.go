package plunk

import "testing"

func FuzzParseCredentialSemanticClosure(f *testing.F) {
	f.Add([]byte("sk_test_123"))
	f.Add([]byte("pk_public_123"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, value []byte) {
		credential, err := ParseCredential(value)
		if err != nil {
			if validationErr := credential.Validate(); validationErr == nil {
				t.Fatalf("ParseCredential(%q) = rejected %v with validation %v, want rejected invalid credential", value, err, validationErr)
			}
			return
		}
		if validationErr := credential.Validate(); validationErr != nil {
			t.Fatalf("ParseCredential(%q) = admitted with validation %v, want nil", value, validationErr)
		}
		_ = credential.Close()
	})
}
