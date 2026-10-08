package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzRemovalPolicyConservesRefusedNativeBytes(f *testing.F) {
	for _, mode := range []uint8{0, uint8(filestore.RemovalDurabilityEphemeral), uint8(filestore.RemovalDurabilityDurable), 255} {
		f.Add(mode, []byte{0, 255, 31}, false)
	}
	f.Add(uint8(filestore.RemovalDurabilityEphemeral), []byte{}, true)
	f.Fuzz(func(t *testing.T, rawMode uint8, body []byte, canceled bool) {
		body = body[:min(len(body), 4096)] // Independent fixture oracle, not a production stream limit.
		directory := t.TempDir()
		absolute, err := core.ParseAbsolutePath(directory)
		if err != nil {
			t.Fatal(err)
		}
		path, err := core.ParseRelativePath("subject")
		if err != nil {
			t.Fatal(err)
		}
		durability := filestore.RemovalDurability(rawMode)
		valid := durability == filestore.RemovalDurabilityEphemeral || durability == filestore.RemovalDurabilityDurable
		if (durability.Validate() == nil) != valid {
			t.Fatalf("durability=%d has validation inconsistent with closed policy", rawMode)
		}
		scope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: absolute, Use: func(ctx context.Context, root *os.Root) error {
			if err := os.WriteFile(filepath.Join(directory, path.String()), body, 0o600); err != nil {
				return err
			}
			operation, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
			if err != nil {
				return err
			}
			defer cancel(nil)
			if canceled {
				cancel(nil)
			}
			got := filestore.Remove(operation, filestore.RemovalRequest{Location: filestore.Location{Root: root, Path: path}, Durability: durability})
			var want error
			if canceled {
				want = context.Canceled
			} else if !valid {
				want = core.ErrFilestoreContract
			}
			if !errors.Is(got, want) {
				t.Fatalf("removal policy=%d canceled=%v result=%v,want %v", rawMode, canceled, got, want)
			}
			observed, err := os.ReadFile(filepath.Join(directory, path.String()))
			if want == nil {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("accepted removal after-state=%v,want native absence", err)
				}
				return nil
			}
			if err != nil || !bytes.Equal(observed, body) {
				t.Fatalf("refused native bytes=(%v,%v),want %v", observed, err, body)
			}
			return nil
		}})
		if err := errors.Join(err, scope.Validate(), scope.OperationError, scope.CleanupError); err != nil {
			t.Fatalf("native fixture=%v,want complete cleanup", err)
		}
	})
}
