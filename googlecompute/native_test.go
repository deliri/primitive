package googlecompute

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func TestGoogleComputeNativeRPCs(t *testing.T) {
	t.Parallel()
	for _, rpc := range []string{"read", "start", "stop", "operation"} {
		t.Run(rpc, func(t *testing.T) {
			t.Parallel()
			r := nativeReference(t)
			wantPath := instancePath(r)
			wantMethod := http.MethodGet
			if rpc == "start" || rpc == "stop" {
				wantPath += "/" + rpc
				wantMethod = http.MethodPost
			}
			if rpc == "operation" {
				wantPath = zonePath(r) + "/operations/operation-123"
			}
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != wantMethod || request.URL.Path != wantPath || request.ContentLength > 0 {
					t.Errorf("request = (%s,%s,%d), want (%s,%s,no body)", request.Method, request.URL.Path, request.ContentLength, wantMethod, wantPath)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if rpc == "read" {
					if err := json.NewEncoder(w).Encode(nativeInstance(t, "RUNNING")); err != nil {
						return
					}
					return
				}
				typeName := rpc
				if rpc == "operation" {
					typeName = "start"
				}
				if err := json.NewEncoder(w).Encode(nativeOperation(t, typeName)); err != nil {
					return
				}
			}))
			if rpc == "read" {
				got, err := client.ReadInstance(t.Context(), InstanceReadRequest{Instance: r, Token: token, Policy: nativePolicy(t)})
				if err != nil || got.State != InstanceStateRunning || got.Validate() != nil {
					t.Fatalf("ReadInstance = (%+v,%v), want valid Running", got, err)
				}
			} else {
				var got OperationObservation
				var err error
				switch rpc {
				case "start":
					got, err = client.StartInstance(t.Context(), InstanceStartRequest{Instance: r, Token: token, Policy: nativePolicy(t)})
				case "stop":
					got, err = client.StopInstance(t.Context(), InstanceStopRequest{Instance: r, Token: token, Policy: nativePolicy(t)})
				case "operation":
					name, nameErr := NewOperationName("operation-123")
					if nameErr != nil {
						t.Fatal(nameErr)
					}
					got, err = client.ReadOperation(t.Context(), OperationReadRequest{Instance: r, Operation: name, Type: OperationTypeStart, Token: token, Policy: nativePolicy(t)})
				}
				wantType := OperationTypeStart
				if rpc == "stop" {
					wantType = OperationTypeStop
				}
				if err != nil || got.State != OperationStateDone || got.Type != wantType || got.Failed || got.Name.String() != "operation-123" || got.Validate() != nil {
					t.Fatalf("%s = (%+v,%v), want exact valid successful Done operation type %v", rpc, got, err, wantType)
				}
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
		})
	}
}

func TestGoogleComputeOperationResponseBoundaries(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name       string
		mutate     func(*compute.Operation)
		status     int
		wantErr    error
		wantState  OperationState
		wantFailed bool
	}{
		{name: "pending remains pending", mutate: func(w *compute.Operation) { w.Status = "PENDING" }, wantState: OperationStatePending},
		{name: "running remains running", mutate: func(w *compute.Operation) { w.Status = "RUNNING" }, wantState: OperationStateRunning},
		{name: "done remains done", wantState: OperationStateDone},
		{name: "provider failure remains a fact", mutate: func(w *compute.Operation) {
			w.Error = &compute.OperationError{Errors: []*compute.OperationErrorErrors{{Code: "RESOURCE_NOT_READY"}}}
		}, wantState: OperationStateDone, wantFailed: true},
		{name: "legacy documented Google link host", mutate: func(w *compute.Operation) {
			w.TargetLink = strings.Replace(w.TargetLink, "compute.googleapis.com", "www.googleapis.com", 1)
			w.Zone = strings.Replace(w.Zone, "compute.googleapis.com", "www.googleapis.com", 1)
		}, wantState: OperationStateDone},
		{name: "foreign operation name", mutate: func(w *compute.Operation) { w.Name = "operation-foreign" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "foreign instance", mutate: func(w *compute.Operation) { w.TargetLink += "-foreign" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "foreign zone", mutate: func(w *compute.Operation) { w.Zone += "-foreign" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "foreign project", mutate: func(w *compute.Operation) {
			w.TargetLink = strings.Replace(w.TargetLink, "fixture-project", "another-project", 1)
		}, wantErr: core.ErrGoogleComputeBinding},
		{name: "opposite operation type", mutate: func(w *compute.Operation) { w.OperationType = "stop" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "unknown operation type", mutate: func(w *compute.Operation) { w.OperationType = "reset" }, wantErr: core.ErrGoogleComputeResponse},
		{name: "empty operation name", mutate: func(w *compute.Operation) { w.Name = "" }, wantErr: core.ErrGoogleComputeResponse},
		{name: "unknown operation status", mutate: func(w *compute.Operation) { w.Status = "FUTURE" }, wantErr: core.ErrGoogleComputeResponse},
		{name: "empty operation status", mutate: func(w *compute.Operation) { w.Status = "" }, wantErr: core.ErrGoogleComputeResponse},
		{name: "null provider error entry", mutate: func(w *compute.Operation) {
			w.Error = &compute.OperationError{Errors: []*compute.OperationErrorErrors{nil}}
		}, wantErr: core.ErrGoogleComputeResponse},
		{name: "unclassified provider error", mutate: func(w *compute.Operation) {
			w.Error = &compute.OperationError{Errors: []*compute.OperationErrorErrors{{}}}
		}, wantErr: core.ErrGoogleComputeResponse},
		{name: "foreign target host", mutate: func(w *compute.Operation) {
			w.TargetLink = strings.Replace(w.TargetLink, "compute.googleapis.com", "example.com", 1)
		}, wantErr: core.ErrGoogleComputeBinding},
		{name: "target userinfo", mutate: func(w *compute.Operation) {
			w.TargetLink = strings.Replace(w.TargetLink, "https://", "https://owner@", 1)
		}, wantErr: core.ErrGoogleComputeBinding},
		{name: "target query", mutate: func(w *compute.Operation) { w.TargetLink += "?other=1" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "target empty query", mutate: func(w *compute.Operation) { w.TargetLink += "?" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "target fragment", mutate: func(w *compute.Operation) { w.TargetLink += "#other" }, wantErr: core.ErrGoogleComputeBinding},
		{name: "target escaped separator", mutate: func(w *compute.Operation) {
			w.TargetLink = strings.Replace(w.TargetLink, "/instances/", "/instances%2f", 1)
		}, wantErr: core.ErrGoogleComputeBinding},
		{name: "cleartext target", mutate: func(w *compute.Operation) { w.TargetLink = strings.Replace(w.TargetLink, "https://", "http://", 1) }, wantErr: core.ErrGoogleComputeBinding},
		{name: "provider unavailable does not retry", status: http.StatusServiceUnavailable, wantErr: core.ErrGoogleComputeResponse},
		{name: "unexpected successful HTTP status", status: http.StatusCreated, wantErr: core.ErrGoogleComputeResponse},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			wire := nativeOperation(t, "start")
			if row.mutate != nil {
				row.mutate(wire)
			}
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if row.status != 0 {
					w.WriteHeader(row.status)
				}
				if err := json.NewEncoder(w).Encode(wire); err != nil {
					return
				}
			}))
			name, err := NewOperationName("operation-123")
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := client.ReadOperation(t.Context(), OperationReadRequest{Instance: nativeReference(t), Operation: name, Type: OperationTypeStart, Token: token, Policy: nativePolicy(t)})
			if row.wantErr != nil {
				if !errors.Is(gotErr, row.wantErr) || got != (OperationObservation{}) {
					t.Fatalf("ReadOperation = (%+v,%v), want zero,%v", got, gotErr, row.wantErr)
				}
				if row.status >= 400 {
					var native *googleapi.Error
					if !errors.As(gotErr, &native) || native.Code != row.status {
						t.Fatalf("provider cause = %v, want Google status %d", gotErr, row.status)
					}
				}
			} else if gotErr != nil || got.State != row.wantState || got.Failed != row.wantFailed || got.Validate() != nil {
				t.Fatalf("ReadOperation = (%+v,%v), want valid state %v failed %t", got, gotErr, row.wantState, row.wantFailed)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("provider calls = %d, want 1", got)
			}
		})
	}
}

func TestGoogleComputeNativeCancellation(t *testing.T) {
	t.Parallel()
	arrived := make(chan struct{})
	client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(arrived); <-r.Context().Done() }))
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(context.Canceled)
	done := make(chan error, 1)
	go func() {
		got, err := client.ReadInstance(ctx, InstanceReadRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)})
		if got != (InstanceObservation{}) {
			done <- core.ErrGoogleComputeBinding
			return
		}
		done <- err
	}()
	select {
	case <-arrived:
	case <-time.After(10 * time.Second):
		cancel(context.DeadlineExceeded)
		t.Fatalf("provider arrival = timed out with %d requests, want one observed arrival", calls.Load())
	}
	cause := core.ErrGoogleComputeBinding
	cancel(cause)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) {
			t.Fatalf("in-flight cancellation = %v, want Canceled and %v", err, cause)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("cancelled call joined = false with %d requests, want joined completion", calls.Load())
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestGoogleComputeAdmissionHasNoEffects(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"client", "instance", "token", "retry", "redirect", "timeout", "cancelled"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			client, token, calls := nativeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
			r := InstanceReadRequest{Instance: nativeReference(t), Token: token, Policy: nativePolicy(t)}
			ctx := t.Context()
			switch field {
			case "client":
				client = Client{}
			case "instance":
				r.Instance = InstanceReference{}
			case "token":
				r.Token = googleidentity.AccessToken{}
			case "retry":
				r.Policy.Retry.MaximumAttempts = 2
			case "redirect":
				r.Policy.Redirect.Mode = 0
			case "timeout":
				r.Policy.AttemptTimeout = temporal.Duration{}
			case "cancelled":
				var cancel context.CancelCauseFunc
				ctx, cancel = context.WithCancelCause(ctx)
				cancel(core.ErrGoogleComputeBinding)
			}
			got, err := client.ReadInstance(ctx, r)
			if !errors.Is(err, core.ErrGoogleComputeContract) || got != (InstanceObservation{}) || calls.Load() != 0 {
				t.Fatalf("admission = (%+v,%v,%d calls), want zero,Contract,0 calls", got, err, calls.Load())
			}
			if field == "cancelled" && (!errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrGoogleComputeBinding)) {
				t.Fatalf("pre-cancel cause = %v, want Canceled and Conflict", err)
			}
		})
	}
}
