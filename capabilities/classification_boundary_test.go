package capabilities

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// A bit set is an independent oracle for uniqueness and exclusion. Every
// possible secondary subset is exercised for every primary owner.
func TestClassificationSecondaryPowerSet(t *testing.T) {
	t.Parallel()
	for primary := EffectFilesystem; primary < effectLimit; primary++ {
		t.Run(primary.String(), func(t *testing.T) {
			t.Parallel()
			for mask := range 1 << IdentityCount {
				value := Classification{Disposition: StandardSymbolEffect, Effect: primary}
				for index := range IdentityCount {
					if mask&(1<<index) != 0 {
						value.Secondary = append(value.Secondary, Effect(index)+EffectFilesystem)
					}
				}
				wantValid := mask&(1<<int(primary-EffectFilesystem)) == 0
				err := value.Validate()
				encoded, marshalErr := value.MarshalJSON()
				if !wantValid {
					if !errors.Is(err, core.ErrCapabilitiesContract) || !errors.Is(marshalErr, core.ErrCapabilitiesContract) || len(encoded) != 0 {
						t.Fatalf("primary %v subset %b leaked (%q,%v,%v)", primary, mask, encoded, err, marshalErr)
					}
					continue
				}
				if err != nil || marshalErr != nil {
					t.Fatalf("admitted subset %b refused: %v / %v", mask, err, marshalErr)
				}
				var got Classification
				if err := got.UnmarshalJSON(encoded); err != nil || !got.Equal(value) {
					t.Fatalf("subset %b decode = (%+v,%v), want %+v", mask, got, err, value)
				}
				slices.Reverse(value.Secondary)
				reversed, err := value.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				if err := got.UnmarshalJSON(reversed); err != nil || !got.Equal(value) {
					t.Fatalf("ordered secondary retention = (%+v,%v), want %+v", got, err, value)
				}
			}
		})
	}
}

func TestClassificationOperationAndSecondaryContradictions(t *testing.T) {
	t.Parallel()
	owners := [operationLimit]Effect{OperationReadFile: EffectFilesystem, OperationWriteFile: EffectFilesystem, OperationRunProcess: EffectProcess, OperationObserveTime: EffectTime}
	for raw := range 256 {
		operation := Operation(raw)
		for disposition := StandardSymbolPure; disposition <= StandardSymbolUnresolved; disposition++ {
			for effect := range effectLimit {
				value := Classification{Operation: operation, Disposition: disposition, Effect: effect}
				want := raw < int(operationLimit) && ((disposition == StandardSymbolEffect && effect != EffectUnknown) || (disposition != StandardSymbolEffect && effect == EffectUnknown))
				if want && operation != OperationUnavailable {
					want = disposition == StandardSymbolEffect && owners[operation] == effect
				}
				err := value.Validate()
				if (err == nil) != want || (!want && !errors.Is(err, core.ErrCapabilitiesContract)) {
					t.Fatalf("classification %+v = %v, want valid %t", value, err, want)
				}
			}
		}
	}
	for primary := EffectFilesystem; primary < effectLimit; primary++ {
		for raw := range 256 {
			secondary := Effect(raw)
			value := Classification{Disposition: StandardSymbolEffect, Effect: primary, Secondary: []Effect{secondary}}
			want := secondary >= EffectFilesystem && secondary < effectLimit && secondary != primary
			if err := value.Validate(); (err == nil) != want {
				t.Fatalf("secondary %d primary %v = %v, want valid %t", raw, primary, err, want)
			}
			if !want {
				continue
			}
			value.Secondary = append(value.Secondary, secondary)
			if err := value.Validate(); !errors.Is(err, core.ErrCapabilitiesContract) {
				t.Fatalf("duplicated secondary accepted: %+v, %v", value, err)
			}
		}
	}
}

func TestClassificationJSONExtentAndAtomicity(t *testing.T) {
	t.Parallel()
	baseline := Classification{Disposition: StandardSymbolEffect, Effect: EffectTransport, Secondary: []Effect{EffectFilesystem}, Operation: OperationUnavailable}
	source, err := baseline.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, extent := range []int{len(source) - 1, len(source), ClassificationJSONMaximumBytes - 1, ClassificationJSONMaximumBytes, ClassificationJSONMaximumBytes + 1} {
		data := bytes.Clone(source)
		if extent < len(source) {
			data = data[:extent]
		} else {
			data = append(data, bytes.Repeat([]byte(" "), extent-len(data))...)
		}
		value := Classification{Disposition: StandardSymbolEffect, Effect: EffectTime, Secondary: []Effect{EffectEntropy}, Operation: OperationObserveTime}
		before := value
		before.Secondary = slices.Clone(value.Secondary)
		err := value.UnmarshalJSON(data)
		wantValid := extent >= len(source) && extent <= ClassificationJSONMaximumBytes
		if wantValid {
			if err != nil || !value.Equal(baseline) {
				t.Fatalf("extent %d decode = (%+v,%v), want %+v", extent, value, err, baseline)
			}
			continue
		}
		if !errors.Is(err, core.ErrCapabilitiesContract) || !value.Equal(before) {
			t.Fatalf("extent %d refusal = (%+v,%v), want preserved %+v", extent, value, err, before)
		}
	}
}

func TestJSONNilReceiversRefuse(t *testing.T) {
	t.Parallel()
	var identity *Identity
	var operation *Operation
	var disposition *StandardSymbolDisposition
	var classification *Classification
	cases := []struct {
		name   string
		decode func([]byte) error
	}{
		{"identity", identity.UnmarshalJSON}, {"operation", operation.UnmarshalJSON}, {"disposition", disposition.UnmarshalJSON}, {"classification", classification.UnmarshalJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.decode(nil); !errors.Is(err, core.ErrCapabilitiesContract) {
				t.Fatalf("nil receiver = %v, want typed refusal", err)
			}
		})
	}
}
