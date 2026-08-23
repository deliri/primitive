package hostfacts

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const directoryBatchEntries = 64

// MissingPathPolicy controls the exact missing-root behavior.
type MissingPathPolicy uint8

const (
	MissingPathUnknown MissingPathPolicy = iota
	MissingPathReject
	MissingPathIsEmpty
	missingPathLimit
)

func missingPathPolicyLabels() [missingPathLimit]string {
	return [...]string{
		MissingPathReject:  "missing-path-reject",
		MissingPathIsEmpty: "is-empty",
	}
}

// Validate rejects policies outside the closed domain.
func (p MissingPathPolicy) Validate() error {
	if !p.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("missing path policy is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed missing-path policy domain.
func (p MissingPathPolicy) IsValid() bool {
	return p > MissingPathUnknown && p < missingPathLimit && missingPathPolicyLabels()[p] != ""
}

// OffWireEnum declares MissingPathPolicy as traversal execution policy rather
// than a wire encoding.
func (MissingPathPolicy) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for p.
func (p MissingPathPolicy) String() string {
	if !p.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return missingPathPolicyLabels()[p]
}

// TreeUsageRequest binds one root to one missing-path policy.
type TreeUsageRequest struct {
	Root          core.AbsolutePath
	MissingPolicy MissingPathPolicy
}

// Validate rejects an unset root or policy.
func (r TreeUsageRequest) Validate() error {
	if err := r.Root.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	return r.MissingPolicy.Validate()
}

// RegularFileCount is a Hostfacts-owned count. Its zero value is valid.
type RegularFileCount struct {
	value uint64
}

func newRegularFileCount(value uint64) RegularFileCount {
	return RegularFileCount{value: value}
}

// Validate accepts the complete unsigned domain.
func (RegularFileCount) Validate() error {
	return nil
}

// Uint64 returns the count.
func (c RegularFileCount) Uint64() uint64 {
	return c.value
}

// TreeUsage is the logical extent and entry count of regular files.
type TreeUsage struct {
	bytes core.ByteLength
	files RegularFileCount
	valid bool
}

// Validate distinguishes an observed empty tree from the invalid zero value.
func (u TreeUsage) Validate() error {
	if !u.valid {
		return errors.Join(core.ErrHostFactsContract, errors.New("tree usage is unset"))
	}
	return errors.Join(u.bytes.Validate(), u.files.Validate())
}

// RegularFileBytes returns logical regular-file bytes.
func (u TreeUsage) RegularFileBytes() core.ByteLength {
	return u.bytes
}

// RegularFileCount returns the typed regular-file count.
func (u TreeUsage) RegularFileCount() RegularFileCount {
	return u.files
}

type treeEntryKind uint8

const (
	treeEntryUnknown treeEntryKind = iota
	treeEntryIgnored
	treeEntryRegular
	treeEntryDirectory
)

// treeEntryDecision is what the walk does with one inspected entry. It is a
// closed domain so that adding an entry kind without deciding its measurement is
// a refusal rather than a silent undercount.
type treeEntryDecision uint8

const (
	treeEntryDecisionRefuse treeEntryDecision = iota
	treeEntryDecisionCount
	treeEntryDecisionSkip
	treeEntryDecisionDescend
)

func (k treeEntryKind) decision() treeEntryDecision {
	if k >= treeEntryKind(len(treeEntryDecisions())) {
		return treeEntryDecisionRefuse
	}
	return treeEntryDecisions()[k]
}

func treeEntryDecisions() [treeEntryDirectory + 1]treeEntryDecision {
	return [...]treeEntryDecision{
		treeEntryRegular:   treeEntryDecisionCount,
		treeEntryIgnored:   treeEntryDecisionSkip,
		treeEntryDirectory: treeEntryDecisionDescend,
	}
}

type treeEntry struct {
	directory *os.File
	size      int64
	kind      treeEntryKind
}

type treeFrame struct {
	directory *os.File
	relative  string
	entries   []fs.DirEntry
	next      int
}

type treeAccumulator struct {
	bytes uint64
	files uint64
}

func (a *treeAccumulator) addRegular(size int64) error {
	if size < 0 || uint64(size) > math.MaxInt64-a.bytes || a.files == math.MaxUint64 {
		return core.ErrNumericOverflow
	}
	a.bytes += uint64(size)
	a.files++
	return nil
}

func (a treeAccumulator) close() (TreeUsage, error) {
	bytes, err := core.NewByteLength(a.bytes)
	if err != nil {
		return TreeUsage{}, err
	}
	return TreeUsage{
		bytes: bytes,
		files: newRegularFileCount(a.files),
		valid: true,
	}, nil
}

type treeWalk struct {
	root        *platformRoot
	stack       []treeFrame
	accumulator treeAccumulator
}

func walkTree(ctx context.Context, root *platformRoot) (TreeUsage, error) {
	directory, err := root.openDirectory(".")
	if err != nil {
		return TreeUsage{}, err
	}
	walk := treeWalk{root: root, stack: []treeFrame{{directory: directory, relative: "."}}}
	for len(walk.stack) > 0 {
		if err := walk.step(ctx); err != nil {
			return TreeUsage{}, closeTreeStack(walk.stack, err)
		}
	}
	usage, err := walk.accumulator.close()
	if err != nil {
		return TreeUsage{}, err
	}
	return usage, usage.Validate()
}

func (w *treeWalk) step(ctx context.Context) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	index := len(w.stack) - 1
	if w.stack[index].next < len(w.stack[index].entries) {
		return w.inspectNext(index)
	}
	entries, err := w.stack[index].directory.ReadDir(directoryBatchEntries)
	if len(entries) != 0 {
		w.stack[index].entries = entries
		w.stack[index].next = 0
		return nil
	}
	next, ascendErr := ascendTree(w.stack, err)
	w.stack = next
	return ascendErr
}

func (w *treeWalk) inspectNext(index int) error {
	entry := w.stack[index].entries[w.stack[index].next]
	w.stack[index].next++
	observed, err := w.root.inspectEntry(w.stack[index].directory, w.stack[index].relative, entry.Name())
	if err != nil {
		return err
	}
	switch observed.kind.decision() {
	case treeEntryDecisionCount:
		return w.accumulator.addRegular(observed.size)
	case treeEntryDecisionSkip:
		return nil
	case treeEntryDecisionDescend:
		return w.descend(index, observed, entry.Name())
	default:
		// An unclassified entry is a leaf that returned no error and no decision.
		// Treating it as ignorable would silently undercount the tree, so the
		// walk refuses rather than reporting an extent it did not measure.
		return core.ErrHostFactsObservation
	}
}

func (w *treeWalk) descend(index int, observed treeEntry, name string) error {
	if observed.directory == nil {
		return core.ErrHostFactsObservation
	}
	if len(w.stack) >= core.FilesystemPathMaximumComponents {
		// The descriptor is closed here rather than left to the runtime
		// finalizer: every directory this walk opens has one deterministic
		// owner, including the one it refuses to descend into.
		return errors.Join(core.ErrHostFactsObservation, observed.directory.Close())
	}
	w.stack = append(w.stack, treeFrame{
		directory: observed.directory,
		relative:  filepath.Join(w.stack[index].relative, name),
	})
	return nil
}

func ascendTree(stack []treeFrame, readErr error) ([]treeFrame, error) {
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return stack, readErr
	}
	index := len(stack) - 1
	return stack[:index], stack[index].directory.Close()
}

func closeTreeStack(stack []treeFrame, cause error) error {
	for _, s := range slices.Backward(stack) {
		cause = errors.Join(cause, s.directory.Close())
	}
	return cause
}
