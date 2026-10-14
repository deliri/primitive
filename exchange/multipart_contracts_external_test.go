package exchange_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestMultipartNominalHeaderAndDataAdmissionPressure(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"model", "file", "a space", "quoted-\"name", "back\\slash", "雪", "café", " leading", "trailing ", "a.b", strings.Repeat("x", 8191), strings.Repeat("x", 8192), strings.Repeat("x", 8193), strings.Repeat("x", 32769)} {
		t.Run(fmt.Sprintf("admitted-field-%q-length-%d", value[:min(len(value), 24)], len(value)), func(t *testing.T) {
			t.Parallel()
			got, err := exchange.ParseMultipartName(value)
			if err != nil || got.Validate() != nil || got.String() != value {
				t.Fatalf("nominal field = (%q,%v), want exact admitted UTF8 without header extent quota", got.String(), err)
			}
		})
	}
	for _, value := range []string{"", "\x00", "line\nfeed", "carriage\rreturn", "tab\tname", "\x01", "\x1f", "\x7f", "\xff", "a\xffb", "\xc0\xaf"} {
		t.Run(fmt.Sprintf("refused-field-header-%q", value), func(t *testing.T) {
			t.Parallel()
			got, err := exchange.ParseMultipartName(value)
			if got != (exchange.MultipartName{}) || !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("nominal field refusal = (%q,%v), want zero/contract", got.String(), err)
			}
		})
	}
	for _, value := range []string{"workout.wav", "file.txt", "雪.wav", "with space.wav", "quoted\"file.wav", ".hidden", "a..b", "a:b", "x", strings.Repeat("x", 32769)} {
		got, err := exchange.ParseMultipartFileName(value)
		if err != nil || got.Validate() != nil || got.String() != value {
			t.Fatalf("filename = (%q,%v), want admitted exact data", got.String(), err)
		}
	}
	for _, value := range []string{"", ".", "..", "../file", "a/b", "a\\b", "\n", "\r", "\x00", "\xff"} {
		got, err := exchange.ParseMultipartFileName(value)
		if got != (exchange.MultipartFileName{}) || !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("filename refusal = (%q,%v), want zero/contract", got.String(), err)
		}
	}
	for _, value := range []string{"", "ordinary", "\x00\r\n\t", "雪", strings.Repeat("x", 32767), strings.Repeat("x", 32768), strings.Repeat("x", 32769)} {
		got, err := exchange.ParseMultipartText(value)
		if err != nil || got.Validate() != nil || got.String() != value {
			t.Fatalf("field data = (%q,%v), want exact admitted data without whitespace/control rewriting", got.String(), err)
		}
	}
	if got, err := exchange.ParseMultipartText("\xff"); got != (exchange.MultipartText{}) || !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("invalidUTF8 data = (%v,%v), want zero/contract", got, err)
	}
	if err := (exchange.MultipartText{}).Validate(); !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("unconstructed field data = %v, want contract", err)
	}
}

func TestMultipartFramingEnumExhaustsTypedByteDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(fmt.Sprintf("framing-byte-%d", raw), func(t *testing.T) {
			t.Parallel()
			kind := exchange.MultipartPartKind(raw)
			want := raw == int(exchange.MultipartPartField) || raw == int(exchange.MultipartPartFile)
			if kind.IsValid() != want || (kind.Validate() == nil) != want || (kind.String() != "") != want {
				t.Fatalf("framing byte%d = (%t,%v,%q), want exact closed admission%t", raw, kind.IsValid(), kind.Validate(), kind.String(), want)
			}
			if !want && !errors.Is(kind.Validate(), core.ErrExchangeContract) {
				t.Fatalf("framing refusal = %v, want contract", kind.Validate())
			}
			kind.OffWireEnum()
		})
	}
}

func FuzzMultipartNominalDoorsSemanticClosure(f *testing.F) {
	name, err := exchange.ParseMultipartName("model")
	if err != nil {
		f.Fatalf("name seed = %v, want nil", err)
	}
	filename, err := exchange.ParseMultipartFileName("workout.wav")
	if err != nil {
		f.Fatalf("filename seed = %v, want nil", err)
	}
	text, err := exchange.ParseMultipartText("gpt-4o-mini-transcribe")
	if err != nil {
		f.Fatalf("text seed = %v, want nil", err)
	}
	f.Add(name.String(), filename.String(), text.String(), uint8(exchange.MultipartPartField))
	f.Add("", "../file", "\xff", uint8(255))
	f.Add("header\r\ninjection", "file.wav", "\x00\r\n", uint8(exchange.MultipartPartFile))
	for _, size := range []int{32767, 32768, 32769} {
		f.Add(strings.Repeat("n", size), "file.wav", strings.Repeat("x", size), uint8(exchange.MultipartPartField))
	}
	f.Fuzz(func(t *testing.T, nameText, fileText, dataText string, rawKind uint8) {
		name, nameErr := exchange.ParseMultipartName(nameText)
		filename, fileErr := exchange.ParseMultipartFileName(fileText)
		text, textErr := exchange.ParseMultipartText(dataText)
		validHeader := func(value string) bool {
			if value == "" || !utf8.ValidString(value) {
				return false
			}
			for _, r := range value {
				if r < 0x20 || r == 0x7f {
					return false
				}
			}
			return true
		}
		wantName := validHeader(nameText)
		wantFile := validHeader(fileText) && fileText != "." && fileText != ".." && !strings.ContainsAny(fileText, "/\\")
		wantText := utf8.ValidString(dataText)
		if (nameErr == nil) != wantName || (fileErr == nil) != wantFile || (textErr == nil) != wantText {
			t.Fatalf("multipart admission = (%v,%v,%v), want exact header/file/text agreement(%t,%t,%t)", nameErr, fileErr, textErr, wantName, wantFile, wantText)
		}
		if !wantName {
			if name != (exchange.MultipartName{}) || !errors.Is(nameErr, core.ErrExchangeContract) {
				t.Fatalf("name refusal = (%v,%v), want zero/contract", name, nameErr)
			}
		} else {
			if name.Validate() != nil || name.String() != nameText {
				t.Fatalf("name closure = %q, want exact%q", name.String(), nameText)
			}
		}
		if !wantFile {
			if filename != (exchange.MultipartFileName{}) || !errors.Is(fileErr, core.ErrExchangeContract) {
				t.Fatalf("file refusal = (%v,%v), want zero/contract", filename, fileErr)
			}
		} else {
			if filename.Validate() != nil || filename.String() != fileText {
				t.Fatalf("filename closure = %q, want exact%q", filename.String(), fileText)
			}
		}
		if !wantText {
			if text != (exchange.MultipartText{}) || !errors.Is(textErr, core.ErrExchangeContract) {
				t.Fatalf("data refusal = (%v,%v), want zero/contract", text, textErr)
			}
		} else {
			if text.Validate() != nil || text.String() != dataText {
				t.Fatalf("data closure = %q, want exact%q", text.String(), dataText)
			}
		}
		kind := exchange.MultipartPartKind(rawKind)
		wantKind := kind == exchange.MultipartPartField || kind == exchange.MultipartPartFile
		if kind.IsValid() != wantKind || (kind.Validate() == nil) != wantKind || (!wantKind && !errors.Is(kind.Validate(), core.ErrExchangeContract)) {
			t.Fatalf("framing closure = (%v,%v), want exact membership%t", kind, kind.Validate(), wantKind)
		}
		source := io.NopCloser(strings.NewReader(""))
		part := exchange.MultipartPart{Kind: kind, Name: name, FileName: filename, Source: source}
		wantPart := kind == exchange.MultipartPartFile && wantName && wantFile
		if kind == exchange.MultipartPartField {
			part.Source = nil
			part.FileName = exchange.MultipartFileName{}
			part.Text = text
			wantPart = wantName && wantText
		}
		if err := part.Validate(); (err == nil) != wantPart || !wantPart && !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("part agreement = %v, want exact admitted%t", err, wantPart)
		}
	})
}

type multipartFuzzSource struct {
	reader multipartPatternSource
	closed *atomic.Int64
}

func (s *multipartFuzzSource) Read(data []byte) (int, error) { return s.reader.Read(data) }
func (s *multipartFuzzSource) Close() error                  { s.closed.Add(1); return nil }

type multipartTextConservationSink struct {
	value   string
	written int
}

func (s *multipartTextConservationSink) Write(data []byte) (int, error) {
	if len(data) > len(s.value)-s.written || string(data) != s.value[s.written:s.written+len(data)] {
		return 0, core.ErrExchangeContract
	}
	s.written += len(data)
	return len(data), nil
}

func FuzzMultipartNativeFramingAndCustodySemanticConservation(f *testing.F) {
	text, err := exchange.ParseMultipartText("gpt-4o-mini-transcribe")
	if err != nil {
		f.Fatalf("typed native field seed = %v, want admitted", err)
	}
	for _, extent := range []uint32{0, 32767, 32768, 32769, 3*32768 + 7} {
		f.Add(text.String(), extent, uint8(1))
	}
	f.Add("\x00\r\nUnicode-雪", uint32(1), uint8(7))
	f.Add("", uint32(0), uint8(0))
	f.Add("\xff", uint32(0), uint8(1))
	f.Fuzz(func(t *testing.T, data string, extentSelector uint32, countSelector uint8) {
		text, err := exchange.ParseMultipartText(data)
		if err != nil {
			if !errors.Is(err, core.ErrExchangeContract) || text != (exchange.MultipartText{}) {
				t.Fatalf("fuzz text refusal = (%v,%v), want zero/contract", text, err)
			}
			return
		}
		// These are finite fuzz-workload selections, not production admission caps.
		extent := int64(extentSelector % (3*32768 + 8))
		count := int(countSelector % 8)
		name, fileName := multipartFieldName(t, "model"), multipartFieldName(t, "file")
		filename := multipartFilename(t, "workout.wav")
		var closes atomic.Int64
		consumed := 0
		err = exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
			Parts: func(yield func(exchange.MultipartPart) bool) {
				if !yield(exchange.MultipartPart{Kind: exchange.MultipartPartField, Name: name, Text: text}) {
					return
				}
				for range count {
					source := &multipartFuzzSource{reader: multipartPatternSource{left: extent}, closed: &closes}
					if !yield(exchange.MultipartPart{Kind: exchange.MultipartPartFile, Name: fileName, FileName: filename, Source: source}) {
						return
					}
				}
			},
			Use: func(ctx context.Context, source exchange.MultipartSource) error {
				if err := source.Validate(); err != nil {
					return err
				}
				_, parameters, err := mime.ParseMediaType(source.ContentType.String())
				if err != nil {
					return err
				}
				decoder := multipart.NewReader(source.Reader, parameters["boundary"])
				part, err := decoder.NextPart()
				if err != nil {
					return err
				}
				sink := &multipartTextConservationSink{value: data}
				if _, err := io.CopyBuffer(sink, part, make([]byte, 32768)); err != nil {
					return err
				}
				if sink.written != len(data) || part.FormName() != name.String() || part.FileName() != "" {
					return core.ErrExchangeContract
				}
				for {
					part, err := decoder.NextPart()
					if err == io.EOF {
						break
					}
					if err != nil {
						return err
					}
					sink := &multipartPatternSink{}
					if _, err := io.CopyBuffer(sink, part, make([]byte, 32768)); err != nil {
						return err
					}
					if sink.written != extent || part.FormName() != fileName.String() || part.FileName() != filename.String() {
						return core.ErrExchangeContract
					}
					consumed++
				}
				_, err = io.Copy(io.Discard, source.Reader)
				return err
			},
		})
		if err != nil || consumed != count || closes.Load() != int64(count) {
			t.Fatalf("native framing = (%v,consumed%d,closes%d), want exact%d streams and one close per admitted source", err, consumed, closes.Load(), count)
		}
	})
}
