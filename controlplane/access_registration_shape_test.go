package controlplane

import (
	"encoding/json/jsontext"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
)

// Runtime execution cannot prove a future public field was also added to the
// strict decoder's delayed-secret projection. Pin that compiler-visible shape.
func TestAccessRegistrationDecoderProjectionMatchesPublicAgreement(t *testing.T) {
	t.Parallel()
	public, wire := reflect.TypeFor[AccessRegistrationRequest](), reflect.TypeFor[accessRegistrationRequestWire]()
	if public.NumField() != wire.NumField() {
		t.Fatalf("public/wire field counts = %d/%d, want equal", public.NumField(), wire.NumField())
	}
	for i := range public.NumField() {
		field := public.Field(i)
		got, ok := wire.FieldByName(field.Name)
		if !ok {
			t.Errorf("decoder field %s = absent, want public agreement field", field.Name)
			continue
		}
		wantType := field.Type
		if field.Name == "Token" {
			if field.Type != reflect.TypeFor[controlwire.AccessToken]() {
				t.Fatalf("public token type = %v, want AccessToken", field.Type)
			}
			wantType = reflect.TypeFor[jsontext.Value]()
		}
		if got.Type != wantType || got.Tag != field.Tag {
			t.Errorf("decoder field %s type/tag = %v/%s, want %v/%s", field.Name, got.Type, got.Tag, wantType, field.Tag)
		}
	}
}
