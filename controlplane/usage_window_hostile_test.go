package controlplane_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The window every rejection case starts from. Bounds and freshness are fixed so
// a failing case names one changed fact rather than a differently built value.
const (
	windowStartNanoseconds     = int64(10)
	windowEndNanoseconds       = int64(20)
	windowFreshnessNanoseconds = int64(20)
	// maximumByteOrdinal is the largest value a class ordinal's underlying byte
	// can hold, so a test that walks to it has exhausted the whole input space.
	maximumByteOrdinal = 255
)

// testWindow builds a window whose units and outcomes are the only variables.
// Every table row that does not name bounds or freshness gets the same interval,
// so the row's name is the whole difference.
func testWindow(
	units []controlplane.WorkUnitCount,
	outcomes []controlplane.OutcomeCount,
) controlplane.UsageWindow {
	return controlplane.UsageWindow{
		Units:    units,
		Outcomes: outcomes,
		Bounds: temporal.IntervalBounds{
			Start: temporal.InstantFromNanoseconds(windowStartNanoseconds),
			End:   temporal.InstantFromNanoseconds(windowEndNanoseconds),
		},
		Freshness: temporal.InstantFromNanoseconds(windowFreshnessNanoseconds),
	}
}

// unitsOf builds an ascending unit list from ordinal/count pairs written as one
// flat sequence, so a table row states its list on one line.
func unitsOf(pairs ...uint64) []controlplane.WorkUnitCount {
	counts := make([]controlplane.WorkUnitCount, 0, len(pairs)/2)
	for index := 0; index+1 < len(pairs); index += 2 {
		counts = append(counts, controlplane.WorkUnitCount{
			Class: controlplane.WorkUnitClass(pairs[index]), Count: pairs[index+1],
		})
	}
	return counts
}

func outcomesOf(pairs ...uint64) []controlplane.OutcomeCount {
	counts := make([]controlplane.OutcomeCount, 0, len(pairs)/2)
	for index := 0; index+1 < len(pairs); index += 2 {
		counts = append(counts, controlplane.OutcomeCount{
			Class: controlplane.OutcomeClass(pairs[index]), Count: pairs[index+1],
		})
	}
	return counts
}

// repeatedUnits returns a list that repeats one class, so the ordering rule must
// refuse it at the second entry however long it is. It is how the table proves
// the walk is bounded by the contract rather than by a length check.
func repeatedUnits(entries int) []controlplane.WorkUnitCount {
	counts := make([]controlplane.WorkUnitCount, entries)
	for index := range counts {
		counts[index] = controlplane.WorkUnitCount{Class: 1, Count: 1}
	}
	return counts
}

func repeatedOutcomes(entries int) []controlplane.OutcomeCount {
	counts := make([]controlplane.OutcomeCount, entries)
	for index := range counts {
		counts[index] = controlplane.OutcomeCount{Class: 1, Count: 1}
	}
	return counts
}

// fullUnitLadder returns one unit of every admitted class, which is the longest
// legal list: the classes are closed and the list is strictly ascending.
func fullUnitLadder() []controlplane.WorkUnitCount {
	counts := make([]controlplane.WorkUnitCount, 0, controlplane.WorkUnitClassMaximum)
	for ordinal := 1; ordinal <= controlplane.WorkUnitClassMaximum; ordinal++ {
		counts = append(counts, controlplane.WorkUnitCount{
			Class: controlplane.WorkUnitClass(ordinal), Count: 1,
		})
	}
	return counts
}

func fullOutcomeLadder() []controlplane.OutcomeCount {
	counts := make([]controlplane.OutcomeCount, 0, controlplane.OutcomeClassMaximum)
	for ordinal := 1; ordinal <= controlplane.OutcomeClassMaximum; ordinal++ {
		counts = append(counts, controlplane.OutcomeCount{
			Class: controlplane.OutcomeClass(ordinal), Count: 1,
		})
	}
	return counts
}

// TestUsageWindowValidateAtEveryBoundary pressures both sides of every rule the
// window owns: class range, list length, list ordering, count floor, arithmetic
// overflow, the freshness-inside-bounds rule, and the agreement between the two
// totals.
//
// The window is the only part of a check-in a customer's machine fills in
// freely, so it is the surface an installation could use to under-report work,
// double-count it, or push the authority into arithmetic it cannot reproduce.
func TestUsageWindowValidateAtEveryBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		window  controlplane.UsageWindow
		wantErr bool
	}{
		// Admitted.
		{
			name:   "one unit classified once is the ordinary window",
			window: testWindow(unitsOf(1, 1), outcomesOf(1, 1)),
		},
		{
			name:   "no work at all is a live installation reporting nothing",
			window: testWindow(nil, nil),
		},
		{
			name:   "empty lists report the same nothing as absent lists",
			window: testWindow([]controlplane.WorkUnitCount{}, []controlplane.OutcomeCount{}),
		},
		{
			name:   "the lowest class ordinal is admitted",
			window: testWindow(unitsOf(1, 3), outcomesOf(1, 3)),
		},
		{
			name: "the highest class ordinal is admitted",
			window: testWindow(
				unitsOf(controlplane.WorkUnitClassMaximum, 3),
				outcomesOf(controlplane.OutcomeClassMaximum, 3),
			),
		},
		{
			name:   "the whole ladder of classes is the longest legal list",
			window: controlplane.UsageWindow{Units: fullUnitLadder(), Outcomes: fullOutcomeLadder(), Bounds: testWindow(nil, nil).Bounds, Freshness: testWindow(nil, nil).Freshness},
		},
		{
			name:   "many units spread over few outcome classes still balance",
			window: testWindow(unitsOf(1, 5, 2, 5), outcomesOf(3, 10)),
		},
		{
			name:   "few unit classes spread over many outcome classes still balance",
			window: testWindow(unitsOf(4, 10), outcomesOf(1, 5, 2, 3, 3, 2)),
		},
		{
			name:   "the maximum count is admitted when both sides agree",
			window: testWindow(unitsOf(1, 1<<63), outcomesOf(1, 1<<63)),
		},
		{
			name: "freshness exactly at the window start is inside it",
			window: controlplane.UsageWindow{
				Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1),
				Bounds: temporal.IntervalBounds{
					Start: temporal.InstantFromNanoseconds(windowStartNanoseconds),
					End:   temporal.InstantFromNanoseconds(windowEndNanoseconds),
				},
				Freshness: temporal.InstantFromNanoseconds(windowStartNanoseconds),
			},
		},
		{
			name: "an instantaneous window with equal bounds is admitted",
			window: controlplane.UsageWindow{
				Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1),
				Bounds: temporal.IntervalBounds{
					Start: temporal.InstantFromNanoseconds(windowStartNanoseconds),
					End:   temporal.InstantFromNanoseconds(windowStartNanoseconds),
				},
				Freshness: temporal.InstantFromNanoseconds(windowStartNanoseconds),
			},
		},

		// Class range, both sides of both ceilings and the floor.
		{
			name:    "the unset unit class reports nothing and is refused",
			window:  testWindow(unitsOf(0, 1), outcomesOf(1, 1)),
			wantErr: true,
		},
		{
			name:    "the unset outcome class reports nothing and is refused",
			window:  testWindow(unitsOf(1, 1), outcomesOf(0, 1)),
			wantErr: true,
		},
		{
			name:    "one unit class above the ceiling is refused",
			window:  testWindow(unitsOf(controlplane.WorkUnitClassMaximum+1, 1), outcomesOf(1, 1)),
			wantErr: true,
		},
		{
			name:    "one outcome class above the ceiling is refused",
			window:  testWindow(unitsOf(1, 1), outcomesOf(controlplane.OutcomeClassMaximum+1, 1)),
			wantErr: true,
		},
		{
			name:    "the largest unsigned byte as a class is refused",
			window:  testWindow(unitsOf(255, 1), outcomesOf(1, 1)),
			wantErr: true,
		},

		// Count floor.
		{
			name:    "a unit class that did no work is absent, not reported as zero",
			window:  testWindow(unitsOf(1, 0), outcomesOf(1, 1)),
			wantErr: true,
		},
		{
			name:    "an outcome class that happened never is absent, not reported as zero",
			window:  testWindow(unitsOf(1, 1), outcomesOf(1, 0)),
			wantErr: true,
		},
		{
			name:    "a zero count hidden behind a valid entry is still refused",
			window:  testWindow(unitsOf(1, 1, 2, 0), outcomesOf(1, 1)),
			wantErr: true,
		},

		// Ordering and uniqueness.
		{
			name:    "a repeated unit class would double count and is refused",
			window:  testWindow(unitsOf(2, 1, 2, 1), outcomesOf(1, 2)),
			wantErr: true,
		},
		{
			name:    "a repeated outcome class would double count and is refused",
			window:  testWindow(unitsOf(1, 2), outcomesOf(2, 1, 2, 1)),
			wantErr: true,
		},
		{
			name:    "descending unit classes are refused",
			window:  testWindow(unitsOf(3, 1, 1, 1), outcomesOf(1, 2)),
			wantErr: true,
		},
		{
			name:    "descending outcome classes are refused",
			window:  testWindow(unitsOf(1, 2), outcomesOf(3, 1, 1, 1)),
			wantErr: true,
		},

		// List length. There is no length rule: a list longer than the ladder
		// cannot hold an admissible class at every position, so the ceiling and
		// the ordering rule are what refuse it, and the walk stops there.
		{
			name: "one unit entry past the class ladder has to repeat a class and is refused",
			window: testWindow(
				append(fullUnitLadder(), controlplane.WorkUnitCount{Class: 1, Count: 1}),
				outcomesOf(1, controlplane.WorkUnitClassMaximum+1),
			),
			wantErr: true,
		},
		{
			name: "one outcome entry past the class ladder is refused",
			window: testWindow(
				unitsOf(1, controlplane.OutcomeClassMaximum+1),
				append(fullOutcomeLadder(), controlplane.OutcomeCount{Class: 1, Count: 1}),
			),
			wantErr: true,
		},
		{
			name:    "an enormous unit list is refused at its second entry, not walked",
			window:  testWindow(repeatedUnits(100_000), outcomesOf(1, 1)),
			wantErr: true,
		},
		{
			name:    "an enormous outcome list is refused at its second entry, not walked",
			window:  testWindow(unitsOf(1, 1), repeatedOutcomes(100_000)),
			wantErr: true,
		},

		// Arithmetic.
		{
			name:    "a unit total that overflows unsigned 64 bits is refused",
			window:  testWindow(unitsOf(1, 1<<63, 2, 1<<63), outcomesOf(1, 1)),
			wantErr: true,
		},
		{
			name:    "an outcome total that overflows unsigned 64 bits is refused",
			window:  testWindow(unitsOf(1, 1), outcomesOf(1, 1<<63, 2, 1<<63)),
			wantErr: true,
		},
		{
			name:    "more units than outcomes leaves work unclassified",
			window:  testWindow(unitsOf(1, 5), outcomesOf(1, 4)),
			wantErr: true,
		},
		{
			name:    "more outcomes than units classifies work that never ran",
			window:  testWindow(unitsOf(1, 4), outcomesOf(1, 5)),
			wantErr: true,
		},
		{
			name:    "one unit off in the totals is refused",
			window:  testWindow(unitsOf(1, 1, 2, 1), outcomesOf(1, 3)),
			wantErr: true,
		},
		{
			name:    "units with no outcomes at all is refused",
			window:  testWindow(unitsOf(1, 1), nil),
			wantErr: true,
		},
		{
			name:    "outcomes with no units at all is refused",
			window:  testWindow(nil, outcomesOf(1, 1)),
			wantErr: true,
		},

		// Interval and freshness, both sides of both bounds.
		{
			name:    "an unset window start is refused",
			window:  controlplane.UsageWindow{Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1), Bounds: temporal.IntervalBounds{End: temporal.InstantFromNanoseconds(windowEndNanoseconds)}, Freshness: temporal.InstantFromNanoseconds(windowEndNanoseconds)},
			wantErr: true,
		},
		{
			name:    "an unset window end is refused",
			window:  controlplane.UsageWindow{Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1), Bounds: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(windowStartNanoseconds)}, Freshness: temporal.InstantFromNanoseconds(windowStartNanoseconds)},
			wantErr: true,
		},
		{
			name:    "unset freshness is refused",
			window:  controlplane.UsageWindow{Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1), Bounds: testWindow(nil, nil).Bounds},
			wantErr: true,
		},
		{
			name: "a window that ends before it starts is refused",
			window: controlplane.UsageWindow{
				Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1),
				Bounds: temporal.IntervalBounds{
					Start: temporal.InstantFromNanoseconds(windowEndNanoseconds),
					End:   temporal.InstantFromNanoseconds(windowStartNanoseconds),
				},
				Freshness: temporal.InstantFromNanoseconds(windowEndNanoseconds),
			},
			wantErr: true,
		},
		{
			name: "freshness one nanosecond before the window start is outside it",
			window: controlplane.UsageWindow{
				Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1),
				Bounds: temporal.IntervalBounds{
					Start: temporal.InstantFromNanoseconds(windowStartNanoseconds),
					End:   temporal.InstantFromNanoseconds(windowEndNanoseconds),
				},
				Freshness: temporal.InstantFromNanoseconds(windowStartNanoseconds - 1),
			},
			wantErr: true,
		},
		{
			name: "freshness one nanosecond after the window end is outside it",
			window: controlplane.UsageWindow{
				Units: unitsOf(1, 1), Outcomes: outcomesOf(1, 1),
				Bounds: temporal.IntervalBounds{
					Start: temporal.InstantFromNanoseconds(windowStartNanoseconds),
					End:   temporal.InstantFromNanoseconds(windowEndNanoseconds),
				},
				Freshness: temporal.InstantFromNanoseconds(windowEndNanoseconds + 1),
			},
			wantErr: true,
		},
		{
			name:   "the zero value carries no window at all and is refused",
			window: controlplane.UsageWindow{}, wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.window.Validate()
			if got := err != nil; got != testCase.wantErr {
				t.Fatalf("Validate() error = %v, want error presence %t", err, testCase.wantErr)
			}
			if !testCase.wantErr {
				return
			}
			if !errors.Is(err, core.ErrControlPlaneUsageWindow) &&
				!errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("Validate() error = %v, want the usage-window contract identity", err)
			}
		})
	}
}

// TestTheClassLadderIsTheOnlyBoundOnAnAcceptedUnitList proves the contract that
// replaced the length guard.
//
// There used to be a check refusing a list longer than the ceiling. It could
// never fire: classes are closed and must strictly increase, so a list cannot
// grow past the ladder without repeating, descending, or naming a class above
// the ceiling, all of which are refused first. Disabling it turned no test red,
// which is how it was found.
//
// Deleting it made that reasoning load bearing, because it is now the only thing
// bounding the walk, so it is proved here instead of asserted in a comment. The
// bound is discovered rather than restated: the loop appends the next legal
// class until Validate refuses, and the length it reaches is the answer. Raising
// the ceiling moves the answer with it, which is correct.
//
// The bound stands on two legs and this test carries one. Opening the ceiling
// lets this loop run past the ladder and turns it red. Relaxing the ordering
// rule from strictly ascending to non-decreasing does not, because this loop
// only ever uses distinct classes: the repeated-class rows in
// TestUsageWindowValidateAtEveryBoundary are what refuse that, and both mutations
// were run to confirm each leg is actually held by something.
func TestTheClassLadderIsTheOnlyBoundOnAnAcceptedUnitList(t *testing.T) {
	t.Parallel()

	// Far above the ceiling, so a relaxed ordering rule ends this test with a
	// failure rather than with a hang.
	const runaway = 4 * controlplane.WorkUnitClassMaximum

	units := make([]controlplane.WorkUnitCount, 0, runaway)
	for len(units) < runaway {
		candidate := make([]controlplane.WorkUnitCount, len(units), len(units)+1)
		copy(candidate, units)
		candidate = append(candidate, controlplane.WorkUnitCount{
			Class: controlplane.WorkUnitClass(len(units) + 1), Count: 1,
		})
		window := testWindow(candidate, outcomesOf(1, uint64(len(candidate))))
		if window.Validate() != nil {
			break
		}
		units = candidate
	}
	if got, want := len(units), controlplane.WorkUnitClassMaximum; got != want {
		t.Fatalf("longest accepted unit list = %d entries, want the class ladder %d", got, want)
	}
}

// TestTheClassLadderIsTheOnlyBoundOnAnAcceptedOutcomeList is the same proof for
// the result classes, which are bounded by the same reasoning and by no length
// check either.
func TestTheClassLadderIsTheOnlyBoundOnAnAcceptedOutcomeList(t *testing.T) {
	t.Parallel()

	const runaway = 4 * controlplane.OutcomeClassMaximum

	outcomes := make([]controlplane.OutcomeCount, 0, runaway)
	for len(outcomes) < runaway {
		candidate := make([]controlplane.OutcomeCount, len(outcomes), len(outcomes)+1)
		copy(candidate, outcomes)
		candidate = append(candidate, controlplane.OutcomeCount{
			Class: controlplane.OutcomeClass(len(outcomes) + 1), Count: 1,
		})
		window := testWindow(unitsOf(1, uint64(len(candidate))), candidate)
		if window.Validate() != nil {
			break
		}
		outcomes = candidate
	}
	if got, want := len(outcomes), controlplane.OutcomeClassMaximum; got != want {
		t.Fatalf("longest accepted outcome list = %d entries, want the class ladder %d", got, want)
	}
}

// TestUsageWindowMarshalRefusesEveryValueValidateRefuses proves the two gates
// agree.
//
// A window that fails Validate but marshals anyway would put a document on the
// wire that the authority must then refuse, and the client would have signed it.
func TestUsageWindowMarshalRefusesEveryValueValidateRefuses(t *testing.T) {
	t.Parallel()

	refused := []controlplane.UsageWindow{
		{},
		testWindow(unitsOf(0, 1), outcomesOf(1, 1)),
		testWindow(unitsOf(1, 5), outcomesOf(1, 4)),
		testWindow(unitsOf(1, 1), nil),
	}
	for index, window := range refused {
		if err := window.Validate(); err == nil {
			t.Fatalf("case %d Validate() error = nil, want a rejection: the fixture must be invalid", index)
		}
		encoded, err := window.MarshalJSON()
		if err == nil {
			t.Fatalf("case %d MarshalJSON() = %s, want a rejection", index, encoded)
		}
		if !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("case %d MarshalJSON() error = %v, want the JSON contract identity", index, err)
		}
	}
}

// TestUsageWindowRoundTripPreservesAbsentAndEmptyLists pins the wire's own
// distinction.
//
// Go renders a nil slice as null and an empty slice as an empty array, and both
// decode back to what they were. Collapsing them here would change the bytes a
// signature covers for a document whose meaning did not change.
func TestUsageWindowRoundTripPreservesAbsentAndEmptyLists(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		want   string
		window controlplane.UsageWindow
	}{
		{
			name:   "absent lists render as null",
			window: testWindow(nil, nil),
			want:   `{"units":null,"outcomes":null,"bounds":{"start":"10","end":"20"},"freshness":"20"}`,
		},
		{
			name:   "empty lists render as empty arrays",
			window: testWindow([]controlplane.WorkUnitCount{}, []controlplane.OutcomeCount{}),
			want:   `{"units":[],"outcomes":[],"bounds":{"start":"10","end":"20"},"freshness":"20"}`,
		},
		{
			name:   "a reported class renders as an ordinal and a count",
			window: testWindow(unitsOf(7, 2), outcomesOf(4, 2)),
			want:   `{"units":[{"class":7,"count":2}],"outcomes":[{"class":4,"count":2}],"bounds":{"start":"10","end":"20"},"freshness":"20"}`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(testCase.window)
			if err != nil {
				t.Fatalf("Marshal() error = %v, want nil", err)
			}
			if got := string(encoded); got != testCase.want {
				t.Fatalf("Marshal() = %s, want %s", got, testCase.want)
			}
			var decoded controlplane.UsageWindow
			if err := decoded.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("UnmarshalJSON(%s) error = %v, want nil", encoded, err)
			}
			if got := decoded.Units == nil; got != (testCase.window.Units == nil) {
				t.Fatalf("decoded units nil = %t, want %t", got, testCase.window.Units == nil)
			}
			if got := decoded.Outcomes == nil; got != (testCase.window.Outcomes == nil) {
				t.Fatalf("decoded outcomes nil = %t, want %t", got, testCase.window.Outcomes == nil)
			}
			again, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("re-Marshal() error = %v, want nil", err)
			}
			if got := string(again); got != testCase.want {
				t.Fatalf("re-Marshal() = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestNewWorkUnitClassAdmitsExactlyTheOrdinalsValidateAdmits proves the door and
// the gate agree over the complete input space of the type.
//
// A bare conversion is how a caller obtains one of these without the
// constructor, so the constructor has to refuse everything the type refuses,
// over every value a byte can hold.
func TestNewWorkUnitClassAdmitsExactlyTheOrdinalsValidateAdmits(t *testing.T) {
	t.Parallel()

	for ordinal := 0; ordinal <= maximumByteOrdinal; ordinal++ {
		want := ordinal >= 1 && ordinal <= controlplane.WorkUnitClassMaximum
		class, err := controlplane.NewWorkUnitClass(uint8(ordinal))
		if got := err == nil; got != want {
			t.Fatalf("NewWorkUnitClass(%d) error = %v, want admitted %t", ordinal, err, want)
		}
		if got := controlplane.WorkUnitClass(ordinal).IsValid(); got != want {
			t.Fatalf("WorkUnitClass(%d).IsValid() = %t, want %t", ordinal, got, want)
		}
		// An admitted ordinal comes back unchanged; a refused one comes back as
		// the zero class rather than as the value that was refused.
		wantClass := controlplane.WorkUnitClass(0)
		if want {
			wantClass = controlplane.WorkUnitClass(ordinal)
		}
		if class != wantClass {
			t.Fatalf("NewWorkUnitClass(%d) = %d, want %d", ordinal, class, wantClass)
		}
	}
}

// TestNewOutcomeClassAdmitsExactlyTheOrdinalsValidateAdmits is the same
// exhaustive proof for the result classes.
func TestNewOutcomeClassAdmitsExactlyTheOrdinalsValidateAdmits(t *testing.T) {
	t.Parallel()

	for ordinal := 0; ordinal <= maximumByteOrdinal; ordinal++ {
		want := ordinal >= 1 && ordinal <= controlplane.OutcomeClassMaximum
		class, err := controlplane.NewOutcomeClass(uint8(ordinal))
		if got := err == nil; got != want {
			t.Fatalf("NewOutcomeClass(%d) error = %v, want admitted %t", ordinal, err, want)
		}
		if got := controlplane.OutcomeClass(ordinal).IsValid(); got != want {
			t.Fatalf("OutcomeClass(%d).IsValid() = %t, want %t", ordinal, got, want)
		}
		wantClass := controlplane.OutcomeClass(0)
		if want {
			wantClass = controlplane.OutcomeClass(ordinal)
		}
		if class != wantClass {
			t.Fatalf("NewOutcomeClass(%d) = %d, want %d", ordinal, class, wantClass)
		}
	}
}

// FuzzUsageWindowDecode pressures the decoder with arbitrary bytes and holds it
// to the contract on both branches: a rejection leaves the receiver untouched,
// and an acceptance produces a value that validates and re-encodes to bytes that
// decode to the same value.
func FuzzUsageWindowDecode(f *testing.F) {
	f.Add([]byte(`{"units":null,"outcomes":null,"bounds":{"start":"10","end":"20"},"freshness":"20"}`))
	f.Add([]byte(`{"units":[{"class":1,"count":1}],"outcomes":[{"class":1,"count":1}],"bounds":{"start":"10","end":"20"},"freshness":"20"}`))
	f.Add([]byte(`{"units":[{"class":0,"count":1}],"outcomes":[],"bounds":{"start":"10","end":"20"},"freshness":"20"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		window := testWindow(unitsOf(9, 4), outcomesOf(9, 4))
		untouched := window
		if err := window.UnmarshalJSON(data); err != nil {
			if window.Freshness != untouched.Freshness || window.Bounds != untouched.Bounds ||
				len(window.Units) != len(untouched.Units) {
				t.Fatalf("UnmarshalJSON() rejected %q but mutated the receiver to %v", data, window)
			}
			return
		}
		if err := window.Validate(); err != nil {
			t.Fatalf("UnmarshalJSON() accepted %q into a window Validate() rejects: %v", data, err)
		}
		encoded, err := window.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() of an accepted window error = %v, want nil", err)
		}
		var again controlplane.UsageWindow
		if err := again.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("UnmarshalJSON() of re-encoded %s error = %v, want nil", encoded, err)
		}
		reencoded, err := again.MarshalJSON()
		if err != nil {
			t.Fatalf("second MarshalJSON() error = %v, want nil", err)
		}
		if string(reencoded) != string(encoded) {
			t.Fatalf("re-encoding is not stable: got %s, want %s", reencoded, encoded)
		}
	})
}
