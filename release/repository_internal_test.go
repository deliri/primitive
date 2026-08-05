package release

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// TestParseRepositoryHeadOutputAcceptsOnlyCanonicalGitHeadBytes pressures the
// exact observation that binds a release to a commit. VerifyRepository derives
// its whole commit fact from one Git stdout string, so a lenient parse here
// would let a padded, repeated, truncated, or case-shifted spelling be read as
// a commit the repository never resolved. The real Git executable cannot be
// made to emit these spellings, which is precisely why this contract needs a
// pure boundary it can be held to.
func TestParseRepositoryHeadOutputAcceptsOnlyCanonicalGitHeadBytes(t *testing.T) {
	t.Parallel()

	const (
		sha1Commit   = "0123456789abcdef0123456789abcdef01234567"
		sha256Commit = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		zeroCommit   = "0000000000000000000000000000000000000000"
		maximumSHA1  = "ffffffffffffffffffffffffffffffffffffffff"
	)

	cases := []struct {
		name       string
		output     string
		wantCommit string
		wantOK     bool
	}{
		{name: "canonical SHA-1 HEAD with one trailing newline", output: sha1Commit + "\n", wantCommit: sha1Commit, wantOK: true},
		{name: "canonical SHA-256 HEAD with one trailing newline", output: sha256Commit + "\n", wantCommit: sha256Commit, wantOK: true},
		{name: "all zero SHA-1 is a representable minimum commit", output: zeroCommit + "\n", wantCommit: zeroCommit, wantOK: true},
		{name: "all f SHA-1 is a representable maximum commit", output: maximumSHA1 + "\n", wantCommit: maximumSHA1, wantOK: true},
		{name: "single hex digit change is still a distinct accepted commit", output: "1" + sha1Commit[1:] + "\n", wantCommit: "1" + sha1Commit[1:], wantOK: true},

		{name: "empty output resolved nothing", output: ""},
		{name: "lone newline resolved nothing", output: "\n"},
		{name: "missing trailing newline is not canonical Git output", output: sha1Commit},
		{name: "two trailing newlines carry an unexplained second line", output: sha1Commit + "\n\n"},
		{name: "carriage return before the newline is not canonical", output: sha1Commit + "\r\n"},
		{name: "leading newline shifts the observed fact", output: "\n" + sha1Commit + "\n"},
		{name: "leading space is not trimmed into a commit", output: " " + sha1Commit + "\n"},
		{name: "trailing space before the newline is not trimmed", output: sha1Commit + " \n"},
		{name: "leading tab is not trimmed into a commit", output: "\t" + sha1Commit + "\n"},
		{name: "embedded NUL after the commit", output: sha1Commit + "\x00\n"},

		{name: "one hex digit below SHA-1 width", output: sha1Commit[:39] + "\n"},
		{name: "one hex digit above SHA-1 width", output: sha1Commit + "0\n"},
		{name: "two hex digits above SHA-1 width", output: sha1Commit + "00\n"},
		{name: "one hex digit below SHA-256 width", output: sha256Commit[:63] + "\n"},
		{name: "one hex digit above SHA-256 width", output: sha256Commit + "0\n"},
		{name: "abbreviated seven digit commit", output: sha1Commit[:7] + "\n"},
		{name: "midpoint width between the two supported widths", output: strings.Repeat("a", 52) + "\n"},
		{name: "odd width cannot decode as hexadecimal pairs", output: strings.Repeat("a", 41) + "\n"},
		{name: "uppercase hexadecimal is not canonical", output: strings.ToUpper(sha1Commit) + "\n"},
		{name: "mixed case hexadecimal is not canonical", output: "0123456789ABCDEF0123456789abcdef01234567\n"},
		{name: "non hexadecimal digit at the first position", output: "g" + sha1Commit[1:] + "\n"},
		{name: "non hexadecimal digit at the last position", output: sha1Commit[:39] + "g\n"},
		{name: "unicode digit that is not ASCII hexadecimal", output: "٠" + sha1Commit[1:] + "\n"},

		{name: "two commits on separate lines", output: sha1Commit + "\n" + sha1Commit + "\n"},
		{name: "commit followed by a Git diagnostic line", output: sha1Commit + "\nwarning: refname is ambiguous\n"},
		{name: "Git diagnostic line before the commit", output: "warning: refname is ambiguous\n" + sha1Commit + "\n"},
		{name: "symbolic ref name instead of a commit", output: "refs/heads/main\n"},
		{name: "literal HEAD instead of a resolved commit", output: "HEAD\n"},
		{name: "commit with a ref suffix", output: sha1Commit + " HEAD\n"},
		{name: "commit wrapped in quotes", output: `"` + sha1Commit + `"` + "\n"},
		{name: "oversized repeated output", output: strings.Repeat(sha1Commit+"\n", 64)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseRepositoryHeadOutput(tc.output)
			if !tc.wantOK {
				if !errors.Is(err, core.ErrReleaseContract) {
					t.Fatalf("parseRepositoryHeadOutput(%q) error = %v, want errors.Is(_, %v)", tc.output, err, core.ErrReleaseContract)
				}
				if got != (core.BuildCommit{}) {
					t.Fatalf("parseRepositoryHeadOutput(%q) commit = %v, want the zero commit on rejection", tc.output, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRepositoryHeadOutput(%q) error = %v, want nil", tc.output, err)
			}
			if got.String() != tc.wantCommit {
				t.Fatalf("parseRepositoryHeadOutput(%q) commit = %q, want %q", tc.output, got.String(), tc.wantCommit)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("parseRepositoryHeadOutput(%q) accepted an invalid commit: %v", tc.output, err)
			}
		})
	}
}

// TestRepositoryGitPolicyArgumentsNeutralizeRepositoryLocalConfiguration is a
// contract ratchet over the local settings that can weaken status or let a
// read-only observation refresh the index. System/global configuration is
// closed independently by repositoryGitEnvironment.
func TestRepositoryGitPolicyArgumentsNeutralizeRepositoryLocalConfiguration(t *testing.T) {
	t.Parallel()

	got := repositoryGitPolicyArguments()
	want := []string{
		"--no-optional-locks",
		"-c", "core.attributesFile=",
		"-c", "core.fsmonitor=false",
		"-c", "core.fileMode=true",
		"-c", "core.trustctime=true",
		"-c", "core.checkStat=default",
		"-c", "core.ignoreStat=false",
	}
	if len(got) != len(want) {
		t.Fatalf("repositoryGitPolicyArguments() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("repositoryGitPolicyArguments() = %v, want %v", got, want)
		}
	}
	got[0] = "mutated"
	if next := repositoryGitPolicyArguments()[0]; next != want[0] {
		t.Fatalf("repositoryGitPolicyArguments()[0] after caller mutation = %q, want %q; the policy must not be shared mutable state", next, want[0])
	}
}

func TestRepositoryGitEnvironmentOwnsEveryGitPolicyVariable(t *testing.T) {
	t.Parallel()

	base, err := process.ParseExactEnvironment([]string{"HOME=/controlled/home", "PATH=/controlled/bin"})
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment(base) error = %v, want nil", err)
	}
	got, err := repositoryGitEnvironment(base)
	if err != nil {
		t.Fatalf("repositoryGitEnvironment() error = %v, want nil", err)
	}
	gotValues, err := got.Strings()
	if err != nil {
		t.Fatalf("repository Git Environment.Strings() error = %v, want nil", err)
	}
	wantValues := []string{
		"HOME=/controlled/home",
		"PATH=/controlled/bin",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_ATTR_NOSYSTEM=1",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
	}
	if len(gotValues) != len(wantValues) {
		t.Fatalf("repositoryGitEnvironment() = %v, want %v", gotValues, wantValues)
	}
	for index := range wantValues {
		if gotValues[index] != wantValues[index] {
			t.Fatalf("repositoryGitEnvironment() = %v, want %v", gotValues, wantValues)
		}
	}

	inherit := process.Environment{Mode: process.EnvironmentModeInherit}
	if _, err := repositoryGitEnvironment(inherit); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("repositoryGitEnvironment(inherit) error = %v, want %v", err, core.ErrReleaseContract)
	}
}

// TestRepositoryIndexWriterAcceptsOnlyCompleteNormalIndexEntries pressures the
// streaming boundary over `git ls-files -v -z`. Real Git supplies arbitrary
// write chunking, paths may contain any non-NUL byte, and special index tags
// must stop the scan before an assume-unchanged or skip-worktree entry can hide
// a source change from status.
func TestRepositoryIndexWriterAcceptsOnlyCompleteNormalIndexEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		chunks       [][]byte
		wantDirty    bool
		wantContract bool
	}{
		{name: "neutral empty index has no entries"},
		{name: "positive one normal tracked path", chunks: [][]byte{[]byte("H tracked.go\x00")}},
		{name: "positive two normal tracked paths", chunks: [][]byte{[]byte("H a.go\x00H b.go\x00")}},
		{name: "positive tag separator and path may arrive separately", chunks: [][]byte{[]byte("H"), []byte(" "), []byte("tracked.go"), []byte{0}}},
		{name: "positive every byte may arrive separately", chunks: [][]byte{[]byte("H"), []byte(" "), []byte("a"), []byte{0}, []byte("H"), []byte(" "), []byte("b"), []byte{0}}},
		{name: "positive path may contain spaces", chunks: [][]byte{[]byte("H path with spaces\x00")}},
		{name: "positive path may contain a newline", chunks: [][]byte{[]byte("H path\ninside\x00")}},
		{name: "positive path may contain non UTF-8 bytes", chunks: [][]byte{{'H', ' ', 0xff, 0}}},
		{name: "negative assume unchanged lowercase tag", chunks: [][]byte{[]byte("h tracked.go\x00")}, wantDirty: true},
		{name: "negative skip worktree tag", chunks: [][]byte{[]byte("S tracked.go\x00")}, wantDirty: true},
		{name: "negative unmerged tag", chunks: [][]byte{[]byte("M tracked.go\x00")}, wantDirty: true},
		{name: "negative removed tag", chunks: [][]byte{[]byte("R tracked.go\x00")}, wantDirty: true},
		{name: "negative modified tag", chunks: [][]byte{[]byte("C tracked.go\x00")}, wantDirty: true},
		{name: "negative killed tag", chunks: [][]byte{[]byte("K tracked.go\x00")}, wantDirty: true},
		{name: "negative unknown tag", chunks: [][]byte{[]byte("? tracked.go\x00")}, wantDirty: true},
		{name: "negative NUL tag", chunks: [][]byte{{0}}, wantDirty: true},
		{name: "malformed tag without separator", chunks: [][]byte{[]byte("Hx")}, wantContract: true},
		{name: "malformed empty path", chunks: [][]byte{[]byte("H \x00")}, wantContract: true},
		{name: "boundary truncated after tag", chunks: [][]byte{[]byte("H")}, wantContract: true},
		{name: "boundary truncated after separator", chunks: [][]byte{[]byte("H ")}, wantContract: true},
		{name: "boundary truncated inside path", chunks: [][]byte{[]byte("H tracked.go")}, wantContract: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			writer := &repositoryIndexWriter{}
			var gotErr error
			for _, chunk := range tc.chunks {
				_, gotErr = writer.Write(chunk)
				if gotErr != nil {
					break
				}
			}
			if gotErr == nil {
				gotErr = writer.finish()
			}
			if tc.wantDirty {
				if !errors.Is(gotErr, errRepositoryStatusObserved) {
					t.Fatalf("repositoryIndexWriter error = %v, want %v", gotErr, errRepositoryStatusObserved)
				}
				return
			}
			if tc.wantContract {
				if !errors.Is(gotErr, core.ErrReleaseContract) {
					t.Fatalf("repositoryIndexWriter error = %v, want %v", gotErr, core.ErrReleaseContract)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("repositoryIndexWriter error = %v, want nil", gotErr)
			}
		})
	}
}

// TestParseRepositoryGitPathOutputAcceptsOnlyOneCanonicalAbsolutePath proves
// the private-attributes check never trims or combines path output before
// opening Git-owned metadata. The real command cannot be induced to emit these
// malformed shapes, so this is a direct unit ratchet over that exact boundary.
func TestParseRepositoryGitPathOutputAcceptsOnlyOneCanonicalAbsolutePath(t *testing.T) {
	t.Parallel()

	first := t.TempDir()
	second := t.TempDir()
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{name: "positive one absolute path and newline", output: first + "\n", want: first},
		{name: "positive distinct absolute path and newline", output: second + "\n", want: second},
		{name: "empty output resolved no path"},
		{name: "lone newline contains an empty path", output: "\n"},
		{name: "missing trailing newline is not canonical", output: first},
		{name: "two trailing newlines carry a second record", output: first + "\n\n"},
		{name: "carriage return is not canonical", output: first + "\r\n"},
		{name: "leading newline shifts the path", output: "\n" + first + "\n"},
		{name: "two paths are not one Git fact", output: first + "\n" + second + "\n"},
		{name: "relative path is not admitted", output: "info/attributes\n"},
		{name: "embedded NUL is not admitted", output: first + "\x00\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseRepositoryGitPathOutput(tc.output)
			if tc.want == "" {
				if !errors.Is(err, core.ErrReleaseContract) {
					t.Fatalf("parseRepositoryGitPathOutput(%q) error = %v, want %v", tc.output, err, core.ErrReleaseContract)
				}
				if got != (core.AbsolutePath{}) {
					t.Fatalf("parseRepositoryGitPathOutput(%q) path = %v, want zero", tc.output, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRepositoryGitPathOutput(%q) error = %v, want nil", tc.output, err)
			}
			if got.String() != tc.want {
				t.Fatalf("parseRepositoryGitPathOutput(%q) path = %q, want %q", tc.output, got.String(), tc.want)
			}
		})
	}
}
