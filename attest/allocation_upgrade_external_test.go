//go:build !race

// Allocation budgets describe ordinary builds; race instrumentation changes allocations.
package attest_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestEnvelopeProjectionAllocationRatchetTable(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	cases := []struct {
		name        string
		decode      bool
		wantMaximum float64
	}{
		{name: "encoding shares owned envelope storage", wantMaximum: 27},
		{name: "decoding uses fixed signature storage", decode: true, wantMaximum: 47},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			envelope := mustEnvelope(t, builtBody{commit: "benchmark-json", count: 1}, deterministicPrivateKey(t, "benchmark-json"))
			canonical, err := envelope.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var got []byte
			var decoded attest.Envelope[testDomain]
			allocations := testing.AllocsPerRun(100, func() {
				if tc.decode {
					err = decoded.UnmarshalJSON(canonical)
				} else {
					got, err = envelope.MarshalJSON()
				}
			})
			if err != nil || allocations > tc.wantMaximum {
				t.Fatalf("projection = %g allocations, %v; want at most %g and no error", allocations, err, tc.wantMaximum)
			}
			if tc.decode {
				if decoded != envelope {
					t.Fatalf("decoded envelope = %+v; want %+v", decoded, envelope)
				}
			} else if !bytes.Equal(got, canonical) {
				t.Fatalf("encoded envelope = %q; want %q", got, canonical)
			}
		})
	}
}
