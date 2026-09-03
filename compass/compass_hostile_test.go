package compass_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

type compassCaseClass uint8

const (
	compassCaseValid compassCaseClass = iota + 1
	compassCaseRejection
	compassCaseBoundary
)

func TestDecodeHostileProjectConfigurationMatrix(t *testing.T) {
	t.Parallel()

	canonical := encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, 3))
	cases := []struct {
		name    string
		data    []byte
		class   compassCaseClass
		wantErr error
	}{
		{name: "valid ordinary project declaration", data: canonical, class: compassCaseValid},
		{name: "valid unicode project name", data: encodedConfiguration(t, projectFixture(t, "Évidence", "example.com/project", "owner/project", 2026, 1, 3)), class: compassCaseValid},
		{name: "valid internal project name whitespace", data: encodedConfiguration(t, projectFixture(t, "Evidence Tool", "example.com/project", "owner/project", 2026, 1, 3)), class: compassCaseValid},
		{name: "valid versioned module path", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project/v2", "owner/project", 2026, 1, 3)), class: compassCaseValid},
		{name: "valid opaque repository coordinate", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "ssh://git.example/project", 2026, 1, 3)), class: compassCaseValid},
		{name: "valid zero minor coordinate", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 0, 3)), class: compassCaseValid},
		{name: "valid zero patch coordinate", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, 0)), class: compassCaseValid},
		{name: "valid maximum minor coordinate", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", 2026, math.MaxUint32, 3)), class: compassCaseValid},
		{name: "valid maximum patch coordinate", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, math.MaxUint32)), class: compassCaseValid},
		{name: "valid surrounding document whitespace", data: append(append([]byte(" \n\t"), canonical...), []byte("\r\n")...), class: compassCaseValid},

		{name: "rejection project name has leading whitespace", data: hostileConfiguration(" Project", "example.com/project", "owner/project", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection project name contains control text", data: hostileConfiguration("Project\\u000aName", "example.com/project", "owner/project", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection project name is empty", data: hostileConfiguration("", "example.com/project", "owner/project", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection module path contains whitespace", data: hostileConfiguration("Project", "example.com/bad module", "owner/project", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection module path is absent", data: hostileConfiguration("Project", "", "owner/project", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection repository contains whitespace", data: hostileConfiguration("Project", "example.com/project", "owner bad/project", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection repository is absent", data: hostileConfiguration("Project", "example.com/project", "", "2026", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection negative release coordinate", data: hostileConfiguration("Project", "example.com/project", "owner/project", "-1", "1", "3"), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection case-insensitive duplicate project member", data: []byte(`{"project":{"name":"Project","Name":"Shadow","module":"example.com/project","repository":"owner/project","release":{"major":2026,"minor":1,"patch":3}}}`), class: compassCaseRejection, wantErr: core.ErrCompassContract},
		{name: "rejection malformed object delimiter", data: []byte(`{"project":`), class: compassCaseRejection, wantErr: core.ErrCompassContract},

		{name: "boundary one-byte project name is accepted", data: encodedConfiguration(t, projectFixture(t, "P", "example.com/project", "owner/project", 2026, 1, 3)), class: compassCaseBoundary},
		{name: "boundary one below project name maximum is accepted", data: encodedConfiguration(t, projectFixture(t, strings.Repeat("n", compass.ProjectNameMaximumBytes-1), "example.com/project", "owner/project", 2026, 1, 3)), class: compassCaseBoundary},
		{name: "boundary exact project name maximum is accepted", data: encodedConfiguration(t, projectFixture(t, strings.Repeat("n", compass.ProjectNameMaximumBytes), "example.com/project", "owner/project", 2026, 1, 3)), class: compassCaseBoundary},
		{name: "boundary one above project name maximum is refused", data: hostileConfiguration(strings.Repeat("n", compass.ProjectNameMaximumBytes+1), "example.com/project", "owner/project", "2026", "1", "3"), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary one below document maximum is accepted", data: padDocument(t, canonical, compass.DocumentMaximumBytes-1), class: compassCaseBoundary},
		{name: "boundary exact document maximum is accepted", data: padDocument(t, canonical, compass.DocumentMaximumBytes), class: compassCaseBoundary},
		{name: "boundary one above document maximum is refused", data: padDocument(t, canonical, compass.DocumentMaximumBytes+1), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary twice document maximum is refused", data: padDocument(t, canonical, compass.DocumentMaximumBytes*2), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary minimum admitted major is accepted", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", 1, 1, 3)), class: compassCaseBoundary},
		{name: "boundary zero major is refused", data: hostileConfiguration("Project", "example.com/project", "owner/project", "0", "1", "3"), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary maximum major is accepted", data: encodedConfiguration(t, projectFixture(t, "Project", "example.com/project", "owner/project", math.MaxUint32, 1, 3)), class: compassCaseBoundary},
		{name: "boundary major above uint32 is refused", data: hostileConfiguration("Project", "example.com/project", "owner/project", "4294967296", "1", "3"), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary empty document is refused", data: nil, class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary null document is refused", data: []byte(`null`), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary array document is refused", data: []byte(`[]`), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary trailing document is refused", data: append(append([]byte{}, canonical...), []byte(` {}`)...), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary missing project member is refused", data: []byte(`{}`), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary unknown root member is refused", data: append(append([]byte{}, canonical[:len(canonical)-1]...), []byte(`,"unknown":true}`)...), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary major string is refused", data: hostileConfiguration("Project", "example.com/project", "owner/project", `"2026"`, "1", "3"), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
		{name: "boundary major fraction is refused", data: hostileConfiguration("Project", "example.com/project", "owner/project", "2026.5", "1", "3"), class: compassCaseBoundary, wantErr: core.ErrCompassContract},
	}

	var counts [4]int
	for _, tc := range cases {
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := compass.Decode[compass.Configuration](bytes.NewReader(tc.data))
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Decode() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (compass.Configuration{}) {
					t.Fatalf("Decode(rejected) = %+v, want zero", got)
				}
				return
			}
			if err := got.Validate(); err != nil || got.Project.Name.String() == "" {
				t.Fatalf("Decode(accepted) = (%+v, %v), want non-vacuous validated project", got, err)
			}
		})
	}
	if counts[compassCaseValid] != 10 || counts[compassCaseRejection] != 10 || counts[compassCaseBoundary] != 20 {
		t.Fatalf("matrix class counts = valid:%d rejection:%d boundary:%d, want 10/10/20", counts[compassCaseValid], counts[compassCaseRejection], counts[compassCaseBoundary])
	}
}

func TestCurrentCompassProducesPrimitiveProject(t *testing.T) {
	t.Parallel()

	got, gotErr := compass.Current()
	if gotErr != nil || got.Validate() != nil || got.Project.Name.String() != "Primitive" || got.Project.Release != (compass.ReleaseCoordinates{Major: 2026, Minor: 1, Patch: 3}) {
		t.Fatalf("Current() = (%+v, %v), want validated Primitive 2026.1.3", got, gotErr)
	}
}

func FuzzDecodeProjectConfigurationSemanticClosure(f *testing.F) {
	seed := encodedConfiguration(f, projectFixture(f, "Project", "example.com/project", "owner/project", 2026, 1, 3))
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := compass.Decode[compass.Configuration](bytes.NewReader(data))
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCompassContract) || got != (compass.Configuration{}) {
				t.Fatalf("Decode(rejected) = (%+v, %v), want zero and %v", got, gotErr, core.ErrCompassContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Decode(accepted).Validate() error = %v, want nil", err)
		}
		canonical, err := core.MarshalCanonicalJSONDocument(got)
		if err != nil || uint64(len(canonical)) > compass.DocumentMaximumBytes {
			t.Fatalf("MarshalCanonicalJSONDocument(accepted) = (%d bytes, %v), want bounded and nil", len(canonical), err)
		}
		roundTrip, err := compass.Decode[compass.Configuration](bytes.NewReader(canonical))
		if err != nil || roundTrip != got {
			t.Fatalf("canonical round trip = (%+v, %v), want (%+v, nil)", roundTrip, err, got)
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

func padDocument(t testing.TB, document []byte, extent uint64) []byte {
	t.Helper()
	if extent < uint64(len(document)) || extent > uint64(math.MaxInt) {
		t.Fatalf("padDocument extent = %d, want [%d,%d]", extent, len(document), math.MaxInt)
	}
	padding := make([]byte, int(extent)-len(document))
	for index := range padding {
		padding[index] = ' '
	}
	return append(padding, document...)
}
