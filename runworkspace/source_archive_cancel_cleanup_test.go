package runworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type sourceCancelReader struct {
	source *bytes.Reader
	cancel context.CancelFunc
	after  int
	read   int
}

func (r *sourceCancelReader) Len() int { return r.source.Len() }

func (r *sourceCancelReader) Read(data []byte) (int, error) {
	count, err := r.source.Read(data)
	r.read += count
	if r.read >= r.after {
		r.cancel()
	}
	return count, err
}

func TestCanceledArchiveExtractionRemovesNativeCheckout(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			manager, unit := sourceWorkspaceFixture(t)
			defer closeSourceManager(t, manager)
			defer cleanupSourceUnit(t, manager, unit)
			archive, document, grant, trusted := sourceArchiveFixture(t, unit, bytes.Repeat([]byte{byte(index)}, 8192))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			after := [...]int{1, 512, 513, 1024, 1536, 2048, 4096, len(archive) - 1}[index%8]
			source := &sourceCancelReader{source: bytes.NewReader(archive), cancel: cancel, after: after}
			got, err := manager.AcquireSourceArchive(ctx, SourceArchiveAcquisitionRequest{Unit: unit, Grant: grant, Document: document, Trusted: trusted, ObservedAt: temporal.InstantFromNanoseconds(10), Source: source})
			if !errors.Is(err, context.Canceled) || got != (VerifiedSource{}) || source.read < after {
				t.Fatalf("cancelled extraction=%+v/%v, read=%d after=%d", got, err, source.read, after)
			}
			checkout, pathErr := joinLiteral(unit.Root, "checkout")
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			_, statErr := manager.root.Lstat(checkout.String())
			if !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("cancelled extraction left native checkout: %v", statErr)
			}
			if err := unit.Validate(); err != nil {
				t.Fatalf("cleanup destroyed enclosing unit: %v", err)
			}
			_, statErr = manager.root.Stat(unit.Root.String())
			if statErr != nil {
				t.Fatalf("cleanup lost native unit: %v", statErr)
			}
			if err != nil && errors.Is(err, core.ErrFilestoreCleanup) {
				t.Fatalf("cancellation leaked into cleanup capability: %v", err)
			}
		})
	}
}
