package release_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type repositoryFixture struct {
	home        string
	root        core.AbsolutePath
	git         core.AbsolutePath
	environment process.Environment
	commit      core.BuildCommit
}

type repositoryFileWrite struct {
	name string
	body string
	root core.AbsolutePath
}

// TestVerifyRepositoryPressuresEveryObservableCheckoutState drives the real
// Git executable and real worktree. The dirty cases cover tracked, staged,
// deleted, untracked, and output-larger-than-the process capture ceiling so a
// regression cannot restore whole-status buffering or omit one Git state.
func TestVerifyRepositoryPressuresEveryObservableCheckoutState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate func(*testing.T, repositoryFixture)
		name   string
		clean  bool
	}{
		{name: "positive unchanged checkout", clean: true},
		{name: "neutral lightweight tag leaves checkout clean", clean: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			runRepositoryGitForTest(t, fixture, "tag", "reviewed")
		}},
		{name: "negative tracked modification is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "negative staged addition is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "staged.txt", body: "staged\n"})
			runRepositoryGitForTest(t, fixture, "add", "--", "staged.txt")
		}},
		{name: "negative tracked deletion is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			location := releaseFixtureLocation(t, absolutePathForTest(t, filepath.Join(fixture.root.String(), "tracked.txt")))
			if err := filestore.Remove(t.Context(), filestore.RemovalRequest{Location: location}); err != nil {
				t.Fatalf("filestore.Remove(tracked file) error = %v, want nil", err)
			}
		}},
		{name: "negative untracked addition is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "untracked.txt", body: "untracked\n"})
		}},
		{name: "boundary oversized status is still typed dirty", mutate: writeOversizedRepositoryStatusForTest},
		// The next three rows are the reason VerifyRepository passes Git policy
		// on the command line. Each installs a machine-wide setting through
		// HOME, which the request contract cannot refuse because it is not a
		// GIT_ variable. Without the policy arguments the first two report a
		// dirty worktree as clean, which is a false release admission.
		{name: "negative global excludes file cannot hide an untracked file", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeGlobalGitExcludesForTest(t, fixture, "*.txt\n")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "hidden.txt", body: "untracked\n"})
		}},
		{name: "negative global excludes wildcard cannot hide every untracked file", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeGlobalGitExcludesForTest(t, fixture, "*\n")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "hidden.dat", body: "untracked\n"})
		}},
		{name: "neutral global excludes file does not fabricate dirt in a clean checkout", clean: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeGlobalGitExcludesForTest(t, fixture, "*\n")
		}},
		{name: "negative tracked modification survives a global excludes file", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeGlobalGitExcludesForTest(t, fixture, "*\n")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "negative repository info attributes cannot normalize a tracked modification", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryInfoAttributesForTest(t, fixture, "*.txt filter=normalize-review\n")
			runRepositoryGitForTest(t, fixture, "config", "filter.normalize-review.clean", "printf 'initial\\n'")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "negative local attributes file cannot normalize a tracked modification", mutate: func(t *testing.T, fixture repositoryFixture) {
			attributes := filepath.Join(fixture.home, "local-attributes")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: absolutePathForTest(t, fixture.home), name: "local-attributes", body: "*.txt filter=normalize-review\n"})
			runRepositoryGitForTest(t, fixture, "config", "core.attributesFile", attributes)
			runRepositoryGitForTest(t, fixture, "config", "filter.normalize-review.clean", "printf 'initial\\n'")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "negative nonempty repository info attributes policy is unverifiable", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryInfoAttributesForTest(t, fixture, "# local policy is still uncommitted input\n")
		}},
		{name: "neutral empty repository info attributes file leaves checkout clean", clean: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryInfoAttributesForTest(t, fixture, "")
		}},
		{name: "negative repository info exclude cannot hide an untracked file", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryInfoExcludeForTest(t, fixture, "*\n")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "hidden-by-info.dat", body: "untracked\n"})
		}},
		{name: "negative local file mode policy cannot hide a mode change", mutate: func(t *testing.T, fixture repositoryFixture) {
			runRepositoryGitForTest(t, fixture, "config", "core.fileMode", "false")
			location := releaseFixtureLocation(t, absolutePathForTest(t, filepath.Join(fixture.root.String(), "tracked.txt")))
			if err := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{Location: location, Mode: 0o700}); err != nil {
				t.Fatalf("filestore.SetPermissions(tracked file) error = %v, want nil", err)
			}
		}},
		{name: "negative weakened stat policy cannot hide same size content replacement", mutate: func(t *testing.T, fixture repositoryFixture) {
			path := absolutePathForTest(t, filepath.Join(fixture.root.String(), "tracked.txt"))
			observed, err := filestore.Inspect(t.Context(), path)
			if err != nil {
				t.Fatalf("filestore.Inspect(tracked file) error = %v, want nil", err)
			}
			stamp, err := observed.ModifiedAt()
			if err != nil {
				t.Fatalf("Inspection.ModifiedAt() error = %v, want nil", err)
			}
			runRepositoryGitForTest(t, fixture, "config", "core.trustctime", "false")
			runRepositoryGitForTest(t, fixture, "config", "core.checkStat", "minimal")
			location := releaseFixtureLocation(t, path)
			held, err := filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{Location: location})
			if err != nil {
				t.Fatalf("filestore.OpenUpdate(tracked file) error = %v, want nil", err)
			}
			defer func() {
				if err := held.Close(); err != nil {
					t.Errorf("tracked update Close() error = %v, want nil", err)
				}
			}()
			replacement := []byte("changed\n")
			if count, err := held.WriteAt(replacement, 0); err != nil || count != len(replacement) {
				t.Fatalf("tracked in-place WriteAt() = (%d, %v), want (%d, nil)", count, err, len(replacement))
			}
			if err := filestore.Touch(t.Context(), filestore.TouchRequest{Location: location, ModifiedAt: stamp}); err != nil {
				t.Fatalf("filestore.Touch(tracked file) error = %v, want nil", err)
			}
			standing, err := filestore.ObserveHeldStanding(t.Context(), held, path)
			if err != nil || standing != filestore.HeldStandingSame {
				t.Fatalf("in-place standing = (%v, %v), want same and nil", standing, err)
			}
		}},
		{name: "negative assume unchanged index bit cannot hide a tracked modification", mutate: func(t *testing.T, fixture repositoryFixture) {
			runRepositoryGitForTest(t, fixture, "update-index", "--assume-unchanged", "--", "tracked.txt")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "negative skip worktree index bit cannot hide a tracked modification", mutate: func(t *testing.T, fixture repositoryFixture) {
			runRepositoryGitForTest(t, fixture, "update-index", "--skip-worktree", "--", "tracked.txt")
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "changed\n"})
		}},
		{name: "neutral detached HEAD at the same commit stays clean", clean: true, mutate: func(t *testing.T, fixture repositoryFixture) {
			runRepositoryGitForTest(t, fixture, "checkout", "--quiet", "--detach", "HEAD")
		}},
		{name: "negative untracked file inside a nested directory is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			ensureRepositoryDirectoryForTest(t, fixture.root, filepath.Join("nested", "deeper"))
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: filepath.Join("nested", "deeper", "buried.txt"), body: "x\n"})
		}},
		{name: "negative empty untracked file with no content is still dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "empty.txt"})
		}},
		{name: "negative staged deletion of the only tracked file is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			runRepositoryGitForTest(t, fixture, "rm", "--quiet", "--", "tracked.txt")
		}},
		{name: "negative tracked file replaced by a directory is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			location := releaseFixtureLocation(t, absolutePathForTest(t, filepath.Join(fixture.root.String(), "tracked.txt")))
			if err := filestore.Remove(t.Context(), filestore.RemovalRequest{Location: location}); err != nil {
				t.Fatalf("filestore.Remove(tracked file) error = %v, want nil", err)
			}
			ensureRepositoryDirectoryForTest(t, fixture.root, "tracked.txt")
		}},
		{name: "negative mode-only change on a tracked file is dirty", mutate: func(t *testing.T, fixture repositoryFixture) {
			location := releaseFixtureLocation(t, absolutePathForTest(t, filepath.Join(fixture.root.String(), "tracked.txt")))
			if err := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{Location: location, Mode: 0o700}); err != nil {
				t.Fatalf("filestore.SetPermissions(tracked file) error = %v, want nil", err)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newRepositoryFixtureAt(t, t.TempDir(), t.TempDir())
			if tc.mutate != nil {
				tc.mutate(t, fixture)
			}
			verified, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, fixture))
			if tc.clean {
				if err != nil {
					t.Fatalf("VerifyRepository(clean checkout) error = %v, want nil", err)
				}
				if verified.Root() != fixture.root || verified.GitExecutable() != fixture.git || verified.Commit() != fixture.commit {
					t.Fatalf("VerifyRepository(clean checkout) facts = (%v, %v, %v), want (%v, %v, %v)", verified.Root(), verified.GitExecutable(), verified.Commit(), fixture.root, fixture.git, fixture.commit)
				}
				return
			}
			var dirty release.RepositoryDirtyError
			if !errors.As(err, &dirty) || !errors.Is(err, core.ErrReleaseContract) {
				t.Fatalf("VerifyRepository(dirty checkout) error = %v, want typed dirty error and %v", err, core.ErrReleaseContract)
			}
			if dirty.Root() != fixture.root {
				t.Fatalf("RepositoryDirtyError.Root() = %v, want %v", dirty.Root(), fixture.root)
			}
			if verified != (release.VerifiedRepository{}) {
				t.Fatalf("VerifyRepository(dirty checkout) returned nonzero proof")
			}
		})
	}
}

// TestVerifyRepositoryRejectsSubstitutionAndHostileCapabilities proves the
// release boundary refuses commit substitution, ambient or Git-owned policy,
// cancellation, non-repositories, and a missing executable before returning a
// proof. It also pins typed mismatch facts and the native process failure.
func TestVerifyRepositoryRejectsSubstitutionAndHostileCapabilities(t *testing.T) {
	t.Parallel()

	if err := (release.RepositoryCommitMismatchError{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("zero RepositoryCommitMismatchError.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}
	if err := (release.RepositoryDirtyError{}).Validate(); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("zero RepositoryDirtyError.Validate() error = %v, want %v", err, core.ErrReleaseContract)
	}

	fixture := newRepositoryFixtureAt(t, t.TempDir(), t.TempDir())
	request := repositoryRequestForTest(t, fixture)
	environmentCases := []struct {
		name  string
		value process.Environment
	}{
		{name: "ambient inheritance", value: process.Environment{Mode: process.EnvironmentModeInherit}},
		{name: "uppercase Git policy", value: exactEnvironmentForTest(t, []string{"GIT_DIR=foreign"})},
		{name: "case-folded Git policy", value: exactEnvironmentForTest(t, []string{"git_work_tree=foreign"})},
	}
	for _, tc := range environmentCases {
		candidate := request
		candidate.Environment = tc.value
		if err := candidate.Validate(); err == nil || !errors.Is(err, core.ErrReleaseContract) {
			t.Fatalf("RepositoryVerificationRequest.Validate(%s) error = %v, want %v", tc.name, err, core.ErrReleaseContract)
		}
	}
	foreignCommit, err := core.ParseBuildCommit(strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("ParseBuildCommit(foreign) error = %v", err)
	}
	mismatched := request
	mismatched.ExpectedCommit = foreignCommit
	verified, mismatchErr := release.VerifyRepository(t.Context(), mismatched)
	var mismatch release.RepositoryCommitMismatchError
	if !errors.As(mismatchErr, &mismatch) || !errors.Is(mismatchErr, core.ErrReleaseContract) {
		t.Fatalf("VerifyRepository(substituted commit) error = %v, want typed mismatch and %v", mismatchErr, core.ErrReleaseContract)
	}
	if mismatch.Expected() != foreignCommit || mismatch.Observed() != fixture.commit {
		t.Fatalf("RepositoryCommitMismatchError facts = (%v, %v), want (%v, %v)", mismatch.Expected(), mismatch.Observed(), foreignCommit, fixture.commit)
	}
	if verified != (release.VerifiedRepository{}) {
		t.Fatalf("VerifyRepository(substituted commit) returned nonzero proof")
	}

	cases := []struct {
		invoke            func(context.Context, release.RepositoryVerificationRequest) (release.VerifiedRepository, error)
		mutate            func(*testing.T, *release.RepositoryVerificationRequest)
		name              string
		wantNativeFailure bool
	}{
		{name: "nil context", invoke: func(_ context.Context, request release.RepositoryVerificationRequest) (release.VerifiedRepository, error) {
			var nilContext context.Context
			return release.VerifyRepository(nilContext, request)
		}},
		{name: "cancelled context", invoke: func(_ context.Context, request release.RepositoryVerificationRequest) (release.VerifiedRepository, error) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return release.VerifyRepository(ctx, request)
		}},
		{name: "unset repository root", mutate: func(_ *testing.T, request *release.RepositoryVerificationRequest) {
			request.Root = core.AbsolutePath{}
		}},
		{name: "unset Git executable", mutate: func(_ *testing.T, request *release.RepositoryVerificationRequest) {
			request.GitExecutable = core.AbsolutePath{}
		}},
		{name: "unset expected commit", mutate: func(_ *testing.T, request *release.RepositoryVerificationRequest) {
			request.ExpectedCommit = core.BuildCommit{}
		}},
		{name: "zero wait delay", mutate: func(_ *testing.T, request *release.RepositoryVerificationRequest) {
			request.WaitDelay = temporal.Duration{}
		}},
		{name: "ambient environment", mutate: func(_ *testing.T, request *release.RepositoryVerificationRequest) {
			request.Environment = process.Environment{Mode: process.EnvironmentModeInherit}
		}},
		{name: "uppercase Git policy environment", mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			request.Environment = exactEnvironmentForTest(t, []string{"GIT_DIR=foreign"})
		}},
		{name: "case-folded Git policy environment", mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			request.Environment = exactEnvironmentForTest(t, []string{"git_work_tree=foreign"})
		}},
		{name: "valid directory that is not a repository", mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			request.Root = absolutePathForTest(t, t.TempDir())
		}},
		{name: "missing Git executable preserves native process failure", wantNativeFailure: true, mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			request.GitExecutable = absolutePathForTest(t, filepath.Join(t.TempDir(), "missing-git"))
		}},
		{name: "directory supplied as the Git executable preserves native process failure", wantNativeFailure: true, mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			request.GitExecutable = absolutePathForTest(t, t.TempDir())
		}},
		{name: "repository root that does not exist", mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			request.Root = absolutePathForTest(t, filepath.Join(t.TempDir(), "absent"))
		}},
		{name: "repository root that is a regular file", mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			directory := absolutePathForTest(t, t.TempDir())
			writeRepositoryFileForTest(t, repositoryFileWrite{root: directory, name: "regular", body: "x"})
			request.Root = absolutePathForTest(t, filepath.Join(directory.String(), "regular"))
		}},
		{name: "repository with no commits cannot resolve HEAD", mutate: func(t *testing.T, request *release.RepositoryVerificationRequest) {
			empty := repositoryFixture{
				home: t.TempDir(), root: absolutePathForTest(t, t.TempDir()), git: request.GitExecutable,
			}
			empty.environment = repositoryEnvironmentForTest(t, empty.home)
			runRepositoryGitForTest(t, empty, "init", "--quiet")
			request.Root = empty.root
			request.Environment = empty.environment
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidate := request
			if tc.mutate != nil {
				tc.mutate(t, &candidate)
			}
			invoke := tc.invoke
			if invoke == nil {
				invoke = release.VerifyRepository
			}
			proof, err := invoke(t.Context(), candidate)
			if err == nil || !errors.Is(err, core.ErrReleaseContract) {
				t.Fatalf("VerifyRepository(hostile capability) error = %v, want %v", err, core.ErrReleaseContract)
			}
			if proof != (release.VerifiedRepository{}) {
				t.Fatalf("VerifyRepository(hostile capability) returned nonzero proof")
			}
			if tc.wantNativeFailure {
				var failure process.Failure
				if !errors.As(err, &failure) || failure.Command() != candidate.GitExecutable {
					t.Fatalf("VerifyRepository(%s) error = %v, want process failure for %v", tc.name, err, candidate.GitExecutable)
				}
			}
		})
	}
}

// TestVerifyRepositoryRefusesDirtySubmoduleDespiteRepositoryIgnorePolicy proves
// the parent repository cannot suppress a changed child checkout with
// submodule.<name>.ignore=all. The verifier asks Git to observe every submodule
// explicitly, while Git still owns traversal and native repository semantics.
func TestVerifyRepositoryRefusesDirtySubmoduleDespiteRepositoryIgnorePolicy(t *testing.T) {
	t.Parallel()

	parent := newRepositoryFixtureAt(t, t.TempDir(), t.TempDir())
	child := newRepositoryFixtureAt(t, t.TempDir(), t.TempDir())
	runRepositoryGitForTest(t, parent,
		"-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "--",
		child.root.String(), "dependency",
	)
	runRepositoryGitForTest(t, parent, "commit", "--quiet", "-am", "add dependency")
	parent.commit = repositoryHeadForTest(t, parent)
	runRepositoryGitForTest(t, parent, "config", "submodule.dependency.ignore", "all")
	writeRepositoryFileForTest(t, repositoryFileWrite{
		root: parent.root, name: filepath.Join("dependency", "tracked.txt"), body: "changed\n",
	})

	proof, err := release.VerifyRepository(t.Context(), repositoryRequestForTest(t, parent))
	var dirty release.RepositoryDirtyError
	if !errors.As(err, &dirty) || !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("VerifyRepository(dirty ignored submodule) error = %v, want typed dirty error and %v", err, core.ErrReleaseContract)
	}
	if proof != (release.VerifiedRepository{}) {
		t.Fatalf("VerifyRepository(dirty ignored submodule) returned nonzero proof")
	}
}

func newRepositoryFixtureAt(t *testing.T, rootText, home string) repositoryFixture {
	t.Helper()
	name, err := core.ParsePathComponent("git")
	if err != nil {
		t.Fatalf("ParsePathComponent(git) error = %v, want nil", err)
	}
	git, err := process.Resolve(t.Context(), name)
	if err != nil {
		t.Fatalf("process.Resolve(git) error = %v, want nil", err)
	}
	fixture := repositoryFixture{
		home:        home,
		root:        absolutePathForTest(t, rootText),
		git:         git,
		environment: repositoryEnvironmentForTest(t, home),
	}
	runRepositoryGitForTest(t, fixture, "init", "--quiet")
	runRepositoryGitForTest(t, fixture, "config", "user.email", "release@example.invalid")
	runRepositoryGitForTest(t, fixture, "config", "user.name", "Primitive Release Test")
	writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: "tracked.txt", body: "initial\n"})
	runRepositoryGitForTest(t, fixture, "add", "--", "tracked.txt")
	runRepositoryGitForTest(t, fixture, "commit", "--quiet", "-m", "initial")
	fixture.commit = repositoryHeadForTest(t, fixture)
	return fixture
}

func repositoryHeadForTest(t *testing.T, fixture repositoryFixture) core.BuildCommit {
	t.Helper()
	commitText := strings.TrimSpace(runRepositoryGitForTest(t, fixture, "rev-parse", "--verify", "HEAD"))
	commit, err := core.ParseBuildCommit(commitText)
	if err != nil {
		t.Fatalf("ParseBuildCommit(repository HEAD) error = %v", err)
	}
	return commit
}

func repositoryRequestForTest(t *testing.T, fixture repositoryFixture) release.RepositoryVerificationRequest {
	t.Helper()
	wait, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatalf("DurationFromSeconds(wait) error = %v", err)
	}
	return release.RepositoryVerificationRequest{
		Root: fixture.root, GitExecutable: fixture.git, ExpectedCommit: fixture.commit,
		Environment: fixture.environment, WaitDelay: wait,
	}
}

// repositoryEnvironmentForTest builds an exact environment instead of
// forwarding the developer's own. Ambient inheritance would make the verdict
// depend on the operator's global gitconfig: commit.gpgsign would break the
// fixture commit, and core.excludesFile would decide whether an untracked file
// is even observable. home is a per-fixture directory so every global Git
// setting this suite observes is one the suite wrote itself.
func repositoryEnvironmentForTest(t *testing.T, home string) process.Environment {
	t.Helper()
	values := []string{"HOME=" + home, "USERPROFILE=" + home}
	ambient, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatalf("hostfacts.AmbientEnvironment() error = %v, want nil", err)
	}
	for _, variable := range ambient.Variables {
		name, err := variable.Name.Value()
		if err != nil {
			t.Fatalf("EnvironmentName.Value() error = %v, want nil", err)
		}
		if name != "PATH" && name != "SYSTEMROOT" {
			continue
		}
		value, err := variable.Value.Value()
		if err != nil {
			t.Fatalf("EnvironmentValue.Value() error = %v, want nil", err)
		}
		values = append(values, name+"="+value)
	}
	return exactEnvironmentForTest(t, values)
}

// writeGlobalGitExcludesForTest installs a machine-wide ignore rule through the
// same channel a developer workstation uses. Nothing in
// RepositoryVerificationRequest can refuse it: core.excludesFile is reached
// through HOME, not through a GIT_ variable. Without the command-line policy
// arguments, porcelain status omits every matching file and an untracked
// worktree reports itself clean.
func writeGlobalGitExcludesForTest(t *testing.T, fixture repositoryFixture, patterns string) {
	t.Helper()
	home := absolutePathForTest(t, fixture.home)
	excludes := filepath.Join(fixture.home, "global-excludes")
	writeRepositoryFileForTest(t, repositoryFileWrite{root: home, name: "global-excludes", body: patterns})
	config := "[core]\n\texcludesFile = " + excludes + "\n"
	writeRepositoryFileForTest(t, repositoryFileWrite{root: home, name: ".gitconfig", body: config})
}

// writeRepositoryInfoExcludeForTest installs an ignore rule in repository
// metadata rather than HOME. core.excludesFile does not control this channel;
// the status invocation must explicitly report ignored paths or an untracked
// build input can disappear from the cleanliness observation.
func writeRepositoryInfoExcludeForTest(t *testing.T, fixture repositoryFixture, patterns string) {
	t.Helper()
	ensureRepositoryDirectoryForTest(t, fixture.root, filepath.Join(".git", "info"))
	writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: filepath.Join(".git", "info", "exclude"), body: patterns})
}

func writeRepositoryInfoAttributesForTest(t *testing.T, fixture repositoryFixture, attributes string) {
	t.Helper()
	ensureRepositoryDirectoryForTest(t, fixture.root, filepath.Join(".git", "info"))
	writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: filepath.Join(".git", "info", "attributes"), body: attributes})
}

func ensureRepositoryDirectoryForTest(t *testing.T, directory core.AbsolutePath, name string) {
	t.Helper()
	root, err := filestore.OpenRoot(t.Context(), directory)
	if err != nil {
		t.Fatalf("filestore.OpenRoot(fixture directory) error = %v, want nil", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Errorf("fixture directory root Close() error = %v, want nil", err)
		}
	}()
	path, err := core.ParseRelativePath(name)
	if err != nil {
		t.Fatalf("ParseRelativePath(fixture directory) error = %v, want nil", err)
	}
	if err := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o750}); err != nil {
		t.Fatalf("filestore.EnsureDirectory(%s) error = %v, want nil", name, err)
	}
}

func exactEnvironmentForTest(t *testing.T, values []string) process.Environment {
	t.Helper()
	environment, err := process.ParseEffectiveEnvironment(values)
	if err != nil {
		t.Fatalf("ParseEffectiveEnvironment() error = %v", err)
	}
	return environment
}

func runRepositoryGitForTest(t *testing.T, fixture repositoryFixture, arguments ...string) string {
	t.Helper()
	// commit.gpgsign is refused explicitly: a system-wide gitconfig that turns
	// signing on would otherwise fail every fixture commit on the operator's
	// machine and nowhere else. The fixture deliberately does not neutralize
	// core.excludesFile, because one case installs that setting on purpose.
	hooks := absolutePathForTest(t, filepath.Join(fixture.home, "empty-hooks"))
	location, err := filestore.OpenParent(t.Context(), hooks)
	if err != nil {
		t.Fatalf("filestore.OpenParent(hooks) error = %v, want nil", err)
	}
	err = filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{Location: location, Mode: 0o750})
	if err = errors.Join(err, location.Root.Close()); err != nil {
		t.Fatalf("prepare hooks error = %v, want nil", err)
	}
	arguments = append([]string{
		"-c", "commit.gpgsign=false",
		"-c", "init.templateDir=",
		"-c", "core.hooksPath=" + hooks.String(),
	}, arguments...)
	base, err := fixture.environment.Strings()
	if err != nil {
		t.Fatalf("Environment.Strings() error = %v", err)
	}
	commandEnvironment := exactEnvironmentForTest(t, append(base,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_ATTR_NOSYSTEM=1",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
	))
	args, err := process.ParseArguments(arguments)
	if err != nil {
		t.Fatalf("process.ParseArguments(git) error = %v, want nil", err)
	}
	wait, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds() error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(4 << 20)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return runFixtureProcess(t.Context(), t, process.Request{
		Command: fixture.git, WorkingDirectory: fixture.root, Arguments: args,
		Environment: commandEnvironment, WaitDelay: wait, OutputLimit: limit,
		Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalKill},
	})
}

func runFixtureProcess(ctx context.Context, t testing.TB, request process.Request) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	request.Streams = process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	result, err := process.Run(ctx, request)
	if err != nil {
		t.Fatalf("process.Run(fixture) error = %v, want nil; stderr = %q", err, stderr.String())
	}
	exit, err := result.ExitCode()
	if err != nil {
		t.Fatalf("process.Result.ExitCode() error = %v, want nil", err)
	}
	success, err := exit.Success()
	if err != nil || !success {
		t.Fatalf("fixture process success = %t, error = %v, want true, nil; stderr = %q", success, err, stderr.String())
	}
	return stdout.String()
}

func writeRepositoryFileForTest(t *testing.T, request repositoryFileWrite) {
	t.Helper()
	path, err := request.root.ResolveText(request.name)
	if err != nil {
		t.Fatalf("AbsolutePath.ResolveText(fixture) error = %v, want nil", err)
	}
	location, err := filestore.OpenParent(t.Context(), path)
	if err != nil {
		t.Fatalf("filestore.OpenParent(fixture) error = %v, want nil", err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Errorf("fixture root Close() error = %v, want nil", err)
		}
	}()
	temporary, err := core.ParseRelativePath("release-fixture-stage")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(stage) error = %v, want nil", err)
	}
	recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: strings.NewReader(request.body), Location: location, Temporary: temporary,
		Mode: 0o600, Install: filestore.InstallReplace,
	})
	if err != nil {
		t.Fatalf("filestore.Write(%s) error = %v, recovery = %v, want nil", request.name, err, recovery)
	}
}

func writeOversizedRepositoryStatusForTest(t *testing.T, fixture repositoryFixture) {
	t.Helper()
	for index := range 32 {
		name := strings.Repeat(string(rune('a'+index%26)), 180) + string(rune('A'+index%26))
		writeRepositoryFileForTest(t, repositoryFileWrite{root: fixture.root, name: name, body: "dirty\n"})
	}
}

func absolutePathForTest(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v", value, err)
	}
	return path
}
