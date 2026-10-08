package filestore

import "github.com/deliri/primitive/v2026/core"

// RemovalDurability selects the caller's namespace persistence requirement.
// Ephemeral removal executes unlink only. Durable removal also synchronizes
// the parent. Zero and unknown policies refuse before any native mutation.
type RemovalDurability uint8

const (
	RemovalDurabilityUnknown RemovalDurability = iota
	RemovalDurabilityEphemeral
	RemovalDurabilityDurable
)

func (d RemovalDurability) Validate() error {
	switch d {
	case RemovalDurabilityEphemeral, RemovalDurabilityDurable:
		return nil
	default:
		return core.ErrFilestoreContract
	}
}

var _ core.Validatable = RemovalDurabilityUnknown
