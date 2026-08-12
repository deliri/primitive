package hostfacts

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestTerminalColumnsAdmitOnlyUsableWidths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		columns TerminalColumns
		wantErr error
	}{
		{name: "zero columns are rejected", columns: 0, wantErr: core.ErrHostFactsContract},
		{name: "one column is the smallest usable width", columns: 1},
		{name: "two columns sit one above the floor", columns: 2},
		{name: "seventy nine columns sit one below the classic terminal", columns: 79},
		{name: "eighty columns is the classic terminal", columns: 80},
		{name: "eighty one columns sit one above the classic terminal", columns: 81},
		{name: "the uint16 midpoint is a usable width", columns: math.MaxUint16 / 2},
		{name: "two below the uint16 ceiling is a usable width", columns: math.MaxUint16 - 2},
		{name: "the uint16 ceiling is a usable width", columns: math.MaxUint16},
		{name: "one below the ceiling is a usable width", columns: math.MaxUint16 - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.columns.Validate()
			if tc.wantErr == nil && err != nil {
				t.Fatalf("TerminalColumns(%d).Validate() error = %v, want nil", tc.columns, err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("TerminalColumns(%d).Validate() error = %v, want errors.Is %v", tc.columns, err, tc.wantErr)
			}
			if got, want := tc.columns.IsValid(), tc.wantErr == nil; got != want {
				t.Fatalf("TerminalColumns(%d).IsValid() = %t, want %t", tc.columns, got, want)
			}
		})
	}
}

func TestTerminalGeometryValidatesAttachmentAgainstColumns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		geometry TerminalGeometry
		wantErr  error
	}{
		{name: "unset attachment describes nothing", geometry: TerminalGeometry{}, wantErr: core.ErrHostFactsContract},
		{name: "unset attachment with columns is still nothing", geometry: TerminalGeometry{columns: 80}, wantErr: core.ErrHostFactsContract},
		{name: "attachment beyond the closed domain is rejected", geometry: TerminalGeometry{attachment: terminalAttachmentLimit}, wantErr: core.ErrHostFactsContract},
		{name: "attachment far beyond the closed domain is rejected", geometry: TerminalGeometry{attachment: TerminalAttachment(math.MaxUint8)}, wantErr: core.ErrHostFactsContract},
		{name: "a terminal with zero columns is a contradiction", geometry: TerminalGeometry{attachment: TerminalAttachmentTerminal}, wantErr: core.ErrHostFactsContract},
		{name: "a terminal with one column is the smallest observation", geometry: TerminalGeometry{attachment: TerminalAttachmentTerminal, columns: 1}},
		{name: "a terminal with the ceiling width is observable", geometry: TerminalGeometry{attachment: TerminalAttachmentTerminal, columns: math.MaxUint16}},
		{name: "a detached descriptor with no columns is the detached observation", geometry: TerminalGeometry{attachment: TerminalAttachmentNotTerminal}},
		{name: "a detached descriptor claiming one column is a contradiction", geometry: TerminalGeometry{attachment: TerminalAttachmentNotTerminal, columns: 1}, wantErr: core.ErrHostFactsContract},
		{name: "a detached descriptor claiming the ceiling is a contradiction", geometry: TerminalGeometry{attachment: TerminalAttachmentNotTerminal, columns: math.MaxUint16}, wantErr: core.ErrHostFactsContract},
		{name: "a terminal without geometry carrying no columns is the geometryless observation", geometry: TerminalGeometry{attachment: TerminalAttachmentTerminalWithoutGeometry}},
		{name: "a terminal without geometry claiming one column is a contradiction", geometry: TerminalGeometry{attachment: TerminalAttachmentTerminalWithoutGeometry, columns: 1}, wantErr: core.ErrHostFactsContract},
		{name: "a terminal without geometry claiming the ceiling is a contradiction", geometry: TerminalGeometry{attachment: TerminalAttachmentTerminalWithoutGeometry, columns: math.MaxUint16}, wantErr: core.ErrHostFactsContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.geometry.Validate()
			if tc.wantErr == nil && err != nil {
				t.Fatalf("TerminalGeometry.Validate() error = %v, want nil", err)
			}
			if tc.wantErr == nil {
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("TerminalGeometry.Validate() error = %v, want errors.Is %v", err, tc.wantErr)
			}
			if attachment, accessErr := tc.geometry.Attachment(); !errors.Is(accessErr, tc.wantErr) || attachment != TerminalAttachmentUnknown {
				t.Fatalf("invalid geometry Attachment() = (%v, %v), want (%v, contract refusal)", attachment, accessErr, TerminalAttachmentUnknown)
			}
			if columns, accessErr := tc.geometry.Columns(); !errors.Is(accessErr, tc.wantErr) || columns != 0 {
				t.Fatalf("invalid geometry Columns() = (%v, %v), want (0, contract refusal)", columns, accessErr)
			}
		})
	}
}

func TestTerminalGeometryConstructorsSealEveryObservation(t *testing.T) {
	t.Parallel()

	attached, err := newAttachedTerminalGeometry(121)
	if err != nil {
		t.Fatalf("newAttachedTerminalGeometry(121) error = %v, want nil", err)
	}
	if attachment, err := attached.Attachment(); err != nil || attachment != TerminalAttachmentTerminal {
		t.Fatalf("attached.Attachment() = (%v, %v), want (%v, nil)", attachment, err, TerminalAttachmentTerminal)
	}
	if columns, err := attached.Columns(); err != nil || columns != 121 {
		t.Fatalf("attached.Columns() = (%v, %v), want (121, nil)", columns, err)
	}

	if _, err := newAttachedTerminalGeometry(0); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("newAttachedTerminalGeometry(0) error = %v, want %v", err, core.ErrHostFactsContract)
	}

	detached, err := newDetachedTerminalGeometry()
	if err != nil {
		t.Fatalf("newDetachedTerminalGeometry() error = %v, want nil", err)
	}
	if attachment, err := detached.Attachment(); err != nil || attachment != TerminalAttachmentNotTerminal {
		t.Fatalf("detached.Attachment() = (%v, %v), want (%v, nil)", attachment, err, TerminalAttachmentNotTerminal)
	}
	if columns, err := detached.Columns(); !errors.Is(err, core.ErrHostFactsContract) || columns != 0 {
		t.Fatalf("detached.Columns() = (%v, %v), want (0, %v)", columns, err, core.ErrHostFactsContract)
	}

	geometryless, err := newTerminalWithoutGeometry()
	if err != nil {
		t.Fatalf("newTerminalWithoutGeometry() error = %v, want nil", err)
	}
	if attachment, err := geometryless.Attachment(); err != nil || attachment != TerminalAttachmentTerminalWithoutGeometry {
		t.Fatalf("geometryless.Attachment() = (%v, %v), want (%v, nil)",
			attachment, err, TerminalAttachmentTerminalWithoutGeometry)
	}
	if columns, err := geometryless.Columns(); !errors.Is(err, core.ErrHostFactsContract) || columns != 0 {
		t.Fatalf("geometryless.Columns() = (%v, %v), want (0, %v)", columns, err, core.ErrHostFactsContract)
	}
}

func TestTerminalGeometryRequestRefusesAMissingFile(t *testing.T) {
	t.Parallel()

	err := TerminalGeometryRequest{}.Validate()
	if !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("TerminalGeometryRequest{}.Validate() error = %v, want %v", err, core.ErrHostFactsContract)
	}
	if _, err := ObserveTerminalGeometry(TerminalGeometryRequest{}); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("ObserveTerminalGeometry(missing file) error = %v, want %v", err, core.ErrHostFactsContract)
	}
}
