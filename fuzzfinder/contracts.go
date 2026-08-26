package fuzzfinder

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	// MaximumRetainedEntries is the product-neutral safety ceiling for one
	// bounded observation.
	MaximumRetainedEntries uint16 = 128

	generatedNameBytesGo1_27 = 16
)

func cacheFormatDiagnostics() [cacheFormatLimit]string {
	return [...]string{
		CacheFormatGo1_27: "go1.27",
	}
}

// CacheFormat identifies one exact Go-generated fuzz-artifact filename format.
type CacheFormat uint8

const (
	CacheFormatUnknown CacheFormat = iota
	// CacheFormatGo1_27 is the generated cache/crasher filename format written
	// by Go 1.27's internal fuzz engine.
	CacheFormatGo1_27
	cacheFormatLimit
)

// Validate rejects unsupported cache formats.
func (f CacheFormat) Validate() error {
	if !f.IsValid() {
		return formatError(errors.New("cache format is unsupported"))
	}
	return nil
}

// IsValid reports whether f is a supported cache format.
func (f CacheFormat) IsValid() bool {
	return f > CacheFormatUnknown && f < cacheFormatLimit && cacheFormatDiagnostics()[f] != ""
}

// OffWireEnum declares CacheFormat as execution policy rather than wire data.
func (CacheFormat) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for f.
func (f CacheFormat) String() string {
	if !f.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return cacheFormatDiagnostics()[f]
}

// GeneratedNameBytes returns the exact persisted filename width for kind under
// f. Go 1.27 writes both cache corpus and durable crashers through writeToCorpus.
func (f CacheFormat) GeneratedNameBytes(kind ArtifactKind) (core.ByteCount, error) {
	width, err := f.generatedNameBytes(kind)
	if err != nil {
		return core.ByteCount{}, err
	}
	value, err := core.NewByteCount(uint64(width))
	if err != nil {
		return core.ByteCount{}, contractError(err)
	}
	return value, nil
}

// generatedNameBytes is the single owner of each format and artifact kind's
// exact generated filename width.
func (f CacheFormat) generatedNameBytes(kind ArtifactKind) (uint8, error) {
	if err := kind.Validate(); err != nil {
		return 0, err
	}
	switch f {
	case CacheFormatGo1_27:
		return generatedNameBytesGo1_27, nil
	default:
		return 0, f.Validate()
	}
}

// RetentionLimit is the caller-selected bounded number of canonical names to
// retain from one observation.
type RetentionLimit struct {
	value uint16
}

// NewRetentionLimit constructs a nonzero limit at or below the shared ceiling.
func NewRetentionLimit(value uint16) (RetentionLimit, error) {
	limit := RetentionLimit{value: value}
	if err := limit.Validate(); err != nil {
		return RetentionLimit{}, err
	}
	return limit, nil
}

// Validate rejects zero or above-ceiling limits.
func (l RetentionLimit) Validate() error {
	if l.value == 0 || l.value > MaximumRetainedEntries {
		return contractError(errors.New("retention limit is outside the admitted range"))
	}
	return nil
}

// Uint16 returns the exact retained-name ceiling.
func (l RetentionLimit) Uint16() uint16 {
	return l.value
}

// FindRequest binds one rooted directory, one declared artifact class, the
// exact Go format, and a retention limit to one observation.
//
// Kind is declared rather than observed. Go 1.27 persists cache corpus and
// testdata crashers through the same filename projection, so the name alone
// cannot identify its class. Binding Kind into the request and retained name
// prevents corpus and crasher facts from being merged.
type FindRequest struct {
	Location  filestore.Location
	Retention RetentionLimit
	Kind      ArtifactKind
	Format    CacheFormat
}

// Validate rejects every unset or unsupported request boundary.
func (r FindRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if err := r.Format.Validate(); err != nil {
		return err
	}
	return r.Retention.Validate()
}

func contractError(cause error) error {
	return errors.Join(core.ErrFuzzFinderContract, cause)
}

func formatError(cause error) error {
	return errors.Join(core.ErrFuzzFinderFormat, cause)
}

func observationError(cause error) error {
	return errors.Join(core.ErrFuzzFinderObservation, cause)
}
