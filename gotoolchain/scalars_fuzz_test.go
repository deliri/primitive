package gotoolchain

import (
	"errors"
	"go/version"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This ratchets the external scalar doors. Accepted toolchain spellings must
// also be real Go versions, independently of the nominal value's validator.
func FuzzCompilerScalarSemanticClosure(f *testing.F) {
	toolchain, err := ParseToolchainVersion("go1.27.1")
	if err != nil {
		f.Fatal(err)
	}
	name, err := ParsePackageName("compilerfixture")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(toolchain.String(), true)
	f.Add(name.String(), false)
	f.Add("go1.27.1.1", true)
	f.Add("go1.027.1", true)
	f.Add("go1.27.01", true)
	f.Add(strings.Repeat("1", toolchainVersionMaximumBytes+1), true)
	f.Add("", true)
	f.Add("_", false)
	f.Add("package", false)
	f.Fuzz(func(t *testing.T, value string, isVersion bool) {
		if isVersion {
			got, err := ParseToolchainVersion(value)
			if err != nil {
				if !errors.Is(err, core.ErrGoToolchainContract) || got != (ToolchainVersion{}) {
					t.Fatalf("ParseToolchainVersion(refused) = (%v,%v), want zero and typed refusal", got, err)
				}
				return
			}
			if !version.IsValid(value) || got.Validate() != nil || got.String() != value {
				t.Fatalf("ParseToolchainVersion(%q) = %v, want an exact valid Go version", value, got)
			}
			roundTrip, err := ParseToolchainVersion(got.String())
			if err != nil || roundTrip != got {
				t.Fatalf("version round trip = (%v,%v), want (%v,nil)", roundTrip, err, got)
			}
			return
		}
		got, err := ParsePackageName(value)
		if err != nil {
			if !errors.Is(err, core.ErrGoToolchainContract) || got != (PackageName{}) {
				t.Fatalf("ParsePackageName(refused) = (%v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		roundTrip, err := ParsePackageName(got.String())
		if got.Validate() != nil || got.String() != value || err != nil || roundTrip != got {
			t.Fatalf("package name closure = (%v,%v,%v), want exact %q and nil", got, roundTrip, err, value)
		}
	})
}
