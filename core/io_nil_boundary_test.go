package core

import (
	"bytes"
	"io"
	"testing"
)

type nilIOMap map[int]byte

func (nilIOMap) Read([]byte) (int, error)    { panic("nil classification invoked Read") }
func (nilIOMap) Write(p []byte) (int, error) { panic("nil classification invoked Write") }

type nilIOSlice []byte

func (nilIOSlice) Read([]byte) (int, error)    { panic("nil classification invoked Read") }
func (nilIOSlice) Write(p []byte) (int, error) { panic("nil classification invoked Write") }

type nilIOFunc func()

func (nilIOFunc) Read([]byte) (int, error)    { panic("nil classification invoked Read") }
func (nilIOFunc) Write(p []byte) (int, error) { panic("nil classification invoked Write") }

type nilIOChannel chan byte

func (nilIOChannel) Read([]byte) (int, error)    { panic("nil classification invoked Read") }
func (nilIOChannel) Write(p []byte) (int, error) { panic("nil classification invoked Write") }

type nilIOValue struct{}

func (nilIOValue) Read([]byte) (int, error)    { panic("nil classification invoked Read") }
func (nilIOValue) Write(p []byte) (int, error) { panic("nil classification invoked Write") }

// These are pure interface-classification fixtures. Neither helper may call
// Read or Write: nil is an admission fact, not inferred from method behavior.
func TestIOInterfaceNilKindLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		reader io.Reader
		writer io.Writer
		want   bool
	}{
		{name: "absent interface is refused", want: true},
		{name: "typed nil pointer is refused", reader: (*bytes.Buffer)(nil), writer: (*bytes.Buffer)(nil), want: true},
		{name: "allocated empty pointer remains a capability", reader: new(bytes.Buffer), writer: new(bytes.Buffer)},
		{name: "typed nil map is refused", reader: nilIOMap(nil), writer: nilIOMap(nil), want: true},
		{name: "allocated empty map remains a capability", reader: nilIOMap{}, writer: nilIOMap{}},
		{name: "typed nil slice is refused", reader: nilIOSlice(nil), writer: nilIOSlice(nil), want: true},
		{name: "allocated empty slice remains a capability", reader: nilIOSlice{}, writer: nilIOSlice{}},
		{name: "typed nil function is refused", reader: nilIOFunc(nil), writer: nilIOFunc(nil), want: true},
		{name: "function capability remains present", reader: nilIOFunc(func() {}), writer: nilIOFunc(func() {})},
		{name: "typed nil channel is refused", reader: nilIOChannel(nil), writer: nilIOChannel(nil), want: true},
		{name: "allocated channel remains a capability", reader: make(nilIOChannel), writer: make(nilIOChannel)},
		{name: "zero value implementation is present", reader: nilIOValue{}, writer: nilIOValue{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotReader, gotWriter := ReaderIsNil(tc.reader), WriterIsNil(tc.writer)
			if gotReader != tc.want || gotWriter != tc.want {
				t.Fatalf("reader/writer nil = (%t, %t), want (%t, %t)", gotReader, gotWriter, tc.want, tc.want)
			}
		})
	}
}
