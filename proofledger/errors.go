package proofledger

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(causes ...error) error {
	return errors.Join(append([]error{core.ErrProofLedgerContract}, causes...)...)
}

func jsonError(causes ...error) error {
	return errors.Join(append([]error{core.ErrJSONContract, core.ErrProofLedgerContract}, causes...)...)
}
