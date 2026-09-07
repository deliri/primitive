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
		t.Fatal("derived field fixture does not represent source")
	}
	cases := []struct {
		name    string
		data    []byte
		want    Classification
		wantErr error
	}{
		{"exact typed envelope", encoded, source, nil},
		{"reordered independent fields", object(s, o, e, d), source, nil},
		{"optional unavailable operation omitted", object(d, e, s), source, nil},
		{"optional secondary omitted", object(d, e, o), Classification{Disposition: StandardSymbolEffect, Effect: EffectTransport}, nil},
		{"empty document", nil, Classification{}, core.ErrCapabilitiesContract},
		{"null is not classification", []byte("null"), Classification{}, core.ErrCapabilitiesContract},
		{"empty object lacks disposition", []byte("{}"), Classification{}, core.ErrCapabilitiesContract},
		{"array is not object", []byte("[]"), Classification{}, core.ErrCapabilitiesContract},
		{"boolean is not object", []byte("true"), Classification{}, core.ErrCapabilitiesContract},
		{"number is not object", []byte("1"), Classification{}, core.ErrCapabilitiesContract},
		{"unknown field cannot become evidence", object(d, e, o, s, member("future", "true")), Classification{}, core.ErrCapabilitiesContract},
		{"identical field duplicate refused", object(d, e, o, s, d), Classification{}, core.ErrCapabilitiesContract},
		{"conflicting field duplicate refused", object(d, e, o, s, member(disposition, quote(StandardSymbolPure.String()))), Classification{}, core.ErrCapabilitiesContract},
		{"case folded duplicate refused", object(d, e, o, s, member(strings.ToUpper(disposition), quote(StandardSymbolEffect.String()))), Classification{}, core.ErrCapabilitiesContract},
		{"field case does not establish a contract", object(member(strings.ToUpper(disposition), quote(StandardSymbolEffect.String())), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"null required disposition", object(member(disposition, "null"), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"numeric disposition", object(member(disposition, "3"), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"future disposition", object(member(disposition, quote("future")), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"effect disposition missing owner", object(d, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"effect disposition null owner", object(d, member(effect, "null"), o, s), Classification{}, core.ErrCapabilitiesContract},
		{"owner has array type", object(d, member(effect, "[]"), o, s), Classification{}, core.ErrCapabilitiesContract},
		{"secondary has string type", object(d, e, o, member(secondary, quote(EffectFilesystem.String()))), Classification{}, core.ErrCapabilitiesContract},
		{"secondary null element", object(d, e, o, member(secondary, "[null]")), Classification{}, core.ErrCapabilitiesContract},
		{"primary repeated as secondary", object(d, e, o, member(secondary, "["+quote(EffectTransport.String())+"]")), Classification{}, core.ErrCapabilitiesContract},
		{"duplicate secondary", object(d, e, o, member(secondary, "["+quote(EffectFilesystem.String())+","+quote(EffectFilesystem.String())+"]")), Classification{}, core.ErrCapabilitiesContract},
		{"pure cannot own transport", object(member(disposition, quote(StandardSymbolPure.String())), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"context cannot claim secondary", object(member(disposition, quote(StandardSymbolContextual.String())), o, s), Classification{}, core.ErrCapabilitiesContract},
		{"unknown cannot claim operation", object(member(disposition, quote(StandardSymbolUnresolved.String())), member(operation, quote(OperationReadFile.String()))), Classification{}, core.ErrCapabilitiesContract},
		{"operation owner mismatch", object(d, e, member(operation, quote(OperationReadFile.String())), s), Classification{}, core.ErrCapabilitiesContract},
		{"null operation", object(d, e, member(operation, "null"), s), Classification{}, core.ErrCapabilitiesContract},
		{"unpaired UTF16 surrogate", object(member(disposition, `"\ud800"`), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"invalid UTF8 in disposition", object(member(disposition, "\"\xff\""), e, o, s), Classification{}, core.ErrCapabilitiesContract},
		{"truncated final delimiter", bytes.Clone(encoded[:len(encoded)-1]), Classification{}, core.ErrCapabilitiesContract},
		{"second JSON document", append(bytes.Clone(encoded), encoded...), Classification{}, core.ErrCapabilitiesContract},
		{"trailing garbage", append(bytes.Clone(encoded), 'x'), Classification{}, core.ErrCapabilitiesContract},
		{"leading non JSON whitespace", append([]byte{'\v'}, encoded...), Classification{}, core.ErrCapabilitiesContract},
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
