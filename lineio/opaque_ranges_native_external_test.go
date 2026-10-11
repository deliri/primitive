package lineio_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

type opaqueReadFailure struct{ *bytes.Reader }

func (s opaqueReadFailure) ReadAt(data []byte, offset int64) (int, error) {
	count, err := s.Reader.ReadAt(data, offset)
	if count != 0 {
		return count, errors.Join(io.EOF, io.ErrUnexpectedEOF, err)
	}
	return count, err
}

func opaqueRange(t testing.TB, offset, extent int) lineio.RecordRange {
	t.Helper()
	length, err := core.NewByteLength(uint64(extent))
	if err != nil {
		t.Fatal(err)
	}
	return lineio.RecordRange{Offset: lineio.SourceByteOffset(offset), Bytes: length, Framing: lineio.RecordFramingOpaque}
}

func TestOpaqueRangesConserveNativeBytesAcrossPhysicalLines(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			extent := [...]int{1, 15, 16, 63, 4095, 4096, 4097, 8193}[index%8]
			body := bytes.Repeat([]byte{0, '\n', 255, '\r', '%', '=', '&', byte(index)}, (extent+7)/8)[:extent]
			prefix := bytes.Repeat([]byte{0xee}, index/8)
			data := append(append(bytes.Clone(prefix), body...), 0xaa, '\n')
			source := bytes.NewReader(data)
			buffer, err := core.NewByteCount(uint64([...]int{1, 16, 4096, 32768}[index/8%4]))
			if err != nil {
				t.Fatal(err)
			}
			request := lineio.RecordFragmentRequest{Source: source, Record: opaqueRange(t, len(prefix), len(body)), BufferBytes: buffer}
			var actual bytes.Buffer
			for fragment, err := range lineio.RecordFragments(t.Context(), request) {
				if err != nil || fragment.Validate() != nil {
					t.Fatalf("native span fragment: %v", err)
				}
				_, _ = actual.Write(fragment.Bytes)
			}
			if !bytes.Equal(actual.Bytes(), body) {
				t.Fatal("opaque span changed native bytes or crossed source coordinates")
			}
			oracle := bufio.NewReader(bytes.NewReader(body))
			offset := len(prefix)
			for character, err := range lineio.RecordCharacters(t.Context(), request) {
				value, width, nativeErr := oracle.ReadRune()
				observedWidth, widthErr := character.Bytes.Int64()
				if err != nil || nativeErr != nil || widthErr != nil || character.Validate() != nil || character.Value != lineio.RecordRune(value) || observedWidth != int64(width) || int64(character.Offset) != int64(offset) {
					t.Fatalf("opaque native rune disagreement: %+v/%v/%v", character, err, nativeErr)
				}
				offset += width
			}
			if _, _, err := oracle.ReadRune(); err != io.EOF || offset != len(prefix)+len(body) {
				t.Fatal("opaque rune replay lost source bytes")
			}
			short := request
			short.Record = opaqueRange(t, len(prefix), len(body)+3)
			var shortErr error
			for _, err := range lineio.RecordFragments(t.Context(), short) {
				if err != nil {
					shortErr = err
					break
				}
			}
			if !errors.Is(shortErr, io.ErrUnexpectedEOF) || !errors.Is(shortErr, core.ErrLineIOScan) {
				t.Fatalf("opaque span invented missing native tail: %v", shortErr)
			}
			request.Source = opaqueReadFailure{Reader: bytes.NewReader(data)}
			published := 0
			var failure error
			for fragment, err := range lineio.RecordFragments(t.Context(), request) {
				published += len(fragment.Bytes)
				if err != nil {
					failure = err
					break
				}
			}
			if published != 0 || !errors.Is(failure, io.ErrUnexpectedEOF) || !errors.Is(failure, core.ErrLineIOScan) {
				t.Fatalf("complete-byte native failure escaped: bytes=%d err=%v", published, failure)
			}
		})
	}
}

func FuzzOpaqueRangeNativeByteConservation(f *testing.F) {
	f.Add([]byte{'\n', 0, 255, '\n'}, uint8(0))
	f.Add([]byte{}, uint8(255))
	f.Fuzz(func(t *testing.T, body []byte, prefixBytes uint8) {
		prefix := bytes.Repeat([]byte{0xee}, int(prefixBytes))
		data := append(append(bytes.Clone(prefix), body...), 0xaa)
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			t.Fatal(err)
		}
		request := lineio.RecordFragmentRequest{Source: bytes.NewReader(data), Record: opaqueRange(t, len(prefix), len(body)), BufferBytes: buffer}
		var actual bytes.Buffer
		var failure error
		for fragment, err := range lineio.RecordFragments(t.Context(), request) {
			if err != nil {
				failure = err
				break
			}
			if err := fragment.Validate(); err != nil {
				t.Fatalf("admitted fragment invalid: %v", err)
			}
			_, _ = actual.Write(fragment.Bytes)
		}
		if len(body) == 0 {
			if !errors.Is(failure, core.ErrLineIOContract) || actual.Len() != 0 {
				t.Fatalf("empty span invented native bytes: %v", failure)
			}
			return
		}
		if failure != nil || !bytes.Equal(actual.Bytes(), body) {
			t.Fatalf("native span disagreement: %v", failure)
		}
		oracle := bufio.NewReader(bytes.NewReader(body))
		offset := int64(len(prefix))
		for character, err := range lineio.RecordCharacters(t.Context(), request) {
			value, width, nativeErr := oracle.ReadRune()
			observedWidth, widthErr := character.Bytes.Int64()
			if err != nil || nativeErr != nil || widthErr != nil || character.Validate() != nil || character.Value != lineio.RecordRune(value) || observedWidth != int64(width) || int64(character.Offset) != offset {
				t.Fatalf("mutated native rune disagreement: %+v/%v/%v", character, err, nativeErr)
			}
			offset += int64(width)
		}
		if _, _, err := oracle.ReadRune(); err != io.EOF || offset != int64(len(prefix)+len(body)) {
			t.Fatal("mutated opaque rune replay lost source bytes")
		}
	})
}
