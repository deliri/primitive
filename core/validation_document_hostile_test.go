package core

import (
	"encoding/json/jsontext"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestValidateStrictJSONDocumentOwnsCompleteAdmission(t *testing.T) {
	t.Parallel()
	limits := StrictJSONLimits{DocumentMaximumBytes: mustByteCountForTest(t, 64), NestingDepthMaximum: 2, ObjectFieldMaximum: 2, ArrayItemMaximum: 2}
	for _, value := range []string{`null`, `{"a":[1,2],"b":0}`, `[[]]`} {
		if err := limits.ValidateDocument([]byte(value)); err != nil {
			t.Fatalf("valid document %q = %v", value, err)
		}
	}
	for _, value := range []string{"", strings.Repeat(" ", 64) + `0`, `[1,2,3]`, `{"a":1,"b":2,"c":3}`, `[[[]]]`, `{"a":1,"A":2}`, `null true`, `"` + string([]byte{0xff}) + `"`} {
		if err := limits.ValidateDocument([]byte(value)); !errors.Is(err, ErrJSONContract) {
			t.Fatalf("invalid document %q = %v, want typed refusal", value, err)
		}
	}
	if err := (StrictJSONLimits{}).ValidateDocument([]byte(`0`)); !errors.Is(err, ErrJSONContract) {
		t.Fatalf("invalid limits = %v, want typed refusal", err)
	}
}

func FuzzValidateStrictJSONDocumentGrammar(f *testing.F) {
	for _, value := range []string{`null`, `{"a":[1,2]}`, "", `{}[]`, `{"a":1,"A":2}`} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		limits := DefaultStrictJSONLimits()
		err := limits.ValidateDocument([]byte(value))
		if err != nil {
			if !errors.Is(err, ErrJSONContract) {
				t.Fatalf("refusal identity = %v", err)
			}
			return
		}
		decoder := jsontext.NewDecoder(strings.NewReader(value))
		if err := decoder.SkipValue(); err != nil {
			t.Fatalf("admitted document violates Go grammar: %v", err)
		}
		if err := decoder.SkipValue(); err != io.EOF {
			t.Fatalf("admitted document has trailing input: %v", err)
		}
		if err := limits.ValidateDocument([]byte(value + " null")); !errors.Is(err, ErrJSONContract) {
			t.Fatalf("second-value mutation = %v, want refusal", err)
		}
	})
}
