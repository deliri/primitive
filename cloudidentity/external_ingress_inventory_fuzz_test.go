package cloudidentity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// cloudIdentityExternalDoorInventory binds every raw caller or provider door
// to its real public entry point. New Parse, Acquire, or signed-AWS
// constructors are found by the AST ratchet below and must enter this struct.
type cloudIdentityExternalDoorInventory struct {
	AcquireAmazonWebServices      func(context.Context, Client, AmazonWebServicesRequest) (Token, error)
	AcquireGoogleCloud            func(context.Context, Client, Request) (Token, error)
	NewAmazonWebServicesRequest   func(AmazonWebServicesRequestInput) (AmazonWebServicesRequest, error)
	ParseAudience                 func(string) (Audience, error)
	ParseGoogleCloudCommandOutput func([]byte) (Token, error)
}

var cloudIdentityExternalDoors = cloudIdentityExternalDoorInventory{
	AcquireAmazonWebServices:      AcquireAmazonWebServices,
	AcquireGoogleCloud:            AcquireGoogleCloud,
	NewAmazonWebServicesRequest:   NewAmazonWebServicesRequest,
	ParseAudience:                 ParseAudience,
	ParseGoogleCloudCommandOutput: ParseGoogleCloudCommandOutput,
}

func FuzzParseGoogleCloudCommandOutput(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		{},
		[]byte("a"),
		[]byte(testIdentityToken),
		[]byte(testIdentityToken + "\n"),
		[]byte(testIdentityToken + "\r\n"),
		[]byte("=leading-padding"),
		[]byte("embedded\x00byte"),
		bytes.Repeat([]byte{'a'}, TokenMaximumBytes),
		bytes.Repeat([]byte{'a'}, TokenMaximumBytes+1),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, output []byte) {
		got, gotErr := cloudIdentityExternalDoors.ParseGoogleCloudCommandOutput(output)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudIdentityContract) {
				t.Fatalf("ParseGoogleCloudCommandOutput() error = %v, want %v", gotErr, core.ErrCloudIdentityContract)
			}
			if got != (Token{}) {
				t.Fatalf("ParseGoogleCloudCommandOutput() token = %#v, want zero", got)
			}
			return
		}
		proveGoogleTokenClosure(t, got, commandOutputTokenBytes(output))
	})
}

func commandOutputTokenBytes(output []byte) []byte {
	value := output
	if bytes.HasSuffix(value, []byte{'\n'}) {
		value = value[:len(value)-1]
		if bytes.HasSuffix(value, []byte{'\r'}) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func proveGoogleTokenClosure(t testing.TB, got Token, want []byte) {
	t.Helper()

	if got.Provider() != ProviderGoogleCloud {
		t.Fatalf("Token.Provider() = %v, want %v", got.Provider(), ProviderGoogleCloud)
	}
	if gotErr := got.Validate(); gotErr != nil {
		t.Fatalf("Token.Validate() error = %v, want nil", gotErr)
	}
	gotBearer, gotErr := got.BearerValue()
	wantBearer := bearerPrefix + string(want)
	if gotErr != nil || gotBearer != wantBearer {
		t.Fatalf("Token.BearerValue() = (%q, %v), want (%q, nil)", gotBearer, gotErr, wantBearer)
	}
	if gotFormat := fmt.Sprintf("%v", got); gotFormat != core.RedactedValueText {
		t.Fatalf("formatted Token = %q, want %q", gotFormat, core.RedactedValueText)
	}
	roundTrip, gotErr := cloudIdentityExternalDoors.ParseGoogleCloudCommandOutput(want)
	if gotErr != nil {
		t.Fatalf("canonical ParseGoogleCloudCommandOutput() error = %v, want nil", gotErr)
	}
	roundTripBearer, gotErr := roundTrip.BearerValue()
	if gotErr != nil || roundTripBearer != gotBearer {
		t.Fatalf("canonical token round trip = (%q, %v), want (%q, nil)", roundTripBearer, gotErr, gotBearer)
	}
}

func TestCloudIdentityExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, gotErr := scanCloudIdentityExternalDoors(".")
	if gotErr != nil {
		t.Fatalf("scanCloudIdentityExternalDoors() error = %v, want nil", gotErr)
	}
	want := cloudIdentityExternalDoorFieldNames(cloudIdentityExternalDoors)
	if !slices.Equal(got, want) {
		t.Fatalf("Cloudidentity external doors = %q, want compiler inventory %q", got, want)
	}
}

func cloudIdentityExternalDoorFieldNames(inventory any) []string {
	typeOf := reflect.TypeOf(inventory)
	fields := make([]string, 0, typeOf.NumField())
	for index := range typeOf.NumField() {
		fields = append(fields, typeOf.Field(index).Name)
	}
	slices.Sort(fields)
	return fields
}

func scanCloudIdentityExternalDoors(root string) ([]string, error) {
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
			if !ok || declaration.Recv != nil {
				return true
			}
			name := declaration.Name.Name
			if strings.HasPrefix(name, "Parse") || strings.HasPrefix(name, "Acquire") ||
				name == "NewAmazonWebServicesRequest" {
				doors = append(doors, name)
			}
			return false
		})
	}
	slices.Sort(doors)
	return doors, nil
}
