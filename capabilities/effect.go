package capabilities

import "github.com/deliri/primitive/v2026/core"

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
		EffectProcess:       core.PackageProcess.String(),
		EffectTransport:     "transport",
		EffectTime:          timeContractText,
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
	owner := effectOwners()[effect]
	if owner == core.PackageUnknown {
		return core.PackageUnknown, contractError("validated effect has no capability owner")
	}
	return owner, nil
}

func effectOwners() [effectLimit]core.PackageIdentity {
	return [...]core.PackageIdentity{
		EffectFilesystem:    core.PackageFilestore,
		EffectProcess:       core.PackageProcess,
		EffectTransport:     core.PackageExchange,
		EffectTime:          core.PackageTemporal,
		EffectEntropy:       core.PackageKeygen,
		EffectSecret:        core.PackageSecretStore,
		EffectHost:          core.PackageHostFacts,
		EffectLocking:       core.PackageFileLock,
		EffectSignal:        core.PackageShutdown,
		EffectObjectStorage: core.PackageObjectStore,
	}
}

var _ core.OffWireEnum = EffectUnknown
