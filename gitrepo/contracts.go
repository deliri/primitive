package gitrepo

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// WorktreeSelection is the closed Git-owned source-admission mechanism.
// Product policy decides whether this is the right source set for its task.
type WorktreeSelection uint8

const (
	WorktreeSelectionUnknown WorktreeSelection = iota
	// WorktreeSelectionTrackedAndUnignored emits tracked paths plus untracked
	// paths not excluded by repository-owned .gitignore files. Ambient global
	// excludes and repository-private info/exclude state do not participate.
	WorktreeSelectionTrackedAndUnignored
	worktreeSelectionLimit
)

func worktreeSelectionTexts() [worktreeSelectionLimit]string {
	return [...]string{
		WorktreeSelectionTrackedAndUnignored: "tracked-and-repository-unignored",
	}
}

func (s WorktreeSelection) Validate() error {
	if s != WorktreeSelectionTrackedAndUnignored {
		return contractError(errors.New("worktree selection is outside the admitted domain"))
	}
	return nil
}

func (s WorktreeSelection) IsValid() bool { return s.Validate() == nil }

func (WorktreeSelection) OffWireEnum() {}

func (s WorktreeSelection) String() string {
	if s.IsValid() {
		return worktreeSelectionTexts()[s]
	}
	return core.UnknownEnumDiagnostic
}

// Configuration bounds cancellation cleanup while leaving total work under
// the caller's context. The streamed output limit is the platform's exact
// representable process-I/O extent, not a project-size policy ceiling.
type Configuration struct {
	WaitDelay temporal.Duration
}

// DefaultConfiguration returns Primitive's Git process cleanup contract.
func DefaultConfiguration() (Configuration, error) {
	wait, err := temporal.DurationFromSeconds(30)
	if err != nil {
		return Configuration{}, contractError(err)
	}
	configuration := Configuration{WaitDelay: wait}
	return configuration, configuration.Validate()
}

func (c Configuration) Validate() error {
	if err := c.WaitDelay.Validate(); err != nil || c.WaitDelay.IsZero() {
		return contractError(errors.Join(errors.New("git cleanup delay is invalid"), err))
	}
	return nil
}

// WorktreeRequest identifies one local Git worktree source stream.
type WorktreeRequest struct {
	Root      core.AbsolutePath
	Selection WorktreeSelection
}

func (r WorktreeRequest) Validate() error {
	if err := errors.Join(r.Root.Validate(), r.Selection.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// WorktreeEntry is one canonical repository-relative path emitted by Git.
type WorktreeEntry struct {
	Path core.SourcePath
}

func (e WorktreeEntry) Validate() error {
	if err := e.Path.Validate(); err != nil || e.Path.String() == "." {
		return contractError(errors.Join(errors.New("worktree entry path is invalid"), err))
	}
	return nil
}

// WorktreeConsumer receives one validated entry at a time. Entries remain
// provisional until StreamWorktree returns a validated summary; callers must
// not publish a projection from a failed stream.
type WorktreeConsumer interface {
	ConsumeWorktreeEntry(WorktreeEntry) error
}

// WorktreeSummary accounts for the complete successfully consumed stream.
type WorktreeSummary struct {
	Entries uint64
	Bytes   core.ByteLength
}

func (s WorktreeSummary) Validate() error {
	if err := s.Bytes.Validate(); err != nil {
		return contractError(err)
	}
	if s.Entries == 0 && s.Bytes.Uint64() != 0 {
		return contractError(errors.New("empty worktree summary contains bytes"))
	}
	if s.Entries != 0 && s.Bytes.Uint64() == 0 {
		return contractError(errors.New("nonempty worktree summary contains no bytes"))
	}
	return nil
}

func processOutputMaximum() (core.ByteCount, error) {
	maximum, err := core.NewByteCount(math.MaxInt64)
	if err != nil {
		return core.ByteCount{}, contractError(err)
	}
	return maximum, nil
}
