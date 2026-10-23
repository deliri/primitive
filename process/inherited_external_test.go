package process_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
)

// relayLending names how a relay parent lends its standard streams.
type relayLending uint8

const (
	relayLendInputOutput relayLending = iota + 1
	relayLendOutputInput
	relayLendNothing
	// relayLendEcho lends input then output, and feeds the child's own
	// standard output back as its standard input: upward bytes exist only
	// once downward bytes flowed, as in rsync's interleaved protocol.
	relayLendEcho
)

func (l relayLending) selector() string {
	return [...]string{relayLendInputOutput: "in-out", relayLendOutputInput: "out-in", relayLendNothing: "none", relayLendEcho: "echo"}[l]
}

func relayLendingOf(selector string) relayLending {
	for _, l := range []relayLending{relayLendInputOutput, relayLendOutputInput, relayLendNothing, relayLendEcho} {
		if l.selector() == selector {
			return l
		}
	}
	return 0
}

// streams returns what the parent lends and what the child believes it got.
func (l relayLending) streams() (lent, believed []process.InheritedStream) {
	inOut := []process.InheritedStream{process.InheritedStandardInput, process.InheritedStandardOutput}
	switch l {
	case relayLendOutputInput:
		outIn := []process.InheritedStream{process.InheritedStandardOutput, process.InheritedStandardInput}
		return outIn, outIn
	case relayLendNothing:
		return nil, inOut
	default:
		return inOut, inOut
	}
}

// relayPayload is a deterministic, position-dependent byte pattern, so a
// truncated, reordered or interleaved copy cannot match it.
func relayPayload(size int, seed byte) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(i*31) ^ seed
	}
	return payload
}

const (
	relaySessionSeed = 0x5a
	relayChildSeed   = 0xa5
	// relayChildRefused is the child's exit when its relay fails on a closed
	// descriptor with the native bad-descriptor cause; any other failure
	// exits differently.
	relayChildRefused = 96
)

// helperRelayParent plays the SSH session's process: it lends its own
// standard streams to a relay child whose own stdin carries a payload, then
// reports what the child wrote back on its stderr. Its stdout is the lent
// output, so the child's upward copy reaches the test directly.
func helperRelayParent(t *testing.T, selector string, arguments []string) {
	t.Helper()
	lending := relayLendingOf(selector)
	size, err := strconv.Atoi(arguments[0])
	if lending == 0 || err != nil {
		os.Exit(93)
	}
	lent, believed := lending.streams()
	var fromChild bytes.Buffer
	streams := process.Streams{Stdin: bytes.NewReader(relayPayload(size, relayChildSeed)), Stdout: &fromChild, Stderr: io.Discard}
	if lending == relayLendEcho {
		// The child's stdin ends after exactly the bytes it relayed down.
		echo, echoWriter := io.Pipe()
		streams = process.Streams{Stdin: io.LimitReader(echo, int64(size)), Stdout: echoWriter, Stderr: io.Discard}
	}
	request := processRequest(t, "relay-child:"+relayBelief(believed), streams)
	request.Inherited = lent
	result, runErr := process.Run(context.Background(), request)
	observation, observeErr := result.Observation()
	if runErr != nil || observeErr != nil {
		os.Exit(94)
	}
	if lending == relayLendNothing {
		fmt.Fprintf(os.Stderr, "child-exit:%d", observation.ExitCode)
		return
	}
	if observation.ExitCode != 0 {
		os.Exit(92)
	}
	if lending == relayLendEcho {
		return
	}
	if _, err := os.Stderr.Write(fromChild.Bytes()); err != nil {
		os.Exit(95)
	}
}

func relayBelief(streams []process.InheritedStream) string {
	if streams[0] == process.InheritedStandardOutput {
		return relayLendOutputInput.selector()
	}
	return relayLendInputOutput.selector()
}

// helperRelayChild is the bridge, exactly as a consumer composes it: its own
// standard streams, its parent's lent ones, and filestore's relay.
func helperRelayChild(selector string) {
	_, believed := relayLendingOf(selector).streams()
	own, err := process.StandardStreams()
	if err != nil {
		os.Exit(98)
	}
	lent, err := process.LentStreams(process.LentRequest{Inherited: believed})
	if err != nil {
		os.Exit(97)
	}
	defer func() { _ = lent.Input.Close() }()
	_, err = filestore.Relay(filestore.RelayRequest{NearInput: own.Stdin, NearOutput: own.Stdout, FarInput: lent.Input, FarOutput: lent.Output})
	if errors.Is(err, syscall.EBADF) {
		os.Exit(relayChildRefused)
	}
	if err != nil {
		os.Exit(99)
	}
}

// A relay child joins its own standard streams to its parent's lent ones
// (process.LentStreams with filestore.Relay), exactly as an rsync
// remote-shell bridge reaches the SSH session around it:
// the child's stdin reaches the parent's stdout, and the parent's stdin
// reaches the child's stdout. Each row runs three real processes through
// process.Run: this test, the lending parent and the relay child.
func TestInheritedStreamRelayLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		lending relayLending
		// size is each direction's payload; both directions run at once.
		size int
		// wantRefused expects the child's relay refusal, reported by the parent.
		wantRefused bool
	}{
		{name: "lent input then output relays both directions", lending: relayLendInputOutput, size: 4096},
		{name: "lent output then input maps descriptors by request order", lending: relayLendOutputInput, size: 4096},
		{name: "half a megabyte each way exceeds any pipe buffer intact", lending: relayLendInputOutput, size: 512 << 10},
		// A relay that copied one direction before the other would wait
		// forever here: the upward bytes are the downward bytes echoed.
		{name: "upward bytes that depend on downward bytes need both copies at once", lending: relayLendEcho, size: 256 << 10},
		{name: "empty streams on both sides end cleanly with no bytes", lending: relayLendInputOutput, size: 0},
		{name: "a child whose parent lent nothing fails on the closed descriptors", lending: relayLendNothing, size: 16, wantRefused: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			session := relayPayload(row.size, relaySessionSeed)
			request := processRequest(t, "relay-parent:"+row.lending.selector(), process.Streams{Stdin: bytes.NewReader(session), Stdout: &stdout, Stderr: &stderr})
			request = withArguments(t, request, strconv.Itoa(row.size))
			ctx, cancel := context.WithTimeout(context.Background(), processTestBackstop)
			defer cancel()
			result, err := process.Run(ctx, request)
			if err != nil {
				t.Fatalf("relay parent run error = %v, want nil; stderr %q", err, truncated(stderr.Bytes()))
			}
			observation, err := result.Observation()
			if err != nil || observation.ExitCode != 0 {
				t.Fatalf("relay parent exit = %+v/%v, want 0; stderr %q", observation, err, truncated(stderr.Bytes()))
			}
			if row.wantRefused {
				if got, want := stderr.String(), fmt.Sprintf("child-exit:%d", relayChildRefused); got != want {
					t.Fatalf("parent report = %q, want %q (relay failed with the native bad-descriptor cause)", got, want)
				}
				if stdout.Len() != 0 {
					t.Fatalf("unlent relay wrote %d bytes upward, want none", stdout.Len())
				}
				return
			}
			if row.lending == relayLendEcho {
				if !bytes.Equal(stdout.Bytes(), session) || stderr.Len() != 0 {
					t.Fatalf("echoed upward bytes = %d (stderr %d), want the session's %d bytes returned exactly", stdout.Len(), stderr.Len(), len(session))
				}
				return
			}
			if want := relayPayload(row.size, relayChildSeed); !bytes.Equal(stdout.Bytes(), want) {
				t.Fatalf("upward bytes = %d, want the child's %d-byte stdin exactly", stdout.Len(), len(want))
			}
			if !bytes.Equal(stderr.Bytes(), session) {
				t.Fatalf("downward bytes = %d, want the session's %d-byte stdin exactly", stderr.Len(), len(session))
			}
		})
	}
}

func truncated(data []byte) string {
	text := string(data)
	if len(text) > 200 {
		return text[:200]
	}
	return text
}

// The inherited-stream domain is exhaustive over its byte representation:
// only the two standard streams are admitted, each labelled, and every other
// value is refused with the process contract identity.
func TestInheritedStreamDomain(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		stream := process.InheritedStream(value)
		want := value == int(process.InheritedStandardInput) || value == int(process.InheritedStandardOutput)
		if stream.IsValid() != want {
			t.Fatalf("InheritedStream(%d).IsValid() = %t, want %t", value, stream.IsValid(), want)
		}
		err := stream.Validate()
		if want && err != nil || !want && !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("InheritedStream(%d).Validate() = %v, want admitted %t", value, err, want)
		}
		if label := stream.String(); want == (label == core.UnknownEnumDiagnostic) {
			t.Fatalf("InheritedStream(%d).String() = %q, want a label only when admitted", value, label)
		}
	}
}

// A request lends each standard stream at most once, in any order; nothing
// lent is the neutral default. Opening lent streams needs exactly both.
func TestInheritedRequestLayerTriad(t *testing.T) {
	t.Parallel()
	in, out := process.InheritedStandardInput, process.InheritedStandardOutput
	for _, row := range []struct {
		name      string
		inherited []process.InheritedStream
		wantRun   bool
		wantLent  bool
	}{
		{name: "nothing lent is the neutral default", wantRun: true},
		{name: "standard input alone", inherited: []process.InheritedStream{in}, wantRun: true},
		{name: "standard output alone", inherited: []process.InheritedStream{out}, wantRun: true},
		{name: "input then output", inherited: []process.InheritedStream{in, out}, wantRun: true, wantLent: true},
		{name: "output then input", inherited: []process.InheritedStream{out, in}, wantRun: true, wantLent: true},
		{name: "the unknown zero stream is refused", inherited: []process.InheritedStream{0}},
		{name: "a future stream value is refused", inherited: []process.InheritedStream{3}},
		{name: "the same stream lent twice is refused", inherited: []process.InheritedStream{in, in}},
		{name: "one more than the standard streams is refused", inherited: []process.InheritedStream{in, out, in}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			request := processRequest(t, "silent", process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
			request.Inherited = row.inherited
			if err := request.Validate(); row.wantRun && err != nil || !row.wantRun && !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Request.Validate() = %v, want admitted %t", err, row.wantRun)
			}
			lent := process.LentRequest{Inherited: row.inherited}
			if err := lent.Validate(); row.wantLent && err != nil || !row.wantLent && !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("LentRequest.Validate() = %v, want admitted %t", err, row.wantLent)
			}
			// A refused request never opens a descriptor.
			if !row.wantLent {
				if got, err := process.LentStreams(lent); !errors.Is(err, core.ErrProcessContract) || got != (process.Lent{}) {
					t.Fatalf("LentStreams(%v) = %+v/%v, want zero and the process contract refusal", row.inherited, got, err)
				}
			}
		})
	}
}
