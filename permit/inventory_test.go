package permit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Inventory is a wiring proof only; behavioral tests own permission evidence.
func TestPermitProductionStructInventory(t *testing.T) {
	t.Parallel()
	roles := []struct {
		name, role string
		found      bool
	}{
		{"RegistrationResponse", "protocol fact: enrollment and permission bound to one response", false},
		{"CheckInResponse", "protocol fact: usage response and permission bound to one response", false},
		{"Action", "protocol fact: opaque canonical action identity", false},
		{"Actions", "protocol fact: immutable bounded action set", false},
		{"Terms", "protocol fact: exact server-issued opaque action and validity terms", false},
		{"Document", "protocol fact: terms bound to Primitive attestation", false},
		{"VerifyRequest", "internal flow: independently supplied verification context", false},
		{"Verified", "capability wrapper: authenticated terms for one verification instant", false},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir = %v, want nil", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s = %v, want nil", entry.Name(), err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok {
				return true
			}
			found := false
			for i := range roles {
				if roles[i].name == spec.Name.Name && roles[i].role != "" {
					found, roles[i].found = true, true
				}
			}
			if !found {
				t.Errorf("production struct %s classified = false, want intentional role", spec.Name.Name)
			}
			return true
		})
	}
	for _, role := range roles {
		if !role.found {
			t.Errorf("inventory struct %s declaration = absent, want present", role.name)
		}
	}
}

// Each external representation boundary names its semantic fuzz oracle.
func TestPermitExternalDoorInventory(t *testing.T) {
	t.Parallel()
	doors := []struct{ name, file, target string }{
		{"ParseAction", "action_test.go", "FuzzParseActionSemanticClosure"},
		{"Action.UnmarshalJSON", "action_test.go", "FuzzActionSemanticClosure"},
		{"Actions.UnmarshalJSON", "action_test.go", "FuzzActionsSemanticClosure"},
		{"Decode", "document_test.go", "FuzzPermitDecodeSignedSemanticClosure"},
		{"Domain.ParseCanonicalText", "document_test.go", "FuzzPermitDomainText"},
		{"Revision.UnmarshalJSON", "document_test.go", "FuzzPermitRevisionJSON"},
		{"RegistrationResponse.UnmarshalJSON", "response_fuzz_test.go", "FuzzRegistrationResponseSemanticClosure"},
		{"CheckInResponse.UnmarshalJSON", "response_fuzz_test.go", "FuzzCheckInResponseSemanticClosure"},
	}
	for _, door := range doors {
		found := false
		file, err := parser.ParseFile(token.NewFileSet(), door.file, nil, 0)
		if err != nil {
			t.Fatalf("parse tests = %v, want nil", err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == door.target {
				found = true
			}
		}
		if !found {
			t.Errorf("external door %s oracle %s found = false, want true", door.name, door.target)
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir = %v, want nil", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s = %v, want nil", entry.Name(), err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !externalRepresentationDoor(fn.Name.Name) {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil {
				receiver := fn.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if identifier, ok := receiver.(*ast.Ident); ok {
					name = identifier.Name + "." + name
				}
			}
			found := false
			for _, door := range doors {
				found = found || door.name == name
			}
			if !found {
				t.Errorf("external door %s fuzz inventory = absent, want named semantic oracle", name)
			}
		}
	}

}

// Only conventional public representation ingress names are discovered here.
// Constructors admitting external material under other names need review.
func externalRepresentationDoor(name string) bool {
	for _, prefix := range []string{"Parse", "Decode", "Read", "Load", "Replay", "Unmarshal"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func TestExternalRepresentationDoorMatcher(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"Decode", true}, {"ParseCanonicalText", true}, {"UnmarshalJSON", true},
		{"ReadDocument", true}, {"LoadDocument", true}, {"Replay", true},
		{"MarshalJSON", false}, {"Validate", false}, {"decodeInternal", false},
	} {
		got := externalRepresentationDoor(tc.name)
		if got != tc.want {
			t.Errorf("externalRepresentationDoor(%q) = %t, want %t", tc.name, got, tc.want)
		}
	}
}
