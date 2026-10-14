package jsonio_test

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type ownedCodecLeaf struct {
	Value int64 `json:"value"`
}

func (v ownedCodecLeaf) Validate() error {
	if v.Value <= 0 {
		return core.ErrNumericOverflow
	}
	return nil
}

type ownedCodecDocument struct {
	Text     string          `json:"text"`
	Count    int64           `json:"count"`
	Optional *ownedCodecLeaf `json:"optional"`
}

func (v ownedCodecDocument) Validate() error {
	if v.Text == "" || v.Count < 0 {
		return core.ErrJSONContract
	}
	if v.Optional != nil {
		return v.Optional.Validate()
	}
	return nil
}

type ownedCodecOpaque struct {
	Value int64 `json:"value"`
}

func (v ownedCodecOpaque) Validate() error {
	if v.Value < 0 {
		return core.ErrNumericOverflow
	}
	return nil
}
func (v ownedCodecOpaque) MarshalJSON() ([]byte, error) {
	return jsonv2.Marshal(struct {
		Value int64 `json:"value"`
	}{Value: v.Value})
}
func (v *ownedCodecOpaque) UnmarshalJSON(data []byte) error {
	var wire struct {
		Value int64 `json:"value"`
	}
	if err := jsonv2.Unmarshal(data, &wire, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	v.Value = wire.Value
	return nil
}

type ownedCodecMalformedMarshaler struct {
	Bytes []byte
	Cause error
}

func (v ownedCodecMalformedMarshaler) Validate() error              { return nil }
func (v ownedCodecMalformedMarshaler) MarshalJSON() ([]byte, error) { return v.Bytes, v.Cause }

type ownedCodecRefusingUnmarshaller struct{}

func (ownedCodecRefusingUnmarshaller) Validate() error             { return nil }
func (*ownedCodecRefusingUnmarshaller) UnmarshalJSON([]byte) error { return core.ErrCurrencyOverflow }

type ownedCodecOmittedPointer struct {
	Optional *ownedCodecLeaf `json:"optional,omitempty"`
	Ignored  *ownedCodecLeaf `json:"-"`
}

func (ownedCodecOmittedPointer) Validate() error { return nil }

func TestOwnedCodecNativeNullProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, raw string
		wantErr   error
	}{
		{"ordinary object", `{"value":1}`, nil},
		{"empty object neutral", `{}`, nil},
		{"text spelling null is data", `{"value":"null"}`, nil},
		{"nested member null refused", `{"value":{"nested":null}}`, core.ErrJSONContract},
		{"array element null refused", `{"value":[1,null,2]}`, core.ErrJSONContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.MarshalValidated(ownedCodecMalformedMarshaler{Bytes: []byte(row.raw)}, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
			if row.wantErr != nil {
				if got != nil || !errors.Is(err, row.wantErr) {
					t.Fatalf("native null projection=%q/%v, want nil/%v", got, err, row.wantErr)
				}
				return
			}
			if err != nil || string(got) != row.raw {
				t.Fatalf("native null projection=%q/%v, want %q/nil", got, err, row.raw)
			}
		})
	}
	got, err := jsonio.MarshalValidated(ownedCodecOmittedPointer{}, jsonio.OwnedCodecPolicy{})
	if err != nil || string(got) != `{}` {
		t.Fatalf("native omitted pointer projection=%q/%v, want {}/nil", got, err)
	}
}

func TestOwnedCodecPolicyEnumerationsAreExhaustiveAndExclusive(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(fmt.Sprintf("policy_representation_%d", raw), func(t *testing.T) {
			t.Parallel()
			root, nulls := jsonio.RootCodecPolicy(raw), jsonio.PointerNullPolicy(raw)
			for _, row := range []struct {
				name  string
				got   core.OffWireEnum
				valid bool
				text  string
			}{{"root", root, raw <= 1, root.String()}, {"pointer null", nulls, raw <= 1, nulls.String()}} {
				err := row.got.Validate()
				if row.valid && (err != nil || row.text == core.UnknownEnumDiagnostic) || !row.valid && (!errors.Is(err, core.ErrJSONContract) || row.text != core.UnknownEnumDiagnostic) {
					t.Fatalf("%s(%d)=%q/%v, want valid=%t", row.name, raw, row.text, err, row.valid)
				}
			}
		})
	}
	seed := ownedCodecOpaque{Value: 1}
	for _, policy := range []jsonio.OwnedCodecPolicy{{Root: jsonio.RootCodecPolicy(2)}, {PointerNulls: jsonio.PointerNullPolicy(2)}, {Members: jsonio.MemberPolicy(2)}} {
		data, err := jsonio.MarshalValidated(seed, policy)
		got, decodeErr := jsonio.DecodeOwned[ownedCodecOpaque]([]byte(`{"value":1}`), policy)
		if data != nil || got != (ownedCodecOpaque{}) || !errors.Is(err, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("future policy encode=%q/%v decode=%v/%v, want zero/contract", data, err, got, decodeErr)
		}
	}
}

func TestOwnedCodecDeclaredFieldPoliciesLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, raw string
		policy    jsonio.OwnedCodecPolicy
		want      ownedCodecDocument
		wantErr   error
	}{
		{"ordinary declared fields", `{"text":"typed","count":1,"optional":{"value":2}}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{Text: "typed", Count: 1, Optional: &ownedCodecLeaf{Value: 2}}, nil},
		{"zero count neutral", `{"text":"typed","count":0,"optional":{"value":2}}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{Text: "typed", Optional: &ownedCodecLeaf{Value: 2}}, nil},
		{"missing optional neutral", `{"text":"typed","count":0}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{Text: "typed"}, nil},
		{"pointer null strict refusal", `{"text":"typed","count":0,"optional":null}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"pointer null explicit permission", `{"text":"typed","count":0,"optional":null}`, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted}, ownedCodecDocument{Text: "typed"}, nil},
		{"null integer stays forbidden", `{"text":"typed","count":null,"optional":null}`, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted}, ownedCodecDocument{}, core.ErrJSONContract},
		{"null text stays forbidden", `{"text":null,"count":0}`, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted}, ownedCodecDocument{}, core.ErrJSONContract},
		{"unknown extra strict refusal", `{"text":"typed","count":0,"extra":true}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"foreign extra explicit permission", `{"text":"typed","count":0,"extra":{"nested":[true,null,1]}}`, jsonio.OwnedCodecPolicy{Members: jsonio.MembersIgnoreExtra}, ownedCodecDocument{Text: "typed"}, nil},
		{"foreign extra cannot hide bad owned field", `{"text":"typed","count":-1,"extra":true}`, jsonio.OwnedCodecPolicy{Members: jsonio.MembersIgnoreExtra}, ownedCodecDocument{}, core.ErrJSONContract},
		{"foreign unknown duplicate is not open schema", `{"text":"typed","count":0,"extra":1,"extra":2}`, jsonio.OwnedCodecPolicy{Members: jsonio.MembersIgnoreExtra}, ownedCodecDocument{}, core.ErrJSONContract},
		{"known duplicate refusal", `{"text":"first","text":"second","count":0}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"escaped duplicate refusal", `{"text":"first","\u0074ext":"second","count":0}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"nested duplicate refusal", `{"text":"typed","count":0,"optional":{"value":1,"value":2}}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"root null even pointer permission", `null`, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted}, ownedCodecDocument{}, core.ErrJSONContract},
		{"second root is refused", `{"text":"typed","count":0} {"text":"again"}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"wrong numeric type", `{"text":"typed","count":"0"}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"integer overflow", `{"text":"typed","count":9223372036854775808}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"nested owner refusal preserved", `{"text":"typed","count":0,"optional":{"value":0}}`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrNumericOverflow},
		{"root array refused", `[]`, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
		{"empty root refused", ``, jsonio.OwnedCodecPolicy{}, ownedCodecDocument{}, core.ErrJSONContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(row.raw)
			original := bytes.Clone(source)
			got, err := jsonio.DecodeOwned[ownedCodecDocument](source, row.policy)
			if !bytes.Equal(source, original) {
				t.Fatalf("borrowed source changed=%q, want %q", source, original)
			}
			if row.wantErr != nil {
				if got != (ownedCodecDocument{}) || !errors.Is(err, row.wantErr) {
					t.Fatalf("owned refusal=%v/%v, want zero/%v", got, err, row.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.Text != row.want.Text || got.Count != row.want.Count || (got.Optional == nil) != (row.want.Optional == nil) || got.Optional != nil && *got.Optional != *row.want.Optional {
				t.Fatalf("owned fields=%v/%v, want %v", got, err, row.want)
			}
			data, encodeErr := jsonio.MarshalValidated(got, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted})
			again, decodeErr := jsonio.DecodeOwned[ownedCodecDocument](data, jsonio.OwnedCodecPolicy{PointerNulls: jsonio.PointerNullPermitted})
			if encodeErr != nil || decodeErr != nil || again.Text != got.Text || again.Count != got.Count || bytes.HasSuffix(data, []byte{'\n'}) {
				t.Fatalf("owned roundtrip=%q/%v/%v, want compact exact fields", data, encodeErr, decodeErr)
			}
		})
	}
	data, err := jsonio.MarshalValidated(ownedCodecDocument{Text: "typed"}, jsonio.OwnedCodecPolicy{})
	if data != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("default nil pointer marshal=%q/%v, want zero/contract", data, err)
	}
}

func TestOwnedCodecNativeNumericAndWholeValueBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		count, leaf int64
		wantErr     error
	}{
		{"count threshold below", -1, 1, core.ErrJSONContract},
		{"count threshold exact", 0, 1, nil},
		{"count threshold above", 1, 1, nil},
		{"count extreme positive", math.MaxInt64, 1, nil},
		{"count extreme negative", math.MinInt64, 1, core.ErrJSONContract},
		{"leaf threshold below", 1, 0, core.ErrNumericOverflow},
		{"leaf threshold exact", 1, 1, nil},
		{"leaf threshold above", 1, 2, nil},
		{"leaf extreme positive", 1, math.MaxInt64, nil},
		{"leaf extreme negative", 1, math.MinInt64, core.ErrNumericOverflow},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			value := ownedCodecDocument{Text: "typed", Count: row.count, Optional: &ownedCodecLeaf{Value: row.leaf}}
			data, err := jsonio.MarshalValidated(value, jsonio.OwnedCodecPolicy{})
			if row.wantErr != nil {
				if data != nil || !errors.Is(err, row.wantErr) {
					t.Fatalf("owned numeric refusal=%q/%v, want nil/%v", data, err, row.wantErr)
				}
				return
			}
			got, decodeErr := jsonio.DecodeOwned[ownedCodecDocument](data, jsonio.OwnedCodecPolicy{})
			if err != nil || decodeErr != nil || got.Count != row.count || got.Optional == nil || got.Optional.Value != row.leaf {
				t.Fatalf("owned numeric boundary=%v errors %v/%v, want exact %d/%d", got, err, decodeErr, row.count, row.leaf)
			}
		})
	}
	for _, extent := range []int{0, 1, 32767, 32768, 32769, 262151, 4194305} {
		t.Run(fmt.Sprintf("owned_document_text_%d_bytes", extent), func(t *testing.T) {
			t.Parallel()
			value := ownedCodecDocument{Text: strings.Repeat("x", extent), Optional: &ownedCodecLeaf{Value: 1}}
			data, err := jsonio.MarshalValidated(value, jsonio.OwnedCodecPolicy{})
			if extent == 0 {
				if data != nil || !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("empty owning text=%q/%v, want nil/contract", data, err)
				}
				return
			}
			got, decodeErr := jsonio.DecodeOwned[ownedCodecDocument](data, jsonio.OwnedCodecPolicy{})
			if err != nil || decodeErr != nil || got.Text != value.Text || got.Optional == nil || got.Optional.Value != 1 {
				t.Fatalf("owned value extent=%d errors %v/%v, want exact %d and admitted leaf", len(got.Text), err, decodeErr, extent)
			}
		})
	}
}

func TestOwnedCodecNativeMethodsRequireExplicitAdmission(t *testing.T) {
	t.Parallel()
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}
	seed := ownedCodecOpaque{Value: 7}
	data, err := jsonio.MarshalValidated(seed, policy)
	got, decodeErr := jsonio.DecodeOwned[ownedCodecOpaque](data, policy)
	if err != nil || decodeErr != nil || got != seed || string(data) != `{"value":7}` {
		t.Fatalf("owned method=%q/%v/%v/%v, want compact exact %v", data, got, err, decodeErr, seed)
	}
	refused, refuseErr := jsonio.MarshalValidated(seed, jsonio.OwnedCodecPolicy{})
	zero, zeroErr := jsonio.DecodeOwned[ownedCodecOpaque](data, jsonio.OwnedCodecPolicy{})
	if refused != nil || zero != (ownedCodecOpaque{}) || !errors.Is(refuseErr, core.ErrJSONContract) || !errors.Is(zeroErr, core.ErrJSONContract) {
		t.Fatalf("opaque default=%q/%v/%v/%v, want refusals", refused, zero, refuseErr, zeroErr)
	}
	for _, raw := range []string{"", `null`, `{"value":1,"value":2}`, `{"value":1} {"value":2}`, `{"value":"\ud800"}`} {
		got, err := jsonio.DecodeOwned[ownedCodecOpaque]([]byte(raw), policy)
		if got != (ownedCodecOpaque{}) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("opaque native grammar refusal(%q)=%v/%v, want zero/contract", raw, got, err)
		}
	}
	for _, raw := range []string{`{"same":1,"same":2}`, `{"value":"\ud800"}`, `{"value":1} {"value":2}`, `null`} {
		data, err := jsonio.MarshalValidated(ownedCodecMalformedMarshaler{Bytes: []byte(raw)}, policy)
		if data != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("opaque encoded grammar(%q)=%q/%v, want nil/contract", raw, data, err)
		}
	}
	data, err = jsonio.MarshalValidated(ownedCodecMalformedMarshaler{Cause: core.ErrCurrencyOverflow}, policy)
	if data != nil || !errors.Is(err, core.ErrCurrencyOverflow) {
		t.Fatalf("owner encoder failure=%q/%v, want nil/original cause", data, err)
	}
	refusal, err := jsonio.DecodeOwned[ownedCodecRefusingUnmarshaller]([]byte(`{}`), policy)
	if refusal != (ownedCodecRefusingUnmarshaller{}) || !errors.Is(err, core.ErrCurrencyOverflow) {
		t.Fatalf("owner decoder failure=%v/%v, want zero/original cause", refusal, err)
	}
	var pointer *ownedCodecOpaque
	data, err = jsonio.MarshalValidated(pointer, policy)
	if data != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("typed nil owner=%q/%v, want nil/contract", data, err)
	}
}

func FuzzOwnedCodecPolicyAndGrammarSemanticClosure(f *testing.F) {
	seed := ownedCodecDocument{Text: "typed", Count: 1, Optional: &ownedCodecLeaf{Value: 2}}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	data, err := jsonio.MarshalValidated(seed, jsonio.OwnedCodecPolicy{})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data, uint8(0), uint8(0), uint8(0))
	f.Add([]byte(`{"text":"typed","count":0,"optional":null,"extra":true}`), uint8(1), uint8(1), uint8(0))
	f.Add([]byte{}, uint8(0), uint8(0), uint8(0))
	f.Add([]byte(`{"text":"typed","text":"again"}`), uint8(1), uint8(1), uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, members, nulls, root uint8) {
		policy := jsonio.OwnedCodecPolicy{Members: jsonio.MemberPolicy(members), PointerNulls: jsonio.PointerNullPolicy(nulls), Root: jsonio.RootCodecPolicy(root)}
		got, err := jsonio.DecodeOwned[ownedCodecDocument](data, policy)
		var want ownedCodecDocument
		nativeErr := jsonv2.Unmarshal(data, &want, jsonv2.RejectUnknownMembers(members == 0))
		var probe struct {
			Text     jsontext.Value `json:"text"`
			Count    jsontext.Value `json:"count"`
			Optional jsontext.Value `json:"optional"`
		}
		probeErr := jsonv2.Unmarshal(data, &probe)
		accepted := members <= 1 && nulls <= 1 && root <= 1 && jsontext.Value(data).IsValid() && jsontext.Value(data).Kind() == jsontext.KindBeginObject && nativeErr == nil && probeErr == nil && want.Validate() == nil && jsontext.Value(probe.Text).Kind() != jsontext.KindNull && jsontext.Value(probe.Count).Kind() != jsontext.KindNull && (nulls == 1 || jsontext.Value(probe.Optional).Kind() != jsontext.KindNull)
		if !accepted {
			if got != (ownedCodecDocument{}) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("owned grammar refusal=%v/%v, want zero/contract", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Text != want.Text || got.Count != want.Count || (got.Optional == nil) != (want.Optional == nil) || got.Optional != nil && *got.Optional != *want.Optional {
			t.Fatalf("owned native parity=%v/%v, want %v", got, err, want)
		}
		encodePolicy := policy
		encodePolicy.PointerNulls = jsonio.PointerNullPermitted
		canonical, encodeErr := jsonio.MarshalValidated(got, encodePolicy)
		again, decodeErr := jsonio.DecodeOwned[ownedCodecDocument](canonical, encodePolicy)
		second, secondErr := jsonio.MarshalValidated(again, encodePolicy)
		if encodeErr != nil || decodeErr != nil || secondErr != nil || !bytes.Equal(canonical, second) {
			t.Fatalf("owned canonical closure=%q errors %v/%v/%v, want stablecompact", canonical, encodeErr, decodeErr, secondErr)
		}
	})
}
