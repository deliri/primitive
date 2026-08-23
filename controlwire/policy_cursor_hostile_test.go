package controlwire_test

import (
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

const (
	// policyRevisionRealWorld is the exact rendering a control plane emits in a
	// signed response header. It is pinned as a literal rather than generated,
	// so a change to the codec fails here instead of agreeing with itself.
	policyRevisionRealWorld = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	// policyRevisionAllZero is well-formed and reserved as the absent identity.
	policyRevisionAllZero = "00000000000000000000000000"
	// policyRevisionMinimum is the smallest identifier that is not the reserved
	// absent value.
	policyRevisionMinimum = "00000000000000000000000001"
	// policyRevisionMaximum is the largest value that fits in sixteen bytes.
	policyRevisionMaximum = "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"
	// policyRevisionFirstOverflow is one past the maximum. It is well-formed
	// base32 and the correct length, and names a 129-bit number.
	policyRevisionFirstOverflow = "80000000000000000000000000"
)

// TestParsePolicyRevisionIDRefusesEveryNonCanonicalRendering drives the parser
// against the renderings a hostile or buggy peer would actually produce.
//
// The rejections that matter are not malformed text. They are the values that
// look right: a lowercase spelling, a Crockford alias, the reserved all-zero
// identity, and a number one bit too large. Each of those would either decode
// to something this build did not receive or re-encode to bytes it cannot echo.
func TestParsePolicyRevisionIDRefusesEveryNonCanonicalRendering(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "the rendering a control plane emits", value: policyRevisionRealWorld, want: true},
		{name: "the smallest identifier that is not absent", value: policyRevisionMinimum, want: true},
		{name: "the largest value that fits in sixteen bytes", value: policyRevisionMaximum, want: true},
		{name: "every symbol in the alphabet", value: "0123456789ABCDEFGHJKMNPQRS", want: true},
		{name: "the remaining symbols in the alphabet", value: "0TVWXYZ0123456789ABCDEFGHJ", want: true},

		{name: "empty text is refused", value: ""},
		{name: "one symbol short is refused", value: policyRevisionRealWorld[:25]},
		{name: "one symbol long is refused", value: policyRevisionRealWorld + "0"},
		{name: "the reserved absent identity is refused", value: policyRevisionAllZero},
		{name: "one past the sixteen-byte maximum is refused", value: policyRevisionFirstOverflow},
		{name: "a leading nine overflows and is refused", value: "90000000000000000000000000"},
		{name: "the largest leading symbol overflows and is refused", value: "Z0000000000000000000000000"},
		{name: "lowercase is refused", value: strings.ToLower(policyRevisionRealWorld)},
		{name: "one lowercase symbol is refused", value: "01ARZ3NDEKTSV4RRFFQ69G5FAv"},
		{name: "the crockford alias I for one is refused", value: "0IARZ3NDEKTSV4RRFFQ69G5FAV"},
		{name: "the crockford alias L for one is refused", value: "0LARZ3NDEKTSV4RRFFQ69G5FAV"},
		{name: "the crockford alias O for zero is refused", value: "0OARZ3NDEKTSV4RRFFQ69G5FAV"},
		{name: "the excluded symbol U is refused", value: "0UARZ3NDEKTSV4RRFFQ69G5FAV"},
		{name: "a leading space is refused", value: " 1ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{name: "a trailing space is refused", value: policyRevisionRealWorld[:25] + " "},
		{name: "a space padded value of the right length is refused", value: strings.Repeat(" ", 26)},
		{name: "a trailing newline is refused", value: policyRevisionRealWorld[:25] + "\n"},
		{name: "an embedded null byte is refused", value: "01ARZ3NDEKTSV4RRFFQ69G5FA\x00"},
		{name: "a hyphen is refused", value: "01ARZ3NDEKTSV4RRFFQ69G5FA-"},
		{name: "a non-ascii symbol is refused", value: "01ARZ3NDEKTSV4RRFFQ69G5FAé"},
		{name: "hex is refused", value: "0123456789abcdef0123456789"},
		{name: "an oversized value is refused", value: strings.Repeat(policyRevisionRealWorld, 64)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.ParsePolicyRevisionID(testCase.value)
			if testCase.want {
				if err != nil {
					t.Fatalf("ParsePolicyRevisionID(%q) error = %v, want nil", testCase.value, err)
				}
				// Acceptance is only correct if the value survives unchanged.
				if round := got.String(); round != testCase.value {
					t.Fatalf("round trip of %q = %q, want the input unchanged", testCase.value, round)
				}
				return
			}
			if !errors.Is(err, core.ErrControlWirePolicyCursor) {
				t.Fatalf("ParsePolicyRevisionID(%q) error = %v, want %v",
					testCase.value, err, core.ErrControlWirePolicyCursor)
			}
			if !errors.Is(err, core.ErrControlWireContract) {
				t.Errorf("ParsePolicyRevisionID(%q) error = %v, want the %v identity",
					testCase.value, err, core.ErrControlWireContract)
			}
			if got != (controlwire.PolicyRevisionID{}) {
				t.Errorf("ParsePolicyRevisionID(%q) = %v, want the zero identifier on rejection",
					testCase.value, got)
			}
			if err := got.Validate(); !errors.Is(err, core.ErrControlWirePolicyCursor) {
				t.Errorf("refused identifier Validate() error = %v, want %v", err, core.ErrControlWirePolicyCursor)
			}
		})
	}

	for bit := range 64 {
		values := [...]uint64{uint64(1) << bit, ^(uint64(1) << bit)}
		for variant, value := range values {
			got, err := controlwire.NewPolicyActivation(value)
			if err != nil || got.Uint64() != value {
				t.Fatalf("NewPolicyActivation(bit %d variant %d value %d) = (%d, %v), want (%d, nil)",
					bit, variant, value, got.Uint64(), err, value)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("NewPolicyActivation(bit %d variant %d).Validate() error = %v, want nil",
					bit, variant, err)
			}
		}
	}
}

// TestPolicyRevisionIDEncodingIsABijection proves the codec agrees with itself
// in the direction the wire actually travels: bytes a control plane holds
// become text, and that exact text must become those exact bytes again.
//
// Driven from byte patterns rather than from renderings, so a codec that is
// merely self-consistent on the values a test author happened to type still
// fails on a carry that crosses the sixty-four bit word boundary.
func TestPolicyRevisionIDEncodingIsABijection(t *testing.T) {
	t.Parallel()

	patterns := []struct {
		name  string
		bytes [16]byte
	}{
		{name: "minimum non-absent", bytes: [16]byte{15: 0x01}},
		{name: "maximum", bytes: [16]byte{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		}},
		{name: "high word only", bytes: [16]byte{0x01}},
		{name: "low word only", bytes: [16]byte{8: 0x01}},
		{name: "the word boundary carry", bytes: [16]byte{7: 0x01, 8: 0x80}},
		{name: "alternating bits", bytes: [16]byte{
			0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa,
			0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa, 0x55, 0xaa,
		}},
		{name: "every byte distinct", bytes: [16]byte{
			0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
			0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
		}},
	}

	for _, pattern := range patterns {
		t.Run(pattern.name, func(t *testing.T) {
			t.Parallel()

			identifier := controlwire.PolicyRevisionID(pattern.bytes)
			rendered := identifier.String()
			if len(rendered) != controlwire.PolicyRevisionTextLength {
				t.Fatalf("String() width = %d, want %d", len(rendered), controlwire.PolicyRevisionTextLength)
			}
			parsed, err := controlwire.ParsePolicyRevisionID(rendered)
			if err != nil {
				t.Fatalf("ParsePolicyRevisionID(%q) error = %v, want nil", rendered, err)
			}
			if parsed != identifier {
				t.Fatalf("ParsePolicyRevisionID(String(%x)) = %x, want the original bytes",
					pattern.bytes, parsed)
			}
		})
	}
}

// TestPolicyRevisionIDDistinctBytesRenderDistinctText proves the codec loses
// nothing. A renderer that dropped the top bits would still round trip every
// value whose top bits were zero, so collision is checked directly.
func TestPolicyRevisionIDDistinctBytesRenderDistinctText(t *testing.T) {
	t.Parallel()

	seen := make(map[string][16]byte)
	for bit := range 128 {
		var pattern [16]byte
		pattern[bit/8] = 1 << (bit % 8)
		rendered := controlwire.PolicyRevisionID(pattern).String()
		if previous, collided := seen[rendered]; collided {
			t.Fatalf("bit %d and bytes %x both render as %q, want distinct text",
				bit, previous, rendered)
		}
		seen[rendered] = pattern
	}
	if len(seen) != 128 {
		t.Fatalf("distinct renderings = %d, want 128", len(seen))
	}
}

func TestPolicyActivationRefusesTheUnsetCounter(t *testing.T) {
	t.Parallel()

	if err := controlwire.PolicyActivation(0).Validate(); !errors.Is(err, core.ErrControlWirePolicyCursor) {
		t.Fatalf("PolicyActivation(0).Validate() = %v, want %v", err, core.ErrControlWirePolicyCursor)
	}
	for _, activation := range []controlwire.PolicyActivation{1, 2, 1 << 32, 1<<64 - 1} {
		if err := activation.Validate(); err != nil {
			t.Errorf("PolicyActivation(%d).Validate() = %v, want nil", activation, err)
		}
		if got := activation.Uint64(); got != uint64(activation) {
			t.Errorf("PolicyActivation(%d).Uint64() = %d, want %d", activation, got, uint64(activation))
		}
	}
}

// TestNewPolicyActivationIsTheOnlyValidatedAdmission proves the door refuses
// exactly what the type refuses, at both ends of the counter's range.
//
// Before this door existed the only route to a value was the bare conversion
// PolicyActivation(n), which skips the Validate the type declares for itself.
// The activation is peer visible through PolicyCursor, so an unvalidated one
// minted on the producing end travels to the consuming end as a fact.
func TestNewPolicyActivationIsTheOnlyValidatedAdmission(t *testing.T) {
	t.Parallel()

	cases := [...]struct {
		wantErr error
		name    string
		value   uint64
	}{
		{name: "zero is the unset counter", value: 0, wantErr: core.ErrControlWirePolicyCursor},
		{name: "one is the first activation", value: 1},
		{name: "two is one above first", value: 2},
		{name: "one below eight bit boundary", value: 1<<8 - 1},
		{name: "at eight bit boundary", value: 1 << 8},
		{name: "one below sixteen bit boundary", value: 1<<16 - 1},
		{name: "at sixteen bit boundary", value: 1 << 16},
		{name: "one below thirty two bit boundary", value: 1<<32 - 1},
		{name: "at thirty two bit boundary", value: 1 << 32},
		{name: "one above thirty two bit boundary", value: 1<<32 + 1},
		{name: "one below maximum counter", value: 1<<64 - 2},
		{name: "maximum counter", value: 1<<64 - 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.NewPolicyActivation(testCase.value)
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Fatalf("NewPolicyActivation(%d) error = %v, want %v", testCase.value, err, testCase.wantErr)
				}
				if got != 0 {
					t.Fatalf("NewPolicyActivation(%d) = %d on refusal, want the zero value", testCase.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewPolicyActivation(%d) error = %v, want nil", testCase.value, err)
			}
			if got.Uint64() != testCase.value {
				t.Fatalf("NewPolicyActivation(%d).Uint64() = %d, want %d", testCase.value, got.Uint64(), testCase.value)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("NewPolicyActivation(%d).Validate() error = %v, want nil", testCase.value, err)
			}
		})
	}
}

// TestPolicyActivationOrdersOneRevisionHistory covers the comparison a caller
// uses to decide whether a cursor moved forward, including the ties and the
// saturating ends.
func TestPolicyActivationOrdersOneRevisionHistory(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  controlwire.PolicyActivation
		right controlwire.PolicyActivation
		want  core.Comparison
	}{
		{name: "an earlier activation is less", left: 1, right: 2, want: core.ComparisonLess},
		{name: "a later activation is greater", left: 2, right: 1, want: core.ComparisonGreater},
		{name: "the same activation ties", left: 7, right: 7, want: core.ComparisonEqual},
		{name: "the unset counter ties itself", left: 0, right: 0, want: core.ComparisonEqual},
		{name: "the unset counter is below every real one", left: 0, right: 1, want: core.ComparisonLess},
		{name: "the saturated counter is above every real one", left: 1<<64 - 1, right: 1, want: core.ComparisonGreater},
		{name: "the saturated counter ties itself", left: 1<<64 - 1, right: 1<<64 - 1, want: core.ComparisonEqual},
		{name: "one below saturation is still less", left: 1<<64 - 2, right: 1<<64 - 1, want: core.ComparisonLess},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := testCase.left.Compare(testCase.right); got != testCase.want {
				t.Fatalf("PolicyActivation(%d).Compare(%d) = %v, want %v",
					testCase.left, testCase.right, got, testCase.want)
			}
		})
	}
}

// TestPolicyCursorJSONBoundaryRefusesEveryMalformedDocument drives the decoder
// at the exact boundary where a peer's bytes become a decision this build acts
// on.
func TestPolicyCursorJSONBoundaryRefusesEveryMalformedDocument(t *testing.T) {
	t.Parallel()

	valid := `{"revision":"` + policyRevisionRealWorld + `","activation":1}`

	cases := []struct {
		wantErr  error
		name     string
		document string
	}{
		{name: "the document a control plane emits", document: valid},
		{name: "the largest activation", document: `{"revision":"` + policyRevisionRealWorld + `","activation":18446744073709551615}`},

		{name: "an empty document is refused", document: ``, wantErr: &jsontext.SyntacticError{}},
		{name: "null is refused", document: `null`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "an empty object is refused", document: `{}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a bare string is refused", document: `"` + policyRevisionRealWorld + `"`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "an array is refused", document: `["` + policyRevisionRealWorld + `",1]`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a missing revision is refused", document: `{"activation":1}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a missing activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `"}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "an unset activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":0}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "the reserved absent revision is refused", document: `{"revision":"` + policyRevisionAllZero + `","activation":1}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a null revision is refused", document: `{"revision":null,"activation":1}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a null activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":null}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a quoted activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":"1"}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a negative activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":-1}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a fractional activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":1.5}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "an exponent activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":1e2}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a leading-zero activation is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":01}`, wantErr: &jsontext.SyntacticError{}},
		{name: "an activation past the counter width is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":18446744073709551616}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "an unknown field is refused", document: `{"revision":"` + policyRevisionRealWorld + `","activation":1,"mode":"open"}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a lowercase revision is refused", document: `{"revision":"` + strings.ToLower(policyRevisionRealWorld) + `","activation":1}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "a nested object is refused", document: `{"revision":{"revision":"` + policyRevisionRealWorld + `"},"activation":1}`, wantErr: core.ErrControlWirePolicyCursor},
		{name: "trailing content is refused", document: valid + `{}`, wantErr: &jsontext.SyntacticError{}},
		{name: "a truncated document is refused", document: valid[:len(valid)-1], wantErr: &jsontext.SyntacticError{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// The receiver starts holding a real cursor, so a rejection that
			// silently blanked it would be visible here rather than later.
			existing := mustPolicyCursor(t)
			got := existing
			err := json.Unmarshal([]byte(testCase.document), &got)
			if testCase.wantErr == nil {
				if err != nil {
					t.Fatalf("json.Unmarshal(%s) error = %v, want nil", testCase.document, err)
				}
				encoded, marshalErr := json.Marshal(got)
				if marshalErr != nil {
					t.Fatalf("json.Marshal() error = %v, want nil", marshalErr)
				}
				if string(encoded) != testCase.document {
					t.Fatalf("re-encoded = %s, want the received bytes %s", encoded, testCase.document)
				}
				return
			}
			if _, syntax := testCase.wantErr.(*jsontext.SyntacticError); syntax {
				if _, ok := errors.AsType[*jsontext.SyntacticError](err); !ok {
					t.Fatalf("json.Unmarshal(%s) error = %v, want *jsontext.SyntacticError", testCase.document, err)
				}
			} else if !errors.Is(err, testCase.wantErr) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(%s) error = %v, want %v/%v", testCase.document, err, testCase.wantErr, core.ErrJSONContract)
			}
			if got != existing {
				t.Fatalf("json.Unmarshal(%s) mutated the receiver to %v, want it unchanged",
					testCase.document, got)
			}
		})
	}
}

// TestPolicyCursorRefusesToEmitAnUnsetValue proves the unset cursor cannot be
// put on the wire, in either half.
func TestPolicyCursorRefusesToEmitAnUnsetValue(t *testing.T) {
	t.Parallel()

	var cursor controlwire.PolicyCursor
	if err := cursor.Validate(); !errors.Is(err, core.ErrControlWirePolicyCursor) {
		t.Fatalf("unset PolicyCursor.Validate() = %v, want %v", err, core.ErrControlWirePolicyCursor)
	}
	for _, subject := range []struct {
		value any
		name  string
	}{
		{name: "cursor", value: cursor},
		{name: "revision", value: cursor.Revision},
	} {
		encoded, err := json.Marshal(subject.value)
		if !errors.Is(err, core.ErrControlWirePolicyCursor) || !errors.Is(err, core.ErrJSONContract) {
			t.Errorf("json.Marshal(unset %s) error = %v, want %v/%v", subject.name, err, core.ErrControlWirePolicyCursor, core.ErrJSONContract)
		}
		if len(encoded) != 0 {
			t.Errorf("json.Marshal(unset %s) = %s, want no bytes", subject.name, encoded)
		}
	}

	// A cursor with only one half set is the shape a partial write produces.
	half := controlwire.PolicyCursor{Revision: mustPolicyRevision(t)}
	if err := half.Validate(); !errors.Is(err, core.ErrControlWirePolicyCursor) {
		t.Errorf("PolicyCursor with no activation validated, want %v", core.ErrControlWirePolicyCursor)
	}
	other := controlwire.PolicyCursor{Activation: 1}
	if err := other.Validate(); !errors.Is(err, core.ErrControlWirePolicyCursor) {
		t.Errorf("PolicyCursor with no revision validated, want %v", core.ErrControlWirePolicyCursor)
	}
}

// TestUnsetPolicyRevisionRendersAWellFormedIdentifier states the one property
// that separates this identifier from the digest-backed scalars in this
// package.
//
// A digest scalar carries a set flag and renders as empty text when unset. This
// one is sixteen raw bytes, so the unset value renders as a perfectly
// well-formed twenty-six symbol identifier. That is precisely why all-zero is
// reserved and refused at every boundary rather than left to look absent, and
// the test exists so nobody later "fixes" String() to return empty and breaks
// the bijection the wire depends on.
func TestUnsetPolicyRevisionRendersAWellFormedIdentifier(t *testing.T) {
	t.Parallel()

	var unset controlwire.PolicyRevisionID
	if got := unset.String(); got != policyRevisionAllZero {
		t.Fatalf("unset PolicyRevisionID.String() = %q, want %q", got, policyRevisionAllZero)
	}
	if err := unset.Validate(); !errors.Is(err, core.ErrControlWirePolicyCursor) {
		t.Fatalf("unset PolicyRevisionID.Validate() = %v, want %v", err, core.ErrControlWirePolicyCursor)
	}
	if got, err := controlwire.ParsePolicyRevisionID(unset.String()); !errors.Is(err, core.ErrControlWirePolicyCursor) || got != (controlwire.PolicyRevisionID{}) {
		t.Fatalf("ParsePolicyRevisionID(%q) = (%v, %v), want zero and %v", unset.String(), got, err, core.ErrControlWirePolicyCursor)
	}
}

func mustPolicyRevision(t *testing.T) controlwire.PolicyRevisionID {
	t.Helper()

	revision, err := controlwire.ParsePolicyRevisionID(policyRevisionRealWorld)
	if err != nil {
		t.Fatalf("ParsePolicyRevisionID(%q) error = %v, want nil", policyRevisionRealWorld, err)
	}
	return revision
}

func mustPolicyCursor(t *testing.T) controlwire.PolicyCursor {
	t.Helper()

	cursor := controlwire.PolicyCursor{Revision: mustPolicyRevision(t), Activation: 1}
	if err := cursor.Validate(); err != nil {
		t.Fatalf("PolicyCursor.Validate() error = %v, want nil", err)
	}
	return cursor
}
