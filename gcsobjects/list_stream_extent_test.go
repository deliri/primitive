package gcsobjects

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// One native provider record is reused; neither fixture nor consumer retains
// an inventory. Metadata is independently checked against the provider name.
type nativeGCSListing struct {
	attrs    storage.ObjectAttrs
	terminal error
	extent   uint64
	next     uint64
}

func (s *nativeGCSListing) Next() (*storage.ObjectAttrs, error) {
	if s.next == s.extent {
		if s.terminal != nil {
			return nil, s.terminal
		}
		return nil, iterator.Done
	}
	s.attrs.Name = fmt.Sprintf("forge-recovery/v1/project/%09d.json", s.next)
	s.next++
	return &s.attrs, nil
}

func TestGCSListingContinuesBeyondFormerInventoryCeiling(t *testing.T) {
	t.Parallel()
	for _, extent := range [...]uint64{0, 1, 9999, 10000, 10001, 65537} {
		for _, terminal := range [...]error{nil, io.ErrUnexpectedEOF} {
			t.Run(strconv.FormatUint(extent, 10)+"/"+strconv.FormatBool(terminal != nil), func(t *testing.T) {
				t.Parallel()
				source := nativeGCSListing{attrs: *validMediaGCSAttrs(t), extent: extent, terminal: terminal}
				var visits uint64
				err := visitGCSObjects(&source, func(metadata GCSObjectMetadata) error {
					want := fmt.Sprintf("forge-recovery/v1/project/%09d.json", visits)
					generation, generationErr := metadata.Generation().Int64()
					if metadata.Name().String() != want || generationErr != nil || generation != source.attrs.Generation {
						t.Fatalf("provider projection = %v, want exact name %q and generation %d", metadata, want, source.attrs.Generation)
					}
					visits++
					return nil
				})
				if !errors.Is(err, terminal) || visits != extent || source.next != extent {
					t.Fatalf("listing = (%d visits, %d source advances, %v), want (%d,%d,%v)", visits, source.next, err, extent, extent, terminal)
				}
			})
		}
	}
}

func TestGCSListingSinkRefusalDoesNotAdvanceProviderAgain(t *testing.T) {
	t.Parallel()
	source := nativeGCSListing{attrs: *validMediaGCSAttrs(t), extent: 65537}
	var visits uint64
	err := visitGCSObjects(&source, func(GCSObjectMetadata) error {
		visits++
		if visits == 10001 {
			return io.ErrClosedPipe
		}
		return nil
	})
	if !errors.Is(err, io.ErrClosedPipe) || visits != 10001 || source.next != visits {
		t.Fatalf("consumer refusal = (%d visits,%d advances,%v), want (10001,10001,io.ErrClosedPipe)", visits, source.next, err)
	}
}

func FuzzGCSListingNativePrefixConservation(f *testing.F) {
	for _, count := range [...]uint16{0, 1, 9999, 10000, 10001, 65535} {
		f.Add(count, uint16(0))
	}
	f.Add(uint16(10001), uint16(10000))
	f.Fuzz(func(t *testing.T, count, refuse uint16) {
		source := nativeGCSListing{attrs: *validMediaGCSAttrs(t), extent: uint64(count)}
		var visits uint64
		err := visitGCSObjects(&source, func(metadata GCSObjectMetadata) error {
			want := fmt.Sprintf("forge-recovery/v1/project/%09d.json", visits)
			if metadata.Name().String() != want {
				t.Fatalf("name = %q, want %q", metadata.Name().String(), want)
			}
			visits++
			if refuse != 0 && visits == uint64(refuse) {
				return io.ErrClosedPipe
			}
			return nil
		})
		wantCount := uint64(count)
		var wantErr error
		if refuse != 0 && refuse <= count {
			wantCount = uint64(refuse)
			wantErr = io.ErrClosedPipe
		}
		if !errors.Is(err, wantErr) || visits != wantCount || source.next != visits {
			t.Fatalf("native prefix = (%d,%d,%v), want (%d,%d,%v)", visits, source.next, err, wantCount, wantCount, wantErr)
		}
	})
}

func BenchmarkGCSListingNativeVisitor(b *testing.B) {
	for _, count := range [...]uint64{8, 10001} {
		b.Run(strconv.FormatUint(count, 10), func(b *testing.B) {
			attrs := *validMediaGCSAttrs(b)
			b.ReportAllocs()
			for b.Loop() {
				source := nativeGCSListing{attrs: attrs, extent: count}
				var visits uint64
				err := visitGCSObjects(&source, func(metadata GCSObjectMetadata) error { visits++; return metadata.Validate() })
				if err != nil || visits != count {
					b.Fatalf("listing = (%d,%v), want (%d,nil)", visits, err, count)
				}
			}
		})
	}
}
