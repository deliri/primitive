package firestoredocuments

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type notFoundStatusLeaf struct{ value *status.Status }

func (notFoundStatusLeaf) Error() string                { return "native status fixture" }
func (v notFoundStatusLeaf) GRPCStatus() *status.Status { return v.value }

type notFoundStatusWrapper struct {
	value *status.Status
	child error
}

func (notFoundStatusWrapper) Error() string                { return "native wrapper fixture" }
func (v notFoundStatusWrapper) GRPCStatus() *status.Status { return v.value }
func (v notFoundStatusWrapper) Unwrap() error              { return v.child }

type notFoundChildren struct{ children []error }

func (notFoundChildren) Error() string     { return "joined fixture" }
func (v notFoundChildren) Unwrap() []error { return v.children }

type notFoundTypedNil struct{}

func (*notFoundTypedNil) Error() string { panic(io.ErrClosedPipe) }
func (*notFoundTypedNil) GRPCStatus() *status.Status {
	return status.New(codes.NotFound, "typed nil must not be observed")
}

type notFoundIsPretender struct{}

func (notFoundIsPretender) Error() string { return "not an operation or native status" }
func (notFoundIsPretender) Is(error) bool { return true }

type notFoundPanicUnwrap struct{}

func (notFoundPanicUnwrap) Error() string { return "panic unwrap fixture" }
func (notFoundPanicUnwrap) Unwrap() error { panic(io.ErrClosedPipe) }

type notFoundPanicChildren struct{}

func (notFoundPanicChildren) Error() string   { return "panic children fixture" }
func (notFoundPanicChildren) Unwrap() []error { panic(io.ErrClosedPipe) }

type notFoundPanicStatus struct{}

func (notFoundPanicStatus) Error() string              { return "panic status fixture" }
func (notFoundPanicStatus) GRPCStatus() *status.Status { panic(io.ErrClosedPipe) }

func TestIsNotFoundLayerTriad(t *testing.T) {
	t.Parallel()
	native := status.Error(codes.NotFound, "missing document fixture")
	operation := core.ErrFirestoreDocumentsOperation
	wrapped := fmt.Errorf("owning SDK read: %w", errors.Join(operation, native))
	deep := native
	for range 64 {
		deep = fmt.Errorf("transparent: %w", deep)
	}
	var typedNil *notFoundTypedNil
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "native_missing_leaf", err: native, want: true},
		{name: "transparent_single_wrapper", err: fmt.Errorf("reader: %w", native), want: true},
		{name: "SDK_operation_marker_and_native_leaf", err: errors.Join(operation, native), want: true},
		{name: "marker_sibling_order_does_not_change_absence", err: errors.Join(native, operation), want: true},
		{name: "owning_formatted_SDK_error", err: wrapped, want: true},
		{name: "multiple_native_missing_observations", err: errors.Join(native, status.Error(codes.NotFound, "same classification")), want: true},
		{name: "nested_transparent_marker_and_native_joins", err: errors.Join(fmt.Errorf("marker: %w", operation), errors.Join(operation, native)), want: true},
		{name: "multiple_fmt_wrap_operands", err: fmt.Errorf("operation %w, cause %w", operation, native), want: true},
		{name: "deep_finite_transparent_chain", err: deep, want: true},
		{name: "outer_status_agrees_with_native_child", err: notFoundStatusWrapper{value: status.New(codes.NotFound, "wrapper"), child: native}, want: true},
		{name: "nil_error_has_no_observation"},
		{name: "operation_marker_alone_is_not_native", err: operation},
		{name: "wrapped_marker_alone_is_not_native", err: fmt.Errorf("read: %w", operation)},
		{name: "repeated_markers_do_not_manufacture_native_fact", err: errors.Join(operation, operation)},
		{name: "unknown_source_leaf", err: io.ErrClosedPipe},
		{name: "native_missing_with_source_failure", err: errors.Join(operation, native, io.ErrClosedPipe)},
		{name: "source_failure_before_native_missing", err: errors.Join(io.ErrClosedPipe, native, operation)},
		{name: "nested_source_failure_cannot_hide_in_marker_wrapper", err: errors.Join(native, fmt.Errorf("source: %w", errors.Join(operation, io.ErrUnexpectedEOF)))},
		{name: "contract_refusal_competes_with_missing", err: errors.Join(operation, native, core.ErrFirestoreDocumentsContract)},
		{name: "cancelled_caller_competes_with_missing", err: errors.Join(operation, native, context.Canceled)},
		{name: "deadline_competes_with_missing", err: errors.Join(operation, native, context.DeadlineExceeded)},
		{name: "native_permission_failure_competes_with_missing", err: errors.Join(native, status.Error(codes.PermissionDenied, "denied"))},
		{name: "native_service_failure_competes_with_missing", err: errors.Join(native, status.Error(codes.Unavailable, "unavailable"))},
		{name: "nil_native_status", err: notFoundStatusLeaf{}},
		{name: "typed_nil_does_not_call_status_method", err: typedNil},
		{name: "typed_nil_sibling_refuses_join", err: errors.Join(native, typedNil)},
		{name: "empty_foreign_join_has_no_leaves", err: notFoundChildren{}},
		{name: "nil_foreign_join_child_is_not_a_native_leaf", err: notFoundChildren{children: []error{native, nil}}},
		{name: "outer_missing_status_cannot_hide_source_child", err: notFoundStatusWrapper{value: status.New(codes.NotFound, "outer"), child: errors.Join(native, io.ErrClosedPipe)}},
		{name: "outer_service_status_contradicts_missing_child", err: notFoundStatusWrapper{value: status.New(codes.Unavailable, "outer"), child: native}},
		{name: "outer_missing_status_and_marker_has_no_native_leaf", err: notFoundStatusWrapper{value: status.New(codes.NotFound, "outer"), child: operation}},
		{name: "outer_nil_status_with_native_child_refuses", err: notFoundStatusWrapper{child: native}},
		{name: "Is_pretender_is_not_the_exact_operation_marker", err: notFoundIsPretender{}},
		{name: "Is_pretender_sibling_refuses", err: errors.Join(native, notFoundIsPretender{})},
		{name: "panicking_single_unwrap_refuses", err: notFoundPanicUnwrap{}},
		{name: "panicking_multiple_unwrap_refuses", err: notFoundPanicChildren{}},
		{name: "panicking_native_status_refuses", err: notFoundPanicStatus{}},
		{name: "panicking_sibling_refuses_even_after_native_fact", err: errors.Join(native, notFoundPanicStatus{})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNotFound(tc.err); got != tc.want {
				t.Fatalf("IsNotFound(%T) = %t, want %t", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsNotFoundNativeCodeDomain(t *testing.T) {
	t.Parallel()
	// Exhaust every published native code, then the next and extreme future
	// representations. Only the physical NotFound status supplies absence.
	codesToCheck := []codes.Code{codes.OK, codes.Canceled, codes.Unknown, codes.InvalidArgument, codes.DeadlineExceeded, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied, codes.ResourceExhausted, codes.FailedPrecondition, codes.Aborted, codes.OutOfRange, codes.Unimplemented, codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unauthenticated, codes.Code(17), codes.Code(math.MaxUint32)}
	for _, code := range codesToCheck {
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()
			native := status.Error(code, "native status domain fixture")
			want := code == codes.NotFound
			if got := IsNotFound(native); got != want {
				t.Fatalf("IsNotFound(native code %d) = %t, want %t", code, got, want)
			}
			if native != nil {
				if got := IsNotFound(errors.Join(core.ErrFirestoreDocumentsOperation, native)); got != want {
					t.Fatalf("IsNotFound(SDK code %d) = %t, want %t", code, got, want)
				}
			}
		})
	}
}

func TestIsNotFoundPreservesCompetingCauses(t *testing.T) {
	t.Parallel()
	native := status.Error(codes.NotFound, "missing")
	cause := io.ErrClosedPipe
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	err := errors.Join(core.ErrFirestoreDocumentsOperation, native, ctx.Err(), context.Cause(ctx))
	if IsNotFound(err) {
		t.Fatal("IsNotFound(competing cancellation) = true, want false")
	}
	for _, want := range []error{core.ErrFirestoreDocumentsOperation, native, context.Canceled, cause} {
		if !errors.Is(err, want) {
			t.Fatalf("classified source errors.Is(%T) = false, want original cause retained", want)
		}
	}
}

func TestIsNotFoundNativeGetMissing(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	got, err := Get[nativeRecord](t.Context(), GetRequest{Reference: nativeRef(client, t)})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || status.Code(err) != codes.NotFound || got != (Document[nativeRecord]{}) || !IsNotFound(err) {
		t.Fatalf("native missing read = (%+v, %v), classification = %t, want zero and admitted native absence", got, err, IsNotFound(err))
	}
	if IsNotFound(errors.Join(err, io.ErrClosedPipe)) || !errors.Is(err, core.ErrFirestoreDocumentsOperation) {
		t.Fatal("native missing plus independent source failure admitted or original error changed")
	}
	if costs.reads.Load() != 1 || costs.writes.Load() != 0 || costs.commits.Load() != 0 {
		t.Fatalf("native absence costs = %d reads/%d writes/%d commits, want 1/0/0", costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
	}
	reportNativeCosts(t, costs)
}

type notFoundRefusalProvider struct {
	firestorepb.UnimplementedFirestoreServer
	code  codes.Code
	calls *atomic.Int64
}

func (p notFoundRefusalProvider) BatchGetDocuments(_ *firestorepb.BatchGetDocumentsRequest, _ firestorepb.Firestore_BatchGetDocumentsServer) error {
	p.calls.Add(1)
	return status.Error(p.code, "native provider refusal fixture")
}

func TestIsNotFoundNativeGetRPCRefusals(t *testing.T) {
	t.Parallel()
	for _, code := range []codes.Code{codes.PermissionDenied, codes.FailedPrecondition} {
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			calls := new(atomic.Int64)
			server := grpc.NewServer()
			firestorepb.RegisterFirestoreServer(server, notFoundRefusalProvider{code: code, calls: calls})
			served := make(chan error, 1)
			go func() { served <- server.Serve(listener) }()
			t.Cleanup(func() {
				server.Stop()
				_ = listener.Close()
				select {
				case err := <-served:
					if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
						t.Errorf("native refusal server exit = %v, want joined stop", err)
					}
				case <-time.After(10 * time.Second):
					t.Error("native refusal server joined = false, want stop")
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
					t.Errorf("native refusal client close = %v, want nil", err)
				}
			})
			got, readErr := Get[nativeRecord](t.Context(), GetRequest{Reference: client.Collection("records").Doc("refused")})
			if !errors.Is(readErr, core.ErrFirestoreDocumentsOperation) || status.Code(readErr) != code || got != (Document[nativeRecord]{}) || IsNotFound(readErr) || calls.Load() != 1 {
				t.Fatalf("native refused read = (%+v,%v), classified %t/calls %d, want zero/native code %d/false/one RPC", got, readErr, IsNotFound(readErr), calls.Load(), code)
			}
		})
	}
}
