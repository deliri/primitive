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
		wantErr             error
		name                string
		start, refresh, end int64
		decision            Decision
	}{
		{name: "nonempty allow interval", start: 100, refresh: 200, end: 300, decision: DecisionAllow, wantErr: nil},
		{name: "explicit refusal still retains exact contact interval", start: 100, refresh: 200, end: 300, decision: DecisionRefuse, wantErr: nil},
		{name: "refresh one below start cannot precede activation", start: 100, refresh: 99, end: 300, decision: DecisionAllow, wantErr: core.ErrAccessPermitContract},
		{name: "refresh exactly at start", start: 100, refresh: 100, end: 300, decision: DecisionAllow, wantErr: nil},
		{name: "refresh one above start", start: 100, refresh: 101, end: 300, decision: DecisionAllow, wantErr: nil},
		{name: "refresh one below end", start: 100, refresh: 299, end: 300, decision: DecisionAllow, wantErr: nil},
		{name: "refresh exactly at end", start: 100, refresh: 300, end: 300, decision: DecisionAllow, wantErr: nil},
		{name: "refresh one above end refuses", start: 100, refresh: 301, end: 300, decision: DecisionAllow, wantErr: core.ErrAccessPermitContract},
		{name: "end one below start refuses", start: 100, refresh: 100, end: 99, decision: DecisionAllow, wantErr: core.ErrAccessPermitContract},
		{name: "end exactly at start refuses empty authority", start: 100, refresh: 100, end: 100, decision: DecisionAllow, wantErr: core.ErrAccessPermitContract},
		{name: "one nanosecond interval remains representable", start: 100, refresh: 100, end: 101, decision: DecisionAllow, wantErr: nil},
		{name: "full signed range never subtracts into overflow", start: math.MinInt64, refresh: 0, end: math.MaxInt64, decision: DecisionAllow, wantErr: nil},
		{name: "minimum timestamp starts one nanosecond interval", start: math.MinInt64, refresh: math.MinInt64, end: math.MinInt64 + 1, decision: DecisionAllow, wantErr: nil},
		{name: "maximum timestamp ends one nanosecond interval", start: math.MaxInt64 - 1, refresh: math.MaxInt64, end: math.MaxInt64, decision: DecisionAllow, wantErr: nil},
		{name: "reverse full range refuses without overflow", start: math.MaxInt64, refresh: 0, end: math.MinInt64, decision: DecisionAllow, wantErr: core.ErrAccessPermitContract},
		{name: "missing decision cannot silently refuse", start: 100, refresh: 200, end: 300, decision: DecisionUnknown, wantErr: core.ErrAccessPermitContract},
		{name: "future decision cannot silently grant", start: 100, refresh: 200, end: 300, decision: Decision(3), wantErr: core.ErrAccessPermitContract},
		{name: "pathological decision cannot silently grant", start: 100, refresh: 200, end: 300, decision: Decision(math.MaxUint8), wantErr: core.ErrAccessPermitContract},
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
	for selector := range uint8(7) {
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
