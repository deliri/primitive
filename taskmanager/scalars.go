package taskmanager

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// PageLimitMaximum bounds every task-manager collection response.
	PageLimitMaximum PageLimit = 100
	// TitleMaximumRunes bounds one project, phase, or task title.
	TitleMaximumRunes = 160
	// DescriptionMaximumRunes bounds one project, phase, or task description.
	DescriptionMaximumRunes = 4000
	// EvidenceSummaryMaximumRunes bounds one evidence summary.
	EvidenceSummaryMaximumRunes = 1000
	// RepositoryMaximumRunes bounds one repository identity.
	RepositoryMaximumRunes = 255
	// CommitSummaryMaximumRunes bounds one Git commit summary.
	CommitSummaryMaximumRunes = 512
)

// PageLimit is the exact requested collection ceiling.
type PageLimit uint16

func (l PageLimit) Validate() error {
	if l == 0 || l > PageLimitMaximum {
		return contractError()
	}
	return nil
}

func (l PageLimit) IsValid() bool { return l.Validate() == nil }

func (l PageLimit) String() string {
	if err := l.Validate(); err != nil {
		return ""
	}
	return strconv.FormatUint(uint64(l), 10)
}

func (l PageLimit) MarshalJSON() ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	return strconv.AppendUint(nil, uint64(l), 10), nil
}

func (l *PageLimit) UnmarshalJSON(data []byte) error {
	if l == nil {
		return jsonContractError()
	}
	value, err := strconv.ParseUint(string(data), 10, 16)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonContractError(err)
	}
	parsed := PageLimit(value)
	if err := parsed.Validate(); err != nil {
		return jsonContractError(err)
	}
	*l = parsed
	return nil
}

// Revision is one positive entity-local optimistic revision.
type Revision struct{ value uint64 }

func NewRevision(value uint64) (Revision, error) {
	candidate := Revision{value: value}
	if err := candidate.Validate(); err != nil {
		return Revision{}, err
	}
	return candidate, nil
}

func (r Revision) Validate() error {
	if r.value == 0 {
		return contractError()
	}
	return nil
}

func (r Revision) Uint64() (uint64, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	return r.value, nil
}

func (r Revision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonContractError(err)
	}
	return strconv.AppendUint(nil, r.value, 10), nil
}

func (r *Revision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonContractError()
	}
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonContractError(err)
	}
	parsed, err := NewRevision(value)
	if err != nil {
		return jsonContractError(err)
	}
	*r = parsed
	return nil
}

// Title is one bounded human-facing project, phase, or task title.
type Title string

func ParseTitle(value string) (Title, error) {
	parsed := Title(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (t Title) Validate() error {
	return validateText(string(t), TitleMaximumRunes, false)
}

func (t Title) String() string { return string(t) }

// Description is one optional bounded task-manager description.
type Description string

func ParseDescription(value string) (Description, error) {
	parsed := Description(value)
	if err := parsed.Validate(); err != nil {
		return "", err
	}
	return parsed, nil
}

func (d Description) Validate() error {
	return validateText(string(d), DescriptionMaximumRunes, true)
}

func (d Description) String() string { return string(d) }

func validateText(value string, maximumRunes int, optional bool) error {
	if optional && value == "" {
		return nil
	}
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) ||
		utf8.RuneCountInString(value) > maximumRunes || strings.ContainsAny(value, "\x00\r\n") {
		return contractError()
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return contractError()
		}
	}
	return nil
}

type enumFact[Enum ~uint8] struct {
	value Enum
	name  string
}

func enumName[Enum ~uint8](value Enum, facts []enumFact[Enum]) (string, error) {
	for _, fact := range facts {
		if fact.value == value && fact.name != "" {
			return fact.name, nil
		}
	}
	return "", contractError()
}

func parseEnum[Enum ~uint8](value string, facts []enumFact[Enum]) (Enum, error) {
	for _, fact := range facts {
		if fact.name == value {
			return fact.value, nil
		}
	}
	var zero Enum
	return zero, contractError()
}

func marshalEnum[Enum ~uint8](value Enum, facts []enumFact[Enum]) ([]byte, error) {
	name, err := enumName(value, facts)
	if err != nil {
		return nil, jsonContractError(err)
	}
	return core.MarshalCanonicalJSONString(name)
}

func unmarshalEnum[Enum ~uint8](data []byte, facts []enumFact[Enum]) (Enum, error) {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		var zero Enum
		return zero, jsonContractError(err)
	}
	parsed, err := parseEnum(value, facts)
	if err != nil {
		var zero Enum
		return zero, jsonContractError(err)
	}
	return parsed, nil
}

var (
	_ core.ValidatedJSONMarshaler = PageLimit(0)
	_ core.Validatable            = Revision{}
	_ core.ValidatedJSONMarshaler = Revision{}
)
