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

// The near peer sends its final EOF only after receiving EOF from the far
// peer, as rsync's remote-shell protocol does. Closing NearOutput before
// joining upward is what breaks that dependency; payload equality proves the
// copies drained rather than merely returning early.
func TestRelayClosesDownwardBeforeJoiningDependentUpward(t *testing.T) {
	t.Parallel()
	nearInput, nearOutput := io.Pipe()
	far := &closeRecorder{}
	payload := relayBytes(256<<10, 0x9a)
	type outcome struct {
		result filestore.RelayResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := filestore.Relay(filestore.RelayRequest{NearInput: nearInput, NearOutput: nearOutput, FarInput: bytes.NewReader(payload), FarOutput: far})
		done <- outcome{result, err}
	}()
	select {
	case got := <-done:
		_ = nearInput.Close()
		if got.err != nil || got.result.Validate() != nil || far.closes != 1 || !bytes.Equal(far.Bytes(), payload) {
			t.Fatalf("dependent EOF relay = %+v/%v, bytes %d, closes %d; want joined, exact %d bytes and one close", got.result, got.err, far.Len(), far.closes, len(payload))
		}
	case <-time.After(relayBackstop):
		// Release and join the owned worker before reporting a failed backstop.
		_ = nearInput.Close()
		_ = nearOutput.Close()
		select {
		case <-done:
		case <-time.After(relayBackstop):
			t.Fatal("released relay did not join")
		}
		t.Fatal("downward EOF did not release dependent upward EOF")
	}
}

func TestRelayRefusesTypedNilBeforeAnyCopyOrClose(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		clear func(*filestore.RelayRequest)
	}{
		{name: "near input", clear: func(r *filestore.RelayRequest) { var absent *bytes.Reader; r.NearInput = absent }},
		{name: "near output", clear: func(r *filestore.RelayRequest) { var absent *closeRecorder; r.NearOutput = absent }},
		{name: "far input", clear: func(r *filestore.RelayRequest) { var absent *bytes.Reader; r.FarInput = absent }},
		{name: "far output", clear: func(r *filestore.RelayRequest) { var absent *closeRecorder; r.FarOutput = absent }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			near, far := &closeRecorder{}, &closeRecorder{}
			request := filestore.RelayRequest{NearInput: bytes.NewReader([]byte("up")), NearOutput: near, FarInput: bytes.NewReader([]byte("down")), FarOutput: far}
			row.clear(&request)
			if err := request.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("typed-nil admission = %v, want %v before execution", err, core.ErrFilestoreContract)
			}
			got, err := filestore.Relay(request)
			if !errors.Is(err, core.ErrFilestoreContract) || got != (filestore.RelayResult{}) || got.Validate() == nil || near.Len() != 0 || far.Len() != 0 || near.closes != 0 || far.closes != 0 {
				t.Fatalf("typed-nil relay = %+v/%v, writes %d/%d closes %d/%d; want zero refusal without effects", got, err, near.Len(), far.Len(), near.closes, far.closes)
			}
		})
	}
}
