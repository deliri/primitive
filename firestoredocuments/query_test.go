package firestoredocuments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type queryObservation struct {
	mutex     sync.Mutex
	contexts  []context.Context
	documents atomic.Int64
	fault     error
	cancel    context.CancelCauseFunc
	cause     error
}

type queryObservedStream struct {
	grpc.ClientStream
	observation *queryObservation
}

func (s *queryObservedStream) RecvMsg(value any) error {
	o := s.observation
	if o.fault != nil && o.documents.Load() != 0 {
		if o.cancel != nil {
			o.cancel(o.cause)
		}
		return o.fault
	}
	err := s.ClientStream.RecvMsg(value)
	if response, ok := value.(*firestorepb.RunQueryResponse); err == nil && ok && response.Document != nil {
		o.documents.Add(1)
	}
	return err
}

func (o *queryObservation) stream(ctx context.Context, description *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
	stream, err := streamer(ctx, description, connection, method, options...)
	if err != nil || !strings.HasSuffix(method, "/RunQuery") {
		return stream, err
	}
	o.mutex.Lock()
	o.contexts = append(o.contexts, ctx)
	o.mutex.Unlock()
	return &queryObservedStream{ClientStream: stream, observation: o}, nil
}

func queryContextsStopped(t *testing.T, o *queryObservation, want int) {
	t.Helper()
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if len(o.contexts) != want {
		t.Fatalf("native query streams = %d, want %d", len(o.contexts), want)
	}
	for _, ctx := range o.contexts {
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
			t.Fatal("query native stream context remains open after return, want SDK Stop cancellation")
		}
	}
}

func TestQueryEmulatorOrderingCursorAndEmpty(t *testing.T) {
	t.Parallel()
	observed := new(queryObservation)
	client, costs := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream))
	collection := client.Collection("query-order").Doc(strings.ReplaceAll(t.Name(), "/", "-")).Collection("items")
	for i, record := range []nativeRecord{{Owner: "target", Value: 1}, {Owner: "target", Value: 3}, {Owner: "target", Value: 2}, {Owner: "foreign", Value: 9}} {
		if _, err := collection.Doc(fmt.Sprint(i)).Set(t.Context(), record); err != nil {
			t.Fatal(err)
		}
	}
	beforeWrites, beforeReads := costs.writes.Load(), costs.reads.Load()
	query := collection.Where("owner", "==", "target").OrderBy("value", firestore.Desc).Limit(2)
	var values []int64
	visitor := func(ctx context.Context, d Document[nativeRecord]) (QueryVisitDecision, error) {
		if ctx != t.Context() || d.Record.Owner != "target" {
			t.Fatalf("visitor context/record = %v/%+v, want exact caller context and target", ctx, d.Record)
		}
		values = append(values, d.Record.Value)
		return QueryContinue, nil
	}
	if err := Query(t.Context(), QueryRequest[nativeRecord]{Query: query, Visit: visitor}); err != nil || !slices.Equal(values, []int64{3, 2}) {
		t.Fatalf("ordered Query = %v/%v, want [3 2]/nil", values, err)
	}
	values = nil
	if err := Query(t.Context(), QueryRequest[nativeRecord]{Query: query.StartAfter(int64(2)), Visit: visitor}); err != nil || !slices.Equal(values, []int64{1}) {
		t.Fatalf("cursor Query = %v/%v, want [1]/nil", values, err)
	}
	values = nil
	if err := Query(t.Context(), QueryRequest[nativeRecord]{Query: collection.Where("owner", "==", "absent"), Visit: visitor}); err != nil || len(values) != 0 {
		t.Fatalf("empty Query = %v/%v, want no visits/nil", values, err)
	}
	if reads, writes, queries := costs.reads.Load()-beforeReads, costs.writes.Load()-beforeWrites, costs.queries.Load(); reads != 0 || writes != 0 || queries != 3 || observed.documents.Load() != 3 {
		t.Fatalf("point reads/writes/query RPCs/delivered query documents = %d/%d/%d/%d, want 0/0/3/3 (not a billing claim)", reads, writes, queries, observed.documents.Load())
	}
	queryContextsStopped(t, observed, 3)
}

func TestQueryEmulatorAdmissionStopAndCauses(t *testing.T) {
	t.Parallel()
	visitorCause := errors.New("native query visitor refusal")
	cancelCause := errors.New("native query caller cancellation")
	for _, row := range []struct {
		name                                                                                                string
		stop, visitorFailure, cancel, providerFailure, invalidDecision, invalidRecord, decodeFailure, empty bool
		wantVisits                                                                                          int
		want                                                                                                error
	}{
		{name: "native exhaustion admits every validated record", wantVisits: 3},
		{name: "explicit early stop visits exactly first record", stop: true, wantVisits: 1},
		{name: "visitor refusal survives alongside stop", stop: true, visitorFailure: true, wantVisits: 1, want: visitorCause},
		{name: "unknown decision refuses rather than stopping", invalidDecision: true, wantVisits: 1, want: core.ErrFirestoreDocumentsContract},
		{name: "canceled stop cannot become success", stop: true, cancel: true, wantVisits: 1, want: cancelCause},
		{name: "visitor and caller causes both survive", visitorFailure: true, cancel: true, wantVisits: 1, want: visitorCause},
		{name: "native stream fault and caller cancellation both survive", providerFailure: true, wantVisits: 1, want: cancelCause},
		{name: "owning invalid record never reaches visitor", invalidRecord: true, want: core.ErrFirestoreDocumentsContract},
		{name: "native type mismatch never reaches visitor", decodeFailure: true, want: core.ErrFirestoreDocumentsOperation},
		{name: "empty collection is neutral", empty: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			observed := &queryObservation{cancel: cancel, cause: cancelCause}
			if row.providerFailure {
				observed.fault = status.Error(codes.PermissionDenied, "controlled native stream fault after real document")
			}
			client, costs := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream))
			collection := client.Collection("query-exits").Doc(strings.ReplaceAll(t.Name(), "/", "-")).Collection("items")
			if !row.empty {
				for i := 0; i < 3; i++ {
					record := nativeRecord{Owner: "owner", Value: int64(i)}
					if row.invalidRecord {
						record.Owner = ""
					}
					var err error
					if row.decodeFailure {
						_, err = collection.Doc(fmt.Sprint(i)).Set(t.Context(), incompatibleRecord{Owner: "owner", Value: "wrong-native-type"})
					} else {
						_, err = collection.Doc(fmt.Sprint(i)).Set(t.Context(), record)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			beforeWrites, beforeReads := costs.writes.Load(), costs.reads.Load()
			visits := 0
			err := Query(ctx, QueryRequest[nativeRecord]{Query: collection.OrderBy("value", firestore.Asc), Visit: func(gotCtx context.Context, d Document[nativeRecord]) (QueryVisitDecision, error) {
				visits++
				if gotCtx != ctx || d.Validate() != nil {
					t.Fatalf("visitor = %v/%+v, want owning context and validated record", gotCtx, d)
				}
				if row.cancel {
					cancel(cancelCause)
				}
				decision := QueryContinue
				if row.stop {
					decision = QueryStop
				}
				if row.invalidDecision {
					decision = QueryVisitDecision(255)
				}
				if row.visitorFailure {
					return decision, visitorCause
				}
				return decision, nil
			}})
			if row.want == nil {
				if err != nil {
					t.Fatalf("Query error = %v, want nil", err)
				}
			} else if !errors.Is(err, row.want) {
				t.Fatalf("Query error = %v, want typed cause %v", err, row.want)
			}
			if (row.cancel || row.providerFailure) && (!errors.Is(err, context.Canceled) || !errors.Is(err, cancelCause)) {
				t.Fatalf("competing Query error = %v, want Canceled and custom cause", err)
			}
			if row.providerFailure && status.Code(err) != codes.PermissionDenied {
				t.Fatalf("native Query status = %v, want PermissionDenied", status.Code(err))
			}
			if visits != row.wantVisits || costs.writes.Load() != beforeWrites || costs.reads.Load() != beforeReads || costs.queries.Load() != 1 {
				t.Fatalf("visits/point reads/writes/query RPCs = %d/%d/%d/%d, want %d/0/0/1", visits, costs.reads.Load()-beforeReads, costs.writes.Load()-beforeWrites, costs.queries.Load(), row.wantVisits)
			}
			queryContextsStopped(t, observed, 1)
		})
	}
}

func TestQueryRefusesBeforeNativeEffects(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	query := client.Collection("query-contract").Query
	visitor := func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error) {
		t.Fatal("refused query reached visitor")
		return QueryContinue, nil
	}
	for _, row := range []struct {
		name  string
		query firestore.Query
		visit func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error)
	}{
		{name: "zero native query", visit: visitor},
		{name: "missing visitor", query: query},
		{name: "invalid native operator", query: query.Where("value", "unknown", 1), visit: visitor},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := Query(t.Context(), QueryRequest[nativeRecord]{Query: row.query, Visit: row.visit})
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) {
				t.Fatalf("query admission = %v, want contract", err)
			}
		})
	}
	if err := Query[nativeRecord](nil, QueryRequest[nativeRecord]{Query: query, Visit: visitor}); !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context Query = %v, want nil-context identity", err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("query pre-admission canceled")
	cancel(cause)
	if err := Query(ctx, QueryRequest[nativeRecord]{Query: query, Visit: visitor}); !errors.Is(err, cause) || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel Query = %v, want both causes", err)
	}
	if err := Query(t.Context(), QueryRequest[nonStructRecord]{Query: query, Visit: func(context.Context, Document[nonStructRecord]) (QueryVisitDecision, error) {
		t.Fatal("nonstruct visited")
		return QueryContinue, nil
	}}); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("nonstruct Query = %v, want contract", err)
	}
	t.Cleanup(func() {
		if costs.queries.Load() != 0 || costs.reads.Load() != 0 || costs.writes.Load() != 0 {
			t.Fatalf("refused query effects = %d/%d/%d, want all zero", costs.queries.Load(), costs.reads.Load(), costs.writes.Load())
		}
	})
}

func TestQueryVisitDecisionExhaustive(t *testing.T) {
	t.Parallel()
	for i := 0; i < 256; i++ {
		d := QueryVisitDecision(i)
		err := d.Validate()
		want := d == QueryContinue || d == QueryStop
		if (err == nil) != want || err != nil && !errors.Is(err, core.ErrFirestoreDocumentsContract) {
			t.Fatalf("decision %d validation = %v, want admitted=%v", i, err, want)
		}
	}
}

func FuzzQueryAdmitsExactPersistedTypedRecords(f *testing.F) {
	client, _ := nativeClient(f)
	seed := nativeRecord{Owner: "canonical-query-owner", Value: 17}
	seedRef := client.Collection("query-fuzz-seed").Doc(fmt.Sprint(os.Getpid()))
	if _, err := seedRef.Set(f.Context(), seed); err != nil {
		f.Fatal(err)
	}
	canonical, err := Get[nativeRecord](f.Context(), GetRequest{Reference: seedRef})
	if err != nil || canonical.Record != seed {
		f.Fatalf("native query seed = %+v/%v, want %+v/nil", canonical, err, seed)
	}
	f.Add(canonical.Record.Owner, canonical.Record.Value, false)
	f.Add("", int64(0), false)
	f.Add("wrong-native-type", int64(3), true)
	var ordinal atomic.Uint64
	f.Fuzz(func(t *testing.T, owner string, value int64, wrongType bool) {
		ref := client.Collection("query-fuzz").Doc(fmt.Sprintf("%d-%d", os.Getpid(), ordinal.Add(1)))
		defer func() {
			if _, err := ref.Delete(t.Context()); err != nil {
				t.Errorf("query fuzz cleanup = %v, want nil", err)
			}
		}()
		var err error
		if wrongType {
			_, err = ref.Set(t.Context(), incompatibleRecord{Owner: owner, Value: "wrong-native-type"})
		} else {
			_, err = ref.Set(t.Context(), nativeRecord{Owner: owner, Value: value})
		}
		if err != nil {
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("native persisted fixture error = %v, want native invalid material refusal", err)
			}
			return
		}
		point, pointErr := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
		visits := 0
		var got nativeRecord
		err = Query(t.Context(), QueryRequest[nativeRecord]{Query: ref.Parent.Where(firestore.DocumentID, "==", ref), Visit: func(_ context.Context, d Document[nativeRecord]) (QueryVisitDecision, error) {
			visits++
			got = d.Record
			return QueryContinue, nil
		}})
		if wrongType || owner == "" {
			want := core.ErrFirestoreDocumentsContract
			if wrongType {
				want = core.ErrFirestoreDocumentsOperation
			}
			if !errors.Is(err, want) || visits != 0 || got != (nativeRecord{}) || point != (Document[nativeRecord]{}) || !errors.Is(pointErr, want) {
				t.Fatalf("query typed refusal = %+v/%v/%d, point=%+v/%v, want zero visitor and typed %v", got, err, visits, point, pointErr, want)
			}
			return
		}
		want := nativeRecord{Owner: owner, Value: value}
		if err != nil || pointErr != nil || visits != 1 || got != want || point.Record != got {
			t.Fatalf("query/point = %+v/%v/%d, %+v/%v, want exactly %+v", got, err, visits, point, pointErr, want)
		}
	})
}
