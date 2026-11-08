package timematch

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppointmentLifetimeJSONPreservesClosedDomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value Lifetime
		want  string
	}{
		{name: "one-shot visitor keeps its original wire identity", value: LifetimeVisitor, want: "visitor"},
		{name: "ongoing business keeps its original wire identity", value: LifetimeBusiness, want: "business"},
		{name: "appointment has its own roundtrip wire identity", value: LifetimeAppointment, want: "appointment"},
		{name: "zero lifetime has no wire authority"},
		{name: "future lifetime has no wire authority", value: LifetimeAppointment + 1},
		{name: "maximum lifetime has no wire authority", value: Lifetime(255)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := tc.value.MarshalJSON()
			if tc.want == "" {
				if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrTimeMatchContract) || encoded != nil || tc.value.IsValid() || tc.value.String() != "" {
					t.Fatalf("refused lifetime encoding/name/error = %q/%q/%v, want no wire authority and typed contract refusal", encoded, tc.value.String(), err)
				}
				return
			}
			want, wantErr := core.MarshalCanonicalJSONString(tc.want)
			if wantErr != nil || err != nil || !bytes.Equal(encoded, want) || tc.value.String() != tc.want || !tc.value.IsValid() {
				t.Fatalf("lifetime encoding/name/error = %q/%q/%v, want %q/%q/nil (fixture error %v)", encoded, tc.value.String(), err, want, tc.want, wantErr)
			}
			var got Lifetime
			if err := got.UnmarshalJSON(encoded); err != nil || got != tc.value {
				t.Fatalf("lifetime roundtrip/error = %v/%v, want %v/nil", got, err, tc.value)
			}
		})
	}
}

func FuzzAppointmentLifetimeJSON(f *testing.F) {
	for _, lifetime := range []Lifetime{LifetimeVisitor, LifetimeBusiness, LifetimeAppointment} {
		canonical, err := lifetime.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(canonical)
	}
	for _, raw := range []string{"", "null", "3", "true", `"unknown"`, `"appointment" {}`, `"Appointment"`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got := LifetimeAppointment
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrTimeMatchContract) || !errors.Is(err, core.ErrJSONContract) || got != LifetimeAppointment {
				t.Fatalf("lifetime refused decode/error = %v/%v, want preserved appointment and typed JSON/contract refusal", got, err)
			}
			return
		}
		if !got.IsValid() || got.Validate() != nil || got.String() == "" {
			t.Fatalf("accepted lifetime = %v, want a named closed-domain value", got)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("accepted lifetime encoding error = %v, want nil", err)
		}
		var round Lifetime
		if err := round.UnmarshalJSON(encoded); err != nil || round != got {
			t.Fatalf("accepted lifetime roundtrip/error = %v/%v, want %v/nil", round, err, got)
		}
		second, err := round.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("accepted lifetime canonical bytes/error = %q/%v, want %q/nil", second, err, encoded)
		}
	})
}
