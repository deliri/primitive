package github

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestParseRepositoryHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
		want    string
	}{
		{name: "ordinary organization and repository", value: "offGridSoft/blink-kernel", want: "offGridSoft/blink-kernel"},
		{name: "hyphenated owner and dotted repository", value: "owner-name/repo.name", want: "owner-name/repo.name"},
		{name: "underscore in repository remains exact", value: "owner/repo_name", want: "owner/repo_name"},
		{name: "single rune segments remain distinct", value: "o/r", want: "o/r"},
		{name: "unicode remains exact transport text", value: "équipe/dépôt", want: "équipe/dépôt"},
		{name: "zero value is rejected", wantErr: core.ErrGitHubContract},
		{name: "owner without repository is rejected", value: "owner", wantErr: core.ErrGitHubContract},
		{name: "empty owner is rejected", value: "/repo", wantErr: core.ErrGitHubContract},
		{name: "empty repository is rejected", value: "owner/", wantErr: core.ErrGitHubContract},
		{name: "third path segment is rejected", value: "owner/repo/extra", wantErr: core.ErrGitHubContract},
		{name: "owner traversal is rejected", value: "../repo", wantErr: core.ErrGitHubContract},
		{name: "repository traversal is rejected", value: "owner/..", wantErr: core.ErrGitHubContract},
		{name: "query control is rejected", value: "owner/repo?ref=main", wantErr: core.ErrGitHubContract},
		{name: "fragment control is rejected", value: "owner/repo#main", wantErr: core.ErrGitHubContract},
		{name: "percent escape ambiguity is rejected", value: "owner/repo%2Fother", wantErr: core.ErrGitHubContract},
		{name: "backslash ambiguity is rejected", value: `owner\repo`, wantErr: core.ErrGitHubContract},
		{name: "whitespace is rejected", value: "owner name/repo", wantErr: core.ErrGitHubContract},
		{name: "control text is rejected", value: "owner/repo\n", wantErr: core.ErrGitHubContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseRepository(testCase.value)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("ParseRepository(%q) error = %v, want %v", testCase.value, gotErr, testCase.wantErr)
			}
			if got.String() != testCase.want {
				t.Fatalf("ParseRepository(%q).String() = %q, want %q", testCase.value, got.String(), testCase.want)
			}
		})
	}
}

func TestFileRequestProviderAndProductBounds(t *testing.T) {
	t.Parallel()

	repository := parsedRepository(t, "owner/repository")
	commit := parsedCommit(t)
	path := parsedPath(t, "source/main.go")
	tests := []struct {
		wantErr error
		name    string
		maximum uint64
	}{
		{name: "one byte product budget is admitted", maximum: 1},
		{name: "one below provider inline ceiling is admitted", maximum: core.GitHubContentsInlineMaximumBytes - 1},
		{name: "exact provider inline ceiling is admitted", maximum: core.GitHubContentsInlineMaximumBytes},
		{name: "zero budget is rejected", maximum: 0, wantErr: core.ErrGitHubContract},
		{name: "one above provider inline ceiling is rejected", maximum: core.GitHubContentsInlineMaximumBytes + 1, wantErr: core.ErrGitHubContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			maximum := core.ByteCount{}
			if testCase.maximum != 0 {
				maximum = byteCountFixture(t, testCase.maximum)
			}
			gotErr := (FileRequest{Repository: repository, Commit: commit, Path: path, MaximumBytes: maximum}).Validate()
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("FileRequest{MaximumBytes:%d}.Validate() error = %v, want %v", testCase.maximum, gotErr, testCase.wantErr)
			}
		})
	}
}

func TestTreeEntryKindClosedDomainIsExhaustive(t *testing.T) {
	t.Parallel()

	for value := TreeEntryKind(0); value <= treeEntryKindLimit; value++ {
		gotErr := value.Validate()
		wantValid := value == TreeEntryBlob || value == TreeEntryDirectory || value == TreeEntrySubmodule
		if (gotErr == nil) != wantValid {
			t.Fatalf("TreeEntryKind(%d).Validate() error = %v, want valid %t", value, gotErr, wantValid)
		}
		if got := value.IsValid(); got != wantValid {
			t.Fatalf("TreeEntryKind(%d).IsValid() = %t, want %t", value, got, wantValid)
		}
		if got := value.String(); (got != core.UnknownEnumDiagnostic) != wantValid {
			t.Fatalf("TreeEntryKind(%d).String() = %q, want recognized %t", value, got, wantValid)
		}
	}
}

func FuzzParseRepositorySemanticClosure(f *testing.F) {
	seed := parsedRepository(f, "owner/repository")
	f.Add(seed.String())
	f.Add("")
	f.Add("owner/repository/extra")
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseRepository(value)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubContract) || got != (Repository{}) {
				t.Fatalf("ParseRepository(%q) = (%v, %v), want zero and %v", value, got, gotErr, core.ErrGitHubContract)
			}
			return
		}
		if got.Validate() != nil || got.String() != value || strings.Count(value, repositorySeparator) != 1 {
			t.Fatalf("ParseRepository(%q) = %v, want exact validated one-separator coordinate", value, got)
		}
	})
}

func parsedRepository(t testing.TB, value string) Repository {
	t.Helper()
	got, err := ParseRepository(value)
	if err != nil {
		t.Fatalf("ParseRepository(%q) error = %v, want nil", value, err)
	}
	return got
}

func parsedCommit(t testing.TB) core.BuildCommit {
	t.Helper()
	got, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(fixture) error = %v, want nil", err)
	}
	return got
}

func parsedPath(t testing.TB, value string) core.SourcePath {
	t.Helper()
	got, err := core.ParseSourcePath(value)
	if err != nil {
		t.Fatalf("core.ParseSourcePath(%q) error = %v, want nil", value, err)
	}
	return got
}
