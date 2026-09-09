package id_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/id"
)

func BenchmarkParseULID(b *testing.B) {
	value, err := id.NewULID(testRequest(b, 1, testEntropy()))
	if err != nil {
		b.Fatalf("NewULID() error = %v, want nil", err)
	}
	text := value.String()
	if text == "" {
		b.Fatalf("ULID.String()=%q, want canonical spelling", text)
	}
	var wantErr error
	b.ReportAllocs()
	var last id.ULID
	for b.Loop() {
		parsed, err := id.ParseULID(text)
		if !errors.Is(err, wantErr) {
			b.Fatalf("id.ParseULID() error = %v, want %v", err, wantErr)
		}
		last = parsed
	}
	if last != value {
		b.Fatalf("id.ParseULID() = %v, want %v", last, value)
	}
}

func BenchmarkParseUUIDv7(b *testing.B) {
	value, err := id.NewUUIDv7(testRequest(b, 1, testEntropy()))
	if err != nil {
		b.Fatalf("NewUUIDv7() error = %v, want nil", err)
	}
	text := value.String()
	if text == "" {
		b.Fatalf("UUIDv7.String()=%q, want canonical spelling", text)
	}
	var wantErr error
	b.ReportAllocs()
	var last id.UUIDv7
	for b.Loop() {
		parsed, err := id.ParseUUIDv7(text)
		if !errors.Is(err, wantErr) {
			b.Fatalf("id.ParseUUIDv7() error = %v, want %v", err, wantErr)
		}
		last = parsed
	}
	if last != value {
		b.Fatalf("id.ParseUUIDv7() = %v, want %v", last, value)
	}
}

func BenchmarkULIDAppendTextReusedBuffer(b *testing.B) {
	value, err := id.NewULID(testRequest(b, 1, testEntropy()))
	if err != nil {
		b.Fatalf("NewULID() error = %v, want nil", err)
	}
	destination := make([]byte, 0, 64)
	var wantErr error
	b.ReportAllocs()
	var last []byte
	for b.Loop() {
		appended, err := value.AppendText(destination[:0])
		if !errors.Is(err, wantErr) || len(appended) == 0 {
			b.Fatalf("ULID.AppendText() = (%d bytes, %v), want spelling and %v", len(appended), err, wantErr)
		}
		last = appended
	}
	if string(last) != value.String() {
		b.Fatalf("ULID.AppendText() = %q, want %q", last, value.String())
	}
}

func BenchmarkNewUUIDv7(b *testing.B) {
	b.ReportAllocs()
	request := testRequest(b, 1, testEntropy())
	want, err := id.NewUUIDv7(request)
	if err != nil {
		b.Fatal(err)
	}
	var got id.UUIDv7
	for b.Loop() {
		got, err = id.NewUUIDv7(request)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("constructed identity = %v, want %v", got, want)
	}
}

func BenchmarkUUIDv7JSON(b *testing.B) {
	b.ReportAllocs()
	value, err := id.NewUUIDv7(testRequest(b, 1, testEntropy()))
	if err != nil {
		b.Fatal(err)
	}
	wire, err := value.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	b.Run("encode", func(b *testing.B) {
		b.ReportAllocs()
		var got []byte
		var err error
		for b.Loop() {
			got, err = value.MarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
		}
		if !bytes.Equal(got, wire) {
			b.Fatalf("JSON = %q, want %q", got, wire)
		}
	})
	b.Run("decode", func(b *testing.B) {
		b.ReportAllocs()
		var got id.UUIDv7
		for b.Loop() {
			if err := got.UnmarshalJSON(wire); err != nil {
				b.Fatal(err)
			}
		}
		if got != value {
			b.Fatalf("decoded identity = %v, want %v", got, value)
		}
	})
}

func BenchmarkNewULID(b *testing.B) {
	b.ReportAllocs()
	request := testRequest(b, 1, testEntropy())
	want, err := id.NewULID(request)
	if err != nil {
		b.Fatal(err)
	}
	var got id.ULID
	for b.Loop() {
		got, err = id.NewULID(request)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got != want {
		b.Fatalf("constructed identity = %v, want %v", got, want)
	}
}

func BenchmarkULIDJSON(b *testing.B) {
	b.ReportAllocs()
	value, err := id.NewULID(testRequest(b, 1, testEntropy()))
	if err != nil {
		b.Fatal(err)
	}
	wire, err := value.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	b.Run("encode", func(b *testing.B) {
		b.ReportAllocs()
		var got []byte
		var err error
		for b.Loop() {
			got, err = value.MarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
		}
		if !bytes.Equal(got, wire) {
			b.Fatalf("JSON = %q, want %q", got, wire)
		}
	})
	b.Run("decode", func(b *testing.B) {
		b.ReportAllocs()
		var got id.ULID
		for b.Loop() {
			if err := got.UnmarshalJSON(wire); err != nil {
				b.Fatal(err)
			}
		}
		if got != value {
			b.Fatalf("decoded identity = %v, want %v", got, value)
		}
	})
}

func BenchmarkUUIDv7AppendTextReusedBuffer(b *testing.B) {
	b.ReportAllocs()
	value, err := id.NewUUIDv7(testRequest(b, 1, testEntropy()))
	if err != nil {
		b.Fatal(err)
	}
	want := value.String()
	destination := make([]byte, 0, len(want))
	var got []byte
	for b.Loop() {
		got, err = value.AppendText(destination[:0])
		if err != nil {
			b.Fatal(err)
		}
	}
	if string(got) != want {
		b.Fatalf("appended text = %q, want %q", got, want)
	}
}
