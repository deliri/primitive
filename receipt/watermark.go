package receipt

import (
	"encoding/json"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	scopeCanonicalJSONMaximumBytes = len(
		`{"account_identity":"","offering_identity":""}`,
	) + 2*LifecycleIdentityHexBytes
	// WatermarkCanonicalJSONMaximumBytes is the exact compact watermark bound.
	WatermarkCanonicalJSONMaximumBytes = len(
		`{"revision":"","scope":,"generation":,"cursor_digest":"","chain_hash":""}`,
	) + len("v1") + scopeCanonicalJSONMaximumBytes + 20 + 64 + 64
	scopeJSONWhitespaceAllowance     = 1 << 10
	watermarkJSONWhitespaceAllowance = 4 << 10
	// WatermarkJSONMaximumBytes bounds accepted durable watermark JSON.
	WatermarkJSONMaximumBytes = WatermarkCanonicalJSONMaximumBytes +
		watermarkJSONWhitespaceAllowance
)

type cursorDigestDomain uint8
type chainHashDomain uint8

// CursorDigest closes the exact remote cursor represented by a watermark.
type CursorDigest struct {
	value core.SHA256Digest
	_     cursorDigestDomain
}

// ChainHash closes the accepted evidence history represented by a watermark.
type ChainHash struct {
	value core.SHA256Digest
	_     chainHashDomain
}

// NewCursorDigest constructs a nominal cursor closure.
func NewCursorDigest(value core.SHA256Digest) (CursorDigest, error) {
	if err := value.Validate(); err != nil {
		return CursorDigest{}, contractError(errors.New("cursor digest is invalid"), err)
	}
	return CursorDigest{value: value}, nil
}

func (d CursorDigest) Validate() error {
	if err := d.value.Validate(); err != nil {
		return contractError(errors.New("cursor digest is unset"), err)
	}
	return nil
}

// NewChainHash constructs a nominal accepted-history closure.
func NewChainHash(value core.SHA256Digest) (ChainHash, error) {
	if err := value.Validate(); err != nil {
		return ChainHash{}, contractError(errors.New("chain hash is invalid"), err)
	}
	return ChainHash{value: value}, nil
}

func (h ChainHash) Validate() error {
	if err := h.value.Validate(); err != nil {
		return contractError(errors.New("chain hash is unset"), err)
	}
	return nil
}

func (d CursorDigest) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(d.value)
}
func (d *CursorDigest) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil cursor digest receiver"))
	}
	var value core.SHA256Digest
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewCursorDigest(value)
	if err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func (h ChainHash) MarshalJSON() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(h.value)
}
func (h *ChainHash) UnmarshalJSON(data []byte) error {
	if h == nil {
		return jsonError(errors.New("nil chain hash receiver"))
	}
	var value core.SHA256Digest
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewChainHash(value)
	if err != nil {
		return jsonError(err)
	}
	*h = candidate
	return nil
}

// Scope is the exact account and offering sequence namespace.
type Scope struct {
	Account  AccountIdentity  `json:"account_identity"`
	Offering OfferingIdentity `json:"offering_identity"`
}

func (s Scope) Validate() error {
	if err := s.Account.Validate(); err != nil {
		return contractError(errors.New("watermark account is invalid"), err)
	}
	if err := s.Offering.Validate(); err != nil {
		return contractError(errors.New("watermark offering is invalid"), err)
	}
	return nil
}

// MarshalJSON emits the scope through its canonical member order.
func (s Scope) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := json.Marshal(scopeWire{Account: &s.Account, Offering: &s.Offering})
	if err != nil || len(encoded) > scopeCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("watermark scope encoding exceeded its bound"), err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts one canonical scope without mutating on failure.
func (s *Scope) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil watermark scope receiver"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: scopeCanonicalJSONMaximumBytes + scopeJSONWhitespaceAllowance,
		depth:        1, fields: 2,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSONStructure[scopeWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	if wire.Account == nil || wire.Offering == nil {
		return jsonError(errors.New("watermark scope omits a required field"))
	}
	candidate := Scope{Account: *wire.Account, Offering: *wire.Offering}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*s = candidate
	return nil
}

// Watermark is one fixed-size durable high-water fact.
type Watermark struct {
	Generation   Generation   `json:"generation"`
	Scope        Scope        `json:"scope"`
	CursorDigest CursorDigest `json:"cursor_digest"`
	ChainHash    ChainHash    `json:"chain_hash"`
	Revision     Revision     `json:"revision"`
}

// watermarkWire fixes the canonical member order for both directions. Every
// member is a pointer, so every field has the same size and alignment and no
// layout optimizer can reorder the wire contract. Reordering members here
// changes the durable bytes; reordering Watermark's own fields does not.
type watermarkWire struct {
	Revision     *Revision     `json:"revision"`
	Scope        *Scope        `json:"scope"`
	Generation   *Generation   `json:"generation"`
	CursorDigest *CursorDigest `json:"cursor_digest"`
	ChainHash    *ChainHash    `json:"chain_hash"`
}

// scopeWire fixes the canonical member order of the nested scope object.
type scopeWire struct {
	Account  *AccountIdentity  `json:"account_identity"`
	Offering *OfferingIdentity `json:"offering_identity"`
}

// WatermarkRequest carries exact initial watermark facts.
type WatermarkRequest struct {
	Generation   Generation
	Scope        Scope
	CursorDigest CursorDigest
	ChainHash    ChainHash
}

func (r WatermarkRequest) Validate() error {
	return Watermark{
		Revision: RevisionV1, Scope: r.Scope, Generation: r.Generation,
		CursorDigest: r.CursorDigest, ChainHash: r.ChainHash,
	}.Validate()
}

// NewWatermark closes one durable high-water fact.
func NewWatermark(request WatermarkRequest) (Watermark, error) {
	if err := request.Validate(); err != nil {
		return Watermark{}, err
	}
	return Watermark{
		Revision: RevisionV1, Scope: request.Scope, Generation: request.Generation,
		CursorDigest: request.CursorDigest, ChainHash: request.ChainHash,
	}, nil
}

func (w Watermark) Validate() error {
	if err := w.Revision.Validate(); err != nil {
		return contractError(errors.New("watermark revision is invalid"), err)
	}
	if err := w.Scope.Validate(); err != nil {
		return contractError(err)
	}
	if err := w.Generation.Validate(); err != nil {
		return contractError(errors.New("watermark generation is invalid"), err)
	}
	if err := w.CursorDigest.Validate(); err != nil {
		return contractError(err)
	}
	if err := w.ChainHash.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// MarshalJSON emits the canonical durable watermark projection.
func (w Watermark) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := json.Marshal(watermarkWire{
		Revision: &w.Revision, Scope: &w.Scope, Generation: &w.Generation,
		CursorDigest: &w.CursorDigest, ChainHash: &w.ChainHash,
	})
	if err != nil || len(encoded) > WatermarkCanonicalJSONMaximumBytes {
		return nil, jsonError(errors.New("watermark encoding exceeded its bound"), err)
	}
	return encoded, nil
}

// UnmarshalJSON accepts bounded strict JSON without receiver mutation on failure.
func (w *Watermark) UnmarshalJSON(data []byte) error {
	if w == nil {
		return jsonError(errors.New("nil watermark receiver"))
	}
	limits, err := (jsonStructureContract{
		maximumBytes: WatermarkJSONMaximumBytes,
		depth:        2,
		fields:       5,
	}).limits()
	if err != nil {
		return err
	}
	wire, err := core.DecodeStrictJSONStructure[watermarkWire](data, limits)
	if err != nil {
		return jsonError(err)
	}
	if wire.Revision == nil || wire.Scope == nil || wire.Generation == nil ||
		wire.CursorDigest == nil || wire.ChainHash == nil {
		return jsonError(errors.New("watermark omits a required field"))
	}
	candidate := Watermark{
		Revision: *wire.Revision, Scope: *wire.Scope, Generation: *wire.Generation,
		CursorDigest: *wire.CursorDigest, ChainHash: *wire.ChainHash,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*w = candidate
	return nil
}

// AdvanceWatermarkRequest compares a durable current watermark with a candidate.
type AdvanceWatermarkRequest struct {
	Current   Watermark
	Candidate Watermark
}

func (r AdvanceWatermarkRequest) Validate() error {
	if err := r.Current.Validate(); err != nil {
		return contractError(errors.New("current watermark is invalid"), err)
	}
	if err := r.Candidate.Validate(); err != nil {
		return contractError(errors.New("candidate watermark is invalid"), err)
	}
	return nil
}

// AdvanceResult contains exactly one selected watermark.
type AdvanceResult struct {
	watermark Watermark
	state     AdvanceState
}

// AdvanceWatermark accepts exact replay or strict forward progress within one
// scope. Identity and scope decide before generation, and every rejection names
// its exact reason. A rejected advance always returns the zero result, so no
// caller can hold a selected watermark alongside a failure.
func AdvanceWatermark(request AdvanceWatermarkRequest) (AdvanceResult, error) {
	if err := request.Validate(); err != nil {
		return AdvanceResult{}, err
	}
	current := request.Current
	candidate := request.Candidate
	switch {
	case current.Scope != candidate.Scope:
		return AdvanceResult{}, conflictError(ConflictReasonScope)
	case candidate.Generation.value < current.Generation.value:
		return AdvanceResult{}, rollbackError()
	case candidate.Generation.value == current.Generation.value:
		return replayWatermark(current, candidate)
	default:
		return advanceWatermark(current, candidate)
	}
}

func replayWatermark(current, candidate Watermark) (AdvanceResult, error) {
	if current != candidate {
		return AdvanceResult{}, conflictError(ConflictReasonReplayDivergence)
	}
	return sealAdvance(current, AdvanceReplay)
}

func advanceWatermark(current, candidate Watermark) (AdvanceResult, error) {
	switch {
	case candidate.CursorDigest == current.CursorDigest:
		return AdvanceResult{}, conflictError(ConflictReasonCursorUnchanged)
	case candidate.ChainHash == current.ChainHash:
		return AdvanceResult{}, conflictError(ConflictReasonChainUnchanged)
	default:
		return sealAdvance(candidate, AdvanceAccepted)
	}
}

func sealAdvance(watermark Watermark, state AdvanceState) (AdvanceResult, error) {
	result := AdvanceResult{watermark: watermark, state: state}
	if err := result.Validate(); err != nil {
		return AdvanceResult{}, err
	}
	return result, nil
}

func (r AdvanceResult) Validate() error {
	if err := r.state.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.watermark.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// State returns the advance disposition of a sealed result.
func (r AdvanceResult) State() (AdvanceState, error) {
	if err := r.Validate(); err != nil {
		return AdvanceUnknown, err
	}
	return r.state, nil
}

// Watermark returns the exact watermark the advance selected.
func (r AdvanceResult) Watermark() (Watermark, error) {
	if err := r.Validate(); err != nil {
		return Watermark{}, err
	}
	return r.watermark, nil
}

var (
	_ core.Validatable = CursorDigest{}
	_ core.Validatable = ChainHash{}
	_ core.Validatable = Scope{}
	_ core.Validatable = Watermark{}
	_ core.Validatable = AdvanceResult{}
)
