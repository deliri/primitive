package main

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type commandOutputScenario struct {
	destination io.Writer
	observed    func() string
}

type refusingCommandWriter struct{ cause error }

func (w refusingCommandWriter) Write([]byte) (int, error) { return 0, w.cause }

type shortCommandWriter struct{ buffer *bytes.Buffer }

func (w shortCommandWriter) Write(value []byte) (int, error) {
	count, err := w.buffer.Write(value[:len(value)-1])
	return count, err
}

func TestCommandJSONOutputPinsExactBytesAndResourceFailures(t *testing.T) {
	t.Parallel()
	document := currentSchema()
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("schema MarshalJSON() error = %v, want nil", err)
	}
	wantComplete := string(append(encoded, '\n'))
	resourceErr := errors.New("writer refused output")
	cases := []struct {
		wantErr   error
		wantCause error
		build     func() commandOutputScenario
		name      string
		wantText  string
	}{
		{
			name: "positive writer receives one canonical document and newline",
			build: func() commandOutputScenario {
				var output bytes.Buffer
				return commandOutputScenario{destination: &output, observed: output.String}
			},
			wantText: wantComplete,
		},
		{
			name: "negative nil writer returns typed refusal without output",
			build: func() commandOutputScenario {
				return commandOutputScenario{observed: func() string { return "" }}
			},
			wantErr: core.ErrTaskManagerContract,
		},
		{
			name: "negative writer error preserves native cause and emits no bytes",
			build: func() commandOutputScenario {
				return commandOutputScenario{
					destination: refusingCommandWriter{cause: resourceErr},
					observed:    func() string { return "" },
				}
			},
			wantErr: core.ErrTaskManagerContract, wantCause: resourceErr,
		},
		{
			name: "negative short writer reports typed short write and exact partial bytes",
			build: func() commandOutputScenario {
				var output bytes.Buffer
				return commandOutputScenario{
					destination: shortCommandWriter{buffer: &output}, observed: output.String,
				}
			},
			wantText: wantComplete[:len(wantComplete)-1],
			wantErr:  core.ErrTaskManagerContract, wantCause: io.ErrShortWrite,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scenario := tc.build()
			gotErr := writeJSON(scenario.destination, document)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("writeJSON() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
				t.Fatalf("writeJSON() native error = %v, want %v", gotErr, tc.wantCause)
			}
			if gotText := scenario.observed(); gotText != tc.wantText {
				t.Fatalf("writeJSON() output = %q, want %q", gotText, tc.wantText)
			}
		})
	}
}
