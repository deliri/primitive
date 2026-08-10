package gcsobjects

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// GCSObjectSegment is one validated leaf shared by logical directory and
// object-name composition. It never contains a separator.
type GCSObjectSegment struct{ value string }

// ParseGCSObjectSegment admits one nonempty provider object-name segment.
func ParseGCSObjectSegment(value string) (GCSObjectSegment, error) {
	segment := GCSObjectSegment{value: value}
	if err := segment.Validate(); err != nil {
		return GCSObjectSegment{}, err
	}
	return segment, nil
}

// String returns the validated segment.
func (s GCSObjectSegment) String() string { return s.value }

// Validate rejects separators, navigation aliases, control framing, and values
// that cannot fit inside any GCS object name.
func (s GCSObjectSegment) Validate() error {
	if len(s.value) == 0 || len(s.value) >= GCSObjectNameMaximumBytes ||
		!utf8.ValidString(s.value) || s.value == "." || s.value == ".." ||
		strings.ContainsAny(s.value, "/\r\n") {
		return core.ErrObjectStoreContract
	}
	return nil
}

type GCSRootPrefixRequest struct{ Segment GCSObjectSegment }

func (r GCSRootPrefixRequest) Validate() error { return r.Segment.Validate() }

// ComposeGCSRootPrefix creates one logical root directory as a typed prefix.
// Flat object storage performs no provider write merely to name a prefix.
func ComposeGCSRootPrefix(request GCSRootPrefixRequest) (GCSObjectPrefix, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectPrefix{}, err
	}
	return ParseGCSObjectPrefix(request.Segment.String() + "/")
}

type GCSChildPrefixRequest struct {
	Parent  GCSObjectPrefix
	Segment GCSObjectSegment
}

func (r GCSChildPrefixRequest) Validate() error {
	return errors.Join(r.Parent.Validate(), r.Segment.Validate())
}

// ComposeGCSChildPrefix creates one nested logical directory without copying
// slash conventions into callers.
func ComposeGCSChildPrefix(request GCSChildPrefixRequest) (GCSObjectPrefix, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectPrefix{}, err
	}
	return ParseGCSObjectPrefix(request.Parent.String() + request.Segment.String() + "/")
}

type GCSObjectInPrefixRequest struct {
	Prefix GCSObjectPrefix
	Leaf   GCSObjectSegment
}

func (r GCSObjectInPrefixRequest) Validate() error {
	return errors.Join(r.Prefix.Validate(), r.Leaf.Validate())
}

// ComposeGCSObjectName creates one exact object name under a typed prefix.
func ComposeGCSObjectName(request GCSObjectInPrefixRequest) (GCSObjectName, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectName{}, err
	}
	return ParseGCSObjectName(request.Prefix.String() + request.Leaf.String())
}

var (
	_ core.Validatable = GCSObjectSegment{}
	_ core.Validatable = GCSRootPrefixRequest{}
	_ core.Validatable = GCSChildPrefixRequest{}
	_ core.Validatable = GCSObjectInPrefixRequest{}
)
