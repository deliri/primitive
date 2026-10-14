// Package temporal provides exact typed nanosecond values and thin,
// validated effects over Go's real context and time primitives. Civil dates
// use the four-digit Gregorian ISO profile, including year 0000. Reusable
// Location capabilities execute timezone work from embedded Go TZif data.
// TimeLayout requires at least one calendar coordinate; clock-only display uses
// the closed ClockLayout domain. Native DST folds follow Go's choice; gaps are
// refused.
package temporal
