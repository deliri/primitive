package controlplane_test

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// goldenWatermarkSubject takes a real subject from the authority's own bytes
// rather than assembling a plausible one, so these tests exercise the exact
// shape a live installation carries.
func goldenWatermarkSubject(t *testing.T) lease.Subject {
	t.Helper()

	var document controlplane.RegistrationDocument
	if err := document.UnmarshalJSON(readGolden(t, "registration_response.json")); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	return document.Payload.Watermark.Subject
}

// TestInitialUsageWatermarkIsTheOnlyAdmittedStartingPoint pins the genesis fact.
// A starting watermark that varied between two callers would let one
// installation's chain be presented as another's from the first window on.
func TestInitialUsageWatermarkIsTheOnlyAdmittedStartingPoint(t *testing.T) {
	t.Parallel()

	subject := goldenWatermarkSubject(t)
	first, err := controlplane.NewInitialUsageWatermark(subject)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	second, err := controlplane.NewInitialUsageWatermark(subject)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() second call error = %v, want nil", err)
	}
	if first != second {
		t.Fatalf("NewInitialUsageWatermark() = %v, want the identical genesis %v", second, first)
	}
	got, err := first.Generation.Uint64()
	if err != nil {
		t.Fatalf("Generation.Uint64() error = %v, want nil", err)
	}
	if got != controlplane.UsageWatermarkInitialGeneration {
		t.Fatalf("genesis generation = %d, want %d", got, controlplane.UsageWatermarkInitialGeneration)
	}
	// Genesis sets both digests from the subject, so an installation that had
	// accepted no window cannot be confused with one that had accepted a window
	// whose digest happened to be zero.
	if first.WindowDigest != first.ChainDigest {
		t.Fatalf("genesis window digest = %v, want it equal to the chain digest %v",
			first.WindowDigest, first.ChainDigest)
	}
	if first.WindowDigest == (core.SHA256Digest{}) {
		t.Fatalf("genesis window digest = %v, want a domain-separated digest", first.WindowDigest)
	}
}

// TestInitialUsageWatermarkSeparatesSubjects proves the subject is part of the
// fact and not context around it.
func TestInitialUsageWatermarkSeparatesSubjects(t *testing.T) {
	t.Parallel()

	subject := goldenWatermarkSubject(t)
	other := subject
	_, other.DeviceID = testDeviceKey(t, 31)

	first, err := controlplane.NewInitialUsageWatermark(subject)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	second, err := controlplane.NewInitialUsageWatermark(other)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() for the other device error = %v, want nil", err)
	}
	if first.ChainDigest == second.ChainDigest {
		t.Fatalf("two devices share genesis chain digest %v, want distinct chains", first.ChainDigest)
	}
}

// TestNewInitialUsageWatermarkRefusesAnUnvalidatedSubject keeps the genesis
// gate closed. A watermark built from an unset subject would anchor a chain to
// nothing.
func TestNewInitialUsageWatermarkRefusesAnUnvalidatedSubject(t *testing.T) {
	t.Parallel()

	got, err := controlplane.NewInitialUsageWatermark(lease.Subject{})
	if !errors.Is(err, core.ErrControlPlaneUsageWatermark) {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want %v", err, core.ErrControlPlaneUsageWatermark)
	}
	if got != (controlplane.UsageWatermark{}) {
		t.Fatalf("NewInitialUsageWatermark() = %v, want the zero watermark on rejection", got)
	}
}

// TestAdvanceUsageWatermarkOrdersAndChainsAcceptedWindows is the sequence
// contract: each accepted window moves the generation by exactly one and mixes
// into a chain that depends on every window before it.
func TestAdvanceUsageWatermarkOrdersAndChainsAcceptedWindows(t *testing.T) {
	t.Parallel()

	current, err := controlplane.NewInitialUsageWatermark(goldenWatermarkSubject(t))
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	windows := []controlplane.UsageWindow{
		testWindow(unitsOf(1, 1), outcomesOf(1, 1)),
		testWindow(unitsOf(1, 2), outcomesOf(1, 2)),
		testWindow(unitsOf(1, 3), outcomesOf(1, 3)),
	}
	seenChains := map[core.SHA256Digest]int{current.ChainDigest: 0}

	for index, window := range windows {
		next, err := controlplane.AdvanceUsageWatermark(current, window)
		if err != nil {
			t.Fatalf("AdvanceUsageWatermark(window %d) error = %v, want nil", index, err)
		}
		got, err := next.Generation.Uint64()
		if err != nil {
			t.Fatalf("Generation.Uint64() error = %v, want nil", err)
		}
		want := controlplane.UsageWatermarkInitialGeneration + uint64(index) + 1
		if got != want {
			t.Fatalf("generation after window %d = %d, want %d", index, got, want)
		}
		if next.Subject != current.Subject {
			t.Fatalf("subject after window %d = %v, want the unchanged subject %v",
				index, next.Subject, current.Subject)
		}
		if previous, repeated := seenChains[next.ChainDigest]; repeated {
			t.Fatalf("chain digest after window %d repeats the digest from step %d, want a fresh link",
				index, previous)
		}
		seenChains[next.ChainDigest] = index + 1
		// The chain must depend on more than the newest window, or a replayed
		// window would reproduce a whole prefix of the sequence.
		if next.ChainDigest == next.WindowDigest {
			t.Fatalf("chain digest after window %d = the window digest %v, want them domain separated",
				index, next.WindowDigest)
		}
		current = next
	}
}

// TestAdvanceUsageWatermarkIsOrderDependent proves the chain records the order
// windows were accepted in, not merely the set of them.
func TestAdvanceUsageWatermarkIsOrderDependent(t *testing.T) {
	t.Parallel()

	genesis, err := controlplane.NewInitialUsageWatermark(goldenWatermarkSubject(t))
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	first := testWindow(unitsOf(1, 1), outcomesOf(1, 1))
	second := testWindow(unitsOf(1, 2), outcomesOf(1, 2))

	forward := advanceThrough(t, genesis, first, second)
	reverse := advanceThrough(t, genesis, second, first)
	if forward.ChainDigest == reverse.ChainDigest {
		t.Fatalf("chain digest %v is identical for both orderings, want an order-dependent chain",
			forward.ChainDigest)
	}
}

func advanceThrough(t *testing.T, from controlplane.UsageWatermark, windows ...controlplane.UsageWindow) controlplane.UsageWatermark {
	t.Helper()

	current := from
	for index, window := range windows {
		next, err := controlplane.AdvanceUsageWatermark(current, window)
		if err != nil {
			t.Fatalf("AdvanceUsageWatermark(window %d) error = %v, want nil", index, err)
		}
		current = next
	}
	return current
}

// TestAdvanceUsageWatermarkRefusesHostileInputs is the boundary table. Each row
// is a way an authority or a forger could try to move a sequence somewhere it
// must not go.
func TestAdvanceUsageWatermarkRefusesHostileInputs(t *testing.T) {
	t.Parallel()

	genesis, err := controlplane.NewInitialUsageWatermark(goldenWatermarkSubject(t))
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	saturated := genesis
	if saturated.Generation, err = lease.NewGeneration(math.MaxUint64); err != nil {
		t.Fatalf("NewGeneration(MaxUint64) error = %v, want nil", err)
	}
	unsetSubject := genesis
	unsetSubject.Subject = lease.Subject{}
	unsetChain := genesis
	unsetChain.ChainDigest = core.SHA256Digest{}

	acceptedWindow := testWindow(unitsOf(1, 1), outcomesOf(1, 1))
	cases := []struct {
		name    string
		window  controlplane.UsageWindow
		current controlplane.UsageWatermark
	}{
		{name: "the zero window is not an accepted window", current: genesis, window: controlplane.UsageWindow{}},
		{name: "a saturated generation refuses to wrap to one", current: saturated, window: acceptedWindow},
		{name: "an unset subject cannot anchor a chain", current: unsetSubject, window: acceptedWindow},
		{name: "an unset chain digest is not a chain", current: unsetChain, window: acceptedWindow},
		{name: "the zero watermark is not a starting point", current: controlplane.UsageWatermark{}, window: acceptedWindow},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlplane.AdvanceUsageWatermark(testCase.current, testCase.window)
			if !errors.Is(err, core.ErrControlPlaneUsageWatermark) {
				t.Fatalf("AdvanceUsageWatermark() error = %v, want %v",
					err, core.ErrControlPlaneUsageWatermark)
			}
			if got != (controlplane.UsageWatermark{}) {
				t.Fatalf("AdvanceUsageWatermark() = %v, want the zero watermark on rejection", got)
			}
		})
	}
}

// TestUsageWatermarkSurvivesDecodeAndReEncode keeps the durable form stable.
// A watermark is compared against a stored one, so a value that re-encoded
// differently would read as a sequence break.
func TestUsageWatermarkSurvivesDecodeAndReEncode(t *testing.T) {
	t.Parallel()

	genesis, err := controlplane.NewInitialUsageWatermark(goldenWatermarkSubject(t))
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	advanced, err := controlplane.AdvanceUsageWatermark(genesis, testWindow(unitsOf(1, 7), outcomesOf(1, 7)))
	if err != nil {
		t.Fatalf("AdvanceUsageWatermark() error = %v, want nil", err)
	}
	want, err := json.Marshal(advanced)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	var decoded controlplane.UsageWatermark
	if err := decoded.UnmarshalJSON(want); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	if decoded != advanced {
		t.Fatalf("decoded watermark = %v, want %v", decoded, advanced)
	}
	got, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-encoding error = %v, want nil", err)
	}
	if string(got) != string(want) {
		t.Fatalf("re-encoded watermark = %s, want %s", got, want)
	}
}
