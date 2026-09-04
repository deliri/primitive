package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
)

// Git command and environment contracts:
// https://git-scm.com/docs/git-ls-files
// https://git-scm.com/docs/git#Documentation/git.txt-codeGITCONFIGNOSYSTEMcode
const (
	gitEnvironmentPrefix   = "GIT_"
	gitConfigNoSystem      = "GIT_CONFIG_NOSYSTEM=1"
	gitConfigGlobal        = "GIT_CONFIG_GLOBAL="
	gitAttributesNoSystem  = "GIT_ATTR_NOSYSTEM=1"
	gitOptionalLocksOff    = "GIT_OPTIONAL_LOCKS=0"
	gitTerminalPromptOff   = "GIT_TERMINAL_PROMPT=0"
	gitNoOptionalLocks     = "--no-optional-locks"
	gitConfigOverride      = "-c"
	gitNoGlobalExcludes    = "core.excludesFile="
	gitNoFilesystemMonitor = "core.fsmonitor=false"
	gitListFiles           = "ls-files"
	gitCached              = "--cached"
	gitOthers              = "--others"
	gitRepositoryIgnore    = "--exclude-per-directory=.gitignore"
	gitDeduplicate         = "--deduplicate"
	gitNullTerminate       = "-z"
	gitArgumentSeparator   = "--"
)

// Capability is one resolved Git repository observation boundary.
type Capability struct {
	command       core.AbsolutePath
	environment   process.Environment
	configuration Configuration
}

// Open resolves Git and captures one exact policy-neutral child environment.
func Open(ctx context.Context, configuration Configuration) (Capability, error) {
	if ctx == nil {
		return Capability{}, contractError(errors.New("git capability context is nil"))
	}
	if err := configuration.Validate(); err != nil {
		return Capability{}, err
	}
	name, err := core.ParsePathComponent("git")
	if err != nil {
		return Capability{}, contractError(err)
	}
	command, err := process.Resolve(ctx, name)
	if err != nil {
		return Capability{}, executionError(err)
	}
	ambient, err := hostfacts.AmbientEnvironment()
	if err != nil {
		return Capability{}, executionError(err)
	}
	environment, err := exactGitEnvironment(ambient)
	if err != nil {
		return Capability{}, err
	}
	capability := Capability{command: command, environment: environment, configuration: configuration}
	return capability, capability.Validate()
}

func (c Capability) Validate() error {
	if err := errors.Join(c.command.Validate(), c.environment.Validate(), c.configuration.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// StreamWorktree emits Git's tracked and repository-unignored file paths in
// Git's exact stream order without retaining the repository path set.
func (c Capability) StreamWorktree(ctx context.Context, request WorktreeRequest, consumer WorktreeConsumer) (WorktreeSummary, error) {
	if ctx == nil || consumer == nil {
		return WorktreeSummary{}, contractError(errors.New("worktree stream dependency is nil"))
	}
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return WorktreeSummary{}, contractError(err)
	}
	maximum, err := processOutputMaximum()
	if err != nil {
		return WorktreeSummary{}, err
	}
	arguments, err := process.ParseArguments(worktreeArguments(request.Selection))
	if err != nil {
		return WorktreeSummary{}, contractError(err)
	}
	writer := worktreeEntryWriter{consumer: consumer}
	result, runErr := process.Run(ctx, process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: &writer, Stderr: io.Discard},
		Command: c.command, WorkingDirectory: request.Root, Arguments: arguments, Environment: c.environment,
		OutputLimit: maximum, WaitDelay: c.configuration.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalTerminate},
	})
	if writer.failure != nil {
		return WorktreeSummary{}, writer.failure
	}
	if runErr != nil {
		return WorktreeSummary{}, executionError(runErr)
	}
	if err := requireSuccess(result); err != nil {
		return WorktreeSummary{}, err
	}
	return writer.finish()
}

func worktreeArguments(selection WorktreeSelection) []string {
	if selection != WorktreeSelectionTrackedAndUnignored {
		return nil
	}
	return []string{
		gitNoOptionalLocks,
		gitConfigOverride, gitNoGlobalExcludes,
		gitConfigOverride, gitNoFilesystemMonitor,
		gitListFiles, gitCached, gitOthers, gitRepositoryIgnore,
		gitDeduplicate, gitNullTerminate, gitArgumentSeparator,
	}
}

func exactGitEnvironment(ambient process.Environment) (process.Environment, error) {
	values, err := ambient.Strings()
	if err != nil || values == nil {
		return process.Environment{}, contractError(errors.Join(errors.New("ambient Git environment is invalid"), err))
	}
	filtered := make([]string, 0, len(values)+5)
	for _, value := range values {
		name, _, _ := strings.Cut(value, "=")
		if len(name) >= len(gitEnvironmentPrefix) && strings.EqualFold(name[:len(gitEnvironmentPrefix)], gitEnvironmentPrefix) {
			continue
		}
		filtered = append(filtered, value)
	}
	filtered = append(filtered,
		gitConfigNoSystem,
		gitConfigGlobal+os.DevNull,
		gitAttributesNoSystem,
		gitOptionalLocksOff,
		gitTerminalPromptOff,
	)
	environment, err := process.ParseExactEnvironment(filtered)
	if err != nil {
		return process.Environment{}, contractError(err)
	}
	return environment, nil
}

func requireSuccess(result process.Result) error {
	exit, err := result.ExitCode()
	if err != nil {
		return executionError(err)
	}
	success, err := exit.Success()
	if err != nil || !success {
		return executionError(err)
	}
	return nil
}

type worktreeEntryWriter struct {
	consumer WorktreeConsumer
	path     [core.SourcePathMaximumBytes]byte
	length   uint16
	entries  uint64
	bytes    uint64
	failure  error
}

func (w *worktreeEntryWriter) Write(data []byte) (int, error) {
	if w == nil || w.consumer == nil {
		return 0, contractError(errors.New("worktree writer is invalid"))
	}
	if w.failure != nil {
		return 0, w.failure
	}
	for index, value := range data {
		if err := w.consume(value); err != nil {
			w.failure = err
			return index + 1, err
		}
	}
	return len(data), nil
}

func (w *worktreeEntryWriter) consume(value byte) error {
	if value != 0 {
		if w.length >= core.SourcePathMaximumBytes {
			return outputError(errors.New("git path exceeds the source identity ceiling"))
		}
		w.path[w.length] = value
		w.length++
		return nil
	}
	path, err := core.ParseSourcePath(string(w.path[:w.length]))
	if err != nil || path.String() == "." {
		return outputError(errors.Join(errors.New("git emitted an invalid source path"), err))
	}
	entry := WorktreeEntry{Path: path}
	if err := entry.Validate(); err != nil {
		return outputError(err)
	}
	if err := w.consumer.ConsumeWorktreeEntry(entry); err != nil {
		return err
	}
	if w.entries == math.MaxUint64 {
		return outputError(errors.New("git entry count overflow"))
	}
	consumed := uint64(w.length) + 1
	if w.bytes > math.MaxInt64-consumed {
		return outputError(errors.New("git path stream byte count overflow"))
	}
	w.entries++
	w.bytes += consumed
	w.length = 0
	return nil
}

func (w *worktreeEntryWriter) finish() (WorktreeSummary, error) {
	if w == nil || w.consumer == nil {
		return WorktreeSummary{}, contractError(errors.New("worktree writer is invalid"))
	}
	if w.failure != nil {
		return WorktreeSummary{}, w.failure
	}
	if w.length != 0 {
		return WorktreeSummary{}, outputError(errors.New("git path stream is truncated"))
	}
	length, err := core.NewByteLength(w.bytes)
	if err != nil {
		return WorktreeSummary{}, outputError(err)
	}
	summary := WorktreeSummary{Entries: w.entries, Bytes: length}
	return summary, summary.Validate()
}
