package gcsobjects

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	storageapi "google.golang.org/api/storage/v1"
)

func TestGCSListRangeNativeQueryLayerTriad(t *testing.T) {
	t.Parallel()
	first := "brand/_usage_2026_10_28_20_00_00_first_v0"
	middle := "brand/_usage_2026_10_28_20_59_59_late_v0"
	before := "brand/_usage_2026_10_28_19_59_59_neighbor_v0"
	after := "brand/_usage_2026_10_28_21_00_00_neighbor_v0"
	source := []string{before, first, middle, after}
	for _, tc := range []struct {
		name, start, end                     string
		want                                 []string
		wantErr                              error
		ignore, cancel, refuse, unauthorized bool
		wantCalls                            int
	}{
		{name: "positive exact closed-open native range", start: first, end: after, want: []string{first, middle}, wantCalls: 1},
		{name: "positive flat hour filename bounds", start: "brand/_usage_2026_10_28_20", end: "brand/_usage_2026_10_28_21", want: []string{first, middle}, wantCalls: 1},
		{name: "positive open start preserves native upper bound", end: first, want: []string{before}, wantCalls: 1},
		{name: "positive open end preserves native lower bound", start: middle, want: []string{middle, after}, wantCalls: 1},
		{name: "neutral zero range preserves incumbent prefix listing", want: source, wantCalls: 1},
		{name: "neutral equal bounds return an empty native interval", start: first, end: first, want: []string{}, wantCalls: 1},
		{name: "neutral native range and directory prefix intersect", start: "other/a", end: "other/z", want: []string{}, wantCalls: 1},
		{name: "negative reverse bounds refuse before native request", start: after, end: first, wantErr: core.ErrObjectStoreContract},
		{name: "negative provider name outside requested range never reaches visitor", start: first, end: after, ignore: true, wantErr: core.ErrObjectStoreIntegrity, wantCalls: 1},
		{name: "negative caller visitor cause survives range guard", start: first, end: after, refuse: true, want: []string{first}, wantErr: core.ErrObjectStoreDestination, wantCalls: 1},
		{name: "negative native provider refusal retains source identity", start: first, end: after, unauthorized: true, wantErr: core.ErrObjectStoreSource, wantCalls: 1},
		{name: "neutral canceled caller issues no native query", start: first, end: after, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := &atomic.Int32{}
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/o") {
					t.Errorf("list request=%s %s,want native objects GET", r.Method, r.URL.Path)
				}
				query := r.URL.Query()
				if query.Get("prefix") != "brand/" || query.Get("startOffset") != tc.start || query.Get("endOffset") != tc.end {
					t.Errorf("native prefix/start/end=%q/%q/%q,want brand/ %q %q", query.Get("prefix"), query.Get("startOffset"), query.Get("endOffset"), tc.start, tc.end)
				}
				if (tc.start == "" && query.Has("startOffset")) || (tc.end == "" && query.Has("endOffset")) {
					t.Errorf("omitted bound query=%v,want no empty offset parameters", query)
				}
				if tc.unauthorized {
					w.WriteHeader(http.StatusUnauthorized)
					writeProviderJSON(t, w, struct {
						Error struct {
							Code    int    `json:"code"`
							Message string `json:"message"`
						} `json:"error"`
					}{Error: struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}{Code: http.StatusUnauthorized, Message: "fixture access refused"}})
					return
				}
				result := storageapi.Objects{Items: []*storageapi.Object{}}
				for _, name := range source {
					if !tc.ignore && ((query.Get("startOffset") != "" && name < query.Get("startOffset")) || (query.Get("endOffset") != "" && name >= query.Get("endOffset"))) {
						continue
					}
					object := (&gcsListReadProvider{t: t, payload: []byte("x")}).object()
					object.Name = name
					result.Items = append(result.Items, &object)
					if tc.ignore {
						break
					}
				}
				writeProviderJSON(t, w, result)
			}))
			request := GCSListRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Prefix: parsedGCSObjectPrefix(t, "brand/")}
			if tc.start != "" {
				request.Range.Start = parsedGCSObjectName(t, tc.start)
			}
			if tc.end != "" {
				request.Range.End = parsedGCSObjectName(t, tc.end)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			var got []string
			err := ListGCSObjects(ctx, client, request, func(metadata GCSObjectMetadata) error {
				got = append(got, metadata.Name().String())
				if tc.refuse {
					return core.ErrObjectStoreDestination
				}
				return nil
			})
			if !errors.Is(err, tc.wantErr) || !slices.Equal(got, tc.want) {
				t.Fatalf("ListGCSObjects()=(%q,%v),want (%q,%v)", got, err, tc.want, tc.wantErr)
			}
			if calls.Load() != int32(tc.wantCalls) {
				t.Fatalf("native calls=%d,want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

func TestGCSListRangeAdmissionBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		rangeValue GCSListRange
		wantErr    error
	}{
		{name: "zero range is unbounded"},
		{name: "valid lower bound has native open end", rangeValue: GCSListRange{Start: parsedGCSObjectName(t, "brand/a")}},
		{name: "valid upper bound has native open start", rangeValue: GCSListRange{End: parsedGCSObjectName(t, "brand/z")}},
		{name: "equal nonzero bounds encode empty", rangeValue: GCSListRange{Start: parsedGCSObjectName(t, "brand/a"), End: parsedGCSObjectName(t, "brand/a")}},
		{name: "ascending bounds are admitted", rangeValue: GCSListRange{Start: parsedGCSObjectName(t, "brand/a"), End: parsedGCSObjectName(t, "brand/z")}},
		{name: "descending bounds are refused", rangeValue: GCSListRange{Start: parsedGCSObjectName(t, "brand/z"), End: parsedGCSObjectName(t, "brand/a")}, wantErr: core.ErrObjectStoreContract},
		{name: "invalid lower nominal is refused", rangeValue: GCSListRange{Start: GCSObjectName{value: "\n"}}, wantErr: core.ErrObjectStoreContract},
		{name: "invalid upper nominal is refused", rangeValue: GCSListRange{End: GCSObjectName{value: "\n"}}, wantErr: core.ErrObjectStoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.rangeValue.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("GCSListRange.Validate()=%v,want %v", got, tc.wantErr)
			}
			if got := tc.rangeValue.IsZero(); got != (tc.rangeValue == (GCSListRange{})) {
				t.Fatalf("IsZero()=%t,want exact zero range identity", got)
			}
		})
	}
}
