package standard

import "testing"

// TestDeriveExperimentIDBindsCanonicalRunAndProbe is a contract ratchet for
// deterministic selection expansion: the same typed child is stable and a
// one-fact probe mutation cannot reuse its experiment identity.
func TestDeriveExperimentIDBindsCanonicalRunAndProbe(t *testing.T) {
	t.Parallel()

	request, observation := fixtureAdmittedObservation(t)
	run := request.Disposition.Admitted.Run
	probe := observation.Probe
	first, firstErr := DeriveExperimentID(run, probe)
	second, secondErr := DeriveExperimentID(run, probe)
	if firstErr != nil || secondErr != nil || first != second {
		t.Fatalf("DeriveExperimentID(same typed child) = (%v, %v) then (%v, %v), want one stable identity", first, firstErr, second, secondErr)
	}
	// The package path is load-bearing identity material. The clean rename from
	// projectstandards to standard intentionally changes this one fixture while
	// the projection above prevents field alignment from changing it again.
	const wantCanonicalIdentity = "01890f2e-7b00-7fa9-934e-075aa0b6400b"
	if first.String() != wantCanonicalIdentity {
		t.Fatalf("DeriveExperimentID(canonical fixture) = %q, want %q", first.String(), wantCanonicalIdentity)
	}
	mutated := probe
	mutated.Target.GoDeclaration.Symbol = fixtureName(t, "TestDifferentEvidence")
	different, differentErr := DeriveExperimentID(run, mutated)
	if differentErr != nil || different == first {
		t.Fatalf("DeriveExperimentID(one-fact symbol mutation) = (%v, %v), want identity different from %v", different, differentErr, first)
	}
	refused, refusedErr := DeriveExperimentID(run, ProbeIdentity{})
	if refusedErr == nil || refused.Validate() == nil {
		t.Fatalf("DeriveExperimentID(zero probe) = (%v, %v), want invalid zero and typed rejection", refused, refusedErr)
	}
}
