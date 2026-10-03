package hostfacts

import (
	"strings"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// TimeZoneRequest names an explicit database entry, without inferring a zone.
type TimeZoneRequest struct{ Name string }

func (r TimeZoneRequest) Validate() error {
	if r.Name == "" || r.Name == "Local" || len(r.Name) > core.TimeZoneNameMaximumBytes || strings.Contains(r.Name, "..") || strings.HasPrefix(r.Name, "/") || strings.HasSuffix(r.Name, "/") || strings.Contains(r.Name, "//") {
		return fail(OperationTimeZone, core.ErrHostFactsContract, nil)
	}
	if strings.Trim(r.Name, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789/_-+") != "" {
		return fail(OperationTimeZone, core.ErrHostFactsContract, nil)
	}
	return nil
}

// ObserveTimeZone performs one standard-library timezone database lookup.
// It returns Go's real immutable Location, without a copied zone database,
// product cache, timezone guessing, wall-clock read or calendar state machine.
// Lookup failure returns nil with the strongest stable Hostfacts identity.
func ObserveTimeZone(request TimeZoneRequest) (*time.Location, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(request.Name)
	if err != nil {
		return nil, fail(OperationTimeZone, core.ErrHostFactsObservation, err)
	}
	return location, nil
}
