package capabilities

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestByteCountReceiverContractMatchesNativeAdmission(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[core.ByteCount]()
	method, found := typ.MethodByName("Uint64")
	if !found || method.Func.Pointer() != reflect.ValueOf(core.ByteCount.Uint64).Pointer() {
		t.Fatalf("native method = %+v/%t, want ByteCount.Uint64", method, found)
	}
	symbol := ownershipSymbol(t, typ.PkgPath(), typ.Name(), method.Name)
	for _, row := range [...]struct {
		name  string
		value uint64
	}{
		{name: "unset"},
		{name: "minimum positive", value: 1},
		{name: "maximum unsigned", value: math.MaxUint64},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			count, producerErr := core.NewByteCount(row.value)
			got, err := count.Uint64()
			if row.value == 0 {
				if !errors.Is(producerErr, core.ErrPrimitiveContract) || count != (core.ByteCount{}) || !errors.Is(err, core.ErrPrimitiveContract) || got != 0 {
					t.Fatalf("unset producer=%+v/%v conversion=%d/%v, want zero and preserved native refusal", count, producerErr, got, err)
				}
			} else if producerErr != nil || err != nil || got != row.value {
				t.Fatalf("native count producer=%v conversion=%d/%v, want retained %d/nil", producerErr, got, err, row.value)
			}
			admitting, err := symbol.ValidatesReceiver()
			if err != nil || !admitting {
				t.Fatalf("native admission contract=%t/%v, want true/nil", admitting, err)
			}
		})
	}
}

func TestByteCountAdmissionCannotBorrowOtherSymbolAuthority(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeFor[core.ByteCount]()
	for _, row := range [...]struct{ name, path, receiver, selector string }{
		{name: "package function", path: typ.PkgPath(), selector: "Uint64"},
		{name: "another receiver", path: typ.PkgPath(), receiver: "ByteLength", selector: "Uint64"},
		{name: "another conversion is unadvertised", path: typ.PkgPath(), receiver: typ.Name(), selector: "Int64"},
		{name: "explicit validate is not an alternate", path: typ.PkgPath(), receiver: typ.Name(), selector: "Validate"},
		{name: "foreign package", path: "example.com/core", receiver: typ.Name(), selector: "Uint64"},
		{name: "subpackage", path: typ.PkgPath() + "/child", receiver: typ.Name(), selector: "Uint64"},
		{name: "package prefix", path: typ.PkgPath() + "evil", receiver: typ.Name(), selector: "Uint64"},
		{name: "temporal owner cannot lend core authority", path: reflect.TypeFor[temporal.NumericInstant]().PkgPath(), receiver: typ.Name(), selector: "Uint64"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			baseline := ownershipSymbol(t, typ.PkgPath(), typ.Name(), "Uint64")
			admitting, err := baseline.ValidatesReceiver()
			if err != nil || !admitting {
				t.Fatalf("baseline=%t/%v, want true/nil", admitting, err)
			}
			symbol := ownershipSymbol(t, row.path, row.receiver, row.selector)
			admitting, err = symbol.ValidatesReceiver()
			if err != nil || admitting {
				t.Fatalf("changed contract=%t/%v, want false/nil", admitting, err)
			}
		})
	}
}
