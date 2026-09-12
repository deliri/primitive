//go:build unix

package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const groupLifecycleMaximumPayloadBytes = 1024

func TestGroupLivenessNativeLayerTriadOwnsObservationAndReaping(t *testing.T) {
	t.Parallel()
	t.Run("direct child cannot manufacture a group observation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
		defer cancel()
		request := processRequest(t, "copy", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
		execution, err := process.Begin(ctx, request)
		if err != nil {
			t.Fatalf("Begin direct child error = %v, want nil", err)
		}
		got, gotErr := execution.GroupLiveness()
		_, waitErr := execution.Wait()
		if got != process.LivenessUnknown || !errors.Is(gotErr, core.ErrProcessContract) || waitErr != nil {
			t.Fatalf("direct GroupLiveness = (%v, %v), Wait = %v, want (%v, %v), nil", got, gotErr, waitErr, process.LivenessUnknown, core.ErrProcessContract)
		}
	})
	t.Run("owned group exists before EOF and is absent after reap", func(t *testing.T) {
		t.Parallel()
		proveGroupLifecycle(t, []byte("owned input\n"))
	})
	for _, tc := range []struct {
		name   string
		handle *process.Execution
	}{
		{name: "nil capability exposes no observation"},
		{name: "unstarted capability exposes no observation", handle: new(process.Execution)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := tc.handle.GroupLiveness()
			if got != process.LivenessUnknown || !errors.Is(gotErr, core.ErrProcessContract) {
				t.Fatalf("GroupLiveness = (%v, %v), want (%v, %v)", got, gotErr, process.LivenessUnknown, core.ErrProcessContract)
			}
		})
	}
}

// The payload reaches the actual helper process. Kernel facts are independently
// pinned before input EOF and after reaping; output proves the child did work.
func FuzzGroupLivenessNativeLifecycle(f *testing.F) {
	var seed bytes.Buffer
	streams := process.Streams{Stdin: bytes.NewReader(nil), Stdout: &seed, Stderr: io.Discard}
	if _, err := streams.WriteOutput(process.StreamStdout, []byte{'x', 0, 0xff}); err != nil {
		f.Fatalf("typed seed WriteOutput error = %v, want nil", err)
	}
	f.Add(seed.Bytes())
	f.Fuzz(func(t *testing.T, payload []byte) {
		// Group observation accepts no arbitrary byte document. This bounds
		// the companion echo workload, not a production parser's admission.
		payload = payload[:min(len(payload), groupLifecycleMaximumPayloadBytes)]
		proveGroupLifecycle(t, payload)
	})
}

func proveGroupLifecycle(t *testing.T, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	var stdout bytes.Buffer
	request := processRequest(t, "copy", process.Streams{Stdin: reader, Stdout: &stdout, Stderr: io.Discard})
	request.Containment.Isolation = process.IsolationGroup
	if err := request.Validate(); err != nil {
		t.Fatalf("native request.Validate error = %v, want nil", err)
	}
	execution, err := process.Begin(ctx, request)
	if err != nil {
		t.Fatalf("Begin error = %v, want nil", err)
	}
	waited := false
	defer func() {
		if !waited {
			cancel()
			_ = writer.Close()
			if err := execution.Sweep(); err != nil {
				t.Errorf("cleanup Sweep error = %v, want nil", err)
			}
			_, err := execution.Wait()
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("cleanup Wait error = %v, want nil or cancellation", err)
			}
		}
	}()
	got, gotErr := execution.GroupLiveness()
	if got != process.LivenessAlive || gotErr != nil {
		t.Fatalf("before EOF GroupLiveness = (%v, %v), want (%v, nil)", got, gotErr, process.LivenessAlive)
	}
	if n, err := writer.Write(payload); err != nil || n != len(payload) {
		t.Fatalf("input Write = (%d, %v), want (%d, nil)", n, err, len(payload))
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("input Close error = %v, want nil", err)
	}
	_, err = execution.Wait()
	waited = true
	if err != nil {
		t.Fatalf("Wait error = %v, want nil", err)
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Fatalf("child output = %q, want %q", stdout.Bytes(), payload)
	}
	for range 2 {
		got, gotErr = execution.GroupLiveness()
		if got != process.LivenessGone || gotErr != nil {
			t.Fatalf("after reap GroupLiveness = (%v, %v), want (%v, nil)", got, gotErr, process.LivenessGone)
		}
	}
}
