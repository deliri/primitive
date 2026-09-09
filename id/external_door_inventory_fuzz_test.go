package id

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The inventory compiles actual signatures, including the typed constructors.
type idExternalDoorInventory struct {
	NewULID             func(Request) (ULID, error)
	NewULIDFromBytes    func([identityBytes]byte) (ULID, error)
	NewUUIDv7           func(Request) (UUIDv7, error)
	ParseULID           func(string) (ULID, error)
	ParseUUIDv7         func(string) (UUIDv7, error)
	ULIDUnmarshalJSON   func(*ULID, []byte) error
	UUIDv7UnmarshalJSON func(*UUIDv7, []byte) error
}

var _ = idExternalDoorInventory{NewULID, NewULIDFromBytes, NewUUIDv7, ParseULID, ParseUUIDv7, (*ULID).UnmarshalJSON, (*UUIDv7).UnmarshalJSON}

func idExternalDoorName(function *ast.FuncDecl) string {
	if !function.Name.IsExported() {
		return ""
	}
	if function.Recv == nil {
		for _, prefix := range []string{"New", "Parse", "Decode"} {
			if strings.HasPrefix(function.Name.Name, prefix) {
				return function.Name.Name
			}
		}
		return ""
	}
	if strings.HasPrefix(function.Name.Name, "Unmarshal") {
		return idReceiverName(function.Recv.List[0].Type) + function.Name.Name
	}
	return ""
}

func TestIDExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	names, err := idProductionGoFiles()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, name := range names {
		source, err := idSources.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				if name := idExternalDoorName(function); name != "" {
					got = append(got, name)
				}
			}
		}
	}
	var want []string
	for field := range reflect.TypeFor[idExternalDoorInventory]().Fields() {
		want = append(want, field.Name)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("ingress = %q, compiler inventory = %q", got, want)
	}
}

func TestIDExternalDoorsHaveSemanticFuzzOwners(t *testing.T) {
	t.Parallel()
	cases := []struct {
		door  string
		owner func(*testing.F)
	}{
		{door: "NewULID", owner: FuzzIdentityRequest},
		{door: "NewUUIDv7", owner: FuzzIdentityRequest},
		{door: "NewULIDFromBytes", owner: FuzzULIDFromBytes},
		{door: "ParseULID", owner: FuzzParseULID},
		{door: "ParseUUIDv7", owner: FuzzParseUUIDv7},
		{door: "ULIDUnmarshalJSON", owner: FuzzULIDJSON},
		{door: "UUIDv7UnmarshalJSON", owner: FuzzUUIDv7JSON},
	}
	if len(cases) != reflect.TypeFor[idExternalDoorInventory]().NumField() {
		t.Fatalf("fuzz owners=%d, want %d compiled ingress fields", len(cases), reflect.TypeFor[idExternalDoorInventory]().NumField())
	}
	for _, tc := range cases {
		t.Run(tc.door, func(t *testing.T) {
			t.Parallel()
			if _, ok := reflect.TypeFor[idExternalDoorInventory]().FieldByName(tc.door); !ok || tc.owner == nil {
				t.Fatalf("fuzz mapping %q: field present=%t owner present=%t, want both true", tc.door, ok, tc.owner != nil)
			}
		})
	}
}

func TestIDExternalDoorMatcherAttacksNewIngress(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, source, want string }{
		{"constructor", "package p; func NewOther(){}", "NewOther"},
		{"parser", "package p; func ParseOther(){}", "ParseOther"},
		{"decoder", "package p; func DecodeOther(){}", "DecodeOther"},
		{"unmarshal", "package p; func (*Other) UnmarshalText(){}", "OtherUnmarshalText"},
		{"private", "package p; func parseOther(){}", ""},
		{"projection", "package p; func (Other) String(){}", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got := idExternalDoorName(file.Decls[0].(*ast.FuncDecl))
			if got != tc.want {
				t.Fatalf("door=%q, want %q", got, tc.want)
			}
		})
	}
}
