package googlecompute

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleComputeJSONMediaParameters(t *testing.T) {
	t.Parallel()
	for _, rpc := range []string{"read", "start"} {
		t.Run(rpc, func(t *testing.T) {
			t.Parallel()
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				if rpc == "read" {
					if err := json.NewEncoder(w).Encode(nativeInstance(t, "RUNNING")); err != nil {
						return
					}
					return
				}
				if err := json.NewEncoder(w).Encode(nativeOperation(t, "start")); err != nil {
					return
				}
			}))
			if rpc == "read" {
				got, err := client.ReadInstance(t.Context(), InstanceReadRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
				if err != nil || got.State != InstanceStateRunning {
					t.Fatalf("JSON read = (%+v,%v), want Running,nil", got, err)
				}
			} else {
				got, err := client.StartInstance(t.Context(), InstanceStartRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
				if err != nil || got.State != OperationStateDone {
					t.Fatalf("JSON start = (%+v,%v), want Done,nil", got, err)
				}
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
		})
	}
}

func TestGoogleComputeNullOperation(t *testing.T) {
	t.Parallel()
	client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("null")); err != nil {
			return
		}
	}))
	got, err := client.StartInstance(t.Context(), InstanceStartRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
	if !errors.Is(err, core.ErrGoogleComputeResponse) || got != (OperationObservation{}) || calls.Load() != 1 {
		t.Fatalf("null operation = (%+v,%v,%d calls), want zero,Response,1 call", got, err, calls.Load())
	}
}

func TestGoogleComputeInstanceBinding(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"name", "zone", "self-link"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			wire := nativeInstance(t, "RUNNING")
			switch field {
			case "name":
				wire.Name = "another-instance"
			case "zone":
				wire.Zone += "-foreign"
			case "self-link":
				wire.SelfLink += "-foreign"
			}
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(wire); err != nil {
					return
				}
			}))
			got, err := client.ReadInstance(t.Context(), InstanceReadRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
			if !errors.Is(err, core.ErrGoogleComputeBinding) || got != (InstanceObservation{}) || calls.Load() != 1 {
				t.Fatalf("foreign instance = (%+v,%v,%d calls), want zero,Binding,1 call", got, err, calls.Load())
			}
		})
	}
}
