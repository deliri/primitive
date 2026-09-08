package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	goOOMBufferBytes = 32 << 10
	// GoOOMMaximumEvidenceBytes bounds one banner-classification source.
	GoOOMMaximumEvidenceBytes = 1 << 20

	// GoOOMPrefixedBanner and GoOOMPlainBanner are the exact Go runtime
	// diagnostics recognized by ClassifyGoOOMBanner. Consumers use these
	// compiler-visible values when constructing boundary fixtures.
	GoOOMPrefixedBanner = "fatal error: runtime: out of memory"
	GoOOMPlainBanner    = "fatal error: out of memory"

	goOOMAbsentToken         = "absent"
	goOOMPresentToken        = "present"
	goOOMEvidenceBytesPrefix = `{"bytes_examined":`
	goOOMEvidenceStatePrefix = `,"state":`
)

// GoOOMBannerState reports only whether a canonical Go runtime OOM banner was
// present. It does not claim that a process terminated or identify its cause.
type GoOOMBannerState uint8

const (
	GoOOMBannerUnknown GoOOMBannerState = iota
	GoOOMBannerAbsent
	GoOOMBannerPresent
	goOOMBannerLimit
)

// Validate rejects states outside the closed domain.
func (s GoOOMBannerState) Validate() error {
	if !s.IsValid() {
		return errors.Join(core.ErrHostFactsEvidence, errors.New("go OOM banner state is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed wire domain.
func (s GoOOMBannerState) IsValid() bool {
	return s > GoOOMBannerUnknown && s < goOOMBannerLimit &&
		goOOMBannerTokens()[s] != ""
}

// String returns the canonical wire token or "unknown" for an invalid value.
func (s GoOOMBannerState) String() string {
	if s >= goOOMBannerLimit || goOOMBannerTokens()[s] == "" {
		return core.UnknownEnumDiagnostic
	}
	return goOOMBannerTokens()[s]
}

func goOOMBannerTokens() [goOOMBannerLimit]string {
	return [...]string{"", goOOMAbsentToken, goOOMPresentToken}
}

// MarshalJSON emits the canonical state token.
func (s GoOOMBannerState) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return strconv.AppendQuote(nil, s.String()), nil
}

// UnmarshalJSON accepts one canonical state token without mutating on refusal.
func (s *GoOOMBannerState) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence)
	}
	token, err := decodeBannerToken(data)
	if err != nil {
		return err
	}
	decoded := GoOOMBannerUnknown
	switch token {
	case goOOMAbsentToken:
		decoded = GoOOMBannerAbsent
	case goOOMPresentToken:
		decoded = GoOOMBannerPresent
	default:
		return errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence)
	}
	*s = decoded
	return nil
}

func decodeBannerToken(data []byte) (string, error) {
	const maximum = len(goOOMPresentToken) + 2
	if len(data) == 0 || len(data) > maximum {
		return "", errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence)
	}
	token, err := strconv.Unquote(string(data))
	var canonical [maximum]byte
	if err != nil || !bytes.Equal(strconv.AppendQuote(canonical[:0], token), data) {
		return "", errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence, err)
	}
	return token, nil
}

// GoOOMBannerRequest declares the exact source extent to examine.
type GoOOMBannerRequest struct {
	Source io.Reader
	Length core.ByteLength
}

// Validate rejects a nil source or an extent beyond the production bound.
func (r GoOOMBannerRequest) Validate() error {
	if r.Source == nil || r.Length.Uint64() > GoOOMMaximumEvidenceBytes {
		return errors.Join(core.ErrHostFactsContract, errors.New("go OOM banner request is invalid"))
	}
	return nil
}

// GoOOMBannerEvidence is bounded, persistable banner-presence evidence.
type GoOOMBannerEvidence struct {
	examined core.ByteLength
	state    GoOOMBannerState
}

// Validate rejects evidence that the production classifier could not emit.
func (e GoOOMBannerEvidence) Validate() error {
	if e.examined.Uint64() > GoOOMMaximumEvidenceBytes {
		return errors.Join(core.ErrHostFactsEvidence, errors.New("go OOM evidence extent exceeds the classifier bound"))
	}
	if err := e.state.Validate(); err != nil {
		return err
	}
	if e.state == GoOOMBannerPresent &&
		e.examined.Uint64() < uint64(len(GoOOMPlainBanner)) {
		return errors.Join(core.ErrHostFactsEvidence, errors.New("go OOM banner presence contradicts examined extent"))
	}
	return nil
}

// BytesExamined returns the exact declared extent consumed.
func (e GoOOMBannerEvidence) BytesExamined() core.ByteLength {
	return e.examined
}

// State returns banner presence or absence.
func (e GoOOMBannerEvidence) State() GoOOMBannerState {
	return e.state
}

type goOOMBannerWire struct {
	BytesExamined *core.ByteLength  `json:"bytes_examined"`
	State         *GoOOMBannerState `json:"state"`
}

func (w goOOMBannerWire) Validate() error {
	if w.BytesExamined == nil || w.State == nil {
		return core.ErrHostFactsEvidence
	}
	if w.BytesExamined.Uint64() > GoOOMMaximumEvidenceBytes {
		return core.ErrHostFactsEvidence
	}
	return w.State.Validate()
}

func goOOMEvidenceJSONLimits() core.StrictJSONLimits {
	const maximumDocumentBytes = len(goOOMEvidenceBytesPrefix) + 20 +
		len(goOOMEvidenceStatePrefix) + len(`"present"`) + len(`}`)
	maximum, _ := core.NewByteCount(uint64(maximumDocumentBytes))
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	limits.NestingDepthMaximum = 1
	limits.ObjectFieldMaximum = 2
	limits.ArrayItemMaximum = 1
	return limits
}

// MarshalJSON emits one canonical object.
func (e GoOOMBannerEvidence) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	data := append([]byte(goOOMEvidenceBytesPrefix), strconv.FormatUint(e.examined.Uint64(), 10)...)
	data = append(data, goOOMEvidenceStatePrefix...)
	data = strconv.AppendQuote(data, e.state.String())
	return append(data, '}'), nil
}

// UnmarshalJSON accepts only the canonical object and preserves the receiver
// on refusal.
func (e *GoOOMBannerEvidence) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence)
	}
	wire, err := core.DecodeStrictJSON[goOOMBannerWire](bytes.NewReader(data), goOOMEvidenceJSONLimits())
	if err != nil || wire.BytesExamined == nil || wire.State == nil {
		return errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence, err)
	}
	decoded := GoOOMBannerEvidence{examined: *wire.BytesExamined, state: *wire.State}
	if err := decoded.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	canonical, err := decoded.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, data) {
		return errors.Join(core.ErrJSONContract, core.ErrHostFactsEvidence, err)
	}
	*e = decoded
	return nil
}

type oomScanner struct {
	source     io.Reader
	found      bool
	carry      int
	remaining  uint64
	emptyReads int
	buffer     [goOOMBufferBytes + goOOMOverlapBytes]byte
}

func (s *oomScanner) read(ctx context.Context) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	maximum := min(uint64(goOOMBufferBytes), s.remaining)
	count, readErr := s.source.Read(s.buffer[s.carry : s.carry+int(maximum)])
	if count < 0 || uint64(count) > maximum {
		return errors.Join(core.ErrHostFactsObservation, errors.New("reader returned an invalid count"))
	}
	if count > 0 {
		s.match(count)
		s.remaining -= uint64(count)
		s.emptyReads = 0
	} else {
		s.emptyReads++
	}
	return classifyOOMRead(s.remaining, s.emptyReads, readErr)
}

// A match crossing two reads needs only the longest banner minus one byte
// from the previous read. bytes.Contains owns the search implementation.
const goOOMOverlapBytes = max(len(GoOOMPlainBanner), len(GoOOMPrefixedBanner)) - 1

func (s *oomScanner) match(count int) {
	if s.found {
		return
	}
	data := s.buffer[:s.carry+count]
	s.found = bytes.Contains(data, []byte(GoOOMPlainBanner)) || bytes.Contains(data, []byte(GoOOMPrefixedBanner))
	s.carry = min(len(data), goOOMOverlapBytes)
	copy(s.buffer[:s.carry], data[len(data)-s.carry:])
}

func classifyOOMRead(remaining uint64, emptyReads int, readErr error) error {
	if remaining == 0 {
		return nil
	}
	if errors.Is(readErr, io.EOF) {
		return errors.Join(io.ErrUnexpectedEOF, readErr)
	}
	if readErr != nil {
		return readErr
	}
	if emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
		return io.ErrNoProgress
	}
	return nil
}
