package hostfacts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestSystemdScopeObservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       process.Liveness
		wantErr    error
	}{
		{"loaded scope remains charged regardless of process groups", "loaded\n", process.LivenessAlive, nil},
		{"collected scope is absent", "not-found\n", process.LivenessGone, nil},
		{"empty command output is unknown rather than gone", "", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"failed loading cannot imply absence", "error\n", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"masked unit is outside transient scope contract", "masked\n", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"truncated state is refused", "not-foun", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"multiple replies cannot hide a surviving scope", "not-found\nloaded\n", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"duplicate records are refused", "loaded\nloaded\n", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"unknown future state is refused", "future\n", process.LivenessUnknown, core.ErrHostFactsObservation},
		{"NUL cannot disappear from a state", "not-found\x00\n", process.LivenessUnknown, core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSystemdScopePresence([]byte(tc.text))
			if got != tc.want || tc.wantErr == nil && err != nil || tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("scope=(%v,%v), want (%v,%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestSystemdScopeRequestRefusesBeforeEffects(t *testing.T) {
	t.Parallel()
	unit, err := core.ParsePathComponent("primitive-observation.scope")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := ObserveSystemdScope(ctx, SystemdScopeRequest{Unit: unit, UserManager: true})
	if got != process.LivenessUnknown || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scope=(%v,%v), want unknown and cancellation", got, err)
	}
	got, err = ObserveSystemdScope(t.Context(), SystemdScopeRequest{})
	if got != process.LivenessUnknown || !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("zero scope=(%v,%v), want unknown and contract refusal", got, err)
	}
	platform, err := CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	if platform.OperatingSystem != core.OperatingSystemLinux {
		got, err = ObserveSystemdScope(t.Context(), SystemdScopeRequest{Unit: unit, UserManager: true})
		if got != process.LivenessUnknown || !errors.Is(err, core.ErrHostFactsUnsupported) {
			t.Fatalf("unsupported scope=(%v,%v), want explicit unsupported and no observation", got, err)
		}
	}
}

func FuzzSystemdScopePresenceSemanticClosure(f *testing.F) {
	f.Add([]byte(systemdLoaded + "\n"))
	f.Add([]byte(systemdNotFound + "\n"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseSystemdScopePresence(data)
		if err != nil {
			if got != process.LivenessUnknown || !errors.Is(err, core.ErrHostFactsObservation) {
				t.Fatalf("rejected scope=(%v,%v), want zero observation and typed refusal", got, err)
			}
			return
		}
		text := strings.TrimSpace(string(data))
		if got == process.LivenessAlive && text != "loaded" || got == process.LivenessGone && text != "not-found" || got != process.LivenessAlive && got != process.LivenessGone {
			t.Fatalf("admitted scope=(%v,%q), want exact native state meaning", got, data)
		}
		if mutated, err := parseSystemdScopePresence(append(append([]byte{}, data...), '!')); err == nil || mutated != process.LivenessUnknown {
			t.Fatal("one load-bearing state mutation was admitted")
		}
	})
}
