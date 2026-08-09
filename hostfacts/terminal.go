package hostfacts

import (
	"errors"
	"os"
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// TerminalColumns is the column count of one attached terminal.
//
// The value is bounded by the kernel's window-size structure, which reports
// columns as an unsigned 16-bit integer on every supported platform, so the
// type carries exactly the domain the observation can produce. Zero is not a
// column count: a terminal reporting zero columns has no usable geometry and
// is reported as exactly that rather than as a zero-width terminal a renderer
// would divide by.
type TerminalColumns uint16

// Validate rejects the zero column count.
func (c TerminalColumns) Validate() error {
	if c == 0 {
		return errors.Join(core.ErrHostFactsContract, errors.New("terminal columns are zero"))
	}
	return nil
}

// IsValid reports whether c is a usable column count.
func (c TerminalColumns) IsValid() bool { return c.Validate() == nil }

// String renders the column count for diagnostics.
func (c TerminalColumns) String() string {
	if !c.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return strconv.FormatUint(uint64(c), 10)
}

// OffWireEnum declares TerminalColumns as an in-process observation that is
// never serialized: a renderer reads the width and draws, and nothing durable
// or wire-bound carries it.
func (TerminalColumns) OffWireEnum() {}

// TerminalAttachment is the closed set of things one interrogated descriptor
// can turn out to be.
//
// Closed rather than a boolean because the zero value must be rejectable: a
// geometry whose attachment was never observed is not the same fact as a
// descriptor that was observed to be detached, and a bool would collapse the
// two.
type TerminalAttachment uint8

const (
	// TerminalAttachmentUnknown is the unset attachment and describes nothing.
	TerminalAttachmentUnknown TerminalAttachment = iota
	// TerminalAttachmentTerminal means the descriptor answered the geometry
	// question: it is attached to a terminal with a usable column count.
	TerminalAttachmentTerminal
	// TerminalAttachmentNotTerminal means the descriptor is not attached to a
	// terminal at all: it is a pipe, a regular file, or a device that is not a
	// terminal. The kernel named that case itself, so nobody who asked for a
	// width should also be told a terminal was there.
	TerminalAttachmentNotTerminal
	// TerminalAttachmentTerminalWithoutGeometry means the descriptor answered
	// as a terminal but reported no usable width. Attachment was positively
	// observed; geometry was not. A fresh pseudo terminal reports zero columns
	// until someone sets its window size, and recording that as "not a
	// terminal" would be a detachment nobody observed: a caller deciding
	// whether it may draw interactively must see the terminal, and only the
	// width question stays unanswered.
	TerminalAttachmentTerminalWithoutGeometry
	terminalAttachmentLimit
)

func terminalAttachmentLabels() [terminalAttachmentLimit]string {
	return [...]string{
		TerminalAttachmentTerminal:                "terminal",
		TerminalAttachmentNotTerminal:             "not a terminal",
		TerminalAttachmentTerminalWithoutGeometry: "terminal without geometry",
	}
}

// Validate rejects the unset attachment and every value outside the closed set.
func (a TerminalAttachment) Validate() error {
	if !a.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("terminal attachment is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed attachment domain.
func (a TerminalAttachment) IsValid() bool {
	return a > TerminalAttachmentUnknown && a < terminalAttachmentLimit && terminalAttachmentLabels()[a] != ""
}

// String names the attachment for diagnostics.
func (a TerminalAttachment) String() string {
	if !a.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return terminalAttachmentLabels()[a]
}

// OffWireEnum declares TerminalAttachment as an in-process observation
// vocabulary. It names what a caller found on this machine and is never
// serialized.
func (TerminalAttachment) OffWireEnum() {}

// TerminalGeometryRequest names the open file whose descriptor is
// interrogated.
type TerminalGeometryRequest struct {
	File *os.File
}

// Validate rejects a request naming no file.
func (r TerminalGeometryRequest) Validate() error {
	if r.File == nil {
		return errors.Join(core.ErrHostFactsContract, errors.New("terminal geometry request names no file"))
	}
	return nil
}

// TerminalGeometry is one observed descriptor. Its fields are unexported and
// reachable only through accessors that revalidate, so a caller cannot
// assemble an observation it never made.
type TerminalGeometry struct {
	columns    TerminalColumns
	attachment TerminalAttachment
}

// terminalWithoutGeometryDiagnostic is the one spelling of the columns
// refusal, shared by the geometry gate and the column read.
const terminalWithoutGeometryDiagnostic = "only an attached terminal with geometry carries columns"

// Validate rejects a geometry whose attachment and column count disagree.
func (g TerminalGeometry) Validate() error {
	if err := g.attachment.Validate(); err != nil {
		return err
	}
	if g.attachment == TerminalAttachmentTerminal {
		return g.columns.Validate()
	}
	if g.columns != 0 {
		return errors.Join(core.ErrHostFactsContract, errors.New(terminalWithoutGeometryDiagnostic))
	}
	return nil
}

// Attachment returns what the descriptor turned out to be.
func (g TerminalGeometry) Attachment() (TerminalAttachment, error) {
	if err := g.Validate(); err != nil {
		return TerminalAttachmentUnknown, err
	}
	return g.attachment, nil
}

// Columns returns the attached terminal's column count.
//
// Only an attached terminal with geometry has one. A detached descriptor and a
// terminal that reported no usable width are both refused rather than answered
// with zero, which a renderer would treat as a real width.
func (g TerminalGeometry) Columns() (TerminalColumns, error) {
	attachment, err := g.Attachment()
	if err != nil {
		return 0, err
	}
	if attachment != TerminalAttachmentTerminal {
		return 0, errors.Join(core.ErrHostFactsContract, errors.New(terminalWithoutGeometryDiagnostic))
	}
	return g.columns, nil
}

func newAttachedTerminalGeometry(columns TerminalColumns) (TerminalGeometry, error) {
	geometry := TerminalGeometry{attachment: TerminalAttachmentTerminal, columns: columns}
	return geometry, geometry.Validate()
}

func newDetachedTerminalGeometry() (TerminalGeometry, error) {
	geometry := TerminalGeometry{attachment: TerminalAttachmentNotTerminal}
	return geometry, geometry.Validate()
}

func newTerminalWithoutGeometry() (TerminalGeometry, error) {
	geometry := TerminalGeometry{attachment: TerminalAttachmentTerminalWithoutGeometry}
	return geometry, geometry.Validate()
}
