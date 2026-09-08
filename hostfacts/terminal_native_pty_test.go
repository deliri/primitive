//go:build darwin || linux

package hostfacts

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/deliri/primitive/v2026/core"
)

func TestTerminalPTYObservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		columns        uint16
		wantAttachment TerminalAttachment
		wantColumnsErr error
	}{
		{name: "zero width preserves attachment without geometry", wantAttachment: TerminalAttachmentTerminalWithoutGeometry, wantColumnsErr: core.ErrHostFactsContract},
		{name: "minimum column remains exact", columns: 1, wantAttachment: TerminalAttachmentTerminal},
		{name: "nondefault width cannot be replaced by a conventional default", columns: 121, wantAttachment: TerminalAttachmentTerminal},
		{name: "maximum kernel width cannot narrow", columns: ^uint16(0), wantAttachment: TerminalAttachmentTerminal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			slave := openPseudoTerminalSlave(t)
			setTerminalColumns(t, slave, tc.columns)
			got, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
			if err != nil {
				t.Fatalf("PTY observation = %+v/%v, want nil error", got, err)
			}
			attachment, err := got.Attachment()
			if err != nil || attachment != tc.wantAttachment {
				t.Fatalf("PTY attachment = %v/%v, want %v", attachment, err, tc.wantAttachment)
			}
			columns, err := got.Columns()
			if !errors.Is(err, tc.wantColumnsErr) || uint16(columns) != tc.columns {
				t.Fatalf("PTY columns = %d/%v, want %d/%v", columns, err, tc.columns, tc.wantColumnsErr)
			}
			observed, err := unix.IoctlGetWinsize(int(slave.Fd()), unix.TIOCGWINSZ)
			if err != nil || observed == nil || observed.Col != tc.columns {
				t.Fatalf("post-observation kernel window = %+v/%v, want unchanged width %d", observed, err, tc.columns)
			}
		})
	}
}

// setTerminalColumns fixes the slave's window size so the observation under
// test reads back a width this test chose, not whatever the host had.
func setTerminalColumns(t *testing.T, slave *os.File, columns uint16) {
	t.Helper()
	window := unix.Winsize{Col: columns, Row: 40}
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &window); err != nil {
		t.Fatalf("set pty winsize: %v", err)
	}
}
