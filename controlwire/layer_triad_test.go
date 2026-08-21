package controlwire_test

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

// TestControlWireScalarLayerTriad is this package's layer triad (testing
// protocol: test/layer-triad). The layer is the wire boundary itself: the one
// place a peer's bytes become a typed scalar and a typed scalar becomes bytes.
//
// Each scalar gets all three cases rather than one shared example, because the
// three fail differently. The revision decides whether the conversation can
// happen at all, the nonce decides whether a request is distinguishable from a
// replay, and the token is a bearer secret. A triad proved only on the revision
// would say nothing about whether the token's neutral case leaks.
func TestControlWireScalarLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every scalar round trips through its canonical text", func(t *testing.T) {
		t.Parallel()

		revision, err := controlwire.ParseRevision(controlwire.Revision2026V1Token)
		if err != nil || revision != controlwire.Revision2026V1 {
			t.Fatalf("ParseRevision() = (%v, %v), want (%v, nil)", revision, err, controlwire.Revision2026V1)
		}
		nonce, err := controlwire.ParseRequestNonce(nonceHexWithLetters)
		if err != nil {
			t.Fatalf("ParseRequestNonce() error = %v, want nil", err)
		}
		token := mustToken(t, tokenHexWithLetters)

		for _, subject := range []struct {
			value any
			name  string
			want  string
		}{
			{name: "revision", value: revision, want: `"` + controlwire.Revision2026V1Token + `"`},
			{name: "nonce", value: nonce, want: `"` + nonceHexWithLetters + `"`},
			{name: "token", value: token, want: `"` + tokenHexWithLetters + `"`},
			{
				name: "policy cursor", value: mustPolicyCursor(t),
				want: `{"revision":"` + policyRevisionRealWorld + `","activation":1}`,
			},
		} {
			encoded, err := json.Marshal(subject.value)
			if err != nil {
				t.Fatalf("json.Marshal(%s) error = %v, want nil", subject.name, err)
			}
			if string(encoded) != subject.want {
				t.Fatalf("json.Marshal(%s) = %s, want %s", subject.name, encoded, subject.want)
			}
		}
	})

	t.Run("negative every scalar refuses a hostile value with its own identity", func(t *testing.T) {
		t.Parallel()

		for _, subject := range []struct {
			decode   func([]byte) error
			name     string
			document string
			want     core.ErrorIdentity
		}{
			{
				name: "revision refuses an unpublished contract", document: `"2027.1"`,
				want: core.ErrControlWireRevision,
				decode: func(data []byte) error {
					var value controlwire.Revision
					return value.UnmarshalJSON(data)
				},
			},
			{
				name: "nonce refuses a value that is not unpredictable", document: `"` + nonceHexAllZero + `"`,
				want: core.ErrControlWireNonce,
				decode: func(data []byte) error {
					var value controlwire.RequestNonce
					return value.UnmarshalJSON(data)
				},
			},
			{
				name: "token refuses a value that is not secret material", document: `"` + tokenHexAllZero + `"`,
				want: core.ErrControlWireToken,
				decode: func(data []byte) error {
					var value controlwire.RegistrationToken
					return value.UnmarshalJSON(data)
				},
			},
			{
				name: "verifier refuses a digest no token can derive", document: `"` + verifierHexAllZero + `"`,
				want: core.ErrControlWireToken,
				decode: func(data []byte) error {
					var value controlwire.RegistrationTokenVerifier
					return value.UnmarshalJSON(data)
				},
			},
			{
				name:     "policy cursor refuses the reserved absent revision",
				document: `{"revision":"` + policyRevisionAllZero + `","activation":1}`,
				want:     core.ErrControlWirePolicyCursor,
				decode: func(data []byte) error {
					var value controlwire.PolicyCursor
					return value.UnmarshalJSON(data)
				},
			},
		} {
			err := subject.decode([]byte(subject.document))
			if !errors.Is(err, subject.want) {
				t.Errorf("%s error = %v, want %v", subject.name, err, subject.want)
			}
			if !errors.Is(err, core.ErrControlWireContract) {
				t.Errorf("%s error = %v, want %v", subject.name, err, core.ErrControlWireContract)
			}
			if !errors.Is(err, core.ErrPrimitiveContract) {
				t.Errorf("%s error = %v, want the %v parent", subject.name, err, core.ErrPrimitiveContract)
			}
		}
	})

	t.Run("neutral an absent value produces no wire text and no fake identity", func(t *testing.T) {
		t.Parallel()

		// The unset case is the one a caller reaches by forgetting a field
		// rather than by sending something hostile. It must render as nothing
		// and emit nothing, never as a plausible-looking zero identity that a
		// peer would accept.
		var (
			revision controlwire.Revision
			nonce    controlwire.RequestNonce
			token    controlwire.RegistrationToken
			verifier controlwire.RegistrationTokenVerifier
		)
		if got := revision.String(); got != "" {
			t.Errorf("unset Revision.String() = %q, want empty", got)
		}
		if got := nonce.String(); got != "" {
			t.Errorf("unset RequestNonce.String() = %q, want empty", got)
		}
		if got := verifier.String(); got != "" {
			t.Errorf("unset RegistrationTokenVerifier.String() = %q, want empty", got)
		}
		// The unset token renders as the redaction rather than as empty text,
		// because a secret carrier must not have a rendering mode that depends
		// on whether it currently holds a secret.
		if got := fmt.Sprintf("%v", token); got != core.RedactedValueText {
			t.Errorf("unset RegistrationToken rendering = %q, want %q", got, core.RedactedValueText)
		}
		// The policy revision is the one scalar here that cannot render as
		// empty: it is sixteen raw bytes with no set flag, so its unset value
		// renders as a well-formed identifier. Refusing it is therefore a
		// Validate() obligation rather than a rendering one, which is why the
		// marshal refusal below is the property that actually protects the wire.
		var cursor controlwire.PolicyCursor
		if got := cursor.Revision.String(); got == "" {
			t.Errorf("unset PolicyRevisionID.String() = %q, want a well-formed identifier", got)
		}

		for _, subject := range []struct {
			value any
			name  string
		}{
			{name: "revision", value: revision},
			{name: "nonce", value: nonce},
			{name: "token", value: token},
			{name: "verifier", value: verifier},
			{name: "policy cursor", value: cursor},
			{name: "policy revision", value: cursor.Revision},
		} {
			encoded, err := json.Marshal(subject.value)
			if err == nil {
				t.Errorf("json.Marshal(unset %s) = %s, want a refusal", subject.name, encoded)
			}
			if len(encoded) != 0 {
				t.Errorf("json.Marshal(unset %s) = %s, want no bytes", subject.name, encoded)
			}
		}
	})
}
