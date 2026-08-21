package manual_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

func FuzzViewJSONSemanticClosure(f *testing.F) {
	for _, value := range []manual.View{manual.ViewHelp, manual.ViewManual} {
		encoded, err := value.MarshalJSON()
		if err != nil {
			f.Fatalf("View.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))
	f.Fuzz(fuzzViewCase)
}

func FuzzSelectionModeJSONSemanticClosure(f *testing.F) {
	for _, value := range []manual.SelectionMode{manual.SelectionModeIndex, manual.SelectionModeTopic} {
		encoded, err := value.MarshalJSON()
		if err != nil {
			f.Fatalf("SelectionMode.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))
	f.Fuzz(fuzzSelectionModeCase)
}

func fuzzViewCase(t *testing.T, data []byte) {
	t.Helper()
	original := manual.ViewHelp
	got := original
	gotErr := got.UnmarshalJSON(data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrManualContract) || got != original {
			t.Fatalf("View.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, original, core.ErrManualContract)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("View.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	canonical, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("View.MarshalJSON(accepted) error = %v, want nil", err)
	}
	var roundTrip manual.View
	if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != got {
		t.Fatalf("View canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("View second canonical projection = (%q, %v), want (%q, nil)", second, err, canonical)
	}
}

func fuzzSelectionModeCase(t *testing.T, data []byte) {
	t.Helper()
	original := manual.SelectionModeIndex
	got := original
	gotErr := got.UnmarshalJSON(data)
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrManualContract) || got != original {
			t.Fatalf("SelectionMode.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, original, core.ErrManualContract)
		}
		return
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("SelectionMode.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	canonical, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("SelectionMode.MarshalJSON(accepted) error = %v, want nil", err)
	}
	var roundTrip manual.SelectionMode
	if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != got {
		t.Fatalf("SelectionMode canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("SelectionMode second canonical projection = (%q, %v), want (%q, nil)", second, err, canonical)
	}
}
