package googlecompute

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/compute/v1"
)

// Every public provider response door uses the same official JSON admission.
// The selector reaches each actual RPC; accepted facts must conserve the
// provider's exact state and requested resource/type/name binding.
func FuzzGoogleComputeProviderSemanticClosure(f *testing.F) {
	instance, err := json.Marshal(nativeInstance(f, "RUNNING"))
	if err != nil {
		f.Fatal(err)
	}
	start, err := json.Marshal(nativeOperation(f, "start"))
	if err != nil {
		f.Fatal(err)
	}
	stop, err := json.Marshal(nativeOperation(f, "stop"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(0), instance)
	f.Add(uint8(1), start)
	f.Add(uint8(2), stop)
	f.Add(uint8(3), start)
	f.Add(uint8(3), []byte("null"))
	f.Add(uint8(0), []byte("{"))
	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(data); err != nil {
				return
			}
		}))
		reference := nativeReference(t)
		if selector%4 == 0 {
			got, err := client.ReadInstance(t.Context(), InstanceReadRequest{Instance: reference, Token: token, Policy: nativePolicy(t)})
			if err != nil {
				if !errors.Is(err, core.ErrGoogleComputeContract) || got != (InstanceObservation{}) {
					t.Fatalf("rejected instance = (%+v,%v), want zero,typed contract", got, err)
				}
				return
			}
			var wire compute.Instance
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatalf("accepted instance JSON error = %v, want nil", err)
			}
			states := []string{"", "PENDING", "PROVISIONING", "STAGING", "RUNNING", "STOPPING", "SUSPENDING", "SUSPENDED", "REPAIRING", "TERMINATED"}
			if got.Validate() != nil || int(got.State) >= len(states) || states[got.State] != wire.Status || wire.Name != "anvil" || !fuzzInstanceLink(wire.SelfLink) || !fuzzZoneLink(wire.Zone) {
				t.Fatalf("accepted instance = %+v from %+v, want exact bound facts", got, wire)
			}
		} else {
			var got OperationObservation
			var err error
			wantType := OperationTypeStart
			switch selector % 4 {
			case 1:
				got, err = client.StartInstance(t.Context(), InstanceStartRequest{Instance: reference, Token: token, Policy: nativePolicy(t)})
			case 2:
				wantType = OperationTypeStop
				got, err = client.StopInstance(t.Context(), InstanceStopRequest{Instance: reference, Token: token, Policy: nativePolicy(t)})
			case 3:
				name, nameErr := NewOperationName("operation-123")
				if nameErr != nil {
					t.Fatal(nameErr)
				}
				got, err = client.ReadOperation(t.Context(), OperationReadRequest{Instance: reference, Operation: name, Type: wantType, Token: token, Policy: nativePolicy(t)})
			}
			if err != nil {
				if !errors.Is(err, core.ErrGoogleComputeContract) || got != (OperationObservation{}) {
					t.Fatalf("rejected operation = (%+v,%v), want zero,typed contract", got, err)
				}
				return
			}
			var wire compute.Operation
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatalf("accepted operation JSON error = %v, want nil", err)
			}
			states := []string{"", "PENDING", "RUNNING", "DONE"}
			types := []string{"", "start", "stop"}
			wantFailed := wire.Error != nil && len(wire.Error.Errors) > 0
			if got.Validate() != nil || got.Type != wantType || int(got.State) >= len(states) || states[got.State] != wire.Status || types[got.Type] != wire.OperationType || got.Name.String() != wire.Name || got.TargetLink.String() != wire.TargetLink || got.Zone.String() != wire.Zone || !fuzzInstanceLink(wire.TargetLink) || !fuzzZoneLink(wire.Zone) || got.Failed != wantFailed || selector%4 == 3 && wire.Name != "operation-123" {
				t.Fatalf("accepted operation = %+v from %+v, want exact bound facts", got, wire)
			}
		}
		if got := calls.Load(); got != 1 {
			t.Fatalf("provider calls = %d, want 1", got)
		}
	})
}

func fuzzInstanceLink(text string) bool {
	return slices.Contains([]string{
		"https://compute.googleapis.com/compute/v1/projects/fixture-project/zones/northamerica-northeast2-a/instances/anvil",
		"https://www.googleapis.com/compute/v1/projects/fixture-project/zones/northamerica-northeast2-a/instances/anvil",
	}, text)
}
func fuzzZoneLink(text string) bool {
	return slices.Contains([]string{
		"https://compute.googleapis.com/compute/v1/projects/fixture-project/zones/northamerica-northeast2-a",
		"https://www.googleapis.com/compute/v1/projects/fixture-project/zones/northamerica-northeast2-a",
	}, text)
}

func FuzzGoogleComputeResourceSegments(f *testing.F) {
	r, err := NewResourceName("fixture-project")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(r.String())
	f.Add("a/b")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		r, rErr := NewResourceName(text)
		o, oErr := NewOperationName(text)
		if rErr != nil || oErr != nil {
			if !errors.Is(rErr, core.ErrGoogleComputeContract) || !errors.Is(oErr, core.ErrGoogleComputeContract) || r != (ResourceName{}) || o != (OperationName{}) {
				t.Fatalf("refused segment = (%v,%v), want typed zero", rErr, oErr)
			}
			return
		}
		if r.Validate() != nil || o.Validate() != nil || r.String() != text || o.String() != text {
			t.Fatalf("admitted segment = (%q,%q), want exact %q", r.String(), o.String(), text)
		}
		for _, c := range text {
			if !slices.Contains([]rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"), c) {
				t.Fatalf("admitted path rune = %q, want resource alphabet", c)
			}
		}
	})
}
