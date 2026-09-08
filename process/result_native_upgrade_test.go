package process_test

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// Missing optional resource accounting must not discard a reaped child's exit
// or stream facts. A replay of the unsupported RSS leaf makes that seam red on
// a Unix host without claiming Windows runtime execution.
func TestReapedResultCaptureLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		exit    int
		invalid bool
		wantErr error
	}{
		{name: "neutral/silent zero exit remains a real observation"},
		{name: "positive/nonzero exit is an exact observation", exit: 7},
		{name: "boundary/maximum portable exit remains exact", exit: 255},
		{name: "negative/invalid command cannot produce a result", invalid: true, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			request := processRequest(t, "exit:"+strconv.Itoa(tc.exit), process.Streams{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr})
			if tc.invalid {
				request.Command = core.AbsolutePath{}
			}
			got, err := process.Run(t.Context(), request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Run exit capture error = %v, want %v", err, tc.wantErr)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("silent child emitted stdout=%d stderr=%d", stdout.Len(), stderr.Len())
			}
			if tc.wantErr != nil {
				if got != (process.Result{}) {
					t.Fatalf("refusal exposed a result: %+v", got)
				}
				return
			}
			observation, observationErr := got.Observation()
			if observationErr != nil || int64(observation.ExitCode) != int64(tc.exit) || observation.StdinBytes.Uint64() != 0 || observation.StdoutBytes.Uint64() != 0 || observation.StderrBytes.Uint64() != 0 {
				t.Fatalf("reaped facts = %+v, error=%v, want exit=%d with zero streams", observation, observationErr, tc.exit)
			}
		})
	}
}
