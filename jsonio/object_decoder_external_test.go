package jsonio_test

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type decoderNativeRefusal struct{ source io.Reader }

type decoderNativeAtRefusal struct{ source io.ReaderAt }

func (r decoderNativeAtRefusal) ReadAt(data []byte, offset int64) (int, error) {
	n, err := r.source.ReadAt(data, offset)
	return n, errors.Join(err, io.ErrClosedPipe)
}

func (r decoderNativeRefusal) Read(data []byte) (int, error) {
	n, err := r.source.Read(data)
	return n, errors.Join(err, io.ErrClosedPipe)
}

func TestJSONObjectDecoderNativeResetAdmissionMatrix(t *testing.T) {
	t.Parallel()
	for _, count := range []int{0, 1, 2, 3, 8, 64, 128, 129} {
		for mode := 0; mode < 8; mode++ {
			t.Run(fmt.Sprintf("objects-%d/mode-%d", count, mode), func(t *testing.T) {
				t.Parallel()
				var raw bytes.Buffer
				for i := 0; i < count; i++ {
					data, err := jsonv2.Marshal(scalarObject{Name: fmt.Sprintf("row-%d", i), Count: uint64(i)})
					if err != nil {
						t.Fatal(err)
					}
					raw.Write(data)
					raw.WriteByte('\n')
				}
				switch mode {
				case 1:
					raw.WriteString("{}")
				case 2:
					raw.WriteString(`{"Name":"tail","Unknown":1}`)
				case 3:
					raw.WriteString(`{"Name":"tail","Name":"other"}`)
				case 4:
					raw.WriteString(`{"Name":null}`)
				case 7:
					raw.WriteString(`{"Name":"truncated`)
				}
				path := filepath.Join(t.TempDir(), "objects")
				if err := os.WriteFile(path, raw.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				file, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: strings.NewReader("broken")})
				if err != nil {
					t.Fatal(err)
				}
				if object, err := decoder.Decode(t.Context()); err == nil || object != (scalarObject{}) {
					t.Fatalf("unowned malformed prefix %+v/%v", object, err)
				}
				ctx := t.Context()
				var cancel context.CancelFunc
				var source io.Reader = iotest.OneByteReader(io.NewSectionReader(file, 0, int64(raw.Len())))
				if mode == 6 {
					source = decoderNativeRefusal{source: io.NewSectionReader(file, 0, int64(raw.Len()))}
				}
				if err := decoder.Reset(ctx, jsonio.ObjectSourceRequest{Source: source}); err != nil {
					t.Fatal(err)
				}
				if mode == 5 {
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				seen := 0
				var terminal error
				for {
					object, err := decoder.Decode(ctx)
					if err != nil {
						terminal = err
						if object != (scalarObject{}) {
							t.Fatal("refusal retained partial object")
						}
						break
					}
					if object.Name != fmt.Sprintf("row-%d", seen) || object.Count != uint64(seen) {
						t.Fatalf("row%d value%+v", seen, object)
					}
					seen++
				}
				if mode == 0 {
					if terminal != io.EOF || seen != count {
						t.Fatalf("closure%v count%d want%d", terminal, seen, count)
					}
				} else {
					cause := error(core.ErrJSONContract)
					if mode == 5 {
						cause = context.Canceled
					}
					if mode == 6 {
						cause = io.ErrClosedPipe
					}
					if !errors.Is(terminal, cause) {
						t.Fatalf("refusal%v lost%v", terminal, cause)
					}
					if mode != 5 && mode != 6 && seen != count {
						t.Fatalf("prefix count%d want%d", seen, count)
					}
				}
				// The same decoder must recover by resetting Go's own engine,
				// including after EOF, context refusal, parse or native IO failure.
				if err := decoder.Reset(t.Context(), jsonio.ObjectSourceRequest{Source: strings.NewReader(`{"Name":"after","Count":7}`)}); err != nil {
					t.Fatal(err)
				}
				object, err := decoder.Decode(t.Context())
				if err != nil || object != (scalarObject{Name: "after", Count: 7}) {
					t.Fatalf("reset object%+v error%v", object, err)
				}
				if _, err := decoder.Decode(t.Context()); err != io.EOF {
					t.Fatalf("reset closure%v", err)
				}
				if _, err := file.Stat(); err != nil {
					t.Fatalf("decoder closed borrowed file: %v", err)
				}
			})
		}
	}
}

func TestJSONObjectDecoderNilAndInvalidResetRefusals(t *testing.T) {
	t.Parallel()
	var absent *jsonio.ObjectDecoder[scalarObject]
	if object, err := absent.Decode(t.Context()); !errors.Is(err, core.ErrJSONContract) || object != (scalarObject{}) {
		t.Fatalf("nil decode %+v/%v", object, err)
	}
	if err := absent.Reset(t.Context(), jsonio.ObjectSourceRequest{Source: strings.NewReader("{}")}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil reset%v", err)
	}
	if decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{}); decoder != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil source%v/%v", decoder, err)
	}
	if decoder, err := jsonio.NewObjectDecoder[opaqueRootObject](jsonio.ObjectSourceRequest{Source: strings.NewReader("{}")}); decoder != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("opaque root%v/%v", decoder, err)
	}
	decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: strings.NewReader(`{"Name":"kept"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Reset(t.Context(), jsonio.ObjectSourceRequest{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("invalid reset%v", err)
	}
	object, err := decoder.Decode(t.Context())
	if err != nil || object.Name != "kept" {
		t.Fatalf("refused reset changed source%+v/%v", object, err)
	}
}

func TestJSONObjectDecoderPreservesNativeRefusalAlongsideCompleteBytes(t *testing.T) {
	t.Parallel()
	data := []byte(`{"Name":"one","Count":1}`)
	path := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []int{0, 1, 8, len(data) - 1, len(data)} {
		t.Run(fmt.Sprint(prefix), func(t *testing.T) {
			t.Parallel()
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			var source io.Reader = io.NewSectionReader(decoderNativeAtRefusal{source: file}, 0, int64(prefix))
			if prefix == 0 {
				source = decoderNativeRefusal{source: source}
			}
			decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: source})
			if err != nil {
				t.Fatal(err)
			}
			var refused error
			for i := 0; i < 2; i++ {
				_, err := decoder.Decode(t.Context())
				if err != nil {
					refused = err
					break
				}
			}
			if !errors.Is(refused, io.ErrClosedPipe) || !errors.Is(refused, core.ErrJSONContract) {
				t.Fatalf("native refusal lost: %v", refused)
			}
		})
	}
}

func FuzzJSONObjectDecoderResetMatchesIndependentV2Meaning(f *testing.F) {
	for _, seed := range []string{"", "ascii", "quote\"\n雪😀", strings.Repeat("x", 4097)} {
		f.Add(seed, uint64(1))
	}
	f.Fuzz(func(t *testing.T, data string, count uint64) {
		// Native wire ingress carries valid UTF-8. Normalize before both
		// observations; every mutated scalar still reaches the complete codec.
		name := strings.ToValidUTF8(data, "�")
		want := scalarObject{Name: name, Count: count}
		raw, err := jsonv2.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: strings.NewReader("null")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decoder.Decode(t.Context()); err == nil {
			t.Fatal("accepted root null")
		}
		if err := decoder.Reset(t.Context(), jsonio.ObjectSourceRequest{Source: bytes.NewReader(raw)}); err != nil {
			t.Fatal(err)
		}
		got, err := decoder.Decode(t.Context())
		if name == "" {
			if got != (scalarObject{}) || !errors.Is(err, core.ErrSourceObservationContract) {
				t.Fatalf("empty meaning%+v/%v", got, err)
			}
			return
		}
		if err != nil || got != want {
			t.Fatalf("decoded%+v/%v want%+v", got, err, want)
		}
		if got, err := decoder.Decode(t.Context()); got != (scalarObject{}) || err != io.EOF {
			t.Fatalf("closure%+v/%v", got, err)
		}
	})
}

func BenchmarkJSONObjectDecoderResetScalar(b *testing.B) {
	data := []byte(`{"Name":"row","Count":18446744073709551615}`)
	decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: bytes.NewReader(data)})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := decoder.Reset(b.Context(), jsonio.ObjectSourceRequest{Source: bytes.NewReader(data)}); err != nil {
			b.Fatal(err)
		}
		got, err := decoder.Decode(b.Context())
		if err != nil || got.Name != "row" || got.Count != ^uint64(0) {
			b.Fatalf("decoded%+v/%v", got, err)
		}
		if _, err := decoder.Decode(b.Context()); err != io.EOF {
			b.Fatalf("closure%v", err)
		}
	}
}
