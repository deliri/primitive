package controlwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzRevisionUnmarshalJSON(f *testing.F) {
	canonical, err := Revision2026V1.MarshalJSON()
	if err != nil {
		f.Fatalf("revision seed error=%v, want nil", err)
	}
	f.Add(canonical)
	f.Add(append([]byte{' '}, canonical...))
	for _, bad := range []string{`null`, `[]`, `{}`, `""`, `0`, `true`, `"\ud800"`} {
		f.Add([]byte(bad))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got := Revision2026V1
		wantAccept := controlwireJSONReferenceAccepts(controlwireJSONDoorRevision, data)
		err := got.UnmarshalJSON(data)
		if (err == nil) != wantAccept || got != Revision2026V1 {
			t.Fatalf("revision=%v/%v, want preserved %v and acceptance=%v", got, err, Revision2026V1, wantAccept)
		}
		if err != nil && (!errors.Is(err, core.ErrControlWireRevision) || !errors.Is(err, core.ErrJSONContract)) {
			t.Fatalf("revision refusal=%v, want revision/JSON identities", err)
		}
	})
}

func FuzzParseRequestNonce(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	canonical := fixtures.requestNonce.String()
	for _, seed := range []string{canonical, strings.ToUpper(canonical), canonical[:len(canonical)-1], canonical + "0", "", strings.Repeat("0", 2*core.SHA256DigestBytes)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		wantAccept := canonicalDigestReference(text, true)
		got, err := ParseRequestNonce(text)
		if (err == nil) != wantAccept {
			t.Fatalf("nonce error=%v, want acceptance=%v", err, wantAccept)
		}
		if err != nil {
			if !errors.Is(err, core.ErrControlWireNonce) || got != (RequestNonce{}) {
				t.Fatalf("nonce=%v/%v, want zero and nonce identity", got, err)
			}
			return
		}
		if got.String() != text || got.Validate() != nil {
			t.Fatalf("nonce=%q/%v, want %q/nil", got.String(), got.Validate(), text)
		}
		wire, err := got.MarshalJSON()
		var round RequestNonce
		roundErr := round.UnmarshalJSON(wire)
		if err != nil || roundErr != nil || round != got {
			t.Fatalf("nonce round trip=%v/%v/%v, want %v/nil", round, err, roundErr, got)
		}
	})
}

func FuzzParseRegistrationToken(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	canonical, err := fixtures.tokenText()
	if err != nil {
		f.Fatalf("token seed error=%v, want nil", err)
	}
	for _, seed := range []string{canonical, strings.ToUpper(canonical), canonical[:len(canonical)-1], canonical + "0", "", strings.Repeat("0", 2*RegistrationTokenBytes)} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) != 2*RegistrationTokenBytes {
			got, err := ParseRegistrationToken(data)
			if !errors.Is(err, core.ErrControlWireToken) || got.Validate() == nil {
				t.Fatalf("wrong-width token=%v/%v, want typed invalid zero", got, err)
			}
			return
		}
		wantAccept := canonicalDigestReference(string(data), true)
		before := bytes.Clone(data)
		owned := bytes.Clone(data)
		got, err := ParseRegistrationToken(owned)
		if (err == nil) != wantAccept || !bytes.Equal(data, before) {
			t.Fatalf("token parse error=%v input preserved=%v, want acceptance=%v and preserved", err, bytes.Equal(data, before), wantAccept)
		}
		if err != nil {
			if !errors.Is(err, core.ErrControlWireToken) || got.Validate() == nil {
				t.Fatalf("token=%v/%v, want invalid and token identity", got, err)
			}
			return
		}
		defer func() { _ = got.Destroy() }()
		raw, err := hex.DecodeString(string(data))
		if err != nil {
			t.Fatalf("reference hex error=%v, want nil", err)
		}
		want := sha256.Sum256(raw)
		clear(owned) // Caller storage must not alias the owned secret.
		verifier, err := got.Verifier()
		if err != nil || verifier.String() != hex.EncodeToString(want[:]) {
			t.Fatalf("verifier=%v/%v, want SHA256 %x", verifier, err, want)
		}
		wire, err := got.MarshalJSON()
		var round RegistrationToken
		roundErr := round.UnmarshalJSON(wire)
		if err != nil || roundErr != nil {
			t.Fatalf("token round trip error=%v/%v, want nil", err, roundErr)
		}
		defer func() { _ = round.Destroy() }()
		roundVerifier, err := round.Verifier()
		if err != nil || roundVerifier != verifier {
			t.Fatalf("round verifier=%v/%v, want %v/nil", roundVerifier, err, verifier)
		}
	})
}

func FuzzParseRegistrationTokenVerifier(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	canonical := fixtures.verifier.String()
	for _, seed := range []string{canonical, strings.ToUpper(canonical), canonical[:len(canonical)-1], canonical + "0", "", strings.Repeat("0", 2*core.SHA256DigestBytes)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		wantAccept := canonicalDigestReference(text, true)
		got, err := ParseRegistrationTokenVerifier(text)
		if (err == nil) != wantAccept {
			t.Fatalf("verifier error=%v, want acceptance=%v", err, wantAccept)
		}
		if err != nil {
			if !errors.Is(err, core.ErrControlWireToken) || got != (RegistrationTokenVerifier{}) {
				t.Fatalf("verifier=%v/%v, want zero and token identity", got, err)
			}
			return
		}
		if got.String() != text || !got.Equal(got) || got.Equal(RegistrationTokenVerifier{}) {
			t.Fatalf("verifier=%q reflexive=%v, want %q, reflexive and unequal to zero", got.String(), got.Equal(got), text)
		}
		wire, err := got.MarshalJSON()
		var round RegistrationTokenVerifier
		roundErr := round.UnmarshalJSON(wire)
		if err != nil || roundErr != nil || round != got {
			t.Fatalf("verifier round trip=%v/%v/%v, want %v/nil", round, err, roundErr, got)
		}
	})
}

func FuzzParsePolicyRevisionID(f *testing.F) {
	// Constructor is the compiler-visible byte array; big.Int is the independent
	// reference for the 128-bit integer, including the two unused leading bits.
	for _, value := range []PolicyRevisionID{{15: 1}, {0: 0x80}, {0: 0xff, 15: 0xff}} {
		f.Add(value.String())
	}
	for _, bad := range []string{"", (PolicyRevisionID{}).String(), strings.Repeat("Z", PolicyRevisionTextLength), strings.Repeat("I", PolicyRevisionTextLength)} {
		f.Add(bad)
	}
	f.Fuzz(func(t *testing.T, text string) {
		want, wantAccept := policyIDReference(text)
		got, err := ParsePolicyRevisionID(text)
		if (err == nil) != wantAccept || got != want {
			t.Fatalf("policy ID=%x/%v, want %x acceptance=%v", got, err, want, wantAccept)
		}
		if err != nil {
			if !errors.Is(err, core.ErrControlWirePolicyCursor) {
				t.Fatalf("policy refusal=%v, want %v", err, core.ErrControlWirePolicyCursor)
			}
			return
		}
		if got.String() != text {
			t.Fatalf("policy rendering=%q, want %q", got.String(), text)
		}
	})
}

func FuzzPolicyCursorUnmarshalJSON(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	canonical, err := fixtures.policyCursor.MarshalJSON()
	if err != nil {
		f.Fatalf("cursor seed error=%v, want nil", err)
	}
	f.Add(string(canonical))
	f.Add(" " + string(canonical))
	for _, bad := range []string{"", `null`, `{}`, `[]`} {
		f.Add(bad)
	}
	f.Fuzz(func(t *testing.T, document string) {
		data := []byte(document)
		wantAccept := controlwireJSONReferenceAccepts(controlwireJSONDoorPolicyCursor, data)
		got := fixtures.policyCursor
		err := got.UnmarshalJSON(data)
		if (err == nil) != wantAccept {
			t.Fatalf("cursor error=%v, want acceptance=%v", err, wantAccept)
		}
		if err != nil {
			if !errors.Is(err, core.ErrControlWirePolicyCursor) || !errors.Is(err, core.ErrJSONContract) || got != fixtures.policyCursor {
				t.Fatalf("cursor=%v/%v, want preserved %v and typed refusal", got, err, fixtures.policyCursor)
			}
			return
		}
		var reference cursorReference
		if err := json.Unmarshal(data, &reference); err != nil {
			t.Fatalf("reference decode error=%v, want nil", err)
		}
		revision, ok := policyIDReference(reference.Revision)
		want := PolicyCursor{Revision: revision, Activation: PolicyActivation(reference.Activation)}
		if !ok || got != want {
			t.Fatalf("cursor=%v, want exact facts %v", got, want)
		}
		wire, err := got.MarshalJSON()
		var round PolicyCursor
		roundErr := round.UnmarshalJSON(wire)
		second, secondErr := round.MarshalJSON()
		if err != nil || roundErr != nil || secondErr != nil || round != got || !bytes.Equal(second, wire) {
			t.Fatalf("cursor round trip=%v/%v/%v/%v, want %v and canonical fixed point", round, err, roundErr, secondErr, got)
		}
	})
}
