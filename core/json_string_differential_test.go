package core

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzMarshalCanonicalJSONStringMatchesDocumentEncoder holds the direct string
// quoting byte-identical to the document encoder for every valid string, and
// keeps invalid UTF-8 refused with the JSON contract identity. Seeds cover
// every escape class the quoting grammar distinguishes.
func FuzzMarshalCanonicalJSONStringMatchesDocumentEncoder(f *testing.F) {
	for _, seed := range []string{
		"",
		"plain",
		"/srv/primitive/objects/entry-0001.json",
		"quote\" backslash\\ slash/",
		"controls\x00\x01\x08\x09\x0a\x0c\x0d\x1f\x7f",
		"html <script>&amp;</script>",
		"line separators    ",
		"non-ASCII é ß 世界 \U0001f600",
		"replacement �",
		strings.Repeat("\x1f", 64),
		"invalid \xff utf-8",
		"truncated \xe2\x82",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := MarshalCanonicalJSONString(value)
		if !utf8.ValidString(value) {
			if !errors.Is(gotErr, ErrJSONContract) || got != nil {
				t.Fatalf("MarshalCanonicalJSONString(%q) = %q, %v; want nil, %v", value, got, gotErr, ErrJSONContract)
			}
			return
		}
		want, wantErr := MarshalCanonicalJSONDocument(value)
		if wantErr != nil || gotErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("MarshalCanonicalJSONString(%q) = %q, %v; want document spelling %q, %v", value, got, gotErr, want, wantErr)
		}
		decoded, decodeErr := DecodeJSONStringToken(got)
		if decodeErr != nil || decoded != value {
			t.Fatalf("DecodeJSONStringToken(%q) = %q, %v; want %q", got, decoded, decodeErr, value)
		}
	})
}

// TestLowerHexJSONMatchesCanonicalString proves the fixed-identity encoders
// spell exactly what the canonical string encoder would for the same digits.
func TestLowerHexJSONMatchesCanonicalString(t *testing.T) {
	t.Parallel()
	digest := SHA256Of([]byte("lower hex differential"))
	commit, err := ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatal(err)
	}
	key := Ed25519PublicKey{value: [32]byte{0: 0xff, 31: 0x0a}, set: true}
	cases := []struct {
		name    string
		marshal func() ([]byte, error)
		text    func() (string, error)
	}{
		{name: "sha256 digest", marshal: digest.MarshalJSON, text: digest.Hex},
		{name: "build commit", marshal: commit.MarshalJSON, text: func() (string, error) { return commit.String(), nil }},
		{name: "ed25519 public key", marshal: key.MarshalJSON, text: key.Hex},
	}
	for _, tc := range cases {
		text, textErr := tc.text()
		want, wantErr := MarshalCanonicalJSONString(text)
		got, gotErr := tc.marshal()
		if textErr != nil || wantErr != nil || gotErr != nil || !bytes.Equal(got, want) || len(text) == 0 {
			t.Fatalf("%s MarshalJSON() = %q, %v; want %q (text %q, %v)", tc.name, got, gotErr, want, text, textErr)
		}
	}
	for _, unset := range []func() ([]byte, error){SHA256Digest{}.MarshalJSON, Ed25519PublicKey{}.MarshalJSON, BuildCommit{}.MarshalJSON} {
		if got, gotErr := unset(); got != nil || !errors.Is(gotErr, ErrJSONContract) {
			t.Fatalf("unset MarshalJSON() = %q, %v; want nil, %v", got, gotErr, ErrJSONContract)
		}
	}
}
