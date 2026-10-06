package cloudflare

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func imageInfoFixture(t testing.TB) ImageInfo {
	t.Helper()
	format, err := core.ParseHTTPMediaType("image/png")
	if err != nil {
		t.Fatal(err)
	}
	length, err := core.NewByteLength(754168)
	if err != nil {
		t.Fatal(err)
	}
	return ImageInfo{ImageDimensions: core.ImageDimensions{Width: 320, Height: 400}, Original: ImageOriginalInfo{ImageDimensions: core.ImageDimensions{Width: 1122, Height: 1402}, FileSize: length, Format: format}}
}

func imageInfoBytes(t testing.TB, value ImageInfo) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestImageInfoAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	seed := imageInfoFixture(t)
	canonical := imageInfoBytes(t, seed)
	for _, tc := range []struct {
		name    string
		mutate  func(*ImageInfo)
		raw     string
		wantErr error
	}{
		{name: "observed resized dimensions remain distinct from original"},
		{name: "one output pixel is meaningful", mutate: func(i *ImageInfo) { i.Width, i.Height = 1, 1 }},
		{name: "small original remains an original fact", mutate: func(i *ImageInfo) { i.Original.Width, i.Original.Height = 1, 1 }},
		{name: "provider may report upscaling", mutate: func(i *ImageInfo) { i.Width = i.Original.Width + 1 }},
		{name: "no invented dimension ceiling", mutate: func(i *ImageInfo) { i.Width, i.Original.Width = math.MaxUint64, math.MaxUint64 }},
		{name: "absent body", raw: " ", wantErr: core.ErrCloudflareResponse},
		{name: "null is not metadata", raw: "null", wantErr: core.ErrCloudflareResponse},
		{name: "empty object is not metadata", raw: "{}", wantErr: core.ErrCloudflareResponse},
		{name: "array is not metadata", raw: "[]", wantErr: core.ErrCloudflareResponse},
		{name: "truncated body cannot leak original facts", raw: string(canonical[:len(canonical)-1]), wantErr: core.ErrCloudflareResponse},
		{name: "trailing document refuses", raw: string(canonical) + "{}", wantErr: core.ErrCloudflareResponse},
		{name: "unknown member refuses", raw: strings.TrimSuffix(string(canonical), "}") + `,"unexpected":1}`, wantErr: core.ErrCloudflareResponse},
		{name: "duplicate dimension refuses", raw: strings.TrimSuffix(string(canonical), "}") + `,"width":320}`, wantErr: core.ErrCloudflareResponse},
		{name: "fraction is not a pixel dimension", raw: strings.Replace(string(canonical), `"width":320`, `"width":320.5`, 1), wantErr: core.ErrCloudflareResponse},
		{name: "negative dimension refuses", raw: strings.Replace(string(canonical), `"width":320`, `"width":-1`, 1), wantErr: core.ErrCloudflareResponse},
		{name: "dimension above native representation refuses", raw: strings.Replace(string(canonical), `"width":320`, `"width":18446744073709551616`, 1), wantErr: core.ErrCloudflareResponse},
		{name: "quoted dimension refuses", raw: strings.Replace(string(canonical), `"width":320`, `"width":"320"`, 1), wantErr: core.ErrCloudflareResponse},
		{name: "zero output width refuses", mutate: func(i *ImageInfo) { i.Width = 0 }, wantErr: core.ErrCloudflareResponse},
		{name: "zero output height refuses", mutate: func(i *ImageInfo) { i.Height = 0 }, wantErr: core.ErrCloudflareResponse},
		{name: "zero original width refuses", mutate: func(i *ImageInfo) { i.Original.Width = 0 }, wantErr: core.ErrCloudflareResponse},
		{name: "zero original height refuses", mutate: func(i *ImageInfo) { i.Original.Height = 0 }, wantErr: core.ErrCloudflareResponse},
		{name: "nonimage source cannot masquerade as image", mutate: func(i *ImageInfo) { i.Original.Format = core.HTTPMediaTypeJSON() }, wantErr: core.ErrCloudflareResponse},
		{name: "whitespace across many former document windows", raw: strings.Repeat(" \n", 1024*1024) + string(canonical)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := seed
			if tc.mutate != nil {
				tc.mutate(&want)
				if want == seed {
					t.Fatal("mutation changed no fact")
				}
			}
			input := imageInfoBytes(t, want)
			if tc.raw != "" {
				input = []byte(tc.raw)
			}
			if tc.wantErr != nil {
				want = ImageInfo{}
			}
			got, err := ReadImageInfo(bytes.NewReader(input))
			if !errors.Is(err, tc.wantErr) || got != want {
				t.Fatalf("ReadImageInfo = (%+v,%v), want (%+v,%v)", got, err, want, tc.wantErr)
			}
		})
	}
}

func imageResizeFixture(t testing.TB) ImageResizeRequest {
	t.Helper()
	delivery := imageDeliveryFixture(t)
	return ImageResizeRequest{Source: ImageDeliverySource{Origin: delivery.Origin, Account: delivery.Account, Image: delivery.Image}, Width: 320, Fit: ImageFitScaleDown, Format: ImageFormatAuto, Metadata: ImageMetadataNone}
}

func TestImageResizeNativeDimensionsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		width, height core.PixelDimension
		wantOptions   string
		wantErr       error
	}{
		{name: "width constrained height native", width: 320, wantOptions: "width=320,"},
		{name: "height constrained width native", height: 200, wantOptions: "height=200,"},
		{name: "both axes constrained", width: 320, height: 200, wantOptions: "width=320,height=200,"},
		{name: "one pixel remains representable", width: 1, wantOptions: "width=1,"},
		{name: "provider receives native representable maximum", width: math.MaxUint64, wantOptions: "width=18446744073709551615,"},
		{name: "neither axis is a resize", wantErr: core.ErrCloudflareContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := imageResizeFixture(t)
			request.Width, request.Height = tc.width, tc.height
			got, err := request.Address()
			want := ""
			if tc.wantErr == nil {
				want = "https://media.example.test/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/photo/" + tc.wantOptions + "fit=scale-down,metadata=none,format=auto"
			}
			if !errors.Is(err, tc.wantErr) || got.String() != want {
				t.Fatalf("address = %q/%v, want %q/%v", got.String(), err, want, tc.wantErr)
			}
		})
	}
}

func TestImageResizeInspectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                               string
		status                             int
		malformed, cancelled, emptyRequest bool
		wantErr                            error
		wantCalls                          int64
	}{
		{name: "real TLS metadata decode retains exact dimensions", status: http.StatusOK, wantCalls: 1},
		{name: "malformed provider body closes pipe and joins decoder", status: http.StatusOK, malformed: true, wantCalls: 1, wantErr: core.ErrCloudflareResponse},
		{name: "native absence retains status refusal", status: http.StatusNotFound, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "caller cancellation emits no observation", cancelled: true, wantErr: context.Canceled},
		{name: "invalid intent has no external effect", emptyRequest: true, wantErr: core.ErrCloudflareContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := imageResizeFixture(t)
			want := imageInfoFixture(t)
			body := imageInfoBytes(t, want)
			if tc.malformed {
				body = []byte("{")
			}
			var calls atomic.Int64
			client, err := NewImagesClient(testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				wantPath := "/cdn-cgi/imagedelivery/ZWd9g1K7eljCn_KDTu_MWA/tenant/photo/width=320,fit=scale-down,metadata=none,format=json,anim=false"
				if r.URL.Path != wantPath || r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
					t.Errorf("request = %s %s, want credential-free exact metadata GET", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			if tc.emptyRequest {
				request = ImageResizeRequest{}
			}
			got, err := client.InspectResize(ctx, request, testPolicy())
			if tc.status == http.StatusNotFound {
				var status exchange.StatusError
				if !errors.As(err, &status) || !status.Status().IsNotFound() {
					t.Fatalf("status refusal = %v, want typed 404", err)
				}
			}
			if tc.wantErr != nil {
				want = ImageInfo{}
			}
			if !errors.Is(err, tc.wantErr) || got != want || calls.Load() != tc.wantCalls {
				t.Fatalf("inspect = (%+v,%v,%d calls), want (%+v,%v,%d)", got, err, calls.Load(), want, tc.wantErr, tc.wantCalls)
			}
		})
	}
}

func FuzzImageInfoSemanticClosure(f *testing.F) {
	seed := imageInfoFixture(f)
	canonical := imageInfoBytes(f, seed)
	f.Add(canonical)
	f.Add([]byte("null"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := ReadImageInfo(bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (ImageInfo{}) {
				t.Fatalf("refused metadata = (%+v,%v), want zero and response error", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted validation = %v, want nil", err)
		}
		var independent ImageInfo
		if err := json.Unmarshal(data, &independent, json.RejectUnknownMembers(true)); err != nil || independent != got {
			t.Fatalf("source facts = (%+v,%v), want %+v", independent, err, got)
		}
		encoded := imageInfoBytes(t, got)
		roundTrip, err := ReadImageInfo(bytes.NewReader(encoded))
		if err != nil || roundTrip != got || !bytes.Equal(encoded, imageInfoBytes(t, roundTrip)) {
			t.Fatalf("canonical round trip = (%+v,%v), want %+v and identical bytes", roundTrip, err, got)
		}
	})
}

func FuzzImageResizeInspectionBinding(f *testing.F) {
	seed := imageInfoFixture(f)
	f.Add(imageInfoBytes(f, seed))
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		want, wantErr := ReadImageInfo(bytes.NewReader(data))
		var calls atomic.Int64
		client, err := NewImagesClient(testExchange(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			// The production decoder can refuse and close the connection before
			// the malformed fixture is fully written. No other effect depends on it.
			if _, err := w.Write(data); err != nil {
				return
			}
		}))
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.InspectResize(t.Context(), imageResizeFixture(t), testPolicy())
		if calls.Load() != 1 {
			t.Fatalf("GET count = %d, want one", calls.Load())
		}
		if wantErr != nil {
			if got != (ImageInfo{}) || !errors.Is(err, core.ErrCloudflareResponse) {
				t.Fatalf("HTTP refusal = %+v/%v, want zero and metadata refusal", got, err)
			}
			return
		}
		if got != want || err != nil {
			t.Fatalf("HTTP metadata = %+v/%v, want exact decoded source %+v", got, err, want)
		}
	})
}
