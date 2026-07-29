package garble

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestArgumentInternalKindSeedRelationMatrix ratchets the one relation an
// external test cannot reach. Argument's fields are unexported and every
// externally reachable Argument comes from BuildIntent.Arguments, which never
// builds a wrong pairing. That leaves the relation itself unproven: a non-seed
// argument must carry no seed, and a seed argument must carry a usable one.
// Both directions are forged here so the invariant is proved rather than
// implied by its only current producer.
func TestArgumentInternalKindSeedRelationMatrix(t *testing.T) {
	t.Parallel()

	usable := NewSeed([SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8})
	cases := []struct {
		wantErr  error
		name     string
		wantText string
		argument Argument
	}{
		{name: "seed kind with usable seed lowers", argument: Argument{kind: ArgumentKindSeed, seed: usable}, wantText: seedArgumentPrefix + "AQIDBAUGBwg"},
		{name: "literals kind without seed lowers", argument: Argument{kind: ArgumentKindLiterals}, wantText: literalsArgument},
		{name: "tiny kind without seed lowers", argument: Argument{kind: ArgumentKindTiny}, wantText: tinyArgument},
		{name: "build kind without seed lowers", argument: Argument{kind: ArgumentKindBuild}, wantText: buildArgument},
		{name: "seed kind with unset seed rejects", argument: Argument{kind: ArgumentKindSeed}, wantErr: core.ErrGarbleBuildIntent},
		{name: "literals kind carrying a seed rejects", argument: Argument{kind: ArgumentKindLiterals, seed: usable}, wantErr: core.ErrGarbleBuildIntent},
		{name: "tiny kind carrying a seed rejects", argument: Argument{kind: ArgumentKindTiny, seed: usable}, wantErr: core.ErrGarbleBuildIntent},
		{name: "build kind carrying a seed rejects", argument: Argument{kind: ArgumentKindBuild, seed: usable}, wantErr: core.ErrGarbleBuildIntent},
		{name: "literals kind carrying an all-zero set seed rejects", argument: Argument{kind: ArgumentKindLiterals, seed: NewSeed([SeedBytes]byte{})}, wantErr: core.ErrGarbleBuildIntent},
		{name: "unknown kind rejects", argument: Argument{}, wantErr: core.ErrGarbleBuildIntent},
		{name: "unknown kind carrying a seed rejects", argument: Argument{seed: usable}, wantErr: core.ErrGarbleBuildIntent},
		{name: "exact kind limit rejects", argument: Argument{kind: argumentKindLimit}, wantErr: core.ErrGarbleBuildIntent},
		{name: "one past kind limit rejects", argument: Argument{kind: argumentKindLimit + 1}, wantErr: core.ErrGarbleBuildIntent},
		{name: "maximum kind rejects", argument: Argument{kind: ArgumentKind(math.MaxUint8)}, wantErr: core.ErrGarbleBuildIntent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotValidateErr := tc.argument.Validate()
			if !errors.Is(gotValidateErr, tc.wantErr) {
				t.Fatalf("Argument.Validate() error = %v, want %v", gotValidateErr, tc.wantErr)
			}
			gotText, gotTextErr := tc.argument.Text()
			if !errors.Is(gotTextErr, tc.wantErr) || gotText != tc.wantText {
				t.Fatalf(
					"Argument.Text() = (%q, %v), want (%q, %v)",
					gotText, gotTextErr, tc.wantText, tc.wantErr,
				)
			}
			gotKind, gotKindErr := tc.argument.Kind()
			wantKind := tc.argument.kind
			if tc.wantErr != nil {
				wantKind = ArgumentKindUnknown
			}
			if !errors.Is(gotKindErr, tc.wantErr) || gotKind != wantKind {
				t.Fatalf(
					"Argument.Kind() = (%d, %v), want (%d, %v)",
					gotKind, gotKindErr, wantKind, tc.wantErr,
				)
			}
		})
	}
}

// TestArgumentInternalTextClosedSwitchCoversEveryAdmittedKind proves Text's
// default arm is unreachable for every value the kind domain admits. If a kind
// is added to the enum without a Text arm, the rejection fires here instead of
// reaching a consumer as an empty CLI argument.
func TestArgumentInternalTextClosedSwitchCoversEveryAdmittedKind(t *testing.T) {
	t.Parallel()

	seed := NewSeed([SeedBytes]byte{9})
	for kind := ArgumentKindSeed; kind < argumentKindLimit; kind++ {
		argument := Argument{kind: kind}
		if kind == ArgumentKindSeed {
			argument.seed = seed
		}
		got, gotErr := argument.Text()
		if gotErr != nil || got == "" {
			t.Fatalf("Argument{kind: %d}.Text() = (%q, %v), want non-empty text and nil", kind, got, gotErr)
		}
	}
}
