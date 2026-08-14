package garble_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

func TestParseLiteralPolicyExhaustsCanonicalAndHostileLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
		want    garble.LiteralPolicy
	}{
		{name: "preserve label is accepted", value: garble.LiteralPolicyPreserve.String(), want: garble.LiteralPolicyPreserve},
		{name: "obfuscate label is accepted", value: garble.LiteralPolicyObfuscate.String(), want: garble.LiteralPolicyObfuscate},
		{name: "empty label is rejected", wantErr: core.ErrGarbleContract},
		{name: "unknown diagnostic is rejected", value: core.UnknownEnumDiagnostic, wantErr: core.ErrGarbleContract},
		{name: "uppercase preserve is rejected", value: "PRESERVE", wantErr: core.ErrGarbleContract},
		{name: "uppercase obfuscate is rejected", value: "OBFUSCATE", wantErr: core.ErrGarbleContract},
		{name: "leading whitespace is rejected", value: " preserve", wantErr: core.ErrGarbleContract},
		{name: "trailing whitespace is rejected", value: "obfuscate ", wantErr: core.ErrGarbleContract},
		{name: "hyphenated label is rejected", value: "ob-fuscate", wantErr: core.ErrGarbleContract},
		{name: "numeric ordinal is rejected", value: "1", wantErr: core.ErrGarbleContract},
		{name: "embedded nul is rejected", value: "pre\x00serve", wantErr: core.ErrGarbleContract},
		{name: "oversized lookalike is rejected", value: strings.Repeat("preserve", 128), wantErr: core.ErrGarbleContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := garble.ParseLiteralPolicy(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("garble.ParseLiteralPolicy(%q) error = %v, want %v", tc.value, gotErr, core.ErrGarbleContract)
				}
				if got != garble.LiteralPolicyUnknown {
					t.Fatalf("garble.ParseLiteralPolicy(%q) = %v, want unknown on rejection", tc.value, got)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("garble.ParseLiteralPolicy(%q) = (%v, %v), want (%v, nil)", tc.value, got, gotErr, tc.want)
			}
		})
	}
}

func TestParseDiagnosticPolicyExhaustsCanonicalAndHostileLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
		want    garble.DiagnosticPolicy
	}{
		{name: "preserve label is accepted", value: garble.DiagnosticPolicyPreserve.String(), want: garble.DiagnosticPolicyPreserve},
		{name: "strip label is accepted", value: garble.DiagnosticPolicyStrip.String(), want: garble.DiagnosticPolicyStrip},
		{name: "empty label is rejected", wantErr: core.ErrGarbleContract},
		{name: "unknown diagnostic is rejected", value: core.UnknownEnumDiagnostic, wantErr: core.ErrGarbleContract},
		{name: "uppercase preserve is rejected", value: "PRESERVE", wantErr: core.ErrGarbleContract},
		{name: "uppercase strip is rejected", value: "STRIP", wantErr: core.ErrGarbleContract},
		{name: "leading whitespace is rejected", value: " preserve", wantErr: core.ErrGarbleContract},
		{name: "trailing whitespace is rejected", value: "strip ", wantErr: core.ErrGarbleContract},
		{name: "hyphenated label is rejected", value: "str-ip", wantErr: core.ErrGarbleContract},
		{name: "numeric ordinal is rejected", value: "2", wantErr: core.ErrGarbleContract},
		{name: "embedded nul is rejected", value: "st\x00rip", wantErr: core.ErrGarbleContract},
		{name: "oversized lookalike is rejected", value: strings.Repeat("strip", 128), wantErr: core.ErrGarbleContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := garble.ParseDiagnosticPolicy(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("garble.ParseDiagnosticPolicy(%q) error = %v, want %v", tc.value, gotErr, core.ErrGarbleContract)
				}
				if got != garble.DiagnosticPolicyUnknown {
					t.Fatalf("garble.ParseDiagnosticPolicy(%q) = %v, want unknown on rejection", tc.value, got)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("garble.ParseDiagnosticPolicy(%q) = (%v, %v), want (%v, nil)", tc.value, got, gotErr, tc.want)
			}
		})
	}
}

func TestParseDerivationGenerationExhaustsCanonicalAndHostileLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "generation one label is accepted", value: garble.DerivationGenerationOne.String()},
		{name: "empty label is rejected", wantErr: core.ErrGarbleContract},
		{name: "unknown diagnostic is rejected", value: core.UnknownEnumDiagnostic, wantErr: core.ErrGarbleContract},
		{name: "zero word is rejected", value: "zero", wantErr: core.ErrGarbleContract},
		{name: "generation two is rejected before admission", value: "two", wantErr: core.ErrGarbleContract},
		{name: "uppercase label is rejected", value: "ONE", wantErr: core.ErrGarbleContract},
		{name: "leading whitespace is rejected", value: " one", wantErr: core.ErrGarbleContract},
		{name: "trailing whitespace is rejected", value: "one ", wantErr: core.ErrGarbleContract},
		{name: "numeric ordinal is rejected", value: "1", wantErr: core.ErrGarbleContract},
		{name: "embedded nul is rejected", value: "o\x00ne", wantErr: core.ErrGarbleContract},
		{name: "hyphenated lookalike is rejected", value: "o-ne", wantErr: core.ErrGarbleContract},
		{name: "oversized lookalike is rejected", value: strings.Repeat("one", 128), wantErr: core.ErrGarbleContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := garble.ParseDerivationGeneration(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("garble.ParseDerivationGeneration(%q) error = %v, want %v", tc.value, gotErr, core.ErrGarbleContract)
				}
				if got != garble.DerivationGenerationUnknown {
					t.Fatalf("garble.ParseDerivationGeneration(%q) = %v, want unknown on rejection", tc.value, got)
				}
				return
			}
			if gotErr != nil || got != garble.DerivationGenerationOne {
				t.Fatalf("garble.ParseDerivationGeneration(%q) = (%v, %v), want (%v, nil)",
					tc.value, got, gotErr, garble.DerivationGenerationOne)
			}
		})
	}
}

func TestResolveToolRejectsEverySingleProvenanceSubstitution(t *testing.T) {
	t.Parallel()

	current := garble.CurrentTool()
	provenance, err := current.Provenance()
	if err != nil {
		t.Fatalf("garble.ToolIdentity.Provenance() error = %v, want nil", err)
	}
	if err := provenance.Validate(); err != nil {
		t.Fatalf("garble.ToolProvenance.Validate() error = %v, want nil", err)
	}
	if got, gotErr := garble.ResolveTool(provenance); gotErr != nil || got != current {
		t.Fatalf("garble.ResolveTool(current provenance) = (%v, %v), want (%v, nil)", got, gotErr, current)
	}

	cases := []struct {
		mutate func(*garble.ToolProvenance)
		name   string
	}{
		{name: "empty module path is rejected", mutate: func(p *garble.ToolProvenance) { p.ModulePath = "" }},
		{name: "changed module path is rejected", mutate: func(p *garble.ToolProvenance) { p.ModulePath += "/fork" }},
		{name: "empty version is rejected", mutate: func(p *garble.ToolProvenance) { p.Version = "" }},
		{name: "changed version is rejected", mutate: func(p *garble.ToolProvenance) { p.Version += "1" }},
		{name: "empty revision is rejected", mutate: func(p *garble.ToolProvenance) { p.Revision = "" }},
		{name: "changed revision is rejected", mutate: func(p *garble.ToolProvenance) { p.Revision = flipLastByte(p.Revision) }},
		{name: "empty module sum is rejected", mutate: func(p *garble.ToolProvenance) { p.ModuleSum = "" }},
		{name: "changed module sum is rejected", mutate: func(p *garble.ToolProvenance) { p.ModuleSum = flipLastByte(p.ModuleSum) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidate := provenance
			tc.mutate(&candidate)
			got, gotErr := garble.ResolveTool(candidate)
			if !errors.Is(gotErr, core.ErrGarbleContract) {
				t.Fatalf("garble.ResolveTool() error = %v, want %v", gotErr, core.ErrGarbleContract)
			}
			if got != garble.ToolIdentityUnknown {
				t.Fatalf("garble.ResolveTool() = %v, want unknown on rejection", got)
			}
			if gotErr := candidate.Validate(); !errors.Is(gotErr, core.ErrGarbleContract) {
				t.Fatalf("garble.ToolProvenance.Validate() error = %v, want %v", gotErr, core.ErrGarbleContract)
			}
		})
	}
}

func flipLastByte(value string) string {
	last := value[len(value)-1]
	return value[:len(value)-1] + string(last^1)
}
