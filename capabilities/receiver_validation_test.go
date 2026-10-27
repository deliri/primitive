package capabilities

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReceiverValidationMatchesTemporalAdmission(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[temporal.NumericInstant]()
	method, found := typ.MethodByName("Instant")
	if !found || method.Func.Pointer() != reflect.ValueOf(temporal.NumericInstant.Instant).Pointer() {
		t.Fatalf("native method = %+v/%t, want NumericInstant.Instant", method, found)
	}
	symbol := ownershipSymbol(t, typ.PkgPath(), typ.Name(), method.Name)
	for _, row := range [...]struct {
		name  string
		ns    int64
		unset bool
	}{
		{name: "unset", unset: true},
		{name: "minimum", ns: math.MinInt64},
		{name: "before epoch", ns: -1},
		{name: "epoch"},
		{name: "after epoch", ns: 1},
		{name: "maximum", ns: math.MaxInt64},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var value temporal.NumericInstant
			if !row.unset {
				var err error
				value, err = temporal.NewNumericInstant(temporal.InstantFromNanoseconds(row.ns))
				if err != nil {
					t.Fatalf("native producer = %v, want nil", err)
				}
			}
			instant, err := value.Instant()
			if row.unset {
				if !errors.Is(err, core.ErrTemporalContract) || instant != (temporal.Instant{}) {
					t.Fatalf("unset admission = %+v/%v, want zero and native temporal refusal", instant, err)
				}
			} else {
				if err != nil {
					t.Fatalf("native admission = %v, want nil", err)
				}
				ns, err := instant.Nanoseconds()
				if err != nil || ns != row.ns {
					t.Fatalf("admitted instant = %d/%v, want %d/nil", ns, err, row.ns)
				}
			}
			got, err := symbol.ValidatesReceiver()
			if err != nil || !got {
				t.Fatalf("native method contract = %t/%v, want true/nil", got, err)
			}
		})
	}
}

func TestReceiverValidationDoesNotGrantLookalikeContracts(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[temporal.NumericInstant]()
	baseline := ownershipSymbol(t, typ.PkgPath(), typ.Name(), "Instant")
	for _, row := range [...]struct {
		name    string
		change  func(*StandardSymbol)
		refused bool
	}{
		{name: "package function", change: func(s *StandardSymbol) { s.Receiver = nil }},
		{name: "another receiver", change: func(s *StandardSymbol) { n, _ := ParseSymbolName("Instant"); s.Receiver = &n }},
		{name: "numeric duration conversion is not validation", change: func(s *StandardSymbol) {
			n, _ := ParseSymbolName("NumericDuration")
			s.Receiver = &n
			s.Selector, _ = ParseSymbolName("Duration")
		}},
		{name: "presence is not validation", change: func(s *StandardSymbol) { s.Selector, _ = ParseSymbolName("IsSet") }},
		{name: "encoding is unadvertised", change: func(s *StandardSymbol) { s.Selector, _ = ParseSymbolName("MarshalJSON") }},
		{name: "explicit validate needs no alternate contract", change: func(s *StandardSymbol) { s.Selector, _ = ParseSymbolName("Validate") }},
		{name: "foreign namespace", change: func(s *StandardSymbol) { s.ImportPath, _ = gomodule.ParseImportPath("example.com/temporal") }},
		{name: "subpackage", change: func(s *StandardSymbol) {
			s.ImportPath, _ = gomodule.ParseImportPath(baseline.ImportPath.String() + "/child")
		}},
		{name: "namespace prefix", change: func(s *StandardSymbol) {
			s.ImportPath, _ = gomodule.ParseImportPath(baseline.ImportPath.String() + "evil")
		}},
		{name: "invalid path", change: func(s *StandardSymbol) { s.ImportPath = gomodule.ImportPath{} }, refused: true},
		{name: "invalid receiver", change: func(s *StandardSymbol) { n := SymbolName{}; s.Receiver = &n }, refused: true},
		{name: "invalid selector", change: func(s *StandardSymbol) { s.Selector = SymbolName{} }, refused: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			before, err := baseline.ValidatesReceiver()
			if err != nil || !before {
				t.Fatalf("baseline = %t/%v, want true/nil", before, err)
			}
			changed := baseline
			row.change(&changed)
			got, err := changed.ValidatesReceiver()
			if got || (row.refused && !errors.Is(err, core.ErrCapabilitiesContract)) || (!row.refused && err != nil) {
				t.Fatalf("changed contract = %t/%v, want false with refusal=%t", got, err, row.refused)
			}
		})
	}
}

func FuzzReceiverValidationCannotEscapeDeclaringSymbol(f *testing.F) {
	f.Add(uint8(0), "")
	f.Add(uint8(1), "Unicode_λ")
	f.Add(uint8(2), "\x00")
	f.Fuzz(func(t *testing.T, slot uint8, suffix string) {
		temporalType := reflect.TypeFor[temporal.NumericInstant]()
		countType := reflect.TypeFor[core.ByteCount]()
		for _, baseline := range [...]StandardSymbol{
			ownershipSymbol(t, temporalType.PkgPath(), temporalType.Name(), "Instant"),
			ownershipSymbol(t, countType.PkgPath(), countType.Name(), "Uint64"),
		} {
			before, err := baseline.ValidatesReceiver()
			if err != nil || !before {
				t.Fatalf("baseline = %t/%v, want true/nil", before, err)
			}
			changed := baseline
			switch slot % 3 {
			case 0:
				changed.ImportPath, err = gomodule.ParseImportPath(baseline.ImportPath.String() + "X" + suffix)
			case 1:
				var name SymbolName
				name, err = ParseSymbolName(baseline.Receiver.String() + "X" + suffix)
				changed.Receiver = &name
			case 2:
				changed.Selector, err = ParseSymbolName(baseline.Selector.String() + "X" + suffix)
			}
			parseErr := err
			got, err := changed.ValidatesReceiver()
			if got || (parseErr != nil && !errors.Is(err, core.ErrCapabilitiesContract)) || (parseErr == nil && err != nil) {
				t.Fatalf("single-symbol mutation = %t/%v after ingress %v, want no grant and preserved admission state", got, err, parseErr)
			}
		}
	})
}
