package filestore

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deliri/primitive/v2026/core"
)

const (
	targetEqualsTemporaryDiagnostic = "filestore target equals its temporary path"
	crossDirectoryDiagnostic        = "filestore target and temporary must share a directory"
)

// Location names one path through one real OS root capability.
type Location struct {
	Root *os.Root
	Path core.RelativePath
}

// Validate rejects an unset root or path.
func (l Location) Validate() error {
	if l.Root == nil {
		return contractError(errors.New("filestore root is missing"))
	}
	if err := l.Path.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// InstallMode declares whether activation must create or may replace a target.
type InstallMode uint8

const (
	// InstallUnknown is the invalid zero state.
	InstallUnknown InstallMode = iota
	// InstallCreate requires the target to be absent.
	InstallCreate
	// InstallReplace atomically replaces an existing target and also admits an
	// absent target.
	InstallReplace
	installModeLimit
)

// Validate rejects values outside the closed install domain.
func (m InstallMode) Validate() error {
	if m <= InstallUnknown || m >= installModeLimit {
		return contractError(errors.New("filestore install mode is invalid"))
	}
	return nil
}

// DirectoryRequest prepares one rooted directory chain with an exact final
// permission mode.
type DirectoryRequest struct {
	Location Location
	Mode     fs.FileMode
}

// Validate rejects an invalid location or permission mode.
func (r DirectoryRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	return validatePermissionMode(r.Mode)
}

// ReadRequest streams one bounded regular file into Destination.
type ReadRequest struct {
	Destination  io.Writer
	Location     Location
	MaximumBytes core.ByteCount
}

// Validate rejects every unset read boundary.
func (r ReadRequest) Validate() error {
	if r.Destination == nil {
		return contractError(errors.New("filestore read destination is missing"))
	}
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := r.MaximumBytes.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// WriteRequest streams Source into one caller-named same-directory temporary
// before atomic activation.
type WriteRequest struct {
	Source       io.Reader
	Location     Location
	Temporary    core.RelativePath
	Mode         fs.FileMode
	Install      InstallMode
	MaximumBytes core.ByteCount
}

// Validate rejects every unset write boundary.
func (r WriteRequest) Validate() error {
	if r.Source == nil {
		return contractError(errors.New("filestore write source is missing"))
	}
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	if err := validateTemporary(r.Location.Path, r.Temporary); err != nil {
		return err
	}
	if err := validatePermissionMode(r.Mode); err != nil {
		return err
	}
	if err := r.Install.Validate(); err != nil {
		return err
	}
	if err := r.MaximumBytes.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// StageRequest streams Source into one exact caller-owned temporary name.
type StageRequest struct {
	Source       io.Reader
	Temporary    Location
	Mode         fs.FileMode
	MaximumBytes core.ByteCount
}

// Validate rejects every unset staging boundary.
func (r StageRequest) Validate() error {
	if r.Source == nil {
		return contractError(errors.New("filestore stage source is missing"))
	}
	if err := r.Temporary.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Temporary.Path); err != nil {
		return err
	}
	if err := validatePermissionMode(r.Mode); err != nil {
		return err
	}
	if err := r.MaximumBytes.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// CommitRequest names the target selected for one completed stage. Target and
// stage may occupy different directories within the same rooted capability.
type CommitRequest struct {
	Target  core.RelativePath
	Staged  StagedFile
	Install InstallMode
}

// Validate rejects an invalid stage, target, or activation mode.
func (r CommitRequest) Validate() error {
	if err := r.Staged.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Target); err != nil {
		return err
	}
	if r.Target == r.Staged.path {
		return contractError(errors.New(targetEqualsTemporaryDiagnostic))
	}
	return r.Install.Validate()
}

// AppendMode declares which namespace state an append open admits.
type AppendMode uint8

const (
	// AppendUnknown is the invalid zero state.
	AppendUnknown AppendMode = iota
	// AppendCreate requires the target to be absent and creates it exclusively.
	AppendCreate
	// AppendExisting requires the target to be an existing regular file.
	AppendExisting
	// AppendCreateOrOpen creates an absent target or opens an existing regular
	// file without truncating it.
	AppendCreateOrOpen
	appendModeLimit
)

const appendModeInvalidReason = "filestore append mode is invalid"

// Validate rejects values outside the closed append domain.
func (m AppendMode) Validate() error {
	if m <= AppendUnknown || m >= appendModeLimit {
		return contractError(errors.New(appendModeInvalidReason))
	}
	return nil
}

// AppendRequest opens one real OS file for append below a rooted boundary.
type AppendRequest struct {
	Location Location
	Mode     fs.FileMode
	Append   AppendMode
}

// Validate rejects an invalid location, permission mode, or append intent.
func (r AppendRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	if err := validatePermissionMode(r.Mode); err != nil {
		return err
	}
	return r.Append.Validate()
}

// RotationRequest transfers Outgoing to the append-rotation operation and
// names the exclusively created incoming generation.
type RotationRequest struct {
	Outgoing *os.File
	Incoming AppendRequest
}

// Validate rejects a missing outgoing handle or an incoming request that does
// not exclusively create the next generation.
func (r RotationRequest) Validate() error {
	if r.Outgoing == nil {
		return contractError(errors.New("filestore rotation outgoing handle is missing"))
	}
	if err := r.Incoming.Validate(); err != nil {
		return err
	}
	if r.Incoming.Append != AppendCreate {
		return contractError(errors.New("filestore rotation incoming append mode must create"))
	}
	return nil
}

// RemovalRequest names one rooted entry to remove durably.
type RemovalRequest struct {
	Location Location
}

// Validate rejects an invalid or root-naming location.
func (r RemovalRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	return validateMutablePath(r.Location.Path)
}

// TreeRemovalRequest names one rooted tree to remove durably. Symlinks are
// removed as entries and are never traversed outside the rooted namespace.
type TreeRemovalRequest struct {
	Location Location
}

// Validate rejects an invalid or root-naming tree location.
func (r TreeRemovalRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	return validateMutablePath(r.Location.Path)
}

// WalkDirective controls descent after one streamed directory entry.
type WalkDirective uint8

const (
	// WalkContinue visits a directory's descendants.
	WalkContinue WalkDirective = iota + 1
	// WalkSkipDirectory omits a directory's descendants.
	WalkSkipDirectory
	walkDirectiveLimit
)

// Validate closes the walk-directive domain.
func (d WalkDirective) Validate() error {
	if d < WalkContinue || d >= walkDirectiveLimit {
		return contractError(errors.New("filestore walk directive is invalid"))
	}
	return nil
}

// WalkOrder selects the bounded directory-entry observation strategy.
type WalkOrder uint8

const (
	// WalkOrderNative streams fixed batches in operating-system order.
	WalkOrderNative WalkOrder = iota + 1
	// WalkOrderLexical sorts each directory under an explicit entry ceiling.
	WalkOrderLexical
	walkOrderLimit
)

// Validate closes the walk-order domain.
func (o WalkOrder) Validate() error {
	if o < WalkOrderNative || o >= walkOrderLimit {
		return contractError(errors.New("filestore walk order is invalid"))
	}
	return nil
}

// DirectoryEntryMaximum is one positive fixed allocation ceiling for a
// lexically ordered directory.
type DirectoryEntryMaximum struct {
	value uint32
}

// NewDirectoryEntryMaximum constructs one lexical directory ceiling.
func NewDirectoryEntryMaximum(value uint32) (DirectoryEntryMaximum, error) {
	maximum := DirectoryEntryMaximum{value: value}
	if err := maximum.Validate(); err != nil {
		return DirectoryEntryMaximum{}, err
	}
	return maximum, nil
}

// Validate rejects a zero lexical directory ceiling.
func (m DirectoryEntryMaximum) Validate() error {
	if m.value == 0 {
		return contractError(errors.New("filestore directory entry maximum is zero"))
	}
	return nil
}

// WalkEntry is one descendant observed from a rooted streaming traversal.
type WalkEntry struct {
	Entry fs.DirEntry
	Path  core.RelativePath
}

// Validate rejects an unset path or directory entry.
func (e WalkEntry) Validate() error {
	if err := e.Path.Validate(); err != nil {
		return contractError(err)
	}
	if e.Entry == nil {
		return contractError(errors.New("filestore walk entry is missing"))
	}
	return nil
}

// WalkRequest streams every descendant of one rooted directory to Visit.
type WalkRequest struct {
	Visit                 func(WalkEntry) (WalkDirective, error)
	Location              Location
	DirectoryEntryMaximum DirectoryEntryMaximum
	Order                 WalkOrder
}

// Validate rejects an unset root, path, or visitor.
func (r WalkRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := r.Order.Validate(); err != nil {
		return err
	}
	if r.Order == WalkOrderLexical {
		if err := r.DirectoryEntryMaximum.Validate(); err != nil {
			return err
		}
	} else if r.DirectoryEntryMaximum != (DirectoryEntryMaximum{}) {
		return contractError(errors.New("filestore native walk carries a lexical entry maximum"))
	}
	if r.Visit == nil {
		return contractError(errors.New("filestore walk visitor is missing"))
	}
	return nil
}

func validatePermissionMode(mode fs.FileMode) error {
	if mode == 0 || mode != mode.Perm() {
		return contractError(errors.New("filestore permission mode is invalid"))
	}
	return nil
}

func validateMutablePath(path core.RelativePath) error {
	if err := path.Validate(); err != nil {
		return contractError(err)
	}
	if path.String() == "." {
		return contractError(errors.New("filestore mutation cannot name the root entry"))
	}
	return nil
}

func validateTemporary(target, temporary core.RelativePath) error {
	if err := validateMutablePath(temporary); err != nil {
		return err
	}
	if target == temporary {
		return contractError(errors.New(targetEqualsTemporaryDiagnostic))
	}
	if filepath.Dir(target.String()) != filepath.Dir(temporary.String()) {
		return contractError(errors.New(crossDirectoryDiagnostic))
	}
	return nil
}

var (
	_ core.Validatable = Location{}
	_ core.Validatable = InstallMode(0)
	_ core.Validatable = DirectoryRequest{}
	_ core.Validatable = ReadRequest{}
	_ core.Validatable = WriteRequest{}
	_ core.Validatable = StageRequest{}
	_ core.Validatable = CommitRequest{}
	_ core.Validatable = AppendMode(0)
	_ core.Validatable = AppendRequest{}
	_ core.Validatable = RotationRequest{}
	_ core.Validatable = RemovalRequest{}
	_ core.Validatable = TreeRemovalRequest{}
	_ core.Validatable = WalkDirective(0)
	_ core.Validatable = WalkOrder(0)
	_ core.Validatable = DirectoryEntryMaximum{}
	_ core.Validatable = WalkEntry{}
	_ core.Validatable = WalkRequest{}
)
