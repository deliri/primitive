package firestoredocuments

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"math"
	"slices"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ func(context.Context, GetRequest) (JSONDocument[jsonCallerRecord], error) = GetIntegerJSON[jsonCallerRecord]

func TestGetIntegerJSONEmulatorNativeIntegerBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value int64
	}{
		{"zero remains a real integer", 0}, {"smallest positive nanosecond", 1}, {"nearest negative nanosecond", -1},
		{"below positive microsecond", 999}, {"exact positive microsecond", 1000}, {"above positive microsecond", 1001},
		{"above negative microsecond", -999}, {"exact negative microsecond", -1000}, {"below negative microsecond", -1001},
		{"signed native minimum", math.MinInt64}, {"one above native minimum", math.MinInt64 + 1},
		{"one below native maximum", math.MaxInt64 - 1}, {"signed native maximum", math.MaxInt64},
		{"below double precision edge", 1<<53 - 1}, {"exact double precision edge", 1 << 53}, {"above double precision edge", 1<<53 + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			want := jsonCallerRecord{Owner: "integer-native-kind", Value: tc.value}
			if _, err := ref.Set(t.Context(), map[string]any{"owner": want.Owner, "value": tc.value}); err != nil {
				t.Fatal(err)
			}
			got, err := GetIntegerJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: ref})
			encoded, encodeErr := core.MarshalCanonicalJSONDocument(want)
			extent, extentErr := got.Bytes.Uint64()
			if err != nil || encodeErr != nil || extentErr != nil || got.Record != want || got.Validate() != nil || extent != uint64(len(encoded)) {
				t.Fatalf("native integer %d = (%+v,%v), extent/error=%d/%v, want (%+v,nil) and exact extent%d", tc.value, got, err, extent, extentErr, want, len(encoded))
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
				t.Fatalf("native reads/writes = %d/%d, want1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}

// DOUBLE1.0 must be refused before JSON erases its native kind. The old public
// GetJSON deliberately keeps its existing integral-double decoding behavior.
func TestGetIntegerJSONRejectsIntegralDoubleBeforeProjection(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	if _, err := ref.Set(t.Context(), map[string]any{"owner": "double-native-kind", "value": float64(1)}); err != nil {
		t.Fatal(err)
	}
	relaxed, err := GetJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: ref})
	if err != nil || relaxed.Record != (jsonCallerRecord{Owner: "double-native-kind", Value: 1}) {
		t.Fatalf("existing JSON double = (%+v,%v), want unchanged integral projection", relaxed, err)
	}
	before := costs.reads.Load()
	got, err := GetIntegerJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: ref})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, core.ErrJSONContract) || got != (JSONDocument[jsonCallerRecord]{}) {
		t.Fatalf("native DOUBLE1.0 = (%+v,%v), want zero and operation/JSON refusal", got, err)
	}
	if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != before+1 || writes != 1 {
		t.Fatalf("strict reads/writes = %d/%d, want%d/1", reads, writes, before+1)
	}
	snapshot, err := ref.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	value, ok := snapshot.Data()["value"].(float64)
	if !ok || value != 1 {
		t.Fatalf("stored kind/value = double:%t/%v, want true/1 without repair", ok, value)
	}
	if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != before+2 || writes != 1 {
		t.Fatalf("verified reads/writes = %d/%d, want%d/1", reads, writes, before+2)
	}
	reportNativeCosts(t, costs)
}

type integerJSONChild struct {
	Value int64 `json:"value"`
}
type integerJSONNestedRecord struct {
	Owner  string             `json:"owner"`
	Value  int64              `json:"value"`
	Nested integerJSONChild   `json:"nested"`
	Values []int64            `json:"values"`
	Rows   []integerJSONChild `json:"rows"`
}

func (r integerJSONNestedRecord) Validate() error {
	if r.Owner == "" {
		return core.ErrFirestoreDocumentsContract
	}
	return nil
}

func TestGetIntegerJSONEmulatorNestedKindLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		field   string
		value   any
		wantErr bool
	}{
		{name: "nested map retains submicrosecond integer", field: "nested", value: map[string]any{"value": int64(1)}},
		{name: "array retains signed integer extrema", field: "values", value: []any{int64(math.MinInt64), int64(math.MaxInt64)}},
		{name: "array map retains postmicrosecond integer", field: "rows", value: []any{map[string]any{"value": int64(1001)}}},
		{name: "empty native array emits no numeric fact", field: "values", value: []any{}},
		{name: "empty native map emits no numeric fact", field: "nested", value: map[string]any{}},
		{name: "null array remains absent", field: "rows", value: nil},
		{name: "nested DOUBLE zero cannot impersonate integer", field: "nested", value: map[string]any{"value": float64(0)}, wantErr: true},
		{name: "array DOUBLE one cannot impersonate integer", field: "values", value: []any{int64(0), float64(1)}, wantErr: true},
		{name: "array map DOUBLE nanosecond cannot impersonate integer", field: "rows", value: []any{map[string]any{"value": float64(1)}}, wantErr: true},
		{name: "fractional DOUBLE remains a kind refusal", field: "nested", value: map[string]any{"value": 0.5}, wantErr: true},
		{name: "negative zero DOUBLE remains a kind refusal", field: "values", value: []any{math.Copysign(0, -1)}, wantErr: true},
		{name: "single precision provider input remains DOUBLE", field: "rows", value: []any{map[string]any{"value": float32(1)}}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			source := map[string]any{"owner": "nested-native-kind", "value": int64(999), tc.field: tc.value}
			if _, err := ref.Set(t.Context(), source); err != nil {
				t.Fatal(err)
			}
			got, err := GetIntegerJSON[integerJSONNestedRecord](t.Context(), GetRequest{Reference: ref})
			if tc.wantErr {
				if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, core.ErrJSONContract) || got.Record.Owner != "" || got.Record.Value != 0 || got.Record.Nested.Value != 0 || got.Record.Values != nil || got.Record.Rows != nil || got.Bytes != (core.ByteCount{}) {
					t.Fatalf("nested native numeric refusal = (%+v,%v), want entirely zero and operation/JSON refusal", got, err)
				}
			} else {
				var want integerJSONNestedRecord
				wire, encodeErr := core.MarshalCanonicalJSONDocument(source)
				decodeErr := json.Unmarshal(wire, &want, json.RejectUnknownMembers(true))
				if err != nil || encodeErr != nil || decodeErr != nil || got.Validate() != nil || got.Record.Owner != want.Owner || got.Record.Value != want.Value || got.Record.Nested != want.Nested || !slices.Equal(got.Record.Values, want.Values) || !slices.Equal(got.Record.Rows, want.Rows) {
					t.Fatalf("nested integer projection = (%+v,%v), want (%+v,nil), encoding/decoding=%v/%v", got, err, want, encodeErr, decodeErr)
				}
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
				t.Fatalf("nested native reads/writes = %d/%d, want1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestGetIntegerJSONEmulatorTransactionAndTypedRefusal(t *testing.T) {
	t.Parallel()
	client, costs := nativeClient(t)
	ref := nativeRef(client, t)
	if _, err := ref.Set(t.Context(), map[string]any{"owner": "transaction-native-kind", "value": int64(1001)}); err != nil {
		t.Fatal(err)
	}
	_, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		got, err := GetIntegerJSON[jsonCallerRecord](ctx, GetRequest{Reference: ref, Transaction: tx})
		if err != nil || got.Record != (jsonCallerRecord{Owner: "transaction-native-kind", Value: 1001}) {
			t.Fatalf("transaction integer projection = (%+v,%v), want exact typed value", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
		t.Fatalf("transaction reads/writes = %d/%d, want1/1", reads, writes)
	}
	missing := ref.Parent.Doc(ref.ID + "-missing")
	got, err := GetIntegerJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: missing})
	if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || status.Code(err) != codes.NotFound || got != (JSONDocument[jsonCallerRecord]{}) {
		t.Fatalf("missing integer document = (%+v,%v), want zero/native NotFound", got, err)
	}
	before := costs.reads.Load()
	cause := errors.New("integer JSON caller cancellation")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	got, err = GetIntegerJSON[jsonCallerRecord](ctx, GetRequest{Reference: ref})
	if !errors.Is(err, cause) || !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrFirestoreDocumentsContract) || got != (JSONDocument[jsonCallerRecord]{}) {
		t.Fatalf("canceled integer projection = (%+v,%v), want zero and exact cause/contract", got, err)
	}
	scalar, err := GetIntegerJSON[nonStructRecord](t.Context(), GetRequest{Reference: ref})
	if !errors.Is(err, core.ErrFirestoreDocumentsContract) || scalar != (JSONDocument[nonStructRecord]{}) || costs.reads.Load() != before {
		t.Fatalf("invalid scalar integer projection = (%+v,%v), reads%d want unchanged%d", scalar, err, costs.reads.Load(), before)
	}
	reportNativeCosts(t, costs)
}

func TestIntegerJSONNativeProjectionGuardExhaustsSDKKinds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "native null"}, {name: "native true", value: true}, {name: "native false", value: false},
		{name: "native empty string", value: ""}, {name: "native integer zero", value: int64(0)},
		{name: "native integer minimum", value: int64(math.MinInt64)}, {name: "native integer maximum", value: int64(math.MaxInt64)},
		{name: "native bytes", value: []byte{0, 255}}, {name: "native empty map", value: map[string]any{}}, {name: "native empty array", value: []any{}},
		{name: "native single precision positive zero", value: float32(0), wantErr: true},
		{name: "native single precision subnormal", value: float32(math.SmallestNonzeroFloat32), wantErr: true},
		{name: "native double integral one", value: float64(1), wantErr: true},
		{name: "native double negative zero", value: math.Copysign(0, -1), wantErr: true},
		{name: "native double subnormal", value: math.SmallestNonzeroFloat64, wantErr: true},
		{name: "native double largest", value: math.MaxFloat64, wantErr: true},
		{name: "native positive infinity", value: math.Inf(1), wantErr: true}, {name: "native negative infinity", value: math.Inf(-1), wantErr: true}, {name: "native not a number", value: math.NaN(), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateIntegerJSONValue(t.Context(), tc.value)
			if (err != nil) != tc.wantErr || err != nil && !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("native kind%T error=%v, want typed refusal%t", tc.value, err, tc.wantErr)
			}
		})
	}
	cause := errors.New("native projection canceled before traversal")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	if err := validateIntegerJSONValue(ctx, map[string]any{"nested": []any{int64(1)}}); !errors.Is(err, cause) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled native traversal=%v, want exact cause and Canceled", err)
	}
}

func FuzzGetIntegerJSONEmulatorNativeKindConservation(f *testing.F) {
	client, _ := nativeClient(f)
	seedRef := client.Collection("integer-json-fuzz").Doc("typed-seed")
	seed := jsonProviderRecord{Owner: "native-integer-seed", Value: 1001}
	if _, err := seedRef.Set(f.Context(), seed); err != nil {
		f.Fatal(err)
	}
	canonical, err := GetIntegerJSON[jsonCallerRecord](f.Context(), GetRequest{Reference: seedRef})
	if err != nil || canonical.Record != (jsonCallerRecord{Owner: seed.Owner, Value: seed.Value}) || canonical.Validate() != nil {
		f.Fatalf("canonical integer seed=(%+v,%v), want exact validated provider facts", canonical, err)
	}
	f.Add(canonical.Record.Value, uint8(0))
	f.Add(int64(1), uint8(1))
	f.Add(int64(math.MinInt64), uint8(0))
	f.Add(int64(math.MaxInt64), uint8(0))
	for kind := uint8(2); kind < 8; kind++ {
		f.Add(canonical.Record.Value, kind)
	}
	f.Fuzz(func(t *testing.T, value int64, kind uint8) {
		key, keyErr := core.SHA256Of([]byte(t.TempDir())).Hex()
		if keyErr != nil {
			t.Fatalf("owned fixture key error = %v, want nil", keyErr)
		}
		ref := client.Collection("integer-json-fuzz").Doc(key)
		defer func() {
			if _, err := ref.Delete(t.Context()); err != nil {
				t.Errorf("native fixture cleanup=%v, want nil", err)
			}
		}()
		var nativeValue any = value
		if kind&1 != 0 {
			nativeValue = float64(value)
		}
		source := map[string]any{"owner": "native-integer-mutation"}
		want := integerJSONNestedRecord{Owner: "native-integer-mutation"}
		switch (kind >> 1) & 3 {
		case 0:
			source["value"] = nativeValue
			want.Value = value
		case 1:
			source["nested"] = map[string]any{"value": nativeValue}
			want.Nested.Value = value
		case 2:
			source["values"] = []any{int64(0), nativeValue}
			want.Values = []int64{0, value}
		case 3:
			source["rows"] = []any{map[string]any{"value": nativeValue}}
			want.Rows = []integerJSONChild{{Value: value}}
		}
		if _, err := ref.Set(t.Context(), source); err != nil {
			t.Fatal(err)
		}
		got, err := GetIntegerJSON[integerJSONNestedRecord](t.Context(), GetRequest{Reference: ref})
		if kind&1 != 0 {
			if !errors.Is(err, core.ErrFirestoreDocumentsOperation) || !errors.Is(err, core.ErrJSONContract) || got.Record.Owner != "" || got.Record.Value != 0 || got.Record.Nested.Value != 0 || got.Record.Values != nil || got.Record.Rows != nil || got.Bytes != (core.ByteCount{}) {
				t.Fatalf("mutated DOUBLE(%d) shape%d=(%+v,%v), want zero/operation/JSON refusal", value, (kind>>1)&3, got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Record.Owner != want.Owner || got.Record.Value != want.Value || got.Record.Nested != want.Nested || !slices.Equal(got.Record.Values, want.Values) || !slices.Equal(got.Record.Rows, want.Rows) {
			t.Fatalf("mutated INTEGER(%d) shape%d=(%+v,%v), want (%+v,nil)", value, (kind>>1)&3, got, err, want)
		}
	})
}

func TestGetIntegerJSONEmulatorCallerSchemaAndValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		source       map[string]any
		wantErr      error
		wantSemantic bool
	}{
		{name: "missing integer retains caller zero semantics", source: map[string]any{"owner": "schema-owner"}},
		{name: "caller validation still refuses absent owner", source: map[string]any{"value": int64(1)}, wantErr: core.ErrFirestoreDocumentsContract},
		{name: "unknown integer member remains strict JSON refusal", source: map[string]any{"owner": "schema-owner", "value": int64(1), "future": int64(1)}, wantErr: core.ErrFirestoreDocumentsOperation, wantSemantic: true},
		{name: "string cannot impersonate integer member", source: map[string]any{"owner": "schema-owner", "value": "1"}, wantErr: core.ErrFirestoreDocumentsOperation, wantSemantic: true},
		{name: "boolean cannot impersonate integer member", source: map[string]any{"owner": "schema-owner", "value": true}, wantErr: core.ErrFirestoreDocumentsOperation, wantSemantic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, costs := nativeClient(t)
			ref := nativeRef(client, t)
			if _, err := ref.Set(t.Context(), tc.source); err != nil {
				t.Fatal(err)
			}
			got, err := GetIntegerJSON[jsonCallerRecord](t.Context(), GetRequest{Reference: ref})
			if tc.wantErr == nil {
				if err != nil || got.Record != (jsonCallerRecord{Owner: "schema-owner"}) || got.Validate() != nil {
					t.Fatalf("missing integer caller zero=(%+v,%v), want schema-owner/0/nil", got, err)
				}
			} else {
				if !errors.Is(err, tc.wantErr) || got != (JSONDocument[jsonCallerRecord]{}) {
					t.Fatalf("caller schema refusal=(%+v,%v), want zero/%v", got, err, tc.wantErr)
				}
				if _, ok := errors.AsType[*json.SemanticError](err); tc.wantSemantic && !ok {
					t.Fatalf("caller schema error=%v, want native SemanticError ancestry", err)
				}
			}
			if reads, writes := costs.reads.Load(), costs.writes.Load(); reads != 1 || writes != 1 {
				t.Fatalf("caller schema reads/writes=%d/%d, want1/1", reads, writes)
			}
			reportNativeCosts(t, costs)
		})
	}
}

func TestGetIntegerJSONAdmissionRetainsNoRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		wantCause error
	}{
		{name: "missing reference", ctx: t.Context()},
		{name: "nil context", wantCause: core.ErrNilContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetIntegerJSON[jsonCallerRecord](tc.ctx, GetRequest{})
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || tc.wantCause != nil && !errors.Is(err, tc.wantCause) || got != (JSONDocument[jsonCallerRecord]{}) {
				t.Fatalf("integer JSON admission=(%+v,%v), want zero/contract/cause%v", got, err, tc.wantCause)
			}
		})
	}
}
