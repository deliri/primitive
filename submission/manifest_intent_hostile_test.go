package submission

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
)

func TestUploadIDCanonicalUUIDv7BoundaryTable(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name  string
		value string
	}{
		{name: "minimum nonzero timestamp and payload", value: "00000000-0001-7000-8000-000000000001"},
		{name: "next timestamp tick", value: "00000000-0002-7000-8000-000000000002"},
		{name: "low version payload boundary", value: "00000000-0010-7000-8000-000000000010"},
		{name: "timestamp crosses low word", value: "00000001-0000-7000-8000-000000000001"},
		{name: "timestamp crosses hexadecimal digit", value: "00000010-0000-7000-8000-000000000010"},
		{name: "timestamp crosses second hexadecimal digit", value: "00000100-0000-7000-8000-000000000100"},
		{name: "timestamp crosses third hexadecimal digit", value: "00001000-0000-7000-8000-000000001000"},
		{name: "timestamp crosses fourth hexadecimal digit", value: "00010000-0000-7000-8000-000000010000"},
		{name: "timestamp crosses fifth hexadecimal digit", value: "00100000-0000-7000-8000-000000100000"},
		{name: "maximum canonical uuidv7 fixture", value: "7fffffff-ffff-7fff-bfff-ffffffffffff"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseUploadID(tc.value)
			if gotErr != nil || got.String() != tc.value || got.Validate() != nil {
				t.Fatalf("ParseUploadID(%q) = (%q, %v), want exact valid identity", tc.value, got.String(), gotErr)
			}
			encoded, gotErr := got.MarshalJSON()
			if gotErr != nil {
				t.Fatalf("UploadID.MarshalJSON() error = %v, want nil", gotErr)
			}
			var roundTrip UploadID
			gotErr = roundTrip.UnmarshalJSON(encoded)
			if gotErr != nil || roundTrip != got {
				t.Fatalf("UploadID.UnmarshalJSON() = (%v, %v), want (%v, nil)", roundTrip, gotErr, got)
			}
		})
	}

	invalid := []struct {
		name  string
		value string
	}{
		{name: "empty identity", value: ""},
		{name: "version one below seven", value: "00000000-0001-6000-8000-000000000001"},
		{name: "variant one above rfc4122", value: "00000000-0001-7000-c000-000000000001"},
		{name: "variant one below rfc4122", value: "00000000-0001-7000-0000-000000000001"},
		{name: "nonhex final byte", value: "00000000-0001-7000-8000-00000000000g"},
		{name: "hyphens absent", value: "00000000000170008000000000000001"},
		{name: "braced spelling", value: "{00000000-0001-7000-8000-000000000001}"},
		{name: "trailing whitespace", value: "00000000-0001-7000-8000-000000000001 "},
		{name: "one byte truncated", value: "00000000-0001-7000-8000-00000000001"},
		{name: "uppercase is noncanonical", value: strings.ToUpper(valid[9].value)},
		{name: "one extra final byte", value: "00000000-0001-7000-8000-0000000000010"},
		{name: "future version nibble", value: "00000000-0001-8000-8000-000000000001"},
	}
	preserved, gotErr := ParseUploadID(valid[0].value)
	if gotErr != nil {
		t.Fatalf("ParseUploadID(preserved setup) error = %v, want nil", gotErr)
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseUploadID(tc.value)
			if !errors.Is(gotErr, core.ErrControlPlaneContract) || got != (UploadID{}) {
				t.Fatalf("ParseUploadID(%q) = (%v, %v), want zero and errors.Is %v", tc.value, got, gotErr, core.ErrControlPlaneContract)
			}
			encoded, encodeErr := core.MarshalCanonicalJSONString(tc.value)
			if encodeErr != nil {
				t.Fatalf("core.MarshalCanonicalJSONString() setup error = %v, want nil", encodeErr)
			}
			got = preserved
			gotErr = got.UnmarshalJSON(encoded)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != preserved {
				t.Fatalf("UploadID.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, gotErr, core.ErrJSONContract)
			}
		})
	}
}

type manifestIntentCase struct {
	mutate   func(*ManifestIntent)
	name     string
	objects  uint64
	sequence uint64
	wantErr  error
}

func TestManifestIntentSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact sequence and object boundaries validate", func(t *testing.T) {
		t.Parallel()

		cases := []manifestIntentCase{
			{name: "one object first entry", sequence: 1, objects: 1},
			{name: "two objects first entry", sequence: 1, objects: 2},
			{name: "two objects final entry", sequence: 2, objects: 2},
			{name: "three objects first entry", sequence: 1, objects: 3},
			{name: "three objects middle entry", sequence: 2, objects: 3},
			{name: "three objects final entry", sequence: 3, objects: 3},
			{name: "ten objects one below midpoint", sequence: 4, objects: 10},
			{name: "ten objects at midpoint", sequence: 5, objects: 10},
			{name: "ten objects one above midpoint", sequence: 6, objects: 10},
			{name: "maximum sequence equals maximum count", sequence: ^uint64(0), objects: ^uint64(0)},
			{name: "one below maximum sequence", sequence: ^uint64(0) - 1, objects: ^uint64(0)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := testManifestIntent(t)
				got.Sequence = manifestSequence(t, tc.sequence)
				got.Objects = manifestObjects(t, tc.objects)
				if gotErr := got.Validate(); gotErr != nil {
					t.Fatalf("ManifestIntent.Validate(sequence %d of %d) error = %v, want nil", tc.sequence, tc.objects, gotErr)
				}
			})
		}
	})

	t.Run("negative missing and contradictory organization facts reject", func(t *testing.T) {
		t.Parallel()

		cases := []manifestIntentCase{
			{name: "zero intent", mutate: func(value *ManifestIntent) { *value = ManifestIntent{} }, wantErr: core.ErrControlPlaneContract},
			{name: "upload identity absent", mutate: func(value *ManifestIntent) { value.Upload = UploadID{} }, wantErr: core.ErrControlPlaneContract},
			{name: "collection identity absent", mutate: func(value *ManifestIntent) { value.Collection = chit.CollectionID{} }, wantErr: core.ErrControlPlaneContract},
			{name: "portable entry name absent", mutate: func(value *ManifestIntent) { value.Name = chit.EntryName{} }, wantErr: core.ErrControlPlaneContract},
			{name: "entry sequence absent", mutate: func(value *ManifestIntent) { value.Sequence = chit.EntrySequence{} }, wantErr: core.ErrControlPlaneContract},
			{name: "object count absent", mutate: func(value *ManifestIntent) { value.Objects = chit.ObjectCount{} }, wantErr: core.ErrControlPlaneContract},
			{name: "sequence one above one-object count", sequence: 2, objects: 1, wantErr: core.ErrControlPlaneContract},
			{name: "sequence one above ten-object count", sequence: 11, objects: 10, wantErr: core.ErrControlPlaneContract},
			{name: "sequence far above object count", sequence: 255, objects: 1, wantErr: core.ErrControlPlaneContract},
			{name: "maximum sequence above one-below-maximum count", sequence: ^uint64(0), objects: ^uint64(0) - 1, wantErr: core.ErrControlPlaneContract},
			{name: "sequence above midpoint by count", sequence: 6, objects: 5, wantErr: core.ErrControlPlaneContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := testManifestIntent(t)
				if tc.mutate != nil {
					tc.mutate(&got)
				} else {
					got.Sequence = manifestSequence(t, tc.sequence)
					got.Objects = manifestObjects(t, tc.objects)
				}
				gotErr := got.Validate()
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ManifestIntent.Validate() error = %v, want errors.Is %v", gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero intent emits no plausible json organization", func(t *testing.T) {
		t.Parallel()

		got, gotErr := json.Marshal(ManifestIntent{})
		if !errors.Is(gotErr, core.ErrJSONContract) || got != nil {
			t.Fatalf("json.Marshal(zero ManifestIntent) = (%q, %v), want nil and errors.Is %v", got, gotErr, core.ErrJSONContract)
		}
	})
}

func TestManifestIntentStructuralInvariantCarriesOnlyBlindOrganizationFacts(t *testing.T) {
	t.Parallel()

	type fieldContract struct {
		name   string
		typeOf reflect.Type
	}
	want := []fieldContract{
		{name: "Name", typeOf: reflect.TypeFor[chit.EntryName]()},
		{name: "Sequence", typeOf: reflect.TypeFor[chit.EntrySequence]()},
		{name: "Objects", typeOf: reflect.TypeFor[chit.ObjectCount]()},
		{name: "Collection", typeOf: reflect.TypeFor[chit.CollectionID]()},
		{name: "Upload", typeOf: reflect.TypeFor[UploadID]()},
	}
	got := reflect.TypeFor[ManifestIntent]()
	if got.NumField() != len(want) {
		t.Fatalf("ManifestIntent field count = %d, want exactly %d blind organization facts", got.NumField(), len(want))
	}
	for index, contract := range want {
		field := got.Field(index)
		if field.Name != contract.name || field.Type != contract.typeOf {
			t.Fatalf("ManifestIntent field %d = (%s, %v), want (%s, %v)", index, field.Name, field.Type, contract.name, contract.typeOf)
		}
	}
}

func manifestSequence(t *testing.T, value uint64) chit.EntrySequence {
	t.Helper()
	sequence, gotErr := chit.NewEntrySequence(value)
	if gotErr != nil {
		t.Fatalf("chit.NewEntrySequence(%d) error = %v, want nil", value, gotErr)
	}
	return sequence
}

func manifestObjects(t *testing.T, value uint64) chit.ObjectCount {
	t.Helper()
	objects, gotErr := chit.NewObjectCount(value)
	if gotErr != nil {
		t.Fatalf("chit.NewObjectCount(%d) error = %v, want nil", value, gotErr)
	}
	return objects
}
