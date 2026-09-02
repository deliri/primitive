package proofledger

import (
	"errors"
	"math"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const (
	PageEventMaximum               = 8
	PageJSONMaximumBytes           = 800 << 10
	pageFramingMaximumBytes        = 32 << 10
	_                         uint = PageJSONMaximumBytes - PageEventMaximum*EventJSONMaximumBytes - pageFramingMaximumBytes
	genesisDomain                  = "primitive-proof-ledger-genesis-v1"
	sequenceJSONMaximumBytes       = 20
	pageLimitJSONMaximumBytes      = 3
)

type Sequence uint64
type Position uint64
type PageLimit struct{ value uint16 }

func NewSequence(value uint64) (Sequence, error) {
	candidate := Sequence(value)
	return candidate, candidate.Validate()
}

func NewPageLimit(value uint16) (PageLimit, error) {
	candidate := PageLimit{value: value}
	return candidate, candidate.Validate()
}

func (p Position) Validate() error { return nil }

func (p Position) MarshalJSON() ([]byte, error) {
	return strconv.AppendUint(nil, uint64(p), 10), nil
}

func (p *Position) UnmarshalJSON(data []byte) error {
	if p == nil || len(data) == 0 || len(data) > sequenceJSONMaximumBytes {
		return jsonError()
	}
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonError(err)
	}
	*p = Position(value)
	return nil
}

func (s Sequence) Validate() error {
	if s == 0 {
		return contractError(errors.New("proof ledger sequence is zero"))
	}
	return nil
}

func (s Sequence) Next() (Sequence, error) {
	if err := s.Validate(); err != nil || s == Sequence(math.MaxUint64) {
		return 0, errors.Join(core.ErrProofLedgerSequenceConflict, err)
	}
	return s + 1, nil
}

func (s Sequence) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return strconv.AppendUint(nil, uint64(s), 10), nil
}

func (s *Sequence) UnmarshalJSON(data []byte) error {
	if s == nil || len(data) == 0 || len(data) > sequenceJSONMaximumBytes {
		return jsonError()
	}
	value, err := strconv.ParseUint(string(data), 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonError(err)
	}
	candidate, err := NewSequence(value)
	if err != nil {
		return jsonError(err)
	}
	*s = candidate
	return nil
}

func (l PageLimit) Validate() error {
	if l.value == 0 || l.value > PageEventMaximum {
		return contractError(errors.New("proof ledger page limit is outside its bound"))
	}
	return nil
}

func (l PageLimit) MarshalJSON() ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return strconv.AppendUint(nil, uint64(l.value), 10), nil
}

func (l *PageLimit) UnmarshalJSON(data []byte) error {
	if l == nil || len(data) == 0 || len(data) > pageLimitJSONMaximumBytes {
		return jsonError()
	}
	value, err := strconv.ParseUint(string(data), 10, 16)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return jsonError(err)
	}
	candidate, err := NewPageLimit(uint16(value))
	if err != nil {
		return jsonError(err)
	}
	*l = candidate
	return nil
}

func (l PageLimit) Uint16() (uint16, error) {
	if err := l.Validate(); err != nil {
		return 0, err
	}
	return l.value, nil
}

func GenesisHash() core.SHA256Digest { return core.SHA256Of([]byte(genesisDomain)) }
