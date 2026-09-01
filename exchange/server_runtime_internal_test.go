package exchange

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestServerRuntimeReadinessSlotReplacesUnconsumedAttempt(t *testing.T) {
	t.Parallel()

	runtime := &ServerRuntime{ready: make(chan error, 1)}
	runtime.publishReady(core.ErrExchangeTransport)
	runtime.publishReady(nil)
	select {
	case got := <-runtime.ready:
		if got != nil {
			t.Fatalf("latest readiness result = %v, want nil successful retry", got)
		}
	default:
		t.Fatalf("readiness result count = %d, want 1 successful retry result", 0)
	}
	select {
	case got := <-runtime.ready:
		t.Fatalf("extra readiness result = %v, want exactly one latest attempt", got)
	default:
	}
}
