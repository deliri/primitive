package capabilities

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Effect identifies a real-world boundary that product policy must reach
// through Primitive rather than implement independently.
type Effect uint8

const (
	// EffectUnknown is outside the admitted effect domain.
	EffectUnknown Effect = iota
	// EffectFilesystem covers filesystem observation and mutation.
	EffectFilesystem
	// EffectProcess covers process execution and process-state observation.
	EffectProcess
	// EffectTransport covers HTTP and network transport.
	EffectTransport
	// EffectTime covers wall time, durations, deadlines, waits, and tickers.
	EffectTime
	// EffectEntropy covers cryptographic randomness and generated secret material.
	EffectEntropy
	// EffectSecret covers provider-managed secret access.
	EffectSecret
	// EffectHost covers host resource and platform observation.
	EffectHost
	// EffectLocking covers operating-system and filesystem-backed locking.
	EffectLocking
	// EffectSignal covers operating-system signal observation and shutdown.
	EffectSignal
	// EffectObjectStorage covers remote object observation and mutation.
	EffectObjectStorage
	effectLimit
)

// Validate rejects values outside the closed effect domain.
func (e Effect) Validate() error {
	if e <= EffectUnknown || e >= effectLimit {
		return contractError("effect is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether e belongs to the closed effect domain.
func (e Effect) IsValid() bool { return e.Validate() == nil }

// OffWireEnum marks Effect as a compiler-only enum.
func (Effect) OffWireEnum() {}

// String returns the stable doctrine identity of a valid effect.
func (e Effect) String() string {
	if !e.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return effectNames()[e]
}

func effectNames() [effectLimit]string {
	return [...]string{
		EffectFilesystem:    "filesystem",
		EffectProcess:       "process",
		EffectTransport:     "transport",
		EffectTime:          "time",
		EffectEntropy:       "entropy",
		EffectSecret:        "secret",
		EffectHost:          "host",
		EffectLocking:       "locking",
		EffectSignal:        "signal",
		EffectObjectStorage: "object_storage",
	}
}

func effectOwner(effect Effect) (core.PackageIdentity, error) {
	if err := effect.Validate(); err != nil {
		return core.PackageUnknown, err
	}
	switch effect {
	case EffectFilesystem:
		return core.PackageFilestore, nil
	case EffectProcess:
		return core.PackageProcess, nil
	case EffectTransport:
		return core.PackageExchange, nil
	case EffectTime:
		return core.PackageTemporal, nil
	case EffectEntropy:
		return core.PackageKeygen, nil
	case EffectSecret:
		return core.PackageSecretStore, nil
	case EffectHost:
		return core.PackageHostFacts, nil
	case EffectLocking:
		return core.PackageFileLock, nil
	case EffectSignal:
		return core.PackageShutdown, nil
	case EffectObjectStorage:
		return core.PackageObjectStore, nil
	default:
		return core.PackageUnknown, errors.Join(
			core.ErrCapabilitiesContract,
			errors.New("validated effect has no capability owner"),
		)
	}
}

var _ core.OffWireEnum = EffectUnknown
