package controlwire

import (
	"github.com/deliri/primitive/v2026/core"
)

// Revision2026V1Token is the canonical text of the published 2026.1 control
// wire revision.
const Revision2026V1Token = "2026.1"

// Revision is the closed set of published control wire revisions. A build
// states which revision it speaks; it never negotiates one.
type Revision uint8

const (
	// RevisionUnknown is the invalid zero revision.
	RevisionUnknown Revision = iota
	// Revision2026V1 is the published 2026.1 revision.
	Revision2026V1
	revisionLimit
)

func revisionTokens() [revisionLimit]string {
	return [...]string{
		RevisionUnknown: "",
		Revision2026V1:  Revision2026V1Token,
	}
}

// Validate rejects the unset revision and every unpublished revision.
func (r Revision) Validate() error {
	if r <= RevisionUnknown || r >= revisionLimit || revisionTokens()[r] == "" {
		return revisionError()
	}
	return nil
}

// IsValid reports whether r is a published revision.
func (r Revision) IsValid() bool { return r.Validate() == nil }

// String returns the canonical revision text, or empty text when unset.
func (r Revision) String() string {
	if r >= revisionLimit {
		return ""
	}
	return revisionTokens()[r]
}

// ParseRevision accepts one exact published token. An unrecognized token is
// refused rather than assumed compatible, so a build never speaks a contract it
// does not implement.
func ParseRevision(value string) (Revision, error) {
	tokens := revisionTokens()
	for revision := RevisionUnknown + 1; revision < revisionLimit; revision++ {
		if tokens[revision] != "" && tokens[revision] == value {
			return revision, nil
		}
	}
	return RevisionUnknown, revisionError()
}

// MarshalJSON emits the canonical revision token and refuses an unset value, so
// no request can ask the other end to guess which contract it speaks.
func (r Revision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(r.String())
	if err != nil {
		return nil, jsonError(revisionError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts only a published revision token and leaves r unchanged
// on every rejection.
func (r *Revision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(revisionError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(revisionError(err))
	}
	parsed, err := ParseRevision(token)
	if err != nil {
		return jsonError(err)
	}
	*r = parsed
	return nil
}
