package github

import (
	"errors"
	"syscall"
	"testing"
)

// The consumer intentionally closes these HTTP fixture connections on refusal.
// A successful write or the two exact peer-close errors are the fixture contract.
func retainProviderWriteResult(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if !errors.Is(err, syscall.EPIPE) && !errors.Is(err, syscall.ECONNRESET) {
		t.Errorf("provider write error=%v, want nil or peer-closed pipe/connection", err)
		return
	}
	t.Logf("provider write observed consumer refusal: %v", err)
}
