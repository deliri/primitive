package temporal

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

//go:embed browser_module.js
var temporalBrowserSource string

// BrowserModule emits the exact signed-nanosecond RFC3339 parser for browser
// callers. Native Date owns Gregorian interpretation; BigInt preserves the
// fraction and explicit offset without a millisecond-precision handoff.
func BrowserModule() string {
	return strings.ReplaceAll(temporalBrowserSource, "__TEMPORAL_CONTRACT_IDENTITY__", strconv.Itoa(int(core.ErrTemporalContract)))
}
