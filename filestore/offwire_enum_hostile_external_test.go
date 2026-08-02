package filestore_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type filestoreOffWireEnum interface {
	comparable
	core.OffWireEnum
	IsValid() bool
	String() string
}

func TestFilestoreOffWireEnumsExhaustClosedDomains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func(*testing.T)
		name string
	}{
		{name: "install modes reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveFilestoreOffWireEnum(t, func(raw uint8) filestore.InstallMode { return filestore.InstallMode(raw) }, []filestore.InstallMode{
				filestore.InstallCreate,
				filestore.InstallReplace,
			})
		}},
		{name: "append modes reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveFilestoreOffWireEnum(t, func(raw uint8) filestore.AppendMode { return filestore.AppendMode(raw) }, []filestore.AppendMode{
				filestore.AppendCreate,
				filestore.AppendExisting,
				filestore.AppendCreateOrOpen,
			})
		}},
		{name: "walk directives reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveFilestoreOffWireEnum(t, func(raw uint8) filestore.WalkDirective { return filestore.WalkDirective(raw) }, []filestore.WalkDirective{
				filestore.WalkContinue,
				filestore.WalkSkipDirectory,
			})
		}},
		{name: "walk orders reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveFilestoreOffWireEnum(t, func(raw uint8) filestore.WalkOrder { return filestore.WalkOrder(raw) }, []filestore.WalkOrder{
				filestore.WalkOrderNative,
				filestore.WalkOrderLexical,
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func proveFilestoreOffWireEnum[T filestoreOffWireEnum](
	t *testing.T,
	fromRaw func(uint8) T,
	admitted []T,
) {
	t.Helper()

	wantAdmitted := make(map[T]struct{}, len(admitted))
	wantLabels := make(map[string]T, len(admitted))
	unknownLabel := fromRaw(0).String()
	if unknownLabel == "" {
		t.Fatalf("%T zero String() = empty, want one safe diagnostic", fromRaw(0))
	}
	for _, value := range admitted {
		wantAdmitted[value] = struct{}{}
	}
	for raw := uint16(0); raw <= math.MaxUint8; raw++ {
		value := fromRaw(uint8(raw))
		_, wantValid := wantAdmitted[value]
		gotErr := value.Validate()
		if value.IsValid() != wantValid || (gotErr == nil) != wantValid {
			t.Fatalf(
				"%T(%d) validity = IsValid:%t Validate:%v, want %t",
				value,
				raw,
				value.IsValid(),
				gotErr,
				wantValid,
			)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrFilestoreContract) || value.String() != unknownLabel {
				t.Fatalf(
					"%T(%d) rejection = error:%v label:%q, want %v/%q",
					value,
					raw,
					gotErr,
					value.String(),
					core.ErrFilestoreContract,
					unknownLabel,
				)
			}
			continue
		}
		label := value.String()
		if label == "" || label == unknownLabel {
			t.Fatalf("%T(%d).String() = %q, want an admitted diagnostic", value, raw, label)
		}
		if prior, exists := wantLabels[label]; exists {
			t.Fatalf("%T values %v and %v share label %q, want unique labels", value, prior, value, label)
		}
		wantLabels[label] = value
	}
	proveFilestoreEnumStaysOffWire(t, admitted[0])
}

func proveFilestoreEnumStaysOffWire[T filestoreOffWireEnum](t *testing.T, value T) {
	t.Helper()

	if _, implemented := any(value).(json.Marshaler); implemented {
		t.Fatalf("%T implements json.Marshaler, want an off-wire enum", value)
	}
	if _, implemented := any(&value).(json.Unmarshaler); implemented {
		t.Fatalf("*%T implements json.Unmarshaler, want an off-wire enum", value)
	}
}
