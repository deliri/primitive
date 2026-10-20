package gotoolchain

import (
	"bytes"

	"github.com/deliri/primitive/v2026/core"
)

type VersionBannerPresence uint8

const (
	VersionBannerPresenceUnknown VersionBannerPresence = iota
	VersionBannerAbsent
	VersionBannerPresent
)

func (p VersionBannerPresence) Validate() error {
	if p != VersionBannerAbsent && p != VersionBannerPresent {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (VersionBannerPresence) OffWireEnum() {}

// VersionBannerRequest borrows native go-version output, including diagnostics.
// Every byte representation is meaningful input; empty output has no banner.
type VersionBannerRequest struct{ Output []byte }

func (VersionBannerRequest) goToolchainProtocolFact() {}

type VersionBannerObservation struct {
	Version  ToolchainVersion
	Presence VersionBannerPresence
}

func (o VersionBannerObservation) Validate() error {
	if err := o.Presence.Validate(); err != nil {
		return err
	}
	if o.Presence == VersionBannerAbsent {
		if o.Version != (ToolchainVersion{}) {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	return o.Version.Validate()
}
func (VersionBannerObservation) goToolchainProtocolFact() {}

// ObserveVersionBanner uses native byte-line/field iterators and Go's version
// grammar. Only the first admitted version token is copied. Ignored fields,
// records and byte extents never become a retained output population.
func ObserveVersionBanner(request VersionBannerRequest) VersionBannerObservation {
	for line := range bytes.Lines(request.Output) {
		var fields [3][]byte
		used := 0
		for field := range bytes.FieldsSeq(line) {
			fields[used] = field
			used++
			if used == len(fields) {
				break
			}
		}
		if used != len(fields) || !bytes.Equal(fields[0], []byte("go")) || !bytes.Equal(fields[1], []byte("version")) {
			continue
		}
		version, err := ParseToolchainVersion(string(fields[2]))
		if err == nil {
			return VersionBannerObservation{Version: version, Presence: VersionBannerPresent}
		}
	}
	return VersionBannerObservation{Presence: VersionBannerAbsent}
}

var (
	_ core.OffWireEnum = VersionBannerPresenceUnknown
	_ protocolFact     = VersionBannerRequest{}
	_ protocolFact     = VersionBannerObservation{}
)
