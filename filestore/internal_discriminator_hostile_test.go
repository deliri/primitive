package filestore

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Every backing value must retain both admission and error ownership. An
// unassigned value cannot silently choose the caller or owned-file branch.
func TestStreamDestinationExhaustiveErrorOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	cases := make([]struct {
		want  error
		name  string
		value streamDestination
	}, math.MaxUint8+1)
	for raw := range cases {
		value := streamDestination(raw)
		want := error(core.ErrFilestoreContract)
		switch value {
		case streamDestinationCaller:
			want = core.ErrFilestoreDestination
		case streamDestinationFile:
			want = core.ErrFilestoreActivation
		}
		cases[raw].name, cases[raw].value, cases[raw].want = fmt.Sprintf("backing value %d retains exact owner", raw), value, want
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantValid := tc.value == streamDestinationCaller || tc.value == streamDestinationFile
			validation := tc.value.Validate()
			if tc.value.IsValid() != wantValid || (validation == nil) != wantValid || !wantValid && !errors.Is(validation, core.ErrFilestoreContract) {
				t.Fatalf("value %d admission = (%t,%v), want %t", tc.value, tc.value.IsValid(), validation, wantValid)
			}
			if (tc.value.String() != core.UnknownEnumDiagnostic) != wantValid || tc.value.String() == "" {
				t.Fatalf("value %d diagnostic = %q, want admitted=%t", tc.value, tc.value.String(), wantValid)
			}
			native := errors.New("owned destination failure")
			got := classifyDestinationError(tc.value, native)
			if !errors.Is(got, tc.want) || !errors.Is(got, native) {
				t.Fatalf("classification = %v, want %v and native cause", got, tc.want)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreSize, core.ErrFilestoreConflict, core.ErrFilestoreActivationIndeterminate} {
				if errors.Is(got, class) != (class == tc.want) || errors.Is(validation, class) {
					t.Fatalf("value %d error class %v = (%v,%v), want only %v on effect", tc.value, class, validation, got, tc.want)
				}
			}
			tc.value.OffWireEnum()
			if _, ok := any(tc.value).(json.Marshaler); ok {
				t.Fatalf("destination implements json.Marshaler = %t, want false", ok)
			}
			if _, ok := any(&tc.value).(json.Unmarshaler); ok {
				t.Fatalf("destination implements json.Unmarshaler = %t, want false", ok)
			}
		})
	}
	if streamDestinationCaller.String() == streamDestinationFile.String() {
		t.Fatalf("destination diagnostics = (%q,%q), want distinct owners", streamDestinationCaller.String(), streamDestinationFile.String())
	}
}

func TestDirectoryPositionExhaustiveOffWireDomain(t *testing.T) {
	t.Parallel()
	cases := make([]directoryPosition, math.MaxUint8+1)
	for raw := range cases {
		cases[raw] = directoryPosition(raw)
	}
	for _, value := range cases {
		t.Run(fmt.Sprintf("backing value %d cannot select an unassigned position", value), func(t *testing.T) {
			t.Parallel()
			wantValid := value == directoryIntermediate || value == directoryFinal
			got := value.Validate()
			if value.IsValid() != wantValid || (got == nil) != wantValid || !wantValid && !errors.Is(got, core.ErrFilestoreContract) {
				t.Fatalf("position %d admission = (%t,%v), want %t", value, value.IsValid(), got, wantValid)
			}
			if (value.String() != core.UnknownEnumDiagnostic) != wantValid || value.String() == "" {
				t.Fatalf("position %d diagnostic = %q, want admitted=%t", value, value.String(), wantValid)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreActivation, core.ErrFilestoreCleanup} {
				if errors.Is(got, class) {
					t.Fatalf("position validation = %v, want no %v effect", got, class)
				}
			}
			value.OffWireEnum()
			if _, ok := any(value).(json.Marshaler); ok {
				t.Fatalf("position implements json.Marshaler = %t, want false", ok)
			}
			if _, ok := any(&value).(json.Unmarshaler); ok {
				t.Fatalf("position implements json.Unmarshaler = %t, want false", ok)
			}
		})
	}
	if directoryIntermediate.String() == directoryFinal.String() {
		t.Fatalf("position diagnostics = (%q,%q), want distinct positions", directoryIntermediate.String(), directoryFinal.String())
	}
}

func TestDirectoryPositionModeOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr       error
		name          string
		wantMode      fs.FileMode
		position      directoryPosition
		wantPreserved bool
	}{
		{name: "intermediate retains ancestor mode", position: directoryIntermediate, wantPreserved: true},
		{name: "final applies requested mode", position: directoryFinal, wantMode: 0o750},
		{name: "zero position refuses before changing mode", wantPreserved: true, wantErr: core.ErrFilestoreContract},
		{name: "future position refuses before changing mode", position: directoryPositionLimit, wantPreserved: true, wantErr: core.ErrFilestoreContract},
		{name: "maximum position cannot wrap into final", position: directoryPosition(math.MaxUint8), wantPreserved: true, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			if err := root.Mkdir("entry", 0o700); err != nil {
				t.Fatal(err)
			}
			if err := root.WriteFile("entry/child", []byte{0, 255, 1}, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := root.Stat("entry")
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("entry")
			if err != nil {
				t.Fatal(err)
			}
			got := ensureDirectoryEntry(directoryEntryEnsure{root: root, path: path, mode: 0o750, position: tc.position})
			if (got == nil) != (tc.wantErr == nil) || tc.wantErr != nil && !errors.Is(got, tc.wantErr) || errors.Is(got, core.ErrFilestoreActivation) {
				t.Fatalf("effect = %v, want %v without activation failure", got, tc.wantErr)
			}
			after, err := root.Stat("entry")
			if err != nil {
				t.Fatal(err)
			}
			data, err := root.ReadFile("entry/child")
			if err != nil {
				t.Fatal(err)
			}
			// Preservation includes inherited native bits such as setgid. A fresh
			// chmod is only an oracle for the branch that actually requests chmod;
			// it may clear bits that a no-effect branch must retain.
			wantMode := before.Mode()
			if !tc.wantPreserved {
				// The OS supplies its actual chmod semantics, including on Windows.
				if err := root.Mkdir("oracle", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := root.Chmod("oracle", tc.wantMode); err != nil {
					t.Fatal(err)
				}
				oracle, err := root.Stat("oracle")
				if err != nil {
					t.Fatal(err)
				}
				wantMode = oracle.Mode()
			}
			if !os.SameFile(before, after) || after.Mode() != wantMode || !after.ModTime().Equal(before.ModTime()) || string(data) != string([]byte{0, 255, 1}) {
				t.Fatalf("directory custody = (%v,%v,%v), want same inode, native mode %v and unchanged child", after.Mode(), after.ModTime(), data, wantMode)
			}
		})
	}
}
