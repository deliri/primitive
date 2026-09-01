package timeproof

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// serialMaximumBytes is RFC 5280's certificate-serial ceiling in bytes.
	serialMaximumBytes = SerialMaximumBits / 8
	// serialTokenMaximumLength bounds the canonical hexadecimal token.
	serialTokenMaximumLength = 2 * serialMaximumBytes
	// serialJSONMaximumBytes bounds the quoted canonical token.
	serialJSONMaximumBytes = serialTokenMaximumLength + 2
)

// SerialNumber is an exact positive RFC 3161 serial bounded by RFC 5280's
// 160-bit certificate-serial ceiling and carried without leading zero bytes.
type SerialNumber struct {
	value  [serialMaximumBytes]byte
	length uint8
}

func newSerialNumber(value *big.Int) (SerialNumber, error) {
	if value == nil || value.Sign() <= 0 || value.BitLen() > SerialMaximumBits {
		return SerialNumber{}, invalidError(nil)
	}
	raw := value.Bytes()
	length, err := core.CheckedUint8FromInt(len(raw))
	if err != nil {
		return SerialNumber{}, invalidError(err)
	}
	var serial SerialNumber
	copy(serial.value[len(serial.value)-len(raw):], raw)
	serial.length = length
	return serial, serial.Validate()
}

func parseSerialNumber(token string) (SerialNumber, error) {
	if len(token) < 2 || len(token) > serialTokenMaximumLength ||
		len(token)%2 != 0 {
		return SerialNumber{}, contractError(nil)
	}
	raw := make([]byte, len(token)/2)
	if err := core.DecodeCanonicalHex(raw, token); err != nil || raw[0] == 0 {
		return SerialNumber{}, contractError(nil)
	}
	return newSerialNumber(new(big.Int).SetBytes(raw))
}

// Validate rejects any serial that is absent, over-length, or noncanonically
// padded with leading zero bytes.
func (s SerialNumber) Validate() error {
	length := int(s.length)
	if length < 1 || length > len(s.value) {
		return contractError(nil)
	}
	offset := len(s.value) - length
	if s.value[offset] == 0 {
		return contractError(nil)
	}
	for _, value := range s.value[:offset] {
		if value != 0 {
			return contractError(nil)
		}
	}
	return nil
}

// String returns canonical lowercase hexadecimal without leading zero bytes.
func (s SerialNumber) String() string {
	if s.Validate() != nil {
		return ""
	}
	return hex.EncodeToString(s.value[len(s.value)-int(s.length):])
}

// MarshalJSON emits the canonical quoted serial token.
func (s SerialNumber) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return strconv.AppendQuote(nil, s.String()), nil
}

// UnmarshalJSON accepts only the exact canonical token and leaves the receiver
// unchanged on every rejection.
func (s *SerialNumber) UnmarshalJSON(data []byte) error {
	if s == nil {
		return errorsJSON()
	}
	token, err := decodeJSONToken(data, serialJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := parseSerialNumber(token)
	if err != nil {
		return errorsJSON()
	}
	canonical, err := parsed.MarshalJSON()
	if err != nil || !bytes.Equal(data, canonical) {
		return errorsJSON()
	}
	*s = parsed
	return nil
}
