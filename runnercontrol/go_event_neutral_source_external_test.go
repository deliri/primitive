package runnercontrol_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoEventStreamNeutralPhysicalRecordsAreNotEvents(t *testing.T) {
	t.Parallel()
	seed := runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionStart}
	encoded, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, text string }{{name: "empty"}, {name: "space", text: " "}, {name: "tab and carriage return", text: "\t\r"}, {name: "blank LF", text: "\n"}, {name: "blank CRLF", text: "\r\n"}, {name: "mixed blank records", text: " \r\n\n\t\n"}} {
		text := tc.text
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, withFrame := range []bool{false, true} {
				var source io.Reader = strings.NewReader(text)
				want := 0
				if withFrame {
					source = io.MultiReader(source, bytes.NewReader(encoded), strings.NewReader("\n"+text))
					want = 1
				}
				count, fields := 0, 0
				err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: source, OnString: func(fragment runnercontrol.GoEventStringFragment) error { fields++; return fragment.Validate() }, OnEvent: func(frame runnercontrol.GoEventFrame) error {
					if frame != seed {
						t.Fatalf("neutral text changed frame=%+v", frame)
					}
					count++
					return nil
				}})
				if err != nil || count != want || !withFrame && fields != 0 {
					t.Fatalf("neutral source frame=%t: %d frames, %d fields, %v", withFrame, count, fields, err)
				}
			}
		})
	}
}

type neutralGoEventSource struct{ offset uint64 }

func (s *neutralGoEventSource) Read(destination []byte) (int, error) {
	for index := range destination {
		destination[index] = [...]byte{' ', '\t', '\r', '\n'}[s.offset%4]
		s.offset++
	}
	return len(destination), nil
}
func TestGoEventStreamNeutralSourceHasNoExtentQuota(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{32767, 32768, 32769, 8*1024*1024 + 1} {
		t.Run("neutral-bytes-"+strconv.FormatInt(extent, 10), func(t *testing.T) {
			t.Parallel()
			fields, events := 0, 0
			err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: io.LimitReader(&neutralGoEventSource{}, extent), OnString: func(runnercontrol.GoEventStringFragment) error { fields++; return nil }, OnEvent: func(runnercontrol.GoEventFrame) error { events++; return nil }})
			if err != nil || fields != 0 || events != 0 {
				t.Fatalf("neutral extent=%d: %d frames, %d fields, %v", extent, events, fields, err)
			}
		})
	}
}
func TestGoEventStreamNeutralTextCannotCompletePartialFrame(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"{\n", `{"Action":` + "\n", `{"Action":"start",` + "\n", `{"Action":"start"` + "\n", `{"Action":"start"` + "\r\n"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			events := 0
			err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{Source: strings.NewReader(source), OnString: func(fragment runnercontrol.GoEventStringFragment) error { return fragment.Validate() }, OnEvent: func(runnercontrol.GoEventFrame) error { events++; return nil }})
			if events != 0 || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("partial native frame=%d events, %v", events, err)
			}
		})
	}
}
