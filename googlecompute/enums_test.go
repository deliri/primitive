package googlecompute

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestGoogleComputeInstanceStateJSON(t *testing.T) {
	t.Parallel()
	texts := [...]string{"", "PENDING", "PROVISIONING", "STAGING", "RUNNING", "STOPPING", "SUSPENDING", "SUSPENDED", "REPAIRING", "TERMINATED"}
	for value := range 256 {
		got := InstanceState(value)
		wantValid := value > 0 && value < len(texts)
		if got.IsValid() != wantValid {
			t.Fatalf("InstanceState(%d).IsValid = %t, want %t", value, got.IsValid(), wantValid)
		}
		encoded, err := got.MarshalJSON()
		if !wantValid {
			if !errors.Is(err, core.ErrGoogleComputeContract) || encoded != nil || got.String() != "" {
				t.Fatalf("invalid enum = (%q,%v), want nil,typed refusal", encoded, err)
			}
			continue
		}
		if err != nil || got.String() != texts[value] {
			t.Fatalf("enum wire = (%q,%v), want %q,nil", got.String(), err, texts[value])
		}
		var decoded InstanceState
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
			t.Fatalf("enum round trip = (%v,%v), want %v,nil", decoded, err, got)
		}
	}
	for _, data := range [][]byte{nil, []byte("null"), []byte("42"), []byte(`"FUTURE"`), []byte(`""`), []byte(`"RUNNING" {}`)} {
		got := InstanceStatePending
		if err := got.UnmarshalJSON(data); !errors.Is(err, core.ErrGoogleComputeContract) || got != InstanceStatePending {
			t.Fatalf("enum refusal = (%v,%v), want preserved,typed refusal", got, err)
		}
	}
	var absent *InstanceState
	if err := absent.UnmarshalJSON([]byte(`"FUTURE"`)); !errors.Is(err, core.ErrGoogleComputeContract) {
		t.Fatalf("nil receiver error = %v, want typed refusal", err)
	}
}

func TestGoogleComputeOperationStateJSON(t *testing.T) {
	t.Parallel()
	texts := [...]string{"", "PENDING", "RUNNING", "DONE"}
	for value := range 256 {
		got := OperationState(value)
		wantValid := value > 0 && value < len(texts)
		if got.IsValid() != wantValid {
			t.Fatalf("OperationState(%d).IsValid = %t, want %t", value, got.IsValid(), wantValid)
		}
		encoded, err := got.MarshalJSON()
		if !wantValid {
			if !errors.Is(err, core.ErrGoogleComputeContract) || encoded != nil || got.String() != "" {
				t.Fatalf("invalid enum = (%q,%v), want nil,typed refusal", encoded, err)
			}
			continue
		}
		if err != nil || got.String() != texts[value] {
			t.Fatalf("enum wire = (%q,%v), want %q,nil", got.String(), err, texts[value])
		}
		var decoded OperationState
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
			t.Fatalf("enum round trip = (%v,%v), want %v,nil", decoded, err, got)
		}
	}
	for _, data := range [][]byte{nil, []byte("null"), []byte("42"), []byte(`"FUTURE"`), []byte(`""`), []byte(`"RUNNING" {}`)} {
		got := OperationStatePending
		if err := got.UnmarshalJSON(data); !errors.Is(err, core.ErrGoogleComputeContract) || got != OperationStatePending {
			t.Fatalf("enum refusal = (%v,%v), want preserved,typed refusal", got, err)
		}
	}
	var absent *OperationState
	if err := absent.UnmarshalJSON([]byte(`"FUTURE"`)); !errors.Is(err, core.ErrGoogleComputeContract) {
		t.Fatalf("nil receiver error = %v, want typed refusal", err)
	}
}

func TestGoogleComputeOperationTypeJSON(t *testing.T) {
	t.Parallel()
	texts := [...]string{"", "start", "stop"}
	for value := range 256 {
		got := OperationType(value)
		wantValid := value > 0 && value < len(texts)
		if got.IsValid() != wantValid {
			t.Fatalf("OperationType(%d).IsValid = %t, want %t", value, got.IsValid(), wantValid)
		}
		encoded, err := got.MarshalJSON()
		if !wantValid {
			if !errors.Is(err, core.ErrGoogleComputeContract) || encoded != nil || got.String() != "" {
				t.Fatalf("invalid enum = (%q,%v), want nil,typed refusal", encoded, err)
			}
			continue
		}
		if err != nil || got.String() != texts[value] {
			t.Fatalf("enum wire = (%q,%v), want %q,nil", got.String(), err, texts[value])
		}
		var decoded OperationType
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
			t.Fatalf("enum round trip = (%v,%v), want %v,nil", decoded, err, got)
		}
	}
	for _, data := range [][]byte{nil, []byte("null"), []byte("42"), []byte(`"FUTURE"`), []byte(`""`), []byte(`"RUNNING" {}`)} {
		got := OperationTypeStart
		if err := got.UnmarshalJSON(data); !errors.Is(err, core.ErrGoogleComputeContract) || got != OperationTypeStart {
			t.Fatalf("enum refusal = (%v,%v), want preserved,typed refusal", got, err)
		}
	}
	var absent *OperationType
	if err := absent.UnmarshalJSON([]byte(`"FUTURE"`)); !errors.Is(err, core.ErrGoogleComputeContract) {
		t.Fatalf("nil receiver error = %v, want typed refusal", err)
	}
}

func FuzzGoogleComputeEnumJSONSemanticClosure(f *testing.F) {
	seed, err := InstanceStateRunning.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(0), seed)
	seed, err = OperationStateDone.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(1), seed)
	seed, err = OperationTypeStart.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(2), seed)
	f.Add(uint8(0), []byte("null"))
	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		switch selector % 3 {
		case 0:
			got := InstanceStatePending
			err := got.UnmarshalJSON(data)
			if err != nil {
				if !errors.Is(err, core.ErrGoogleComputeContract) || got != InstanceStatePending {
					t.Fatalf("rejected enum = (%v,%v), want preserved,typed refusal", got, err)
				}
				return
			}
			var text string
			if err := json.Unmarshal(data, &text); err != nil || got.String() != text || !got.IsValid() {
				t.Fatalf("accepted enum = %v from %q, want exact valid token", got, data)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("canonical enum error = %v, want nil", err)
			}
			var again InstanceState
			if err := again.UnmarshalJSON(encoded); err != nil || again != got {
				t.Fatalf("canonical enum round trip = (%v,%v), want %v,nil", again, err, got)
			}
			second, err := again.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, second) {
				t.Fatalf("second encoding = (%q,%v), want %q,nil", second, err, encoded)
			}
		case 1:
			got := OperationStatePending
			err := got.UnmarshalJSON(data)
			if err != nil {
				if !errors.Is(err, core.ErrGoogleComputeContract) || got != OperationStatePending {
					t.Fatalf("rejected enum = (%v,%v), want preserved,typed refusal", got, err)
				}
				return
			}
			var text string
			if err := json.Unmarshal(data, &text); err != nil || got.String() != text || !got.IsValid() {
				t.Fatalf("accepted enum = %v from %q, want exact valid token", got, data)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("canonical enum error = %v, want nil", err)
			}
			var again OperationState
			if err := again.UnmarshalJSON(encoded); err != nil || again != got {
				t.Fatalf("canonical enum round trip = (%v,%v), want %v,nil", again, err, got)
			}
			second, err := again.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, second) {
				t.Fatalf("second encoding = (%q,%v), want %q,nil", second, err, encoded)
			}
		case 2:
			got := OperationTypeStart
			err := got.UnmarshalJSON(data)
			if err != nil {
				if !errors.Is(err, core.ErrGoogleComputeContract) || got != OperationTypeStart {
					t.Fatalf("rejected enum = (%v,%v), want preserved,typed refusal", got, err)
				}
				return
			}
			var text string
			if err := json.Unmarshal(data, &text); err != nil || got.String() != text || !got.IsValid() {
				t.Fatalf("accepted enum = %v from %q, want exact valid token", got, data)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("canonical enum error = %v, want nil", err)
			}
			var again OperationType
			if err := again.UnmarshalJSON(encoded); err != nil || again != got {
				t.Fatalf("canonical enum round trip = (%v,%v), want %v,nil", again, err, got)
			}
			second, err := again.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, second) {
				t.Fatalf("second encoding = (%q,%v), want %q,nil", second, err, encoded)
			}

		}
	})
}
