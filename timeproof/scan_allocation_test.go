package timeproof

import (
	"bytes"
	"encoding/asn1"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestDERScanAllocationsDoNotScaleWithElementCount(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	cases := []struct {
		name  string
		build func(testing.TB, int) []byte
		run   func([]byte) (int, error)
	}{
		{
			name:  "digest declarations reuse one raw decode destination",
			build: func(t testing.TB, count int) []byte { return digestSetFixture(t, count, 1) },
			run: func(data []byte) (int, error) {
				got, rest, err := consumeAlgorithmSet(data)
				return len(got.FullBytes) - len(rest), err
			},
		},
		{
			name: "signed attributes reuse one raw decode destination",
			build: func(t testing.TB, count int) []byte {
				t.Helper()
				attribute := cmsAttribute{Type: oidContentType(), Values: []asn1.RawValue{rawValueFromDER(t, encodeSequence())}}
				raw := encodedSignedAttributes(t, []cmsAttribute{attribute})
				return bytes.Repeat(raw.Bytes, count)
			},
			run: func(data []byte) (int, error) {
				got, err := parseSignedAttributes(asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: data})
				return len(got.Bytes), err
			},
		},
		{
			name: "status texts reuse one raw decode destination",
			build: func(t testing.TB, count int) []byte {
				t.Helper()
				encoded, err := asn1.MarshalWithParams("fixed text", "utf8")
				if err != nil {
					t.Fatalf("asn1.MarshalWithParams(status) error = %v, want nil", err)
				}
				return bytes.Repeat(encoded, count)
			},
			run: func(data []byte) (int, error) { return len(data), validateStatusText(data) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			var allocations [2]float64
			for i, count := range []int{4, 4096} {
				input := tc.build(t, count)
				if len(input) == 0 {
					t.Fatal("scan workload bytes = 0, want a nonempty encoded collection")
				}
				allocations[i] = testing.AllocsPerRun(20, func() {
					got, err := tc.run(input)
					if err != nil || got != len(input) {
						t.Fatalf("scan consumed bytes/error = (%d, %v), want (%d, nil)", got, err, len(input))
					}
				})
			}
			if got, want := allocations[1], allocations[0]; got != want {
				t.Fatalf("4096-element allocations = %g, want same fixed storage as 4-element scan (%g)", got, want)
			}
		})
	}
}
