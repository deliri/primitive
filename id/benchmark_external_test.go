package id_test

import (
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
		b.Fatal("ULID.String() is empty, want canonical spelling")
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
		b.Fatal("UUIDv7.String() is empty, want canonical spelling")
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
