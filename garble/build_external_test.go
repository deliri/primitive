package garble_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

func TestBuildIntentPolicyMatrixKeepsGarbleFlagsTypedAndOrdered(t *testing.T) {
	t.Parallel()

	seed := garble.NewSeed([garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8})
	encoded, gotEncodedErr := seed.Encoded()
	if gotEncodedErr != nil {
		t.Fatalf("Seed.Encoded() error = %v, want nil", gotEncodedErr)
	}
	cases := []struct {
		name        string
		wantKinds   []garble.ArgumentKind
		wantTexts   []string
		literals    garble.LiteralPolicy
		diagnostics garble.DiagnosticPolicy
	}{
		{
			name:        "preserve literals and diagnostics",
			literals:    garble.LiteralPolicyPreserve,
			diagnostics: garble.DiagnosticPolicyPreserve,
			wantKinds:   []garble.ArgumentKind{garble.ArgumentKindSeed, garble.ArgumentKindBuild},
			wantTexts:   []string{"-seed=" + encoded, "build"},
		},
		{
			name:        "obfuscate literals only",
			literals:    garble.LiteralPolicyObfuscate,
			diagnostics: garble.DiagnosticPolicyPreserve,
			wantKinds:   []garble.ArgumentKind{garble.ArgumentKindSeed, garble.ArgumentKindLiterals, garble.ArgumentKindBuild},
			wantTexts:   []string{"-seed=" + encoded, "-literals", "build"},
		},
		{
			name:        "strip diagnostics only",
			literals:    garble.LiteralPolicyPreserve,
			diagnostics: garble.DiagnosticPolicyStrip,
			wantKinds:   []garble.ArgumentKind{garble.ArgumentKindSeed, garble.ArgumentKindTiny, garble.ArgumentKindBuild},
			wantTexts:   []string{"-seed=" + encoded, "-tiny", "build"},
		},
		{
			name:        "obfuscate literals and strip diagnostics",
			literals:    garble.LiteralPolicyObfuscate,
			diagnostics: garble.DiagnosticPolicyStrip,
			wantKinds:   []garble.ArgumentKind{garble.ArgumentKindSeed, garble.ArgumentKindLiterals, garble.ArgumentKindTiny, garble.ArgumentKindBuild},
			wantTexts:   []string{"-seed=" + encoded, "-literals", "-tiny", "build"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			intent, gotPrepareErr := garble.PrepareBuild(garble.BuildRequest{
				Tool:        garble.CurrentTool(),
				Seed:        seed,
				Literals:    tc.literals,
				Diagnostics: tc.diagnostics,
			})
			if gotPrepareErr != nil {
				t.Fatalf("PrepareBuild() error = %v, want nil", gotPrepareErr)
			}
			if gotValidateErr := intent.Validate(); gotValidateErr != nil {
				t.Fatalf("BuildIntent.Validate() error = %v, want nil", gotValidateErr)
			}
			arguments, gotArgumentsErr := intent.Arguments()
			if gotArgumentsErr != nil {
				t.Fatalf("BuildIntent.Arguments() error = %v, want nil", gotArgumentsErr)
			}
			var gotKinds []garble.ArgumentKind
			var gotTexts []string
			for argument := range arguments {
				kind, gotKindErr := argument.Kind()
				if gotKindErr != nil {
					t.Fatalf("Argument.Kind() error = %v, want nil", gotKindErr)
				}
				gotText, gotTextErr := argument.Text()
				if gotTextErr != nil {
					t.Fatalf("Argument.Text() error = %v, want nil", gotTextErr)
				}
				gotKinds = append(gotKinds, kind)
				gotTexts = append(gotTexts, gotText)
			}
			if !slices.Equal(gotKinds, tc.wantKinds) {
				t.Fatalf("BuildIntent argument kinds = %v, want %v", gotKinds, tc.wantKinds)
			}
			if !slices.Equal(gotTexts, tc.wantTexts) {
				t.Fatalf("BuildIntent argument text = %q, want official Garble syntax %q", gotTexts, tc.wantTexts)
			}
		})
	}
}

func TestBuildRequestRejectsIncompleteAndUnknownPolicies(t *testing.T) {
	t.Parallel()

	seed := garble.NewSeed([garble.SeedBytes]byte{1})
	valid := garble.BuildRequest{
		Tool:        garble.CurrentTool(),
		Seed:        seed,
		Literals:    garble.LiteralPolicyPreserve,
		Diagnostics: garble.DiagnosticPolicyPreserve,
	}
	cases := []struct {
		mutate func(*garble.BuildRequest)
		name   string
	}{
		{name: "zero tool", mutate: func(r *garble.BuildRequest) { r.Tool = garble.ToolIdentityUnknown }},
		{name: "first future tool", mutate: func(r *garble.BuildRequest) { r.Tool = garble.ToolIdentityPrimitive2026 + 1 }},
		{name: "maximum tool", mutate: func(r *garble.BuildRequest) { r.Tool = garble.ToolIdentity(math.MaxUint8) }},
		{name: "zero seed", mutate: func(r *garble.BuildRequest) { r.Seed = garble.Seed{} }},
		{name: "zero literal policy", mutate: func(r *garble.BuildRequest) { r.Literals = garble.LiteralPolicyUnknown }},
		{name: "first future literal policy", mutate: func(r *garble.BuildRequest) { r.Literals = garble.LiteralPolicyObfuscate + 1 }},
		{name: "maximum literal policy", mutate: func(r *garble.BuildRequest) { r.Literals = garble.LiteralPolicy(math.MaxUint8) }},
		{name: "zero diagnostic policy", mutate: func(r *garble.BuildRequest) { r.Diagnostics = garble.DiagnosticPolicyUnknown }},
		{name: "first future diagnostic policy", mutate: func(r *garble.BuildRequest) { r.Diagnostics = garble.DiagnosticPolicyStrip + 1 }},
		{name: "maximum diagnostic policy", mutate: func(r *garble.BuildRequest) { r.Diagnostics = garble.DiagnosticPolicy(math.MaxUint8) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			tc.mutate(&request)
			got, gotErr := garble.PrepareBuild(request)
			if got != (garble.BuildIntent{}) ||
				!errors.Is(gotErr, core.ErrGarbleBuildIntent) ||
				!errors.Is(gotErr, core.ErrGarbleContract) {
				t.Fatalf("PrepareBuild() = (%v, %v), want (zero, %v and %v)", got, gotErr, core.ErrGarbleBuildIntent, core.ErrGarbleContract)
			}
		})
	}

	if gotErr := (garble.BuildIntent{}).Validate(); !errors.Is(gotErr, core.ErrGarbleBuildIntent) {
		t.Fatalf("BuildIntent{}.Validate() error = %v, want %v", gotErr, core.ErrGarbleBuildIntent)
	}
	if _, gotErr := (garble.BuildIntent{}).Arguments(); !errors.Is(gotErr, core.ErrGarbleBuildIntent) {
		t.Fatalf("BuildIntent{}.Arguments() error = %v, want %v", gotErr, core.ErrGarbleBuildIntent)
	}
}

func TestArgumentKindExhaustsClosedDomainAndZeroArgumentRejects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		kind    garble.ArgumentKind
	}{
		{name: "unknown kind rejected", kind: garble.ArgumentKindUnknown, wantErr: core.ErrGarbleBuildIntent},
		{name: "seed kind admitted", kind: garble.ArgumentKindSeed},
		{name: "literals kind admitted", kind: garble.ArgumentKindLiterals},
		{name: "tiny kind admitted", kind: garble.ArgumentKindTiny},
		{name: "build kind admitted", kind: garble.ArgumentKindBuild},
		{name: "first future kind rejected", kind: garble.ArgumentKindBuild + 1, wantErr: core.ErrGarbleBuildIntent},
		{name: "maximum kind rejected", kind: garble.ArgumentKind(math.MaxUint8), wantErr: core.ErrGarbleBuildIntent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.kind.Validate()
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("ArgumentKind(%d).Validate() error = %v, want nil", tc.kind, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) ||
				!errors.Is(gotErr, core.ErrGarbleContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf(
					"ArgumentKind(%d).Validate() error = %v, want %v/%v/%v",
					tc.kind,
					gotErr,
					tc.wantErr,
					core.ErrGarbleContract,
					core.ErrPrimitiveContract,
				)
			}
		})
	}

	zero := garble.Argument{}
	if gotErr := zero.Validate(); !errors.Is(gotErr, core.ErrGarbleBuildIntent) {
		t.Fatalf("Argument{}.Validate() error = %v, want %v", gotErr, core.ErrGarbleBuildIntent)
	}
	if got, gotErr := zero.Kind(); got != garble.ArgumentKindUnknown ||
		!errors.Is(gotErr, core.ErrGarbleBuildIntent) {
		t.Fatalf("Argument{}.Kind() = (%v, %v), want (%v, %v)", got, gotErr, garble.ArgumentKindUnknown, core.ErrGarbleBuildIntent)
	}
	if got, gotErr := zero.Text(); got != "" ||
		!errors.Is(gotErr, core.ErrGarbleBuildIntent) {
		t.Fatalf("Argument{}.Text() = (%q, %v), want (empty, %v)", got, gotErr, core.ErrGarbleBuildIntent)
	}
}

func TestBuildIntentArgumentIteratorStopsImmediatelyWhenConsumerStops(t *testing.T) {
	t.Parallel()

	seed := garble.NewSeed([garble.SeedBytes]byte{1})
	intent, gotPrepareErr := garble.PrepareBuild(garble.BuildRequest{
		Tool:        garble.CurrentTool(),
		Seed:        seed,
		Literals:    garble.LiteralPolicyObfuscate,
		Diagnostics: garble.DiagnosticPolicyStrip,
	})
	if gotPrepareErr != nil {
		t.Fatalf("PrepareBuild() error = %v, want nil", gotPrepareErr)
	}
	sequence, gotArgumentsErr := intent.Arguments()
	if gotArgumentsErr != nil {
		t.Fatalf("BuildIntent.Arguments() error = %v, want nil", gotArgumentsErr)
	}
	// Stopping only at the first yield leaves every later yield's stop path
	// unproven. An ignored yield result at any position panics the runtime with
	// "range function continued iteration after loop body break", so the
	// consumer stop is exercised at every position the sequence can reach.
	for stopAfter := 1; stopAfter <= 4; stopAfter++ {
		t.Run(fmt.Sprintf("stop after %d", stopAfter), func(t *testing.T) {
			t.Parallel()

			gotYields := 0
			for range sequence {
				gotYields++
				if gotYields == stopAfter {
					break
				}
			}
			if gotYields != stopAfter {
				t.Fatalf("BuildIntent iterator yields with stop after %d = %d, want %d", stopAfter, gotYields, stopAfter)
			}
		})
	}
	t.Run("complete sequence", func(t *testing.T) {
		t.Parallel()

		gotYields := 0
		for range sequence {
			gotYields++
		}
		if gotYields != 4 {
			t.Fatalf("BuildIntent iterator complete yields = %d, want 4", gotYields)
		}
	})
}
