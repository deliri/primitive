package jsonio_test

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/lineio"
)

// benchLeaf and benchRecord declare members out of UTF-16 order so canonical
// projection performs its real reordering work rather than a sorted fast path.
type benchLeaf struct {
	Zone  string `json:"zone"`
	Count int64  `json:"count"`
}

type benchRecord struct {
	Zeta   string    `json:"zeta"`
	Alpha  uint64    `json:"alpha"`
	Mid    *string   `json:"mid,omitempty"`
	Tags   []string  `json:"tags"`
	Nested benchLeaf `json:"nested"`
}

func (r benchRecord) Validate() error {
	if r.Zeta == "" {
		return core.ErrSourceObservationContract
	}
	return nil
}

var (
	benchBytesSink  []byte
	benchCountSink  int
	benchRecordSink benchRecord
)

func benchFixtureRecord() benchRecord {
	return benchRecord{
		Zeta:   "record-zeta-value",
		Alpha:  18446744073709551615,
		Tags:   []string{"north", "south", "east", "west"},
		Nested: benchLeaf{Zone: "ca-central", Count: -42},
	}
}

func benchStrictPolicy() jsonio.OwnedCodecPolicy {
	return jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly}
}

// benchRecordStream returns count newline-delimited scalar objects.
func benchRecordStream(count int) []byte {
	var stream bytes.Buffer
	for i := range count {
		stream.WriteString(`{"Name":"row-`)
		stream.WriteString(strconv.Itoa(i))
		stream.WriteString(`","Count":`)
		stream.WriteString(strconv.Itoa(i * 7919))
		stream.WriteString("}\n")
	}
	return stream.Bytes()
}

// benchTokenDocument returns one array of count small objects. Each object
// contributes eight tokens; the outer array contributes two.
func benchTokenDocument(count int) []byte {
	var document bytes.Buffer
	document.WriteByte('[')
	for i := range count {
		if i > 0 {
			document.WriteByte(',')
		}
		document.WriteString(`{"name":"item-`)
		document.WriteString(strconv.Itoa(i))
		document.WriteString(`","size":`)
		document.WriteString(strconv.Itoa(i * 31))
		document.WriteString(`,"ok":true}`)
	}
	document.WriteByte(']')
	return document.Bytes()
}

const benchStreamRecords = 64

func BenchmarkJSONTokenEncoderNumberArray(b *testing.B) {
	const number = "-12345.678e-9"
	w := &tokenCountingWriter{}
	e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: w})
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	if err := e.Encode(ctx, jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := e.Encode(ctx, jsonio.Token{Kind: jsonio.TokenNumber, Text: number}); err != nil {
			b.Fatal(err)
		}
	}
	if err := e.Encode(ctx, jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
		b.Fatal(err)
	}
	benchCountSink = w.bytes
	if want := (len(number)+1)*b.N + 2; w.bytes != want {
		b.Fatalf("bytes=%d want=%d", w.bytes, want)
	}
}

func BenchmarkJSONTokensArray(b *testing.B) {
	const items = 128
	document := benchTokenDocument(items)
	want := items*8 + 2
	ctx := context.Background()
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	for b.Loop() {
		count := 0
		for _, err := range jsonio.Tokens(ctx, jsonio.TokenRequest{Source: bytes.NewReader(document), NestingDepthMaximum: core.JSONNestingDepthMaximum}) {
			if err != nil {
				b.Fatal(err)
			}
			count++
		}
		benchCountSink = count
	}
	if benchCountSink != want {
		b.Fatalf("tokens=%d want=%d", benchCountSink, want)
	}
}

func BenchmarkJSONTokenDecoderReadValue(b *testing.B) {
	document := []byte(`{"zeta":"z","nested":{"zone":"a","list":[1,2,3,{"deep":[true,false,null]}]}}`)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		decoder, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: bytes.NewReader(document), NestingDepthMaximum: core.JSONNestingDepthMaximum})
		if err != nil {
			b.Fatal(err)
		}
		value, err := decoder.ReadValue(ctx)
		if err != nil {
			b.Fatal(err)
		}
		benchBytesSink = value
	}
	if !bytes.Equal(benchBytesSink, document) {
		b.Fatalf("ReadValue=%s want %s", benchBytesSink, document)
	}
}

func BenchmarkJSONMarshalValidated(b *testing.B) {
	record := benchFixtureRecord()
	policy := benchStrictPolicy()
	b.ReportAllocs()
	for b.Loop() {
		data, err := jsonio.MarshalValidated(record, policy)
		if err != nil {
			b.Fatal(err)
		}
		benchBytesSink = data
	}
	if !bytes.Contains(benchBytesSink, []byte(`"alpha":18446744073709551615`)) {
		b.Fatalf("MarshalValidated=%s, want complete record", benchBytesSink)
	}
}

func BenchmarkJSONDecodeOwned(b *testing.B) {
	data, err := jsonio.MarshalValidated(benchFixtureRecord(), benchStrictPolicy())
	if err != nil {
		b.Fatal(err)
	}
	policy := benchStrictPolicy()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		record, err := jsonio.DecodeOwned[benchRecord](data, policy)
		if err != nil {
			b.Fatal(err)
		}
		benchRecordSink = record
	}
	if benchRecordSink.Nested.Count != -42 || len(benchRecordSink.Tags) != 4 {
		b.Fatalf("DecodeOwned=%+v, want fixture", benchRecordSink)
	}
}

func BenchmarkJSONWriteValidated(b *testing.B) {
	record := benchFixtureRecord()
	policy := benchStrictPolicy()
	w := &tokenCountingWriter{}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		w.bytes = 0
		if err := jsonio.WriteValidated(ctx, w, record, policy); err != nil {
			b.Fatal(err)
		}
	}
	benchCountSink = w.bytes
	if w.bytes < 100 {
		b.Fatalf("WriteValidated bytes=%d, want complete record", w.bytes)
	}
}

func BenchmarkJSONWriteCanonicalProjection(b *testing.B) {
	record := benchFixtureRecord()
	policy := benchStrictPolicy()
	var out bytes.Buffer
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		out.Reset()
		if err := jsonio.WriteCanonicalProjectionContext(ctx, &out, record, policy); err != nil {
			b.Fatal(err)
		}
	}
	benchBytesSink = out.Bytes()
	if !bytes.HasPrefix(benchBytesSink, []byte(`{"alpha":18446744073709551615,"nested":{"count":-42,"zone":"ca-central"}`)) {
		b.Fatalf("WriteCanonicalProjection=%s, want canonical member order", benchBytesSink)
	}
}

func BenchmarkJSONCanonicalize(b *testing.B) {
	data := []byte(`{"zeta":"z","alpha":18446744073709551615,"nested":{"zone":"a","count":-42,"list":[1.50,2e3,"é"]},"tags":["n","s"]}`)
	policy := jsonio.CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		canonical, err := jsonio.Canonicalize(data, policy)
		if err != nil {
			b.Fatal(err)
		}
		benchBytesSink = canonical
	}
	if !bytes.HasPrefix(benchBytesSink, []byte(`{"alpha":18446744073709551615,"nested":{"count":-42,"list":[1.5,2000,"é"]`)) {
		b.Fatalf("Canonicalize=%s", benchBytesSink)
	}
}

func BenchmarkJSONDocumentsStream(b *testing.B) {
	stream := benchRecordStream(benchStreamRecords)
	request := jsonio.Request{Limits: core.DefaultStrictJSONLimits(), Members: jsonio.MembersDeclaredOnly}
	ctx := context.Background()
	b.SetBytes(int64(len(stream)))
	b.ReportAllocs()
	for b.Loop() {
		request.Source = bytes.NewReader(stream)
		count := 0
		for document, err := range jsonio.Documents[scalarObject](ctx, request) {
			if err != nil {
				b.Fatal(err)
			}
			count += len(document.Name)
		}
		benchCountSink = count
	}
	if benchCountSink < benchStreamRecords*5 {
		b.Fatalf("decoded name bytes=%d, want %d records", benchCountSink, benchStreamRecords)
	}
}

func BenchmarkJSONObjectsStream(b *testing.B) {
	stream := benchRecordStream(benchStreamRecords)
	ctx := context.Background()
	b.SetBytes(int64(len(stream)))
	b.ReportAllocs()
	for b.Loop() {
		count := 0
		for document, err := range jsonio.Objects[scalarObject](ctx, jsonio.ObjectSourceRequest{Source: bytes.NewReader(stream)}) {
			if err != nil {
				b.Fatal(err)
			}
			count += len(document.Name)
		}
		benchCountSink = count
	}
	if benchCountSink < benchStreamRecords*5 {
		b.Fatalf("decoded name bytes=%d, want %d records", benchCountSink, benchStreamRecords)
	}
}

func BenchmarkJSONObjectEncoderRecords(b *testing.B) {
	w := &tokenCountingWriter{}
	encoder, err := jsonio.NewObjectEncoder[scalarObject](jsonio.ObjectDestinationRequest{Destination: w})
	if err != nil {
		b.Fatal(err)
	}
	record := scalarObject{Name: "row", Count: 18446744073709551615}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := encoder.Encode(ctx, record); err != nil {
			b.Fatal(err)
		}
	}
	benchCountSink = w.bytes
	if want := len(`{"Name":"row","Count":18446744073709551615}`+"\n") * b.N; w.bytes != want {
		b.Fatalf("bytes=%d want=%d", w.bytes, want)
	}
}

func BenchmarkJSONValidateDocument(b *testing.B) {
	document := benchTokenDocument(128)
	ctx := context.Background()
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	for b.Loop() {
		if err := jsonio.ValidateDocument(ctx, bytes.NewReader(document)); err != nil {
			b.Fatal(err)
		}
		benchCountSink++
	}
	if benchCountSink == 0 {
		b.Fatal("ValidateDocument never ran")
	}
}

func BenchmarkJSONDecodeScalarText(b *testing.B) {
	data := []byte(`"` + strings.Repeat("café ", 8) + `"`)
	b.ReportAllocs()
	for b.Loop() {
		text, err := jsonio.DecodeText(data)
		if err != nil {
			b.Fatal(err)
		}
		benchCountSink = len(text)
	}
	if want := len(strings.Repeat("café ", 8)); benchCountSink != want {
		b.Fatalf("DecodeText length=%d want=%d", benchCountSink, want)
	}
}

func BenchmarkJSONDecodeProjectionRecords(b *testing.B) {
	const records = 64
	text := strings.Repeat("r", 1024)
	var stream bytes.Buffer
	for i := range records {
		stream.WriteString(`  {"text":"` + text + `","count":` + strconv.Itoa(i) + "}\t\r\n")
	}
	data := stream.Bytes()
	ctx := context.Background()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		reader := projectionRecordReader(b, bytes.NewReader(data))
		var offset lineio.SourceByteOffset
		count := 0
		for range records {
			got, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](ctx, jsonio.ProjectionRecordRequest{Reader: reader, Offset: offset})
			if err != nil || got.DecodeError != nil || got.Syntax != jsonio.RecordSyntaxComplete {
				b.Fatalf("record %d = (%+v, %v), want complete projection", count, got, err)
			}
			offset += lineio.SourceByteOffset(got.Record.Bytes.Uint64())
			count += int(got.Value.Count)
		}
		benchCountSink = count
	}
	if want := records * (records - 1) / 2; benchCountSink != want {
		b.Fatalf("count sum=%d want=%d", benchCountSink, want)
	}
}
