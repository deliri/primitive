package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// relayBackstop bounds a relay that should have finished; a wedged relay
// fails the row instead of hanging the package.
const relayBackstop = 20 * time.Second

var errRelayWrite = errors.New("relay test write refused")

// closeRecorder records FarOutput's bytes and how often it was closed.
type closeRecorder struct {
	bytes.Buffer
	closes   int
	closeErr error
}

func (c *closeRecorder) Close() error {
	c.closes++
	return c.closeErr
}

// refusingWriter fails every write with errRelayWrite.
type refusingWriter struct{}

func (refusingWriter) Write([]byte) (int, error) { return 0, errRelayWrite }
func (refusingWriter) Close() error              { return nil }

// refusingCloser fails every write and records its close.
type refusingCloser struct{ closes int }

func (*refusingCloser) Write([]byte) (int, error) { return 0, errRelayWrite }
func (r *refusingCloser) Close() error            { r.closes++; return nil }

func relayBytes(size int, seed byte) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*29) ^ seed
	}
	return data
}

// Relay copies both directions at once and returns only after both joined.
// Each row's streams are finite, so its outcome is decided by the streams,
// not by timing; the backstop only distinguishes a wedged relay.
func TestRelayLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		// up and down are the NearInput and FarInput payload sizes.
		up, down int
		// echo feeds NearOutput back into NearInput, ended after down bytes.
		echo bool
		// refuseNear fails NearOutput; refuseFar fails FarOutput.
		refuseNear, refuseFar bool
		closeErr              error
		nearCloseErr          error
		wantUpErr             bool
		wantDownErr           bool
	}{
		{name: "both directions carry exact independent payloads", up: 4096, down: 8192},
		{name: "empty streams on both sides join with no bytes", up: 0, down: 0},
		{name: "upward bytes that depend on downward bytes need both copies at once", down: 256 << 10, echo: true},
		{name: "a refused near writer fails downward and still joins upward", up: 1024, down: 1024, refuseNear: true, wantDownErr: true},
		{name: "a refused far writer fails upward and still joins downward", up: 1024, down: 1024, refuseFar: true, wantUpErr: true},
		{name: "a far close failure is reported beside successful copies", up: 64, down: 64, closeErr: errRelayWrite},
		{name: "a near close failure is reported beside successful copies", up: 64, down: 64, nearCloseErr: io.ErrClosedPipe},
		{name: "both close failures remain independently reachable", up: 64, down: 64, closeErr: errRelayWrite, nearCloseErr: io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			upward, downward := relayBytes(row.up, 0x3c), relayBytes(row.down, 0xc3)
			var nearInput io.Reader = bytes.NewReader(upward)
			nearBuffer := &closeRecorder{closeErr: row.nearCloseErr}
			var nearOutput io.WriteCloser = nearBuffer
			var ownedEchoInput *io.PipeReader
			if row.echo {
				echoReader, echoWriter := io.Pipe()
				nearInput, nearOutput = io.LimitReader(echoReader, int64(row.down)), echoWriter
				ownedEchoInput = echoReader
				t.Cleanup(func() { _ = echoReader.Close(); _ = echoWriter.Close() })
				upward = downward
			}
			if row.refuseNear {
				nearOutput = refusingWriter{}
			}
			far := &closeRecorder{closeErr: row.closeErr}
			var farOutput io.WriteCloser = far
			refusingFar := &refusingCloser{}
			if row.refuseFar {
				farOutput = refusingFar
			}
			type outcome struct {
				result filestore.RelayResult
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				result, err := filestore.Relay(filestore.RelayRequest{NearInput: nearInput, NearOutput: nearOutput, FarInput: bytes.NewReader(downward), FarOutput: farOutput})
				done <- outcome{result, err}
			}()
			var got outcome
			backstop, release := newFilesystemBackstop(t.Context(), t, relayBackstop)
			defer release()
			select {
			case got = <-done:
			case <-backstop.Done():
				if ownedEchoInput != nil {
					_ = ownedEchoInput.Close()
				}
				_ = nearOutput.Close()
				joinBackstop, releaseJoin := newFilesystemBackstop(t.Context(), t, relayBackstop)
				defer releaseJoin()
				select {
				case <-done:
				case <-joinBackstop.Done():
					t.Fatalf("released Relay wait reached %v, want both owned copies joined", relayBackstop)
				}
				t.Fatalf("Relay got no return within %v, want both directions joined", relayBackstop)
			}
			if err := got.result.Validate(); err != nil {
				t.Fatalf("RelayResult.Validate() = %v, want a joined relay", err)
			}
			if (got.result.UpwardError != nil) != row.wantUpErr || row.wantUpErr && !errors.Is(got.result.UpwardError, errRelayWrite) {
				t.Fatalf("upward error = %v, want failure %t carrying the native cause", got.result.UpwardError, row.wantUpErr)
			}
			if (got.result.DownwardError != nil) != row.wantDownErr || row.wantDownErr && !errors.Is(got.result.DownwardError, errRelayWrite) {
				t.Fatalf("downward error = %v, want failure %t carrying the native cause", got.result.DownwardError, row.wantDownErr)
			}
			if row.closeErr != nil && !errors.Is(got.result.CloseError, row.closeErr) || row.nearCloseErr != nil && !errors.Is(got.result.CloseError, row.nearCloseErr) || row.closeErr == nil && row.nearCloseErr == nil && got.result.CloseError != nil {
				t.Fatalf("close error = %v, want %v", got.result.CloseError, row.closeErr)
			}
			if row.closeErr != nil && !errors.Is(got.err, row.closeErr) || row.nearCloseErr != nil && !errors.Is(got.err, row.nearCloseErr) {
				t.Fatalf("Relay error = %v, want both native close causes %v and %v reachable", got.err, row.closeErr, row.nearCloseErr)
			}
			wantErr := row.wantUpErr || row.wantDownErr || row.closeErr != nil || row.nearCloseErr != nil
			if (got.err != nil) != wantErr {
				t.Fatalf("Relay error = %v, want failure %t", got.err, wantErr)
			}
			// FarOutput is closed exactly once, after the upward copy.
			if closes := far.closes + refusingFar.closes; closes != 1 {
				t.Fatalf("FarOutput closed %d times, want exactly once", closes)
			}
			if !row.refuseFar && !bytes.Equal(far.Bytes(), upward) {
				t.Fatalf("upward bytes = %d, want the near input's %d bytes exactly", far.Len(), len(upward))
			}
			if !row.refuseNear && !row.echo && nearBuffer.closes != 1 {
				t.Fatalf("NearOutput closed %d times, want exactly once", nearBuffer.closes)
			}
			if !row.refuseNear && !row.echo && !bytes.Equal(nearBuffer.Bytes(), downward) {
				t.Fatalf("downward bytes = %d, want the far input's %d bytes exactly", nearBuffer.Len(), len(downward))
			}
		})
	}
}

// Every missing stream is refused before any copy, with a zero result; a
// zero result never claims a joined relay.
func TestRelayRequestRefusesMissingStreams(t *testing.T) {
	t.Parallel()
	full := func() filestore.RelayRequest {
		return filestore.RelayRequest{NearInput: bytes.NewReader(nil), NearOutput: &closeRecorder{}, FarInput: bytes.NewReader(nil), FarOutput: &closeRecorder{}}
	}
	for _, row := range []struct {
		name  string
		clear func(*filestore.RelayRequest)
	}{
		{name: "missing near input", clear: func(r *filestore.RelayRequest) { r.NearInput = nil }},
		{name: "missing near output", clear: func(r *filestore.RelayRequest) { r.NearOutput = nil }},
		{name: "missing far input", clear: func(r *filestore.RelayRequest) { r.FarInput = nil }},
		{name: "missing far output", clear: func(r *filestore.RelayRequest) { r.FarOutput = nil }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			request := full()
			row.clear(&request)
			got, err := filestore.Relay(request)
			if !errors.Is(err, core.ErrFilestoreContract) || got.Validate() == nil {
				t.Fatalf("Relay(%s) = %+v/%v, want an unjoined zero result and %v", row.name, got, err, core.ErrFilestoreContract)
			}
		})
	}
	if err := (filestore.RelayResult{}).Validate(); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("zero RelayResult.Validate() = %v, want %v", err, core.ErrFilestoreContract)
	}
}

// Arbitrary bytes in both directions arrive exactly, the relay joins, and
// FarOutput is closed exactly once; nothing else is claimed.
func FuzzRelayPreservesBothDirections(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte("near"), []byte("far"))
	f.Add(relayBytes(70000, 0x11), relayBytes(1, 0x22))
	f.Fuzz(func(t *testing.T, up, down []byte) {
		far := &closeRecorder{}
		near := &closeRecorder{}
		result, err := filestore.Relay(filestore.RelayRequest{NearInput: bytes.NewReader(up), NearOutput: near, FarInput: bytes.NewReader(down), FarOutput: far})
		if err != nil || result.Validate() != nil {
			t.Fatalf("Relay = %+v/%v, want a joined relay with no error", result, err)
		}
		if !bytes.Equal(far.Bytes(), up) || !bytes.Equal(near.Bytes(), down) || far.closes != 1 || near.closes != 1 {
			t.Fatalf("relayed up %d/%d down %d/%d closes %d, want exact bytes and one close", far.Len(), len(up), near.Len(), len(down), far.closes)
		}
	})
}
