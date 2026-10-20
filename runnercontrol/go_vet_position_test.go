package runnercontrol_test

import (
	"errors"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoVetNativePositionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       token.Position
		refuses    bool
	}{
		{name: "native absolute file", text: "/tmp/native.go:3:7", want: token.Position{Filename: "/tmp/native.go", Offset: -1, Line: 3, Column: 7}},
		{name: "native relative file", text: "native.go:3:7", want: token.Position{Filename: "native.go", Offset: -1, Line: 3, Column: 7}},
		{name: "native colon filename and drive", text: `C:\source:path\native.go:23:71`, want: token.Position{Filename: `C:\source:path\native.go`, Offset: -1, Line: 23, Column: 71}},
		{name: "zero line", text: "native.go:0:1", refuses: true},
		{name: "zero column", text: "native.go:1:0", refuses: true},
		{name: "negative coordinate", text: "native.go:-1:2", refuses: true},
		{name: "missing filename", text: ":1:2", refuses: true},
		{name: "missing coordinate", text: "native.go:1", refuses: true},
		{name: "malformed coordinate", text: "native.go:no:2", refuses: true},
		{name: "coordinate overflow", text: "native.go:" + strings.Repeat("9", 128) + ":2", refuses: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := runnercontrol.GoVetObservation{Kind: runnercontrol.GoVetObservationDiagnostic, Unit: "unit", Analyzer: "lostcancel", Position: tc.text}
			got, err := o.SourcePosition()
			if tc.refuses {
				if got != (token.Position{}) || !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("position=(%+v,%v), want zero,typed refusal", got, err)
				}
			} else if err != nil || got != tc.want || !got.IsValid() {
				t.Fatalf("position=(%+v,%v), want %+v,nil", got, err, tc.want)
			}
		})
	}
	got, err := (runnercontrol.GoVetObservation{Kind: runnercontrol.GoVetObservationAnalyzerError, Unit: "unit", Analyzer: "lostcancel", Message: "refused"}).SourcePosition()
	if got != (token.Position{}) || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("analyzer refusal position=(%+v,%v), want zero,typed refusal", got, err)
	}
}

func FuzzGoVetNativePositionConservation(f *testing.F) {
	f.Add("C:\\source:path\\界.go", uint16(7), uint16(3))
	f.Add("relative.go", uint16(0), uint16(0))
	f.Fuzz(func(t *testing.T, name string, line, column uint16) {
		for start := 0; ; {
			end := min(start+4096, len(name))
			file := strings.ToValidUTF8(name[start:end], "�")
			if file == "" {
				file = "native.go"
			}
			want := token.Position{Filename: file, Offset: -1, Line: int(line) + 1, Column: int(column) + 1}
			o := runnercontrol.GoVetObservation{Kind: runnercontrol.GoVetObservationDiagnostic, Unit: "unit", Analyzer: "waitgroup", Position: file + ":" + strconv.Itoa(want.Line) + ":" + strconv.Itoa(want.Column)}
			got, err := o.SourcePosition()
			if err != nil || got != want {
				t.Fatalf("native position=(%+v,%v), want %+v,nil", got, err, want)
			}
			if end == len(name) {
				break
			}
			start = end
		}
	})
}
