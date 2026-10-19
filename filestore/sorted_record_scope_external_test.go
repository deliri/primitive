package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func sortedScopeRecord(ctx context.Context, source lineio.RecordSource, record lineio.RecordRange) (string, error) {
	length, err := record.Bytes.Int64()
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.NewSectionReader(source, int64(record.Offset), length))
	return string(data), err
}

func sortedScopeCompare(ctx context.Context, source lineio.RecordSource, left, right lineio.RecordRange) (core.Comparison, error) {
	a, err := sortedScopeRecord(ctx, source, left)
	if err != nil {
		return 0, err
	}
	b, err := sortedScopeRecord(ctx, source, right)
	if err != nil {
		return 0, err
	}
	// Only the first byte determines order, so equal-key arrival order is
	// independently observable through each row's different suffix.
	switch strings.Compare(a[:1], b[:1]) {
	case -1:
		return core.ComparisonLess, nil
	case 1:
		return core.ComparisonGreater, nil
	default:
		return core.ComparisonEqual, nil
	}
}

func TestSortedRecordScopeOwnsNativeStreamIndexAndCleanup(t *testing.T) {
	t.Parallel()
	var mergeRows []string
	for i := range 4097 {
		mergeRows = append(mergeRows, fmt.Sprintf("%c:%d\n", 'a'+i%8, i))
	}
	for _, rows := range [][]string{{"c:first\n", "a:first\n", "b:first\n", "a:second\n"}, {}, mergeRows} {
		t.Run(fmt.Sprintf("%d rows", len(rows)), func(t *testing.T) {
			t.Parallel()
			parentText := t.TempDir()
			parent, err := core.ParseAbsolutePath(parentText)
			if err != nil {
				t.Fatal(err)
			}
			var borrowed filestore.SortedRecordReadView
			var got []string
			err = filestore.WithSortedRecordScope(t.Context(), filestore.SortedRecordScopeRequest{Parent: parent, Compare: sortedScopeCompare,
				Produce: func(_ context.Context, writer io.Writer) error {
					for _, row := range rows {
						if _, err := io.WriteString(writer, row); err != nil {
							return err
						}
					}
					return nil
				}, Use: func(ctx context.Context, view filestore.SortedRecordReadView) error {
					borrowed = view
					if err := view.Validate(); err != nil {
						return err
					}
					for {
						record, err := filestore.ReadRecordRangeIndex(view.Index)
						if errors.Is(err, io.EOF) {
							return nil
						}
						if err != nil {
							return err
						}
						row, err := sortedScopeRecord(ctx, view.Source, record)
						if err != nil {
							return err
						}
						got = append(got, row)
					}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := slices.Clone(rows)
			slices.SortStableFunc(want, func(a, b string) int { return strings.Compare(a[:1], b[:1]) })
			if !slices.Equal(got, want) {
				t.Fatalf("native sorted tuples=%q, independent stable order=%q", got, want)
			}
			if _, err := borrowed.Index.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("borrowed index escaped lifetime: %v", err)
			}
			files, err := os.ReadDir(parentText)
			if err != nil || len(files) != 0 {
				t.Fatalf("native scope cleanup=%v/%v", files, err)
			}
		})
	}
}

func TestSortedRecordScopePreservesEachRefusalAndWithholdsUnreadiedView(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"produce", "compare", "use", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			parentText := t.TempDir()
			parent, err := core.ParseAbsolutePath(parentText)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			used := false
			err = filestore.WithSortedRecordScope(ctx, filestore.SortedRecordScopeRequest{Parent: parent,
				Produce: func(_ context.Context, writer io.Writer) error {
					if _, err := io.WriteString(writer, "b:one\na:two\n"); err != nil {
						return err
					}
					if stage == "produce" {
						return io.ErrClosedPipe
					}
					if stage == "cancel" {
						cancel(io.ErrClosedPipe)
					}
					return nil
				}, Compare: func(ctx context.Context, source lineio.RecordSource, a, b lineio.RecordRange) (core.Comparison, error) {
					if stage == "compare" {
						return 0, io.ErrClosedPipe
					}
					return sortedScopeCompare(ctx, source, a, b)
				}, Use: func(context.Context, filestore.SortedRecordReadView) error { used = true; return io.ErrClosedPipe },
			})
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("%s lost original refusal: %v", stage, err)
			}
			if used != (stage == "use") {
				t.Fatalf("unready native view escaped after %s refusal", stage)
			}
			if stage == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel lost native identity: %v", err)
			}
			files, err := os.ReadDir(parentText)
			if err != nil || len(files) != 0 {
				t.Fatalf("%s refused scope leaked namespace: %v/%v", stage, files, err)
			}
		})
	}
	if err := filestore.WithSortedRecordScope(t.Context(), filestore.SortedRecordScopeRequest{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("invalid request=%v", err)
	}
}

func FuzzSortedRecordScopeMatchesIndependentStableRows(f *testing.F) {
	f.Add([]byte{3, 1, 2, 1})
	f.Add([]byte{})
	f.Add([]byte{0, 255, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 32 {
			return
		}
		parent, err := core.ParseAbsolutePath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var expected, got []string
		for i, value := range data {
			expected = append(expected, fmt.Sprintf("%c:%d\n", 'a'+value%8, i))
		}
		err = filestore.WithSortedRecordScope(t.Context(), filestore.SortedRecordScopeRequest{Parent: parent, Compare: sortedScopeCompare,
			Produce: func(_ context.Context, writer io.Writer) error {
				for _, row := range expected {
					if _, err := io.WriteString(writer, row); err != nil {
						return err
					}
				}
				return nil
			},
			Use: func(ctx context.Context, view filestore.SortedRecordReadView) error {
				for position := filestore.RecordRangeIndexPosition(0); ; position++ {
					record, err := filestore.ReadRecordRangeIndexAt(ctx, filestore.RecordRangeIndexReadRequest{Source: view.Index, Position: position})
					if errors.Is(err, io.EOF) {
						return nil
					}
					if err != nil {
						return err
					}
					row, err := sortedScopeRecord(ctx, view.Source, record)
					if err != nil {
						return err
					}
					got = append(got, row)
				}
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		slices.SortStableFunc(expected, func(a, b string) int { return strings.Compare(a[:1], b[:1]) })
		if !slices.Equal(got, expected) {
			t.Fatalf("native tuples=%q, independently sorted rows=%q", got, expected)
		}
	})
}
