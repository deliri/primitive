package upgrade

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The persistence contract is one nominal document. A growable file-sized
// accumulator defeats that contract before the decoder can reject its input.
func TestMetadataReadersCannotAccumulateWholeFiles(t *testing.T) {
	t.Parallel()
	for _, source := range upgradeProductionFiles(t) {
		for _, decl := range source.syntax.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || (fn.Name.Name != "readSelection" && fn.Name.Name != "readTrial") {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				selector, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if ok && pkg.Name == "bytes" && selector.Sel.Name == "Buffer" {
					t.Errorf("%s uses bytes.Buffer, want fixed nominal storage before file reads", fn.Name.Name)
				}
				return true
			})
		}
	}
}

// This exploratory allocation measurement uses an actual malformed persisted
// selector. Its declared workload is independent of the installed artifact.
func BenchmarkRejectOversizedMetadata(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatalf("OpenRoot error = %v, want nil", err)
	}
	b.Cleanup(func() {
		if err := root.Close(); err != nil {
			b.Errorf("root.Close error = %v, want nil", err)
		}
	})
	data := bytes.Repeat([]byte("x"), 16<<20)
	if err := root.WriteFile(selectionFilename, data, documentMode); err != nil {
		b.Fatalf("WriteFile error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := readSelection(b.Context(), root)
		if !errors.Is(err, core.ErrUpgradePersistence) || !errors.Is(err, core.ErrJSONContract) || got != (selectionDocument{}) {
			b.Fatalf("oversized selector = (%v, %v), want zero and typed JSON/persistence refusal", got, err)
		}
	}
}

func TestMetadataBufferLayerTriad(t *testing.T) {
	t.Parallel()
	for _, maximum := range []int{selectionDocumentMaximumBytes, trialDocumentMaximumBytes} {
		for _, size := range []int{0, maximum - 1, maximum, maximum + 1, maximum * 8} {
			t.Run(fmt.Sprintf("nominal extent %d input %d", maximum, size), func(t *testing.T) {
				t.Parallel()
				destination := metadataBuffer{maximum: maximum}
				payload := bytes.Repeat([]byte("x"), size)
				count, err := destination.Write(payload)
				if size > maximum {
					if count != 0 || !errors.Is(err, core.ErrUpgradeContract) || !errors.Is(err, core.ErrJSONContract) || destination.length != 0 {
						t.Fatalf("oversized Write = (%d, %v), retained %d, want zero and typed refusal", count, err, destination.length)
					}
					return
				}
				if err != nil || count != size || !bytes.Equal(destination.data[:destination.length], payload) {
					t.Fatalf("Write = (%d, %v), retained %d, want exact %d bytes", count, err, destination.length, size)
				}
				if size == maximum {
					count, err = destination.Write([]byte("y"))
					if count != 0 || !errors.Is(err, core.ErrJSONContract) || !bytes.Equal(destination.data[:destination.length], payload) {
						t.Fatalf("overflow after exact write = (%d, %v), want refusal and preserved prior bytes", count, err)
					}
				}
			})
		}
	}
}

func TestPersistedMetadataReadLayerTriad(t *testing.T) {
	t.Parallel()
	old := artifactForTest(t, []byte("installed"), 1)
	candidate := artifactForTest(t, []byte("candidate"), 2)
	selection := selectionDocument{Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: old}
	trial := trialDocument{Revision: trialRevisionCurrent, Prior: selection, Candidate: candidate}
	selected, err := encodeSelection(selection)
	if err != nil {
		t.Fatalf("encodeSelection error = %v, want nil", err)
	}
	trialBytes, err := encodeTrial(trial)
	if err != nil {
		t.Fatalf("encodeTrial error = %v, want nil", err)
	}
	for _, kind := range []struct {
		name      string
		canonical []byte
		maximum   int
	}{
		{"selector", selected, selectionDocumentMaximumBytes}, {"trial", trialBytes, trialDocumentMaximumBytes},
	} {
		for _, mode := range []string{"canonical", "empty", "oversized", "absent"} {
			t.Run(kind.name+" "+mode, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root, err := os.OpenRoot(directory)
				if err != nil {
					t.Fatalf("OpenRoot error = %v, want nil", err)
				}
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Errorf("Close error = %v, want nil", err)
					}
				})
				filename := selectionFilename
				if kind.name == "trial" {
					if err := root.Mkdir(SlotB.String(), directoryMode); err != nil {
						t.Fatalf("Mkdir error = %v, want nil", err)
					}
					filename = SlotB.String() + "/" + trialFilename
				}
				data := kind.canonical
				if mode == "empty" {
					data = nil
				}
				if mode == "oversized" {
					data = bytes.Repeat([]byte("x"), kind.maximum+1)
				}
				if mode != "absent" {
					if err := root.WriteFile(filename, data, documentMode); err != nil {
						t.Fatalf("WriteFile error = %v, want nil", err)
					}
				}
				var readErr error
				var matches, zero bool
				if kind.name == "selector" {
					got, err := readSelection(t.Context(), root)
					readErr = err
					matches = got == selection
					zero = got == (selectionDocument{})
				} else {
					got, err := readTrial(t.Context(), root, SlotB)
					readErr = err
					matches = got == trial
					zero = got == (trialDocument{})
				}
				if mode == "canonical" {
					if readErr != nil || !matches {
						t.Fatalf("read = (%t, %v), want exact nominal document", matches, readErr)
					}
				} else {
					var want error = core.ErrJSONContract
					if mode == "absent" {
						want = os.ErrNotExist
					}
					if !zero || !errors.Is(readErr, want) || !errors.Is(readErr, core.ErrUpgradePersistence) {
						t.Fatalf("read refusal = zero %t, %v, want zero and %v/persistence", zero, readErr, want)
					}
				}
				if mode != "absent" {
					got, err := root.ReadFile(filename)
					if err != nil || !bytes.Equal(got, data) {
						t.Fatalf("read changed persistence: (%d bytes, %v), want exact %d bytes", len(got), err, len(data))
					}
				}
			})
		}
	}
}
