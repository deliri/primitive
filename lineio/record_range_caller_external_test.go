package lineio_test

import (
	"context"
	"io"
	"io/fs"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/lineio"
)

type callerFrameRecordSource struct{ *strings.Reader }

func (s callerFrameRecordSource) ReadAt(destination []byte, offset int64) (int, error) {
	var pcs [32]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(1, pcs[:])])
	want := runtime.FuncForPC(reflect.ValueOf(readRangeOnCallerStack).Pointer()).Name()
	for {
		frame, more := frames.Next()
		if frame.Function == want {
			return s.Reader.ReadAt(destination, offset)
		}
		if !more {
			return 0, fs.ErrPermission
		}
	}
}

func readRangeOnCallerStack(_ context.Context, reader io.Reader) error {
	_, err := io.Copy(io.Discard, reader)
	return err
}

func TestRecordRangeReadsRemainOnSynchronousConsumerStack(t *testing.T) {
	t.Parallel()
	body := "one native range\n"
	source := callerFrameRecordSource{Reader: strings.NewReader(body)}
	err := lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{
		Range: rangeScopeRequest(t, source, 0, body, lineio.RecordFramingLF), Use: readRangeOnCallerStack,
	})
	if err != nil {
		t.Fatalf("synchronous range read = %v, want source read on the consumer's native call stack", err)
	}
}
