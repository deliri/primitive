package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCompareContentObservesExactBytesAcrossNativeWindows(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{0, 1, 32767, 32768, 32769, 131075} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			t.Parallel()
			expected := bytes.Repeat([]byte{0xf4}, extent)
			variants := []string{"equal", "longer source"}
			if extent > 0 {
				variants = append(variants, "shorter source", "changed final byte")
			}
			for _, variant := range variants {
				t.Run(variant, func(t *testing.T) {
					t.Parallel()
					actual := append([]byte{}, expected...)
					switch variant {
					case "longer source":
						actual = append(actual, 0xf4)
					case "shorter source":
						actual = actual[:len(actual)-1]
					case "changed final byte":
						actual[len(actual)-1] = 0x87
					}
					got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{
						Source: iotest.HalfReader(bytes.NewReader(actual)),
						Expected: func(ctx context.Context, destination io.Writer) error {
							for offset := 0; offset < len(expected); {
								next := min(offset+32769, len(expected))
								if _, err := destination.Write(expected[offset:next]); err != nil {
									return err
								}
								offset = next
							}
							return nil
						},
					})
					if err != nil || got.Equal != bytes.Equal(actual, expected) || got.ExpectedBytes.Uint64() != uint64(len(expected)) || got.ObservedBytes.Uint64() != uint64(len(actual)) {
						t.Fatalf("comparison = (%+v, %v), want native equality %t and complete extents %d/%d", got, err, bytes.Equal(actual, expected), len(expected), len(actual))
					}
					if err := got.Validate(); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

func TestCompareContentPreservesNativeFailuresWithoutPartialProof(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source func() io.Reader
		want   error
	}{
		{name: "source refusal before progress", source: func() io.Reader { return iotest.ErrReader(io.ErrClosedPipe) }, want: io.ErrClosedPipe},
		{name: "source refusal after expected bytes", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader([]byte("abc")), iotest.ErrReader(io.ErrClosedPipe))
		}, want: io.ErrClosedPipe},
		{name: "joined EOF remains a source failure", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader([]byte("abc")), iotest.ErrReader(errors.Join(io.EOF, io.ErrClosedPipe)))
		}, want: io.ErrClosedPipe},
		{name: "native unexpected EOF is not ordinary truncation", source: func() io.Reader { return iotest.ErrReader(io.ErrUnexpectedEOF) }, want: io.ErrUnexpectedEOF},
		{name: "mismatch does not hide a trailing source refusal", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader([]byte("xyz longer")), iotest.ErrReader(io.ErrClosedPipe))
		}, want: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: tc.source(), Expected: func(_ context.Context, destination io.Writer) error {
				_, err := destination.Write([]byte("abc"))
				return err
			}})
			if !errors.Is(err, tc.want) || !errors.Is(err, core.ErrFilestoreSource) || got != (filestore.ContentComparison{}) {
				t.Fatalf("failed comparison = (%+v, %v), want no proof and native source cause %v", got, err, tc.want)
			}
		})
	}
	t.Run("closed native file", func(t *testing.T) {
		t.Parallel()
		file, err := os.Create(filepath.Join(t.TempDir(), "closed-source"))
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: file, Expected: func(_ context.Context, destination io.Writer) error {
			_, err := destination.Write([]byte("abc"))
			return err
		}})
		var native *fs.PathError
		if !errors.Is(err, fs.ErrClosed) || !errors.As(err, &native) || got != (filestore.ContentComparison{}) {
			t.Fatalf("closed file comparison = (%+v, %v), want no proof and native closed-file error", got, err)
		}
	})
}

func TestCompareContentOwnsCallbackRefusalCancellationAndWriterLoan(t *testing.T) {
	t.Parallel()
	var escaped io.Writer
	actual := bytes.NewReader([]byte("abc"))
	got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: actual, Expected: func(_ context.Context, destination io.Writer) error {
		escaped = destination
		_, err := destination.Write([]byte("abc"))
		return err
	}})
	if err != nil || !got.Equal {
		t.Fatalf("comparison = (%+v, %v), want equality", got, err)
	}
	if count, err := escaped.Write([]byte("late")); count != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expired loan = (%d, %v), want zero and closed pipe", count, err)
	}
	got, err = filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: bytes.NewReader(nil), Expected: func(_ context.Context, destination io.Writer) error {
		escaped = destination
		return io.ErrUnexpectedEOF
	}})
	if !errors.Is(err, io.ErrUnexpectedEOF) || got != (filestore.ContentComparison{}) {
		t.Fatalf("producer refusal = (%+v, %v), want no proof and original cause", got, err)
	}
	if _, err := escaped.Write(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed callback loan = %v, want closed pipe", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	got, err = filestore.CompareContent(ctx, filestore.CompareContentRequest{Source: bytes.NewReader([]byte("abc")), Expected: func(_ context.Context, destination io.Writer) error {
		cancel()
		_, _ = destination.Write([]byte("abc"))
		return nil
	}})
	if !errors.Is(err, context.Canceled) || got != (filestore.ContentComparison{}) {
		t.Fatalf("swallowed cancelled write = (%+v, %v), want no proof and cancellation", got, err)
	}
	got, err = filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: iotest.ErrReader(io.ErrClosedPipe), Expected: func(_ context.Context, destination io.Writer) error {
		_, _ = destination.Write([]byte("abc"))
		return nil
	}})
	if !errors.Is(err, io.ErrClosedPipe) || got != (filestore.ContentComparison{}) {
		t.Fatalf("swallowed source failure = (%+v, %v), want no proof and native cause", got, err)
	}
}

func TestCompareContentRefusesAbsentContractsBeforeCallingProducer(t *testing.T) {
	t.Parallel()
	var absent *bytes.Reader
	for _, source := range []io.Reader{nil, absent} {
		got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: source, Expected: func(context.Context, io.Writer) error { t.Error("invalid source reached producer"); return nil }})
		if !errors.Is(err, core.ErrFilestoreContract) || got != (filestore.ContentComparison{}) {
			t.Fatalf("invalid source = (%+v, %v), want no proof and contract refusal", got, err)
		}
	}
	got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: bytes.NewReader(nil)})
	if !errors.Is(err, core.ErrFilestoreContract) || got != (filestore.ContentComparison{}) {
		t.Fatalf("absent producer = (%+v, %v), want no proof and contract refusal", got, err)
	}
}

func FuzzCompareContentMatchesNativeBytes(f *testing.F) {
	f.Add([]byte("typed streamed projection"), []byte("typed streamed projection"))
	f.Add([]byte{}, []byte{})
	f.Add([]byte("abc"), []byte("abd"))
	f.Add(bytes.Repeat([]byte{0xf4}, 32769), bytes.Repeat([]byte{0xf4}, 32768))
	f.Fuzz(func(t *testing.T, actual, expected []byte) {
		got, err := filestore.CompareContent(t.Context(), filestore.CompareContentRequest{Source: iotest.HalfReader(bytes.NewReader(actual)), Expected: func(_ context.Context, destination io.Writer) error {
			_, err := destination.Write(expected)
			return err
		}})
		if err != nil || got.Equal != bytes.Equal(actual, expected) || got.ExpectedBytes.Uint64() != uint64(len(expected)) || got.ObservedBytes.Uint64() != uint64(len(actual)) {
			t.Fatalf("comparison = (%+v, %v), want native equality %t and extents %d/%d", got, err, bytes.Equal(actual, expected), len(expected), len(actual))
		}
	})
}
