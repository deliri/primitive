package process_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestUncappedExecutionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, size := range []uint64{0, (1 << 20) + 1, (8 << 20) + 7} {
		t.Run("exact-stream-bytes-"+strconv.FormatUint(size, 10), func(t *testing.T) {
			t.Parallel()
			destination := sha256.New()
			request := processRequest(t, "output:"+strconv.FormatUint(size, 10), process.Streams{Stdin: bytes.NewReader(nil), Stdout: destination, Stderr: io.Discard})
			request.OutputPolicy = process.OutputPolicy{Mode: process.OutputModeStreaming}
			got, gotErr := process.Run(t.Context(), request)
			if gotErr != nil {
				t.Fatalf("Run(streaming) error = %v, want nil", gotErr)
			}
			observation, err := got.Observation()
			if err != nil || observation.ExitCode != 0 || observation.StdoutBytes.Uint64() != size || observation.StderrBytes.Uint64() != 0 {
				t.Fatalf("Observation() = (%+v, %v), want exit zero and %d exact stdout bytes", observation, err, size)
			}
			oracle := sha256.New()
			block := bytes.Repeat([]byte{'x'}, 32768)
			for remaining := size; remaining > 0; {
				n := min(remaining, uint64(len(block)))
				if _, err := oracle.Write(block[:n]); err != nil {
					t.Fatal(err)
				}
				remaining -= n
			}
			if !bytes.Equal(destination.Sum(nil), oracle.Sum(nil)) {
				t.Fatalf("stdout digest = %x, want %x", destination.Sum(nil), oracle.Sum(nil))
			}
		})
	}
	t.Run("destination failure remains typed under streaming", func(t *testing.T) {
		t.Parallel()
		request := processRequest(t, "output:64", process.Streams{Stdin: bytes.NewReader(nil), Stdout: failingWriter{err: io.ErrClosedPipe}, Stderr: io.Discard})
		request.OutputPolicy = process.OutputPolicy{Mode: process.OutputModeStreaming}
		_, err := process.Run(t.Context(), request)
		if !errors.Is(err, core.ErrProcessStream) || !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("Run(stream failure) = %v, want process stream and closed pipe identities", err)
		}
	})
}

func FuzzOutputPolicyExternalIngress(f *testing.F) {
	f.Add(uint8(process.OutputModeStreaming), uint64(0))
	f.Add(uint8(process.OutputModeBounded), uint64(1))
	f.Add(uint8(process.OutputModeBounded), uint64(math.MaxInt64))
	f.Add(uint8(process.OutputModeBounded), uint64(math.MaxInt64)+1)
	f.Fuzz(func(t *testing.T, mode uint8, extent uint64) {
		input := process.OutputPolicy{Mode: process.OutputMode(mode)}
		if extent != 0 {
			value, err := core.NewByteCount(extent)
			if err != nil {
				t.Fatal(err)
			}
			input.Maximum = value
		}
		before := input
		err := input.Validate()
		wantAccepted := (mode == uint8(process.OutputModeStreaming) && extent == 0) || (mode == uint8(process.OutputModeBounded) && extent > 0 && extent <= math.MaxInt64)
		if wantAccepted {
			if err != nil {
				t.Fatalf("policy (%d,%d) error = %v, want nil", mode, extent, err)
			}
		} else if !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("policy (%d,%d) error = %v, want process contract", mode, extent, err)
		}
		if input != before {
			t.Fatalf("policy after Validate = %+v, want %+v", input, before)
		}
	})
}

func FuzzOutputModeJSONSemanticClosure(f *testing.F) {
	for _, mode := range [...]process.OutputMode{process.OutputModeStreaming, process.OutputModeBounded} {
		seed, err := mode.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(seed)
	}
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := process.OutputModeStreaming
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrProcessContract) || got != process.OutputModeStreaming {
				t.Fatalf("refused mode = (%v,%v), want preserved streaming and typed rejection", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted mode.Validate = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip process.OutputMode
		if err := roundtrip.UnmarshalJSON(encoded); err != nil || roundtrip != got {
			t.Fatalf("roundtrip = (%v,%v), want (%v,nil)", roundtrip, err, got)
		}
		second, err := roundtrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("canonical bytes = (%q,%v), want (%q,nil)", second, err, encoded)
		}
	})
}
