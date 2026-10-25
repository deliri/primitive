package github

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestParseRemoteHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
		want    string
	}{
		{name: "https with suffix", value: "https://github.com/offGridSoft/anvil.git", want: "offgridsoft/anvil"},
		{name: "https without suffix", value: "https://github.com/offGridSoft/anvil", want: "offgridsoft/anvil"},
		{name: "scp-like ssh with suffix", value: "git@github.com:offGridSoft/anvil.git", want: "offgridsoft/anvil"},
		{name: "scp-like ssh without suffix", value: "git@github.com:offGridSoft/anvil", want: "offgridsoft/anvil"},
		{name: "ssh url with suffix", value: "ssh://git@github.com/offGridSoft/anvil.git", want: "offgridsoft/anvil"},
		{name: "ascii case folds", value: "https://github.com/OffGridSoft/Anvil.git", want: "offgridsoft/anvil"},
		{name: "non-ascii text is kept", value: "https://github.com/Équipe/Dépôt", want: "Équipe/dépôt"},
		{name: "only one suffix is removed", value: "https://github.com/owner/name.git.git", want: "owner/name.git"},
		{name: "zero value is rejected", wantErr: core.ErrGitHubContract},
		{name: "invalid UTF-8 is not repaired into a repository", value: "git@github.com:owner/\xff", wantErr: core.ErrGitHubContract},
		{name: "bare owner and name is not a remote", value: "offGridSoft/anvil", wantErr: core.ErrGitHubContract},
		{name: "another host is rejected", value: "https://gitlab.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "another host spelled scp-like is rejected", value: "git@gitlab.com:offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "plain http is rejected", value: "http://github.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "explicit port is rejected", value: "ssh://git@github.com:22/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "https credential is rejected", value: "https://token@github.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "ssh password is rejected", value: "ssh://git:secret@github.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "ssh url without user is rejected", value: "ssh://github.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "another ssh user is rejected", value: "ssh://deploy@github.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "query is rejected", value: "https://github.com/offGridSoft/anvil.git?ref=main", wantErr: core.ErrGitHubContract},
		{name: "empty query is rejected", value: "https://github.com/offGridSoft/anvil.git?", wantErr: core.ErrGitHubContract},
		{name: "fragment is rejected", value: "https://github.com/offGridSoft/anvil.git#main", wantErr: core.ErrGitHubContract},
		{name: "escaped separator is rejected", value: "https://github.com/offGridSoft%2Fanvil", wantErr: core.ErrGitHubContract},
		{name: "escaped ordinary rune is rejected", value: "https://github.com/offGridSoft/%61nvil", wantErr: core.ErrGitHubContract},
		{name: "trailing slash is rejected", value: "https://github.com/offGridSoft/anvil/", wantErr: core.ErrGitHubContract},
		{name: "third path segment is rejected", value: "https://github.com/offGridSoft/anvil/tree", wantErr: core.ErrGitHubContract},
		{name: "suffix alone is rejected", value: "git@github.com:offGridSoft/.git", wantErr: core.ErrGitHubContract},
		{name: "scp-like absolute path is rejected", value: "git@github.com:/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "traversal is rejected", value: "git@github.com:../anvil.git", wantErr: core.ErrGitHubContract},
		{name: "whitespace is rejected", value: "git@github.com:offGridSoft/anvil.git ", wantErr: core.ErrGitHubContract},
		{name: "scheme case is not normalized", value: "HTTPS://github.com/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
		{name: "look-alike host is rejected", value: "https://github.com.example.invalid/offGridSoft/anvil.git", wantErr: core.ErrGitHubContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseRemote(testCase.value)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("ParseRemote(%q) error = %v, want %v", testCase.value, gotErr, testCase.wantErr)
			}
			if got.String() != testCase.want {
				t.Fatalf("ParseRemote(%q).String() = %q, want %q", testCase.value, got.String(), testCase.want)
			}
		})
	}
}

// The spellings and ASCII case variants of one repository are equal values;
// other repositories are not.
func TestParseRemoteEquatesOnlyTheSameRepository(t *testing.T) {
	t.Parallel()

	ssh := parsedRemote(t, "git@github.com:offGridSoft/anvil.git")
	for _, value := range []string{"https://github.com/offGridSoft/anvil.git", "https://github.com/offGridSoft/anvil", "ssh://git@github.com/offGridSoft/anvil", "git@github.com:offgridsoft/ANVIL.git"} {
		if got := parsedRemote(t, value); got != ssh {
			t.Fatalf("ParseRemote(%q) = %v, want equal to %v", value, got, ssh)
		}
	}
	for _, value := range []string{"git@github.com:offGridSoft/witness.git", "git@github.com:deliri/anvil.git", "git@github.com:offGridSoft/anvil2.git"} {
		if got := parsedRemote(t, value); got == ssh {
			t.Fatalf("ParseRemote(%q) = %v, want distinct from %v", value, got, ssh)
		}
	}
}

func FuzzParseRemoteSemanticClosure(f *testing.F) {
	f.Add("git@github.com:owner/repository.git")
	f.Add("https://github.com/owner/repository")
	f.Add("ssh://git@github.com/owner/repository.git")
	f.Add("https://github.com:443/owner/repository")
	f.Add("git@github.com:owner/\xff")
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseRemote(value)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubContract) || got != (Repository{}) {
				t.Fatalf("ParseRemote(%q) = (%v, %v), want zero and %v", value, got, gotErr, core.ErrGitHubContract)
			}
			return
		}
		coordinate := got.String()
		if got.Validate() != nil || got != parsedRepository(t, coordinate) {
			t.Fatalf("ParseRemote(%q) = %v, want a validated owner/name coordinate", value, got)
		}
		for _, spelling := range []string{"https://github.com/", "git@github.com:", "ssh://git@github.com/"} { // independent of remotePrefixes
			for _, suffix := range []string{"", ".git"} {
				if len(value) >= len(spelling)+len(suffix) && value[:len(spelling)] == spelling && value[len(value)-len(suffix):] == suffix && lowerASCIIOracle(value[len(spelling):len(value)-len(suffix)]) == coordinate {
					return
				}
			}
		}
		t.Fatalf("ParseRemote(%q) = %v, want the input to be one of its six GitHub spellings", value, got)
	})
}

func parsedRemote(t testing.TB, value string) Repository {
	t.Helper()
	got, err := ParseRemote(value)
	if err != nil {
		t.Fatalf("ParseRemote(%q) error = %v, want nil", value, err)
	}
	return got
}

// lowerASCIIOracle is the test's own byte-wise ASCII fold.
func lowerASCIIOracle(value string) string {
	b := []byte(value)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
