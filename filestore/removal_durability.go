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

// IsValid reports whether native removal has an explicit durability policy.
func (d RemovalDurability) IsValid() bool { return d.Validate() == nil }

// String renders the closed policy for diagnostics.
func (d RemovalDurability) String() string {
	switch d {
	case RemovalDurabilityUnknown:
		return "unknown"
	case RemovalDurabilityEphemeral:
		return "ephemeral"
	case RemovalDurabilityDurable:
		return "durable"
	default:
		return "invalid"
	}
}

// OffWireEnum prevents numeric wire reconstruction outside the owning contract.
func (RemovalDurability) OffWireEnum() {}

var _ core.Validatable = RemovalDurabilityUnknown
