package googlecompute

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGoogleComputeConstructorContext(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	network, err := exchange.NewClient(server.Client())
	if err != nil {
		t.Fatalf("network admission = %v, want nil", err)
	}
	client, err := NewClient(t.Context(), network)
	if err != nil || client.Validate() != nil {
		t.Fatalf("active constructor = (%v,%v), want admitted client", client.Validate(), err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(core.ErrGoogleComputeBinding)
	got, err := NewClient(ctx, network)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrGoogleComputeBinding) || got != (Client{}) || calls.Load() != 0 {
		t.Fatalf("cancelled constructor = (%+v,%v,%d calls), want zero,exact cause,0 calls", got, err, calls.Load())
	}
}

func TestGoogleComputeClosedInstanceState(t *testing.T) {
	t.Parallel()
	var _ interface{ OffWireEnum() } = InstanceState(0)
	texts := [...]string{"", "PENDING", "PROVISIONING", "STAGING", "RUNNING", "STOPPING", "SUSPENDING", "SUSPENDED", "REPAIRING", "TERMINATED"}
	for value := range 256 {
		got := InstanceState(value)
		wantValid := value > 0 && value < len(texts)
		if got.IsValid() != wantValid {
			t.Fatalf("InstanceState(%d).IsValid = %t, want %t", value, got.IsValid(), wantValid)
		}
		if !wantValid {
			if !errors.Is(got.Validate(), core.ErrGoogleComputeContract) || got.String() != "" {
				t.Fatalf("invalid enum = (%q,%v), want empty,typed refusal", got.String(), got.Validate())
			}
			continue
		}
		if got.Validate() != nil || got.String() != texts[value] {
			t.Fatalf("enum fact = (%q,%v), want %q,nil", got.String(), got.Validate(), texts[value])
		}
	}
}

func TestGoogleComputeClosedOperationState(t *testing.T) {
	t.Parallel()
	var _ interface{ OffWireEnum() } = OperationState(0)
	texts := [...]string{"", "PENDING", "RUNNING", "DONE"}
	for value := range 256 {
		got := OperationState(value)
		wantValid := value > 0 && value < len(texts)
		if got.IsValid() != wantValid {
			t.Fatalf("OperationState(%d).IsValid = %t, want %t", value, got.IsValid(), wantValid)
		}
		if !wantValid {
			if !errors.Is(got.Validate(), core.ErrGoogleComputeContract) || got.String() != "" {
				t.Fatalf("invalid enum = (%q,%v), want empty,typed refusal", got.String(), got.Validate())
			}
			continue
		}
		if got.Validate() != nil || got.String() != texts[value] {
			t.Fatalf("enum fact = (%q,%v), want %q,nil", got.String(), got.Validate(), texts[value])
		}
	}
}

func TestGoogleComputeClosedOperationType(t *testing.T) {
	t.Parallel()
	var _ interface{ OffWireEnum() } = OperationType(0)
	texts := [...]string{"", "start", "stop"}
	for value := range 256 {
		got := OperationType(value)
		wantValid := value > 0 && value < len(texts)
		if got.IsValid() != wantValid {
			t.Fatalf("OperationType(%d).IsValid = %t, want %t", value, got.IsValid(), wantValid)
		}
		if !wantValid {
			if !errors.Is(got.Validate(), core.ErrGoogleComputeContract) || got.String() != "" {
				t.Fatalf("invalid enum = (%q,%v), want empty,typed refusal", got.String(), got.Validate())
			}
			continue
		}
		if got.Validate() != nil || got.String() != texts[value] {
			t.Fatalf("enum fact = (%q,%v), want %q,nil", got.String(), got.Validate(), texts[value])
		}
	}
}
