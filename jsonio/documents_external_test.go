package jsonio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

type streamDocument struct {
	Index uint64 `json:"index"`
	Value uint8  `json:"value"`
}

func (d streamDocument) Validate() error {
	if d.Index > 1000000 {
		return core.ErrPrimitiveContract
	}
	return nil
}

func TestJSONDocumentsHostileAdmissionMatrix(t *testing.T) {
	t.Parallel()
	const valid = `{"index":1,"value":2}`
	first := streamDocument{Index: 1, Value: 2}
	cases := []struct {
		name, body string
		maximum    uint64
		want       []streamDocument
		refuse     bool
	}{
		{"empty", "", 128, nil, false},
		{"whitespace_only", " \r\n\t", 128, nil, false},
		{"canonical", valid, 128, []streamDocument{first}, false},
		{"adjacent_documents", valid + valid, 128, []streamDocument{first, first}, false},
		{"separated_documents", valid + "\n \t" + valid, 128, []streamDocument{first, first}, false},
		{"leading_whitespace", " \t" + valid, 128, []streamDocument{first}, false},
		{"trailing_whitespace", valid + " \n", 128, []streamDocument{first}, false},
		{"reordered_fields", `{"value":2,"index":1}`, 128, []streamDocument{first}, false},
		{"zero_fields", `{}`, 128, []streamDocument{{}}, false},
		{"maximum_byte", `{"value":255}`, 128, []streamDocument{{Value: 255}}, false},
		{"maximum_index", `{"index":1000000}`, 128, []streamDocument{{Index: 1000000}}, false},
		{"unknown_field", `{"extra":1}`, 128, nil, true},
		{"case_variant_unknown_field", `{"Index":1}`, 128, nil, true},
		{"duplicate_field", `{"index":1,"index":2}`, 128, nil, true},
		{"case_fold_duplicate", `{"index":1,"INDEX":2}`, 128, nil, true},
		{"escaped_duplicate", `{"index":1,"\u0069ndex":2}`, 128, nil, true},
		{"null_document", `null`, 128, nil, true},
		{"null_optional_index_uses_owner_zero_policy", `{"index":null}`, 128, []streamDocument{{}}, false},
		{"null_optional_value_uses_owner_zero_policy", `{"value":null}`, 128, []streamDocument{{}}, false},
		{"root_array", `[]`, 128, nil, true},
		{"root_string", `"text"`, 128, nil, true},
		{"root_boolean", `true`, 128, nil, true},
		{"root_number", `1`, 128, nil, true},
		{"negative_index", `{"index":-1}`, 128, nil, true},
		{"fractional_index", `{"index":1.5}`, 128, nil, true},
		{"index_overflow", `{"index":18446744073709551616}`, 128, nil, true},
		{"index_invariant_refusal", `{"index":1000001}`, 128, nil, true},
		{"byte_overflow", `{"value":256}`, 128, nil, true},
		{"negative_byte", `{"value":-1}`, 128, nil, true},
		{"fractional_byte", `{"value":0.5}`, 128, nil, true},
		{"quoted_number", `{"value":"2"}`, 128, nil, true},
		{"boolean_field", `{"value":true}`, 128, nil, true},
		{"array_field", `{"value":[]}`, 128, nil, true},
		{"object_field", `{"value":{}}`, 128, nil, true},
		{"trailing_comma", `{"index":1,}`, 128, nil, true},
		{"missing_value", `{"index":}`, 128, nil, true},
		{"missing_colon", `{"index" 1}`, 128, nil, true},
		{"missing_close", `{"index":1`, 128, nil, true},
		{"single_quoted_key", `{'index':1}`, 128, nil, true},
		{"unquoted_key", `{index:1}`, 128, nil, true},
		{"invalid_UTF8_key", "{\"\xff\":1}", 128, nil, true},
		{"unpaired_surrogate", `{"\ud800":1}`, 128, nil, true},
		{"NUL_after_prefix", valid + "\x00", 128, []streamDocument{first}, true},
		{"garbage_after_prefix", valid + "broken", 128, []streamDocument{first}, true},
		{"truncated_tail", valid + `{"index":`, 128, []streamDocument{first}, true},
		{"unknown_tail", valid + `{"extra":1}`, 128, []streamDocument{first}, true},
		{"invalid_tail_is_not_skipped", valid + `null` + valid, 128, []streamDocument{first}, true},
		{"exact_extent", valid, uint64(len(valid)), []streamDocument{first}, false},
		{"one_over_extent", valid, uint64(len(valid) - 1), nil, true},
		{"leading_whitespace_counts_toward_extent", " " + valid, uint64(len(valid)), nil, true},
		{"exact_extent_many_documents", strings.Repeat(valid, 17), uint64(len(valid)), slices.Repeat([]streamDocument{first}, 17), false},
		{"oversize_tail_preserves_prefix", valid + `{"index":1000000,"value":255}`, uint64(len(valid)), []streamDocument{first}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limits := core.DefaultStrictJSONLimits()
			var err error
			limits.DocumentMaximumBytes, err = core.NewByteCount(tc.maximum)
			if err != nil {
				t.Fatal(err)
			}
			var got []streamDocument
			refusals := 0
			for document, err := range jsonio.Documents[streamDocument](t.Context(), jsonio.Request{Source: strings.NewReader(tc.body), Limits: limits}) {
				if err != nil {
					if !errors.Is(err, core.ErrJSONContract) || document != (streamDocument{}) || refusals != 0 {
						t.Fatalf("refusal = %v/%v after %d refusals", document, err, refusals)
					}
					refusals++
					continue
				}
				if refusals != 0 {
					t.Fatal("admitted document after terminal refusal")
				}
				got = append(got, document)
			}
			if !slices.Equal(got, tc.want) || (refusals != 0) != tc.refuse {
				t.Fatalf("documents/refusals = %v/%d, want %v/%t", got, refusals, tc.want, tc.refuse)
			}
		})
	}
}

func TestJSONDocumentsReadLifetime(t *testing.T) {
	t.Parallel()
	const body = `{"index":1,"value":2}`
	for _, terminal := range []error{io.ErrUnexpectedEOF, io.ErrClosedPipe, context.Canceled, context.DeadlineExceeded, errors.Join(io.EOF, io.ErrClosedPipe)} {
		source := io.MultiReader(strings.NewReader(body), iotest.ErrReader(terminal))
		count, refusals := 0, 0
		for document, err := range jsonio.Documents[streamDocument](t.Context(), jsonio.Request{Source: source, Limits: core.DefaultStrictJSONLimits()}) {
			if err != nil {
				if !errors.Is(err, terminal) || document != (streamDocument{}) {
					t.Fatalf("terminal read = %v/%v, want cause %v", document, err, terminal)
				}
				refusals++
				continue
			}
			count++
		}
		if count != 1 || refusals != 1 {
			t.Fatalf("read observations = %d/%d, want prefix plus one refusal", count, refusals)
		}
	}
	source := &jsonStreamReadCounter{Reader: strings.NewReader(strings.Repeat(body, 100))}
	for document, err := range jsonio.Documents[streamDocument](t.Context(), jsonio.Request{Source: source, Limits: core.DefaultStrictJSONLimits()}) {
		if err != nil || document.Index != 1 {
			t.Fatalf("first document = %v/%v", document, err)
		}
		break
	}
	if source.calls != 1 {
		t.Fatalf("stopped consumer read calls = %d, want one", source.calls)
	}
	for _, cancelled := range []bool{false, true} {
		duration := temporal.Duration{}
		if cancelled {
			var err error
			duration, err = temporal.DurationFromSeconds(60)
			if err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
		if err != nil {
			t.Fatal(err)
		}
		want := context.DeadlineExceeded
		if cancelled {
			cancel()
			want = context.Canceled
		}
		source := &jsonStreamReadCounter{Reader: strings.NewReader(body)}
		refusals := 0
		for document, err := range jsonio.Documents[streamDocument](ctx, jsonio.Request{Source: source, Limits: core.DefaultStrictJSONLimits()}) {
			if document != (streamDocument{}) || !errors.Is(err, want) {
				t.Fatalf("terminal context = %v/%v, want %v", document, err, want)
			}
			refusals++
		}
		cancel()
		if source.calls != 0 || refusals != 1 {
			t.Fatalf("terminal context reads/refusals = %d/%d", source.calls, refusals)
		}
	}
}

type jsonStreamReadCounter struct {
	io.Reader
	calls int
}

func (r *jsonStreamReadCounter) Read(p []byte) (int, error) { r.calls++; return r.Reader.Read(p) }

func FuzzJSONDocumentsByteConservation(f *testing.F) {
	f.Add([]byte{0, 1, 127, 255}, uint8(1))
	f.Add([]byte{}, uint8(17))
	f.Add(bytes.Repeat([]byte{42}, 4096), uint8(7))
	f.Fuzz(func(t *testing.T, data []byte, chunk uint8) {
		source := &generatedJSONDocumentReader{data: data, chunk: int(chunk) + 1}
		seen := 0
		for document, err := range jsonio.Documents[streamDocument](t.Context(), jsonio.Request{Source: source, Limits: core.DefaultStrictJSONLimits()}) {
			if err != nil {
				t.Fatal(err)
			}
			if seen >= len(data) || document.Index != uint64(seen%1000001) || document.Value != data[seen] {
				t.Fatalf("document %d = %+v differs from source", seen, document)
			}
			seen++
		}
		if seen != len(data) {
			t.Fatalf("documents = %d, want %d", seen, len(data))
		}
	})
}

type generatedJSONDocumentReader struct {
	data         []byte
	pending      []byte
	index, chunk int
}

func (r *generatedJSONDocumentReader) Read(p []byte) (int, error) {
	if len(r.pending) == 0 {
		if r.index == len(r.data) {
			return 0, io.EOF
		}
		encoded, err := core.MarshalCanonicalJSONDocument(streamDocument{Index: uint64(r.index % 1000001), Value: r.data[r.index]})
		if err != nil {
			return 0, err
		}
		r.pending = encoded
		r.index++
	}
	n := copy(p[:min(len(p), r.chunk)], r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
