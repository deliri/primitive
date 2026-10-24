package filestore

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestRelayCurrentProcessRefusesBeforeStarting(t *testing.T) {
	t.Parallel()
	for _, inherited := range [][]process.InheritedStream{nil, {process.InheritedStandardInput}, {process.InheritedStandardInput, process.InheritedStandardInput}, {process.InheritedStandardInput, process.InheritedStream(255)}} {
		err := RelayCurrentProcess(process.LentRequest{Inherited: inherited})
		if !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("terminal relay refusal = %v, want Filestore and Process contract identities without terminating", err)
		}
	}
	if err := relayCurrentProcess(process.Streams{}, process.Lent{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("missing native streams = %v, want %v before starting", err, core.ErrFilestoreContract)
	}
	var buffer bytes.Buffer
	own := process.Streams{Stdin: bytes.NewReader(nil), Stdout: &buffer, Stderr: io.Discard}
	if err := relayCurrentProcess(own, process.Lent{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("unowned stdout refusal = %v, want %v before starting", err, core.ErrFilestoreContract)
	}
	near := &currentRelayTestOutput{}
	own.Stdout = near
	if err := relayCurrentProcess(own, process.Lent{}); !errors.Is(err, core.ErrFilestoreContract) || near.Len() != 0 || near.closed {
		t.Fatalf("missing lent streams = %v, bytes %d, closed %t; want refusal without output effects", err, near.Len(), near.closed)
	}
}

type currentRelayTestOutput struct {
	bytes.Buffer
	closed bool
}

func (output *currentRelayTestOutput) Close() error {
	output.closed = true
	return nil
}

func FuzzCurrentRelayOutputConservesBytes(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("output\x00bytes"))
	f.Fuzz(func(t *testing.T, payload []byte) {
		var output bytes.Buffer
		stream := currentRelayOutput{streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: &output, Stderr: io.Discard}}
		n, err := stream.Write(payload)
		if err != nil || n != len(payload) || !bytes.Equal(output.Bytes(), payload) {
			t.Fatalf("native output projection = %d/%v, retained %d bytes; want exact %d input bytes", n, err, output.Len(), len(payload))
		}
	})
}
