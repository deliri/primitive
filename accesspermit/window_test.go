package accesspermit

import (
	"encoding/hex"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestWindowSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		start, refresh, end int64
		decision            Decision
		wantErr             error
	}{
		{"nonempty allow interval", 100, 200, 300, DecisionAllow, nil},
		{"explicit refusal still retains exact contact interval", 100, 200, 300, DecisionRefuse, nil},
		{"refresh one below start cannot precede activation", 100, 99, 300, DecisionAllow, core.ErrAccessPermitContract},
		{"refresh exactly at start", 100, 100, 300, DecisionAllow, nil},
		{"refresh one above start", 100, 101, 300, DecisionAllow, nil},
		{"refresh one below end", 100, 299, 300, DecisionAllow, nil},
		{"refresh exactly at end", 100, 300, 300, DecisionAllow, nil},
		{"refresh one above end refuses", 100, 301, 300, DecisionAllow, core.ErrAccessPermitContract},
		{"end one below start refuses", 100, 100, 99, DecisionAllow, core.ErrAccessPermitContract},
		{"end exactly at start refuses empty authority", 100, 100, 100, DecisionAllow, core.ErrAccessPermitContract},
		{"one nanosecond interval remains representable", 100, 100, 101, DecisionAllow, nil},
		{"full signed range never subtracts into overflow", math.MinInt64, 0, math.MaxInt64, DecisionAllow, nil},
		{"minimum timestamp starts one nanosecond interval", math.MinInt64, math.MinInt64, math.MinInt64 + 1, DecisionAllow, nil},
		{"maximum timestamp ends one nanosecond interval", math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64, DecisionAllow, nil},
		{"reverse full range refuses without overflow", math.MaxInt64, 0, math.MinInt64, DecisionAllow, core.ErrAccessPermitContract},
		{"missing decision cannot silently refuse", 100, 200, 300, DecisionUnknown, core.ErrAccessPermitContract},
		{"future decision cannot silently grant", 100, 200, 300, Decision(3), core.ErrAccessPermitContract},
		{"pathological decision cannot silently grant", 100, 200, 300, Decision(math.MaxUint8), core.ErrAccessPermitContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := Window{NotBefore: temporal.InstantFromNanoseconds(tc.start), RefreshAfter: temporal.InstantFromNanoseconds(tc.refresh), NotAfter: temporal.InstantFromNanoseconds(tc.end), Decision: tc.decision}
			if got := in.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("Window(%+v).Validate() = %v, want %v", in, got, tc.wantErr)
			}
		})
	}
	if err := (Window{}).Validate(); !errors.Is(err, core.ErrAccessPermitContract) {
		t.Fatalf("zero Window.Validate() = %v, want contract refusal", err)
	}
}

func FuzzPermitSignedFactMutations(f *testing.F) {
	seed, keys, _ := permitFixture(f)
	for selector := uint8(0); selector < 7; selector++ {
		f.Add(selector, int64(201))
	}
	f.Fuzz(func(t *testing.T, selector uint8, value int64) {
		got := seed
		switch selector % 7 {
		case 0:
			got.Terms.Window.NotBefore = temporal.InstantFromNanoseconds(value)
		case 1:
			got.Terms.Window.RefreshAfter = temporal.InstantFromNanoseconds(value)
		case 2:
			got.Terms.Window.NotAfter = temporal.InstantFromNanoseconds(value)
		case 3:
			got.Terms.Window.Decision = DecisionRefuse
		case 4:
			got.Terms.Revision = uint16(value)
		case 5:
			got.Terms.Binding.Subject.Offering = core.Offering{Token: "another-instrument"}
		case 6:
			raw, err := got.Signature.Signature.Bytes()
			if err != nil {
				t.Fatalf("Signature.Bytes() error = %v, want nil", err)
			}
			raw[0] ^= 1
			encoded, err := core.MarshalCanonicalJSONString(hex.EncodeToString(raw[:]))
			if err != nil {
				t.Fatalf("mutation encoding error = %v, want nil", err)
			}
			if err := got.Signature.Signature.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("signature mutation decode error = %v, want nil", err)
			}
		}
		proof, err := Verify(got, seed.Terms.Binding, keys)
		if got == seed {
			if err != nil || proof.Validate() != nil {
				t.Fatalf("unchanged seed = (%+v,%v), want genuine proof/nil", proof, err)
			}
			return
		}
		if err == nil || proof != (Verified{}) {
			t.Fatalf("load-bearing signed mutation = (%+v,%v), want zero/refusal", proof, err)
		}
		if !errors.Is(err, core.ErrAccessPermitContract) && !errors.Is(err, core.ErrAccessPermitDenied) && !errors.Is(err, core.ErrAccessPermitBinding) {
			t.Fatalf("mutation refusal = %v, want typed permit identity", err)
		}
	})
}
