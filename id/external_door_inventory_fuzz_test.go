package id

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// idExternalDoorInventory makes every external parser a compiled selector.
// The field names are the canonical AST door names used by the drift ratchet.
type idExternalDoorInventory struct {
	ParseULID           func(string) (ULID, error)
	ParseUUIDv7         func(string) (UUIDv7, error)
	ULIDUnmarshalJSON   func(*ULID, []byte) error
	UUIDv7UnmarshalJSON func(*UUIDv7, []byte) error
}

var idExternalDoors = idExternalDoorInventory{
	ParseULID:           ParseULID,
	ParseUUIDv7:         ParseUUIDv7,
	ULIDUnmarshalJSON:   (*ULID).UnmarshalJSON,
	UUIDv7UnmarshalJSON: (*UUIDv7).UnmarshalJSON,
}

type idJSONDoor uint8

const (
	idJSONDoorUnknown idJSONDoor = iota
	idJSONDoorUUIDv7
	idJSONDoorULID
	idJSONDoorLimit
)

func FuzzIDExternalJSONDoorInventory(f *testing.F) {
	uuid := mustParsedUUIDv7ForFuzz(f, "00000000-0001-7000-8000-000000000001")
	ulid := mustParsedULIDForFuzz(f, "01ARZ3NDEKTSV4RRFFQ69G5FAV")
	f.Add(uint8(idJSONDoorUUIDv7), mustIDJSONForFuzz(f, uuid.MarshalJSON))
	f.Add(uint8(idJSONDoorULID), mustIDJSONForFuzz(f, ulid.MarshalJSON))
	for _, hostile := range [][]byte{
		nil,
		{},
		[]byte(`null`),
		[]byte(`""`),
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`0`),
		[]byte(`true`),
		[]byte(`"00000000-0001-6000-8000-000000000001"`),
		[]byte(`"8ZZZZZZZZZZZZZZZZZZZZZZZZZ"`),
	} {
		f.Add(uint8(idJSONDoorUUIDv7), hostile)
		f.Add(uint8(idJSONDoorULID), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch idJSONDoor(rawDoor) {
		case idJSONDoorUUIDv7:
			fuzzUUIDv7JSONDoor(t, data, uuid)
		case idJSONDoorULID:
			fuzzULIDJSONDoor(t, data, ulid)
		case idJSONDoorUnknown, idJSONDoorLimit:
			return
		default:
			return
		}
	})
}

func fuzzUUIDv7JSONDoor(t testing.TB, data []byte, survivor UUIDv7) {
	t.Helper()

	got := survivor
	gotErr := idExternalDoors.UUIDv7UnmarshalJSON(&got, data)
	if gotErr != nil {
		proveIDJSONRefusal(t, gotErr, got == survivor)
		return
	}
	if gotErr := got.Validate(); gotErr != nil {
		t.Fatalf("UUIDv7.UnmarshalJSON(accepted).Validate() error = %v, want nil", gotErr)
	}
	encoded, gotErr := got.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("UUIDv7.MarshalJSON() error = %v, want nil", gotErr)
	}
	var roundTrip UUIDv7
	if gotErr := idExternalDoors.UUIDv7UnmarshalJSON(&roundTrip, encoded); gotErr != nil || roundTrip != got {
		t.Fatalf("UUIDv7 canonical round trip = (%v, %v), want (%v, nil)", roundTrip, gotErr, got)
	}
	proveIDJSONFixedPoint(t, encoded, roundTrip.MarshalJSON)
}

func fuzzULIDJSONDoor(t testing.TB, data []byte, survivor ULID) {
	t.Helper()

	got := survivor
	gotErr := idExternalDoors.ULIDUnmarshalJSON(&got, data)
	if gotErr != nil {
		proveIDJSONRefusal(t, gotErr, got == survivor)
		return
	}
	if gotErr := got.Validate(); gotErr != nil {
		t.Fatalf("ULID.UnmarshalJSON(accepted).Validate() error = %v, want nil", gotErr)
	}
	encoded, gotErr := got.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("ULID.MarshalJSON() error = %v, want nil", gotErr)
	}
	var roundTrip ULID
	if gotErr := idExternalDoors.ULIDUnmarshalJSON(&roundTrip, encoded); gotErr != nil || roundTrip != got {
		t.Fatalf("ULID canonical round trip = (%v, %v), want (%v, nil)", roundTrip, gotErr, got)
	}
	proveIDJSONFixedPoint(t, encoded, roundTrip.MarshalJSON)
}

func proveIDJSONRefusal(t testing.TB, gotErr error, preserved bool) {
	t.Helper()

	if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrIDContract) {
		t.Fatalf("ID JSON refusal error = %v, want %v and %v", gotErr, core.ErrJSONContract, core.ErrIDContract)
	}
	if !preserved {
		t.Fatalf("ID JSON receiver preserved = %t, want true", preserved)
	}
}

func proveIDJSONFixedPoint(
	t testing.TB,
	want []byte,
	marshal func() ([]byte, error),
) {
	t.Helper()

	got, gotErr := marshal()
	if gotErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("ID second canonical projection = (%q, %v), want (%q, nil)", got, gotErr, want)
	}
}

func mustParsedUUIDv7ForFuzz(t testing.TB, value string) UUIDv7 {
	t.Helper()

	got, gotErr := idExternalDoors.ParseUUIDv7(value)
	if gotErr != nil {
		t.Fatalf("ParseUUIDv7() setup error = %v, want nil", gotErr)
	}
	return got
}

func mustParsedULIDForFuzz(t testing.TB, value string) ULID {
	t.Helper()

	got, gotErr := idExternalDoors.ParseULID(value)
	if gotErr != nil {
		t.Fatalf("ParseULID() setup error = %v, want nil", gotErr)
	}
	return got
}

func mustIDJSONForFuzz(t testing.TB, marshal func() ([]byte, error)) []byte {
	t.Helper()

	got, gotErr := marshal()
	if gotErr != nil {
		t.Fatalf("ID MarshalJSON() setup error = %v, want nil", gotErr)
	}
	return got
}

func TestIDExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, gotErr := scanIDExternalDoors(".")
	if gotErr != nil {
		t.Fatalf("scanIDExternalDoors() error = %v, want nil", gotErr)
	}
	want := idExternalDoorFieldNames(idExternalDoors)
	if !slices.Equal(got, want) {
		t.Fatalf("ID external doors = %q, want compiler inventory %q", got, want)
	}
}

func idExternalDoorFieldNames(inventory any) []string {
	typeOf := reflect.TypeOf(inventory)
	fields := make([]string, 0, typeOf.NumField())
	for index := range typeOf.NumField() {
		fields = append(fields, typeOf.Field(index).Name)
	}
	slices.Sort(fields)
	return fields
}

func scanIDExternalDoors(root string) ([]string, error) {
	set := token.NewFileSet()
	entries, gotErr := os.ReadDir(root)
	if gotErr != nil {
		return nil, gotErr
	}
	var doors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			set,
			filepath.Join(root, entry.Name()),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok {
				return true
			}
			if declaration.Recv == nil &&
				(declaration.Name.Name == "ParseULID" || declaration.Name.Name == "ParseUUIDv7") {
				doors = append(doors, declaration.Name.Name)
				return false
			}
			if declaration.Recv == nil || declaration.Name.Name != "UnmarshalJSON" {
				return true
			}
			receiver := idReceiverName(declaration.Recv.List[0].Type)
			if receiver == "ULID" || receiver == "UUIDv7" {
				doors = append(doors, receiver+declaration.Name.Name)
			}
			return false
		})
	}
	slices.Sort(doors)
	return doors, nil
}
