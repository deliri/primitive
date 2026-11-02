package firestoredocuments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type integerWriteRecord struct {
	Owner string                  `json:"owner"`
	When  temporal.NumericInstant `json:"when"`
}

func (r integerWriteRecord) Validate() error {
	if r.Owner == "" {
		return core.ErrFirestoreDocumentsContract
	}
	return r.When.Validate()
}

func integerWriteFixture(t testing.TB, value int64) integerWriteRecord {
	t.Helper()
	when, err := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(value))
	if err != nil {
		t.Fatal(err)
	}
	return integerWriteRecord{Owner: "actual-caller", When: when}
}

var (
	_ func(context.Context, CreateRequest[integerWriteRecord]) (core.ByteCount, error) = CreateIntegerJSON[integerWriteRecord]
	_ func(context.Context, SetRequest[integerWriteRecord]) (core.ByteCount, error)    = SetIntegerJSON[integerWriteRecord]
)

func TestIntegerJSONWriteUsesActualCallerCodecs(t *testing.T) {
	t.Parallel()
	for _, create := range []bool{true, false} {
		name := "replace follows caller JSON codecs"
		if create {
			name = "create follows caller JSON codecs"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			want := integerWriteFixture(t, 1001)
			encoded, err := core.MarshalCanonicalJSONDocument(want)
			if err != nil {
				t.Fatal(err)
			}
			var extent core.ByteCount
			result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				var writeErr error
				if create {
					extent, writeErr = CreateIntegerJSON(ctx, CreateRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: want})
				} else {
					extent, writeErr = SetIntegerJSON(ctx, SetRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: want})
				}
				return writeErr
			}})
			if err != nil || result.Validate() != nil {
				t.Fatalf("JSON write commit=(%+v,%v), want committed", result, err)
			}
			snapshot, err := ref.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			native := snapshot.Data()
			value, integer := native["when"].(int64)
			bytes, bytesErr := extent.Uint64()
			if !integer || value != 1001 || native["owner"] != want.Owner || len(native) != 2 || bytesErr != nil || bytes != uint64(len(encoded)) {
				t.Fatalf("stored native=%v extent=%d/%v, want JSON names, INTEGER1001 and exact extent%d", native, bytes, bytesErr, len(encoded))
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
				t.Fatalf("JSON write reads/writes=%d/%d, want1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}

// This is an intentionally untrusted caller marshaler. Its Validate method
// cannot certify JSON syntax, numeric representation or native document shape.
type integerWriteRawRecord struct {
	wire    []byte
	refusal error
	cancel  context.CancelCauseFunc
	cause   error
}

func (r integerWriteRawRecord) Validate() error { return r.refusal }
func (r integerWriteRawRecord) MarshalJSON() ([]byte, error) {
	if r.cancel != nil {
		r.cancel(r.cause)
	}
	return r.wire, nil
}

func queueIntegerWrite[T core.Validatable](ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, record T, create bool) (core.ByteCount, error) {
	if create {
		return CreateIntegerJSON(ctx, CreateRequest[T]{Reference: ref, Transaction: tx, Record: record})
	}
	return SetIntegerJSON(ctx, SetRequest[T]{Reference: ref, Transaction: tx, Record: record})
}

func TestIntegerJSONWriteSignedNativeBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value int64
	}{
		{"minimum signed instant", math.MinInt64}, {"one above minimum", math.MinInt64 + 1},
		{"below negative microsecond", -1001}, {"negative microsecond", -1000}, {"above negative microsecond", -999},
		{"negative nanosecond", -1}, {"real epoch zero", 0}, {"positive nanosecond", 1},
		{"below positive microsecond", 999}, {"positive microsecond", 1000}, {"above positive microsecond", 1001},
		{"last consecutive binary64 integer", 1 << 53}, {"first integer rounded by binary64", 1<<53 + 1},
		{"negative first rounded integer", -(1<<53 + 1)}, {"one below maximum", math.MaxInt64 - 1}, {"maximum signed instant", math.MaxInt64},
	} {
		for _, create := range []bool{true, false} {
			name := tc.name + " replacement"
			if create {
				name = tc.name + " creation"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				client, costs := nativeClient(t)
				ref := nativeRef(client, t)
				want := integerWriteFixture(t, tc.value)
				var extent core.ByteCount
				result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
					var writeErr error
					extent, writeErr = queueIntegerWrite(ctx, tx, ref, want, create)
					return writeErr
				}})
				if err != nil || result.Validate() != nil {
					t.Fatalf("integer commit=(%+v,%v), want committed", result, err)
				}
				snapshot, err := ref.Get(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				got, integer := snapshot.Data()["when"].(int64)
				if !integer || got != tc.value {
					t.Fatalf("native kind/value=INTEGER:%t/%d, want INTEGER/%d", integer, got, tc.value)
				}
				read, err := GetIntegerJSON[integerWriteRecord](t.Context(), GetRequest{Reference: ref})
				if err != nil || read.Record != want || read.Bytes != extent {
					t.Fatalf("integer caller closure=(%+v,%v), want(%+v,nil) extent%v", read, err, want, extent)
				}
				if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 2 || writes != 1 {
					t.Fatalf("exact integer reads/writes=%d/%d,want2/1", reads, writes)
				}
				reportNativeCosts(t, costs)
			})
		}
	}
}

type integerWriteNestedRecord struct {
	Header  integerWriteRecord       `json:"header"`
	Delay   temporal.NumericDuration `json:"delay"`
	Values  []int64                  `json:"values"`
	Rows    []integerWriteRecord     `json:"rows"`
	Enabled bool                     `json:"enabled"`
	Missing *int64                   `json:"missing"`
	Text    string                   `json:"text"`
}

func (r integerWriteNestedRecord) Validate() error {
	if err := r.Header.Validate(); err != nil {
		return err
	}
	if err := r.Delay.Validate(); err != nil {
		return err
	}
	for _, row := range r.Rows {
		if err := row.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func TestIntegerJSONWriteNestedShapesAndReplacementLayerTriad(t *testing.T) {
	t.Parallel()
	for _, create := range []bool{true, false} {
		name := "replacement removes old fields"
		if create {
			name = "creation preserves nested scalars"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			if !create {
				if _, err := ref.Set(t.Context(), map[string]any{"retired": "must disappear"}); err != nil {
					t.Fatal(err)
				}
			}
			duration, err := temporal.DurationFromNanoseconds(1001)
			if err != nil {
				t.Fatal(err)
			}
			delay, err := temporal.NewNumericDuration(duration)
			if err != nil {
				t.Fatal(err)
			}
			want := integerWriteNestedRecord{Header: integerWriteFixture(t, 1), Delay: delay, Values: []int64{math.MinInt64, 999, math.MaxInt64}, Rows: []integerWriteRecord{integerWriteFixture(t, 1001)}, Enabled: true, Text: "Élaine\n日本語"}
			before := costs.writes.Load()
			_, err = RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				_, err := queueIntegerWrite(ctx, tx, ref, want, create)
				return err
			}})
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := ref.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			native := snapshot.Data()
			header, headerOK := native["header"].(map[string]any)
			values, valuesOK := native["values"].([]any)
			rows, rowsOK := native["rows"].([]any)
			if !headerOK || header["when"] != int64(1) || native["delay"] != int64(1001) || !valuesOK || len(values) != 3 || values[0] != int64(math.MinInt64) || values[1] != int64(999) || values[2] != int64(math.MaxInt64) || !rowsOK || len(rows) != 1 || native["missing"] != nil || native["enabled"] != true || native["text"] != want.Text || len(native) != 7 {
				t.Fatalf("nested native=%v,want exact JSON integer, nullable, boolean and UTF8 shape", native)
			}
			row, rowOK := rows[0].(map[string]any)
			if !rowOK || row["when"] != int64(1001) {
				t.Fatalf("native array row=%v,want owned INTEGER1001", rows[0])
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load()-before; reads != 1 || writes != 1 {
				t.Fatalf("nested reads/writes=%d/%d,want1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestIntegerJSONWriteCodecRefusalsHaveNoEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		wire string
	}{
		{"positive int64 overflow", `{"value":9223372036854775808}`},
		{"negative int64 overflow", `{"value":-9223372036854775809}`},
		{"arbitrary precision cannot silently round", `{"value":9999999999999999999999999999999999999999}`},
		{"fraction remains unrepresentable", `{"value":1.5}`},
		{"negative fraction remains unrepresentable", `{"value":-0.5}`},
		{"integral decimal spelling is not an integer token", `{"value":1.0}`},
		{"exponent spelling is not an integer token", `{"value":1e3}`},
		{"nested fractional number is refused", `{"value":{"inner":0.5}}`},
		{"nested array overflow is refused", `{"value":[{"inner":9223372036854775808}]}`},
		{"duplicate object members are refused", `{"value":1,"value":2}`},
		{"truncated caller marshaler is refused", `{"value":`},
		{"trailing second value is refused", `{} {}`},
		{"wrong top level array is refused", `[1]`},
		{"wrong top level string is refused", `"owner"`},
		{"null is not an empty document", `null`},
		{"invalid UTF8 cannot reach native storage", "{\"value\":\"\xff\"}"},
	} {
		for _, create := range []bool{true, false} {
			name := tc.name + " set"
			if create {
				name = tc.name + " create"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				client, costs := nativeClient(t)
				ref := nativeRef(client, t)
				reached := false
				result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
					reached = true
					extent, writeErr := queueIntegerWrite(ctx, tx, ref, integerWriteRawRecord{wire: []byte(tc.wire)}, create)
					if extent != (core.ByteCount{}) || !errors.Is(writeErr, core.ErrJSONContract) || !errors.Is(writeErr, core.ErrFirestoreDocumentsOperation) {
						t.Fatalf("refused queue=(%v,%v),want zero/JSON+operation", extent, writeErr)
					}
					return writeErr
				}})
				if !reached || result != (TransactionResult{}) || !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("refused transaction=(%+v,%v),reached%t,want zero/JSON/true", result, err, reached)
				}
				if reads, writes, commits := costs.reads.Load(), costs.writes.Load(), costs.commits.Load(); reads != 0 || writes != 0 || commits != 0 {
					t.Fatalf("refusal reads/writes/commits=%d/%d/%d,want0/0/0", reads, writes, commits)
				}
				if _, err := ref.Get(t.Context()); status.Code(err) != codes.NotFound {
					t.Fatalf("refused durable document=%v,want NotFound", err)
				}
				reportNativeCosts(t, costs)
			})
		}
	}
}

func TestIntegerJSONWriteAdmissionAndCancellation(t *testing.T) {
	t.Parallel()
	cause := errors.New("integer JSON cancellation fixture")
	cancelled, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		wantCause error
	}{
		{"nil context refuses before transaction", nil, core.ErrNilContext},
		{"missing transaction refuses before projection", t.Context(), core.ErrFirestoreDocumentsContract},
		{"cancelled context preserves actual cause", cancelled, cause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, create := range []bool{true, false} {
				extent, err := queueIntegerWrite(tc.ctx, nil, nil, integerWriteFixture(t, 1), create)
				if extent != (core.ByteCount{}) || !errors.Is(err, core.ErrFirestoreDocumentsContract) || !errors.Is(err, tc.wantCause) {
					t.Fatalf("admission=(%v,%v),want zero/contract/%v", extent, err, tc.wantCause)
				}
			}
		})
	}
	for _, create := range []bool{true, false} {
		name := "set validates owning record and cancellation"
		if create {
			name = "create validates owning record and cancellation"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			_, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				bad := integerWriteFixture(t, 1)
				bad.Owner = ""
				extent, err := queueIntegerWrite(ctx, tx, ref, bad, create)
				if extent != (core.ByteCount{}) || !errors.Is(err, core.ErrFirestoreDocumentsContract) {
					t.Fatalf("invalid owner=(%v,%v),want zero/contract", extent, err)
				}
				for _, wire := range []string{`{}`, `{"value":1.5}`} {
					child, cancel := context.WithCancelCause(ctx)
					extent, err = queueIntegerWrite(child, tx, ref, integerWriteRawRecord{wire: []byte(wire), cancel: cancel, cause: cause}, create)
					cancel(nil)
					if extent != (core.ByteCount{}) || !errors.Is(err, cause) || !errors.Is(err, context.Canceled) || wire != `{}` && !errors.Is(err, core.ErrJSONContract) {
						t.Fatalf("cancel during marshaling %q=(%v,%v),want zero/cancel/cause and retained projection refusal", wire, extent, err)
					}
				}
				return err
			}})
			if !errors.Is(err, cause) || costs.writes.Load() != 0 || costs.commits.Load() != 0 {
				t.Fatalf("cancel transaction=%v writes/commits=%d/%d,want cause/0/0", err, costs.writes.Load(), costs.commits.Load())
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestIntegerJSONWriteQueuedExtentDoesNotClaimCommit(t *testing.T) {
	t.Parallel()
	for _, create := range []bool{true, false} {
		name := "replacement extent survives abort only as emission"
		if create {
			name = "create extent survives abort only as emission"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			cause := errors.New("caller rejects total extent after queue")
			var extent core.ByteCount
			result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				var queueErr error
				extent, queueErr = queueIntegerWrite(ctx, tx, ref, integerWriteFixture(t, 999), create)
				if queueErr != nil {
					return queueErr
				}
				return cause
			}})
			bytes, extentErr := extent.Uint64()
			if !errors.Is(err, cause) || result != (TransactionResult{}) || extentErr != nil || bytes == 0 || costs.writes.Load() != 0 || costs.commits.Load() != 0 {
				t.Fatalf("aborted emission=(%v,%v),transaction=(%+v,%v),writes/commits=%d/%d,want positive extent/zero commitment/no writes", extent, extentErr, result, err, costs.writes.Load(), costs.commits.Load())
			}
			if _, err := ref.Get(t.Context()); status.Code(err) != codes.NotFound {
				t.Fatalf("aborted document=%v,want NotFound", err)
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestIntegerJSONCreateConflictKeepsNativeIdentityAndStoredValue(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	original := integerWriteFixture(t, 999)
	if _, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := CreateIntegerJSON(ctx, CreateRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: original})
		return err
	}}); err != nil {
		t.Fatal(err)
	}
	var extent core.ByteCount
	result, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		extent, err = CreateIntegerJSON(ctx, CreateRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: integerWriteFixture(t, 1001)})
		return err
	}})
	if status.Code(err) != codes.AlreadyExists || !errors.Is(err, core.ErrFirestoreDocumentsOperation) || result != (TransactionResult{}) || extent == (core.ByteCount{}) {
		t.Fatalf("create collision=(%+v,%v),extent%v,want no commit/native AlreadyExists/emission", result, err, extent)
	}
	got, err := GetIntegerJSON[integerWriteRecord](t.Context(), GetRequest{Reference: ref})
	if err != nil || got.Record != original || costs.writes.Load() != 1 || costs.reads.Load() != 1 || costs.commits.Load() != 2 {
		t.Fatalf("collision preserved=(%+v,%v),reads/writes/commits=%d/%d/%d,want original/1/1/2", got, err, costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
	}
	reportNativeCosts(t, costs)
}

func TestIntegerJSONWriteNativeRetryUsesFreshSource(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	guard := client.Collection("integer-write-guards").Doc(ref.ID)
	if _, err := guard.Create(t.Context(), nativeRecord{Owner: "guard", Value: 1}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	attempt := 0
	result, err := RunTransaction(ctx, TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		attempt++
		if attempt == 2 {
			if _, err := guard.Update(ctx, []firestore.Update{{Path: "value", Value: int64(1001)}}); err != nil {
				return err
			}
		}
		read, err := Get[nativeRecord](ctx, GetRequest{Reference: guard, Transaction: tx})
		if err != nil {
			return err
		}
		if _, err := CreateIntegerJSON(ctx, CreateRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: integerWriteFixture(t, read.Record.Value)}); err != nil {
			return err
		}
		if attempt == 1 {
			_, err := guard.Update(ctx, []firestore.Update{{Path: "value", Value: int64(1001)}})
			if status.Code(err) != codes.Aborted {
				return errors.Join(core.ErrFirestoreDocumentsContract, err)
			}
			return err
		}
		return nil
	}})
	if err != nil || result.Validate() != nil || attempt != 2 {
		t.Fatalf("native retry=(%+v,%v),attempts%d,want committed/2", result, err, attempt)
	}
	got, err := GetIntegerJSON[integerWriteRecord](ctx, GetRequest{Reference: ref})
	if err != nil || got.Record != integerWriteFixture(t, 1001) || costs.reads.Load() != 3 || costs.writes.Load() != 3 || costs.commits.Load() != 4 {
		t.Fatalf("fresh integer result=(%+v,%v),reads/writes/commits=%d/%d/%d,want value1001/3/3/4", got, err, costs.reads.Load(), costs.writes.Load(), costs.commits.Load())
	}
	reportNativeCosts(t, costs)
}

func TestIntegerJSONWriteEmptyDocumentAndMultiwindowExtent(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 32*1024 - 1, 32 * 1024, 32*1024 + 1, 3*32*1024 + 1} {
		t.Run(fmt.Sprintf("native document owner extent %d", size), func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			var extent core.ByteCount
			var emitted []byte
			_, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
				if size == 0 {
					record := integerWriteRawRecord{wire: []byte(`{}`)}
					var err error
					emitted, err = core.MarshalCanonicalJSONDocument(record)
					if err != nil {
						return err
					}
					extent, err = CreateIntegerJSON(ctx, CreateRequest[integerWriteRawRecord]{Reference: ref, Transaction: tx, Record: record})
					return err
				}
				record := integerWriteFixture(t, 999)
				record.Owner = strings.Repeat("x", size)
				var err error
				emitted, err = core.MarshalCanonicalJSONDocument(record)
				if err != nil {
					return err
				}
				extent, err = SetIntegerJSON(ctx, SetRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: record})
				return err
			}})
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := ref.Get(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			got := snapshot.Data()
			bytes, extentErr := extent.Uint64()
			if extentErr != nil || bytes != uint64(len(emitted)) || size == 0 && len(got) != 0 || size != 0 && got["owner"] != strings.Repeat("x", size) {
				t.Fatalf("native document fields%d extent=%d/%v,want preserved owner size%d and emitted%d", len(got), bytes, extentErr, size, len(emitted))
			}
			if costs.reads.Load() != 1 || costs.writes.Load() != 1 {
				t.Fatalf("native extent reads/writes=%d/%d,want1/1", costs.reads.Load(), costs.writes.Load())
			}
			reportNativeCosts(t, costs)
		})
	}
}

func FuzzIntegerJSONWriteNativeSemanticClosure(f *testing.F) {
	client, _ := nativeClient(f)
	seed := integerWriteFixture(f, 1001)
	encoded, err := core.MarshalCanonicalJSONDocument(seed)
	if err != nil || seed.Validate() != nil {
		f.Fatalf("canonical typed seed=%q/%v,want validated caller emission", encoded, err)
	}
	// Public write/read of the seed pins actual Temporal codec and native kind.
	seedRef := client.Collection("integer-write-fuzz").Doc("canonical-seed")
	if _, err := RunTransaction(f.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := SetIntegerJSON(ctx, SetRequest[integerWriteRecord]{Reference: seedRef, Transaction: tx, Record: seed})
		return err
	}}); err != nil {
		f.Fatal(err)
	}
	read, err := GetIntegerJSON[integerWriteRecord](f.Context(), GetRequest{Reference: seedRef})
	if err != nil || read.Record != seed {
		f.Fatalf("canonical native seed=(%+v,%v),want(%+v,nil)", read, err, seed)
	}
	f.Add(seed.Owner, int64(1001), uint8(0), true)
	f.Add("", int64(0), uint8(0), false)
	f.Add("minimum", int64(math.MinInt64), uint8(0), false)
	f.Add("maximum", int64(math.MaxInt64), uint8(0), true)
	for mode := uint8(1); mode <= 4; mode++ {
		f.Add("unrepresentable", int64(1), mode, true)
	}
	f.Fuzz(func(t *testing.T, owner string, value int64, mode uint8, create bool) {
		namespace := sha256.Sum256([]byte(t.TempDir()))
		ref := client.Collection("integer-write-fuzz").Doc(fmt.Sprintf("mutation-%x", namespace))
		defer func() {
			if _, err := ref.Delete(t.Context()); err != nil {
				t.Errorf("native fuzz fixture delete=%v,want nil", err)
			}
		}()
		mode %= 5
		want := integerWriteFixture(t, value)
		want.Owner = owner
		var extent core.ByteCount
		_, writeErr := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
			var err error
			if mode == 0 {
				preflight, preflightErr := IntegerJSONExtent(want)
				extent, err = queueIntegerWrite(ctx, tx, ref, want, create)
				if preflight != extent || (preflightErr == nil) != (err == nil) {
					t.Fatalf("pure/write admission=(%v,%v)/(%v,%v), want identical projection and admission", preflight, preflightErr, extent, err)
				}
				return err
			}
			token := strconv.FormatInt(value, 10)
			switch mode {
			case 1:
				token = strconv.FormatUint(uint64(value)|(uint64(1)<<63), 10)
			case 2:
				token += ".5"
			case 3:
				token += "e0"
			case 4:
				token = "[" + token + ",9223372036854775808]"
			default:
				t.Fatalf("fuzz mode=%d,want exhausted1..4", mode)
			}
			record := integerWriteRawRecord{wire: []byte(`{"value":` + token + `}`)}
			preflight, preflightErr := IntegerJSONExtent(record)
			extent, err = queueIntegerWrite(ctx, tx, ref, record, create)
			if preflight != (core.ByteCount{}) || !errors.Is(preflightErr, core.ErrJSONContract) || preflight != extent {
				t.Fatalf("pure/write numeric refusal=(%v,%v)/(%v,%v), want zero/JSON and shared projection", preflight, preflightErr, extent, err)
			}
			return err
		}})
		if mode != 0 || owner == "" || !utf8.ValidString(owner) {
			wantErr := core.ErrJSONContract
			if mode == 0 && owner == "" {
				wantErr = core.ErrFirestoreDocumentsContract
			}
			if !errors.Is(writeErr, wantErr) || extent != (core.ByteCount{}) {
				t.Fatalf("refused mutation=(%v,%v),want zero/%v", extent, writeErr, wantErr)
			}
			if _, err := ref.Get(t.Context()); status.Code(err) != codes.NotFound {
				t.Fatalf("refused mutation durable result=%v,want NotFound", err)
			}
			return
		}
		if writeErr != nil {
			t.Fatalf("valid typed write=%v,want nil", writeErr)
		}
		snapshot, err := ref.Get(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		native := snapshot.Data()
		integer, isInteger := native["when"].(int64)
		if !isInteger || integer != value || native["owner"] != owner || len(native) != 2 {
			t.Fatalf("native mutation=%v,want JSON owner/INTEGER%d", native, value)
		}
		got, err := GetIntegerJSON[integerWriteRecord](t.Context(), GetRequest{Reference: ref})
		if err != nil || got.Record != want || got.Record.Validate() != nil || got.Bytes != extent {
			t.Fatalf("typed native closure=(%+v,%v),want(%+v,nil),extent%v", got, err, want, extent)
		}
		first, err := core.MarshalCanonicalJSONDocument(got.Record)
		if err != nil {
			t.Fatal(err)
		}
		_, err = RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
			secondExtent, err := SetIntegerJSON(ctx, SetRequest[integerWriteRecord]{Reference: ref, Transaction: tx, Record: got.Record})
			if err == nil && secondExtent != extent {
				t.Fatalf("second emitted extent=%v,want%v", secondExtent, extent)
			}
			return err
		}})
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := GetIntegerJSON[integerWriteRecord](t.Context(), GetRequest{Reference: ref})
		second, encodeErr := core.MarshalCanonicalJSONDocument(roundTrip.Record)
		if err != nil || encodeErr != nil || roundTrip.Record != want || !bytes.Equal(first, second) {
			t.Fatalf("second canonical native roundtrip=(%+v,%v),encode%v,bytesEqual%t,want exact caller and bytes", roundTrip, err, encodeErr, bytes.Equal(first, second))
		}
	})
}

var _ func(integerWriteRecord) (core.ByteCount, error) = IntegerJSONExtent[integerWriteRecord]

func TestIntegerJSONExtentUsesExactSharedProjection(t *testing.T) {
	t.Parallel()
	for _, value := range []int64{math.MinInt64, -1, 0, 1, 999, 1001, 1<<53 + 1, math.MaxInt64} {
		t.Run(fmt.Sprintf("preflight exact signed instant %d", value), func(t *testing.T) {
			t.Parallel()
			record := integerWriteFixture(t, value)
			encoded, err := core.MarshalCanonicalJSONDocument(record)
			if err != nil {
				t.Fatal(err)
			}
			extent, err := IntegerJSONExtent(record)
			bytes, extentErr := extent.Uint64()
			if err != nil || extentErr != nil || bytes != uint64(len(encoded)) {
				t.Fatalf("pure integer extent=(%d,%v),error%v,want emitted%d", bytes, extentErr, err, len(encoded))
			}
		})
	}
	for _, tc := range []struct {
		name   string
		record integerWriteRawRecord
		want   error
	}{
		{"overflow cannot pass a decoder preflight", integerWriteRawRecord{wire: []byte(`{"revision":9223372036854775808}`)}, core.ErrJSONContract},
		{"fraction cannot pass a decoder preflight", integerWriteRawRecord{wire: []byte(`{"revision":1.5}`)}, core.ErrJSONContract},
		{"exponent cannot pass a decoder preflight", integerWriteRawRecord{wire: []byte(`{"revision":1e0}`)}, core.ErrJSONContract},
		{"owner validation precedes marshaling", integerWriteRawRecord{wire: []byte(`{}`), refusal: core.ErrNumericOverflow}, core.ErrNumericOverflow},
		{"empty document has real two byte extent", integerWriteRawRecord{wire: []byte(`{}`)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extent, err := IntegerJSONExtent(tc.record)
			if tc.want != nil {
				if !errors.Is(err, tc.want) || extent != (core.ByteCount{}) {
					t.Fatalf("pure refused extent=(%v,%v),want zero/%v", extent, err, tc.want)
				}
				return
			}
			bytes, extentErr := extent.Uint64()
			if err != nil || extentErr != nil || bytes != 2 {
				t.Fatalf("empty document extent=(%d,%v),error%v,want2/nil", bytes, extentErr, err)
			}
		})
	}
	extent, err := IntegerJSONExtent[nonStructRecord]("non-document")
	if !errors.Is(err, core.ErrFirestoreDocumentsContract) || extent != (core.ByteCount{}) {
		t.Fatalf("non-struct preflight=(%v,%v),want zero/contract", extent, err)
	}
}
