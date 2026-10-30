package firestoredocuments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type queryObservation struct {
	mutex     sync.Mutex
	contexts  []context.Context
	readTimes []time.Time
	documents atomic.Int64
	fault     error
	cancel    context.CancelCauseFunc
	cause     error
}

type queryObservedStream struct {
	grpc.ClientStream
	observation *queryObservation
}

func (s *queryObservedStream) SendMsg(value any) error {
	if request, ok := value.(*firestorepb.RunQueryRequest); ok && request.GetReadTime() != nil {
		s.observation.mutex.Lock()
		s.observation.readTimes = append(s.observation.readTimes, request.GetReadTime().AsTime())
		s.observation.mutex.Unlock()
	}
	return s.ClientStream.SendMsg(value)
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
			t.Fatalf("native stream context error = %v, want canceled after Query return", ctx.Err())
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
	visitor := func(gotContext context.Context, gotDocument Document[nativeRecord]) (QueryVisitDecision, error) {
		t.Fatalf("refused query visitor = %v/%+v, want no visit", gotContext, gotDocument)
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
	cause := errors.New("query pre-admission cancellation")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	canceled, stop := context.WithCancel(t.Context())
	stop()
	for _, row := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "absent context is a typed refusal", ctx: nil, want: core.ErrNilContext},
		{name: "canceled context retains cancellation", ctx: canceled, want: context.Canceled},
		{name: "custom caller cause remains visible", ctx: ctx, want: cause},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := Query[nativeRecord](row.ctx, QueryRequest[nativeRecord]{Query: query, Visit: visitor})
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || !errors.Is(err, row.want) {
				t.Fatalf("Query context refusal = %v, want contract and %v", err, row.want)
			}
		})
	}
	if err := Query(t.Context(), QueryRequest[nonStructRecord]{Query: query, Visit: func(gotContext context.Context, gotDocument Document[nonStructRecord]) (QueryVisitDecision, error) {
		t.Fatalf("nonstruct query visitor = %v/%+v, want no visit", gotContext, gotDocument)
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
	f.Add("\xff", int64(0), false)
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
			writeErr := err
			if !utf8.ValidString(owner) {
				if status.Code(writeErr) != codes.Internal {
					t.Fatalf("native invalid UTF-8 fixture error = %v, want Internal marshaling refusal", writeErr)
				}
				if !strings.Contains(status.Convert(writeErr).Message(), "string field contains invalid UTF-8") {
					t.Fatalf("native Internal diagnostic = %q, want invalid UTF-8 marshaling cause", status.Convert(writeErr).Message())
				}
			} else if status.Code(writeErr) != codes.InvalidArgument {
				t.Fatalf("native persisted fixture error = %v, want native invalid material refusal", writeErr)
			}
			point, pointErr := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
			if !errors.Is(pointErr, core.ErrFirestoreDocumentsOperation) || status.Code(pointErr) != codes.NotFound || point != (Document[nativeRecord]{}) {
				t.Fatalf("native write refusal readback = %+v/%v, want zero document and operation/NotFound", point, pointErr)
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

func TestQueryBindsNativeClientProjectAndDatabase(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	visitor := func(gotContext context.Context, gotDocument Document[nativeRecord]) (QueryVisitDecision, error) {
		t.Fatalf("binding query visitor = %v/%+v, want no execution", gotContext, gotDocument)
		return QueryContinue, nil
	}
	for _, row := range []struct {
		name, project, database string
		foreign                 bool
	}{
		{name: "different client same project and default database refuses", project: "primitive-documents-proof", database: "(default)", foreign: true},
		{name: "foreign project refuses before RPC", project: "primitive-foreign-proof", database: "(default)", foreign: true},
		{name: "same project foreign database refuses before RPC", project: "primitive-documents-proof", database: "other-database", foreign: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			connection, err := grpc.NewClient(nativeEmulatorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainStreamInterceptor(costs.stream))
			if err != nil {
				t.Fatal(err)
			}
			candidate, err := firestore.NewClientWithDatabase(t.Context(), row.project, row.database, option.WithGRPCConn(connection), option.WithoutAuthentication())
			if err != nil {
				_ = connection.Close()
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := candidate.Close(); err != nil {
					t.Errorf("native binding client Close = %v, want nil", err)
				}
				if connection.GetState() != connectivity.Shutdown {
					t.Errorf("binding connection state = %v, want Shutdown", connection.GetState())
				}
			})
			for _, query := range []firestore.Query{candidate.Collection("items").Query, candidate.Collection("parents").Doc("parent").Collection("items").Query} {
				err := (QueryRequest[nativeRecord]{Query: query, Visit: visitor}).ValidateClient(client)
				if row.foreign {
					if !errors.Is(err, core.ErrFirestoreDocumentsContract) {
						t.Fatalf("foreign native resource binding = %v, want typed contract", err)
					}
				} else if err != nil {
					t.Fatalf("same native resource binding = %v, want nil", err)
				}
			}
		})
	}
	if err := (QueryRequest[nativeRecord]{Query: client.Collection("items").Query, Visit: visitor}).ValidateClient(nil); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("nil binding client = %v, want contract", err)
	}
	if err := (QueryRequest[nativeRecord]{Visit: visitor}).ValidateClient(client); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("zero query binding = %v, want contract", err)
	}
	t.Cleanup(func() {
		if costs.queries.Load() != 0 || costs.reads.Load() != 0 || costs.writes.Load() != 0 {
			t.Errorf("query binding native effects = %d/%d/%d, want all zero", costs.queries.Load(), costs.reads.Load(), costs.writes.Load())
		}
	})
}

// A joined native exhaustion marker is not clean exhaustion. This drives an
// actual emulator result before a controlled native stream fault; it does not
// substitute a mock query or a fake Document for the production path.
func TestQueryJoinedExhaustionCannotHideProviderCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		cancel bool
	}{
		{name: "joined exhaustion retains provider failure"},
		{name: "joined exhaustion retains provider and caller failure", cancel: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			cause := errors.New("joined query caller failure")
			observed := &queryObservation{fault: errors.Join(iterator.Done, status.Error(codes.PermissionDenied, "joined native provider failure")), cause: cause}
			if row.cancel {
				observed.cancel = cancel
			}
			client, costs := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream))
			ref := nativeRef(client, t)
			if _, err := ref.Set(t.Context(), nativeRecord{Owner: "native-owner", Value: 1}); err != nil {
				t.Fatal(err)
			}
			visits := 0
			err := Query(ctx, QueryRequest[nativeRecord]{Query: ref.Parent.Where(firestore.DocumentID, "==", ref), Visit: func(_ context.Context, d Document[nativeRecord]) (QueryVisitDecision, error) {
				visits++
				if d.Record.Owner != "native-owner" {
					t.Fatalf("query record owner = %q, want native-owner", d.Record.Owner)
				}
				return QueryContinue, nil
			}})
			if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, iterator.Done) || status.Code(err) != codes.PermissionDenied {
				t.Fatalf("joined query error = %v, want operation, observed exhaustion marker and PermissionDenied together", err)
			}
			if row.cancel && (!errors.Is(err, context.Canceled) || !errors.Is(err, cause)) {
				t.Fatalf("joined query cancellation = %v, want retained caller causes", err)
			}
			if !row.cancel && ctx.Err() != nil {
				t.Fatalf("uncanceled query context = %v, want nil independently of provider refusal", ctx.Err())
			}
			if visits != 1 || costs.queries.Load() != 1 {
				t.Fatalf("joined query visits/RPCs = %d/%d, want 1/1", visits, costs.queries.Load())
			}
			queryContextsStopped(t, observed, 1)
		})
	}
}

func TestOffWireFirestoreEnumMethodContracts(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		mode := TransactionMode(value)
		decision := QueryVisitDecision(value)
		wantMode := "unknown"
		if value == 0 {
			wantMode = "read_write"
		}
		if value == 1 {
			wantMode = "read_only"
		}
		wantDecision := "unknown"
		if value == 1 {
			wantDecision = "continue"
		}
		if value == 2 {
			wantDecision = "stop"
		}
		if mode.String() != wantMode || mode.IsValid() != (value < 2) || (mode.Validate() == nil) != mode.IsValid() {
			t.Fatalf("transaction mode %d = %q/%t/%v, want %q and matching closed validation", value, mode.String(), mode.IsValid(), mode.Validate(), wantMode)
		}
		if decision.String() != wantDecision || decision.IsValid() != (value == 1 || value == 2) || (decision.Validate() == nil) != decision.IsValid() {
			t.Fatalf("query decision %d = %q/%t/%v, want %q and matching closed validation", value, decision.String(), decision.IsValid(), decision.Validate(), wantDecision)
		}
	}
}

func TestQueryNativeFixtureInvalidUTF8LeavesNoDocumentOrQuery(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	_, writeErr := ref.Set(t.Context(), nativeRecord{Owner: "\xff", Value: 17})
	if status.Code(writeErr) != codes.Internal {
		t.Fatalf("native invalid UTF-8 write = %v, want Internal marshaling refusal", writeErr)
	}
	if !strings.Contains(status.Convert(writeErr).Message(), "string field contains invalid UTF-8") {
		t.Fatalf("native Internal diagnostic = %q, want invalid UTF-8 marshaling cause", status.Convert(writeErr).Message())
	}
	point, pointErr := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
	if !errors.Is(pointErr, core.ErrFirestoreDocumentsOperation) || status.Code(pointErr) != codes.NotFound || point != (Document[nativeRecord]{}) {
		t.Fatalf("native invalid UTF-8 readback = %+v/%v, want zero document and operation/NotFound", point, pointErr)
	}
	if gotQueries, gotWrites := costs.queries.Load(), costs.writes.Load(); gotQueries != 0 || gotWrites != 0 {
		t.Fatalf("native invalid UTF-8 effects = %d queries/%d writes, want 0/0", gotQueries, gotWrites)
	}
}

func TestQueryClientlessDeserializeRefusesBeforeEffects(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	defer func() {
		if fault := recover(); fault != nil {
			t.Errorf("clientless Query panic = %T/%v, want typed Contract before effects\n%s", fault, fault, debug.Stack())
		}
		if got := costs.queries.Load(); got != 0 {
			t.Errorf("clientless query RPCs = %d, want 0", got)
		}
	}()
	encoded, err := client.Collection("clientless-query").Query.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	var zero firestore.Query
	restored, err := zero.Deserialize(encoded)
	if err != nil {
		t.Fatal(err)
	}
	visits := 0
	request := QueryRequest[nativeRecord]{Query: restored, Visit: func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error) {
		visits++
		return QueryContinue, nil
	}}
	err = Query(t.Context(), request)
	if !errors.Is(err, core.ErrFirestoreDocumentsContract) || visits != 0 {
		t.Fatalf("clientless query = %v/%d visits, want typed Contract/0", err, visits)
	}
	if err := request.ValidateClient(client); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Fatalf("clientless ValidateClient = %v, want Contract", err)
	}
}

func TestQueryZeroDecisionVisitorFailurePreservesOnlyObservedCauses(t *testing.T) {
	t.Parallel()
	observed := new(queryObservation)
	client, _ := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream))
	ref := nativeRef(client, t)
	if _, err := ref.Set(t.Context(), nativeRecord{Owner: "visitor-owner", Value: 17}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	visitorCause := errors.New("owning visitor refusal")
	callerCause := errors.New("owning cancellation after visit")
	visits := 0
	err := Query(ctx, QueryRequest[nativeRecord]{Query: ref.Parent.Where(firestore.DocumentID, "==", ref), Visit: func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error) {
		visits++
		cancel(callerCause)
		return 0, visitorCause
	}})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, visitorCause) || !errors.Is(err, callerCause) || queryHasLiteralContractCause(err) || visits != 1 {
		t.Fatalf("zero-decision visitor = %v/%d visits, want operation+visitor+caller causes/1 and no invented Contract", err, visits)
	}
	queryContextsStopped(t, observed, 1)
}

func TestQueryPreservesBorrowedNativeReadOptions(t *testing.T) {
	t.Parallel()
	observed := new(queryObservation)
	client, costs := nativeClient(t, grpc.WithChainStreamInterceptor(observed.stream))
	ref := nativeRef(client, t)
	written, err := ref.Set(t.Context(), nativeRecord{Owner: "read-time-owner", Value: 17})
	if err != nil {
		t.Fatal(err)
	}
	query := ref.Parent.Where(firestore.DocumentID, "==", ref)
	query.WithReadOptions(firestore.ReadTime(written.UpdateTime))
	visits := 0
	request := QueryRequest[nativeRecord]{Query: query, Visit: func(_ context.Context, got Document[nativeRecord]) (QueryVisitDecision, error) {
		visits++
		if got.Record != (nativeRecord{Owner: "read-time-owner", Value: 17}) {
			t.Fatalf("read-time record = %+v, want original typed document", got.Record)
		}
		return QueryContinue, nil
	}}
	if err := request.ValidateClient(client); err != nil {
		t.Fatalf("read-time bound request = %v, want nil", err)
	}
	if err := Query(t.Context(), request); err != nil || visits != 1 {
		t.Fatalf("read-time query = %v/%d visits, want nil/1", err, visits)
	}
	observed.mutex.Lock()
	times := append([]time.Time(nil), observed.readTimes...)
	observed.mutex.Unlock()
	if len(times) != 1 || !times[0].Equal(written.UpdateTime) {
		t.Fatalf("native query read times = %v, want exact [%v]", times, written.UpdateTime)
	}
	if costs.queries.Load() != 1 {
		t.Fatalf("read-time query RPCs = %d, want 1", costs.queries.Load())
	}
	queryContextsStopped(t, observed, 1)
}

// Only native join branches are separate causes. A compiler-owned identity's
// declared parent is not another cause contributed by the visitor decision.
func queryHasLiteralContractCause(err error) bool {
	if identity, ok := err.(core.ErrorIdentity); ok {
		return identity == core.ErrFirestoreDocumentsContract
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			if queryHasLiteralContractCause(cause) {
				return true
			}
		}
	}
	return false
}

func TestQueryBindsExecutingClientBeforeResourceAdmission(t *testing.T) {
	t.Parallel()
	receiver, costs := nativeClient(t)
	for _, row := range []struct {
		name, project, database string
	}{
		{name: "different client with same project and database", project: "primitive-documents-proof", database: "(default)"},
		{name: "deserialized foreign project keeps receiver client", project: "primitive-foreign-proof", database: "(default)"},
		{name: "deserialized foreign database keeps receiver client", project: "primitive-documents-proof", database: "other-database"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			connection, err := grpc.NewClient(nativeEmulatorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainStreamInterceptor(costs.stream))
			if err != nil {
				t.Fatal(err)
			}
			bound, err := firestore.NewClientWithDatabase(t.Context(), row.project, row.database, option.WithGRPCConn(connection), option.WithoutAuthentication())
			if err != nil {
				_ = connection.Close()
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := bound.Close(); err != nil {
					t.Errorf("bound client Close = %v, want nil", err)
				}
				if connection.GetState() != connectivity.Shutdown {
					t.Errorf("bound connection state = %v, want Shutdown", connection.GetState())
				}
			})
			encoded, err := bound.Collection("items").Query.Serialize()
			if err != nil {
				t.Fatal(err)
			}
			restored, err := receiver.Collection("receiver").Query.Deserialize(encoded)
			if err != nil {
				t.Fatal(err)
			}
			visits := 0
			request := QueryRequest[nativeRecord]{Query: restored, Visit: func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error) {
				visits++
				return QueryContinue, nil
			}}
			if err := request.ValidateClient(bound); !errors.Is(err, core.ErrFirestoreDocumentsContract) || visits != 0 {
				t.Fatalf("executing-client binding = %v/%d visits, want typed Contract/0", err, visits)
			}
		})
	}
	t.Cleanup(func() {
		if costs.queries.Load() != 0 || costs.reads.Load() != 0 || costs.writes.Load() != 0 {
			t.Errorf("client mismatch effects = %d/%d/%d, want 0/0/0", costs.queries.Load(), costs.reads.Load(), costs.writes.Load())
		}
	})
}

func TestQueryRefusesUninitializedNativeClientBeforeEffects(t *testing.T) {
	t.Parallel()
	visits := 0
	defer func() {
		if fault := recover(); fault != nil {
			t.Errorf("zero-client query panic = %T/%v, want typed Contract before effects\n%s", fault, fault, debug.Stack())
		}
		if visits != 0 {
			t.Errorf("zero-client visitor calls = %d, want 0", visits)
		}
	}()
	client := &firestore.Client{}
	request := QueryRequest[nativeRecord]{Query: client.Collection("items").Query, Visit: func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error) {
		visits++
		return QueryContinue, nil
	}}
	if err := request.Validate(); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Errorf("zero-client validation = %v, want typed Contract", err)
	}
	if err := Query(t.Context(), request); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Errorf("zero-client Query = %v, want typed Contract", err)
	}
	if err := request.ValidateClient(client); !errors.Is(err, core.ErrFirestoreDocumentsContract) {
		t.Errorf("zero-client bound validation = %v, want typed Contract", err)
	}
}

func TestQueryClosedInitializedClientPreservesNativeFailure(t *testing.T) {
	t.Parallel()
	connection, err := grpc.NewClient(nativeEmulatorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	client, err := firestore.NewClient(t.Context(), "primitive-documents-proof", option.WithGRPCConn(connection), option.WithoutAuthentication())
	if err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("initialized client Close = %v, want nil", err)
	}
	if connection.GetState() != connectivity.Shutdown {
		t.Fatalf("closed client connection = %v, want Shutdown", connection.GetState())
	}
	visits := 0
	request := QueryRequest[nativeRecord]{Query: client.Collection("items").Query, Visit: func(context.Context, Document[nativeRecord]) (QueryVisitDecision, error) {
		visits++
		return QueryContinue, nil
	}}
	if err := request.ValidateClient(client); err != nil {
		t.Fatalf("closed initialized binding = %v, want nil before native transport observes closure", err)
	}
	err = Query(t.Context(), request)
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, grpc.ErrClientConnClosing) || visits != 0 {
		t.Fatalf("closed native Query = %v/%d visits, want operation+native connection closing/0", err, visits)
	}
}
