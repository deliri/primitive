package temporal

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"errors"
	"io"
	"time"
)

// zoneInformation is Go 1.27.2's lib/time/zoneinfo.zip, retained as authored
// runtime data. Exact archive member names and TZif bytes are the authority;
// host file names, ZONEINFO and case-insensitive file systems cannot change it.
// Sources: https://go.dev/src/time/zoneinfo_read.go
// https://go.dev/src/time/tzdata/tzdata.go
//
//go:embed tzdata/zoneinfo.zip
var zoneInformation []byte

// zoneArchive is the member directory of that authored distribution, decoded
// once at package initialization. It is immutable, derived from no observed
// input, and shares no native *time.Location: every NewLocation still decodes
// its own TZif member, so callers own their capability outright. Decoding the
// 598-member directory on every call previously cost more than the zone itself.
var zoneArchive, zoneArchiveErr = zip.NewReader(bytes.NewReader(zoneInformation), int64(len(zoneInformation)))

// Location is an immutable caller-owned native location capability. Construct
// it once and reuse it for formatting and calendar work; no process-wide cache
// of constructed capabilities or observed-input collection is retained.
type Location struct {
	name  LocationName
	value *time.Location
}

// NewLocation admits an exact name from the embedded Go timezone distribution.
// UTC is explicit. Aliases shipped by Go remain valid exact IANA names.
func NewLocation(name LocationName) (Location, error) {
	if name == LocationUTC {
		return Location{name: name, value: time.UTC}, nil
	}
	if name == "" || name == "Local" {
		return Location{}, contractError("time location must be explicit")
	}
	if zoneArchiveErr != nil {
		return Location{}, contractError("embedded time zone archive is invalid", zoneArchiveErr)
	}
	for _, member := range zoneArchive.File {
		if member.Name != string(name) {
			continue
		}
		source, openErr := member.Open()
		if openErr != nil {
			return Location{}, contractError("embedded time zone cannot be opened", openErr)
		}
		// A single native TZif document is a caller-owned whole value. This is
		// immutable authored data, never an input stream or growing zone cache.
		data, readErr := io.ReadAll(source)
		closeErr := source.Close()
		if readErr != nil || closeErr != nil {
			return Location{}, contractError("embedded time zone cannot be read", errors.Join(readErr, closeErr))
		}
		value, loadErr := time.LoadLocationFromTZData(string(name), data)
		if loadErr != nil {
			return Location{}, contractError("embedded time zone is invalid", loadErr)
		}
		return Location{name: name, value: value}, nil
	}
	return Location{}, contractError("time location name is not an exact embedded member")
}

// Validate rejects an unset native capability without re-reading timezone data.
func (l Location) Validate() error {
	if l.value == nil || l.name == "" {
		return contractError("time location is unset")
	}
	return nil
}

// Name returns the admitted name; zero remains invalid.
func (l Location) Name() LocationName { return l.name }

// Format uses the already-loaded native timezone capability.
func (l Location) Format(instant Instant, layout TimeLayout) (string, error) {
	value, err := l.time(instant, layout)
	if err != nil {
		return "", err
	}
	return value.Format(string(layout)), nil
}

// Append preserves caller storage on refusal and reuses native timezone data.
func (l Location) Append(destination []byte, instant Instant, layout TimeLayout) ([]byte, error) {
	value, err := l.time(instant, layout)
	if err != nil {
		return destination, err
	}
	return value.AppendFormat(destination, string(layout)), nil
}

func (l Location) time(instant Instant, layout TimeLayout) (time.Time, error) {
	if err := l.Validate(); err != nil {
		return time.Time{}, err
	}
	if err := layout.Validate(); err != nil {
		return time.Time{}, err
	}
	value, err := instant.Time()
	if err != nil {
		return time.Time{}, err
	}
	return value.In(l.value), nil
}
