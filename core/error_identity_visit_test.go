package core

import "testing"

// TestErrorIdentityVisitSetCoversTheWholeClosedDomain proves the traversal's
// visited set is sized from the domain rather than from a number somebody has
// to remember to raise.
//
// The set used to be two fixed words, and the identity that filled the last
// slot turned the next addition into a compile error inside a file nobody was
// editing. The word count is now derived, so this asserts the property that
// derivation is supposed to give: every admitted identity has a bit.
func TestErrorIdentityVisitSetCoversTheWholeClosedDomain(t *testing.T) {
	t.Parallel()

	if errorIdentityVisitWords*errorIdentityVisitWordBits < int(errorIdentityLimit) {
		t.Fatalf(
			"visit set holds %d bits, want at least %d for the closed domain",
			errorIdentityVisitWords*errorIdentityVisitWordBits,
			int(errorIdentityLimit),
		)
	}

	var visited errorIdentityVisitSet
	for identity := ErrUnknown + 1; identity < errorIdentityLimit; identity++ {
		if !visited.mark(identity) {
			t.Fatalf("mark(%d) = false on its first visit, want true", int(identity))
		}
	}
	for identity := ErrUnknown + 1; identity < errorIdentityLimit; identity++ {
		if visited.mark(identity) {
			t.Fatalf("mark(%d) = true on its second visit, want false", int(identity))
		}
	}
}

// TestErrorIdentityVisitSetMarksBothSidesOfEveryWordBoundary pressures the
// index arithmetic where an off-by-one silently stops marking. A traversal that
// fails to mark does not fail loudly: it re-enqueues and can exhaust the stack,
// so these are the cases that matter.
func TestErrorIdentityVisitSetMarksBothSidesOfEveryWordBoundary(t *testing.T) {
	t.Parallel()

	boundaries := []int{0, 1, 62, 63, 64, 65, 126, 127, 128, 129}
	for _, index := range boundaries {
		if index >= int(errorIdentityLimit) {
			continue
		}
		t.Run(visitBoundaryName(index), func(t *testing.T) {
			t.Parallel()

			var visited errorIdentityVisitSet
			identity := ErrorIdentity(index)
			if !visited.mark(identity) {
				t.Fatalf("mark(%d) = false on a fresh set, want true", index)
			}
			if visited.mark(identity) {
				t.Fatalf("mark(%d) = true when already marked, want false", index)
			}
			// Marking one index must not mark its neighbours, which is exactly
			// what a wrong shift or a wrong word would do.
			for _, neighbour := range []int{index - 1, index + 1} {
				if neighbour < 0 || neighbour >= int(errorIdentityLimit) {
					continue
				}
				if !visited.mark(ErrorIdentity(neighbour)) {
					t.Fatalf("mark(%d) = false after marking %d, want true", neighbour, index)
				}
			}
		})
	}
}

// TestErrorIdentityVisitSetRefusesAnIndexItCannotHold proves an index outside
// the set reports unvisited-refused rather than corrupting a neighbouring word.
func TestErrorIdentityVisitSetRefusesAnIndexItCannotHold(t *testing.T) {
	t.Parallel()

	var visited errorIdentityVisitSet
	beyond := ErrorIdentity(errorIdentityVisitWords * errorIdentityVisitWordBits)
	if visited.mark(beyond) {
		t.Fatalf("mark(%d) = true for an index the set cannot hold, want false", int(beyond))
	}
	if !visited.mark(ErrUnknown + 1) {
		t.Fatalf("mark of a real identity = false after a refused index, want true")
	}
}

func visitBoundaryName(index int) string {
	switch index % errorIdentityVisitWordBits {
	case 0:
		return "first bit of a word"
	case errorIdentityVisitWordBits - 1:
		return "last bit of a word"
	default:
		return "interior bit"
	}
}
