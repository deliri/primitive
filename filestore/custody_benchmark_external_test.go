package filestore_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func BenchmarkExistingFileCustody(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name  string
		touch bool
	}{
		{name: "ConfirmDurable"},
		{name: "Touch", touch: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			directory := b.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := root.Close(); err != nil {
					b.Error(err)
				}
			}()
			path, err := core.ParseRelativePath("existing")
			if err != nil {
				b.Fatal(err)
			}
			payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
			if err := os.WriteFile(directory+"/existing", payload, 0o600); err != nil {
				b.Fatal(err)
			}
			before, err := root.Stat(path.String())
			if err != nil {
				b.Fatal(err)
			}
			instant := temporal.InstantFromNanoseconds(1_000_000_000)
			touch := filestore.TouchRequest{Location: filestore.Location{Root: root, Path: path}, ModifiedAt: instant}
			durable := filestore.DurabilityRequest{Location: touch.Location}
			b.ReportAllocs()
			if tc.touch {
				for b.Loop() {
					if err := filestore.Touch(b.Context(), touch); err != nil {
						b.Fatal(err)
					}
				}
			} else {
				for b.Loop() {
					if err := filestore.ConfirmDurable(b.Context(), durable); err != nil {
						b.Fatal(err)
					}
				}
			}
			after, err := root.Stat(path.String())
			if err != nil {
				b.Fatal(err)
			}
			wantStamp := before.ModTime().UnixNano()
			if tc.touch {
				wantStamp = 1_000_000_000
			}
			if !os.SameFile(before, after) || after.ModTime().UnixNano() != wantStamp || after.Mode() != before.Mode() {
				b.Fatalf("file identity/mode/stamp changed unexpectedly: %v", after)
			}
			got, err := os.ReadFile(directory + "/existing")
			if err != nil || !bytes.Equal(got, payload) {
				b.Fatalf("retained bytes = (%v,%v), want exact %v", got, err, payload)
			}
		})
	}
}
