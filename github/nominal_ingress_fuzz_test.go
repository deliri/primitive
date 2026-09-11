package github

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzGitHubNominalIngressSemanticClosure(f *testing.F) {
	ref, err := ParseReference("refs/tags/v1.2.3")
	if err != nil {
		f.Fatal(err)
	}
	agent, err := ParseUserAgent("primitive-test")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(ref.String(), uint64(1))
	f.Add(agent.String(), uint64(math.MaxUint64))
	f.Add("", uint64(0))
	f.Add("\r\n", uint64(2))
	f.Fuzz(func(t *testing.T, raw string, identifier uint64) {
		reference, err := ParseReference(raw)
		if err != nil {
			if !errors.Is(err, core.ErrGitHubContract) || reference != (Reference{}) {
				t.Fatalf("reference=%+v/%v, want zero typed refusal", reference, err)
			}
		} else {
			round, roundErr := ParseReference(reference.String())
			if reference.Validate() != nil || reference.String() != raw || roundErr != nil || round != reference {
				t.Fatalf("reference round trip=%+v/%v, want exact admitted text %q", round, roundErr, raw)
			}
		}
		identity, err := ParseUserAgent(raw)
		if err != nil {
			if !errors.Is(err, core.ErrGitHubContract) || identity != (UserAgent{}) {
				t.Fatalf("user agent=%+v/%v, want zero typed refusal", identity, err)
			}
		} else {
			round, roundErr := ParseUserAgent(identity.String())
			if identity.Validate() != nil || identity.String() != raw || roundErr != nil || round != identity {
				t.Fatalf("agent round trip=%+v/%v, want exact text %q", round, roundErr, raw)
			}
		}
		app, appErr := NewAppID(identifier)
		installation, installationErr := NewInstallationID(identifier)
		if identifier == 0 {
			if !errors.Is(appErr, core.ErrGitHubContract) || !errors.Is(installationErr, core.ErrGitHubContract) || app != (AppID{}) || installation != (InstallationID{}) {
				t.Fatalf("zero IDs=%v/%v, want typed refusals and zero values", appErr, installationErr)
			}
			return
		}
		appValue, appValueErr := app.Uint64()
		installationValue, installationValueErr := installation.Uint64()
		if errors.Join(appErr, installationErr, appValueErr, installationValueErr, app.Validate(), installation.Validate()) != nil || appValue != identifier || installationValue != identifier {
			t.Fatalf("IDs=%d/%d errors=%v/%v, want exact %d", appValue, installationValue, appErr, installationErr, identifier)
		}
	})
}

func FuzzGitHubAppCredentialSemanticCustody(f *testing.F) {
	block, _ := pem.Decode([]byte(fixedRSAPrivateKey))
	if block == nil {
		f.Fatal("fixture PEM block=nil, want RSA key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		f.Fatalf("fixture RSA=%v, want nil", err)
	}
	canonical := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	app, err := NewAppID(1)
	if err != nil {
		f.Fatal(err)
	}
	installation, err := NewInstallationID(2)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := NewAppCredential(app, installation, canonical)
	if err != nil {
		f.Fatal(err)
	}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("-----BEGIN RSA PRIVATE KEY-----\ntruncated"))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > core.GitHubAppPrivateKeyCustodyMaximumBytes+1 {
			t.Skip("input exceeds bounded credential oracle custody")
		}
		before := bytes.Clone(payload)
		ownedInput := bytes.Clone(payload)
		got, err := NewAppCredential(app, installation, ownedInput)
		if !bytes.Equal(ownedInput, before) {
			t.Fatal("caller key changed=true, want false")
		}
		if err != nil {
			if !errors.Is(err, core.ErrGitHubAuthentication) || got.state != nil {
				t.Fatalf("credential=%v, want typed refusal and zero custody", err)
			}
			return
		}
		defer func() {
			if err := got.Close(); err != nil {
				t.Errorf("credential Close()=%v, want nil", err)
			}
		}()
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted Validate()=%v, want nil", err)
		}
		block, _ := pem.Decode(payload)
		if block == nil {
			t.Fatal("accepted PEM block=nil, want key")
		}
		independent, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
		if parseErr != nil {
			parsed, otherErr := x509.ParsePKCS8PrivateKey(block.Bytes)
			if otherErr != nil {
				t.Fatalf("accepted key parse=%v, want nil", otherErr)
			}
			var ok bool
			independent, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				t.Fatal("accepted RSA=false, want true")
			}
		}
		// Independent public-key verification pins usable key material, not just PEM grammar.
		digest := sha256.Sum256([]byte("primitive credential custody"))
		signature, signErr := rsa.SignPKCS1v15(nil, independent, crypto.SHA256, digest[:])
		if signErr != nil {
			t.Fatalf("accepted RSA signing=%v, want nil", signErr)
		}
		if err := rsa.VerifyPKCS1v15(&independent.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
			t.Fatalf("signature verification=%v, want nil", err)
		}
		clear(ownedInput)
		if err := got.Validate(); err != nil {
			t.Fatalf("caller mutation damaged owned credential=%v, want nil", err)
		}
		if got.state.app != app || got.state.installation != installation || !bytes.Equal(got.state.privateKey, before) {
			t.Fatal("retained identity or key changed=true, want false")
		}
		if text := fmt.Sprintf("%v", got); text != core.RedactedValueText {
			t.Fatalf("credential diagnostic=%q, want redacted text", text)
		}
	})
}
