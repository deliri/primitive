package lease_test

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func TestIdentifierEntryPointPressure(t *testing.T) {
	t.Parallel()

	valid := fixtureIdentifierBytes(1)
	oneBit := [lease.IdentifierBytes]byte{1}
	maximum := [lease.IdentifierBytes]byte{}
	for index := range maximum {
		maximum[index] = math.MaxUint8
	}
	cases := []struct {
		wantErr error
		name    string
		value   [lease.IdentifierBytes]byte
	}{
		{name: "ascending bytes", value: valid},
		{name: "single low bit", value: oneBit},
		{name: "all maximum bytes", value: maximum},
		{name: "all zero is unset", wantErr: core.ErrLeaseContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entitlement, entitlementErr := lease.NewEntitlementID(tc.value)
			device, deviceErr := lease.NewDeviceID(tc.value)
			results := []struct {
				err  error
				name string
			}{
				{name: "entitlement", err: entitlementErr},
				{name: "device", err: deviceErr},
			}
			for _, result := range results {
				if !errors.Is(result.err, tc.wantErr) {
					t.Fatalf("%s constructor error = %v, want %v", result.name, result.err, tc.wantErr)
				}
			}
			if tc.wantErr != nil {
				return
			}
			if entitlement.String() != device.String() {
				t.Fatalf("nominal identifier projections differ for equal bytes")
			}
			parsedEntitlement, err := lease.ParseEntitlementID(entitlement.String())
			if err != nil || parsedEntitlement != entitlement {
				t.Fatalf("lease.ParseEntitlementID() = (%v, %v), want (%v, nil)", parsedEntitlement, err, entitlement)
			}
			parsedDevice, err := lease.ParseDeviceID(device.String())
			if err != nil || parsedDevice != device {
				t.Fatalf("lease.ParseDeviceID() = (%v, %v), want (%v, nil)", parsedDevice, err, device)
			}
		})
	}
}

func TestIdentifierJSONHostilePressure(t *testing.T) {
	t.Parallel()

	entitlement, err := lease.NewEntitlementID(fixtureIdentifierBytes(111))
	if err != nil {
		t.Fatalf("lease.NewEntitlementID() error = %v, want nil", err)
	}
	canonical, err := json.Marshal(entitlement)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	uppercase := append([]byte(nil), canonical...)
	for index := 1; index < len(uppercase)-1; index++ {
		if uppercase[index] >= 'a' && uppercase[index] <= 'f' {
			uppercase[index] -= 'a' - 'A'
		}
	}
	cases := []struct {
		wantErr error
		name    string
		data    []byte
	}{
		{name: "canonical", data: canonical},
		{name: "whitespace equivalent", data: append(append([]byte(" \n"), canonical...), '\t')},
		{name: "empty", data: nil, wantErr: core.ErrJSONContract},
		{name: "null", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "unquoted", data: canonical[1 : len(canonical)-1], wantErr: core.ErrJSONContract},
		{name: "uppercase", data: uppercase, wantErr: core.ErrLeaseContract},
		{name: "short", data: canonical[:len(canonical)-2], wantErr: core.ErrLeaseContract},
		{name: "long", data: append(append([]byte(nil), canonical[:len(canonical)-1]...), '0', '"'), wantErr: core.ErrLeaseContract},
		{name: "zero", data: []byte(`"00000000000000000000000000000000"`), wantErr: core.ErrLeaseContract},
		{name: "separator", data: []byte(`"0102030405060708-90a0b0c0d0e0f10"`), wantErr: core.ErrLeaseContract},
		{name: "invalid utf8", data: []byte{'"', 0xff, '"'}, wantErr: core.ErrJSONContract},
		{name: "unpaired surrogate", data: []byte(`"\ud800000000000000000000000000000"`), wantErr: core.ErrJSONContract},
		{name: "oversized", data: make([]byte, lease.IdentifierJSONMaximumBytes+1), wantErr: core.ErrJSONContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := entitlement
			err := got.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("json.Unmarshal() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got != entitlement {
				t.Fatalf("rejected EntitlementID mutated receiver")
			}
		})
	}
}

func TestGenerationBoundaryPressure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   uint64
	}{
		{name: "one", value: 1},
		{name: "two", value: 2},
		{name: "maximum", value: math.MaxUint64},
		{name: "zero", wantErr: core.ErrLeaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := lease.NewGeneration(tc.value)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lease.NewGeneration() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil {
				projected, projectedErr := got.Uint64()
				if projectedErr != nil || projected != tc.value {
					t.Fatalf("Generation.Uint64() = (%d, %v), want (%d, nil)", projected, projectedErr, tc.value)
				}
			}
		})
	}
}

func TestGenerationJSONEntryPointPressure(t *testing.T) {
	t.Parallel()

	maximum := `"18446744073709551615"`
	cases := []struct {
		wantErr error
		name    string
		data    []byte
		want    uint64
	}{
		{name: "one", data: []byte(`"1"`), want: 1},
		{name: "maximum", data: []byte(maximum), want: math.MaxUint64},
		{name: "whitespace equivalent", data: []byte(" \n\"2\"\t"), want: 2},
		{name: "empty", wantErr: core.ErrJSONContract},
		{name: "null", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "unquoted", data: []byte("1"), wantErr: core.ErrJSONContract},
		{name: "zero", data: []byte(`"0"`), wantErr: core.ErrLeaseContract},
		{name: "leading zero", data: []byte(`"01"`), wantErr: core.ErrLeaseContract},
		{name: "negative", data: []byte(`"-1"`), wantErr: core.ErrLeaseContract},
		{name: "fraction", data: []byte(`"1.0"`), wantErr: core.ErrLeaseContract},
		{name: "one above maximum", data: []byte(`"18446744073709551616"`), wantErr: core.ErrLeaseContract},
		{name: "over digit bound", data: []byte(`"100000000000000000000"`), wantErr: core.ErrLeaseContract},
		{name: "unpaired surrogate", data: []byte(`"\ud800"`), wantErr: core.ErrJSONContract},
		{name: "over document bound", data: make([]byte, lease.GenerationJSONMaximumBytes+1), wantErr: core.ErrJSONContract},
	}
	seed, err := lease.NewGeneration(7)
	if err != nil {
		t.Fatalf("lease.NewGeneration() error = %v, want nil", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := seed
			err := got.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Generation.UnmarshalJSON() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != seed {
					t.Fatalf("rejected Generation mutated receiver")
				}
				return
			}
			value, valueErr := got.Uint64()
			if valueErr != nil || value != tc.want {
				t.Fatalf("Generation.Uint64() = (%d, %v), want (%d, nil)", value, valueErr, tc.want)
			}
			encoded, marshalErr := json.Marshal(got)
			wantJSON := `"` + got.String() + `"`
			if marshalErr != nil || string(encoded) != wantJSON {
				t.Fatalf("json.Marshal() = (%q, %v), want (%q, nil)", encoded, marshalErr, wantJSON)
			}
		})
	}
}

func TestWireEnumDomainsExhaustive(t *testing.T) {
	t.Parallel()

	checkRevisionDomain(t)
	checkOutcomeDomain(t)
	checkRevocationReasonDomain(t)
}

type enumJSONPressure[T comparable] struct {
	seed        T
	want        T
	decode      func(*T, []byte) error
	name        string
	valid       []byte
	unsupported []byte
	maximum     int
}

func TestWireEnumJSONEntryPointPressure(t *testing.T) {
	t.Parallel()

	runEnumJSONPressure(t, enumJSONPressure[lease.Revision]{
		name: "revision", seed: lease.RevisionV1, want: lease.RevisionV1,
		valid: []byte(`"v1"`), unsupported: []byte(`"V1"`),
		maximum: lease.RevisionJSONMaximumBytes,
		decode: func(value *lease.Revision, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
	runEnumJSONPressure(t, enumJSONPressure[lease.Outcome]{
		name: "outcome", seed: lease.OutcomeGrant, want: lease.OutcomeRevocation,
		valid: []byte(`"revocation"`), unsupported: []byte(`"Grant"`),
		maximum: lease.OutcomeJSONMaximumBytes,
		decode: func(value *lease.Outcome, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
	runEnumJSONPressure(t, enumJSONPressure[lease.RevocationReason]{
		name: "revocation reason", seed: lease.RevocationReasonLicenceBreach,
		want:  lease.RevocationReasonSecurityOrPlatformRisk,
		valid: []byte(`"security-or-platform-risk"`), unsupported: []byte(`"nonpayment"`),
		maximum: lease.RevocationReasonJSONMaximumBytes,
		decode: func(value *lease.RevocationReason, data []byte) error {
			return value.UnmarshalJSON(data)
		},
	})
}

func runEnumJSONPressure[T comparable](t *testing.T, contract enumJSONPressure[T]) {
	t.Helper()

	cases := []struct {
		wantErr error
		want    T
		name    string
		data    []byte
	}{
		{name: "canonical", data: contract.valid, want: contract.want},
		{name: "whitespace equivalent", data: append(append([]byte(" \n"), contract.valid...), '\t'), want: contract.want},
		{name: "empty", wantErr: core.ErrJSONContract},
		{name: "null", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "unquoted", data: contract.valid[1 : len(contract.valid)-1], wantErr: core.ErrJSONContract},
		{name: "unsupported exact token", data: contract.unsupported, wantErr: core.ErrLeaseContract},
		{name: "unpaired surrogate", data: []byte(`"\ud800"`), wantErr: core.ErrJSONContract},
		{name: "over document bound", data: make([]byte, contract.maximum+1), wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(contract.name+"/"+tc.name, func(t *testing.T) {
			t.Parallel()

			got := contract.seed
			err := contract.decode(&got, tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("enum UnmarshalJSON() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && got != contract.seed {
				t.Fatalf("rejected enum mutated receiver")
			}
			if tc.wantErr == nil && got != tc.want {
				t.Fatalf("decoded enum = %v, want %v", got, tc.want)
			}
		})
	}
}

func checkRevisionDomain(t *testing.T) {
	t.Helper()

	for value := uint16(0); value <= math.MaxUint8; value++ {
		revision := lease.Revision(value)
		want := value == uint16(lease.RevisionV1)
		if revision.IsValid() != want {
			t.Errorf("Revision(%d).IsValid() = %t, want %t", value, revision.IsValid(), want)
		}
	}
	parsed, err := lease.ParseRevision(lease.RevisionV1.String())
	if err != nil || parsed != lease.RevisionV1 {
		t.Errorf("lease.ParseRevision() = (%v, %v), want (%v, nil)", parsed, err, lease.RevisionV1)
	}
}

func checkOutcomeDomain(t *testing.T) {
	t.Helper()

	valid := []lease.Outcome{
		lease.OutcomeGrant,
		lease.OutcomeRefusal,
		lease.OutcomeRevocation,
	}
	for value := uint16(0); value <= math.MaxUint8; value++ {
		outcome := lease.Outcome(value)
		want := slices.Contains(valid, outcome)
		if outcome.IsValid() != want {
			t.Errorf("Outcome(%d).IsValid() = %t, want %t", value, outcome.IsValid(), want)
		}
	}
	for _, value := range valid {
		parsed, err := lease.ParseOutcome(value.String())
		if err != nil || parsed != value {
			t.Errorf("lease.ParseOutcome(%q) = (%v, %v), want (%v, nil)", value.String(), parsed, err, value)
		}
	}
}

func checkRevocationReasonDomain(t *testing.T) {
	t.Helper()

	revocationReasons := []lease.RevocationReason{
		lease.RevocationReasonLicenceBreach,
		lease.RevocationReasonUnlawfulOrAbusiveUse,
		lease.RevocationReasonSecurityOrPlatformRisk,
		lease.RevocationReasonInsolvency,
	}
	for _, value := range revocationReasons {
		if err := value.Validate(); err != nil {
			t.Errorf("RevocationReason(%d).Validate() error = %v, want nil", value, err)
		}
		parsed, err := lease.ParseRevocationReason(value.String())
		if err != nil || parsed != value {
			t.Errorf("lease.ParseRevocationReason(%q) = (%v, %v), want (%v, nil)", value.String(), parsed, err, value)
		}
	}
	for value := uint16(0); value <= math.MaxUint8; value++ {
		revocation := lease.RevocationReason(value)
		want := slices.Contains(revocationReasons, revocation)
		if revocation.IsValid() != want {
			t.Errorf("RevocationReason(%d).IsValid() = %t, want %t", value, revocation.IsValid(), want)
		}
	}
}
