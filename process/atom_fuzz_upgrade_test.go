package process_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func FuzzArgumentEnvironmentAtomsExternalIngress(f *testing.F) {
	argument, err := process.NewArgument("owned argument")
	if err != nil || argument.Validate() != nil {
		f.Fatalf("argument seed: %v", err)
	}
	seed, err := argument.Value()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	name, err := process.NewEnvironmentName("OWNED")
	if err != nil || name.Validate() != nil {
		f.Fatalf("name seed: %v", err)
	}
	seed, err = name.Value()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	value, err := process.NewEnvironmentValue("")
	if err != nil || value.Validate() != nil {
		f.Fatalf("value seed: %v", err)
	}
	seed, err = value.Value()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	for _, hostile := range []string{"bad\x00value", "=value", "\xff", strings.Repeat("N", int(process.EnvironmentNameMaximumBytes)+1), strings.Repeat("x", int(process.ArgumentMaximumBytes)), strings.Repeat("x", int(process.ArgumentMaximumBytes)+1)} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		argument, err := process.NewArgument(raw)
		wantArgument := strings.IndexByte(raw, 0) < 0 && uint64(len(raw)) <= process.ArgumentMaximumBytes
		if !wantArgument {
			if !errors.Is(err, core.ErrProcessContract) || argument != (process.Argument{}) {
				t.Fatalf("argument refusal exposed value: %v", err)
			}
		} else {
			got, projectionErr := argument.Value()
			if err != nil || argument.Validate() != nil || projectionErr != nil || got != raw {
				t.Fatalf("argument changed raw bytes: %v / %v", err, projectionErr)
			}
		}
		name, err := process.NewEnvironmentName(raw)
		wantName := raw != "" && !strings.ContainsAny(raw, "=\x00") && uint64(len(raw)) <= process.EnvironmentNameMaximumBytes
		if !wantName {
			if !errors.Is(err, core.ErrProcessContract) || name != (process.EnvironmentName{}) {
				t.Fatalf("name refusal exposed value: %v", err)
			}
		} else {
			got, projectionErr := name.Value()
			if err != nil || name.Validate() != nil || projectionErr != nil || got != raw {
				t.Fatalf("name changed raw bytes: %v / %v", err, projectionErr)
			}
		}
		value, err := process.NewEnvironmentValue(raw)
		wantValue := strings.IndexByte(raw, 0) < 0 && uint64(len(raw)) <= process.EnvironmentValueMaximumBytes
		if !wantValue {
			if !errors.Is(err, core.ErrProcessContract) || value != (process.EnvironmentValue{}) {
				t.Fatalf("value refusal exposed value: %v", err)
			}
		} else {
			got, projectionErr := value.Value()
			if err != nil || value.Validate() != nil || projectionErr != nil || got != raw {
				t.Fatalf("value changed raw bytes: %v / %v", err, projectionErr)
			}
		}
	})
}
