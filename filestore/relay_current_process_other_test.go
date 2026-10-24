//go:build !unix

package filestore

import "testing"

func RelayCurrentProcessNativeProofForTest(t *testing.T) {
	t.Helper()
	t.Run("refusal", TestRelayCurrentProcessRefusesBeforeStarting)
}
