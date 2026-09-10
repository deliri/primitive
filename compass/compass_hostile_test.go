package compass_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

type compassConfigurationCase struct {
	name    string
	data    []byte
	want    compass.Configuration
	wantErr error
}

func validConfigurationCase(t testing.TB, name string, project compass.Project) compassConfigurationCase {
	t.Helper()
	return compassConfigurationCase{name: name, data: encodedConfiguration(t, project), want: compass.Configuration{Project: project}}
}
func TestDecodeHostileProjectConfigurationLayerTriad(t *testing.T) {
	t.Parallel()
	base := projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, 3)
	canonical := encodedConfiguration(t, base)
	cases := []compassConfigurationCase{
		validConfigurationCase(t, "canonical_exact_fields", base),
		validConfigurationCase(t, "unicode_name", projectFixture(t, "Évidence", "example.com/project", "owner/project", 2026, 1, 3)),
		validConfigurationCase(t, "internal_space", projectFixture(t, "Evidence Tool", "example.com/project", "owner/project", 2026, 1, 3)),
		validConfigurationCase(t, "versioned_module", projectFixture(t, "Project", "example.com/project/v2", "owner/project", 2026, 1, 3)),
		validConfigurationCase(t, "opaque_repository", projectFixture(t, "Project", "example.com/project", "ssh://git.example/project", 2026, 1, 3)),
		validConfigurationCase(t, "minimum_major_zero_minor_patch", projectFixture(t, "Project", "example.com/project", "owner/project", 1, 0, 0)),
		validConfigurationCase(t, "maximum_uint32_coordinates", projectFixture(t, "Project", "example.com/project", "owner/project", math.MaxUint32, math.MaxUint32, math.MaxUint32)),
		{name: "legal_outer_whitespace", data: append(append([]byte(" \n\t"), canonical...), []byte("\r\n")...), want: compass.Configuration{Project: base}},
		{name: "padded_name", data: hostileConfiguration(" Project", "example.com/project", "owner/project", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "control_in_name", data: hostileConfiguration("Project\\u000aName", "example.com/project", "owner/project", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "missing_name", data: hostileConfiguration("", "example.com/project", "owner/project", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "invalid_module", data: hostileConfiguration("Project", "example.com/bad module", "owner/project", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "missing_module", data: hostileConfiguration("Project", "", "owner/project", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "invalid_repository", data: hostileConfiguration("Project", "example.com/project", "owner bad/project", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "missing_repository", data: hostileConfiguration("Project", "example.com/project", "", "2026", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "negative_major", data: hostileConfiguration("Project", "example.com/project", "owner/project", "-1", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "zero_major", data: hostileConfiguration("Project", "example.com/project", "owner/project", "0", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "major_overflow", data: hostileConfiguration("Project", "example.com/project", "owner/project", "4294967296", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "major_fraction", data: hostileConfiguration("Project", "example.com/project", "owner/project", "2026.5", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "major_string", data: hostileConfiguration("Project", "example.com/project", "owner/project", "\"2026\"", "1", "3"), wantErr: core.ErrJSONContract},
		{name: "duplicate_root", data: append(bytes.Clone(canonical[:len(canonical)-1]), []byte(",\"project\":null}")...), wantErr: core.ErrJSONContract},
		{name: "case_fold_duplicate_root", data: append(bytes.Clone(canonical[:len(canonical)-1]), []byte(",\"Project\":null}")...), wantErr: core.ErrJSONContract},
		{name: "unknown_root", data: append(bytes.Clone(canonical[:len(canonical)-1]), []byte(",\"unknown\":true}")...), wantErr: core.ErrJSONContract},
		{name: "missing_project", data: []byte("{}"), wantErr: core.ErrJSONContract},
		{name: "null_project", data: []byte("{\"project\":null}"), wantErr: core.ErrJSONContract},
		{name: "neutral_empty", wantErr: core.ErrJSONContract},
		{name: "null_root", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "array_root", data: []byte("[]"), wantErr: core.ErrJSONContract},
		{name: "truncated_object", data: canonical[:len(canonical)-1], wantErr: core.ErrJSONContract},
		{name: "second_document", data: append(bytes.Clone(canonical), canonical...), wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := compass.Decode[compass.Configuration](bytes.NewReader(tc.data))
			if !errors.Is(err, tc.wantErr) || got != tc.want || tc.wantErr != nil && !errors.Is(err, core.ErrCompassContract) {
				t.Fatalf("got configuration=%v error=%v, want %v and %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func FuzzDecodeProjectConfigurationSemanticClosure(f *testing.F) {
	want := compass.Configuration{Project: projectFixture(f, "Project", "example.com/project", "owner/project", 2026, 1, 3)}
	seed := encodedConfiguration(f, want.Project)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		probe := want
		probe.Project.Release.Major = uint32(len(data)) | 1
		generated, err := core.MarshalCanonicalJSONDocument(probe)
		if err != nil {
			t.Fatal(err)
		}
		generated = append(bytes.Repeat([]byte(" "), len(data)%1024), generated...)
		exact, err := compass.Decode[compass.Configuration](bytes.NewReader(generated))
		if err != nil || exact != probe {
			t.Fatalf("generated valid configuration got=%v error=%v, want %v", exact, err, probe)
		}
		got, err := compass.Decode[compass.Configuration](bytes.NewReader(data))
		if err != nil {
			if bytes.Equal(data, seed) || !errors.Is(err, core.ErrCompassContract) || !errors.Is(err, core.ErrJSONContract) || got != (compass.Configuration{}) {
				t.Fatalf("rejected configuration got=%v error=%v, want typed failure and zero", got, err)
			}
			return
		}
		var reference compass.Configuration
		if err := json.Unmarshal(data, &reference, json.RejectUnknownMembers(true)); err != nil || got != reference {
			t.Fatalf("accepted configuration differs from Go's typed decode: got=%v want=%v error=%v", got, reference, err)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted configuration invalid: %v", err)
		}
		canonical, err := core.MarshalCanonicalJSONDocument(got)
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := compass.Decode[compass.Configuration](bytes.NewReader(canonical))
		if err != nil || roundTrip != got {
			t.Fatalf("round trip got=%v error=%v, want %v", roundTrip, err, got)
		}
		canonicalAgain, err := core.MarshalCanonicalJSONDocument(roundTrip)
		if err != nil || !bytes.Equal(canonicalAgain, canonical) {
			t.Fatalf("second canonical bytes=%q error=%v, want %q", canonicalAgain, err, canonical)
		}

	})
}

func projectFixture(t testing.TB, nameText, moduleText, repositoryText string, major, minor, patch uint32) compass.Project {
	t.Helper()
	name, err := compass.ParseProjectName(nameText)
	if err != nil {
		t.Fatalf("ParseProjectName(%q) error = %v, want nil", nameText, err)
	}
	module, err := gomodule.ParsePath(moduleText)
	if err != nil {
		t.Fatalf("gomodule.ParsePath(%q) error = %v, want nil", moduleText, err)
	}
	repository, err := core.NewRepositoryIdentity(repositoryText)
	if err != nil {
		t.Fatalf("NewRepositoryIdentity(%q) error = %v, want nil", repositoryText, err)
	}
	return compass.Project{
		Name: name, Module: module, Repository: repository,
		Release: compass.ReleaseCoordinates{Major: major, Minor: minor, Patch: patch},
	}
}

func encodedConfiguration(t testing.TB, project compass.Project) []byte {
	t.Helper()
	configuration := compass.Configuration{Project: project}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("test Configuration.Validate() error = %v, want nil", err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(configuration)
	if err != nil {
		t.Fatalf("MarshalCanonicalJSONDocument(test Configuration) error = %v, want nil", err)
	}
	return encoded
}

func hostileConfiguration(name, module, repository, major, minor, patch string) []byte {
	return []byte(fmt.Sprintf(`{"project":{"name":"%s","module":"%s","repository":"%s","release":{"major":%s,"minor":%s,"patch":%s}}}`, name, module, repository, major, minor, patch))
}
