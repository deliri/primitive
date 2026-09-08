package filestore_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Windows measures a real exclusive read probe and close. Other hosts measure
// typed unsupported-platform refusal only; these are different workloads.
func BenchmarkSharingPlatformObservation(b *testing.B) {
	directory := b.TempDir()
	name := filepath.Join(directory, "entry")
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		b.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(name)
	if err != nil {
		b.Fatal(err)
	}
	before, err := os.Stat(name)
	if err != nil {
		b.Fatal(err)
	}
	want, wantErr := nativeSharingProbe(name)
	if wantErr != nil && !errors.Is(wantErr, core.ErrFilestoreContract) {
		b.Fatalf("native seed = (%v,%v), want Available or unsupported", want, wantErr)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.ObserveSharing(b.Context(), path)
		if got != want || !errors.Is(err, wantErr) || errors.Is(err, core.ErrFilestoreSource) {
			b.Fatalf("sharing = (%v,%v), want (%v,%v)", got, err, want, wantErr)
		}
	}
	got, gotErr := nativeSharingProbe(name)
	if got != want || !errors.Is(gotErr, wantErr) {
		b.Fatalf("native custody = (%v,%v), want unchanged", got, gotErr)
	}
	after, err := os.Stat(name)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
		b.Fatalf("entry = (%v,%v), want original metadata", after, err)
	}
	data, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(data, payload) {
		b.Fatalf("entry bytes = (%v,%v), want %v", data, err, payload)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != "entry" {
		b.Fatalf("namespace = (%v,%v), want original entry", entries, err)
	}
}
