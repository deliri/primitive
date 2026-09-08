//go:build unix

package hostfacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type descriptorFixtureKind uint8

const (
	descriptorFixturePipe descriptorFixtureKind = iota
	descriptorFixtureNull
	descriptorFixtureRegular
)

func TestTerminalDescriptorObservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		kind    descriptorFixtureKind
		closed  bool
		wantErr error
	}{
		{name: "pipe remains detached and owned by caller", kind: descriptorFixturePipe},
		{name: "null device remains detached and owned by caller", kind: descriptorFixtureNull},
		{name: "regular file remains detached and owned by caller", kind: descriptorFixtureRegular},
		{name: "closed descriptor cannot become detached evidence", kind: descriptorFixturePipe, closed: true, wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			var file *os.File
			var err error
			switch tc.kind {
			case descriptorFixturePipe:
				var writer *os.File
				file, writer, err = os.Pipe()
				if err == nil {
					t.Cleanup(func() { _ = writer.Close() })
				}
			case descriptorFixtureNull:
				file, err = os.Open(os.DevNull)
			case descriptorFixtureRegular:
				file, err = os.Create(filepath.Join(root, "plain"))
			default:
				t.Fatalf("descriptor kind = %d, want declared fixture", tc.kind)
			}
			if err != nil {
				t.Fatalf("descriptor fixture = %v, want nil", err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if tc.closed {
				if err := file.Close(); err != nil {
					t.Fatalf("fixture close = %v, want nil", err)
				}
			}
			got, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: file})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("descriptor observation = %+v/%v, want %v", got, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				failure, ok := errors.AsType[Failure](err)
				if got != (TerminalGeometry{}) || !ok || failure.Operation != OperationTerminalGeometry || failure.Cause == nil {
					t.Fatalf("refused descriptor = %+v/%v, want zero with exact operation and native cause", got, err)
				}
				return
			}
			attachment, err := got.Attachment()
			if err != nil || attachment != TerminalAttachmentNotTerminal {
				t.Fatalf("attachment = %v/%v, want detached", attachment, err)
			}
			columns, err := got.Columns()
			if !errors.Is(err, core.ErrHostFactsContract) || columns != 0 {
				t.Fatalf("columns = %d/%v, want zero typed absence", columns, err)
			}
			if _, err := file.Stat(); err != nil {
				t.Fatalf("caller-owned descriptor Stat = %v, want still open", err)
			}
		})
	}
}
