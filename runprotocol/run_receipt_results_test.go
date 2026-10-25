package runprotocol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

// resultStream encodes results with the given ordinals, as a producer writes them.
func resultStream(t testing.TB, ordinals ...uint64) []byte {
	t.Helper()
	suite, err := NewTestLabel("example.org/project/pkg")
	if err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	encoder, err := jsonio.NewObjectEncoder[TestResult](jsonio.ObjectDestinationRequest{Destination: &stream})
	if err != nil {
		t.Fatal(err)
	}
	for _, ordinal := range ordinals {
		name, err := NewTestLabel("TestNative")
		if err != nil {
			t.Fatal(err)
		}
		if err := encoder.Encode(t.Context(), TestResult{Test: TestIdentity{Suite: suite, Name: name}, Ordinal: ordinal, Outcome: OutcomePassed}); err != nil {
			t.Fatal(err)
		}
	}
	return stream.Bytes()
}

// receiptFor binds a receipt to exactly these stream bytes and count.
func receiptFor(t testing.TB, stream []byte, count uint64) RunReceipt {
	t.Helper()
	receipt := runReceiptFixture(t)
	size, err := core.NewByteCount(uint64(len(stream)))
	if err != nil {
		t.Fatal(err)
	}
	receipt.Results.Digest, receipt.Results.Bytes, receipt.ResultCount = core.SHA256Of(stream), size, count
	return receipt
}

// A receipt's result stream verifies only when its records are valid, its
// ordinals run exactly 1 through the receipt's count, and its exact bytes are
// the size and digest the receipt names.
func TestVerifyTestResultsLayerTriad(t *testing.T) {
	t.Parallel()
	three := resultStream(t, 1, 2, 3)
	changed := bytes.Clone(three)
	changed[bytes.LastIndex(changed, []byte("TestNative"))] = 'X'
	noResults := receiptFor(t, three, 3)
	noResults.Results, noResults.ResultCount = nil, 0
	for _, row := range []struct {
		name    string
		source  io.Reader
		receipt RunReceipt
		wantErr error
	}{
		{name: "positive exact stream", source: bytes.NewReader(three), receipt: receiptFor(t, three, 3)},
		{name: "neutral a single result", source: bytes.NewReader(resultStream(t, 1)), receipt: receiptFor(t, resultStream(t, 1), 1)},
		{name: "negative an ordinal gap", source: bytes.NewReader(resultStream(t, 1, 3)), receipt: receiptFor(t, resultStream(t, 1, 3), 2), wantErr: core.ErrRunProtocolConflict},
		{name: "negative ordinals not starting at one", source: bytes.NewReader(resultStream(t, 2, 3)), receipt: receiptFor(t, resultStream(t, 2, 3), 2), wantErr: core.ErrRunProtocolConflict},
		{name: "negative a repeated ordinal", source: bytes.NewReader(resultStream(t, 1, 1)), receipt: receiptFor(t, resultStream(t, 1, 1), 2), wantErr: core.ErrRunProtocolConflict},
		{name: "negative fewer results than the count", source: bytes.NewReader(three), receipt: receiptFor(t, three, 4), wantErr: core.ErrRunProtocolConflict},
		{name: "negative more results than the count", source: bytes.NewReader(three), receipt: receiptFor(t, three, 2), wantErr: core.ErrRunProtocolConflict},
		{name: "negative bytes past the declared size", source: io.MultiReader(bytes.NewReader(three), strings.NewReader("\n")), receipt: receiptFor(t, three, 3), wantErr: core.ErrRunProtocolConflict},
		{name: "negative a same-length change", source: bytes.NewReader(changed), receipt: receiptFor(t, three, 3), wantErr: core.ErrRunProtocolConflict},
		{name: "negative a truncated record", source: bytes.NewReader(three[:len(three)-3]), receipt: receiptFor(t, three, 3), wantErr: core.ErrRunProtocolContract},
		{name: "negative an invalid record", source: strings.NewReader(`{"test":{"suite":"s","name":"n"},"ordinal":1,"outcome":"future"}`), receipt: receiptFor(t, three, 3), wantErr: core.ErrRunProtocolContract},
		{name: "negative a receipt naming no stream", source: bytes.NewReader(three), receipt: noResults, wantErr: core.ErrRunProtocolContract},
		{name: "negative a missing source", receipt: receiptFor(t, three, 3), wantErr: core.ErrRunProtocolContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := VerifyTestResults(t.Context(), TestResultStreamRequest{Source: row.source, Receipt: row.receipt})
			if !errors.Is(err, row.wantErr) || row.wantErr == nil && err != nil {
				t.Fatalf("VerifyTestResults = %v, want %v", err, row.wantErr)
			}
		})
	}
}

// endlessLabel is one record whose suite label never ends.
type endlessLabel struct{ started bool }

func (r *endlessLabel) Read(p []byte) (int, error) {
	if !r.started {
		r.started = true
		return copy(p, `{"test":{"suite":"`), nil
	}
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// A hostile stream cannot make verification read past the receipt's declared
// size: an endless record stops there, with allocation far below what the
// source would supply. Serial, so the allocation total sees only this call.
func TestVerifyTestResultsStopsAtTheDeclaredSize(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	stream := resultStream(t, 1)
	receipt := receiptFor(t, stream, 1)
	before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	verifyErr := VerifyTestResults(t.Context(), TestResultStreamRequest{Source: &endlessLabel{}, Receipt: receipt})
	after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if allocated := after.Uint64() - before.Uint64(); !errors.Is(verifyErr, core.ErrRunProtocolContract) || allocated > 4<<20 {
		t.Fatalf("endless record = %v after allocating %d bytes, want a contract refusal within 4 MiB", verifyErr, allocated)
	}
}

func TestVerifyTestResultsExactExtentLayerTriad(t *testing.T) {
	t.Parallel()
	stream := resultStream(t, 1, 2, 3)
	for _, row := range []struct {
		name     string
		declared int
		digest   core.SHA256Digest
		wantErr  error
	}{
		{name: "exact extent and digest agree", declared: len(stream), digest: core.SHA256Of(stream)},
		{name: "extent one larger cannot borrow unchanged digest and count", declared: len(stream) + 1, digest: core.SHA256Of(stream), wantErr: core.ErrRunProtocolConflict},
		{name: "shorter extent with the correct prefix digest cannot certify the whole stream", declared: len(stream) - 1, digest: core.SHA256Of(stream[:len(stream)-1]), wantErr: core.ErrRunProtocolConflict},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			receipt := receiptFor(t, stream, 3)
			var err error
			receipt.Results.Bytes, err = core.NewByteCount(uint64(row.declared))
			if err != nil {
				t.Fatal(err)
			}
			receipt.Results.Digest = row.digest
			if err := receipt.Validate(); err != nil {
				t.Fatal(err)
			}
			err = VerifyTestResults(t.Context(), TestResultStreamRequest{Source: bytes.NewReader(stream), Receipt: receipt})
			if !errors.Is(err, row.wantErr) || row.wantErr == nil && err != nil {
				t.Fatalf("VerifyTestResults(%d declared bytes) = %v, want %v", row.declared, err, row.wantErr)
			}
		})
	}
}

type resultReadFailure struct {
	data   []byte
	cause  error
	cancel context.CancelCauseFunc
	reads  int
}

func (r *resultReadFailure) Read(p []byte) (int, error) {
	r.reads++
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) != 0 {
		return n, nil
	}
	if r.cancel != nil {
		r.cancel(r.cause)
	}
	return n, r.cause
}

func TestVerifyTestResultsNativeFailureAcknowledgement(t *testing.T) {
	t.Parallel()
	stream := resultStream(t, 1, 2, 3)
	for _, row := range []struct {
		name   string
		source func() io.Reader
		want   error
	}{
		{name: "late source error after complete records is not pipe EOF success", source: func() io.Reader { return io.MultiReader(bytes.NewReader(stream), iotest.ErrReader(io.ErrClosedPipe)) }, want: io.ErrClosedPipe},
		{name: "complete record bytes and source error in one Read retain the cause", source: func() io.Reader { return &resultReadFailure{data: bytes.Clone(stream), cause: io.ErrClosedPipe} }, want: io.ErrClosedPipe},
		{name: "partial record bytes and source error retain the cause", source: func() io.Reader {
			return &resultReadFailure{data: bytes.Clone(stream[:len(stream)/2]), cause: io.ErrClosedPipe}
		}, want: io.ErrClosedPipe},
		{name: "joined EOF and source failure cannot be clean termination", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader(stream), iotest.ErrReader(errors.Join(io.EOF, io.ErrClosedPipe)))
		}, want: io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := VerifyTestResults(t.Context(), TestResultStreamRequest{Source: row.source(), Receipt: receiptFor(t, stream, 3)})
			if !errors.Is(err, row.want) || !errors.Is(err, core.ErrRunProtocolContract) {
				t.Fatalf("VerifyTestResults(source refusal) = %v, want contract and native %v", err, row.want)
			}
		})
	}
}

func TestVerifyTestResultsRetainsClosedNativeFileCause(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "source")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	stream := resultStream(t, 1)
	err = VerifyTestResults(t.Context(), TestResultStreamRequest{Source: io.MultiReader(bytes.NewReader(stream), file), Receipt: receiptFor(t, stream, 1)})
	native, ok := errors.AsType[*os.PathError](err)
	if !ok || native.Op != "read" || native.Path != path || !errors.Is(err, os.ErrClosed) || !errors.Is(err, core.ErrRunProtocolContract) {
		t.Fatalf("closed native source = (%v,%v), want original PathError/read/path/closed contract", native, err)
	}
}

type resultPanicReader struct{}

func (resultPanicReader) Read([]byte) (int, error) {
	panic(io.ErrClosedPipe)
}

func TestVerifyTestResultsJoinsRecoveredSourcePanic(t *testing.T) {
	t.Parallel()
	stream := resultStream(t, 1)
	source := io.MultiReader(bytes.NewReader(stream), resultPanicReader{})
	err := VerifyTestResults(t.Context(), TestResultStreamRequest{Source: source, Receipt: receiptFor(t, stream, 1)})
	if !errors.Is(err, core.ErrRunProtocolContract) || !errors.Is(err, core.ErrFilestoreCallbackPanic) || errors.Is(err, core.ErrRunProtocolConflict) {
		t.Fatalf("verification after complete records and recovered source panic = %v, want joined callback-panic contract without a record conflict", err)
	}
}

func TestVerifyTestResultsMaximumNativeExtentStillReadsSource(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name     string
		extent   uint64
		wantRead bool
		want     error
	}{
		{name: "maximum signed extent admits the native source", extent: math.MaxInt64, wantRead: true, want: io.ErrClosedPipe},
		{name: "one above native signed extent refuses before reading", extent: uint64(math.MaxInt64) + 1, want: core.ErrNumericOverflow},
		{name: "maximum unsigned declaration refuses before reading", extent: math.MaxUint64, want: core.ErrNumericOverflow},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			stream := resultStream(t, 1)
			receipt := receiptFor(t, stream, 1)
			var err error
			receipt.Results.Bytes, err = core.NewByteCount(row.extent)
			if err != nil {
				t.Fatal(err)
			}
			source := &resultReadFailure{cause: io.ErrClosedPipe}
			err = VerifyTestResults(t.Context(), TestResultStreamRequest{Source: source, Receipt: receipt})
			if (source.reads != 0) != row.wantRead || !errors.Is(err, row.want) || !errors.Is(err, core.ErrRunProtocolContract) {
				t.Fatalf("declared extent%d = (%d reads,%v), want source admission%t and cause%v", row.extent, source.reads, err, row.wantRead, row.want)
			}
		})
	}
}

func TestVerifyTestResultsCancellationRetainsCause(t *testing.T) {
	t.Parallel()
	stream := resultStream(t, 1)
	for _, before := range []bool{true, false} {
		name := "cancellation after source progress"
		if before {
			name = "cancellation before source progress"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			source := &resultReadFailure{data: bytes.Clone(stream), cause: io.ErrClosedPipe, cancel: cancel}
			if before {
				cancel(io.ErrClosedPipe)
			}
			err := VerifyTestResults(ctx, TestResultStreamRequest{Source: source, Receipt: receiptFor(t, stream, 1)})
			if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || before && source.reads != 0 || !before && source.reads == 0 {
				t.Fatalf("cancelled verification = (%d reads,%v), want cancellation/custom cause and matching source progress", source.reads, err)
			}
		})
	}
}

func TestVerifyTestResultsAcceptsLargeWholeLabel(t *testing.T) {
	t.Parallel()
	label, err := NewTestLabel(strings.Repeat("界", (1<<20)/3+1))
	if err != nil {
		t.Fatal(err)
	}
	result := TestResult{Test: TestIdentity{Suite: label, Name: label}, Ordinal: 1, Outcome: OutcomePassed}
	var stream bytes.Buffer
	encoder, err := jsonio.NewObjectEncoder[TestResult](jsonio.ObjectDestinationRequest{Destination: &stream})
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(t.Context(), result); err != nil {
		t.Fatal(err)
	}
	data := stream.Bytes()
	if len(data) <= 2<<20 {
		t.Fatalf("stream extent = %d, want two whole labels beyond working windows", len(data))
	}
	if err := VerifyTestResults(t.Context(), TestResultStreamRequest{Source: bytes.NewReader(data), Receipt: receiptFor(t, data, 1)}); err != nil {
		t.Fatalf("large valid result stream verification = %v, want exact admitted content", err)
	}
}

func FuzzVerifyTestResultsSemanticClosure(f *testing.F) {
	seed := resultStream(f, 1, 2, 3)
	for _, row := range []struct {
		data        []byte
		size, count uint64
		alterDigest bool
	}{
		{data: seed, size: uint64(len(seed)), count: 3},
		{data: seed, size: uint64(len(seed)) + 1, count: 3},
		{data: seed, size: uint64(len(seed)) - 1, count: 3},
		{data: seed, size: uint64(len(seed)), count: 3, alterDigest: true},
		{data: resultStream(f, 1, 3), size: uint64(len(resultStream(f, 1, 3))), count: 2},
		{data: seed, size: math.MaxInt64, count: 3},
		{data: []byte{}, size: 1, count: 1},
	} {
		f.Add(row.data, row.size, row.count, row.alterDigest)
	}
	f.Fuzz(func(t *testing.T, data []byte, size, count uint64, alterDigest bool) {
		receipt := runReceiptFixture(t)
		receipt.ResultCount = count
		var err error
		receipt.Results.Bytes, err = core.NewByteCount(size)
		if err != nil {
			receipt.Results.Bytes = core.ByteCount{}
		}
		receipt.Results.Digest = core.SHA256Of(data)
		if alterDigest {
			digest, err := receipt.Results.Digest.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			digest[0] ^= 1
			receipt.Results.Digest = core.NewSHA256Digest(digest)
		}
		want := receipt.Validate() == nil && size == uint64(len(data)) && !alterDigest && size <= math.MaxInt64
		var ordinal uint64
		for result, decodeErr := range jsonio.Objects[TestResult](t.Context(), jsonio.ObjectSourceRequest{Source: bytes.NewReader(data), Members: jsonio.MembersDeclaredOnly}) {
			if decodeErr != nil {
				want = false
				break
			}
			if ordinal == math.MaxUint64 || result.Ordinal != ordinal+1 {
				want = false
				break
			}
			ordinal++
		}
		want = want && ordinal == count
		verifyErr := VerifyTestResults(t.Context(), TestResultStreamRequest{Source: bytes.NewReader(data), Receipt: receipt})
		if want && verifyErr != nil || !want && (!errors.Is(verifyErr, core.ErrRunProtocolContract) && !errors.Is(verifyErr, core.ErrRunProtocolConflict)) {
			t.Fatalf("VerifyTestResults(%d bytes,size%d,count%d,altered%t) = %v, want admitted %t", len(data), size, count, alterDigest, verifyErr, want)
		}
	})
}
