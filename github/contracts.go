package github

import (
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	repositorySeparator            = "/"
	referenceCustodyMaximumBytes   = 1024
	userAgentCustodyMaximumBytes   = 256
	minimumTreeRequestEntryMaximum = 1
	// TarArchiveMaximumBytes is Primitive's hard ceiling for one streamed
	// repository archive. GitHub publishes no archive-size maximum, so products
	// may only tighten this one-gibibyte mechanical custody limit.
	// Provider contract: https://docs.github.com/en/rest/repos/contents?apiVersion=2026-03-10#download-a-repository-archive-tar
	TarArchiveMaximumBytes = 1 << 30
)

// Repository is one GitHub owner and repository-name pair. Primitive validates
// its mechanical path shape without assigning product meaning to either name.
type Repository struct {
	identity core.RepositoryIdentity
	owner    string
	name     string
}

// ParseRepository accepts exactly one owner/name coordinate. GitHub does not
// publish one stable combined byte maximum; the outer RepositoryIdentity bound
// is therefore a Primitive custody limit rather than a provider claim.
func ParseRepository(value string) (Repository, error) {
	identity, err := core.NewRepositoryIdentity(value)
	if err != nil {
		return Repository{}, contractError(err)
	}
	owner, name, ok := strings.Cut(value, repositorySeparator)
	candidate := Repository{identity: identity, owner: owner, name: name}
	if !ok || strings.Contains(name, repositorySeparator) || candidate.Validate() != nil {
		return Repository{}, core.ErrGitHubContract
	}
	return candidate, nil
}

// Validate rejects unset, ambiguous, traversal-bearing, or URL-control names.
func (r Repository) Validate() error {
	if err := r.identity.Validate(); err != nil {
		return contractError(err)
	}
	if !validRepositorySegment(r.owner) || !validRepositorySegment(r.name) ||
		r.identity.String() != r.owner+repositorySeparator+r.name {
		return core.ErrGitHubContract
	}
	return nil
}

// String returns owner/name after validation.
func (r Repository) String() string {
	if r.Validate() != nil {
		return ""
	}
	return r.identity.String()
}

func validRepositorySegment(value string) bool {
	if value == "" || value == "." || value == ".." || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) || unicode.IsSpace(current) || strings.ContainsRune("/\\?#%", current) {
			return false
		}
	}
	return true
}

// UserAgent is the caller's bounded GitHub request identity.
// Source for GitHub's User-Agent requirement:
// https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api?apiVersion=2026-03-10#user-agent
type UserAgent struct{ value string }

// ParseUserAgent admits one HTTP-safe caller identity under Primitive custody.
func ParseUserAgent(value string) (UserAgent, error) {
	candidate := UserAgent{value: value}
	if err := candidate.Validate(); err != nil {
		return UserAgent{}, err
	}
	return candidate, nil
}

// Validate rejects absent, oversized, padded, or control-bearing identities.
func (u UserAgent) Validate() error {
	if u.value == "" || len(u.value) > userAgentCustodyMaximumBytes || strings.TrimSpace(u.value) != u.value {
		return core.ErrGitHubContract
	}
	for _, current := range u.value {
		if unicode.IsControl(current) {
			return core.ErrGitHubContract
		}
	}
	return nil
}

// String returns the admitted header value.
func (u UserAgent) String() string {
	if u.Validate() != nil {
		return ""
	}
	return u.value
}

// Reference is one bounded GitHub ref or tag name. It is deliberately not a
// product release version; products decide whether any tag has release meaning.
type Reference struct{ value string }

// ParseReference admits one provider ref under Primitive's custody ceiling.
func ParseReference(value string) (Reference, error) {
	candidate := Reference{value: value}
	if err := candidate.Validate(); err != nil {
		return Reference{}, err
	}
	return candidate, nil
}

// Validate rejects empty, oversized, invalid UTF-8, or control-bearing refs.
func (r Reference) Validate() error {
	if r.value == "" || len(r.value) > referenceCustodyMaximumBytes || !utf8.ValidString(r.value) {
		return core.ErrGitHubContract
	}
	for _, current := range r.value {
		if unicode.IsControl(current) {
			return core.ErrGitHubContract
		}
	}
	return nil
}

// String returns the admitted ref.
func (r Reference) String() string {
	if r.Validate() != nil {
		return ""
	}
	return r.value
}

// Tag is GitHub's exact tag-name to commit observation. It carries no release policy.
type Tag struct {
	Name   Reference
	Commit core.BuildCommit
}

// Validate checks the observed provider facts.
func (t Tag) Validate() error {
	if err := errors.Join(t.Name.Validate(), t.Commit.Validate()); err != nil {
		return responseError(err)
	}
	return nil
}

// TagPageRequest requests one exact GitHub page. Page is one-based.
type TagPageRequest struct {
	Repository Repository
	Page       uint32
}

// Validate checks the repository and one-based page.
func (r TagPageRequest) Validate() error {
	if r.Page == 0 {
		return core.ErrGitHubContract
	}
	return r.Repository.Validate()
}

// TagPage is one bounded provider page and its exact next-page observation.
// NextPage is zero when GitHub did not advertise a continuation.
type TagPage struct {
	Repository Repository
	Tags       []Tag
	Page       uint32
	NextPage   uint32
}

// Validate rejects oversized pages, invalid tags, and impossible continuation.
func (p TagPage) Validate() error {
	if err := p.Repository.Validate(); err != nil || p.Page == 0 || len(p.Tags) > core.GitHubTagPageMaximumEntries {
		return responseError(err)
	}
	if p.NextPage != 0 && p.NextPage != p.Page+1 {
		return core.ErrGitHubResponse
	}
	for _, tag := range p.Tags {
		if err := tag.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// HeadRequest requests the repository's current leading commit observation.
type HeadRequest struct{ Repository Repository }

// Validate checks the repository coordinate.
func (r HeadRequest) Validate() error { return r.Repository.Validate() }

// HeadObservation identifies the exact observed leading commit.
type HeadObservation struct {
	Repository Repository
	Commit     core.BuildCommit
}

// Validate checks repository and commit binding.
func (o HeadObservation) Validate() error {
	if err := errors.Join(o.Repository.Validate(), o.Commit.Validate()); err != nil {
		return responseError(err)
	}
	return nil
}

// FileRequest requests one exact path at one immutable commit. MaximumBytes is
// product policy and may only tighten GitHub's inline-contents provider ceiling.
type FileRequest struct {
	Repository   Repository
	Path         core.SourcePath
	MaximumBytes core.ByteCount
	Commit       core.BuildCommit
}

// Validate checks the complete exact-source request before transport.
func (r FileRequest) Validate() error {
	if err := errors.Join(r.Repository.Validate(), r.Commit.Validate(), r.Path.Validate(), r.MaximumBytes.Validate()); err != nil {
		return contractError(err)
	}
	maximum, err := r.MaximumBytes.Uint64()
	if err != nil || maximum > core.GitHubContentsInlineMaximumBytes {
		return core.ErrGitHubContract
	}
	return nil
}

// FileObservation is the exact bounded file returned by GitHub.
type FileObservation struct {
	Repository Repository
	Path       core.SourcePath
	Content    []byte
	Length     core.ByteLength
	Commit     core.BuildCommit
	SHA256     core.SHA256Digest
}

// ArchiveTransferState closes the mechanical outcome of one streamed archive
// transfer. An incomplete observation accounts for every byte accepted by the
// caller's destination before the typed transfer failure.
type ArchiveTransferState uint8

const (
	ArchiveTransferUnknown ArchiveTransferState = iota
	ArchiveTransferIncomplete
	ArchiveTransferComplete
	archiveTransferStateLimit
)

// Validate rejects unknown and future transfer states until admitted here.
func (s ArchiveTransferState) Validate() error {
	if s <= ArchiveTransferUnknown || s >= archiveTransferStateLimit {
		return core.ErrGitHubResponse
	}
	return nil
}

// IsValid reports whether the state belongs to the closed domain.
func (s ArchiveTransferState) IsValid() bool { return s.Validate() == nil }

// String returns a diagnostic projection, never provider wire text.
func (s ArchiveTransferState) String() string {
	if !s.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{"", "incomplete", "complete"}[s]
}

// OffWireEnum declares the state as a typed observation, not provider wire text.
func (ArchiveTransferState) OffWireEnum() {}

// TarArchiveRequest streams one tar archive for an immutable commit into a
// caller-owned destination. MaximumBytes is product policy and may only
// tighten Primitive's mechanical archive custody ceiling.
type TarArchiveRequest struct {
	Destination  io.Writer
	Repository   Repository
	MaximumBytes core.ByteCount
	Commit       core.BuildCommit
}

// Validate checks the complete exact-source transfer request.
func (r TarArchiveRequest) Validate() error {
	if err := errors.Join(r.Repository.Validate(), r.Commit.Validate(), r.MaximumBytes.Validate()); err != nil || r.Destination == nil {
		return contractError(err)
	}
	maximum, err := r.MaximumBytes.Uint64()
	if err != nil || maximum > TarArchiveMaximumBytes {
		return core.ErrGitHubContract
	}
	return nil
}

// TarArchiveObservation binds the bytes accepted by the destination to the
// exact requested repository and commit. Incomplete observations accompany a
// non-nil transfer error and preserve partial-result accounting.
type TarArchiveObservation struct {
	Repository Repository
	Commit     core.BuildCommit
	SHA256     core.SHA256Digest
	Length     core.ByteLength
	State      ArchiveTransferState
}

// Validate checks coordinate, digest, extent, and transfer-state ownership.
func (o TarArchiveObservation) Validate() error {
	if err := errors.Join(o.Repository.Validate(), o.Commit.Validate(), o.SHA256.Validate(), o.Length.Validate(), o.State.Validate()); err != nil {
		return responseError(err)
	}
	if o.Length.Uint64() > TarArchiveMaximumBytes ||
		(o.State == ArchiveTransferComplete && o.Length.Uint64() == 0) {
		return core.ErrGitHubResponse
	}
	return nil
}

// Validate proves coordinate binding, length, digest, and provider ceiling.
func (o FileObservation) Validate() error {
	if err := errors.Join(o.Repository.Validate(), o.Commit.Validate(), o.Path.Validate(), o.Length.Validate(), o.SHA256.Validate()); err != nil {
		return responseError(err)
	}
	if uint64(len(o.Content)) != o.Length.Uint64() || len(o.Content) > core.GitHubContentsInlineMaximumBytes || core.SHA256Of(o.Content) != o.SHA256 {
		return core.ErrGitHubResponse
	}
	return nil
}

// TreeEntryKind closes the GitHub Git-tree object domain.
type TreeEntryKind uint8

const (
	TreeEntryUnknown TreeEntryKind = iota
	TreeEntryBlob
	TreeEntryDirectory
	TreeEntrySubmodule
	treeEntryKindLimit
)

// Validate rejects unknown and future entry kinds until explicitly admitted.
func (k TreeEntryKind) Validate() error {
	if k <= TreeEntryUnknown || k >= treeEntryKindLimit {
		return core.ErrGitHubResponse
	}
	return nil
}

// IsValid reports whether k belongs to the closed tree-entry domain.
func (k TreeEntryKind) IsValid() bool { return k.Validate() == nil }

// String returns a diagnostic projection, never a provider wire token.
func (k TreeEntryKind) String() string {
	if !k.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{"", "blob", "directory", "submodule"}[k]
}

// OffWireEnum declares the enum as a typed observation, not provider wire text.
func (TreeEntryKind) OffWireEnum() {}

// TreeEntry is one typed recursive-tree observation.
type TreeEntry struct {
	Path core.SourcePath
	Kind TreeEntryKind
}

// Validate checks path and closed kind.
func (e TreeEntry) Validate() error {
	if err := errors.Join(e.Path.Validate(), e.Kind.Validate()); err != nil {
		return responseError(err)
	}
	return nil
}

// TreeVisitor consumes entries as GitHub streams them. The consumer owns any
// retained aggregation and therefore its product-specific memory policy.
type TreeVisitor interface {
	VisitGitHubTreeEntry(TreeEntry) error
}

// TreeRequest requests a recursive immutable tree with a caller-owned tighter bound.
type TreeRequest struct {
	Visitor        TreeVisitor
	Repository     Repository
	MaximumEntries uint64
	Commit         core.BuildCommit
}

// Validate checks exact coordinates, visitor ownership, and provider ceiling.
func (r TreeRequest) Validate() error {
	if err := errors.Join(r.Repository.Validate(), r.Commit.Validate()); err != nil {
		return contractError(err)
	}
	if r.MaximumEntries < minimumTreeRequestEntryMaximum || r.MaximumEntries > core.GitHubRecursiveTreeMaximumEntries || r.Visitor == nil {
		return core.ErrGitHubContract
	}
	return nil
}

// TreeObservation reports the exact completed streamed response extent.
type TreeObservation struct {
	Repository Repository
	Commit     core.BuildCommit
	Entries    uint64
	Bytes      core.ByteLength
}

// Validate checks coordinates and provider ceilings.
func (o TreeObservation) Validate() error {
	if err := errors.Join(o.Repository.Validate(), o.Commit.Validate(), o.Bytes.Validate()); err != nil {
		return responseError(err)
	}
	if o.Entries > core.GitHubRecursiveTreeMaximumEntries || o.Bytes.Uint64() > core.GitHubRecursiveTreeMaximumBytes {
		return core.ErrGitHubResponse
	}
	return nil
}

var (
	_ core.Validatable = Repository{}
	_ core.Validatable = UserAgent{}
	_ core.Validatable = Reference{}
	_ core.Validatable = Tag{}
	_ core.Validatable = TagPageRequest{}
	_ core.Validatable = TagPage{}
	_ core.Validatable = HeadRequest{}
	_ core.Validatable = HeadObservation{}
	_ core.Validatable = FileRequest{}
	_ core.Validatable = FileObservation{}
	_ core.Validatable = ArchiveTransferState(0)
	_ core.OffWireEnum = ArchiveTransferState(0)
	_ core.Validatable = TarArchiveRequest{}
	_ core.Validatable = TarArchiveObservation{}
	_ core.Validatable = TreeEntryKind(0)
	_ core.Validatable = TreeEntry{}
	_ core.Validatable = TreeRequest{}
	_ core.Validatable = TreeObservation{}
)
