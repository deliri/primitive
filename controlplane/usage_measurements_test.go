package controlplane_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

func TestUsageMeasurementsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr      error
		name         string
		measurements []controlplane.UsageCount
		empty        bool
	}{
		{name: "absent measurements preserve ordinary work"},
		{name: "empty work creates no measurements", empty: true},
		{name: "independent counter does not have to equal work units", measurements: unitsOf(1, 9)},
		{name: "different dimensions do not share an overflow total", measurements: unitsOf(1, math.MaxUint64, 2, math.MaxUint64)},
		{name: "every admitted measurement class survives projection", measurements: fullUnitLadder()},
		{name: "measurement without work cannot manufacture evidence", empty: true, measurements: unitsOf(1, 1), wantErr: core.ErrControlPlaneUsageWindow},
		{name: "zero count must be absent", measurements: unitsOf(1, 0), wantErr: core.ErrControlPlaneUsageWindow},
		{name: "identical duplicate class is not silently folded", measurements: unitsOf(1, 1, 1, 1), wantErr: core.ErrControlPlaneUsageWindow},
		{name: "conflicting duplicate class is refused", measurements: unitsOf(1, 1, 1, 2), wantErr: core.ErrControlPlaneUsageWindow},
		{name: "reordered distinct classes are not canonical", measurements: unitsOf(2, 1, 1, 1), wantErr: core.ErrControlPlaneUsageWindow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			window := testWindow(unitsOf(1, 1), outcomesOf(1, 1))
			if tc.empty {
				window.Units, window.Outcomes = nil, nil
			}
			window.Measurements = slices.Clone(tc.measurements)
			if err := window.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tc.wantErr)
			}
			encoded, err := window.MarshalJSON()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(encoded) != 0 {
					t.Fatalf("refused encoding = %d bytes/%v, want empty/%v", len(encoded), err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MarshalJSON() = %v, want nil", err)
			}
			var got controlplane.UsageWindow
			if err := got.UnmarshalJSON(encoded); err != nil || !slices.Equal(got.Measurements, tc.measurements) {
				t.Fatalf("measurements = %+v/%v, want %+v/nil", got.Measurements, err, tc.measurements)
			}
		})
	}
}

func TestUsageMeasurementOrdinalDomainIsExhaustive(t *testing.T) {
	t.Parallel()
	for ordinal := 0; ordinal <= 255; ordinal++ {
		window := testWindow(unitsOf(1, 1), outcomesOf(1, 1))
		window.Measurements = unitsOf(uint64(ordinal), 1)
		wantErr := error(nil)
		if ordinal == 0 || ordinal > controlplane.UsageClassMaximum {
			wantErr = core.ErrControlPlaneUsageWindow
		}
		if err := window.Validate(); !errors.Is(err, wantErr) {
			t.Fatalf("ordinal %d = %v, want %v", ordinal, err, wantErr)
		}
	}
}

func TestUsageMeasurementSignatureAndWatermarkBindingLayerTriad(t *testing.T) {
	t.Parallel()
	window := testCheckInWindow()
	window.Measurements = unitsOf(1, 9, 2, 100)
	issued := issueTestCheckIn(t, controlplaneOffering(t, 3), window)
	server := testControlplaneServer(t, issued.trusted)
	verified, err := server.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request})
	if err != nil {
		t.Fatalf("VerifyCheckIn() = %v, want nil", err)
	}
	// Mutation of caller memory cannot change the already verified facts.
	issued.request.Payload.Window.Measurements[0].Count++
	retained, err := verified.Request()
	if err != nil || retained.Payload.Window.Measurements[0].Count != 9 {
		t.Fatalf("retained measurements = %+v/%v, want original count 9/nil", retained.Payload.Window.Measurements, err)
	}
	_, err = server.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request})
	if !errors.Is(err, core.ErrAttestVerification) {
		t.Fatalf("mutated measurement verification = %v, want %v", err, core.ErrAttestVerification)
	}
	baseline, err := controlplane.AdvanceUsageWatermark(retained.Payload.PreviousWatermark, retained.Payload.Window)
	if err != nil {
		t.Fatalf("baseline watermark = %v, want nil", err)
	}
	changed, err := controlplane.AdvanceUsageWatermark(retained.Payload.PreviousWatermark, issued.request.Payload.Window)
	if err != nil || changed.WindowDigest == baseline.WindowDigest || changed.ChainDigest == baseline.ChainDigest {
		t.Fatalf("changed watermark = %+v/%v, want distinct window and chain digests", changed, err)
	}
}
