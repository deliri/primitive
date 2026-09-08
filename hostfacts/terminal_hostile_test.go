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
		wantErr error
		name    string
		columns TerminalColumns
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
		wantErr  error
		name     string
		geometry TerminalGeometry
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

func TestTerminalGeometryConstructorLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                    string
		construct               func() (TerminalGeometry, error)
		want                    TerminalGeometry
		wantErr, wantColumnsErr error
	}{
		{name: "smallest attached width seals exact columns", construct: func() (TerminalGeometry, error) { return newAttachedTerminalGeometry(1) }, want: TerminalGeometry{attachment: TerminalAttachmentTerminal, columns: 1}},
		{name: "largest attached width does not truncate", construct: func() (TerminalGeometry, error) { return newAttachedTerminalGeometry(math.MaxUint16) }, want: TerminalGeometry{attachment: TerminalAttachmentTerminal, columns: math.MaxUint16}},
		{name: "zero width refuses an attached observation", construct: func() (TerminalGeometry, error) { return newAttachedTerminalGeometry(0) }, wantErr: core.ErrHostFactsContract},
		{name: "detachment carries no columns", construct: newDetachedTerminalGeometry, want: TerminalGeometry{attachment: TerminalAttachmentNotTerminal}, wantColumnsErr: core.ErrHostFactsContract},
		{name: "geometryless attachment does not claim detachment", construct: newTerminalWithoutGeometry, want: TerminalGeometry{attachment: TerminalAttachmentTerminalWithoutGeometry}, wantColumnsErr: core.ErrHostFactsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.construct()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("constructor = %+v/%v, want error %v", got, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (TerminalGeometry{}) {
					t.Fatalf("refused geometry = %+v, want zero", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("geometry = %+v, want %+v", got, tc.want)
			}
			attachment, err := got.Attachment()
			if err != nil || attachment != tc.want.attachment {
				t.Fatalf("attachment = %v/%v, want %v", attachment, err, tc.want.attachment)
			}
			columns, err := got.Columns()
			if !errors.Is(err, tc.wantColumnsErr) || columns != tc.want.columns {
				t.Fatalf("columns = %d/%v, want %d/%v", columns, err, tc.want.columns, tc.wantColumnsErr)
			}
		})
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
