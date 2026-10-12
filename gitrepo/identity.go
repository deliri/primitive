package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/process"
)

// ReferenceLabel is Git's unqualified branch or tag spelling. HEAD is also
// the native abbreviation returned for a detached head. This is display data,
// never a revision expression supplied back to Git.
type ReferenceLabel struct{ value string }

// ParseReferenceLabel follows the Git check-ref-format --allow-onelevel rules:
// https://git-scm.com/docs/git-check-ref-format
func ParseReferenceLabel(value string) (ReferenceLabel, error) {
	if value == "" || value == "@" || !utf8.ValidString(value) {
		return ReferenceLabel{}, core.ErrGitRepositoryOutput
	}
	if invalidReferenceEdges(value) || invalidReferenceSequences(value) {
		return ReferenceLabel{}, core.ErrGitRepositoryOutput
	}
	if err := errors.Join(validateReferenceCharacters(value), validateReferenceComponents(value)); err != nil {
		return ReferenceLabel{}, err
	}
	return ReferenceLabel{value: value}, nil
}

func invalidReferenceEdges(value string) bool {
	return strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".")
}
func invalidReferenceSequences(value string) bool {
	return strings.Contains(value, "//") || strings.Contains(value, "..") || strings.Contains(value, "@{")
}
func validateReferenceCharacters(value string) error {
	for _, character := range value {
		if character < 0x21 || character == 0x7f || strings.ContainsRune("~^:?*[\\", character) {
			return core.ErrGitRepositoryOutput
		}
	}
	return nil
}
func validateReferenceComponents(value string) error {
	for component := range strings.SplitSeq(value, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return core.ErrGitRepositoryOutput
		}
	}
	return nil
}
func (v ReferenceLabel) String() string  { return v.value }
func (v ReferenceLabel) Validate() error { _, err := ParseReferenceLabel(v.value); return err }

type RootRequest struct{ Directory core.AbsolutePath }

func (r RootRequest) Validate() error { return r.Directory.Validate() }

type RootObservation struct {
	Root    core.AbsolutePath
	Process process.ResultObservation
}

func (o RootObservation) Validate() error {
	return errors.Join(o.Root.Validate(), validateIdentityReceipt(o.Process, o.Root.String()))
}

type BranchRequest struct{ Root core.AbsolutePath }

func (r BranchRequest) Validate() error { return r.Root.Validate() }

type BranchObservation struct {
	Branch  ReferenceLabel
	Process process.ResultObservation
}

func (o BranchObservation) Validate() error {
	return errors.Join(o.Branch.Validate(), validateIdentityReceipt(o.Process, o.Branch.String()))
}

type ExactTagRequest struct{ Root core.AbsolutePath }

func (r ExactTagRequest) Validate() error { return r.Root.Validate() }

type ExactTagObservation struct {
	Tag     ReferenceLabel
	Process process.ResultObservation
}

func (o ExactTagObservation) Validate() error {
	return errors.Join(o.Tag.Validate(), validateIdentityReceipt(o.Process, o.Tag.String()))
}

func validateIdentityReceipt(receipt process.ResultObservation, text string) error {
	if err := receipt.Validate(); err != nil {
		return outputError(err)
	}
	if receipt.ExitCode != 0 || receipt.StderrBytes.Uint64() != 0 || receipt.StdoutBytes.Uint64() != uint64(len(text))+1 {
		return core.ErrGitRepositoryOutput
	}
	return nil
}

// CommandExitError retains the actual completed exit, including refusal to
// describe an untagged HEAD. Such an exit is not an authoritative absence fact.
type CommandExitError struct{ Process process.ResultObservation }

func (e CommandExitError) Error() string { return fmt.Sprintf("Git exited %d", e.Process.ExitCode) }
func (e CommandExitError) Unwrap() error { return core.ErrGitRepositoryExecution }
func (e CommandExitError) Validate() error {
	if err := e.Process.Validate(); err != nil {
		return contractError(err)
	}
	if e.Process.ExitCode == 0 {
		return core.ErrGitRepositoryContract
	}
	return nil
}

func (c Capability) ObserveRoot(ctx context.Context, request RootRequest) (RootObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return RootObservation{}, contractError(err)
	}
	var observed RootObservation
	err := c.withIdentityOutput(ctx, request.Directory, []string{"rev-parse", "--path-format=absolute", "--show-toplevel"}, func(owner context.Context, source *io.SectionReader, length int64, receipt process.ResultObservation) error {
		text, err := rootText(owner, source, length)
		if err != nil {
			return err
		}
		root, err := core.ParseAbsolutePath(text)
		if err != nil {
			return outputError(err)
		}
		observed = RootObservation{Root: root, Process: receipt}
		return observed.Validate()
	})
	if err != nil {
		return RootObservation{}, err
	}
	return observed, nil
}

func (c Capability) ObserveBranch(ctx context.Context, request BranchRequest) (BranchObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return BranchObservation{}, contractError(err)
	}
	var observed BranchObservation
	err := c.withIdentityOutput(ctx, request.Root, []string{"rev-parse", "--abbrev-ref", "HEAD"}, func(owner context.Context, source *io.SectionReader, length int64, receipt process.ResultObservation) error {
		text, err := identityText(owner, source, length)
		if err != nil {
			return err
		}
		branch, err := ParseReferenceLabel(text)
		if err != nil {
			return err
		}
		observed = BranchObservation{Branch: branch, Process: receipt}
		return observed.Validate()
	})
	if err != nil {
		return BranchObservation{}, err
	}
	return observed, nil
}

func (c Capability) ObserveExactTag(ctx context.Context, request ExactTagRequest) (ExactTagObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return ExactTagObservation{}, contractError(err)
	}
	var observed ExactTagObservation
	err := c.withIdentityOutput(ctx, request.Root, []string{"describe", "--tags", "--exact-match", "HEAD"}, func(owner context.Context, source *io.SectionReader, length int64, receipt process.ResultObservation) error {
		text, err := identityText(owner, source, length)
		if err != nil {
			return err
		}
		tag, err := ParseReferenceLabel(text)
		if err != nil {
			return err
		}
		observed = ExactTagObservation{Tag: tag, Process: receipt}
		return observed.Validate()
	})
	if err != nil {
		return ExactTagObservation{}, err
	}
	return observed, nil
}

// withIdentityOutput borrows a completed native file, checks physical framing
// before materializing the single result label, and gates publication on native
// cleanup. Scratch grows with stdout; retained result memory grows with the
// admitted label itself. No transcript-sized line collection or flow quota is
// used. Malformed multiline output is refused with fixed scanning memory.
func (c Capability) withIdentityOutput(ctx context.Context, directory core.AbsolutePath, values []string, consume func(context.Context, *io.SectionReader, int64, process.ResultObservation) error) error {
	parent, err := hostfacts.TemporaryDirectory()
	if err != nil {
		return executionError(err)
	}
	arguments, err := process.ParseArguments(append([]string{gitNoOptionalLocks, gitConfigOverride, gitNoFilesystemMonitor}, values...))
	if err != nil {
		return contractError(err)
	}
	return filestore.WithScratchScope(ctx, filestore.ScratchScopeRequest{Parent: parent, Use: func(owner context.Context, root *os.Root) error {
		path, err := core.ParseRelativePath("git-identity")
		if err != nil {
			return err
		}
		scope, err := filestore.WithScratchReplayScope(owner, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Use: func(owner context.Context, stdout filestore.ScratchReplayFile) error {
			result, err := process.Run(ctx, process.Request{Command: c.command, WorkingDirectory: directory, Arguments: arguments, Environment: c.environment, Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: io.Discard}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: c.configuration.WaitDelay, Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalTerminate}})
			if err != nil {
				return executionError(err)
			}
			receipt, err := result.Observation()
			if err != nil {
				return executionError(err)
			}
			if receipt.ExitCode != 0 {
				return CommandExitError{Process: receipt}
			}
			if receipt.StderrBytes.Uint64() != 0 {
				return core.ErrGitRepositoryOutput
			}
			extent, err := filestore.InspectOpenFile(owner, filestore.HandleInspectionRequest{File: stdout})
			if err != nil {
				return err
			}
			size, err := extent.SizeBytes()
			if err != nil {
				return err
			}
			length, err := size.Int64()
			if err != nil {
				return err
			}
			if uint64(length) != receipt.StdoutBytes.Uint64() {
				return core.ErrGitRepositoryOutput
			}
			if err := consume(owner, io.NewSectionReader(stdout, 0, length), length, receipt); err != nil {
				return err
			}
			return contextstate.Validate(ctx)
		}})
		return errors.Join(err, scope.Validate(), scope.OperationError(), scope.CleanupError())
	}})
}

func identityText(ctx context.Context, source lineio.RecordSource, length int64) (string, error) {
	buffer, err := core.NewByteCount(4096)
	if err != nil {
		return "", err
	}
	reader, err := lineio.NewRangeReader(ctx, lineio.RecordRangeRequest{Source: source, BufferBytes: buffer})
	if err != nil {
		return "", err
	}
	record, err := reader.ReadRecord(ctx)
	if err != nil {
		return "", outputError(err)
	}
	if record.Offset != 0 || record.Framing != lineio.RecordFramingLF || record.Bytes.Uint64() != uint64(length) || length < 2 {
		return "", core.ErrGitRepositoryOutput
	}
	var text strings.Builder
	if _, err := io.Copy(&text, io.NewSectionReader(source, 0, length-1)); err != nil {
		return "", outputError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return "", err
	}
	return text.String(), nil
}

// Repository paths may contain LF. Preserve every path byte before Git's final
// LF instead of imposing reference-label physical framing on filesystem data.
func rootText(ctx context.Context, source io.ReaderAt, length int64) (string, error) {
	if length < 2 {
		return "", core.ErrGitRepositoryOutput
	}
	var last [1]byte
	if _, err := source.ReadAt(last[:], length-1); err != nil {
		return "", outputError(err)
	}
	if last[0] != '\n' {
		return "", core.ErrGitRepositoryOutput
	}
	var text strings.Builder
	if _, err := io.Copy(&text, io.NewSectionReader(source, 0, length-1)); err != nil {
		return "", outputError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return "", err
	}
	return text.String(), nil
}
