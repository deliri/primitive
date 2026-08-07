package release

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	repositoryGitOutputMaximumBytes = 4 << 10
	repositoryGitEnvironmentPrefix  = "GIT_"
	repositoryGitRevParse           = "rev-parse"
	repositoryGitVerify             = "--verify"
	repositoryGitHead               = "HEAD"
	repositoryGitPathFormatAbsolute = "--path-format=absolute"
	repositoryGitPath               = "--git-path"
	repositoryGitInfoAttributes     = "info/attributes"
	repositoryGitStatus             = "status"
	repositoryGitPorcelainV1        = "--porcelain=v1"
	repositoryGitUntrackedAll       = "--untracked-files=all"
	repositoryGitIgnoredMatching    = "--ignored=matching"
	repositoryGitSubmodulesNone     = "--ignore-submodules=none"
	repositoryGitListFiles          = "ls-files"
	repositoryGitVerboseFlags       = "-v"
	repositoryGitNullTerminate      = "-z"
	repositoryGitArgumentSeparator  = "--"
	repositoryGitNoOptionalLocks    = "--no-optional-locks"
	repositoryGitConfigOverride     = "-c"
	repositoryGitNoAttributesFile   = "core.attributesFile="
	repositoryGitNoFsmonitor        = "core.fsmonitor=false"
	repositoryGitRespectFileMode    = "core.fileMode=true"
	repositoryGitTrustCTime         = "core.trustctime=true"
	repositoryGitDefaultStat        = "core.checkStat=default"
	repositoryGitNoIgnoreStat       = "core.ignoreStat=false"
	repositoryGitConfigNoSystem     = "GIT_CONFIG_NOSYSTEM=1"
	repositoryGitConfigGlobal       = "GIT_CONFIG_GLOBAL="
	repositoryGitAttributesNoSystem = "GIT_ATTR_NOSYSTEM=1"
	repositoryGitOptionalLocksOff   = "GIT_OPTIONAL_LOCKS=0"
	repositoryGitTerminalPromptOff  = "GIT_TERMINAL_PROMPT=0"
)

// repositoryGitPolicyArguments neutralize repository-local settings that can
// weaken this verdict. System and global configuration are removed separately
// by repositoryGitEnvironment. Optional locks are refused because verification
// observes the repository and must not refresh its index.
func repositoryGitPolicyArguments() []string {
	return []string{
		repositoryGitNoOptionalLocks,
		repositoryGitConfigOverride, repositoryGitNoAttributesFile,
		repositoryGitConfigOverride, repositoryGitNoFsmonitor,
		repositoryGitConfigOverride, repositoryGitRespectFileMode,
		repositoryGitConfigOverride, repositoryGitTrustCTime,
		repositoryGitConfigOverride, repositoryGitDefaultStat,
		repositoryGitConfigOverride, repositoryGitNoIgnoreStat,
	}
}

type repositoryGitOutputPolicy uint8

const (
	repositoryGitOutputUnknown repositoryGitOutputPolicy = iota
	repositoryGitOutputProbe
	repositoryGitOutputIndex
)

func (p repositoryGitOutputPolicy) maximumBytes() (uint64, error) {
	switch p {
	case repositoryGitOutputProbe:
		return repositoryGitOutputMaximumBytes, nil
	case repositoryGitOutputIndex:
		return math.MaxInt64, nil
	default:
		return 0, contractError(errors.New("repository Git output policy is invalid"))
	}
}

var errRepositoryStatusObserved = errors.New("repository status output observed")

// RepositoryVerificationRequest supplies the exact repository and process
// capability used to prove a release commit comes from a clean checkout.
type RepositoryVerificationRequest struct {
	Root           core.AbsolutePath
	GitExecutable  core.AbsolutePath
	Environment    process.Environment
	WaitDelay      temporal.Duration
	ExpectedCommit core.BuildCommit
}

// Validate closes the repository observation boundary before Git is started.
func (r RepositoryVerificationRequest) Validate() error {
	for _, err := range [...]error{
		r.Root.Validate(), r.GitExecutable.Validate(), r.ExpectedCommit.Validate(),
		r.Environment.Validate(), r.WaitDelay.Validate(),
	} {
		if err != nil {
			return contractError(errors.New("repository verification request is invalid"), err)
		}
	}
	if r.WaitDelay.IsZero() {
		return contractError(errors.New("repository verification wait delay is zero"))
	}
	environment, err := r.Environment.Strings()
	if err != nil {
		return contractError(errors.New("repository verification environment is invalid"), err)
	}
	if environment == nil {
		return contractError(errors.New("repository verification environment inherits ambient state"))
	}
	for _, variable := range environment {
		name, _, _ := strings.Cut(variable, "=")
		if len(name) >= len(repositoryGitEnvironmentPrefix) &&
			strings.EqualFold(name[:len(repositoryGitEnvironmentPrefix)], repositoryGitEnvironmentPrefix) {
			return contractError(errors.New("repository verification environment contains Git policy"))
		}
	}
	return nil
}

// VerifiedRepository is proof that the observed checkout was clean and its
// exact HEAD matched the requested release commit.
type VerifiedRepository struct {
	root   core.AbsolutePath
	commit core.BuildCommit
	valid  bool
}

// Validate rechecks the retained proof facts.
func (r VerifiedRepository) Validate() error {
	if !r.valid {
		return contractError(errors.New("verified repository is unset"))
	}
	for _, err := range [...]error{r.root.Validate(), r.commit.Validate()} {
		if err != nil {
			return contractError(errors.New("verified repository is invalid"), err)
		}
	}
	return nil
}

// Root and Commit return the exact repository facts that were verified.
func (r VerifiedRepository) Root() core.AbsolutePath  { return r.root }
func (r VerifiedRepository) Commit() core.BuildCommit { return r.commit }

// RepositoryCommitMismatchError carries both sides of a rejected HEAD binding.
type RepositoryCommitMismatchError struct {
	expected core.BuildCommit
	observed core.BuildCommit
}

func newRepositoryCommitMismatchError(
	expected core.BuildCommit,
	observed core.BuildCommit,
) (RepositoryCommitMismatchError, error) {
	value := RepositoryCommitMismatchError{expected: expected, observed: observed}
	if err := value.Validate(); err != nil {
		return RepositoryCommitMismatchError{}, err
	}
	return value, nil
}

// Validate proves both commits are canonical and differ.
func (e RepositoryCommitMismatchError) Validate() error {
	for _, err := range [...]error{e.expected.Validate(), e.observed.Validate()} {
		if err != nil {
			return contractError(errors.New("repository commit mismatch is invalid"), err)
		}
	}
	if e.expected == e.observed {
		return contractError(errors.New("repository commit mismatch contains equal commits"))
	}
	return nil
}

func (e RepositoryCommitMismatchError) Error() string {
	if e.Validate() != nil {
		return core.ErrReleaseContract.Error()
	}
	return "release: repository HEAD " + e.observed.String() +
		" differs from expected commit " + e.expected.String()
}

// Unwrap preserves the stable release-contract identity.
func (RepositoryCommitMismatchError) Unwrap() error { return core.ErrReleaseContract }

// Expected and Observed expose the exact rejected commit binding.
func (e RepositoryCommitMismatchError) Expected() core.BuildCommit { return e.expected }
func (e RepositoryCommitMismatchError) Observed() core.BuildCommit { return e.observed }

// RepositoryDirtyError identifies the exact checkout that produced status
// output. The output itself is deliberately neither retained nor disclosed.
type RepositoryDirtyError struct {
	root core.AbsolutePath
}

func newRepositoryDirtyError(root core.AbsolutePath) (RepositoryDirtyError, error) {
	value := RepositoryDirtyError{root: root}
	if err := value.Validate(); err != nil {
		return RepositoryDirtyError{}, err
	}
	return value, nil
}

// Validate proves the rejected repository identity.
func (e RepositoryDirtyError) Validate() error {
	if err := e.root.Validate(); err != nil {
		return contractError(errors.New("dirty repository identity is invalid"), err)
	}
	return nil
}

func (e RepositoryDirtyError) Error() string {
	if e.Validate() != nil {
		return core.ErrReleaseContract.Error()
	}
	return "release: repository is dirty: " + e.root.String()
}

// Unwrap preserves the stable release-contract identity.
func (RepositoryDirtyError) Unwrap() error { return core.ErrReleaseContract }

// Root returns the exact checkout that failed cleanliness verification.
func (e RepositoryDirtyError) Root() core.AbsolutePath { return e.root }

// VerifyRepository observes the real Git repository through Primitive Process.
// It retains only fixed-size commit and root facts; status output stops at the
// first byte and is never world-built in memory.
func VerifyRepository(
	ctx context.Context,
	request RepositoryVerificationRequest,
) (VerifiedRepository, error) {
	if ctx == nil {
		return VerifiedRepository{}, contractError(core.ErrNilContext)
	}
	if err := ctx.Err(); err != nil {
		return VerifiedRepository{}, contractError(err)
	}
	if err := request.Validate(); err != nil {
		return VerifiedRepository{}, err
	}
	if err := verifyRepositoryHead(ctx, request); err != nil {
		return VerifiedRepository{}, err
	}
	if err := verifyRepositoryPrivateAttributes(ctx, request); err != nil {
		return VerifiedRepository{}, err
	}
	if err := verifyRepositoryClean(ctx, request); err != nil {
		return VerifiedRepository{}, err
	}
	verified := VerifiedRepository{root: request.Root, commit: request.ExpectedCommit, valid: true}
	return verified, verified.Validate()
}

func verifyRepositoryPrivateAttributes(ctx context.Context, request RepositoryVerificationRequest) (resultErr error) {
	path, err := repositoryPrivateAttributesPath(ctx, request)
	if err != nil {
		return err
	}
	file, err := os.Open(path.String()) // #nosec G304 -- Git resolved its own private metadata path.
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return releaseError(core.ErrReleaseContract, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, releaseError(core.ErrReleaseContract, closeErr))
		}
	}()
	var first [1]byte
	count, err := file.Read(first[:])
	if count > 0 {
		return repositoryDirty(request.Root)
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		err = io.ErrNoProgress
	}
	return releaseError(core.ErrReleaseContract, err)
}

func repositoryPrivateAttributesPath(
	ctx context.Context,
	request RepositoryVerificationRequest,
) (core.AbsolutePath, error) {
	var stdout bytes.Buffer
	result, err := runRepositoryGit(ctx, repositoryGitRequest{
		verification: request,
		arguments: []string{
			repositoryGitRevParse, repositoryGitPathFormatAbsolute,
			repositoryGitPath, repositoryGitInfoAttributes,
		},
		stdout:       &stdout,
		outputPolicy: repositoryGitOutputProbe,
	})
	if err != nil {
		return core.AbsolutePath{}, err
	}
	if err := requireRepositoryGitSuccess(result); err != nil {
		return core.AbsolutePath{}, err
	}
	return parseRepositoryGitPathOutput(stdout.String())
}

func parseRepositoryGitPathOutput(output string) (core.AbsolutePath, error) {
	text, ok := strings.CutSuffix(output, "\n")
	if !ok || strings.ContainsAny(text, "\r\n") {
		return core.AbsolutePath{}, contractError(errors.New("repository Git path output is not canonical"))
	}
	path, err := core.ParseAbsolutePath(text)
	if err != nil || output != path.String()+"\n" {
		return core.AbsolutePath{}, contractError(errors.New("repository Git path output is invalid"), err)
	}
	return path, nil
}

func verifyRepositoryHead(ctx context.Context, request RepositoryVerificationRequest) error {
	var stdout bytes.Buffer
	result, err := runRepositoryGit(ctx, repositoryGitRequest{
		verification: request,
		arguments:    []string{repositoryGitRevParse, repositoryGitVerify, repositoryGitHead},
		stdout:       &stdout,
		outputPolicy: repositoryGitOutputProbe,
	})
	if err != nil {
		return err
	}
	if err := requireRepositoryGitSuccess(result); err != nil {
		return err
	}
	observed, err := parseRepositoryHeadOutput(stdout.String())
	if err != nil {
		return err
	}
	if observed == request.ExpectedCommit {
		return nil
	}
	mismatch, err := newRepositoryCommitMismatchError(request.ExpectedCommit, observed)
	if err != nil {
		return err
	}
	return mismatch
}

// parseRepositoryHeadOutput accepts only the exact bytes canonical Git emits
// for a resolved HEAD: one lowercase forty-hex commit and one trailing
// newline. Anything else means the observation is not the fact this boundary
// claims to have made, so no commit binding is derived from it. Trimming and
// re-projecting the parsed value is what rejects padded, repeated, prefixed,
// and case-shifted spellings that a lenient parse would accept.
func parseRepositoryHeadOutput(output string) (core.BuildCommit, error) {
	observed, err := core.ParseBuildCommit(strings.TrimSuffix(output, "\n"))
	if err != nil || output != observed.String()+"\n" {
		return core.BuildCommit{}, contractError(errors.New("repository HEAD output is not canonical"), err)
	}
	return observed, nil
}

func verifyRepositoryClean(ctx context.Context, request RepositoryVerificationRequest) error {
	if err := verifyRepositoryIndexFlags(ctx, request); err != nil {
		return err
	}
	result, err := runRepositoryGit(ctx, repositoryGitRequest{
		verification: request,
		arguments: []string{
			repositoryGitStatus, repositoryGitPorcelainV1,
			repositoryGitUntrackedAll, repositoryGitIgnoredMatching,
			repositoryGitSubmodulesNone, repositoryGitArgumentSeparator,
		},
		stdout:       repositoryStatusWriter{},
		outputPolicy: repositoryGitOutputProbe,
	})
	if errors.Is(err, errRepositoryStatusObserved) {
		return repositoryDirty(request.Root)
	}
	if err != nil {
		return err
	}
	return requireRepositoryGitSuccess(result)
}

func verifyRepositoryIndexFlags(ctx context.Context, request RepositoryVerificationRequest) error {
	writer := &repositoryIndexWriter{}
	result, err := runRepositoryGit(ctx, repositoryGitRequest{
		verification: request,
		arguments: []string{
			repositoryGitListFiles, repositoryGitVerboseFlags,
			repositoryGitNullTerminate, repositoryGitArgumentSeparator,
		},
		stdout:       writer,
		outputPolicy: repositoryGitOutputIndex,
	})
	if errors.Is(err, errRepositoryStatusObserved) {
		return repositoryDirty(request.Root)
	}
	if err != nil {
		return err
	}
	if err := requireRepositoryGitSuccess(result); err != nil {
		return err
	}
	return writer.finish()
}

func repositoryDirty(root core.AbsolutePath) error {
	dirty, err := newRepositoryDirtyError(root)
	if err != nil {
		return err
	}
	return dirty
}

type repositoryGitRequest struct {
	stdout       io.Writer
	arguments    []string
	verification RepositoryVerificationRequest
	outputPolicy repositoryGitOutputPolicy
}

func runRepositoryGit(ctx context.Context, request repositoryGitRequest) (process.Result, error) {
	arguments, err := process.ParseArguments(append(repositoryGitPolicyArguments(), request.arguments...))
	if err != nil {
		return process.Result{}, contractError(errors.New("repository Git arguments are invalid"), err)
	}
	maximumBytes, err := request.outputPolicy.maximumBytes()
	if err != nil {
		return process.Result{}, err
	}
	maximum, err := core.NewByteCount(maximumBytes)
	if err != nil {
		return process.Result{}, contractError(err)
	}
	environment, err := repositoryGitEnvironment(request.verification.Environment)
	if err != nil {
		return process.Result{}, err
	}
	result, err := process.Run(ctx, process.Request{
		Streams: process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: request.stdout, Stderr: io.Discard,
		},
		Command:          request.verification.GitExecutable,
		WorkingDirectory: request.verification.Root,
		Arguments:        arguments,
		Environment:      environment,
		OutputLimit:      maximum,
		WaitDelay:        request.verification.WaitDelay,
		Containment: process.Containment{
			Isolation:    process.IsolationDirect,
			CancelSignal: process.CancelSignalKill,
		},
	})
	if err != nil {
		return result, releaseError(core.ErrReleaseContract, err)
	}
	return result, nil
}

func repositoryGitEnvironment(base process.Environment) (process.Environment, error) {
	values, err := base.Strings()
	if err != nil || values == nil {
		return process.Environment{}, contractError(errors.New("repository Git base environment is invalid"), err)
	}
	values = append(values,
		repositoryGitConfigNoSystem,
		repositoryGitConfigGlobal+os.DevNull,
		repositoryGitAttributesNoSystem,
		repositoryGitOptionalLocksOff,
		repositoryGitTerminalPromptOff,
	)
	environment, err := process.ParseExactEnvironment(values)
	if err != nil {
		return process.Environment{}, contractError(errors.New("repository Git environment is invalid"), err)
	}
	return environment, nil
}

func requireRepositoryGitSuccess(result process.Result) error {
	exit, err := result.ExitCode()
	if err != nil {
		return contractError(errors.New("repository Git result is invalid"), err)
	}
	success, err := exit.Success()
	if err != nil {
		return contractError(errors.New("repository Git exit is invalid"), err)
	}
	if !success {
		return contractError(errors.New("repository Git command failed"))
	}
	return nil
}

type repositoryStatusWriter struct{}

func (repositoryStatusWriter) Write(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	return len(buffer), errRepositoryStatusObserved
}

type repositoryIndexState uint8

const (
	repositoryIndexExpectTag repositoryIndexState = iota
	repositoryIndexExpectSeparator
	repositoryIndexExpectPath
	repositoryIndexReadPath
)

// repositoryIndexWriter consumes the NUL-delimited `git ls-files -v -z`
// projection without retaining paths. A normal tracked entry starts with
// `H `. Lowercase tags are assume-unchanged entries; `S` is skip-worktree;
// every other tag is a non-normal index state that status policy must not hide.
type repositoryIndexWriter struct {
	state repositoryIndexState
}

func (w *repositoryIndexWriter) Write(buffer []byte) (int, error) {
	for _, value := range buffer {
		if err := w.consume(value); err != nil {
			return len(buffer), err
		}
	}
	return len(buffer), nil
}

func (w *repositoryIndexWriter) consume(value byte) error {
	switch w.state {
	case repositoryIndexExpectTag:
		if value != 'H' {
			return errRepositoryStatusObserved
		}
		w.state = repositoryIndexExpectSeparator
	case repositoryIndexExpectSeparator:
		if value != ' ' {
			return contractError(errors.New("repository index output separator is invalid"))
		}
		w.state = repositoryIndexExpectPath
	case repositoryIndexExpectPath:
		if value == 0 {
			return contractError(errors.New("repository index output path is empty"))
		}
		w.state = repositoryIndexReadPath
	case repositoryIndexReadPath:
		if value == 0 {
			w.state = repositoryIndexExpectTag
		}
	default:
		return contractError(errors.New("repository index output state is invalid"))
	}
	return nil
}

func (w repositoryIndexWriter) finish() error {
	if w.state != repositoryIndexExpectTag {
		return contractError(errors.New("repository index output is truncated"))
	}
	return nil
}
