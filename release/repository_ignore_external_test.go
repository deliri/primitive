package release_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
)

// The repository owns its ignore rules. Every row uses real Git and a committed
// rule, proves the clean baseline, then changes one checkout fact. No binary
// contents are inspected: Git's declared output paths define this boundary.
func TestVerifyRepositoryIgnorePolicyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mutate    func(*testing.T, repositoryFixture)
		name      string
		rulePath  string
		rules     string
		wantDirty bool
	}{
		{name: "absent build output leaves exact source proof unchanged", rulePath: ".gitignore", rules: "/dist/\n"},
		{name: "ignored build directory admits an existing executable", rulePath: ".gitignore", rules: "/dist/\n", mutate: func(t *testing.T, fixture repositoryFixture) {
			ensureRepositoryDirectoryForTest(t, fixture.root, "dist")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "dist/tool", body: "compiled output\x00"})
		}},
		{name: "ignored test executable suffix admits root output", rulePath: ".gitignore", rules: "*.test\n", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "release.test", body: "test executable\x00"})
		}},
		{name: "nested committed ignore rule governs its own output", rulePath: "nested/.gitignore", rules: "/output\n", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "nested/output", body: "nested output\x00"})
		}},
		{name: "ignore negation restores an untracked source to observation", rulePath: ".gitignore", rules: "*.test\n!important.test\n", wantDirty: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "important.test", body: "authored input\n"})
		}},
		{name: "directory prefix does not hide a neighboring source", rulePath: ".gitignore", rules: "/dist/\n", wantDirty: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "distribution.go", body: "package distribution\n"})
		}},
		{name: "ignore rule cannot hide a tracked modification", rulePath: ".gitignore", rules: "tracked.txt\n", wantDirty: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "forced staged output is still an uncommitted index change", rulePath: ".gitignore", rules: "*.test\n", wantDirty: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "release.test", body: "staged output\x00"})
			runRepositoryGitForTest(t, fixture, "add", "--force", "--", "release.test")
		}},
		{name: "edited ignore rules cannot silently widen the committed policy", rulePath: ".gitignore", rules: "/dist/\n", wantDirty: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: ".gitignore", body: "*\n"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newRepositoryFixtureAt(t, t.TempDir(), t.TempDir())
			ensureRepositoryDirectoryForTest(t, fixture.root, "nested")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: tc.rulePath, body: tc.rules})
			runRepositoryGitForTest(t, fixture, "add", "--", tc.rulePath)
			runRepositoryGitForTest(t, fixture, "commit", "--quiet", "-m", "declare generated output")
			fixture.commit = repositoryHeadForTest(t, fixture)
			baseline, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
			if err != nil || baseline.Commit() != fixture.commit {
				t.Fatalf("VerifyRepository(committed ignore policy) = (%v, %v), want commit %v and nil", baseline, err, fixture.commit)
			}
			if tc.mutate != nil {
				tc.mutate(t, fixture)
			}
			got, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
			if tc.wantDirty {
				var dirty release.RepositoryDirtyError
				if !errors.As(err, &dirty) || !errors.Is(err, core.ErrReleaseContract) || dirty.Root() != fixture.root || got != (release.VerifiedRepository{}) {
					t.Fatalf("VerifyRepository(changed source) = (%v, %v), want zero proof and typed dirty refusal for %v", got, err, fixture.root)
				}
				return
			}
			if err != nil || got != baseline {
				t.Fatalf("VerifyRepository(ignored output) = (%v, %v), want unchanged proof %v and nil", got, err, baseline)
			}
		})
	}
}

// FuzzVerifyRepositoryIgnoredOutputAndSourceFacts reaches the public filesystem
// and Git boundary. One admitted and one refused checkout start mutation within
// the short fuzz budget. The oracle is the independently declared source changes,
// never a parsed Git status; named source-change boundaries are tested above.
func FuzzVerifyRepositoryIgnoredOutputAndSourceFacts(f *testing.F) {
	f.Add(true, false, false)
	f.Add(false, false, false)
	f.Fuzz(func(t *testing.T, ignored, tracked, untracked bool) {
		fixture := newRepositoryFixtureAt(t, t.TempDir(), t.TempDir())
		if ignored {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: ".gitignore", body: "/dist/\n"})
			runRepositoryGitForTest(t, fixture, "add", "--", ".gitignore")
			runRepositoryGitForTest(t, fixture, "commit", "--quiet", "-m", "declare build output")
			fixture.commit = repositoryHeadForTest(t, fixture)
		}
		ensureRepositoryDirectoryForTest(t, fixture.root, "dist")
		writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "dist/tool", body: "generated executable\x00"})
		if tracked {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}
		if untracked {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "new.go", body: "package newsource\n"})
		}
		got, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
		if tracked || untracked || !ignored {
			var dirty release.RepositoryDirtyError
			if !errors.As(err, &dirty) || !errors.Is(err, core.ErrReleaseContract) || dirty.Root() != fixture.root || got != (release.VerifiedRepository{}) {
				t.Fatalf("VerifyRepository(source changes) = (%v, %v), want zero proof and typed dirty refusal for %v", got, err, fixture.root)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Commit() != fixture.commit || got.Root() != fixture.root || got.GitExecutable() != fixture.git {
			t.Fatalf("VerifyRepository(ignored output only) = (%v, %v), want valid proof for %v at %v", got, err, fixture.root, fixture.commit)
		}
	})
}
