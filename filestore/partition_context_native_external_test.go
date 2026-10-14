package filestore_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestContentPartitionUsesOperationContextAfterConstructionContextEnds(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "partition-source")
	want := []byte("native-context-scope")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	creation, cancel := context.WithCancel(t.Context())
	source, err := filestore.NewContentPartitionSource(creation, filestore.ContentPartitionSourceRequest{Source: file})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	maximum, err := core.NewByteCount(3)
	if err != nil {
		t.Fatal(err)
	}
	var destination bytes.Buffer
	for {
		got, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: &destination, Maximum: maximum})
		if err != nil {
			t.Fatalf("current-context partition = %+v/%v, want native bytes after construction context ended", got, err)
		}
		if got.Kind == filestore.ContentPartitionLast {
			break
		}
		if got.Kind != filestore.ContentPartitionMore {
			t.Fatalf("partition = %+v, want nonterminal nonempty window", got)
		}
	}
	if !bytes.Equal(destination.Bytes(), want) {
		t.Fatalf("operation-context bytes = %q, want %q", destination.Bytes(), want)
	}
	if _, err := file.Stat(); err != nil {
		t.Fatalf("partition closed caller-owned file = %v", err)
	}
}
