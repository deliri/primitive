package hostfacts

import "testing"

func TestGoMemoryReachedAssessmentIsAnObservationWithoutExecutionFailure(t *testing.T) {
	t.Parallel()
	snapshot, err := newGoMemorySnapshot(100, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	percent, err := NewPercent(50)
	if err != nil {
		t.Fatal(err)
	}
	got, err := assessGoMemorySnapshot(snapshot, GoMemoryPressurePolicy{TriggerPercent: percent})
	if err != nil || got.State() != MemoryPressureReached || got.Validate() != nil {
		t.Fatalf("reached observation = %+v/%v, want validated reached state without an execution error", got, err)
	}
}

func TestGoMemoryNativeZeroLimitCanBeObservedAndAssessed(t *testing.T) {
	t.Parallel()
	snapshot, err := newGoMemorySnapshot(100, 0, 0)
	if err != nil {
		t.Fatalf("zero native limit snapshot = %v, want a valid runtime setting", err)
	}
	percent, err := NewPercent(100)
	if err != nil {
		t.Fatal(err)
	}
	got, err := assessGoMemorySnapshot(snapshot, GoMemoryPressurePolicy{TriggerPercent: percent})
	if err != nil || got.State() != MemoryPressureReached || got.Validate() != nil {
		t.Fatalf("zero native limit assessment = %+v/%v, want reached state", got, err)
	}
}
