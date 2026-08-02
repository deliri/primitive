package exchange_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	identityContentCoding = "identity"
	locationHeaderName    = "Location"
	retryAfterHeaderName  = "Retry-After"
)

func mustHTTPMediaType(testingContext testing.TB, value string) core.HTTPMediaType {
	testingContext.Helper()
	mediaType, err := core.ParseHTTPMediaType(value)
	if err != nil {
		testingContext.Fatalf("core.ParseHTTPMediaType(%q) error = %v, want nil", value, err)
	}
	return mediaType
}
