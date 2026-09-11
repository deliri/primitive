package secretstore

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protowire"
)

type localSecretServer struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer
	access func(context.Context, *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)
}

func (s *localSecretServer) AccessSecretVersion(ctx context.Context, request *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	return s.access(ctx, request)
}

// The same SDK constructor path receives a standard in-memory gRPC transport.
// Disabling SDK retries isolates one response/receipt transition per test.
func localGoogleReader(t *testing.T, access func(context.Context, *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)) *GoogleReader {
	t.Helper()
	listener := bufconn.Listen(64 * 1024)
	server := grpc.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(server, &localSecretServer{access: access})
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := listener.Close(); err != nil {
			t.Errorf("listener.Close() = %v, want nil", err)
		}
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				t.Errorf("server.Serve() = %v, want stopped", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("server join = timed out, want owned server exit")
		}
	})
	reader, err := newGoogleReader(t.Context(), option.WithoutAuthentication(), option.WithEndpoint("local.invalid:443"), option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())), option.WithGRPCDialOption(grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) })))
	if err != nil {
		t.Fatalf("newGoogleReader(local SDK transport) = %v, want nil", err)
	}
	reader.client.CallOptions.AccessSecretVersion = nil
	t.Cleanup(func() {
		if reader.Validate() == nil {
			if err := reader.Close(); err != nil {
				t.Errorf("reader.Close() = %v, want nil", err)
			}
		}
	})
	return reader
}

func TestGoogleSDKTransportLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		payload     []byte
		metadata    int
		providerErr error
		wantErr     error
	}{
		{name: "exact payload crosses the real SDK", payload: []byte("synthetic-secret")},
		{name: "empty payload remains valid custody", payload: []byte{}},
		{name: "provider refusal preserves status and releases no custody", providerErr: status.Error(codes.PermissionDenied, "synthetic refusal"), wantErr: core.ErrSecretStoreAccess},
		{name: "protobuf metadata is not a secret payload quota", payload: []byte("synthetic-secret"), metadata: 128 * 1024},
		{name: "provider payload ceiling is enforced after SDK decoding", payload: bytes.Repeat([]byte{1}, PayloadMaximumBytes+1), wantErr: core.ErrSecretStorePayload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := accessRequestForTest(t)
			wantName, err := request.resourceName()
			if err != nil {
				t.Fatal(err)
			}
			reader := localGoogleReader(t, func(_ context.Context, got *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
				if got.Name != wantName {
					return nil, status.Error(codes.InvalidArgument, "request identity mismatch")
				}
				if tc.providerErr != nil {
					return nil, tc.providerErr
				}
				response := officialChecksummedResponse(request, resolvedVersionForTest, bytes.Clone(tc.payload))
				if tc.metadata > 0 {
					unknown := protowire.AppendTag(nil, 100, protowire.BytesType)
					unknown = protowire.AppendBytes(unknown, bytes.Repeat([]byte{0x61}, tc.metadata))
					response.ProtoReflect().SetUnknown(unknown)
				}
				return response, nil
			})
			got, err := reader.Access(t.Context(), request)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (AccessResult{}) {
					t.Fatalf("Access() = %v/%v, want zero/%v", got, err, tc.wantErr)
				}
				if tc.providerErr != nil && status.Code(err) != codes.PermissionDenied {
					t.Fatalf("status = %v, want PermissionDenied", status.Code(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("Access() = %v, want nil", err)
			}
			data, err := got.Value.CopyBytes()
			if err != nil || !bytes.Equal(data, tc.payload) || got.Reference != resolvedReferenceForTest(request, resolvedVersionForTest) {
				t.Fatalf("SDK projection = %v/%v, want exact reference and payload", got.Reference, err)
			}
			clear(data)
			if err := got.Value.Destroy(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGoogleResponseRefusalClearsOwnedPayload(t *testing.T) {
	t.Parallel()
	response := officialChecksummedResponse(accessRequestForTest(t), resolvedVersionForTest, []byte("synthetic-secret"))
	got, err := accessResultFromGoogleResponse(AccessRequest{}, response)
	if !errors.Is(err, core.ErrSecretStoreContract) || got != (AccessResult{}) {
		t.Fatalf("invalid request = %v/%v, want zero/contract refusal", got, err)
	}
	requireProviderPayloadCleared(t, response)
}

func TestGoogleSDKCancellationReleasesReaderBeforeClose(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	reader := localGoogleReader(t, func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
		close(entered)
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	})
	ctx, cancel := context.WithCancel(t.Context())
	type outcome struct {
		result AccessResult
		err    error
	}
	done := make(chan outcome, 1)
	request := accessRequestForTest(t)
	go func() { result, err := reader.Access(ctx, request); done <- outcome{result, err} }()
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("Access join = timed out, want canceled worker exit")
			}
		}
	})
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("provider entry = timed out, want request observed")
	}
	cancel()
	select {
	case got := <-done:
		joined = true
		if got.result != (AccessResult{}) || !errors.Is(got.err, core.ErrSecretStoreAccess) || status.Code(got.err) != codes.Canceled {
			t.Fatalf("canceled Access = %v/%v, want zero/typed canceled access", got.result, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Access = timed out, want cancellation result")
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close after cancellation = %v, want nil", err)
	}
	if err := reader.Close(); !errors.Is(err, core.ErrSecretStoreContract) {
		t.Fatalf("second Close = %v, want contract refusal", err)
	}
	if got, err := reader.Access(t.Context(), request); got != (AccessResult{}) || !errors.Is(err, core.ErrSecretStoreContract) {
		t.Fatalf("closed Access = %v/%v, want zero/contract refusal", got, err)
	}
}

// Each callback crosses the real SDK decoder and the typed custody boundary.
// The local provider emits canonical protobuf from SDK-owned response structs.
func FuzzGoogleSDKResponseSemanticClosure(f *testing.F) {
	request := accessRequestForTest(f)
	if err := request.Validate(); err != nil {
		f.Fatalf("seed request.Validate() = %v, want nil", err)
	}
	seed, err := NewValue([]byte("synthetic-secret"))
	if err != nil {
		f.Fatalf("NewValue(seed) = %v, want nil", err)
	}
	canonical, err := seed.CopyBytes()
	if err != nil {
		f.Fatalf("seed.CopyBytes() = %v, want nil", err)
	}
	if err := seed.Destroy(); err != nil {
		f.Fatalf("seed.Destroy() = %v, want nil", err)
	}
	for mutation := range uint8(6) {
		f.Add(canonical, mutation)
	}
	f.Add([]byte{}, uint8(0))
	f.Add(bytes.Repeat([]byte{1}, PayloadMaximumBytes), uint8(0))
	f.Add(bytes.Repeat([]byte{1}, PayloadMaximumBytes+1), uint8(0))
	f.Fuzz(func(t *testing.T, payload []byte, mutation uint8) {
		mutation %= 6
		response := officialChecksummedResponse(request, resolvedVersionForTest, bytes.Clone(payload))
		wantErr := error(nil)
		if len(payload) > PayloadMaximumBytes {
			wantErr = core.ErrSecretStorePayload
		}
		switch mutation {
		case 0:
		case 1:
			response.Payload.DataCrc32C = nil
			wantErr = core.ErrSecretStorePayload
		case 2:
			*response.Payload.DataCrc32C ^= 1
			wantErr = core.ErrSecretStorePayload
		case 3:
			response.Name = resolvedNameTextForTest(request, "0")
			wantErr = core.ErrSecretStorePayload
		case 4:
			response.Payload = nil
			wantErr = core.ErrSecretStorePayload
		case 5:
			wantErr = core.ErrSecretStoreAccess
		}
		reader := localGoogleReader(t, func(_ context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			if mutation == 5 {
				return nil, status.Error(codes.PermissionDenied, "synthetic refusal")
			}
			return response, nil
		})
		got, err := reader.Access(t.Context(), request)
		if wantErr != nil {
			if got != (AccessResult{}) || !errors.Is(err, wantErr) {
				t.Fatalf("SDK refusal = %v/%v, want zero/%v", got, err, wantErr)
			}
			if mutation == 5 && status.Code(err) != codes.PermissionDenied {
				t.Fatalf("SDK status = %v, want PermissionDenied", status.Code(err))
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Request != request || got.Reference != resolvedReferenceForTest(request, resolvedVersionForTest) {
			t.Fatalf("SDK accepted = %v/%v, want exact validated binding", got, err)
		}
		data, err := got.Value.CopyBytes()
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("SDK payload = %x/%v, want %x/nil", data, err, payload)
		}
		clear(data)
		if err := got.Value.Destroy(); err != nil {
			t.Fatalf("SDK Value.Destroy() = %v, want nil", err)
		}
	})
}
