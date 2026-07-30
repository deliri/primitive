package lease_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

type boundedWriter struct {
	err    error
	buffer bytes.Buffer
	limit  int
}

func (w *boundedWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.limit == 0 {
		return 0, nil
	}
	if len(data) > w.limit {
		data = data[:w.limit]
	}
	return w.buffer.Write(data)
}

func TestWriteCanonicalStandardWriterPressure(t *testing.T) {
	t.Parallel()

	subject := fixtureSubject(t, 121)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	canonical, err := decision.MarshalJSON()
	if err != nil {
		t.Fatalf("Decision.MarshalJSON() error = %v, want nil", err)
	}
	cases := []struct {
		wantErr   error
		writer    func() *boundedWriter
		name      string
		wantBytes bool
	}{
		{
			name: "complete standard writer",
			writer: func() *boundedWriter {
				return &boundedWriter{limit: len(canonical)}
			},
			wantBytes: true,
		},
		{
			name: "one byte standard short write is refused",
			writer: func() *boundedWriter {
				return &boundedWriter{limit: 1}
			},
			wantErr: io.ErrShortWrite,
		},
		{
			name: "seven byte standard short write is refused",
			writer: func() *boundedWriter {
				return &boundedWriter{limit: 7}
			},
			wantErr: io.ErrShortWrite,
		},
		{
			name: "zero progress is refused",
			writer: func() *boundedWriter {
				return &boundedWriter{}
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "native destination error remains reachable",
			writer: func() *boundedWriter {
				return &boundedWriter{limit: len(canonical), err: io.ErrClosedPipe}
			},
			wantErr: io.ErrClosedPipe,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			writer := tc.writer()
			err := decision.WriteCanonical(writer)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Decision.WriteCanonical() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantBytes && !bytes.Equal(writer.buffer.Bytes(), canonical) {
				t.Fatalf("canonical writer bytes differ from Decision.MarshalJSON()")
			}
		})
	}
}

func TestWriteCanonicalRejectsBeforeWriting(t *testing.T) {
	t.Parallel()

	writer := &boundedWriter{limit: 1}
	err := (lease.Decision{}).WriteCanonical(writer)
	if !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.WriteCanonical() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if writer.buffer.Len() != 0 {
		t.Fatalf("invalid decision wrote %d bytes, want 0", writer.buffer.Len())
	}
}

func TestWriteCanonicalRejectsNilDestination(t *testing.T) {
	t.Parallel()

	decision := fixtureGrantDecision(
		t, fixtureSubject(t, 122), 1, 1_000, fixtureGrant(),
	)
	err := decision.WriteCanonical(nil)
	if !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.WriteCanonical() error = %v, want %v", err, core.ErrLeaseContract)
	}
}
