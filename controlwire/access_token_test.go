package controlwire_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

func accessTokenFixture(t testing.TB) controlwire.AccessToken {
	t.Helper()
	var raw [controlwire.AccessTokenBytes]byte
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	token, err := controlwire.NewAccessToken(raw)
	if err != nil {
		t.Fatalf("NewAccessToken() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := token.Destroy(); err != nil {
			t.Errorf("Destroy() error = %v, want nil", err)
		}
	})
	return token
}

func TestAccessTokenIdentityLayerTriad(t *testing.T) {
	t.Parallel()
	token := accessTokenFixture(t)
	verifier, err := token.Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	text, err := token.Reveal()
	if err != nil {
		t.Fatalf("Reveal() error = %v, want nil", err)
	}
	defer clear(text)
	var raw [controlwire.AccessTokenBytes]byte
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	// Independent standard-library derivation pins the wire version, not the
	// production helper or its constant. Changing this is a credential migration.
	digest := sha256.Sum256(append([]byte("primitive/access-token/v1\x00"), raw[:]...))
	want := []byte(`"` + hex.EncodeToString(digest[:]) + `"`)
	got, err := verifier.MarshalJSON()
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("verifier JSON = (%q, %v), want (%q, nil)", got, err, want)
	}
	for range 3 {
		matches, err := verifier.Matches(token)
		if err != nil || !matches {
			t.Fatalf("repeated Matches() = (%v, %v), want (true, nil)", matches, err)
		}
	}
	raw[0] ^= 1
	mutated, err := controlwire.NewAccessToken(raw)
	if err != nil {
		t.Fatalf("NewAccessToken(mutated) error = %v, want nil", err)
	}
	defer mutated.Destroy()
	other, err := mutated.Verifier()
	if err != nil || other == verifier {
		t.Fatalf("mutation verifier = (%v, %v), want distinct and nil", other, err)
	}
	if matches, err := verifier.Matches(mutated); err != nil || matches {
		t.Fatalf("Matches(one bit changed) = (%v, %v), want (false, nil)", matches, err)
	}
	if matches, err := verifier.Matches(controlwire.AccessToken{}); !errors.Is(err, core.ErrControlWireToken) || matches {
		t.Fatalf("Matches(absent token) = (%v, %v), want (false, typed refusal)", matches, err)
	}
	if matches, err := (controlwire.AccessTokenVerifier{}).Matches(token); !errors.Is(err, core.ErrControlWireToken) || matches {
		t.Fatalf("absent verifier Matches() = (%v, %v), want (false, typed refusal)", matches, err)
	}
	// A presented API token must never spend a one-use registration token.
	if got, err := controlwire.ParseRegistrationToken(text); !errors.Is(err, core.ErrControlWireToken) || got != (controlwire.RegistrationToken{}) {
		t.Fatalf("registration parser(API token) = (%v, %v), want zero and typed refusal", got, err)
	}
}

// The grammar has one fixed width and one byte alphabet, not forty policy
// transitions. Exhaust every byte at every position, plus adjacent extents.
func TestAccessTokenParserExhaustsEveryPositionAndByte(t *testing.T) {
	t.Parallel()
	token := accessTokenFixture(t)
	canonical, err := token.Reveal()
	if err != nil {
		t.Fatalf("Reveal() error = %v, want nil", err)
	}
	defer clear(canonical)
	for position := range canonical {
		for candidate := range 256 {
			input := bytes.Clone(canonical)
			input[position] = byte(candidate)
			wantAccepted := input[position] == canonical[position]
			if position >= len(controlwire.AccessTokenPrefix) {
				wantAccepted = candidate >= '0' && candidate <= '9' || candidate >= 'a' && candidate <= 'f'
			}
			got, err := controlwire.ParseAccessToken(input)
			clear(input)
			if !wantAccepted {
				if !errors.Is(err, core.ErrControlWireToken) || got != (controlwire.AccessToken{}) {
					t.Fatalf("ParseAccessToken(position %d byte %d) = (%v, %v), want zero and typed refusal", position, candidate, got, err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("ParseAccessToken(position %d byte %d) error = %v, want nil", position, candidate, err)
			}
			text, err := got.Reveal()
			if err != nil || len(text) != len(canonical) || text[position] != byte(candidate) {
				t.Fatalf("admitted token position %d = %v, want byte %d and nil", position, err, candidate)
			}
			clear(text)
			if err := got.Destroy(); err != nil {
				t.Fatalf("Destroy() error = %v, want nil", err)
			}
		}
	}
	for _, size := range []int{0, 1, controlwire.AccessTokenTextBytes - 1, controlwire.AccessTokenTextBytes + 1, 4096} {
		input := make([]byte, size)
		copy(input, canonical)
		got, err := controlwire.ParseAccessToken(input)
		clear(input)
		if !errors.Is(err, core.ErrControlWireToken) || got != (controlwire.AccessToken{}) {
			t.Fatalf("ParseAccessToken(%d bytes) = (%v, %v), want zero and typed refusal", size, got, err)
		}
	}
	zero, err := controlwire.NewAccessToken([controlwire.AccessTokenBytes]byte{})
	if !errors.Is(err, core.ErrSecretMaterialAllZero) || zero != (controlwire.AccessToken{}) {
		t.Fatalf("NewAccessToken(all zero) = (%v, %v), want zero and core all-zero refusal", zero, err)
	}
}

func TestAccessTokenCopiesRedactAndDestroyTogether(t *testing.T) {
	t.Parallel()
	token := accessTokenFixture(t)
	text, err := token.Reveal()
	if err != nil {
		t.Fatalf("Reveal() error = %v, want nil", err)
	}
	parsed, err := controlwire.ParseAccessToken(text)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v, want nil", err)
	}
	defer parsed.Destroy()
	clear(text)
	want, err := token.Verifier()
	if err != nil {
		t.Fatalf("Verifier() error = %v, want nil", err)
	}
	if matches, err := want.Matches(parsed); err != nil || !matches {
		t.Fatalf("Matches(after caller wipe) = (%v, %v), want (true, nil)", matches, err)
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
		if got := fmt.Sprintf(verb, token); got != core.RedactedValueText {
			t.Fatalf("token formatting %s = %q, want redaction", verb, got)
		}
	}
	copyHandle := token
	if err := token.Destroy(); err != nil {
		t.Fatalf("Destroy() error = %v, want nil", err)
	}
	if err := copyHandle.Validate(); !errors.Is(err, core.ErrControlWireToken) {
		t.Fatalf("copied handle Validate() error = %v, want typed refusal", err)
	}
	if got, err := copyHandle.Reveal(); !errors.Is(err, core.ErrControlWireToken) || got != nil {
		t.Fatalf("destroyed Reveal() = (%d bytes, %v), want nil and typed refusal", len(got), err)
	}
	if got, err := copyHandle.MarshalJSON(); !errors.Is(err, core.ErrJSONContract) || got != nil {
		t.Fatalf("destroyed MarshalJSON() = (%d bytes, %v), want nil and JSON refusal", len(got), err)
	}
}

func FuzzAccessTokenTextAndJSONSemanticClosure(f *testing.F) {
	token := accessTokenFixture(f)
	canonical, err := token.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	text, err := token.Reveal()
	if err != nil {
		f.Fatalf("Reveal(seed) error = %v, want nil", err)
	}
	f.Add(false, text)
	f.Add(true, canonical)
	f.Add(false, []byte{})
	f.Add(true, []byte(`null`))
	f.Add(true, canonical[:len(canonical)-1])
	f.Add(true, append(bytes.Clone(canonical), '0'))
	f.Fuzz(func(t *testing.T, jsonInput bool, data []byte) {
		prior := accessTokenFixture(t)
		got := prior
		var err error
		input := data
		if jsonInput {
			err = got.UnmarshalJSON(data)
			if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
				input = data[1 : len(data)-1]
			} else {
				input = nil
			}
		} else {
			got, err = controlwire.ParseAccessToken(data)
		}
		// Independent acceptance oracle: stdlib decode then canonical re-encode.
		wantAccepted := false
		var decoded []byte
		if len(input) == controlwire.AccessTokenTextBytes && bytes.HasPrefix(input, []byte(controlwire.AccessTokenPrefix)) {
			var decodeErr error
			decoded, decodeErr = hex.DecodeString(string(input[len(controlwire.AccessTokenPrefix):]))
			wantAccepted = decodeErr == nil && hex.EncodeToString(decoded) == string(input[len(controlwire.AccessTokenPrefix):]) && !bytes.Equal(decoded, make([]byte, controlwire.AccessTokenBytes))
		}
		defer clear(decoded)
		if err != nil {
			want := controlwire.AccessToken{}
			if jsonInput {
				want = prior
			}
			if wantAccepted || !errors.Is(err, core.ErrControlWireToken) || got != want {
				t.Fatalf("rejection = (%v, %v), want preserved/zero and typed rejection of inadmissible bytes", got, err)
			}
			if jsonInput && !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("JSON rejection = %v, want JSON identity", err)
			}
			return
		}
		defer got.Destroy()
		if !wantAccepted || got.Validate() != nil {
			t.Fatalf("acceptance = %v, want independently admitted canonical token", got)
		}
		revealed, err := got.Reveal()
		defer clear(revealed)
		if err != nil || !bytes.Equal(revealed, input) {
			t.Fatalf("Reveal() = (%d bytes, %v), want exact admitted bytes", len(revealed), err)
		}
		encoded, err := got.MarshalJSON()
		defer clear(encoded)
		if err != nil || len(encoded) != controlwire.AccessTokenJSONMaximumBytes {
			t.Fatalf("MarshalJSON() = (%d bytes, %v), want exact bounded document", len(encoded), err)
		}
		var round controlwire.AccessToken
		if err := round.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("canonical parse error = %v, want nil", err)
		}
		defer round.Destroy()
		verifier, err := got.Verifier()
		if err != nil {
			t.Fatalf("Verifier() error = %v, want nil", err)
		}
		if matches, err := verifier.Matches(round); err != nil || !matches {
			t.Fatalf("canonical identity = (%v, %v), want (true, nil)", matches, err)
		}
		second, err := round.MarshalJSON()
		defer clear(second)
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("second encoding = (%d bytes, %v), want byte-identical canonical document", len(second), err)
		}
	})
}

func FuzzAccessTokenVerifierJSONSemanticClosure(f *testing.F) {
	token := accessTokenFixture(f)
	prior, err := token.Verifier()
	if err != nil {
		f.Fatalf("Verifier(seed) error = %v, want nil", err)
	}
	canonical, err := prior.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add(canonical[:len(canonical)-1])
	f.Add(append(bytes.Clone(canonical), '0'))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := prior
		err := got.UnmarshalJSON(data)
		wantAccepted := false
		if len(data) == 2*sha256.Size+2 && data[0] == '"' && data[len(data)-1] == '"' {
			text := string(data[1 : len(data)-1])
			raw, decodeErr := hex.DecodeString(text)
			wantAccepted = decodeErr == nil && hex.EncodeToString(raw) == text && !bytes.Equal(raw, make([]byte, sha256.Size))
		}
		if err != nil {
			if wantAccepted || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlWireToken) || got != prior {
				t.Fatalf("verifier rejection = (%v, %v), want preserved and typed refusal", got, err)
			}
			var zero controlwire.AccessTokenVerifier
			zeroErr := zero.UnmarshalJSON(data)
			if !errors.Is(zeroErr, core.ErrControlWireToken) || zero != (controlwire.AccessTokenVerifier{}) {
				t.Fatalf("fresh verifier rejection = (%v, %v), want zero and typed refusal", zero, zeroErr)
			}
			return
		}
		if !wantAccepted || got.Validate() != nil {
			t.Fatalf("verifier acceptance = %v, want independently admitted digest", got)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, data) {
			t.Fatalf("verifier canonical output = (%q, %v), want (%q, nil)", encoded, err, data)
		}
		var round controlwire.AccessTokenVerifier
		if err := round.UnmarshalJSON(encoded); err != nil || round != got {
			t.Fatalf("verifier round trip = (%v, %v), want (%v, nil)", round, err, got)
		}
		second, err := round.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second verifier output = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}
