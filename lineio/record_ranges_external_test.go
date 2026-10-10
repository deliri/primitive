package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func recordRangeBuffer(t *testing.T, size uint64) core.ByteCount {
	t.Helper()
	value, err := core.NewByteCount(size)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRecordRangesConserveIndependentPhysicalFraming(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body string }{
		{"empty", ""}, {"LF", "x\n"}, {"CRLF", "x\r\n"}, {"unterminated", "x"}, {"blank LF", "\n"}, {"blank CRLF", "\r\n"},
		{"consecutive LF", "\n\n"}, {"CR remains payload", "x\ry"}, {"final CR", "x\r"}, {"initial LF", "\nx\n"}, {"final blank line", "x\n\n"},
		{"UTF8 source", "λ界\n"}, {"invalid UTF8 source", "\xff\xfe\n"}, {"NUL source", "a\x00b\n"}, {"BOM source", "\ufeffx\n"}, {"Unicode separator is payload", "x\u2028y\n"},
		{"metadata", "goos: linux\ngoarch: amd64\n"}, {"mixed framing", "x\ny\r\nz"}, {"trailing whitespace", "x \t\n"}, {"only whitespace", " \t"},
		{"one below buffer", strings.Repeat("x", 4095) + "\n"}, {"at buffer", strings.Repeat("x", 4096) + "\n"}, {"one above buffer", strings.Repeat("x", 4097) + "\n"},
		{"CR at buffer edge", strings.Repeat("x", 4095) + "\r\n"}, {"UTF8 crosses buffer", strings.Repeat("x", 4095) + "界\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prefix := "ignored prefix\n"
			source := strings.NewReader(prefix + tc.body)
			if _, err := source.Seek(int64(len(prefix)), io.SeekStart); err != nil {
				t.Fatal(err)
			}
			want := bytes.SplitAfter([]byte(tc.body), []byte{'\n'})
			if len(want) > 0 && len(want[len(want)-1]) == 0 {
				want = want[:len(want)-1]
			}
			count, offset := 0, len(prefix)
			for record, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 4096)}) {
				if err != nil {
					t.Fatal(err)
				}
				if count >= len(want) {
					t.Fatal("extra record")
				}
				framing := lineio.RecordFramingEOF
				if bytes.HasSuffix(want[count], []byte{'\n'}) {
					framing = lineio.RecordFramingLF
				}
				if uint64(record.Offset) != uint64(offset) || record.Bytes.Uint64() != uint64(len(want[count])) || record.Framing != framing || record.Validate() != nil {
					t.Fatalf("range=%+v expected bytes%d offset%d framing%d", record, len(want[count]), offset, framing)
				}
				var replay bytes.Buffer
				for fragment, err := range lineio.RecordFragments(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: recordRangeBuffer(t, 17)}) {
					if err != nil {
						t.Fatal(err)
					}
					replay.Write(fragment.Bytes)
				}
				if !bytes.Equal(replay.Bytes(), want[count]) {
					t.Fatalf("replay=%q want%q", replay.Bytes(), want[count])
				}
				offset += len(want[count])
				count++
			}
			if count != len(want) {
				t.Fatalf("records%d want%d", count, len(want))
			}
		})
	}
}

type recordRangeSourcePressure struct {
	*strings.Reader
	readErr, readAtErr, seekErr error
	chunk                       int
	cancel                      context.CancelFunc
	readCalls, readAtCalls      int
}

func (s *recordRangeSourcePressure) Read(p []byte) (int, error) {
	s.readCalls++
	if s.chunk > 0 && len(p) > s.chunk {
		p = p[:s.chunk]
	}
	n, err := s.Reader.Read(p)
	if s.cancel != nil {
		s.cancel()
	}
	if s.readErr != nil {
		return n, s.readErr
	}
	return n, err
}
func (s *recordRangeSourcePressure) ReadAt(p []byte, off int64) (int, error) {
	s.readAtCalls++
	n, err := s.Reader.ReadAt(p, off)
	if s.cancel != nil {
		s.cancel()
	}
	if s.readAtErr != nil {
		return n, s.readAtErr
	}
	return n, err
}
func (s *recordRangeSourcePressure) Seek(off int64, whence int) (int64, error) {
	if s.seekErr != nil {
		return 0, s.seekErr
	}
	return s.Reader.Seek(off, whence)
}

func TestRecordRangesNativeReadFailureCancellationAndConsumerStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                             string
		chunk                            int
		readErr, seekErr                 error
		cancelBefore, cancelDuring, stop bool
	}{
		{name: "single byte source", chunk: 1}, {name: "three byte source", chunk: 3}, {name: "full count with source error", readErr: io.ErrUnexpectedEOF},
		{name: "partial count with source error", readErr: io.ErrUnexpectedEOF, chunk: 2}, {name: "wrapped EOF is failure", readErr: errors.Join(core.ErrPrimitiveContract, io.EOF)},
		{name: "wrapped EOF partial source", readErr: errors.Join(core.ErrPrimitiveContract, io.EOF), chunk: 2}, {name: "seek failure", seekErr: io.ErrUnexpectedEOF},
		{name: "cancel before read", cancelBefore: true}, {name: "cancel during read", cancelDuring: true}, {name: "consumer stops before tail", chunk: 1, stop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &recordRangeSourcePressure{Reader: strings.NewReader("first\nsecond\n"), chunk: tc.chunk, readErr: tc.readErr, seekErr: tc.seekErr}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			if tc.cancelDuring {
				source.cancel = cancel
			}
			count := 0
			var gotErr error
			for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 16)}) {
				if err != nil {
					gotErr = err
					if record != (lineio.RecordRange{}) {
						t.Fatal("failure leaked partial range")
					}
					break
				}
				count++
				if tc.stop {
					break
				}
			}
			switch {
			case tc.cancelBefore || tc.cancelDuring:
				if !errors.Is(gotErr, context.Canceled) || count != 0 {
					t.Fatalf("cancel=%v count%d", gotErr, count)
				}
				if tc.cancelBefore && source.readCalls != 0 {
					t.Fatal("canceled source read")
				}
			case tc.readErr != nil:
				if !errors.Is(gotErr, tc.readErr) || count != 0 {
					t.Fatalf("read=%v count%d want%v", gotErr, count, tc.readErr)
				}
			case tc.seekErr != nil:
				if !errors.Is(gotErr, tc.seekErr) || source.readCalls != 0 || count != 0 {
					t.Fatalf("seek=%v count%d reads%d", gotErr, count, source.readCalls)
				}
			case tc.stop:
				if gotErr != nil || count != 1 || source.readCalls != len("first\n") {
					t.Fatalf("stop=%v count%d reads%d", gotErr, count, source.readCalls)
				}
			default:
				if gotErr != nil || count != 2 {
					t.Fatalf("partial reads=%v count%d", gotErr, count)
				}
			}
		})
	}
}

func TestRecordRangeFragmentsPreserveFullCountCauseAndStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                             string
		cause                            error
		cancelBefore, cancelDuring, stop bool
	}{
		{name: "exact source"}, {name: "full source error", cause: io.ErrUnexpectedEOF}, {name: "wrapped source EOF", cause: errors.Join(core.ErrPrimitiveContract, io.EOF)},
		{name: "cancel before fragment", cancelBefore: true}, {name: "cancel during range read", cancelDuring: true}, {name: "stop after first fragment", stop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Repeat("x", 8192) + "\n"
			source := &recordRangeSourcePressure{Reader: strings.NewReader(body)}
			var record lineio.RecordRange
			for observed, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 4096)}) {
				if err != nil {
					t.Fatal(err)
				}
				record = observed
			}
			source.readAtCalls = 0
			source.readAtErr = tc.cause
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			if tc.cancelDuring {
				source.cancel = cancel
			}
			count, total := 0, 0
			var gotErr error
			for fragment, err := range lineio.RecordFragments(ctx, lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: recordRangeBuffer(t, 4096)}) {
				if err != nil {
					gotErr = err
					if len(fragment.Bytes) != 0 {
						t.Fatal("failure leaked range bytes")
					}
					break
				}
				count++
				total += len(fragment.Bytes)
				if tc.stop {
					break
				}
			}
			switch {
			case tc.cancelBefore || tc.cancelDuring:
				if !errors.Is(gotErr, context.Canceled) || count != 0 {
					t.Fatalf("cancel=%v count%d", gotErr, count)
				}
				if tc.cancelBefore && source.readAtCalls != 0 {
					t.Fatal("pre-canceled native read")
				}
			case tc.cause != nil:
				if !errors.Is(gotErr, tc.cause) || count != 0 {
					t.Fatalf("range cause=%v count%d", gotErr, count)
				}
			case tc.stop:
				if gotErr != nil || count != 1 || source.readAtCalls != 1 || total != 4096 {
					t.Fatalf("stop=%v count%d reads%d bytes%d", gotErr, count, source.readAtCalls, total)
				}
			default:
				if gotErr != nil || total != len(body) {
					t.Fatalf("source bytes%d/%v", total, gotErr)
				}
			}
		})
	}
}

func TestRecordRangesWorkingMemoryDoesNotFollowRecordLength(t *testing.T) {
	// witness:waiver test/parallel/default -- process-wide allocation accounting.
	for _, length := range []int{4095, 4096, 4097, 1 << 20, 7 << 20} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			// witness:waiver test/parallel/default -- serial allocation measurement.
			source := strings.NewReader(strings.Repeat("x", length) + "\n")
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count, total := 0, 0
			for record, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 4096)}) {
				if err != nil {
					t.Fatal(err)
				}
				count++
				for fragment, err := range lineio.RecordFragments(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: recordRangeBuffer(t, 4096)}) {
					if err != nil {
						t.Fatal(err)
					}
					total += len(fragment.Bytes)
				}
			}
			runtime.ReadMemStats(&after)
			if count != 1 || total != length+1 {
				t.Fatalf("records%d bytes%d", count, total)
			}
			if used := after.TotalAlloc - before.TotalAlloc; used > 128<<10 {
				t.Fatalf("record materialized%d bytes", used)
			}
		})
	}
}

func FuzzRecordRangesConserveIndependentByteFraming(f *testing.F) {
	for _, body := range []string{"", "x\n", "x\r\ny", "\xff\x00\n\n", "λ界\n", "x\ry"} {
		f.Add([]byte(body), uint8(17))
	}
	f.Fuzz(func(t *testing.T, body []byte, buffer uint8) {
		source := bytes.NewReader(body)
		want := bytes.SplitAfter(body, []byte{'\n'})
		if len(want) > 0 && len(want[len(want)-1]) == 0 {
			want = want[:len(want)-1]
		}
		count := 0
		var replay bytes.Buffer
		for record, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, uint64(buffer)+1)}) {
			if err != nil {
				t.Fatal(err)
			}
			if count >= len(want) || record.Bytes.Uint64() != uint64(len(want[count])) {
				t.Fatalf("range=%+v count%d", record, count)
			}
			for fragment, err := range lineio.RecordFragments(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: recordRangeBuffer(t, uint64(buffer)+1)}) {
				if err != nil {
					t.Fatal(err)
				}
				replay.Write(fragment.Bytes)
			}
			count++
		}
		if count != len(want) || !bytes.Equal(replay.Bytes(), body) {
			t.Fatalf("records%d want%d replay%q source%q", count, len(want), replay.Bytes(), body)
		}
	})
}

func TestRecordFragmentRequestRefusesInvalidCoordinatesBeforeNativeRead(t *testing.T) {
	t.Parallel()
	one, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                string
		record              lineio.RecordRange
		nilSource, typedNil bool
	}{
		{name: "unknown framing", record: lineio.RecordRange{Bytes: one}},
		{name: "outside framing enum", record: lineio.RecordRange{Bytes: one, Framing: lineio.RecordFraming(255)}},
		{name: "empty LF extent", record: lineio.RecordRange{Framing: lineio.RecordFramingLF}},
		{name: "empty EOF extent", record: lineio.RecordRange{Framing: lineio.RecordFramingEOF}},
		{name: "unsigned offset outside native size domain", record: lineio.RecordRange{Offset: lineio.SourceByteOffset(^uint64(0)), Bytes: one, Framing: lineio.RecordFramingLF}},
		{name: "end offset exceeds signed native range", record: lineio.RecordRange{Offset: lineio.SourceByteOffset(^uint64(0) >> 1), Bytes: one, Framing: lineio.RecordFramingLF}},
		{name: "nil native source", record: lineio.RecordRange{Bytes: one, Framing: lineio.RecordFramingLF}, nilSource: true},
		{name: "typed nil native source", record: lineio.RecordRange{Bytes: one, Framing: lineio.RecordFramingLF}, typedNil: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &recordRangeSourcePressure{Reader: strings.NewReader("x\n")}
			var native lineio.RecordSource = source
			if tc.nilSource {
				native = nil
			}
			if tc.typedNil {
				var missing *recordRangeSourcePressure
				native = missing
			}
			count := 0
			for fragment, err := range lineio.RecordFragments(t.Context(), lineio.RecordFragmentRequest{Source: native, Record: tc.record, BufferBytes: recordRangeBuffer(t, 16)}) {
				count++
				if err == nil || len(fragment.Bytes) != 0 {
					t.Fatalf("invalid coordinate published%+v/%v", fragment, err)
				}
			}
			if count != 1 || source.readAtCalls != 0 || source.readCalls != 0 {
				t.Fatalf("refusals%d positional reads%d sequential reads%d", count, source.readAtCalls, source.readCalls)
			}
		})
	}
}

func TestRecordFragmentsRefuseChangedFramingAndIncompleteSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		length     uint64
		framing    lineio.RecordFraming
	}{
		{"LF metadata on unterminated source", "x", 1, lineio.RecordFramingLF},
		{"EOF metadata on LF source", "x\n", 2, lineio.RecordFramingEOF},
		{"range extends beyond source", "x", 2, lineio.RecordFramingEOF},
		{"range spans multiple LF records", "x\ny\n", 4, lineio.RecordFramingLF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			size, err := core.NewByteLength(tc.length)
			if err != nil {
				t.Fatal(err)
			}
			source := strings.NewReader(tc.body)
			count := 0
			for fragment, err := range lineio.RecordFragments(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: size, Framing: tc.framing}, BufferBytes: recordRangeBuffer(t, 4096)}) {
				count++
				if !errors.Is(err, core.ErrLineIOScan) || len(fragment.Bytes) != 0 {
					t.Fatalf("changed native framing=%+v/%v", fragment, err)
				}
			}
			if count != 1 {
				t.Fatalf("refusals%d", count)
			}
		})
	}
}

func TestRecordRangesReplayNativeScratchWithinOwnedScope(t *testing.T) {
	t.Parallel()
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, root *os.Root) error {
		path, err := core.ParseRelativePath("physical-records")
		if err != nil {
			return err
		}
		receipt, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600}, Use: func(ctx context.Context, source filestore.ScratchReplayFile) error {
			body := "\x00\xff native bytes\r\n" + strings.Repeat("x", 7<<20) + "\nlast without LF"
			if _, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Destination: source, Source: strings.NewReader(body)}); err != nil {
				return err
			}
			if _, err := source.Seek(0, io.SeekStart); err != nil {
				return err
			}
			var copied strings.Builder
			count := 0
			for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 4096)}) {
				if err != nil {
					return err
				}
				count++
				for fragment, err := range lineio.RecordFragments(ctx, lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: recordRangeBuffer(t, 4096)}) {
					if err != nil {
						return err
					}
					copied.Write(fragment.Bytes)
				}
			}
			if count != 3 || copied.String() != body {
				t.Fatalf("native record ranges count%d conserved%t", count, copied.String() == body)
			}
			return nil
		}})
		if err != nil {
			return err
		}
		return errors.Join(receipt.OperationError(), receipt.CleanupError())
	}})
	if err != nil {
		t.Fatal(err)
	}
}

var _ lineio.RecordSource = filestore.ScratchReplayFile(nil)
