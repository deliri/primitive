package core

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestOAuthNominalJSONAtomicityAndSecretCustody(t *testing.T) {
	t.Parallel()
	fixtures := coreFixturesForFuzz(t)
	for _, row := range []struct {
		name string
		make func() (coreJSONReceiver, coreJSONReceiver)
	}{
		{name: "client identity", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthClientID) }},
		{name: "grant code", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthAuthorizationCode) }},
		{name: "callback URI", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthRedirectURI) }},
		{name: "anti-forgery state", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthState) }},
		{name: "proof-key verifier", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthPKCEVerifier) }},
		{name: "S256 challenge", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthPKCEChallenge) }},
		{name: "OpenID nonce", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthNonce) }},
		{name: "stable subject", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthSubject) }},
		{name: "provider email", make: func() (coreJSONReceiver, coreJSONReceiver) { return coreJSONReceivers(fixtures.oauthEmail) }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			receiver, fresh := row.make()
			canonical, err := receiver.MarshalJSON()
			if err != nil || len(canonical) == 0 {
				t.Fatalf("nominal JSON = (%q,%v), want owning canonical bytes", canonical, err)
			}
			if err := fresh.UnmarshalJSON(canonical); err != nil {
				t.Fatalf("nominal roundtrip = %v, want nil", err)
			}
			again, err := fresh.MarshalJSON()
			if err != nil || !bytes.Equal(again, canonical) {
				t.Fatalf("nominal fixed point = (%q,%v), want exact canonical bytes", again, err)
			}
			for _, bad := range [][]byte{[]byte("null"), []byte("{}"), []byte("true"), []byte(`""`), []byte(`"unterminated`), append(append([]byte(nil), canonical...), []byte(" false")...)} {
				err := receiver.UnmarshalJSON(bad)
				after, marshalErr := receiver.MarshalJSON()
				if !errors.Is(err, ErrJSONContract) || !errors.Is(err, ErrOAuthContract) || marshalErr != nil || !bytes.Equal(after, canonical) {
					t.Fatalf("refused nominal JSON = (%v,%v,unchanged%t), want typed atomic refusal", err, marshalErr, bytes.Equal(after, canonical))
				}
			}
		})
	}
	t.Run("negative/nil decoder receivers", func(t *testing.T) {
		t.Parallel()
		for _, receiver := range []coreJSONReceiver{(*OAuthClientID)(nil), (*OAuthAuthorizationCode)(nil), (*OAuthRedirectURI)(nil), (*OAuthState)(nil), (*OAuthPKCEVerifier)(nil), (*OAuthPKCEChallenge)(nil), (*OAuthNonce)(nil), (*OAuthSubject)(nil), (*OAuthEmail)(nil)} {
			if err := receiver.UnmarshalJSON([]byte(`"value"`)); !errors.Is(err, ErrJSONContract) || !errors.Is(err, ErrOAuthContract) {
				t.Fatalf("nil nominal JSON = %v, want typed refusal", err)
			}
		}
	})
	t.Run("positive/secret copy destruction and formatter custody", func(t *testing.T) {
		t.Parallel()
		source := []byte("owned-client-secret")
		secret, err := NewOAuthClientSecret(source)
		if err != nil {
			t.Fatal(err)
		}
		source[0] = 'X'
		copyValue, err := secret.CopyBytes()
		if err != nil || string(copyValue) != "owned-client-secret" {
			t.Fatalf("secret copy = (%q,%v), want independent owned bytes", copyValue, err)
		}
		defer clear(copyValue)
		for _, verb := range []string{"%v", "%+v", "%#v"} {
			if text := fmt.Sprintf(verb, secret); strings.Contains(text, "owned-client-secret") {
				t.Fatalf("secret formatter %s leaked material", verb)
			}
		}
		if data, err := secret.MarshalJSON(); len(data) != 0 || !errors.Is(err, ErrJSONContract) {
			t.Fatalf("secret JSON = (%q,%v), want refusal", data, err)
		}
		alias := secret
		if err := secret.Close(); err != nil {
			t.Fatal(err)
		}
		if got, err := alias.CopyBytes(); got != nil || !errors.Is(err, ErrOAuthContract) {
			t.Fatalf("destroyed alias = (%v,%v), want no bytes and owning refusal", got, err)
		}
	})
	if !errors.Is(ErrOAuthContract, ErrPrimitiveContract) || !errors.Is(ErrFirestoreDocumentsOperation, ErrFirestoreDocumentsContract) {
		t.Fatalf("OAuth/Firestore error parents = (%t,%t), want both compiler-owned", errors.Is(ErrOAuthContract, ErrPrimitiveContract), errors.Is(ErrFirestoreDocumentsOperation, ErrFirestoreDocumentsContract))
	}
}
