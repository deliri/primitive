package filestore_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// This is the Go publication boundary, not browser execution proof. The native
// IndexedDB suite consumes these emitted bytes in an actual browser separately.
func TestBrowserCommittedMoveModuleCoreIdentityProjection(t *testing.T) {
	t.Parallel()
	got := filestore.BrowserCommittedMoveModule()
	if got == "" || strings.Contains(got, "__CONTRACT_IDENTITY__") || strings.Contains(got, "__SOURCE_IDENTITY__") || strings.Contains(got, "__DESTINATION_IDENTITY__") || strings.Contains(got, "__CLEANUP_IDENTITY__") || strings.Contains(got, "__CALLBACK_IDENTITY__") {
		t.Fatalf("module bytes=%d unresolved identity marker=%t, want emitted module without placeholders", len(got), strings.Contains(got, "_IDENTITY__"))
	}
	cases := []struct {
		name     string
		identity core.ErrorIdentity
	}{
		{name: "contract", identity: core.ErrFilestoreContract},
		{name: "source", identity: core.ErrFilestoreSource},
		{name: "destination", identity: core.ErrFilestoreDestination},
		{name: "cleanup", identity: core.ErrFilestoreCleanup},
		{name: "callback", identity: core.ErrFilestoreCallbackPanic},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := tc.name + ": " + strconv.Itoa(int(tc.identity)) + ","
			if strings.Count(got, want) != 1 {
				t.Fatalf("Core identity projection %q occurrences=%d, want one", want, strings.Count(got, want))
			}
		})
	}
	if second := filestore.BrowserCommittedMoveModule(); second != got {
		t.Fatalf("second emission differs, want immutable embedded bytes")
	}
}
