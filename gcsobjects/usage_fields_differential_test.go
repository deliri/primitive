package gcsobjects

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestGCSUsageCSVFieldIteratorNativeParity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, wire string
		sourceErr  error
	}{
		{name: "ordinary unquoted records", wire: "a,b\n1,2\n"},
		{name: "header only", wire: "a,b\n"},
		{name: "quoted header starts native reader", wire: "\"a\",b\n1,2\n"},
		{name: "quoted row follows unquoted header", wire: "a,b\n\"1\",2\n"},
		{name: "quoted comma remains one field", wire: "a,b\n\"1,2\",3\n"},
		{name: "escaped quote remains exact data", wire: "a,b\n\"1\"\"2\",3\n"},
		{name: "quoted LF remains data", wire: "a,b\n\"1\n2\",3\n"},
		{name: "CRLF is normalized outside quotes", wire: "a,b\r\n1,2\r\n"},
		{name: "quoted CRLF uses native normalization", wire: "a,b\n\"1\r\n2\",3\n"},
		{name: "trailing empty column is retained", wire: "a,b,\n1,2,\n"},
		{name: "final row without newline", wire: "a,b\n1,2"},
		{name: "empty file yields no records"},
		{name: "blank lines yield no records", wire: "\n\r\n\n"},
		{name: "bare CR at EOF is blank", wire: "\r"},
		{name: "blank lines before quoted header preserve coordinates", wire: "\n\r\n\"a\",b\n1,2\n"},
		{name: "blank line before malformed quote preserves coordinates", wire: "a,b\n1,2\n\n\"broken\n"},
		{name: "bare quote in header", wire: "a\"b,c\n"},
		{name: "unterminated quoted header", wire: "\"a,b\n"},
		{name: "bare quote after an ordinary row", wire: "a,b\n1,2\n3,4\"5\n"},
		{name: "junk after closing quote", wire: "a,b\n\"1\"x,2\n"},
		{name: "quoted transition followed by bare quote", wire: "a,b\n\"1\",2\n3\"4,5\n"},
		{name: "missing quote after an escaped quote", wire: "a,b\n\"1\"\"\n"},
		{name: "empty field does not hide a missing column", wire: "a,b,c\n,\n"},
		{name: "space before a quote is not trimmed", wire: "a,b\n \"1\",2\n"},
		{name: "space after a quote is not trimmed", wire: "a,b\n\"1\" ,2\n"},
		{name: "native source failure remains a failure", wire: "a,b\n1,2\n", sourceErr: io.ErrClosedPipe},
		{name: "EOF joined with a source failure is not clean EOF", wire: "a,b\n1,2\n", sourceErr: errors.Join(io.EOF, io.ErrClosedPipe)},
	}
	for _, extent := range []int{4095, 4096, 4097, 16385} {
		cases = append(cases, struct {
			name, wire string
			sourceErr  error
		}{name: fmt.Sprintf("unquoted record bytes %d", extent), wire: "a\n" + strings.Repeat("x", extent) + "\n"})
		cases = append(cases, struct {
			name, wire string
			sourceErr  error
		}{name: fmt.Sprintf("CRLF record bytes %d", extent), wire: "a\r\n" + strings.Repeat("x", extent-1) + "\r\n"})
		cases = append(cases, struct {
			name, wire string
			sourceErr  error
		}{name: fmt.Sprintf("native quote error byte %d", extent), wire: "a\n" + strings.Repeat("x", extent) + "\"\n"})
		cases = append(cases, struct {
			name, wire string
			sourceErr  error
		}{name: fmt.Sprintf("UTF8 field starts at byte %d", extent), wire: "a\n" + strings.Repeat("x", extent-1) + "🌲\n"})
	}
	for _, columns := range []int{1, 2, 3, 4097} {
		cases = append(cases, struct {
			name, wire string
			sourceErr  error
		}{name: fmt.Sprintf("two declared columns with %d actual fields", columns), wire: "a,b\n" + strings.TrimSuffix(strings.Repeat("x,", columns), ",") + "\n"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { t.Parallel(); compareUsageCSVFieldsWithNative(t, []byte(tc.wire), tc.sourceErr) })
	}
}

func compareUsageCSVFieldsWithNative(t *testing.T, data []byte, sourceErr error) {
	t.Helper()
	source := func() io.Reader {
		if sourceErr != nil {
			return io.MultiReader(bytes.NewReader(data), usageErrorReader{sourceErr})
		}
		return bytes.NewReader(data)
	}
	reference := csv.NewReader(source())
	var gotErr error
	for fields, err := range usageCSVFields(t.Context(), source()) {
		if err != nil {
			gotErr = err
			break
		}
		want, wantErr := reference.Read()
		if wantErr != nil {
			t.Fatalf("iterator emitted beside native refusal=%v", wantErr)
		}
		got := slices.Collect(fields)
		if !slices.Equal(got, want) {
			t.Fatalf("fields=%q, want native exact %q", got, want)
		}
	}
	_, wantErr := reference.Read()
	if wantErr == io.EOF {
		if gotErr != nil {
			t.Fatalf("iterator error=%v, want clean native EOF", gotErr)
		}
		return
	}
	if native, ok := errors.AsType[*csv.ParseError](wantErr); ok {
		got, found := errors.AsType[*csv.ParseError](gotErr)
		if !found || got.StartLine != native.StartLine || got.Line != native.Line || got.Column != native.Column || !errors.Is(got, native.Err) {
			t.Fatalf("CSV refusal=%+v, want exact native %+v", gotErr, native)
		}
		return
	}
	if wantErr == nil || !errors.Is(gotErr, wantErr) {
		t.Fatalf("iterator error=%v, want native refusal %v", gotErr, wantErr)
	}
}

func FuzzGCSUsageCSVFieldsNativeParity(f *testing.F) {
	seed := usageRecordFixture(f)
	names := usageColumnNames()
	values := usageRecordValues(f, seed)
	f.Add(usageCSVFixture(f, names[:], values))
	quoted := slices.Clone(values)
	quoted[usageUserAgent] = "quoted,\nuser agent"
	f.Add(usageCSVFixture(f, names[:], quoted))
	f.Add([]byte("a,b\n1,2\n\"broken\n"))
	f.Add([]byte("\n\n\"a\"x\n"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		compareUsageCSVFieldsWithNative(t, data[:min(len(data), 64<<10)], nil)
	})
}
