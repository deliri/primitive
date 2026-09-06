package attest_test

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type nestedBoundaryCase struct {
	name    string
	input   string
	wantErr error
}

func nestedBoundaryCases() []nestedBoundaryCase {
	limits := core.DefaultStrictJSONLimits()
	cases := []nestedBoundaryCase{
		{name: "empty object is an explicit nested value", input: `{}`},
		{name: "empty array is an explicit nested value", input: `[]`},
		{name: "empty string remains distinct from absent output", input: `""`},
		{name: "false remains distinct from absent output", input: `false`},
		{name: "true remains a boolean", input: `true`},
		{name: "negative zero spelling is owned by the nested emitter", input: `-0`},
		{name: "nested null is admitted inside a non-null container", input: `[null]`},
		{name: "member order and whitespace remain owned by the emitter", input: " \n{\"b\":2, \"a\":1}\t"},
		{name: "same name in siblings does not collide across objects", input: `[{"a":1},{"a":2}]`},
		{name: "same name in parent and child does not collide", input: `{"a":{"a":1}}`},
		{name: "Unicode expansion is not simple case folding", input: `{"ß":1,"ss":2}`},
		{name: "empty callback output is refused", wantErr: core.ErrAttestContract},
		{name: "root null cannot claim a nested fact", input: `null`, wantErr: core.ErrAttestContract},
		{name: "second complete value is refused", input: `{} {}`, wantErr: core.ErrAttestContract},
		{name: "truncated array is refused", input: `[`, wantErr: core.ErrAttestContract},
		{name: "truncated member value is refused", input: `{"a":`, wantErr: core.ErrAttestContract},
		{name: "trailing comma is refused", input: `[1,]`, wantErr: core.ErrAttestContract},
		{name: "invalid UTF8 cannot be repaired", input: "\"\xff\"", wantErr: core.ErrAttestContract},
		{name: "nonadjacent duplicate name is refused", input: `{"a":1,"b":2,"a":3}`, wantErr: core.ErrAttestContract},
		{name: "escaped name collides with its decoded spelling", input: `{"a":1,"\u0061":2}`, wantErr: core.ErrAttestContract},
		{name: "ASCII folded duplicate is refused", input: `{"a":1,"A":2}`, wantErr: core.ErrAttestContract},
		{name: "Kelvin sign collides with ASCII K under simple folding", input: `{"K":1,"K":2}`, wantErr: core.ErrAttestContract},
		{name: "long s collides with ASCII s under simple folding", input: `{"s":1,"ſ":2}`, wantErr: core.ErrAttestContract},
	}
	for _, boundary := range []struct {
		name    string
		delta   int
		wantErr error
	}{
		{name: "one below", delta: -1},
		{name: "exact", delta: 0},
		{name: "one above", delta: 1, wantErr: core.ErrAttestContract},
	} {
		depth := int(limits.NestingDepthMaximum) + boundary.delta
		cases = append(cases, nestedBoundaryCase{name: boundary.name + " nested depth maximum", input: strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth), wantErr: boundary.wantErr})
		items := int(limits.ArrayItemMaximum) + boundary.delta
		cases = append(cases, nestedBoundaryCase{name: boundary.name + " array item maximum", input: "[" + strings.Repeat("0,", items-1) + "0]", wantErr: boundary.wantErr})
		fields := int(limits.ObjectFieldMaximum) + boundary.delta
		var object strings.Builder
		object.WriteByte('{')
		for index := range fields {
			if index > 0 {
				object.WriteByte(',')
			}
			object.Write(strconv.AppendQuote(nil, "f"+strconv.Itoa(index)))
			object.WriteString(":0")
		}
		object.WriteByte('}')
		cases = append(cases, nestedBoundaryCase{name: boundary.name + " nested object field maximum", input: object.String(), wantErr: boundary.wantErr})
		room := attest.CanonicalBodyMaximumBytes - len(`{"nested":""}`) + boundary.delta
		cases = append(cases, nestedBoundaryCase{name: boundary.name + " enclosing document byte maximum", input: `"` + strings.Repeat("x", room) + `"`, wantErr: boundary.wantErr})
	}
	return cases
}

func TestCanonicalObjectNestedRepresentationBoundaryMatrix(t *testing.T) {
	t.Parallel()
	for _, tc := range nestedBoundaryCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			object := attest.BeginCanonicalObject(nil)
			object.Value("nested", externalJSONMember{value: []byte(tc.input)})
			got, err := object.End()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("End() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("rejected nested document = %d bytes, want nil", len(got))
				}
				return
			}
			want := []byte(`{"nested":` + tc.input + `}`)
			if !bytes.Equal(got, want) {
				t.Fatalf("nested document = %d bytes, want %d exact owner bytes", len(got), len(want))
			}
		})
	}
}
