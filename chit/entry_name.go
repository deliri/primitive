package chit

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// EntryNameMaximumBytes bounds one portable customer-visible manifest name.
	EntryNameMaximumBytes = 4096
	// EntryNameMaximumComponents bounds slash-delimited logical nesting.
	EntryNameMaximumComponents = 256
	// EntryNameComponentMaximumBytes is the portable component byte ceiling.
	EntryNameComponentMaximumBytes = 255
	entryNameSeparator             = "/"
	entryNameCurrent               = "."
	entryNameParent                = ".."
)

// EntryName is one portable slash-delimited manifest display path. It is not
// a local filesystem path and never implies that the named local path exists.
type EntryName struct{ value string }

// ParseEntryName closes product-owned display text into portable wire form.
func ParseEntryName(value string) (EntryName, error) {
	name := EntryName{value: value}
	if err := name.Validate(); err != nil {
		return EntryName{}, err
	}
	return name, nil
}

// Validate rejects ambiguous, platform-dependent, or unbounded names.
func (n EntryName) Validate() error {
	if err := validateEntryNameText(n.value); err != nil {
		return err
	}
	return validateEntryNameComponents(strings.Split(n.value, entryNameSeparator))
}

func validateEntryNameText(value string) error {
	if value == "" || len(value) > EntryNameMaximumBytes || !utf8.ValidString(value) {
		return contractError(errors.New("manifest entry name text is invalid"))
	}
	if strings.ContainsRune(value, 0) || strings.Contains(value, `\`) {
		return contractError(errors.New("manifest entry name is platform-dependent"))
	}
	return nil
}

func validateEntryNameComponents(components []string) error {
	if len(components) > EntryNameMaximumComponents {
		return contractError(errors.New("manifest entry name has too many components"))
	}
	for _, component := range components {
		if component == "" || component == entryNameCurrent || component == entryNameParent ||
			len(component) > EntryNameComponentMaximumBytes {
			return contractError(errors.New("manifest entry name component is invalid"))
		}
	}
	return nil
}

// String returns the canonical portable name.
func (n EntryName) String() string {
	if n.Validate() != nil {
		return ""
	}
	return n.value
}

// MarshalJSON emits the portable name as one canonical JSON string.
func (n EntryName) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(n.value)
}

// UnmarshalJSON admits one portable name transactionally.
func (n *EntryName) UnmarshalJSON(data []byte) error {
	if n == nil {
		return jsonError(errors.New("nil manifest entry name receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := ParseEntryName(value)
	if err != nil {
		return jsonError(err)
	}
	*n = parsed
	return nil
}

var (
	_ core.Validatable            = EntryName{}
	_ core.ValidatedJSONMarshaler = EntryName{}
)
