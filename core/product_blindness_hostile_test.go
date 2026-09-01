package core

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type consumerReference struct {
	file  string
	value string
}

func unambiguousConsumerNames() [7]string {
	return [...]string{
		"anvil",
		"peachfuzz",
		"blink-kernel",
		"blink_kernel",
		"blink kernel",
		"github.com/offgridsoft/witness",
		"github.com/offgridsoft/bug",
	}
}

func TestPrimitiveProductionIsBlindToConsumerNames(t *testing.T) {
	t.Parallel()

	paths, err := productionSourcePaths("..")
	if err != nil {
		t.Fatalf("productionSourcePaths() error = %v, want nil", err)
	}
	var got []consumerReference
	for _, path := range paths {
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, readErr)
		}
		references, scanErr := consumerReferences(path, source)
		if scanErr != nil {
			t.Fatalf("consumerReferences(%q) error = %v, want nil", path, scanErr)
		}
		got = append(got, references...)
	}
	if len(got) != 0 {
		t.Fatalf("Primitive production consumer references = %+v, want none", got)
	}
}

func TestPrimitiveConsumerNameMatcherLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
		want   []consumerReference
	}{
		{
			name:   "positive generic execution vocabulary stays product blind",
			source: "package fixture\nconst namespace = \"runner-control-request:\"\n",
		},
		{
			name:   "negative branded comment path and offering are all reported",
			source: "package fixture\n// Anvil policy must not leak.\nconst path = \"github.com/offGridSoft/Witness/cmd/witness\"\nconst offering = \"bug\"\n",
			want: []consumerReference{
				{file: "fixture.go", value: "anvil"},
				{file: "fixture.go", value: "bug"},
				{file: "fixture.go", value: "github.com/offgridsoft/witness"},
			},
		},
		{
			name:   "neutral architecture witness and ordinary bug prose remain usable",
			source: "package fixture\n// A compiler witness catches a regression bug.\ntype contractWitness struct{}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := consumerReferences("fixture.go", []byte(tc.source))
			if err != nil {
				t.Fatalf("consumerReferences() error = %v, want nil", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("consumerReferences() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestPackageCapabilityClosedDomainIsCompilerOwned(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		got := PackageCapability(raw)
		gotErr := got.Validate()
		wantValid := got == PackageCapabilityProcessExecution
		if (gotErr == nil) != wantValid {
			t.Fatalf("PackageCapability(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, ErrPrimitiveContract) || got.String() != UnknownEnumDiagnostic {
				t.Fatalf("PackageCapability(%d) = (%q, %v), want (%q, errors.Is(..., %v))",
					raw, got.String(), gotErr, UnknownEnumDiagnostic, ErrPrimitiveContract)
			}
			continue
		}
		got.OffWireEnum()
		if got.String() != packageCapabilityProcessExecutionName {
			t.Fatalf("PackageCapabilityProcessExecution.String() = %q, want %q", got.String(), packageCapabilityProcessExecutionName)
		}
	}
}

func consumerReferences(name string, source []byte) ([]consumerReference, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, errors.Join(ErrPrimitiveContract, err)
	}
	var values []string
	lower := strings.ToLower(string(source))
	for _, forbidden := range unambiguousConsumerNames() {
		if strings.Contains(lower, forbidden) {
			values = append(values, forbidden)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, unquoteErr := strconv.Unquote(literal.Value)
		if unquoteErr == nil && (strings.EqualFold(value, "witness") || strings.EqualFold(value, "bug")) {
			values = append(values, strings.ToLower(value))
		}
		return true
	})
	slices.Sort(values)
	values = slices.Compact(values)
	references := make([]consumerReference, len(values))
	for index, value := range values {
		references[index] = consumerReference{file: name, value: value}
	}
	return references, nil
}
