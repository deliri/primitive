package runnercontrol_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoEventStreamLayerTriadEmptySourceHasNoEvents(t *testing.T) {
	t.Parallel()
	fields, events := 0, 0
	err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
		Source:   strings.NewReader(""),
		OnString: func(fragment runnercontrol.GoEventStringFragment) error { fields++; return fragment.Validate() },
		OnEvent:  func(event runnercontrol.GoEventFrame) error { events++; return event.Validate() },
	})
	if err != nil || fields != 0 || events != 0 {
		t.Fatalf("ReadGoEventStream(empty) = (%v,%d fields,%d events), want (nil,0,0)", err, fields, events)
	}
}

func TestGoEventStreamLayerTriadLongStringPreservesExactDecodedBytes(t *testing.T) {
	t.Parallel()
	const marker = "PEACHFUZZ_STREAM_FIXTURE"
	seed := struct {
		Action runnercontrol.GoEventAction `json:"Action"`
		Output string                      `json:"Output"`
	}{Action: runnercontrol.GoEventActionOutput, Output: marker}
	encoded, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	prefix, suffix, found := bytes.Cut(encoded, []byte(marker))
	if !found {
		t.Fatalf("typed seed = %q, want unique payload marker", encoded)
	}
	const extent int64 = (64 << 20) + 1
	source := io.MultiReader(bytes.NewReader(prefix), io.LimitReader(goEventPadding{}, extent), bytes.NewReader(suffix))
	var observed int64
	fields, events := 0, 0
	err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
		Source: source,
		OnString: func(fragment runnercontrol.GoEventStringFragment) error {
			if err := fragment.Validate(); err != nil {
				return err
			}
			if fragment.Field != runnercontrol.GoEventFieldOutput {
				return nil
			}
			for _, value := range fragment.Data {
				if value != 'x' {
					t.Fatalf("decoded byte = %d, want x", value)
				}
			}
			observed += int64(len(fragment.Data))
			if fragment.Final {
				fields++
			}
			return nil
		},
		OnEvent: func(frame runnercontrol.GoEventFrame) error {
			events++
			if frame.Action != runnercontrol.GoEventActionOutput || observed != extent || fields != 1 {
				t.Fatalf("completed frame = %+v, decoded %d, field closes %d; want output, %d, 1", frame, observed, fields, extent)
			}
			return frame.Validate()
		},
	})
	if err != nil || events != 1 || observed != extent {
		t.Fatalf("ReadGoEventStream() = (%v,%d events,%d bytes), want (nil,1,%d)", err, events, observed, extent)
	}
}

type goEventPadding struct{}

func (goEventPadding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

type goEventFailureReader struct{ cause error }

func (r goEventFailureReader) Read([]byte) (int, error) { return 0, r.cause }

func TestGoEventStreamLayerTriadRefusalsAndCancellation(t *testing.T) {
	t.Parallel()
	seed := runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionStart}
	if err := seed.Validate(); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		source      func() io.Reader
		stringError error
		eventError  error
		cancelField bool
		wantErr     error
		wantEvents  int
	}{
		{name: "typed nil source rejected", source: func() io.Reader { return (*strings.Reader)(nil) }, wantErr: core.ErrPrimitiveContract},
		{name: "source refusal before frame", source: func() io.Reader { return goEventFailureReader{io.ErrClosedPipe} }, wantErr: io.ErrClosedPipe},
		{name: "source refusal retains admitted prefix without sealing stream", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader(append(append([]byte(nil), canonical...), '\n')), goEventFailureReader{io.ErrClosedPipe})
		}, wantErr: io.ErrClosedPipe, wantEvents: 1},
		{name: "joined EOF cannot accept incomplete source", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader(canonical), goEventFailureReader{errors.Join(io.EOF, io.ErrClosedPipe)})
		}, wantErr: io.ErrClosedPipe},
		{name: "field callback refusal propagates", source: func() io.Reader { return bytes.NewReader(canonical) }, stringError: io.ErrShortWrite, wantErr: io.ErrShortWrite},
		{name: "event callback refusal propagates", source: func() io.Reader { return bytes.NewReader(canonical) }, eventError: io.ErrShortWrite, wantErr: io.ErrShortWrite, wantEvents: 1},
		{name: "field cancellation prevents complete frame", source: func() io.Reader { return bytes.NewReader(canonical) }, cancelField: true, wantErr: context.Canceled},
		{name: "truncated event refuses completion", source: func() io.Reader { return bytes.NewReader(canonical[:len(canonical)-1]) }, wantErr: core.ErrJSONContract},
		{name: "unknown action never becomes a frame", source: func() io.Reader { return strings.NewReader(`{"Action":"future"}`) }, wantErr: core.ErrJSONContract},
		{name: "duplicate action refuses ambiguous frame", source: func() io.Reader { return strings.NewReader(`{"Action":"start","Action":"pass"}`) }, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			events := 0
			err := runnercontrol.ReadGoEventStream(ctx, runnercontrol.GoEventStreamRequest{
				Source: tc.source(),
				OnString: func(fragment runnercontrol.GoEventStringFragment) error {
					if err := fragment.Validate(); err != nil {
						return err
					}
					if tc.cancelField {
						cancel()
					}
					return tc.stringError
				},
				OnEvent: func(frame runnercontrol.GoEventFrame) error {
					events++
					if frame != seed {
						t.Fatalf("frame = %+v, want %+v", frame, seed)
					}
					return tc.eventError
				},
			})
			if !errors.Is(err, tc.wantErr) || events != tc.wantEvents {
				t.Fatalf("ReadGoEventStream() = (%v,%d events), want (%v,%d)", err, events, tc.wantErr, tc.wantEvents)
			}
		})
	}
}

func TestGoEventStreamEveryActionRetainsItsWireIdentity(t *testing.T) {
	t.Parallel()
	for action := runnercontrol.GoEventActionStart; action <= runnercontrol.GoEventActionArtifacts; action++ {
		t.Run(action.String(), func(t *testing.T) {
			t.Parallel()
			want := runnercontrol.GoEventFrame{Action: action}
			if err := want.Validate(); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			events := 0
			err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
				Source:   bytes.NewReader(encoded),
				OnString: func(fragment runnercontrol.GoEventStringFragment) error { return fragment.Validate() },
				OnEvent: func(got runnercontrol.GoEventFrame) error {
					events++
					if got != want {
						t.Fatalf("frame = %+v, want %+v", got, want)
					}
					return nil
				},
			})
			if err != nil || events != 1 {
				t.Fatalf("ReadGoEventStream() = (%v,%d), want (nil,1)", err, events)
			}
		})
	}
}

// The standard-library oracle extracts only the two bounded enum fields from
// each borrowed source line. It does not retain diagnostic strings or a stream
// model. Separate long-string tests prove byte-for-byte fragment delivery.
func FuzzGoEventStreamSemanticClosure(f *testing.F) {
	for action := runnercontrol.GoEventActionStart; action <= runnercontrol.GoEventActionArtifacts; action++ {
		seed := runnercontrol.GoEventFrame{Action: action}
		if err := seed.Validate(); err != nil {
			f.Fatal(err)
		}
		encoded, err := json.Marshal(seed)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`{"Action":"unknown"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		offset, events := 0, 0
		err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
			Source: bytes.NewReader(data),
			OnString: func(fragment runnercontrol.GoEventStringFragment) error {
				if err := fragment.Validate(); err != nil {
					t.Fatalf("emitted string fragment = %+v, want valid: %v", fragment, err)
				}
				return nil
			},
			OnEvent: func(frame runnercontrol.GoEventFrame) error {
				if err := frame.Validate(); err != nil {
					t.Fatalf("emitted frame = %+v, want valid: %v", frame, err)
				}
				end := len(data)
				if index := bytes.IndexByte(data[offset:], '\n'); index >= 0 {
					end = offset + index
				}
				var independent struct {
					Action     string
					OutputType string
				}
				if err := json.Unmarshal(data[offset:end], &independent); err != nil {
					t.Fatalf("accepted source line = %q, want standard-library JSON admission: %v", data[offset:end], err)
				}
				if frame.Action.String() != independent.Action || frame.OutputKind.String() != independent.OutputType {
					t.Fatalf("frame = %+v, want action %q and output kind %q", frame, independent.Action, independent.OutputType)
				}
				offset = end
				if offset < len(data) {
					offset++
				}
				events++
				return nil
			},
		})
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("ReadGoEventStream() error = %v, want typed JSON refusal", err)
			}
			return
		}
		if offset != len(data) && len(bytes.TrimSpace(data[offset:])) != 0 {
			t.Fatalf("accepted stream consumed %d of %d bytes with %d frames, want only neutral whitespace after the last frame", offset, len(data), events)
		}
	})
}

func TestGoEventStreamOutputKindsRemainClosed(t *testing.T) {
	t.Parallel()
	for kind := runnercontrol.GoEventOutputOrdinary; kind <= runnercontrol.GoEventOutputErrorContinue; kind++ {
		name := kind.String()
		if name == "" {
			name = "ordinary output has explicit empty kind"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := runnercontrol.GoEventFrame{Action: runnercontrol.GoEventActionOutput, OutputKind: kind}
			encoded, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
				Source: bytes.NewReader(encoded), OnString: func(fragment runnercontrol.GoEventStringFragment) error { return fragment.Validate() },
				OnEvent: func(got runnercontrol.GoEventFrame) error {
					count++
					if got != want {
						t.Fatalf("frame = %+v, want %+v", got, want)
					}
					return got.Validate()
				},
			})
			if err != nil || count != 1 {
				t.Fatalf("ReadGoEventStream() = (%v,%d), want (nil,1)", err, count)
			}
		})
	}
}

func BenchmarkGoEventStreamFixedBuffer(b *testing.B) {
	seed := struct {
		Action runnercontrol.GoEventAction `json:"Action"`
		Output string                      `json:"Output"`
	}{Action: runnercontrol.GoEventActionOutput, Output: strings.Repeat("x", 64<<10)}
	encoded, err := json.Marshal(seed)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(encoded)))
	for b.Loop() {
		count, decoded := 0, 0
		err := runnercontrol.ReadGoEventStream(b.Context(), runnercontrol.GoEventStreamRequest{
			Source: bytes.NewReader(encoded),
			OnString: func(fragment runnercontrol.GoEventStringFragment) error {
				if fragment.Field == runnercontrol.GoEventFieldOutput {
					decoded += len(fragment.Data)
				}
				return fragment.Validate()
			},
			OnEvent: func(frame runnercontrol.GoEventFrame) error { count++; return frame.Validate() },
		})
		if err != nil || count != 1 || decoded != len(seed.Output) {
			b.Fatalf("ReadGoEventStream() = (%v,%d,%d), want (nil,1,%d)", err, count, decoded, len(seed.Output))
		}
	}
}

func TestGoEventStreamStringFragmentBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, text string }{
		{name: "empty output closes its field", text: ""},
		{name: "ASCII immediately below flush", text: strings.Repeat("x", 190)},
		{name: "ASCII at flush", text: strings.Repeat("x", 191)},
		{name: "ASCII above flush", text: strings.Repeat("x", 192)},
		{name: "two-byte rune spans flush", text: strings.Repeat("x", 190) + "é"},
		{name: "three-byte rune spans flush", text: strings.Repeat("x", 190) + "界"},
		{name: "four-byte rune spans flush", text: strings.Repeat("x", 190) + "😀"},
		{name: "escaped quote spans flush", text: strings.Repeat("x", 190) + "\""},
		{name: "escaped backslash spans flush", text: strings.Repeat("x", 190) + "\\"},
		{name: "escaped newline spans flush", text: strings.Repeat("x", 190) + "\n"},
		{name: "escaped NUL spans flush", text: strings.Repeat("x", 190) + "\x00"},
		{name: "multiple fragments preserve multilingual content", text: strings.Repeat("é界😀\\\n\t", 1024)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := struct {
				Action runnercontrol.GoEventAction `json:"Action"`
				Output string                      `json:"Output"`
			}{Action: runnercontrol.GoEventActionOutput, Output: tc.text}
			encoded, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			offset, closed, events := 0, 0, 0
			err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
				Source: bytes.NewReader(encoded),
				OnString: func(fragment runnercontrol.GoEventStringFragment) error {
					if err := fragment.Validate(); err != nil {
						return err
					}
					if fragment.Field != runnercontrol.GoEventFieldOutput {
						return nil
					}
					end := offset + len(fragment.Data)
					if end > len(tc.text) || string(fragment.Data) != tc.text[offset:end] {
						t.Fatalf("decoded fragment at %d = %q, want exact next source bytes", offset, fragment.Data)
					}
					offset = end
					if fragment.Final {
						closed++
					}
					return nil
				},
				OnEvent: func(frame runnercontrol.GoEventFrame) error {
					events++
					if offset != len(tc.text) || closed != 1 {
						t.Fatalf("complete frame has %d bytes and %d field closes, want %d and 1", offset, closed, len(tc.text))
					}
					return frame.Validate()
				},
			})
			if err != nil || events != 1 {
				t.Fatalf("ReadGoEventStream() = (%v,%d frames), want (nil,1)", err, events)
			}
		})
	}
}

// Each independent encoder/oracle step owns at most one 4-KiB payload. This
// limits test working storage, not input or event count: all fuzz bytes cross
// the real decoder, including input spanning arbitrarily many events.
const goEventOracleChunk = 4 << 10

type goEventPayloadFixture struct {
	Action runnercontrol.GoEventAction `json:"Action"`
	Output string                      `json:"Output"`
}
type goEventPayloadSource struct {
	input   []byte
	pending []byte
	offset  int
	emitted bool
}

func (s *goEventPayloadSource) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(s.pending) == 0 {
		if s.emitted && s.offset == len(s.input) {
			return 0, io.EOF
		}
		end := min(s.offset+goEventOracleChunk, len(s.input))
		fixture := goEventPayloadFixture{Action: runnercontrol.GoEventActionOutput, Output: strings.ToValidUTF8(string(s.input[s.offset:end]), "�")}
		encoded, err := json.Marshal(fixture)
		if err != nil {
			return 0, err
		}
		s.pending = append(encoded, '\n')
		s.offset = end
		s.emitted = true
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

func FuzzGoEventStreamDecodedContent(f *testing.F) {
	seed := goEventPayloadFixture{Action: runnercontrol.GoEventActionOutput, Output: "é界😀\n\t\"\\"}
	if err := seed.Action.Validate(); err != nil {
		f.Fatal(err)
	}
	canonical, err := json.Marshal(seed)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte{0xff, 0x00, 0xc0})
	f.Fuzz(func(t *testing.T, data []byte) {
		source := goEventPayloadSource{input: data}
		inputOffset, decodedOffset, closed, events := 0, 0, 0, 0
		wanted := func() string {
			end := min(inputOffset+goEventOracleChunk, len(data))
			return strings.ToValidUTF8(string(data[inputOffset:end]), "�")
		}
		want := wanted()
		err := runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
			Source: &source,
			OnString: func(fragment runnercontrol.GoEventStringFragment) error {
				if err := fragment.Validate(); err != nil {
					return err
				}
				if fragment.Field != runnercontrol.GoEventFieldOutput {
					return nil
				}
				end := decodedOffset + len(fragment.Data)
				if end > len(want) || string(fragment.Data) != want[decodedOffset:end] {
					t.Fatalf("decoded fragment at %d = %q, want exact encoded payload bytes", decodedOffset, fragment.Data)
				}
				decodedOffset = end
				if fragment.Final {
					closed++
				}
				return nil
			},
			OnEvent: func(frame runnercontrol.GoEventFrame) error {
				if frame.Action != runnercontrol.GoEventActionOutput || frame.OutputKind != runnercontrol.GoEventOutputOrdinary || decodedOffset != len(want) || closed != 1 {
					t.Fatalf("frame = %+v, bytes %d, closes %d; want output, %d bytes and one close", frame, decodedOffset, closed, len(want))
				}
				events++
				inputOffset = min(inputOffset+goEventOracleChunk, len(data))
				decodedOffset, closed = 0, 0
				want = wanted()
				return nil
			},
		})
		wantEvents := max(1, len(data)/goEventOracleChunk)
		if len(data)%goEventOracleChunk != 0 && len(data) > goEventOracleChunk {
			wantEvents++
		}
		if err != nil || events != wantEvents || inputOffset != len(data) {
			t.Fatalf("ReadGoEventStream() = (%v,%d frames,%d input bytes), want (nil,%d,%d)", err, events, inputOffset, wantEvents, len(data))
		}
	})
}
