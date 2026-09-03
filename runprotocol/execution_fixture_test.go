package runprotocol

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func fixtureIdentifier(t testing.TB, value string) Identifier {
	t.Helper()
	got, gotErr := NewIdentifier(value)
	if gotErr != nil {
		t.Fatalf("NewIdentifier(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureName(t testing.TB, value string) Name {
	t.Helper()
	got, gotErr := NewName(value)
	if gotErr != nil {
		t.Fatalf("NewName(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureText(t testing.TB, value string) Text {
	t.Helper()
	got, gotErr := NewText(value)
	if gotErr != nil {
		t.Fatalf("NewText(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixturePath(t testing.TB, value string) SourcePath {
	t.Helper()
	got, gotErr := ParseSourcePath(value)
	if gotErr != nil {
		t.Fatalf("ParseSourcePath(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureRepository(t testing.TB, value string) RepositoryIdentity {
	t.Helper()
	got, gotErr := NewRepositoryIdentity(value)
	if gotErr != nil {
		t.Fatalf("NewRepositoryIdentity(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureProfile(t testing.TB, value string) ProfileIdentity {
	t.Helper()
	got, gotErr := NewProfileIdentity(fixtureIdentifier(t, value), 1)
	if gotErr != nil {
		t.Fatalf("NewProfileIdentity(%q, 1) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureCommit(t testing.TB, value string) core.BuildCommit {
	t.Helper()
	got, gotErr := core.ParseBuildCommit(value)
	if gotErr != nil {
		t.Fatalf("ParseBuildCommit(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}
