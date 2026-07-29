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
// The loop walks Core's closed hazard domain instead of a hand-listed table, so
// a hazard added to Core cannot enter the tree with its accepting path
// unproven. Validate binds the sibling-order hazard to sibling-table scope and
// every process-global hazard to package-process scope, which is the whole
// admitted combination space.
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
		admitted++

		scope := core.TestIsolationScopePackageProcess
		if hazard == core.TestIsolationHazardSiblingOrder {
			scope = core.TestIsolationScopeSiblingTable
		}
		declaration := core.TestIsolationDeclaration{Hazard: hazard, Scope: scope}
		name := hazard.GoIdentifier() + " with " + scope.GoIdentifier()
		if gotErr := declaration.Validate(); gotErr != nil {
			t.Fatalf("TestIsolationDeclaration{%s}.Validate() error = %v, want nil", name, gotErr)
		}

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
			t.Fatalf("testing.RunTests(%q) = false, want an admitted declaration to permit following behavior", name)
		}
		if !reached {
			t.Fatalf("behavior after Declare(%q) reached = false, want true", name)
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
		if gotPassed {
			t.Fatalf("testing.RunTests(%q) = true, want invalid declaration failure", tc.name)
		}
		if reached {
			t.Fatalf("behavior after rejected Declare(%q) reached = true, want false", tc.name)
		}
	}
}
