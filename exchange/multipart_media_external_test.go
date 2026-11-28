package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestMultipartPartMediaNativeLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("exact declared media", func(t *testing.T) {
		type fixturePart struct {
			name, filename, media, body string
		}
		want := []fixturePart{
			{"manifest", "manifest.json", "application/json", `{"version":1}`},
			{"bundle", "demo-\"fixture.zip", "application/zip", "PK\x03\x04\x00\xff"},
			{"révision", "", "text/plain; charset=utf-8", "révision-1"},
		}
		var parts []exchange.MultipartPart
		for _, fixture := range want {
			media, err := core.ParseHTTPMediaType(fixture.media)
			if err != nil {
				t.Fatal(err)
			}
			part := exchange.MultipartPart{Name: multipartFieldName(t, fixture.name), ContentType: media}
			if fixture.filename == "" {
				part.Kind, part.Text = exchange.MultipartPartField, multipartFieldText(t, fixture.body)
			} else {
				part.Kind, part.FileName = exchange.MultipartPartFile, multipartFilename(t, fixture.filename)
				part.Source = io.NopCloser(strings.NewReader(fixture.body))
			}
			parts = append(parts, part)
		}
		err := exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
			Parts: slices.Values(parts),
			Use: func(_ context.Context, source exchange.MultipartSource) error {
				_, parameters, err := mime.ParseMediaType(source.ContentType.String())
				if err != nil {
					return err
				}
				reader := multipart.NewReader(source.Reader, parameters["boundary"])
				for _, fixture := range want {
					part, err := reader.NextRawPart()
					if err != nil {
						return err
					}
					body, err := io.ReadAll(part)
					if err != nil {
						return err
					}
					media := part.Header.Values("Content-Type")
					if part.FormName() != fixture.name || part.FileName() != fixture.filename || !slices.Equal(media, []string{fixture.media}) || string(body) != fixture.body {
						t.Errorf("part = %q %q %v %q, want %+v", part.FormName(), part.FileName(), media, body, fixture)
					}
				}
				if _, err := reader.NextRawPart(); err != io.EOF {
					t.Errorf("multipart terminal = %v, want EOF", err)
				}
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("omit media retains all native bytes", func(t *testing.T) {
		const name, text, filename, data = "quoted-\"é\\", "text\x00\r\n", "é-\"sample.bin", "bytes\x00\xff"
		parts := []exchange.MultipartPart{
			{Kind: exchange.MultipartPartField, Name: multipartFieldName(t, name), Text: multipartFieldText(t, text)},
			{Kind: exchange.MultipartPartFile, Name: multipartFieldName(t, name), FileName: multipartFilename(t, filename), Source: io.NopCloser(strings.NewReader(data))},
		}
		err := exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
			Parts: slices.Values(parts),
			Use: func(_ context.Context, source exchange.MultipartSource) error {
				_, parameters, err := mime.ParseMediaType(source.ContentType.String())
				if err != nil {
					return err
				}
				body, err := io.ReadAll(source.Reader)
				if err != nil {
					return err
				}
				var expected bytes.Buffer
				legacy := multipart.NewWriter(&expected)
				if err := legacy.SetBoundary(parameters["boundary"]); err != nil {
					return err
				}
				if err := legacy.WriteField(name, text); err != nil {
					return err
				}
				legacyFile, err := legacy.CreateFormFile(name, filename)
				if err != nil {
					return err
				}
				if _, err := io.WriteString(legacyFile, data); err != nil {
					return err
				}
				if err := legacy.Close(); err != nil {
					return err
				}
				if !bytes.Equal(body, expected.Bytes()) {
					t.Errorf("multipart = %q, want legacy wire %q", body, expected.Bytes())
				}
				return nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wildcard refuses before file admission", func(t *testing.T) {
		for _, input := range []string{"*/*", "application/*", "*/zip"} {
			media, err := core.ParseHTTPMediaType(input)
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.CreateTemp(t.TempDir(), "unadmitted")
			if err != nil {
				t.Fatal(err)
			}
			tracked := &multipartTrackedFile{file: file}
			part := exchange.MultipartPart{
				Kind: exchange.MultipartPartFile, Name: multipartFieldName(t, "bundle"),
				FileName: multipartFilename(t, "demo.zip"), Source: tracked, ContentType: media,
			}
			err = exchange.WithMultipartSource(t.Context(), exchange.MultipartSourceRequest{
				Parts: slices.Values([]exchange.MultipartPart{part}),
				Use: func(_ context.Context, source exchange.MultipartSource) error {
					size, err := io.Copy(io.Discard, source.Reader)
					if size != 0 {
						t.Errorf("refused media wrote %d bytes", size)
					}
					return err
				},
			})
			if !errors.Is(err, core.ErrExchangeContract) || tracked.closed.Load() != 0 || tracked.maximum.Load() != 0 {
				t.Errorf("wildcard %q returned %v, reads=%d, closes=%d", input, err, tracked.maximum.Load(), tracked.closed.Load())
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("malformed media refuses at parse boundary", func(t *testing.T) {
		for _, input := range []string{"application/", "application/json\r\nX-Injected: 1"} {
			if parsed, err := core.ParseHTTPMediaType(input); err == nil || !parsed.IsZero() {
				t.Errorf("malformed media = %v, %v", parsed, err)
			}
		}
	})
}
