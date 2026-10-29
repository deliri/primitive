package process_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestOutputWriterCapabilityRefusesInvalidRoutesBeforeEffects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		stream    process.Stream
		nilStream process.Stream
		wantErr   error
	}{
		{name: "stdout lends only its admitted route", stream: process.StreamStdout},
		{name: "stderr lends only its admitted route", stream: process.StreamStderr},
		{name: "stdin is not an output route", stream: process.StreamStdin, wantErr: core.ErrProcessContract},
		{name: "unset route has no output authority", stream: process.StreamUnknown, wantErr: core.ErrProcessContract},
		{name: "future route has no output authority", stream: process.Stream(255), wantErr: core.ErrProcessContract},
		{name: "missing input invalidates lent streams", stream: process.StreamStdout, nilStream: process.StreamStdin, wantErr: core.ErrProcessContract},
		{name: "missing selected output invalidates lending", stream: process.StreamStdout, nilStream: process.StreamStdout, wantErr: core.ErrProcessContract},
		{name: "missing unselected output invalidates lending", stream: process.StreamStdout, nilStream: process.StreamStderr, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr := &outputOutcomeWriter{count: 1}, &outputOutcomeWriter{count: 1}
			streams := process.Streams{Stdin: strings.NewReader(""), Stdout: stdout, Stderr: stderr}
			switch tc.nilStream {
			case process.StreamStdin:
				streams.Stdin = nil
			case process.StreamStdout:
				streams.Stdout = nil
			case process.StreamStderr:
				streams.Stderr = nil
			}
			writer, gotErr := streams.OutputWriter(tc.stream)
			if !errors.Is(gotErr, tc.wantErr) || stdout.calls != 0 || stderr.calls != 0 {
				t.Fatalf("lending = %v, effects %d/%d; want %v with no effects", gotErr, stdout.calls, stderr.calls, tc.wantErr)
			}
			if tc.wantErr != nil {
				if writer != (process.OutputWriter{}) || !errors.Is(writer.Validate(), core.ErrProcessContract) {
					t.Fatalf("refused capability = %#v, want zero and typed invalidity", writer)
				}
				if n, err := writer.Write(nil); n != 0 || !errors.Is(err, core.ErrProcessContract) {
					t.Fatalf("zero capability empty write = %d/%v, want zero and contract refusal", n, err)
				}
				return
			}
			if err := writer.Validate(); err != nil {
				t.Fatal(err)
			}
			if n, err := writer.Write([]byte{'x'}); n != 1 || err != nil {
				t.Fatalf("lent route write = %d/%v, want one byte and nil", n, err)
			}
			selected, other := stdout, stderr
			if tc.stream == process.StreamStderr {
				selected, other = stderr, stdout
			}
			if selected.calls != 1 || selected.retained.String() != "x" || other.calls != 0 {
				t.Fatalf("routing = %d/%q, other %d, want one exact byte only at selected output", selected.calls, selected.retained.String(), other.calls)
			}
		})
	}
}

// All native three-byte count/error outcomes include impossible count values,
// successful prefixes, and full-count errors. Empty writes are separately inert.
func TestOutputWriterLayerTriadPreservesCountsAndCauses(t *testing.T) {
	t.Parallel()
	for _, stream := range []process.Stream{process.StreamStdout, process.StreamStderr} {
		for _, cause := range []error{nil, io.ErrClosedPipe} {
			for _, count := range []int{-1, 0, 1, 2, 3, 4} {
				t.Run(fmt.Sprintf("%s/count_%d/cause_%v", stream, count, cause), func(t *testing.T) {
					t.Parallel()
					payload := []byte{0, 255, '\n'}
					selected, other := &outputOutcomeWriter{count: count, cause: cause}, &outputOutcomeWriter{count: 3}
					streams := process.Streams{Stdin: bytes.NewReader(nil), Stdout: selected, Stderr: other}
					if stream == process.StreamStderr {
						streams.Stdout, streams.Stderr = other, selected
					}
					writer, err := streams.OutputWriter(stream)
					if err != nil {
						t.Fatal(err)
					}
					n, gotErr := writer.Write(payload)
					wantCount, wantNative := count, cause
					if count < 0 || count > len(payload) {
						wantCount, wantNative = 0, io.ErrShortWrite
					} else if cause == nil && count != len(payload) {
						wantNative = io.ErrShortWrite
					}
					var wantClass error
					if wantNative != nil {
						wantClass = core.ErrProcessStream
					}
					if n != wantCount || !errors.Is(gotErr, wantClass) || !errors.Is(gotErr, wantNative) || cause != nil && !errors.Is(gotErr, cause) {
						t.Fatalf("write = %d/%v, want %d/%v, class %v, original cause %v", n, gotErr, wantCount, wantNative, wantClass, cause)
					}
					if selected.calls != 1 || other.calls != 0 || other.retained.Len() != 0 || !bytes.Equal(selected.retained.Bytes(), payload[:wantCount]) {
						t.Fatalf("effects = %d/%x other %d/%x, want exact prefix %x once and no other output", selected.calls, selected.retained.Bytes(), other.calls, other.retained.Bytes(), payload[:wantCount])
					}
					for _, empty := range [][]byte{nil, {}} {
						if n, err := writer.Write(empty); n != 0 || err != nil || selected.calls != 1 || other.calls != 0 {
							t.Fatalf("empty write = %d/%v, calls %d/%d, want inert zero/nil", n, err, selected.calls, other.calls)
						}
					}
				})
			}
		}
	}
}

type outputWriterDigest struct {
	digest hash.Hash
	bytes  int
	calls  int
}

func (w *outputWriterDigest) Write(payload []byte) (int, error) {
	w.calls++
	n, err := w.digest.Write(payload)
	w.bytes += n
	return n, err
}

func TestOutputWriterStreamsAcrossFixedWindowsWithoutRetention(t *testing.T) {
	t.Parallel()
	const window = 32768
	for _, extent := range []int{0, window - 1, window, window + 1, 64*window + 7} {
		t.Run(fmt.Sprintf("extent_%d", extent), func(t *testing.T) {
			t.Parallel()
			var buffer [window]byte
			for index := range buffer {
				buffer[index] = byte(index)
			}
			selected := &outputWriterDigest{digest: sha256.New()}
			other := &outputWriterDigest{digest: sha256.New()}
			writer, err := (process.Streams{Stdin: strings.NewReader(""), Stdout: selected, Stderr: other}).OutputWriter(process.StreamStdout)
			if err != nil {
				t.Fatal(err)
			}
			want := sha256.New()
			wantCalls := 0
			for remaining := extent; remaining > 0; {
				size := min(remaining, len(buffer))
				if n, err := writer.Write(buffer[:size]); n != size || err != nil {
					t.Fatalf("window write = %d/%v, want %d/nil", n, err, size)
				}
				if _, err := want.Write(buffer[:size]); err != nil {
					t.Fatal(err)
				}
				remaining -= size
				wantCalls++
			}
			if selected.bytes != extent || selected.calls != wantCalls || other.calls != 0 || !bytes.Equal(selected.digest.Sum(nil), want.Sum(nil)) {
				t.Fatalf("streamed %d bytes/%d calls, other %d, digest %x; want %d/%d, no other writes, digest %x", selected.bytes, selected.calls, other.calls, selected.digest.Sum(nil), extent, wantCalls, want.Sum(nil))
			}
		})
	}
}

func TestOutputWriterNativeFlagDiagnosticsPreserveGrammarAndBytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantHelp bool
		wantFlag bool
	}{
		{name: "no flags leave default and no diagnostics"},
		{name: "boolean flag admits native grammar", args: []string{"--yes"}, wantFlag: true},
		{name: "explicit false preserves native boolean", args: []string{"-yes=false"}},
		{name: "help retains native ErrHelp and usage", args: []string{"-h"}, wantHelp: true},
		{name: "unknown flag retains native failure and usage", args: []string{"--unknown"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var output, other, reference bytes.Buffer
			writer, err := (process.Streams{Stdin: strings.NewReader(""), Stdout: &other, Stderr: &output}).OutputWriter(process.StreamStderr)
			if err != nil {
				t.Fatal(err)
			}
			flags := flag.NewFlagSet("native", flag.ContinueOnError)
			flags.SetOutput(writer)
			yes := flags.Bool("yes", false, "confirm")
			baseline := flag.NewFlagSet("native", flag.ContinueOnError)
			baseline.SetOutput(&reference)
			baselineYes := baseline.Bool("yes", false, "confirm")
			gotErr, nativeErr := flags.Parse(tc.args), baseline.Parse(tc.args)
			if (gotErr == nil) != (nativeErr == nil) || errors.Is(gotErr, flag.ErrHelp) != tc.wantHelp || *yes != tc.wantFlag || *yes != *baselineYes || !bytes.Equal(output.Bytes(), reference.Bytes()) || other.Len() != 0 {
				t.Fatalf("parse = %v/%t, output %q other %q; want native %v/%t/output %q", gotErr, *yes, output.Bytes(), other.Bytes(), nativeErr, *baselineYes, reference.Bytes())
			}
		})
	}
}

func FuzzOutputWriterSemanticClosure(f *testing.F) {
	var canonical bytes.Buffer
	writer, err := (process.Streams{Stdin: strings.NewReader(""), Stdout: &canonical, Stderr: io.Discard}).OutputWriter(process.StreamStdout)
	if err != nil {
		f.Fatal(err)
	}
	seed := []byte{0, 255, 'p', '\n'}
	if n, err := writer.Write(seed); n != len(seed) || err != nil || !bytes.Equal(canonical.Bytes(), seed) {
		f.Fatalf("canonical output = %d/%v/%x, want %d/nil/%x", n, err, canonical.Bytes(), len(seed), seed)
	}
	f.Add(uint8(process.StreamStdout), canonical.Bytes())
	f.Add(uint8(process.StreamStderr), []byte{})
	f.Add(uint8(process.StreamStdin), []byte("refused"))
	f.Add(uint8(255), []byte{0, 255})
	f.Fuzz(func(t *testing.T, raw uint8, payload []byte) {
		var stdout, stderr bytes.Buffer
		stream := process.Stream(raw)
		writer, gotErr := (process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}).OutputWriter(stream)
		if stream != process.StreamStdout && stream != process.StreamStderr {
			if !errors.Is(gotErr, core.ErrProcessContract) || writer != (process.OutputWriter{}) || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("refused route = %#v/%v, effects %d/%d, want zero capability and typed refusal", writer, gotErr, stdout.Len(), stderr.Len())
			}
			if n, err := writer.Write(payload); n != 0 || !errors.Is(err, core.ErrProcessContract) || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("refused capability write = %d/%v, effects %d/%d, want typed zero-effect refusal", n, err, stdout.Len(), stderr.Len())
			}
			return
		}
		if gotErr != nil || writer.Validate() != nil {
			t.Fatalf("admitted capability = %#v/%v, want validated output route", writer, gotErr)
		}
		selected, other := &stdout, &stderr
		if stream == process.StreamStderr {
			selected, other = &stderr, &stdout
		}
		for range 2 {
			selected.Reset()
			if n, err := writer.Write(payload); n != len(payload) || err != nil || !bytes.Equal(selected.Bytes(), payload) || other.Len() != 0 {
				t.Fatalf("output = %d/%v/%x, other %d, want exact payload %x at one route", n, err, selected.Bytes(), other.Len(), payload)
			}
		}
	})
}
