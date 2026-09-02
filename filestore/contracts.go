package filestore

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	targetEqualsTemporaryDiagnostic = "filestore target equals its temporary path"
	crossDirectoryDiagnostic        = "filestore target and temporary must share a directory"
	createModeDiagnostic            = "create"
	// SymbolicLinkTargetMaximumBytes bounds one observed link target without
	// assuming it is a path; procfs also exposes handles such as socket:[inode].
	SymbolicLinkTargetMaximumBytes = 64 << 10
)

// Location names one path through one real OS root capability.
type Location struct {
	Root *os.Root
	Path core.RelativePath
}

// SymbolicLinkTarget is one bounded target observed without following its
// symbolic link. Its bytes are not interpreted as a filesystem path.
type SymbolicLinkTarget struct{ value string }

// Validate rejects an absent, unbounded, or NUL-carrying link target.
func (t SymbolicLinkTarget) Validate() error {
	if len(t.value) == 0 || len(t.value) > SymbolicLinkTargetMaximumBytes || strings.IndexByte(t.value, 0) >= 0 {
		return contractError(errors.New("filestore symbolic-link target is invalid"))
	}
	return nil
}

// String returns the exact observed target, or empty text for an invalid value.
func (t SymbolicLinkTarget) String() string {
	if t.Validate() != nil {
		return ""
	}
	return t.value
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

func installModeDiagnostics() [installModeLimit]string {
	return [...]string{
		InstallCreate:  createModeDiagnostic,
		InstallReplace: "replace",
	}
}

// Validate rejects values outside the closed install domain.
func (m InstallMode) Validate() error {
	if !m.IsValid() {
		return contractError(errors.New("filestore install mode is invalid"))
	}
	return nil
}

// IsValid reports membership in the closed install-mode domain.
func (m InstallMode) IsValid() bool {
	return m > InstallUnknown && m < installModeLimit && installModeDiagnostics()[m] != ""
}

// OffWireEnum declares InstallMode as filesystem execution policy rather than
// a wire encoding.
func (InstallMode) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for m.
func (m InstallMode) String() string {
	if !m.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return installModeDiagnostics()[m]
}

// DirectoryRequest prepares one rooted directory chain with an exact final
// permission mode.
type DirectoryRequest struct {
	Location Location
	Mode     fs.FileMode
}

// PermissionRequest changes one existing rooted entry to an exact permission
// mode and proves the metadata update durable.
type PermissionRequest struct {
	Location Location
	Mode     fs.FileMode
}

func (r PermissionRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	return validatePermissionMode(r.Mode)
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

// ReadHandleRequest names one existing regular file to open for reading.
type ReadHandleRequest struct {
	Location Location
}

// Validate rejects an invalid location.
func (r ReadHandleRequest) Validate() error {
	return r.Location.Validate()
}

// UpdateHandleRequest names one existing regular file to open for bounded
// caller-owned read/update work. The caller chooses the byte transformation;
// Filestore owns only the rooted namespace acquisition and regular-file gate.
type UpdateHandleRequest struct {
	Location Location
}

// Validate rejects an invalid or non-mutable location.
func (r UpdateHandleRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	return validateMutablePath(r.Location.Path)
}

// TouchRequest stamps one existing regular file with a custody instant.
type TouchRequest struct {
	Location   Location
	ModifiedAt temporal.Instant
}

// Validate rejects an invalid location or an unset custody instant. The Go
// zero instant is refused rather than read as "now", because a custody record
// nobody chose the time for is not a custody record. The Unix epoch is a
// chosen instant and is accepted: temporal.Instant already separates "unset"
// from "zero", so this gate delegates instead of restating the rule.
func (r TouchRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	if err := r.ModifiedAt.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// DurabilityRequest names one already-written regular file whose name must be
// proven durable.
type DurabilityRequest struct {
	Location Location
}

// Validate rejects an invalid location or one naming the rooted entry itself,
// which has no parent inside the capability to prove anything standard.
func (r DurabilityRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	return validateMutablePath(r.Location.Path)
}

// LockFileRequest names one rooted file to open or create as a lock carrier.
type LockFileRequest struct {
	Location Location
	Mode     fs.FileMode
}

// Validate rejects an invalid location or permission mode.
func (r LockFileRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	return validatePermissionMode(r.Mode)
}

// RenameRequest moves the entry at Location to Target under the same rooted
// capability. Target and source may occupy different directories within that
// capability.
type RenameRequest struct {
	Location Location
	Target   core.RelativePath
}

// Validate rejects an invalid location, a root-naming source or target, or a
// rename onto itself.
func (r RenameRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Location.Path); err != nil {
		return err
	}
	if err := validateMutablePath(r.Target); err != nil {
		return err
	}
	if r.Location.Path == r.Target {
		return contractError(errors.New("filestore rename target equals its source"))
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

// StageDestinationRequest exclusively creates one temporary that an external
// standard-library writer will fill to an exact final byte length.
type StageDestinationRequest struct {
	Temporary     Location
	ExpectedBytes core.ByteLength
	Mode          fs.FileMode
}

// ActivationRequest is one completely prevalidated external-stage and atomic
// activation plan. It lets a cross-package streaming producer validate every
// local filesystem effect before the temporary is created.
type ActivationRequest struct {
	Temporary     Location
	Target        core.RelativePath
	ExpectedBytes core.ByteLength
	Mode          fs.FileMode
	Install       InstallMode
}

// Validate closes the temporary, target, exact extent, permissions, and
// activation intent at the pre-effect boundary owned by Filestore.
func (r ActivationRequest) Validate() error {
	stage := r.StageDestination()
	if err := stage.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Target); err != nil {
		return err
	}
	if r.Target == r.Temporary.Path {
		return contractError(errors.New(targetEqualsTemporaryDiagnostic))
	}
	return r.Install.Validate()
}

// StageDestination projects the exact external-stage ingress.
func (r ActivationRequest) StageDestination() StageDestinationRequest {
	return StageDestinationRequest{
		Temporary: r.Temporary, ExpectedBytes: r.ExpectedBytes, Mode: r.Mode,
	}
}

// CommitRequest binds one completed stage to this prevalidated activation.
func (r ActivationRequest) CommitRequest(staged StagedFile) (CommitRequest, error) {
	if err := r.Validate(); err != nil {
		return CommitRequest{}, err
	}
	request := CommitRequest{Target: r.Target, Staged: staged, Install: r.Install}
	if err := request.Validate(); err != nil {
		return CommitRequest{}, err
	}
	if staged.Path() != r.Temporary.Path || staged.BytesWritten() != r.ExpectedBytes {
		return CommitRequest{}, contractError(errors.New("filestore completed stage differs from activation plan"))
	}
	return request, nil
}

// Validate rejects every unset external-stage boundary.
func (r StageDestinationRequest) Validate() error {
	if err := r.Temporary.Validate(); err != nil {
		return err
	}
	if err := validateMutablePath(r.Temporary.Path); err != nil {
		return err
	}
	if err := validatePermissionMode(r.Mode); err != nil {
		return err
	}
	if err := r.ExpectedBytes.Validate(); err != nil {
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

func appendModeDiagnostics() [appendModeLimit]string {
	return [...]string{
		AppendCreate:       createModeDiagnostic,
		AppendExisting:     "existing",
		AppendCreateOrOpen: "create-or-open",
	}
}

const appendModeInvalidReason = "filestore append mode is invalid"

// Validate rejects values outside the closed append domain.
func (m AppendMode) Validate() error {
	if !m.IsValid() {
		return contractError(errors.New(appendModeInvalidReason))
	}
	return nil
}

// IsValid reports membership in the closed append-mode domain.
func (m AppendMode) IsValid() bool {
	return m > AppendUnknown && m < appendModeLimit && appendModeDiagnostics()[m] != ""
}

// OffWireEnum declares AppendMode as filesystem execution policy rather than
// a wire encoding.
func (AppendMode) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for m.
func (m AppendMode) String() string {
	if !m.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return appendModeDiagnostics()[m]
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
	// WalkDirectiveUnknown is the invalid zero directive.
	WalkDirectiveUnknown WalkDirective = iota
	// WalkContinue visits a directory's descendants.
	WalkContinue
	// WalkSkipDirectory omits a directory's descendants.
	WalkSkipDirectory
	walkDirectiveLimit
)

func walkDirectiveDiagnostics() [walkDirectiveLimit]string {
	return [walkDirectiveLimit]string{
		WalkContinue:      "continue",
		WalkSkipDirectory: "skip-directory",
	}
}

// Validate closes the walk-directive domain.
func (d WalkDirective) Validate() error {
	if !d.IsValid() {
		return contractError(errors.New("filestore walk directive is invalid"))
	}
	return nil
}

// IsValid reports membership in the closed walk-directive domain.
func (d WalkDirective) IsValid() bool {
	return d > WalkDirectiveUnknown && d < walkDirectiveLimit &&
		walkDirectiveDiagnostics()[d] != ""
}

// OffWireEnum declares WalkDirective as traversal control rather than a wire
// encoding.
func (WalkDirective) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for d.
func (d WalkDirective) String() string {
	if !d.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return walkDirectiveDiagnostics()[d]
}

// WalkOrder selects the bounded directory-entry observation strategy.
type WalkOrder uint8

const (
	// WalkOrderUnknown is the invalid zero order.
	WalkOrderUnknown WalkOrder = iota
	// WalkOrderNative streams fixed batches in operating-system order.
	WalkOrderNative
	// WalkOrderLexical sorts each directory under an explicit entry ceiling.
	WalkOrderLexical
	walkOrderLimit
)

func walkOrderDiagnostics() [walkOrderLimit]string {
	return [walkOrderLimit]string{
		WalkOrderNative:  "native",
		WalkOrderLexical: "lexical",
	}
}

// Validate closes the walk-order domain.
func (o WalkOrder) Validate() error {
	if !o.IsValid() {
		return contractError(errors.New("filestore walk order is invalid"))
	}
	return nil
}

// IsValid reports membership in the closed walk-order domain.
func (o WalkOrder) IsValid() bool {
	return o > WalkOrderUnknown && o < walkOrderLimit &&
		walkOrderDiagnostics()[o] != ""
}

// OffWireEnum declares WalkOrder as traversal execution policy rather than a
// wire encoding.
func (WalkOrder) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for o.
func (o WalkOrder) String() string {
	if !o.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return walkOrderDiagnostics()[o]
}

// DirectoryEntryMaximum is one positive fixed allocation ceiling for a
// lexically ordered directory.
type DirectoryEntryMaximum struct {
	value uint32
}

// DirectoryEntryMaximumLimit is the largest directory that lexical walking
// will retain and sort. It bounds both allocation and the uint32-to-int
// conversion on every supported Go architecture.
const DirectoryEntryMaximumLimit uint32 = 1 << 16

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
	if m.value == 0 || m.value > DirectoryEntryMaximumLimit {
		return contractError(errors.New("filestore directory entry maximum is outside the admitted interval"))
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
	_ core.Validatable = SymbolicLinkTarget{}
	_ core.Validatable = InstallMode(0)
	_ core.Validatable = DirectoryRequest{}
	_ core.Validatable = ReadRequest{}
	_ core.Validatable = ReadHandleRequest{}
	_ core.Validatable = UpdateHandleRequest{}
	_ core.Validatable = RenameRequest{}
	_ core.Validatable = WriteRequest{}
	_ core.Validatable = StageRequest{}
	_ core.Validatable = StageDestinationRequest{}
	_ core.Validatable = ActivationRequest{}
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
