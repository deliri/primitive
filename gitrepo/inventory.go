package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// Git command contracts:
// https://git-scm.com/docs/git-for-each-ref
// https://git-scm.com/docs/git-worktree#_porcelain_format
const (
	gitForEachRef      = "for-each-ref"
	gitReferenceFormat = "--format=%(objectname) %(refname)"
	gitLocalBranches   = "refs/heads"
	gitRemoteBranches  = "refs/remotes"
	gitWorktreeCommand = "worktree"
	gitWorktreeList    = "list"
	gitPorcelain       = "--porcelain"
	// inventoryRecordMaximum bounds one native record; a ref name or worktree
	// path longer than this is refused rather than buffered without limit.
	inventoryRecordMaximum = 64 << 10
)

// InventoryRequest identifies one repository whose refs or worktrees are read.
type InventoryRequest struct{ Root core.AbsolutePath }

func (r InventoryRequest) Validate() error { return r.Root.Validate() }

// ReferenceEntry is one local or remote-tracking branch and the commit it names.
type ReferenceEntry struct {
	Name   ReferenceLabel
	Commit core.BuildCommit
}

func (e ReferenceEntry) Validate() error { return errors.Join(e.Name.Validate(), e.Commit.Validate()) }

// ReferenceConsumer receives each branch reference in Git's stream order.
type ReferenceConsumer interface {
	ConsumeReference(ReferenceEntry) error
}

// WorktreeListEntry is one linked or main worktree from Git's porcelain list.
// A bare repository has no HEAD; a detached worktree has no branch.
type WorktreeListEntry struct {
	Path     core.AbsolutePath
	Head     *core.BuildCommit
	Branch   *ReferenceLabel
	Bare     bool
	Detached bool
	Locked   bool
	Prunable bool
}

func (e WorktreeListEntry) Validate() error {
	if err := e.Path.Validate(); err != nil {
		return contractError(err)
	}
	if e.Head != nil {
		if err := e.Head.Validate(); err != nil {
			return contractError(err)
		}
	}
	if e.Branch != nil {
		if err := e.Branch.Validate(); err != nil {
			return contractError(err)
		}
	}
	if e.Bare && (e.Head != nil || e.Branch != nil || e.Detached) || e.Detached && e.Branch != nil {
		return core.ErrGitRepositoryOutput
	}
	return nil
}

// WorktreeListConsumer receives each worktree in Git's stream order.
type WorktreeListConsumer interface {
	ConsumeWorktreeListEntry(WorktreeListEntry) error
}

// StreamReferences emits the repository's local and remote-tracking branches
// with their commits, one bounded record at a time; it returns the count.
func (c Capability) StreamReferences(ctx context.Context, request InventoryRequest, consumer ReferenceConsumer) (uint64, error) {
	if ctx == nil || consumer == nil {
		return 0, contractError(errors.New("reference stream dependency is nil"))
	}
	writer := recordWriter{separator: '\n', record: func(line []byte) error {
		commit, name, found := strings.Cut(string(line), " ")
		if !found {
			return outputError(errors.New("git emitted a malformed reference record"))
		}
		parsedCommit, err := core.ParseBuildCommit(commit)
		if err != nil {
			return outputError(err)
		}
		label, err := ParseReferenceLabel(name)
		if err != nil {
			return outputError(err)
		}
		entry := ReferenceEntry{Name: label, Commit: parsedCommit}
		if err := entry.Validate(); err != nil {
			return outputError(err)
		}
		return consumer.ConsumeReference(entry)
	}}
	err := c.runInventory(ctx, request, []string{gitNoOptionalLocks, gitConfigOverride, gitNoFilesystemMonitor, gitForEachRef, gitReferenceFormat, gitLocalBranches, gitRemoteBranches}, &writer)
	if err != nil {
		return 0, err
	}
	return writer.records, writer.finish()
}

// StreamWorktreeList emits each of the repository's worktrees, as Git's
// NUL-terminated porcelain list describes them; it returns the count.
func (c Capability) StreamWorktreeList(ctx context.Context, request InventoryRequest, consumer WorktreeListConsumer) (uint64, error) {
	if ctx == nil || consumer == nil {
		return 0, contractError(errors.New("worktree list stream dependency is nil"))
	}
	var current *WorktreeListEntry
	emitted := uint64(0)
	emit := func() error {
		if current == nil {
			return nil
		}
		entry := *current
		current = nil
		if err := entry.Validate(); err != nil {
			return outputError(err)
		}
		emitted++
		return consumer.ConsumeWorktreeListEntry(entry)
	}
	writer := recordWriter{separator: 0, record: func(field []byte) error {
		if len(field) == 0 {
			return emit()
		}
		key, value, _ := strings.Cut(string(field), " ")
		if key == "worktree" {
			if err := emit(); err != nil {
				return err
			}
			path, err := core.ParseAbsolutePath(value)
			if err != nil {
				return outputError(err)
			}
			current = &WorktreeListEntry{Path: path}
			return nil
		}
		if current == nil {
			return outputError(errors.New("git emitted a worktree attribute before its path"))
		}
		return applyWorktreeAttribute(current, key, value)
	}}
	err := c.runInventory(ctx, request, []string{gitNoOptionalLocks, gitConfigOverride, gitNoFilesystemMonitor, gitWorktreeCommand, gitWorktreeList, gitPorcelain, gitNullTerminate}, &writer)
	if err != nil {
		return 0, err
	}
	if err := writer.finish(); err != nil {
		return 0, err
	}
	if err := emit(); err != nil {
		return 0, err
	}
	return emitted, nil
}

func applyWorktreeAttribute(entry *WorktreeListEntry, key, value string) error {
	switch key {
	case "HEAD":
		commit, err := core.ParseBuildCommit(value)
		if err != nil {
			return outputError(err)
		}
		entry.Head = &commit
	case "branch":
		label, err := ParseReferenceLabel(value)
		if err != nil {
			return outputError(err)
		}
		entry.Branch = &label
	case "bare":
		entry.Bare = true
	case "detached":
		entry.Detached = true
	case "locked":
		entry.Locked = true
	case "prunable":
		entry.Prunable = true
	default:
		return outputError(errors.New("git emitted an unknown worktree attribute"))
	}
	return nil
}

func (c Capability) runInventory(ctx context.Context, request InventoryRequest, values []string, writer *recordWriter) error {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return contractError(err)
	}
	maximum, err := processOutputMaximum()
	if err != nil {
		return err
	}
	arguments, err := process.ParseArguments(values)
	if err != nil {
		return contractError(err)
	}
	result, runErr := process.Run(ctx, process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: writer, Stderr: io.Discard},
		Command: c.command, WorkingDirectory: request.Root, Arguments: arguments, Environment: c.environment,
		OutputPolicy: process.OutputPolicy{Mode: process.OutputModeBounded, Maximum: maximum}, WaitDelay: c.configuration.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalTerminate},
	})
	if writer.failure != nil {
		return writer.failure
	}
	if runErr != nil {
		return executionError(runErr)
	}
	return requireSuccess(result)
}

// recordWriter splits a native stream on one separator byte, handing each
// bounded record to record without retaining earlier ones.
type recordWriter struct {
	record    func([]byte) error
	failure   error
	pending   []byte
	records   uint64
	separator byte
}

func (w *recordWriter) Write(data []byte) (int, error) {
	if w == nil || w.record == nil {
		return 0, contractError(errors.New("record writer is invalid"))
	}
	if w.failure != nil {
		return 0, w.failure
	}
	for index, value := range data {
		if value != w.separator {
			if len(w.pending) >= inventoryRecordMaximum {
				w.failure = outputError(errors.New("git record exceeds its bound"))
				return index, w.failure
			}
			w.pending = append(w.pending, value)
			continue
		}
		if err := w.record(w.pending); err != nil {
			w.failure = err
			return index + 1, err
		}
		w.records++
		w.pending = w.pending[:0]
	}
	return len(data), nil
}

func (w *recordWriter) finish() error {
	if w.failure != nil {
		return w.failure
	}
	if len(w.pending) != 0 {
		return outputError(errors.New("git record stream is truncated"))
	}
	return nil
}
