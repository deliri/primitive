package jsonio_test

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

type countingMemberDestination struct{ bytes uint64 }

func (w *countingMemberDestination) Write(data []byte) (int, error) {
	w.bytes += uint64(len(data))
	return len(data), nil
}

func TestObjectMemberStreamRetainsConstantHeapAcrossRowCardinality(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 512 << 10
	row := canonicalMemberRow{Zeta: "actual row", Alpha: 7}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	canonical := jsontext.Value(encoded)
	if err := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		t.Fatal(err)
	}
	for _, count := range [...]int{256, 8192, 65536} {
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var retained int64
		var destination countingMemberDestination
		err = jsonio.WriteObjectMembers(t.Context(), jsonio.ObjectMemberRequest{Destination: &destination, Members: func(ctx context.Context, cursor *jsonio.TokenEncoder) error {
			for _, token := range [...]jsonio.Token{{Kind: jsonio.TokenString, Text: "rows"}, {Kind: jsonio.TokenArrayStart}} {
				if err := cursor.Encode(ctx, token); err != nil {
					return err
				}
			}
			for i := range count {
				if err := jsonio.EncodeCanonicalObject(ctx, cursor, row, jsonio.OwnedCodecPolicy{Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullPermitted}); err != nil {
					return err
				}
				if i == count/2 || i == count-1 {
					after, err := hostfacts.ObserveCollectedGoHeap(ctx)
					if err != nil {
						return err
					}
					retained = max(retained, int64(after.Uint64())-int64(before.Uint64()))
				}
			}
			return cursor.Encode(ctx, jsonio.Token{Kind: jsonio.TokenArrayEnd})
		}})
		wantBytes := uint64(len(`{"rows":[]}`) + count*len(canonical) + count - 1)
		t.Logf("rows=%d accepted_bytes=%d retained_heap_bytes=%d allowance=%d", count, destination.bytes, retained, allowance)
		if err != nil || destination.bytes != wantBytes || retained > allowance {
			t.Fatalf("stream = %d bytes, %v, retained=%d; want %d bytes and <=%d retained", destination.bytes, err, retained, wantBytes, allowance)
		}
	}
}
