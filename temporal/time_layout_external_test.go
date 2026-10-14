package temporal_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestCallerTimeLayoutPreservesNativeFormatting(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			text := strings.Repeat("literal 東京 ", index) + time.RFC3339Nano
			layout, err := temporal.NewTimeLayout(text)
			if err != nil || string(layout) != text {
				t.Fatalf("layout custody=%q/%v", layout, err)
			}
			native := time.Unix(int64(index), int64(index)).UTC()
			got, err := temporal.FormatNativeTime(temporal.NativeTimeFormatRequest{Time: native, Layout: layout})
			if err != nil || got != native.Format(text) {
				t.Fatalf("native formatting drift=%q/%v", got, err)
			}
			zero, err := temporal.NewTimeLayout("")
			if !errors.Is(err, core.ErrTemporalContract) || zero != "" {
				t.Fatalf("empty layout admitted=%q/%v", zero, err)
			}
		})
	}
}

func FuzzCallerTimeLayoutUsesNativeFormat(f *testing.F) {
	f.Add(time.RFC3339Nano, int64(0))
	f.Add("literal 東京", int64(-1))
	f.Add("", int64(1))
	f.Fuzz(func(t *testing.T, text string, nanos int64) {
		layout, err := temporal.NewTimeLayout(text)
		if !nativeLayoutVaries(text) {
			if !errors.Is(err, core.ErrTemporalContract) || layout != "" {
				t.Fatalf("empty layout=%q/%v", layout, err)
			}
			return
		}
		if err != nil || string(layout) != text {
			t.Fatalf("layout extent or content changed=%q/%v", layout, err)
		}
		native := time.Unix(0, nanos).UTC()
		got, err := temporal.FormatNativeTime(temporal.NativeTimeFormatRequest{Time: native, Layout: layout})
		if err != nil || got != native.Format(text) {
			t.Fatalf("native formatting drift=%q/%v", got, err)
		}
	})
}

// This differential oracle asks Go to interpret the layout. It never searches
// token spellings or mirrors a Primitive parser.
func nativeLayoutVaries(text string) bool {
	first := time.Date(2006, time.January, 2, 15, 4, 5, 123456789, time.UTC)
	second := time.Date(1998, time.November, 24, 15, 4, 5, 123456789, time.UTC)
	return text != "" && first.Format(text) != second.Format(text)
}
