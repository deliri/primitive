package firestoredocuments

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// This real native gRPC endpoint holds the document RPC until cancellation.
// It proves cancellation ancestry, not Firestore persistence or billing.
type heldJSONProvider struct {
	firestorepb.UnimplementedFirestoreServer
	entered chan struct{}
}

type jsonCancelObservation struct {
	document JSONDocument[jsonCallerRecord]
	err      error
}

func (p heldJSONProvider) BatchGetDocuments(_ *firestorepb.BatchGetDocumentsRequest, stream firestorepb.Firestore_BatchGetDocumentsServer) error {
	close(p.entered)
	<-stream.Context().Done()
	return status.FromContextError(stream.Context().Err()).Err()
}

func TestGetJSONNativeInflightCancellationRetainsCallerCause(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	provider := heldJSONProvider{entered: make(chan struct{})}
	server := grpc.NewServer()
	firestorepb.RegisterFirestoreServer(server, provider)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		select {
		case err := <-served:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				t.Errorf("native server exit = %v, want joined stop", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("native server joined = false, want completed stop")
		}
	})
	connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	client, err := firestore.NewClient(t.Context(), "primitive-documents-proof", option.WithGRPCConn(connection), option.WithoutAuthentication())
	if err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("native client cleanup = %v, want nil", err)
		}
	})
	cause := errors.New("caller cancelled an admitted native document RPC")
	ctx, cancel := context.WithCancelCause(t.Context())
	done := make(chan jsonCancelObservation, 1)
	go func() {
		got, err := GetJSON[jsonCallerRecord](ctx, GetRequest{Reference: client.Collection("records").Doc("held")})
		done <- jsonCancelObservation{document: got, err: err}
	}()
	defer func() {
		cancel(cause)
		if done != nil {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("native read joined = false, want cancelled RPC exit")
			}
		}
	}()
	select {
	case <-provider.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("native RPC entered = false, want actual provider admission")
	}
	cancel(cause)
	select {
	case got := <-done:
		done = nil
		if !errors.Is(got.err, cause) || !errors.Is(got.err, context.Canceled) || !errors.Is(got.err, core.ErrFirestoreDocumentsOperation) || got.document != (JSONDocument[jsonCallerRecord]{}) {
			t.Fatalf("in-flight JSON refusal = (%+v, %v), want zero plus native operation, standard and custom cancellation", got.document, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("native read completion = false, want cancelled SDK call")
	}
}
