package receipt

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestReceiptValueConstructorsLayerTriad(t *testing.T) {
	t.Parallel()

	var zero [ReceiptIDBytes]byte
	lowest := zero
	lowest[len(lowest)-1] = 1
	highest := [ReceiptIDBytes]byte{}
	for index := range highest {
		highest[index] = math.MaxUint8
	}
	cases := []struct {
		wantErr error
		name    string
		raw     [ReceiptIDBytes]byte
	}{
		{name: "lowest nonzero identity is admitted", raw: lowest},
		{name: "highest identity is admitted", raw: highest},
		{name: "all-zero identity is rejected", raw: zero, wantErr: core.ErrReceiptContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := NewReceiptID(tc.raw)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewReceiptID() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr == nil && got.Validate() != nil {
				t.Fatalf("NewReceiptID() result Validate() error = %v, want nil", got.Validate())
			}
		})
	}

	for _, value := range []uint64{1, 2, math.MaxUint64} {
		got, err := NewGeneration(value)
		if err != nil {
			t.Fatalf("NewGeneration(%d) error = %v, want nil", value, err)
		}
		projected, err := got.Uint64()
		if err != nil || projected != value {
			t.Fatalf("Generation(%d).Uint64() = (%d, %v), want (%d, nil)", value, projected, err, value)
		}
	}
	if _, err := NewGeneration(0); !errors.Is(err, core.ErrReceiptContract) {
		t.Fatalf("NewGeneration(0) error = %v, want %v", err, core.ErrReceiptContract)
	}
}

func TestReceiptValueJSONHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 90)
	identityJSON, err := json.Marshal(fixture.receipt)
	if err != nil {
		t.Fatalf("json.Marshal(ReceiptID) error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input is rejected", data: nil},
		{name: "null is rejected", data: []byte("null")},
		{name: "truncated quote is rejected", data: identityJSON[:len(identityJSON)-1]},
		{name: "uppercase hex is rejected", data: []byte(`"` + strings.ToUpper(fixture.receipt.String()) + `"`)},
		{name: "one byte short is rejected", data: []byte(`"` + fixture.receipt.String()[1:] + `"`)},
		{name: "one byte long is rejected", data: []byte(`"` + fixture.receipt.String() + `0"`)},
		{name: "non-hex text is rejected", data: []byte(`"` + strings.Repeat("z", ReceiptIDHexBytes) + `"`)},
		{name: "invalid UTF-8 is rejected", data: []byte{'"', 0xff, '"'}},
		{name: "number type is rejected", data: []byte("1")},
		{name: "object type is rejected", data: []byte("{}")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := fixture.receipt
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || receiver != fixture.receipt {
				t.Fatalf("ReceiptID.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and %v",
					tc.data, receiver, gotErr, core.ErrJSONContract)
			}
		})
	}

	generation, err := NewGeneration(math.MaxUint64)
	if err != nil {
		t.Fatalf("NewGeneration(maximum) error = %v, want nil", err)
	}
	encoded, err := json.Marshal(generation)
	if err != nil {
		t.Fatalf("json.Marshal(maximum Generation) error = %v, want nil", err)
	}
	var decoded Generation
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != generation {
		t.Fatalf("json.Unmarshal(maximum Generation) = (%v, %v), want (%v, nil)", decoded, err, generation)
	}
	for _, hostile := range [][]byte{
		nil, []byte("0"), []byte("01"), []byte("-1"), []byte("1.0"),
		[]byte(strconvMaximumPlusOne), []byte(`"1"`), []byte("{}"),
	} {
		receiver := generation
		gotErr := receiver.UnmarshalJSON(hostile)
		if !errors.Is(gotErr, core.ErrJSONContract) || receiver != generation {
			t.Fatalf("Generation.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON rejection",
				hostile, receiver, gotErr)
		}
	}
}

const strconvMaximumPlusOne = "18446744073709551616"

// TestSealedRejectionsCannotBeClaimedWithoutAReason is the forgery ratchet for
// Receipt's two typed rejection identities.
//
// Both identities are sealed interfaces backed only by package-private values,
// so no consumer can build a carrier at all. Inside the package the remaining
// hazard is a reasonless carrier reaching a caller: an invalid internal value
// must carry only Receipt's parent contract identity, never the specialized
// identity whose fact it cannot prove. Every constructor and projection must
// refuse it.
func TestSealedRejectionsCannotBeClaimedWithoutAReason(t *testing.T) {
	t.Parallel()

	t.Run("zero scope mismatch answers no field", func(t *testing.T) {
		t.Parallel()
		forged := scopeMismatch{}
		if errors.Is(forged, core.ErrReceiptScope) ||
			!errors.Is(forged, core.ErrReceiptContract) {
			t.Fatalf("scopeMismatch{} identities = scope:%t contract:%t, want false/true",
				errors.Is(forged, core.ErrReceiptScope),
				errors.Is(forged, core.ErrReceiptContract))
		}
		if got, gotErr := forged.Field(); got != ScopeFieldUnknown ||
			!errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("scopeMismatch{}.Field() = (%v, %v), want unknown and %v",
				got, gotErr, core.ErrReceiptContract)
		}
	})
	t.Run("zero watermark conflict answers no reason", func(t *testing.T) {
		t.Parallel()
		forged := watermarkConflict{}
		if errors.Is(forged, core.ErrReceiptConflict) ||
			!errors.Is(forged, core.ErrReceiptContract) {
			t.Fatalf("watermarkConflict{} identities = conflict:%t contract:%t, want false/true",
				errors.Is(forged, core.ErrReceiptConflict),
				errors.Is(forged, core.ErrReceiptContract))
		}
		if got, gotErr := forged.Reason(); got != ConflictReasonUnknown ||
			!errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("watermarkConflict{}.Reason() = (%v, %v), want unknown and %v",
				got, gotErr, core.ErrReceiptContract)
		}
	})
	t.Run("constructors refuse an unadmitted field", func(t *testing.T) {
		t.Parallel()
		for _, field := range []ScopeField{ScopeFieldUnknown, scopeFieldLimit, ScopeField(math.MaxUint8)} {
			gotErr := newScopeMismatch(field)
			var mismatch ScopeMismatch
			if errors.As(gotErr, &mismatch) || !errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("newScopeMismatch(%v) = %v, want a contract rejection carrying no scope identity",
					field, gotErr)
			}
			if errors.Is(gotErr, core.ErrReceiptScope) {
				t.Fatalf("newScopeMismatch(%v) carries the scope identity without a field", field)
			}
		}
	})
	t.Run("constructors refuse an unadmitted reason", func(t *testing.T) {
		t.Parallel()
		for _, reason := range []ConflictReason{ConflictReasonUnknown, conflictReasonLimit, ConflictReason(math.MaxUint8)} {
			gotErr := conflictError(reason)
			var conflict WatermarkConflict
			if errors.As(gotErr, &conflict) || !errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("conflictError(%v) = %v, want a contract rejection carrying no conflict identity",
					reason, gotErr)
			}
			if errors.Is(gotErr, core.ErrReceiptConflict) {
				t.Fatalf("conflictError(%v) carries the conflict identity without a reason", reason)
			}
		}
	})
	t.Run("authentic rejections stay distinguishable", func(t *testing.T) {
		t.Parallel()
		mismatch := newScopeMismatch(ScopeFieldExtent)
		conflict := conflictError(ConflictReasonCursorUnchanged)
		if errors.Is(mismatch, core.ErrReceiptConflict) ||
			errors.Is(conflict, core.ErrReceiptScope) {
			t.Fatalf("scope and conflict identities overlap: %v, %v", mismatch, conflict)
		}
		if !errors.Is(mismatch, core.ErrReceiptContract) ||
			!errors.Is(conflict, core.ErrReceiptContract) {
			t.Fatalf("rejections lost the Receipt parent identity: %v, %v", mismatch, conflict)
		}
		if errors.Is(rollbackError(), core.ErrReceiptConflict) {
			t.Fatalf("rollbackError() = %v, want no conflict identity", rollbackError())
		}
	})
}

func TestReceiptNilReceiverAndZeroMarshalMatrix(t *testing.T) {
	t.Parallel()

	nilCalls := []struct {
		call func() error
		name string
	}{
		{name: "revision", call: func() error { return (*Revision)(nil).UnmarshalJSON([]byte(`"v1"`)) }},
		{name: "receipt identity", call: func() error { return (*ReceiptID)(nil).UnmarshalJSON([]byte(`"00"`)) }},
		{name: "generation", call: func() error { return (*Generation)(nil).UnmarshalJSON([]byte("1")) }},
		{name: "evidence body", call: func() error { return (*EvidenceBody)(nil).UnmarshalJSON([]byte("{}")) }},
		{name: "header", call: func() error { return (*Header)(nil).UnmarshalJSON([]byte("{}")) }},
		{name: "payload", call: func() error { return (*EvidencePayload)(nil).UnmarshalJSON([]byte("{}")) }},
		{name: "document", call: func() error { return (*EvidenceDocument)(nil).UnmarshalJSON([]byte("{}")) }},
		{name: "cursor digest", call: func() error { return (*CursorDigest)(nil).UnmarshalJSON([]byte(`""`)) }},
		{name: "chain hash", call: func() error { return (*ChainHash)(nil).UnmarshalJSON([]byte(`""`)) }},
	}
	for _, tc := range nilCalls {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if gotErr := tc.call(); !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("nil receiver error = %v, want JSON and Receipt identities", gotErr)
			}
		})
	}

	zeroValues := []struct {
		value core.ValidatedJSONMarshaler
		name  string
	}{
		{name: "revision", value: RevisionUnknown},
		{name: "receipt identity", value: ReceiptID{}},
		{name: "generation", value: Generation{}},
		{name: "evidence body", value: EvidenceBody{}},
		{name: "header", value: Header{}},
		{name: "payload", value: EvidencePayload{}},
		{name: "document", value: EvidenceDocument{}},
		{name: "cursor digest", value: CursorDigest{}},
		{name: "chain hash", value: ChainHash{}},
	}
	for _, tc := range zeroValues {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, gotErr := tc.value.MarshalJSON(); !errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("zero MarshalJSON() error = %v, want %v", gotErr, core.ErrReceiptContract)
			}
		})
	}
}

func TestReceiptDomainCanonicalContract(t *testing.T) {
	t.Parallel()

	text, err := DomainEvidenceV1.MarshalText()
	if err != nil || string(text) != evidenceDomainToken {
		t.Fatalf("DomainEvidenceV1.MarshalText() = (%q, %v), want (%q, nil)", text, err, evidenceDomainToken)
	}
	parsed, err := DomainEvidenceV1.ParseCanonicalText(text)
	if err != nil || parsed != DomainEvidenceV1 {
		t.Fatalf("Domain.ParseCanonicalText() = (%v, %v), want (%v, nil)", parsed, err, DomainEvidenceV1)
	}
	for _, hostile := range [][]byte{
		nil,
		text[:len(text)-1],
		append(append([]byte{}, text...), 0),
		bytes.ToUpper(text),
		[]byte{0xff},
	} {
		got, gotErr := DomainEvidenceV1.ParseCanonicalText(hostile)
		if got != DomainUnknown || !errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("Domain.ParseCanonicalText(%q) = (%v, %v), want unknown and Receipt rejection",
				hostile, got, gotErr)
		}
	}
	if _, gotErr := DomainUnknown.MarshalText(); !errors.Is(gotErr, core.ErrReceiptContract) {
		t.Fatalf("DomainUnknown.MarshalText() error = %v, want %v", gotErr, core.ErrReceiptContract)
	}
}

func TestReceiptZeroAccessorsAndInternalJSONBoundsFailClosed(t *testing.T) {
	t.Parallel()

	if got := (ReceiptID{}).String(); got != "" {
		t.Fatalf("ReceiptID{}.String() = %q, want empty", got)
	}
	if got, gotErr := (Generation{}).Uint64(); got != 0 ||
		!errors.Is(gotErr, core.ErrReceiptContract) {
		t.Fatalf("Generation{}.Uint64() = (%d, %v), want (0, %v)",
			got, gotErr, core.ErrReceiptContract)
	}
	mismatch := scopeMismatch{field: ScopeFieldObject}
	if !errors.Is(mismatch, core.ErrReceiptScope) {
		t.Fatalf("errors.Is(scopeMismatch, ErrReceiptScope) = false, want true")
	}
	conflict := watermarkConflict{reason: ConflictReasonScope}
	if !errors.Is(conflict, core.ErrReceiptConflict) {
		t.Fatalf("errors.Is(watermarkConflict, ErrReceiptConflict) = false, want true")
	}
	for _, contract := range []jsonStructureContract{
		{maximumBytes: -1, depth: 1, fields: 1},
		{maximumBytes: 1, depth: 0, fields: 1},
		{maximumBytes: 1, depth: 1, fields: 0},
	} {
		if _, gotErr := contract.limits(); !errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("jsonStructureContract%+v.limits() error = %v, want %v",
				contract, gotErr, core.ErrReceiptContract)
		}
	}
}
