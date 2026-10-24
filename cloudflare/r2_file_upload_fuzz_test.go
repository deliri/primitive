package cloudflare

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// The credential-file decoder admits only the strict two-member document
// with visible key material. Every refusal is an authentication identity
// with zero credentials; every acceptance re-encodes to a canonical document
// that decodes to the same key bytes, so nothing beyond the two members can
// shape the result.
func FuzzR2CredentialFileSemantic(f *testing.F) {
	// The valid seed comes from the owning typed shape, not handwritten text.
	valid, err := jsonio.MarshalValidated(r2CredentialFile{AccessKeyID: "r2-access-key", SecretAccessKey: "r2-secret-key"}, jsonio.OwnedCodecPolicy{})
	if err != nil {
		f.Fatalf("valid credential seed: %v", err)
	}
	f.Add(valid)
	for _, seed := range []string{
		`{"access_key_id":"r2-access-key","secret_access_key":"r2-secret-key","region":"auto"}`,
		`{"access_key_id":"r2-access-key","secret_access_key":"r2-secret-key","session_token":"t"}`,
		`{"access_key_id":"r2-access-key"}`,
		`{"access_key_id":"","secret_access_key":"r2-secret-key"}`,
		`{"access_key_id":"r2 access","secret_access_key":"r2-secret-key"}`,
		`{"ACCESS_KEY_ID":"r2-access-key","secret_access_key":"r2-secret-key"}`,
		`{"access_key_id":"a","access_key_id":"b","secret_access_key":"c"}`,
		`{"access_key_id":`,
		`null`,
		`[]`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		credentials, err := decodeR2Credentials(data)
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareAuthentication) || credentials.accessKey != nil || credentials.secretKey != nil {
				t.Fatalf("refused %q: credentials set %t, err %v, want zero and %v", data, credentials.accessKey != nil, err, core.ErrCloudflareAuthentication)
			}
			return
		}
		if credentials.Validate() != nil {
			t.Fatalf("accepted %q: credentials invalid %v", data, credentials.Validate())
		}
		canonical, err := jsonio.MarshalValidated(r2CredentialFile{AccessKeyID: string(credentials.accessKey), SecretAccessKey: string(credentials.secretKey)}, jsonio.OwnedCodecPolicy{})
		if err != nil {
			t.Fatal(err)
		}
		again, err := decodeR2Credentials(canonical)
		if err != nil || !bytes.Equal(again.accessKey, credentials.accessKey) || !bytes.Equal(again.secretKey, credentials.secretKey) {
			t.Fatalf("canonical %s decoded to %v/%v, want the same key bytes", canonical, again.Validate(), err)
		}
		if err := credentials.Close(); err != nil || credentials.accessKey != nil {
			t.Fatalf("Close = %v, key retained %t, want destroyed", err, credentials.accessKey != nil)
		}
	})
}
