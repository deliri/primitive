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
		wantErr error
		name    string
		data    []byte
	}{
		{name: "canonical", data: canonical, wantErr: nil},
		{name: "large_prefix", data: append(bytes.Clone(gap), canonical...), wantErr: nil},
		{name: "large_interior", data: append(append([]byte{'{'}, gap...), canonical[1:]...), wantErr: nil},
		{name: "large_suffix", data: append(bytes.Clone(canonical), gap...), wantErr: nil},
		{name: "neutral_whitespace", data: gap, wantErr: core.ErrJSONContract},
		{name: "trailing_document_after_gap", data: append(append(bytes.Clone(canonical), gap...), []byte("{}")...), wantErr: core.ErrJSONContract},
		{name: "truncated_after_prefix", data: append(bytes.Clone(gap), canonical[:len(canonical)-1]...), wantErr: core.ErrJSONContract},
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
		wantErr    error
		name, text string
	}{
		{name: "minimum_name", text: "P", wantErr: nil},
		{name: "unicode_name", text: "Évidence", wantErr: nil},
		{name: "internal_space", text: "Evidence Tool", wantErr: nil},
		{name: "escaped_quote_and_backslash", text: "A\"\\B", wantErr: nil},
		{name: "decomposed_unicode_remains_exact", text: "e\u0301", wantErr: nil},
		{name: "beyond_old_byte_ceiling", text: strings.Repeat("n", 129), wantErr: nil},
		{name: "large_multibyte_name", text: strings.Repeat("界", 4096), wantErr: nil},
		{name: "neutral_empty", text: "", wantErr: core.ErrCompassContract},
		{name: "leading_ascii_space", text: " P", wantErr: core.ErrCompassContract},
		{name: "trailing_unicode_space", text: "P\u00a0", wantErr: core.ErrCompassContract},
		{name: "embedded_newline", text: "A\nB", wantErr: core.ErrCompassContract},
		{name: "embedded_null", text: "A\x00B", wantErr: core.ErrCompassContract},
		{name: "unicode_control", text: "A\u0085B", wantErr: core.ErrCompassContract},
		{name: "invalid_utf8", text: "\xff", wantErr: core.ErrCompassContract},
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
	cause error
	data  []byte
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
		wantErr    error
		makeReader func() io.Reader
		name       string
	}{
		{name: "data_and_eof", makeReader: func() io.Reader { return &compassTerminalReader{data: canonical, cause: io.EOF} }, wantErr: nil},
		{name: "one_byte_reads", makeReader: func() io.Reader { return iotest.OneByteReader(bytes.NewReader(canonical)) }, wantErr: nil},
		{name: "native_failure_after_complete_json", makeReader: func() io.Reader { return &compassTerminalReader{data: canonical, cause: io.ErrClosedPipe} }, wantErr: io.ErrClosedPipe},
		{name: "cancellation_and_eof_after_json", makeReader: func() io.Reader {
			return &compassTerminalReader{data: canonical, cause: errors.Join(io.EOF, context.Canceled)}
		}, wantErr: context.Canceled},
		{name: "wrapped_eof_is_not_clean_completion", makeReader: func() io.Reader {
			return &compassTerminalReader{data: canonical, cause: fmt.Errorf("reader: %w", io.EOF)}
		}, wantErr: io.EOF},
		{name: "no_progress", makeReader: func() io.Reader { return &compassTerminalReader{} }, wantErr: io.ErrNoProgress},
		{name: "negative_count", makeReader: func() io.Reader { return compassBadCountReader{count: -1} }, wantErr: core.ErrJSONContract},
		{name: "overreported_count", makeReader: func() io.Reader { return compassBadCountReader{} }, wantErr: core.ErrJSONContract},
		{name: "absent_source", makeReader: func() io.Reader { return nil }, wantErr: core.ErrJSONContract},
		{name: "typed_nil_source", makeReader: func() io.Reader { return (*bytes.Reader)(nil) }, wantErr: core.ErrJSONContract},
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
	Names   []compass.ProjectName `json:"names"`
	Project compass.Project       `json:"project"`
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
		wantErr error
		name    string
		count   int
	}{
		{name: "one_typed_name", count: 1, wantErr: nil},
		{name: "array_beyond_old_default", count: 1025, wantErr: nil},
		{name: "neutral_missing_names", count: 0, wantErr: io.ErrUnexpectedEOF},
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
