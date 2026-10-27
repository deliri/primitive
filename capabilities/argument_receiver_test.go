package capabilities

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestArgumentReceiverContractMatchesNativeAdmission(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[process.Argument]()
	method, found := typ.MethodByName("Value")
	if !found || method.Func.Pointer() != reflect.ValueOf(process.Argument.Value).Pointer() {
		t.Fatalf("native method=%+v/%t, want process.Argument.Value", method, found)
	}
	symbol := ownershipSymbol(t, typ.PkgPath(), typ.Name(), method.Name)
	for _, row := range [...]struct {
		name, value    string
		unset, refused bool
	}{
		{name: "unset receiver refuses", unset: true, refused: true},
		{name: "constructed empty receiver admits"},
		{name: "spaces remain one argument", value: "two words"},
		{name: "shell syntax remains inert", value: "$(id); *"},
		{name: "opaque bytes remain exact", value: "\xff\xfe\n\t"},
		{name: "NUL producer refuses", value: "before\x00after", refused: true},
		{name: "below native admitted extent", value: strings.Repeat("x", int(process.ArgumentMaximumBytes)-1)},
		{name: "exact native admitted extent", value: strings.Repeat("x", int(process.ArgumentMaximumBytes))},
		{name: "above native admitted extent refuses", value: strings.Repeat("x", int(process.ArgumentMaximumBytes)+1), refused: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var argument process.Argument
			if !row.unset {
				var constructionErr error
				argument, constructionErr = process.NewArgument(row.value)
				if row.refused && !errors.Is(constructionErr, core.ErrProcessContract) || !row.refused && constructionErr != nil {
					t.Fatalf("native argument producer=%v, want process refusal=%t", constructionErr, row.refused)
				}
			}
			value, valueErr := argument.Value()
			if row.refused {
				if !errors.Is(valueErr, core.ErrProcessContract) || value != "" || argument != (process.Argument{}) {
					t.Fatalf("refused receiver=%+v projection bytes=%d error=%v, want zero receiver/output and native process cause", argument, len(value), valueErr)
				}
			} else if valueErr != nil || value != row.value {
				t.Fatalf("native projection bytes=%d digest=%+v error=%v, want exact admitted %d bytes digest=%+v", len(value), core.SHA256Of([]byte(value)), valueErr, len(row.value), core.SHA256Of([]byte(row.value)))
			}
			admitting, err := symbol.ValidatesReceiver()
			if err != nil || !admitting {
				t.Fatalf("native Argument.Value admission=%t/%v, want true/nil", admitting, err)
			}
		})
	}
}

func TestArgumentAdmissionCannotBorrowOtherSymbolAuthority(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[process.Argument]()
	for _, row := range [...]struct{ name, path, receiver, selector string }{
		{name: "package function", path: typ.PkgPath(), selector: "Value"},
		{name: "another Value receiver is unadvertised", path: typ.PkgPath(), receiver: "EnvironmentValue", selector: "Value"},
		{name: "explicit validation is not an alternate", path: typ.PkgPath(), receiver: typ.Name(), selector: "Validate"},
		{name: "foreign namespace", path: "example.com/process", receiver: typ.Name(), selector: "Value"},
		{name: "subpackage", path: typ.PkgPath() + "/child", receiver: typ.Name(), selector: "Value"},
		{name: "namespace prefix", path: typ.PkgPath() + "evil", receiver: typ.Name(), selector: "Value"},
		{name: "core cannot lend process authority", path: reflect.TypeFor[core.ByteCount]().PkgPath(), receiver: typ.Name(), selector: "Value"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			baseline := ownershipSymbol(t, typ.PkgPath(), typ.Name(), "Value")
			if admitting, err := baseline.ValidatesReceiver(); err != nil || !admitting {
				t.Fatalf("baseline=%t/%v, want true/nil", admitting, err)
			}
			symbol := ownershipSymbol(t, row.path, row.receiver, row.selector)
			if admitting, err := symbol.ValidatesReceiver(); err != nil || admitting {
				t.Fatalf("other symbol admission=%t/%v, want false/nil", admitting, err)
			}
		})
	}
}
