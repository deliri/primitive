package receipt

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// ReceiptIDBytes is the exact receipt identity width.
	ReceiptIDBytes = 16
	// ReceiptIDHexBytes is the canonical receipt identity text width.
	ReceiptIDHexBytes   = ReceiptIDBytes * 2
	evidenceDomainToken = "primitive-receipt-evidence-v1"
	unknownText         = ""
)

// Revision is the closed Receipt wire revision.
type Revision uint8

const (
	RevisionUnknown Revision = iota
	RevisionV1
	revisionLimit
)

// Domain is the closed Attest signing domain owned by Receipt.
type Domain uint8

const (
	DomainUnknown Domain = iota
	DomainEvidenceV1
	domainLimit
)

// ScopeField identifies one expectation mismatch without exposing values.
type ScopeField uint8

const (
	ScopeFieldUnknown ScopeField = iota
	ScopeFieldAccount
	ScopeFieldOffering
	ScopeFieldSubmission
	ScopeFieldObject
	ScopeFieldExtent
	ScopeFieldSHA256
	ScopeFieldCRC32C
	scopeFieldLimit
)

// AdvanceState reports whether a watermark replayed or advanced.
type AdvanceState uint8

const (
	AdvanceUnknown AdvanceState = iota
	AdvanceAccepted
	AdvanceReplay
	advanceStateLimit
)

// ConflictReason names the exact invariant that refused a watermark advance.
type ConflictReason uint8

const (
	ConflictReasonUnknown ConflictReason = iota
	// ConflictReasonScope reports a candidate from a different account or offering.
	ConflictReasonScope
	// ConflictReasonReplayDivergence reports an equal generation whose facts differ.
	ConflictReasonReplayDivergence
	// ConflictReasonCursorUnchanged reports a higher generation reusing its cursor.
	ConflictReasonCursorUnchanged
	// ConflictReasonChainUnchanged reports a higher generation reusing its chain.
	ConflictReasonChainUnchanged
	conflictReasonLimit
)

// Each diagnostic projection is compiler-sized to its closed enum. Keyed rows
// make the mapping visible at the declaration and returning the array by value
// prevents package-global mutation from changing protocol text at runtime.
func revisionDiagnostics() [revisionLimit]string {
	return [...]string{
		unknownText,
		"v1",
	}
}

func scopeFieldDiagnostics() [scopeFieldLimit]string {
	return [...]string{
		unknownText,
		core.ProtocolMemberAccount,
		core.ProtocolMemberOffering,
		"submission",
		"object",
		"extent",
		"sha256",
		"crc32c",
	}
}

func advanceStateDiagnostics() [advanceStateLimit]string {
	return [...]string{
		unknownText,
		"accepted",
		"replay",
	}
}

func conflictReasonDiagnostics() [conflictReasonLimit]string {
	return [...]string{
		unknownText,
		"scope",
		"replay-divergence",
		"cursor-unchanged",
		"chain-unchanged",
	}
}

func (r Revision) Validate() error {
	if r <= RevisionUnknown || r >= revisionLimit ||
		revisionDiagnostics()[r] == "" {
		return contractError(errors.New("receipt revision is outside the closed domain"))
	}
	return nil
}

func (r Revision) IsValid() bool { return r.Validate() == nil }
func (r Revision) String() string {
	if !r.IsValid() {
		return unknownText
	}
	return revisionDiagnostics()[r]
}

func (r Revision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(r.String())
}

func (r *Revision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil receipt revision receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	if value != revisionDiagnostics()[RevisionV1] {
		return jsonError(errors.New("receipt revision text is unsupported"))
	}
	*r = RevisionV1
	return nil
}

func (d Domain) Validate() error {
	if d <= DomainUnknown || d >= domainLimit || domainDiagnostics()[d] == "" {
		return contractError(errors.New("receipt signing domain is outside the closed domain"))
	}
	return nil
}

func (d Domain) IsValid() bool { return d.Validate() == nil }
func (d Domain) String() string {
	if !d.IsValid() {
		return unknownText
	}
	return domainDiagnostics()[d]
}
func (Domain) OffWireEnum() {}

func (d Domain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(evidenceDomainToken), nil
}

func (Domain) ParseCanonicalText(text []byte) (Domain, error) {
	if string(text) != evidenceDomainToken {
		return DomainUnknown, contractError(errors.New("receipt signing domain text is unsupported"))
	}
	return DomainEvidenceV1, nil
}

func domainDiagnostics() [domainLimit]string {
	return [...]string{unknownText, evidenceDomainToken}
}

func (f ScopeField) Validate() error {
	if f <= ScopeFieldUnknown || f >= scopeFieldLimit ||
		scopeFieldDiagnostics()[f] == "" {
		return contractError(errors.New("receipt scope field is outside the closed domain"))
	}
	return nil
}
func (f ScopeField) IsValid() bool { return f.Validate() == nil }
func (f ScopeField) String() string {
	if !f.IsValid() {
		return unknownText
	}
	return scopeFieldDiagnostics()[f]
}
func (ScopeField) OffWireEnum() {}

func (s AdvanceState) Validate() error {
	if s <= AdvanceUnknown || s >= advanceStateLimit ||
		advanceStateDiagnostics()[s] == "" {
		return contractError(errors.New("receipt advance state is outside the closed domain"))
	}
	return nil
}
func (s AdvanceState) IsValid() bool { return s.Validate() == nil }
func (s AdvanceState) String() string {
	if !s.IsValid() {
		return unknownText
	}
	return advanceStateDiagnostics()[s]
}
func (AdvanceState) OffWireEnum() {}

func (r ConflictReason) Validate() error {
	if r <= ConflictReasonUnknown || r >= conflictReasonLimit ||
		conflictReasonDiagnostics()[r] == "" {
		return contractError(errors.New("receipt conflict reason is outside the closed domain"))
	}
	return nil
}
func (r ConflictReason) IsValid() bool { return r.Validate() == nil }
func (r ConflictReason) String() string {
	if !r.IsValid() {
		return unknownText
	}
	return conflictReasonDiagnostics()[r]
}
func (ConflictReason) OffWireEnum() {}

type receiptIDDomain uint8

// ReceiptID identifies one accepted-evidence fact.
type ReceiptID struct {
	value [ReceiptIDBytes]byte
	_     receiptIDDomain
}

// NewReceiptID constructs one nonzero identity.
func NewReceiptID(value [ReceiptIDBytes]byte) (ReceiptID, error) {
	if value == ([ReceiptIDBytes]byte{}) {
		return ReceiptID{}, contractError(errors.New("receipt identity is all zero"))
	}
	return ReceiptID{value: value}, nil
}

// ParseReceiptID accepts canonical lowercase hexadecimal.
func ParseReceiptID(value string) (ReceiptID, error) {
	var raw [ReceiptIDBytes]byte
	if err := decodeCanonicalIdentity(value, raw[:]); err != nil {
		return ReceiptID{}, contractError(errors.New("receipt identity text is invalid"), err)
	}
	return NewReceiptID(raw)
}

func (i ReceiptID) Validate() error {
	if i.value == ([ReceiptIDBytes]byte{}) {
		return contractError(errors.New("receipt identity is unset"))
	}
	return nil
}
func (i ReceiptID) String() string {
	if i.Validate() != nil {
		return unknownText
	}
	return hex.EncodeToString(i.value[:])
}
func (i ReceiptID) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(i.String())
}
func (i *ReceiptID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil receipt identity receiver"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	candidate, err := ParseReceiptID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

// Generation is a strictly positive monotonic watermark generation.
type Generation struct {
	value uint64
}

// NewGeneration constructs a positive generation.
func NewGeneration(value uint64) (Generation, error) {
	if value == 0 {
		return Generation{}, contractError(errors.New("receipt generation must be positive"))
	}
	return Generation{value: value}, nil
}
func (g Generation) Validate() error {
	if g.value == 0 {
		return contractError(errors.New("receipt generation is unset"))
	}
	return nil
}
func (g Generation) Uint64() (uint64, error) {
	if err := g.Validate(); err != nil {
		return 0, err
	}
	return g.value, nil
}
func (g Generation) MarshalJSON() ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return strconv.AppendUint(nil, g.value, 10), nil
}
func (g *Generation) UnmarshalJSON(data []byte) error {
	if g == nil {
		return jsonError(errors.New("nil receipt generation receiver"))
	}
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || string(data) != strconv.FormatUint(value, 10) {
		return jsonError(errors.New("receipt generation is not canonical"), err)
	}
	candidate, err := NewGeneration(value)
	if err != nil {
		return jsonError(err)
	}
	*g = candidate
	return nil
}

var (
	_ core.Validatable = RevisionUnknown
	_ core.Validatable = DomainUnknown
	_ core.Validatable = ScopeFieldUnknown
	_ core.Validatable = AdvanceUnknown
	_ core.Validatable = ConflictReasonUnknown
	_ core.Validatable = ReceiptID{}
	_ core.Validatable = Generation{}
	_ core.OffWireEnum = DomainUnknown
	_ core.OffWireEnum = ScopeFieldUnknown
	_ core.OffWireEnum = AdvanceUnknown
	_ core.OffWireEnum = ConflictReasonUnknown
)
