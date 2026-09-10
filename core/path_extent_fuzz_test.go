package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzParsePathComponentSemanticClosure(f *testing.F) {
	for _, text := range []string{"entry", strings.Repeat("é", 128), strings.Repeat("a", 4097)} {
		seed, err := ParsePathComponent(text)
		if err != nil {
			f.Fatal(err)
		}
		wire, err := seed.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		var decoded PathComponent
		if err := decoded.UnmarshalJSON(wire); err != nil || decoded != seed {
			f.Fatalf("seed roundtrip = %v/%v, want %v", decoded, err, seed)
		}
		f.Add(decoded.String())
	}
	for _, text := range []string{"", ".", "..", "a\x00b", "\xff", "a" + string(filepath.Separator) + "b"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		separator := false
		for index := range len(text) {
			separator = separator || os.IsPathSeparator(text[index])
		}
		wantOK := text != "" && text != "." && text != ".." && utf8.ValidString(text) &&
			!strings.ContainsRune(text, 0) && !separator && filepath.Base(text) == text
		got, err := ParsePathComponent(text)
		if (err == nil) != wantOK {
			t.Fatalf("component %d bytes admission = %v, want %t", len(text), err, wantOK)
		}
		if !wantOK {
			if !errors.Is(err, ErrPrimitiveContract) || got != (PathComponent{}) {
				t.Fatalf("refusal = %v/%v, want zero and contract", got, err)
			}
			return
		}
		if got.Validate() != nil || got.String() != text {
			t.Fatalf("component = %v, want exact admitted bytes", got)
		}
		wire, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var restored PathComponent
		if err := restored.UnmarshalJSON(wire); err != nil || restored != got {
			t.Fatalf("roundtrip = %v/%v, want %v", restored, err, got)
		}
		second, err := restored.MarshalJSON()
		if err != nil || !bytes.Equal(wire, second) {
			t.Fatalf("second encoding = %d bytes/%v, want identical %d bytes", len(second), err, len(wire))
		}
	})
}

func FuzzParseAbsolutePathSemanticClosure(f *testing.F) {
	root := filepath.VolumeName(f.TempDir()) + string(filepath.Separator)
	for _, text := range []string{root, root + "entry", root + strings.Repeat("é", 128), root + strings.Repeat("a", 4097)} {
		seed, err := ParseAbsolutePath(text)
		if err != nil {
			f.Fatal(err)
		}
		wire, err := seed.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		var decoded AbsolutePath
		if err := decoded.UnmarshalJSON(wire); err != nil || decoded != seed {
			f.Fatalf("seed roundtrip = %v/%v, want %v", decoded, err, seed)
		}
		f.Add(decoded.String())
	}
	for _, text := range []string{"", ".", root + "a\x00b", root + "\xff", root + "a" + string(filepath.Separator) + ".."} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		wantOK := text != "" && utf8.ValidString(text) && !strings.ContainsRune(text, 0) &&
			filepath.IsAbs(text) && filepath.Clean(text) == text
		got, err := ParseAbsolutePath(text)
		if (err == nil) != wantOK {
			t.Fatalf("absolute %d bytes admission = %v, want %t", len(text), err, wantOK)
		}
		if !wantOK {
			if !errors.Is(err, ErrPrimitiveContract) || got != (AbsolutePath{}) {
				t.Fatalf("refusal = %v/%v, want zero and contract", got, err)
			}
			return
		}
		if got.Validate() != nil || got.String() != text {
			t.Fatalf("absolute = %v, want exact admitted bytes", got)
		}
		wire, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var restored AbsolutePath
		if err := restored.UnmarshalJSON(wire); err != nil || restored != got {
			t.Fatalf("roundtrip = %v/%v, want %v", restored, err, got)
		}
		second, err := restored.MarshalJSON()
		if err != nil || !bytes.Equal(wire, second) {
			t.Fatalf("second encoding = %d bytes/%v, want identical %d bytes", len(second), err, len(wire))
		}
	})
}

func TestPathExtentJSONReceiverLayerTriad(t *testing.T) {
	t.Parallel()
	// This is a finite whole-value fixture. Path strings and their JSON values
	// are caller-owned allocations; no claim of streaming a filename is made.
	text := strings.Repeat("x", 1<<20)
	cases := []struct {
		name, text string
		wantErr    error
	}{
		{name: "one MiB component preserves every byte", text: text},
		{name: "large NUL mutation cannot acquire path identity", text: text + "\x00", wantErr: ErrPrimitiveContract},
		{name: "large separator mutation cannot become one component", text: text + string(filepath.Separator), wantErr: ErrPrimitiveContract},
		{name: "empty string preserves populated receiver", wantErr: ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			original, err := ParsePathComponent("retained")
			if err != nil {
				t.Fatal(err)
			}
			wire, err := MarshalCanonicalJSONString(tc.text)
			if err != nil {
				t.Fatal(err)
			}
			got := original
			err = got.UnmarshalJSON(wire)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decode = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, ErrJSONContract) || got != original {
					t.Fatalf("refusal = %v/%v, want preserved receiver and JSON identity", got, err)
				}
				return
			}
			if got.Validate() != nil || got.String() != tc.text {
				t.Fatalf("decoded = %d bytes, want exact %d", len(got.String()), len(tc.text))
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, wire) {
				t.Fatalf("encoding = %d bytes/%v, want exact %d", len(encoded), err, len(wire))
			}
		})
	}
}

func BenchmarkParsePathComponent64KiB(b *testing.B) {
	text := strings.Repeat("a", 64<<10)
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	var got PathComponent
	for b.Loop() {
		value, err := ParsePathComponent(text)
		if err != nil {
			b.Fatal(err)
		}
		got = value
	}
	if got.String() != text || got.Validate() != nil {
		b.Fatalf("component = %d bytes, want exact %d", len(got.String()), len(text))
	}
}
