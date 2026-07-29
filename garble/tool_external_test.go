package garble_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"go/version"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

func TestCurrentToolCarriesExactReviewedProvenanceAndCompatibility(t *testing.T) {
	t.Parallel()

	tool := garble.CurrentTool()
	if gotErr := tool.Validate(); gotErr != nil {
		t.Fatalf("CurrentTool().Validate() error = %v, want nil", gotErr)
	}
	cases := []struct {
		project func(garble.ToolIdentity) (string, error)
		name    string
		want    string
	}{
		{name: "module path is exact", project: garble.ToolIdentity.ModulePath, want: "mvdan.cc/garble"},
		{name: "module version is exact", project: garble.ToolIdentity.Version, want: "v0.16.1-0.20260621195108-ffa2daf72f03"},
		{name: "source revision is exact", project: garble.ToolIdentity.Revision, want: "ffa2daf72f036d7ff72f6a3c8243997f06fa7b4e"},
		{name: "module checksum is exact", project: garble.ToolIdentity.ModuleSum, want: "h1:3/JEpDf12w/71XWzIrnLazgTQD6UWElzrRQWo4oJ7s0="},
		{name: "minimum Go version is exact", project: garble.ToolIdentity.MinimumGoVersion, want: "go1.26.0"},
		{name: "first unsupported Go version is exact", project: garble.ToolIdentity.UnsupportedGoVersion, want: "go1.27"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := tc.project(tool)
			if gotErr != nil || got != tc.want {
				t.Fatalf("ToolIdentity projection = (%q, %v), want (%q, nil)", got, gotErr, tc.want)
			}
		})
	}

	moduleVersion, _ := tool.Version()
	revision, _ := tool.Revision()
	moduleSum, _ := tool.ModuleSum()
	minimumGo, _ := tool.MinimumGoVersion()
	unsupportedGo, _ := tool.UnsupportedGoVersion()
	if !strings.HasSuffix(moduleVersion, revision[:12]) {
		t.Fatalf("tool version %q does not bind revision prefix %q", moduleVersion, revision[:12])
	}
	sum, found := strings.CutPrefix(moduleSum, "h1:")
	decodedSum, gotDecodeErr := base64.StdEncoding.DecodeString(sum)
	if !found || gotDecodeErr != nil || len(decodedSum) != sha256.Size {
		t.Fatalf("tool module sum = (%q, prefix:%t, decode:%v, bytes:%d), want canonical h1 SHA-256", moduleSum, found, gotDecodeErr, len(decodedSum))
	}
	if !version.IsValid(minimumGo) ||
		!version.IsValid(unsupportedGo) ||
		version.Compare(minimumGo, unsupportedGo) >= 0 {
		t.Fatalf("tool Go compatibility interval = [%q, %q), want valid increasing interval", minimumGo, unsupportedGo)
	}
}

func TestToolAndDerivationEnumsRejectUnknownAndFutureValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func() error
		name string
	}{
		{name: "unknown tool rejected", run: func() error { return garble.ToolIdentityUnknown.Validate() }},
		{name: "first future tool rejected", run: func() error { return (garble.ToolIdentityPrimitive2026 + 1).Validate() }},
		{name: "future tool rejected", run: func() error { return garble.ToolIdentity(math.MaxUint8).Validate() }},
		{name: "unknown generation rejected", run: func() error { return garble.DerivationGenerationUnknown.Validate() }},
		{name: "first future generation rejected", run: func() error { return (garble.DerivationGenerationOne + 1).Validate() }},
		{name: "future generation rejected", run: func() error { return garble.DerivationGeneration(math.MaxUint8).Validate() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run()
			if !errors.Is(gotErr, core.ErrGarbleContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("enum Validate() error = %v, want %v and %v", gotErr, core.ErrGarbleContract, core.ErrPrimitiveContract)
			}
		})
	}

	if got := garble.CurrentDerivationGeneration(); got != garble.DerivationGenerationOne {
		t.Fatalf("CurrentDerivationGeneration() = %v, want %v", got, garble.DerivationGenerationOne)
	}
}

func TestInvalidToolIdentityRejectsEveryProjection(t *testing.T) {
	t.Parallel()

	identities := []struct {
		name string
		tool garble.ToolIdentity
	}{
		{name: "unknown tool", tool: garble.ToolIdentityUnknown},
		{name: "first future tool", tool: garble.ToolIdentityPrimitive2026 + 1},
		{name: "future tool", tool: garble.ToolIdentity(math.MaxUint8)},
	}
	projections := []struct {
		project func(garble.ToolIdentity) (string, error)
		name    string
	}{
		{name: "module path", project: garble.ToolIdentity.ModulePath},
		{name: "version", project: garble.ToolIdentity.Version},
		{name: "revision", project: garble.ToolIdentity.Revision},
		{name: "module sum", project: garble.ToolIdentity.ModuleSum},
		{name: "minimum Go", project: garble.ToolIdentity.MinimumGoVersion},
		{name: "unsupported Go", project: garble.ToolIdentity.UnsupportedGoVersion},
	}
	for _, identity := range identities {
		t.Run(identity.name, func(t *testing.T) {
			t.Parallel()

			for _, projection := range projections {
				got, gotErr := projection.project(identity.tool)
				if got != "" ||
					!errors.Is(gotErr, core.ErrGarbleContract) ||
					!errors.Is(gotErr, core.ErrPrimitiveContract) {
					t.Fatalf(
						"%s projection = (%q, %v), want (empty, %v and %v)",
						projection.name,
						got,
						gotErr,
						core.ErrGarbleContract,
						core.ErrPrimitiveContract,
					)
				}
			}
		})
	}
}
