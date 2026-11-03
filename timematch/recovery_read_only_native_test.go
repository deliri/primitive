package timematch

import (
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Recover-only observes a possibly dispatched intent. Even proof of absence
// cannot authorize another create on behalf of a caller that chose read-only.
func TestRecoverOnlyNeverMutatesMissingOrUnavailableProvider(t *testing.T) {
	t.Parallel()
	for _, providerKind := range []Provider{ProviderGoogle, ProviderApple, ProviderMicrosoft} {
		for _, unavailable := range []bool{false, true} {
			name := providerKind.String() + "/missing"
			if unavailable {
				name = providerKind.String() + "/unavailable"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				connection := oauthConnection(t, providerKind)
				owned := intent(t, "primary", "Review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
				if providerKind == ProviderApple {
					connection = appleConnection(t)
					owned = appleMutationIntent(t)
				}
				var reads, writes atomic.Int32
				client, _ := newNative(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
					if r.Method != http.MethodGet {
						writes.Add(1)
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					reads.Add(1)
					if unavailable {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					switch providerKind {
					case ProviderGoogle:
						writeJSON(w, http.StatusOK, `{"items":[]}`)
					case ProviderMicrosoft:
						writeJSON(w, http.StatusOK, `{"value":[]}`)
					case ProviderApple:
						w.WriteHeader(http.StatusNotFound)
					default:
						t.Errorf("provider = %v, want an authored fixture provider", providerKind)
					}
				})
				request := eventWrite(t, connection, owned)
				request.Attempt = EventCreateRecoverOnly
				got, err := client.PutEvent(t.Context(), request)
				if writes.Load() != 0 || reads.Load() == 0 {
					t.Fatalf("recovery reads/writes = %d/%d, want a real read and zero mutations", reads.Load(), writes.Load())
				}
				if !errors.Is(err, core.ErrTimeMatchIndeterminate) || got != (EventReceipt{}) {
					t.Fatalf("recovery = (%v, %v), want zero receipt and Indeterminate", got, err)
				}
			})
		}
	}
}
