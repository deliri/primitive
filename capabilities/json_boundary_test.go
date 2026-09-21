package capabilities

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestClassificationHostileJSONDocuments(t *testing.T) {
	t.Parallel()
	source := Classification{Disposition: StandardSymbolEffect, Effect: EffectTransport, Secondary: []Effect{EffectFilesystem}}
	encoded, err := source.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	field := func(name string) string {
		t.Helper()
		f, ok := reflect.TypeFor[classificationWire]().FieldByName(name)
		if !ok {
			t.Fatalf("missing wire field %s", name)
		}
		return strings.Split(f.Tag.Get("json"), ",")[0]
	}
	disposition := field("Disposition")
	effect := field("Effect")
	secondary := field("Secondary")
	operation := field("Operation")
	quote := func(value string) string {
		t.Helper()
		b, err := core.MarshalCanonicalJSONString(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	object := func(members ...string) []byte { return []byte("{" + strings.Join(members, ",") + "}") }
	member := func(name, value string) string { return quote(name) + ":" + value }
	d := member(disposition, quote(StandardSymbolEffect.String()))
	e := member(effect, quote(EffectTransport.String()))
	o := member(operation, quote(OperationUnavailable.String()))
	s := member(secondary, "["+quote(EffectFilesystem.String())+"]")
	baseMembers := []string{d, e, o, s}
	if got, ok := classificationJSONOracle(object(baseMembers...)); !ok || !got.Equal(source) {
		t.Fatalf("derived fixture=(%+v,%t), want (%+v,true)", got, ok, source)
	}
	cases := []struct {
		wantErr error
		name    string
		data    []byte
		want    Classification
	}{
		{name: "exact typed envelope", data: encoded, want: source, wantErr: nil},
		{name: "reordered independent fields", data: object(s, o, e, d), want: source, wantErr: nil},
		{name: "optional unavailable operation omitted", data: object(d, e, s), want: source, wantErr: nil},
		{name: "optional secondary omitted", data: object(d, e, o), want: Classification{Disposition: StandardSymbolEffect, Effect: EffectTransport}, wantErr: nil},
		{name: "empty document", data: nil, want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "null is not classification", data: []byte("null"), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "empty object lacks disposition", data: []byte("{}"), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "array is not object", data: []byte("[]"), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "boolean is not object", data: []byte("true"), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "number is not object", data: []byte("1"), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "unknown field cannot become evidence", data: object(d, e, o, s, member("future", "true")), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "identical field duplicate refused", data: object(d, e, o, s, d), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "conflicting field duplicate refused", data: object(d, e, o, s, member(disposition, quote(StandardSymbolPure.String()))), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "case folded duplicate refused", data: object(d, e, o, s, member(strings.ToUpper(disposition), quote(StandardSymbolEffect.String()))), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "field case does not establish a contract", data: object(member(strings.ToUpper(disposition), quote(StandardSymbolEffect.String())), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "null required disposition", data: object(member(disposition, "null"), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "numeric disposition", data: object(member(disposition, "3"), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "future disposition", data: object(member(disposition, quote("future")), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "effect disposition missing owner", data: object(d, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "effect disposition null owner", data: object(d, member(effect, "null"), o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "owner has array type", data: object(d, member(effect, "[]"), o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "secondary has string type", data: object(d, e, o, member(secondary, quote(EffectFilesystem.String()))), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "secondary null element", data: object(d, e, o, member(secondary, "[null]")), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "primary repeated as secondary", data: object(d, e, o, member(secondary, "["+quote(EffectTransport.String())+"]")), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "duplicate secondary", data: object(d, e, o, member(secondary, "["+quote(EffectFilesystem.String())+","+quote(EffectFilesystem.String())+"]")), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "pure cannot own transport", data: object(member(disposition, quote(StandardSymbolPure.String())), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "context cannot claim secondary", data: object(member(disposition, quote(StandardSymbolContextual.String())), o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "unknown cannot claim operation", data: object(member(disposition, quote(StandardSymbolUnresolved.String())), member(operation, quote(OperationReadFile.String()))), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "operation owner mismatch", data: object(d, e, member(operation, quote(OperationReadFile.String())), s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "null operation", data: object(d, e, member(operation, "null"), s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "unpaired UTF16 surrogate", data: object(member(disposition, `"\ud800"`), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "invalid UTF8 in disposition", data: object(member(disposition, "\"\xff\""), e, o, s), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "truncated final delimiter", data: bytes.Clone(encoded[:len(encoded)-1]), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "second JSON document", data: append(bytes.Clone(encoded), encoded...), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "trailing garbage", data: append(bytes.Clone(encoded), 'x'), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
		{name: "leading non JSON whitespace", data: append([]byte{'\v'}, encoded...), want: Classification{}, wantErr: core.ErrCapabilitiesContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := Classification{Disposition: StandardSymbolEffect, Effect: EffectTime, Operation: OperationObserveTime, Secondary: []Effect{EffectEntropy}}
			got := before
			retained := slices.Clone(before.Secondary)
			err := got.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decode = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !got.Equal(before) || !slices.Equal(before.Secondary, retained) {
					t.Fatalf("refusal mutated prior value %+v", got)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("decode = %+v, want %+v", got, tc.want)
			}
		})
	}
}
