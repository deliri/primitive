package lease

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// GenerationMaximumDecimalDigits is uint64's maximum decimal width.
	GenerationMaximumDecimalDigits = 20
	// GenerationCanonicalJSONMaximumBytes is the exact compact JSON maximum.
	GenerationCanonicalJSONMaximumBytes = GenerationMaximumDecimalDigits + len(`""`)
	generationJSONWhitespaceAllowance   = 256
	// GenerationJSONMaximumBytes bounds accepted generation JSON.
	GenerationJSONMaximumBytes = GenerationCanonicalJSONMaximumBytes +
		generationJSONWhitespaceAllowance
)

// Generation is a positive decision sequence number.
type Generation struct {
	value uint64
}

// NewGeneration constructs one positive generation.
func NewGeneration(value uint64) (Generation, error) {
	candidate := Generation{value: value}
	return candidate, candidate.Validate()
}

// ParseGeneration parses canonical unsigned decimal.
func ParseGeneration(text string) (Generation, error) {
	if !canonicalGeneration(text) {
		return Generation{}, contractError(errors.New("lease generation is not canonical"))
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return Generation{}, contractError(err)
	}
	return NewGeneration(value)
}

// Validate rejects zero.
func (g Generation) Validate() error {
	if g.value == 0 {
		return contractError(errors.New("lease generation is zero"))
	}
	return nil
}

// Uint64 returns the exact sequence number.
func (g Generation) Uint64() (uint64, error) {
	if err := g.Validate(); err != nil {
		return 0, err
	}
	return g.value, nil
}

// String returns canonical decimal or empty for an invalid value.
func (g Generation) String() string {
	if g.Validate() != nil {
		return ""
	}
	return strconv.FormatUint(g.value, 10)
}

// MarshalJSON emits a quoted canonical decimal to avoid numeric precision loss.
func (g Generation) MarshalJSON() ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(g.String())
}

// UnmarshalJSON accepts one quoted canonical decimal without mutation on
// rejection.
func (g *Generation) UnmarshalJSON(data []byte) error {
	if g == nil {
		return jsonError(errors.New("generation receiver is nil"))
	}
	if len(data) == 0 || len(data) > GenerationJSONMaximumBytes {
		return jsonError(errors.New("generation JSON extent is invalid"))
	}
	text, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	parsed, err := ParseGeneration(text)
	if err != nil {
		return err
	}
	*g = parsed
	return nil
}

func canonicalGeneration(text string) bool {
	if len(text) == 0 || len(text) > GenerationMaximumDecimalDigits ||
		text[0] == '0' {
		return false
	}
	for _, value := range []byte(text) {
		if value < '0' || value > '9' {
			return false
		}
	}
	return true
}
