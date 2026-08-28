package testserial_test

import (
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// TestDeclareAcceptsEveryAdmittedDeclaration drives nested tests through
// testing.RunTests, which writes their results to the shared process output
// stream. That, not allocation observation, is why this test cannot run in
// parallel.
//
// Every nested body calls Declare with the case's declaration, so the harness
// observes Declare itself. A nested body that only set reached would pass with
// Declare deleted; that vacuous shape is what this table exists to avoid.
//
// The nested loops walk Core's closed hazard and scope domains instead of
// copying Core's hazard-to-scope pairing rule into this package. Every
// declaration admitted by the owner Validate method must cross the real
// Declare boundary and permit the following behavior.
func TestDeclareAcceptsEveryAdmittedDeclaration(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessOutput,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	admitted := 0
	for raw := 1; ; raw++ {
		hazard := core.TestIsolationHazard(raw)
		if !hazard.IsValid() {
			break
		}
		for rawScope := 1; ; rawScope++ {
			scope := core.TestIsolationScope(rawScope)
			if !scope.IsValid() {
				break
			}
			declaration := core.TestIsolationDeclaration{Hazard: hazard, Scope: scope}
			if gotErr := declaration.Validate(); gotErr != nil {
				continue
			}
			admitted++
			name := hazard.GoIdentifier() + " with " + scope.GoIdentifier()
			reached := false
			gotPassed := testing.RunTests(
				func(pattern string, test string) (bool, error) {
					return true, nil
				},
				[]testing.InternalTest{{
					Name: name,
					F: func(inner *testing.T) {
						testserial.Declare(inner, declaration)
						reached = true
					},
				}},
			)
			if !gotPassed {
				t.Fatalf("testing.RunTests(%q) = false, want true", name)
			}
			if !reached {
				t.Fatalf("behavior after Declare(%q) reached = false, want true", name)
			}
		}
	}

	// A domain-walking loop that admitted nothing would report success without
	// calling Declare once.
	if admitted == 0 {
		t.Fatal("admitted hazard count = 0, want the closed Core hazard domain")
	}
}

// TestDeclareRejectsInvalidDeclarationsBeforeFollowingBehavior drives nested
// tests through testing.RunTests, which writes their results to the shared
// process output stream. That, not allocation observation, is why this test
// cannot run in parallel.
func TestDeclareRejectsInvalidDeclarationsBeforeFollowingBehavior(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessOutput,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	cases := []struct {
		name        string
		declaration core.TestIsolationDeclaration
		wantPassed  bool
		wantReached bool
	}{
		{name: "zero declaration"},
		{
			name: "zero hazard",
			declaration: core.TestIsolationDeclaration{
				Scope: core.TestIsolationScopePackageProcess,
			},
		},
		{
			name: "zero scope",
			declaration: core.TestIsolationDeclaration{
				Hazard: core.TestIsolationHazardProcessEnvironment,
			},
		},
		{
			name: "future hazard",
			declaration: core.TestIsolationDeclaration{
				Hazard: core.TestIsolationHazard(math.MaxUint8),
				Scope:  core.TestIsolationScopePackageProcess,
			},
		},
		{
			name: "future scope",
			declaration: core.TestIsolationDeclaration{
				Hazard: core.TestIsolationHazardProcessEnvironment,
				Scope:  core.TestIsolationScope(math.MaxUint8),
			},
		},
		{
			name: "process hazard with sibling scope",
			declaration: core.TestIsolationDeclaration{
				Hazard: core.TestIsolationHazardProcessEnvironment,
				Scope:  core.TestIsolationScopeSiblingTable,
			},
		},
		{
			name: "sibling hazard with process scope",
			declaration: core.TestIsolationDeclaration{
				Hazard: core.TestIsolationHazardSiblingOrder,
				Scope:  core.TestIsolationScopePackageProcess,
			},
		},
	}
	for _, tc := range cases {
		reached := false
		gotPassed := testing.RunTests(
			func(pattern string, test string) (bool, error) {
				return true, nil
			},
			[]testing.InternalTest{{
				Name: tc.name,
				F: func(inner *testing.T) {
					testserial.Declare(inner, tc.declaration)
					reached = true
				},
			}},
		)
		if gotPassed != tc.wantPassed {
			t.Fatalf("testing.RunTests(%q) = %t, want %t", tc.name, gotPassed, tc.wantPassed)
		}
		if reached != tc.wantReached {
			t.Fatalf("behavior after rejected Declare(%q) reached = %t, want %t", tc.name, reached, tc.wantReached)
		}
	}
}
