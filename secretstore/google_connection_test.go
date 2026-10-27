package secretstore

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc/connectivity"
)

type nilGoogleContext struct{ context.Context }

func TestGoogleConnectionConstructorRefusalPreservesCallerOwnership(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		context           func(*testing.T) context.Context
		missingConnection bool
		want              error
	}{
		{name: "missing connection refuses before SDK setup", context: func(t *testing.T) context.Context { return t.Context() }, missingConnection: true, want: core.ErrSecretStoreContract},
		{name: "nil context retains native connection", context: func(*testing.T) context.Context { return nil }, want: core.ErrNilContext},
		{name: "typed nil context retains native connection", context: func(*testing.T) context.Context { var ctx *nilGoogleContext; return ctx }, want: core.ErrContextObservation},
		{name: "cancelled context retains native connection", context: func(t *testing.T) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, want: context.Canceled},
		{name: "expired context retains native connection", context: func(t *testing.T) context.Context {
			ctx, cancel := context.WithDeadline(t.Context(), time.Unix(1, 0))
			t.Cleanup(cancel)
			return ctx
		}, want: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			connection := localGoogleConnection(t, func(context.Context, *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
				calls.Add(1)
				return nil, core.ErrSecretStoreAccess
			})
			defer func() {
				if err := connection.Close(); err != nil {
					t.Error(err)
				}
			}()
			request := GoogleConnectionRequest{Connection: connection}
			if tc.missingConnection {
				request.Connection = nil
			}
			got, err := NewGoogleReaderWithConnection(tc.context(t), request)
			if got != nil || !errors.Is(err, core.ErrSecretStoreContract) || !errors.Is(err, tc.want) || connection.GetState() == connectivity.Shutdown || calls.Load() != 0 {
				t.Fatalf("constructor refusal = (reader %v, error %v, connection %v, calls %d), want nil, contract/%v, caller-owned, 0", got, err, connection.GetState(), calls.Load(), tc.want)
			}
		})
	}
}

func TestGoogleConnectionConstructorTransfersNativeCloseOwnership(t *testing.T) {
	t.Parallel()
	request := accessRequestForTest(t)
	var calls atomic.Int32
	connection := localGoogleConnection(t, func(_ context.Context, got *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
		calls.Add(1)
		want, err := request.resourceName()
		if err != nil || got.Name != want {
			t.Errorf("provider name = (%q,%v), want %q,nil", got.Name, err, want)
		}
		return officialChecksummedResponse(request, resolvedVersionForTest, []byte("synthetic-secret")), nil
	})
	reader, err := NewGoogleReaderWithConnection(t.Context(), GoogleConnectionRequest{Connection: connection})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if reader.Validate() == nil {
			if err := reader.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	result, err := reader.Access(t.Context(), request)
	if err != nil || result.Validate() != nil || calls.Load() != 1 {
		t.Fatalf("public SDK access = (%v,%v,%d calls), want exact valid result,nil,1", result.Reference, err, calls.Load())
	}
	if err := result.Value.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil || connection.GetState() != connectivity.Shutdown {
		t.Fatalf("reader.Close = (%v,connection %v), want nil,Shutdown", err, connection.GetState())
	}
	if err := reader.Close(); !errors.Is(err, core.ErrSecretStoreContract) {
		t.Fatalf("second reader.Close = %v, want contract refusal", err)
	}
	got, err := reader.Access(t.Context(), request)
	if got != (AccessResult{}) || !errors.Is(err, core.ErrSecretStoreContract) || calls.Load() != 1 {
		t.Fatalf("closed reader access = (%v,%v,%d calls), want zero,contract,1", got, err, calls.Load())
	}
}
