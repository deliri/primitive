package garble

import (
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// CustodyBytes is the exact long-lived custody-root width.
	CustodyBytes           = 64
	custodyExtentErrorText = "garble custody must contain exactly 64 bytes"
)

// Custody is proof that Core-owned secret material has the exact width needed
// by the Garble derivation protocol. It exposes no raw or serialization path.
type Custody struct {
	material core.SecretMaterial
}

// NewCustody admits only active, exact-width Core secret material.
func NewCustody(material core.SecretMaterial) (Custody, error) {
	if err := material.Validate(); err != nil {
		return Custody{}, contractError(err)
	}
	count, err := material.ByteCount()
	if err != nil {
		return Custody{}, contractError(err)
	}
	value, err := count.Uint64()
	if err != nil {
		return Custody{}, contractError(err)
	}
	if value != CustodyBytes {
		return Custody{}, contractError(errors.New(custodyExtentErrorText))
	}
	return Custody{material: material}, nil
}

// Validate checks that the shared Core material remains active and exact-width.
func (c Custody) Validate() error {
	_, err := NewCustody(c.material)
	return err
}

// Format prevents custody disclosure through formatting.
func (c Custody) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}
