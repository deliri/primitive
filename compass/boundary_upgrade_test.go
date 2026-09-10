package compass_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
)

const compassWhitespaceFixtureBytes = 2 << 20

func TestCompassDocumentExtentLayerTriad(t *testing.T) {
	t.Parallel()
	want := compass.Configuration{Project: projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, 3)}
	canonical := encodedConfiguration(t, want.Project)
	gap := bytes.Repeat([]byte(" "), compassWhitespaceFixtureBytes)
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{"canonical", canonical, nil},
		{"large_prefix", append(bytes.Clone(gap), canonical...), nil},
		{"large_interior", append(append([]byte{'{'}, gap...), canonical[1:]...), nil},
		{"large_suffix", append(bytes.Clone(canonical), gap...), nil},
		{"neutral_whitespace", gap, core.ErrJSONContract},
		{"trailing_document_after_gap", append(append(bytes.Clone(canonical), gap...), []byte("{}")...), core.ErrJSONContract},
		{"truncated_after_prefix", append(bytes.Clone(gap), canonical[:len(canonical)-1]...), core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := compass.Decode[compass.Configuration](bytes.NewReader(tc.data))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (compass.Configuration{}) || !errors.Is(err, core.ErrCompassContract) {
					t.Fatalf("got refused value=%v error=%v, want zero and Compass identity", got, err)
				}
				return
			}
			if got != want {
				t.Fatalf("got=%v, want exact %v", got, want)
			}
		})
	}
}

func TestProjectNameAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		wantErr    error
	}{
		{"minimum_name", "P", nil},
		{"unicode_name", "Évidence", nil},
		{"internal_space", "Evidence Tool", nil},
		{"escaped_quote_and_backslash", "A\"\\B", nil},
		{"decomposed_unicode_remains_exact", "e\u0301", nil},
		{"beyond_old_byte_ceiling", strings.Repeat("n", 129), nil},
		{"large_multibyte_name", strings.Repeat("界", 4096), nil},
		{"neutral_empty", "", core.ErrCompassContract},
		{"leading_ascii_space", " P", core.ErrCompassContract},
		{"trailing_unicode_space", "P\u00a0", core.ErrCompassContract},
		{"embedded_newline", "A\nB", core.ErrCompassContract},
		{"embedded_null", "A\x00B", core.ErrCompassContract},
		{"unicode_control", "A\u0085B", core.ErrCompassContract},
		{"invalid_utf8", "\xff", core.ErrCompassContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := compass.ParseProjectName(tc.text)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got name error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (compass.ProjectName{}) || got.String() != "" {
					t.Fatalf("got refused name=%v, want zero", got)
				}
				encoded, err := got.MarshalJSON()
				if encoded != nil || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrCompassContract) {
					t.Fatalf("got zero name JSON bytes=%d error=%v, want nil typed refusal", len(encoded), err)
				}
				return
			}
			if got.String() != tc.text {
				t.Fatalf("got text=%q, want unchanged %q", got.String(), tc.text)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded compass.ProjectName
			if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
				t.Fatalf("got JSON round trip=%v error=%v, want %v", decoded, err, got)
			}
		})
	}
}

type compassTerminalReader struct {
	data  []byte
	cause error
}

func (r *compassTerminalReader) Read(destination []byte) (int, error) {
	n := copy(destination, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.cause
	}
	return n, nil
}

type compassBadCountReader struct{ count int }

func (r compassBadCountReader) Read(destination []byte) (int, error) {
	if r.count < 0 {
		return r.count, nil
	}
	return len(destination) + 1, nil
}

func TestCompassReaderFailureLayerTriad(t *testing.T) {
	t.Parallel()
	want := compass.Configuration{Project: projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, 3)}
	canonical := encodedConfiguration(t, want.Project)
	for _, tc := range []struct {
		name       string
		makeReader func() io.Reader
		wantErr    error
	}{
		{"data_and_eof", func() io.Reader { return &compassTerminalReader{data: canonical, cause: io.EOF} }, nil},
		{"one_byte_reads", func() io.Reader { return iotest.OneByteReader(bytes.NewReader(canonical)) }, nil},
		{"native_failure_after_complete_json", func() io.Reader { return &compassTerminalReader{data: canonical, cause: io.ErrClosedPipe} }, io.ErrClosedPipe},
		{"cancellation_and_eof_after_json", func() io.Reader {
			return &compassTerminalReader{data: canonical, cause: errors.Join(io.EOF, context.Canceled)}
		}, context.Canceled},
		{"wrapped_eof_is_not_clean_completion", func() io.Reader {
			return &compassTerminalReader{data: canonical, cause: fmt.Errorf("reader: %w", io.EOF)}
		}, io.EOF},
		{"no_progress", func() io.Reader { return &compassTerminalReader{} }, io.ErrNoProgress},
		{"negative_count", func() io.Reader { return compassBadCountReader{count: -1} }, core.ErrJSONContract},
		{"overreported_count", func() io.Reader { return compassBadCountReader{} }, core.ErrJSONContract},
		{"absent_source", func() io.Reader { return nil }, core.ErrJSONContract},
		{"typed_nil_source", func() io.Reader { return (*bytes.Reader)(nil) }, core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := compass.Decode[compass.Configuration](tc.makeReader())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (compass.Configuration{}) || !errors.Is(err, core.ErrCompassContract) || !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("got value=%v error=%v, want zero and typed refusal", got, err)
				}
				return
			}
			if got != want {
				t.Fatalf("got=%v, want exact %v", got, want)
			}
		})
	}
}

type extendedCompass struct {
	Project compass.Project       `json:"project"`
	Names   []compass.ProjectName `json:"names"`
}

func (c extendedCompass) Validate() error {
	if err := c.Project.Validate(); err != nil {
		return err
	}
	for _, name := range c.Names {
		if err := name.Validate(); err != nil {
			return err
		}
	}
	if len(c.Names) == 0 {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func TestCompassCallerOwnedDocumentLayerTriad(t *testing.T) {
	t.Parallel()
	project := projectFixture(t, "Project", "example.com/project", "owner/project", 2026, 1, 3)
	for _, tc := range []struct {
		name    string
		count   int
		wantErr error
	}{
		{"one_typed_name", 1, nil},
		{"array_beyond_old_default", 1025, nil},
		{"neutral_missing_names", 0, io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := extendedCompass{Project: project, Names: make([]compass.ProjectName, tc.count)}
			for i := range want.Names {
				want.Names[i] = project.Name
			}
			data, err := core.MarshalCanonicalJSONDocument(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := compass.Decode[extendedCompass](bytes.NewReader(data))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got decode error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Project != (compass.Project{}) || got.Names != nil || !errors.Is(err, core.ErrCompassContract) {
					t.Fatalf("got rejected value=%v error=%v, want zero and caller validation cause", got, err)
				}
				return
			}
			if got.Project != want.Project || len(got.Names) != len(want.Names) {
				t.Fatalf("got=%v, want exact caller-owned document", got)
			}
			for i := range want.Names {
				if got.Names[i] != want.Names[i] {
					t.Fatalf("got name[%d]=%v, want %v", i, got.Names[i], want.Names[i])
				}
			}
		})
	}
}
