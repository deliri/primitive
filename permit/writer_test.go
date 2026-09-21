package permit

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"testing"
)

type permissionWriterObservation struct {
	err   error
	data  []byte
	calls int
	short bool
}

func (w *permissionWriterObservation) Write(data []byte) (int, error) {
	w.calls++
	w.data = append(w.data, data...)
	if w.short {
		return len(data) - 1, w.err
	}
	return len(data), w.err
}

func TestPermitWriterLayerTriad(t *testing.T) {
	t.Parallel()
	request, _ := permitFixture(t)
	var first, second bytes.Buffer
	if err := request.Document.Write(&first); err != nil || first.Len() == 0 || first.Len() > DocumentMaximumBytes {
		t.Fatalf("Write = %d/%v, want nonempty bounded/nil", first.Len(), err)
	}
	decoded, err := Decode(bytes.NewReader(first.Bytes()))
	if err != nil || decoded != request.Document {
		t.Fatalf("Decode written document = %v, want exact document/nil", err)
	}
	if err := decoded.Write(&second); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("second write equal/error = %v/%v, want true/nil", bytes.Equal(first.Bytes(), second.Bytes()), err)
	}
	invalid := request.Document
	invalid.Terms.Revision = RevisionUnknown
	var sink permissionWriterObservation
	err = invalid.Write(&sink)
	if !errors.Is(err, core.ErrPermitRevision) || sink.calls != 0 || len(sink.data) != 0 {
		t.Fatalf("invalid write = %v/%d/%d, want core.ErrPermitRevision/no calls/no bytes", err, sink.calls, len(sink.data))
	}
	var empty Document
	err = empty.Write(&sink)
	if !errors.Is(err, core.ErrPermitContract) || sink.calls != 0 || len(sink.data) != 0 {
		t.Fatalf("zero write = %v/%d/%d, want core.ErrPermitContract/no calls/no bytes", err, sink.calls, len(sink.data))
	}
}

func TestPermitWriterShortWriteAndCause(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cause     error
		name      string
		short     bool
		wantShort bool
	}{
		{name: "short write without error", short: true, cause: nil, wantShort: true},
		{name: "partial write retains resource failure", short: true, cause: io.ErrClosedPipe, wantShort: true},
		{name: "full extent with resource failure is still failure", short: false, cause: io.ErrClosedPipe, wantShort: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request, _ := permitFixture(t)
			sink := permissionWriterObservation{short: tc.short, err: tc.cause}
			err := request.Document.Write(&sink)
			if !errors.Is(err, core.ErrPermitWrite) || errors.Is(err, io.ErrShortWrite) != tc.wantShort || (tc.cause != nil && !errors.Is(err, tc.cause)) || sink.calls != 1 || len(sink.data) == 0 {
				t.Fatalf("write error/calls/bytes = %v/%d/%d, want core.ErrPermitWrite, short=%v, cause=%v, one nonempty attempt", err, sink.calls, len(sink.data), tc.wantShort, tc.cause)
			}
		})
	}
}

// Direct unit proof for the shared output ceiling. The current fixed document
// schema cannot naturally exceed this ceiling; no fabricated document claims
// production schema coverage here.
func TestPermitCanonicalOutputCeiling(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		size      int
		wantCalls int
	}{
		{name: "empty projection is refused", size: 0, wantErr: core.ErrPermitContract, wantCalls: 0},
		{name: "one below ceiling is written", size: DocumentMaximumBytes - 1, wantErr: nil, wantCalls: 1},
		{name: "exact ceiling is written", size: DocumentMaximumBytes, wantErr: nil, wantCalls: 1},
		{name: "one above ceiling never reaches writer", size: DocumentMaximumBytes + 1, wantErr: core.ErrPermitContract, wantCalls: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte{'x'}, tc.size)
			var sink permissionWriterObservation
			err := writeCanonicalBytes(&sink, data)
			if !errors.Is(err, tc.wantErr) || sink.calls != tc.wantCalls {
				t.Fatalf("bounded write error/calls = %v/%d, want %v/%d", err, sink.calls, tc.wantErr, tc.wantCalls)
			}
			if tc.wantCalls == 0 && len(sink.data) != 0 {
				t.Fatalf("rejected write bytes = %d, want zero", len(sink.data))
			}
			if tc.wantCalls == 1 && !bytes.Equal(sink.data, data) {
				t.Fatal("written bytes match canonical input = false, want true")
			}
		})
	}
}

func TestPermitRevisionNeverReturnsExecution(t *testing.T) {
	t.Parallel()
	for _, revision := range []Revision{RevisionUnknown, RevisionV1 + 1, 255} {
		request, _ := permitFixture(t)
		request.Document.Terms.Revision = revision
		got, err := Verify(request)
		if !errors.Is(err, core.ErrPermitRevision) || got != (Verified{}) || !errors.Is(got.Allows(permitAction(t, "operation-a"), request.EffectiveAt), core.ErrPermitAuthentication) {
			t.Fatalf("revision %d proof/error = %v/%v, want zero/core.ErrPermitRevision", revision, got, err)
		}
	}
}
