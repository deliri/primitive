package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestSynchronousCanonicalProjectionPreservesNativeWireAndCodec(t *testing.T) {
	t.Parallel()
	owner := canonicalProjectionOwner{rows: []canonicalProjection{{A: "雪\n", Z: 9007199254740993}, {A: "tail", Z: 7}}}
	codec, err := jsonio.ProjectCanonical(func(value canonicalProjectionOwner) (canonicalProjectionOwnerWire, error) {
		return canonicalProjectionOwnerWire{Rows: value.rows}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := jsonv2.Marshal(canonicalProjectionOwnerWire{Rows: owner.rows})
	if err != nil {
		t.Fatal(err)
	}
	canonical := jsontext.Value(want)
	if err := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = jsonio.WriteCanonicalProjection(&output, owner, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}, codec)
	if err != nil || !bytes.Equal(output.Bytes(), canonical) {
		t.Fatalf("synchronous canonical projection got%q/%v want native%q/nil", output.Bytes(), err, canonical)
	}
}

func TestSynchronousCanonicalProjectionPreservesNativeDestinationRefusals(t *testing.T) {
	t.Parallel()
	document := canonicalProjection{A: "destination", Z: 3}
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}
	if err := jsonio.WriteCanonicalProjection(nil, document, policy); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil destination got%v want%v", err, core.ErrJSONContract)
	}
	file, err := os.CreateTemp(t.TempDir(), "closed-")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var native *os.PathError
	if err := jsonio.WriteCanonicalProjection(file, document, policy); !errors.Is(err, fs.ErrClosed) || !errors.As(err, &native) || native.Path != file.Name() {
		t.Fatalf("closed native destination got%v want closed identity and original path error", err)
	}

}

func TestCanonicalProjectionContextRefusesInvalidContextWithoutBytes(t *testing.T) {
	t.Parallel()
	var missing context.Context
	expired, stop := context.WithDeadlineCause(t.Context(), time.Unix(0, 1), io.ErrClosedPipe)
	t.Cleanup(stop)
	for _, row := range []struct {
		name        string
		ctx         context.Context
		want, cause error
	}{
		{name: "nil context", ctx: missing, want: core.ErrNilContext},
		{name: "expired caller deadline", ctx: expired, want: context.DeadlineExceeded, cause: io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			err := jsonio.WriteCanonicalProjectionContext(row.ctx, &output, canonicalProjection{A: "admission", Z: 3}, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
			if output.Len() != 0 || !errors.Is(err, row.want) || row.cause != nil && !errors.Is(err, row.cause) {
				t.Fatalf("contextual admission got%q/%v want no bytes and%v/%v", output.Bytes(), err, row.want, row.cause)
			}
		})
	}
}

type canonicalFirstWriteObservation struct {
	ctx    context.Context
	before uint64
	first  uint64
	bytes  int64
}

func (w *canonicalFirstWriteObservation) Write(data []byte) (int, error) {
	if w.bytes == 0 && len(data) != 0 {
		after, err := hostfacts.ObserveGoAllocationTotal(w.ctx)
		if err != nil {
			return 0, err
		}
		w.first = after.Uint64() - w.before
	}
	w.bytes += int64(len(data))
	return len(data), nil
}
func TestSynchronousCanonicalProjectionStreamsBeforeRetainingPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int{64, 4097} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			row := canonicalProjection{A: "row", Z: 7}
			rows := make([]canonicalProjection, extent)
			for i := range rows {
				rows[i] = row
			}
			document := canonicalProjectionOwnerWire{Rows: rows}
			one, err := jsonv2.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			empty, err := jsonv2.Marshal(canonicalProjectionOwnerWire{Rows: []canonicalProjection{}})
			if err != nil {
				t.Fatal(err)
			}
			before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			sink := canonicalFirstWriteObservation{ctx: t.Context(), before: before.Uint64()}
			err = jsonio.WriteCanonicalProjection(&sink, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
			want := int64(len(empty) + extent*len(one) + extent - 1)
			if err != nil || sink.bytes != want || sink.first == 0 || sink.first > 512<<10 {
				t.Fatalf("stream extent%d got bytes%d first-write-allocation%d/%v want bytes%d allocation<=524288/nil", extent, sink.bytes, sink.first, err, want)
			}
			t.Logf("native synchronous extent%d first write allocated%d bytes", extent, sink.first)
		})
	}
}
func BenchmarkSynchronousCanonicalProjection(b *testing.B) {
	document := canonicalProjection{A: "synchronous", Z: 9007199254740993}
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}
	b.ReportAllocs()
	for b.Loop() {
		if err := jsonio.WriteCanonicalProjection(io.Discard, document, policy); err != nil {
			b.Fatal(err)
		}
	}
}
