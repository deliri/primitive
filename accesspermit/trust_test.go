package accesspermit

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestPermitTrustedSignerAndDetachedFacts(t *testing.T) {
	t.Parallel()
	document, keys, _ := permitFixture(t)
	foreignKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	public, err := core.NewEd25519PublicKey(foreignKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("public key = %v, want nil", err)
	}
	foreignKeys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("trusted keys = %v, want nil", err)
	}
	foreignSignature, err := attest.Sign(attest.SignRequest[Domain]{Body: document.Terms, Signer: foreignKey})
	if err != nil {
		t.Fatalf("foreign signature = %v, want nil", err)
	}
	length, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("body length = %v, want nil", err)
	}
	for _, tc := range []struct {
		wantErr error
		mutate  func(*Document)
		name    string
		trusted attest.TrustedKeys
	}{
		{name: "authentic but untrusted signer", mutate: func(d *Document) { d.Signature = foreignSignature }, trusted: keys, wantErr: core.ErrAttestVerification},
		{name: "trusted key removal invalidates previous signer", mutate: func(*Document) {}, trusted: foreignKeys, wantErr: core.ErrAttestVerification},
		{name: "signature from foreign signer cannot be spliced", mutate: func(d *Document) { d.Signature.Signature = foreignSignature.Signature }, trusted: keys, wantErr: core.ErrAttestVerification},
		{name: "signer field cannot be spliced", mutate: func(d *Document) { d.Signature.Signer = public }, trusted: keys, wantErr: core.ErrAttestVerification},
		{name: "body length cannot be replaced", mutate: func(d *Document) { d.Signature.BodyLength = length }, trusted: keys, wantErr: core.ErrAttestVerification},
		{name: "body digest cannot be replaced", mutate: func(d *Document) { d.Signature.BodySHA256 = core.NewSHA256Digest([core.SHA256DigestBytes]byte{1}) }, trusted: keys, wantErr: core.ErrAttestVerification},
		{name: "empty trust set cannot authorize", mutate: func(*Document) {}, trusted: attest.TrustedKeys{}, wantErr: core.ErrAttestContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mutated := document
			tc.mutate(&mutated)
			if mutated == document && tc.trusted == keys {
				t.Fatalf("mutation = %v, want changed document or trust set", mutated)
			}
			got, err := Verify(mutated, document.Terms.Binding, tc.trusted)
			if got != (Verified{}) || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrAccessPermitDenied) {
				t.Fatalf("Verify = (%v, %v), want zero and (%v, denied)", got, err, tc.wantErr)
			}
		})
	}
}
