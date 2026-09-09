package distribution

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
	"strconv"
	"testing"
)

func TestRequestCommitmentDomainLayerTriad(t *testing.T) {
	t.Parallel()
	digest := core.SHA256Of([]byte("commitment domain separation"))
	for raw := range 256 {
		domain := SigningDomain(raw)
		t.Run("domain "+strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			wantErr := error(nil)
			want := RequestCommitment{domain: domain, digest: digest}
			switch domain {
			case SigningDomainPublicationRequestV1, SigningDomainUpdateRequestV1, SigningDomainUpgradeRequestV1:
			default:
				wantErr = core.ErrDistributionContract
				want = RequestCommitment{}
			}
			got, err := newRequestCommitment(domain, digest)
			if !errors.Is(err, wantErr) || got != want {
				t.Fatalf("newRequestCommitment(%d)=(%v,%v), want (%v,%v)", raw, got, err, want, wantErr)
			}
		})
	}
}

func TestSigningDomainJSONRefusalAllocationDoesNotScaleWithInput(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	cases := []struct {
		name string
		size int
	}{{"one kilobyte", 1 << 10}, {"one mebibyte", 1 << 20}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			wire := bytes.Repeat([]byte{'x'}, tc.size)
			wire[0], wire[len(wire)-1] = '"', '"'
			// TotalAlloc is measured by the harness benchmark; this ratchets the fixed
			// error allocation count, with no decoded token or canonical copy.
			got := testing.AllocsPerRun(100, func() {
				v := SigningDomainUpdateRequestV1
				err := v.UnmarshalJSON(wire)
				if !errors.Is(err, core.ErrJSONContract) || v != SigningDomainUpdateRequestV1 {
					t.Fatalf("UnmarshalJSON()=(%v,%v), want preserved typed refusal", v, err)
				}
			})
			if got > 6 {
				t.Fatalf("oversized JSON allocations=%v, want <=6", got)
			}
		})
	}
}

func TestSigningDomainDirectJSONCanonicalLayerTriad(t *testing.T) {
	t.Parallel()
	for domain := SigningDomainUnknown + 1; domain < signingDomainLimit; domain++ {
		t.Run(domain.String(), func(t *testing.T) {
			t.Parallel()
			canonical, err := domain.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON()=%v, want nil", err)
			}
			escaped := append([]byte{'"', '\\', 'u', '0', '0', '7', '0'}, canonical[2:]...)
			cases := []struct {
				name    string
				wire    []byte
				wantErr error
			}{
				{"exact token", canonical, nil},
				{"leading whitespace", append([]byte{' '}, canonical...), core.ErrJSONContract},
				{"trailing whitespace", append(bytes.Clone(canonical), ' '), core.ErrJSONContract},
				{"escaped equivalent ASCII", escaped, core.ErrJSONContract},
				{"null is absence", []byte("null"), core.ErrJSONContract},
				{"trailing document", append(bytes.Clone(canonical), []byte(" false")...), core.ErrJSONContract},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					got := SigningDomainUnknown
					err := got.UnmarshalJSON(tc.wire)
					want := domain
					if tc.wantErr != nil {
						want = SigningDomainUnknown
					}
					if !errors.Is(err, tc.wantErr) || got != want {
						t.Fatalf("UnmarshalJSON()=(%v,%v), want (%v,%v)", got, err, want, tc.wantErr)
					}
					var token string
					if tc.wantErr == nil {
						if err := json.Unmarshal(tc.wire, &token); err != nil || token != domain.String() {
							t.Fatalf("stdlib token=(%q,%v), want %q", token, err, domain.String())
						}
					}
				})
			}
		})
	}
}
