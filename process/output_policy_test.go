package process

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"math"
	"testing"
)

func TestOutputPolicyLayerTriadExhaustsModeAndExtentPartitions(t *testing.T) {
	t.Parallel()
	one, err := core.NewByteCount(1)
	if err != nil {
		t.Fatal(err)
	}
	maximum, err := core.NewByteCount(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	overflow, err := core.NewByteCount(uint64(math.MaxInt64) + 1)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		input   OutputPolicy
		wantErr error
	}{
		{name: "explicit streaming needs no extent", input: OutputPolicy{Mode: OutputModeStreaming}},
		{name: "bounded minimum remains exact", input: OutputPolicy{Mode: OutputModeBounded, Maximum: one}},
		{name: "bounded maximum reportable extent remains exact", input: OutputPolicy{Mode: OutputModeBounded, Maximum: maximum}},
		{name: "zero mode cannot silently choose streaming", wantErr: core.ErrProcessContract},
		{name: "future mode rejected", input: OutputPolicy{Mode: OutputModeBounded + 1}, wantErr: core.ErrProcessContract},
		{name: "streaming rejects an ignored positive bound", input: OutputPolicy{Mode: OutputModeStreaming, Maximum: one}, wantErr: core.ErrProcessContract},
		{name: "bounded mode requires its extent", input: OutputPolicy{Mode: OutputModeBounded}, wantErr: core.ErrProcessContract},
		{name: "unreportable bound rejected before execution", input: OutputPolicy{Mode: OutputModeBounded, Maximum: overflow}, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.input.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("OutputPolicy.Validate() = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}
