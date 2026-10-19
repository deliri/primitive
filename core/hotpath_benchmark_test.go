package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"path/filepath"
	"testing"
)

// benchmarkHotRecord is a representative strict-JSON boundary document: typed
// Core identities, an array, and one nested object, as products decode on
// every request.
type benchmarkHotRecord struct {
	Name   string        `json:"name"`
	Root   AbsolutePath  `json:"root"`
	Entry  PathComponent `json:"entry"`
	Size   ByteCount     `json:"size"`
	Digest SHA256Digest  `json:"digest"`
	Tags   []string      `json:"tags"`
	Nested struct {
		Kind  string `json:"kind"`
		Count uint64 `json:"count"`
	} `json:"nested"`
}

func (r benchmarkHotRecord) Validate() error {
	if err := r.Root.Validate(); err != nil {
		return err
	}
	if err := r.Entry.Validate(); err != nil {
		return err
	}
	if err := r.Size.Validate(); err != nil {
		return err
	}
	return r.Digest.Validate()
}

func benchmarkHotRecordWire(b *testing.B) ([]byte, benchmarkHotRecord) {
	b.Helper()
	root, err := ParseAbsolutePath(filepath.Join(string(filepath.Separator), "srv", "primitive", "objects"))
	if err != nil {
		b.Fatal(err)
	}
	entry, err := ParsePathComponent("entry-0001.json")
	if err != nil {
		b.Fatal(err)
	}
	size, err := NewByteCount(1 << 20)
	if err != nil {
		b.Fatal(err)
	}
	record := benchmarkHotRecord{
		Name:   "release-candidate",
		Root:   root,
		Entry:  entry,
		Size:   size,
		Digest: SHA256Of([]byte("benchmark hot record")),
		Tags:   []string{"alpha", "beta", "gamma", "delta"},
	}
	record.Nested.Kind = "artifact"
	record.Nested.Count = 42
	wire, err := json.Marshal(record)
	if err != nil {
		b.Fatal(err)
	}
	if len(wire) < 200 || !bytes.Contains(wire, []byte(`"nested":{"kind":"artifact"`)) {
		b.Fatalf("record wire = %q, want a non-vacuous nested document", wire)
	}
	return wire, record
}

func BenchmarkDecodeStrictJSONBytesRecord(b *testing.B) {
	wire, want := benchmarkHotRecordWire(b)
	limits := DefaultStrictJSONLimits()
	b.SetBytes(int64(len(wire)))
	b.ReportAllocs()
	var got benchmarkHotRecord
	var err error
	for b.Loop() {
		got, err = DecodeStrictJSONBytes[benchmarkHotRecord](wire, limits)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.Root != want.Root || got.Digest != want.Digest || len(got.Tags) != len(want.Tags) || got.Nested != want.Nested {
		b.Fatalf("decoded = %+v, want %+v", got, want)
	}
}

func BenchmarkStrictJSONValidateDocumentRecord(b *testing.B) {
	wire, _ := benchmarkHotRecordWire(b)
	limits := DefaultStrictJSONLimits()
	b.SetBytes(int64(len(wire)))
	b.ReportAllocs()
	scanned := 0
	for b.Loop() {
		if err := limits.ValidateDocument(wire); err != nil {
			b.Fatal(err)
		}
		scanned++
	}
	if scanned == 0 {
		b.Fatal("ValidateDocument never ran")
	}
	// The same scanner must still refuse a folded duplicate after timing.
	if err := limits.ValidateDocument([]byte(`{"name":1,"Name":2}`)); err == nil {
		b.Fatal("ValidateDocument admitted a folded duplicate after the timed loop")
	}
}

var benchmarkJSONBytesSink []byte

func BenchmarkMarshalCanonicalJSONString(b *testing.B) {
	value := "/srv/primitive/objects/entry-0001.json"
	want := []byte(`"` + value + `"`)
	b.ReportAllocs()
	for b.Loop() {
		got, err := MarshalCanonicalJSONString(value)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkJSONBytesSink = got
	}
	if !bytes.Equal(benchmarkJSONBytesSink, want) {
		b.Fatalf("encoded = %q, want %q", benchmarkJSONBytesSink, want)
	}
}

func BenchmarkDecodeJSONStringToken(b *testing.B) {
	want := "/srv/primitive/objects/entry-0001.json"
	wire := []byte(`"` + want + `"`)
	b.ReportAllocs()
	var got string
	var err error
	for b.Loop() {
		got, err = DecodeJSONStringToken(wire)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("decoded = %q, want %q", got, want)
	}
}

func BenchmarkByteCountUnmarshalJSON(b *testing.B) {
	wire := []byte("1048576")
	b.ReportAllocs()
	var got ByteCount
	for b.Loop() {
		if err := got.UnmarshalJSON(wire); err != nil {
			b.Fatal(err)
		}
	}
	if got.value != 1<<20 {
		b.Fatalf("decoded = %d, want %d", got.value, 1<<20)
	}
}

func BenchmarkSHA256DigestMarshalJSON(b *testing.B) {
	digest := NewSHA256Digest(sha256.Sum256([]byte("benchmark digest")))
	sum := sha256.Sum256([]byte("benchmark digest"))
	want, err := json.Marshal(hex.EncodeToString(sum[:]))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := digest.MarshalJSON()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkJSONBytesSink = got
	}
	if !bytes.Equal(benchmarkJSONBytesSink, want) {
		b.Fatalf("encoded = %q, want %q", benchmarkJSONBytesSink, want)
	}
}

func BenchmarkSHA256DigestUnmarshalJSON(b *testing.B) {
	want := NewSHA256Digest(sha256.Sum256([]byte("benchmark digest")))
	wire, err := want.MarshalJSON()
	if err != nil || len(wire) != 2*SHA256DigestBytes+2 {
		b.Fatalf("fixture = %q, %v; want a quoted digest", wire, err)
	}
	b.ReportAllocs()
	var got SHA256Digest
	for b.Loop() {
		if err := got.UnmarshalJSON(wire); err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("decoded = %v, want %v", got, want)
	}
}

func BenchmarkAbsolutePathResolve(b *testing.B) {
	base, err := ParseAbsolutePath(filepath.Join(string(filepath.Separator), "srv", "primitive"))
	if err != nil {
		b.Fatal(err)
	}
	want := filepath.Join(base.String(), "objects", "ab", "entry-0001.json")
	b.ReportAllocs()
	var got AbsolutePath
	for b.Loop() {
		got, err = base.Resolve("objects", "ab", "entry-0001.json")
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.String() != want {
		b.Fatalf("resolved = %q, want %q", got, want)
	}
}

func BenchmarkAbsolutePathJoinRelative(b *testing.B) {
	base, err := ParseAbsolutePath(filepath.Join(string(filepath.Separator), "srv", "primitive"))
	if err != nil {
		b.Fatal(err)
	}
	relative, err := ParseRelativePath(filepath.Join("objects", "ab", "entry-0001.json"))
	if err != nil {
		b.Fatal(err)
	}
	want := filepath.Join(base.String(), relative.String())
	b.ReportAllocs()
	var got AbsolutePath
	for b.Loop() {
		got, err = base.JoinRelative(relative)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.String() != want {
		b.Fatalf("joined = %q, want %q", got, want)
	}
}
