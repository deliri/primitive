package cloudflare

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestImageResizeClosedOptions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		fit        ImageFit
		format     ImageFormat
		metadata   ImageMetadata
		wantSuffix string
	}{
		{name: "scale down auto removes metadata", fit: ImageFitScaleDown, format: ImageFormatAuto, metadata: ImageMetadataNone, wantSuffix: "fit=scale-down,metadata=none,format=auto"},
		{name: "contain WebP retains copyright", fit: ImageFitContain, format: ImageFormatWebP, metadata: ImageMetadataCopyright, wantSuffix: "fit=contain,metadata=copyright,format=webp"},
		{name: "cover AVIF keeps metadata", fit: ImageFitCover, format: ImageFormatAVIF, metadata: ImageMetadataKeep, wantSuffix: "fit=cover,metadata=keep,format=avif"},
		{name: "crop is native fit", fit: ImageFitCrop, format: ImageFormatAuto, metadata: ImageMetadataNone, wantSuffix: "fit=crop,metadata=none,format=auto"},
		{name: "pad is native fit", fit: ImageFitPad, format: ImageFormatAuto, metadata: ImageMetadataNone, wantSuffix: "fit=pad,metadata=none,format=auto"},
		{name: "unset fit refuses", format: ImageFormatAuto, metadata: ImageMetadataNone},
		{name: "future fit refuses", fit: ImageFit(255), format: ImageFormatAuto, metadata: ImageMetadataNone},
		{name: "unset format refuses", fit: ImageFitScaleDown, metadata: ImageMetadataNone},
		{name: "future format refuses", fit: ImageFitScaleDown, format: ImageFormat(255), metadata: ImageMetadataNone},
		{name: "unset metadata policy refuses", fit: ImageFitScaleDown, format: ImageFormatAuto},
		{name: "future metadata policy refuses", fit: ImageFitScaleDown, format: ImageFormatAuto, metadata: ImageMetadata(255)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := imageResizeFixture(t)
			request.Fit, request.Format, request.Metadata = tc.fit, tc.format, tc.metadata
			got, err := request.Address()
			if tc.wantSuffix == "" {
				if !errors.Is(err, core.ErrCloudflareContract) || got != (core.HTTPEndpoint{}) {
					t.Fatalf("refused options = %v/%v, want zero and contract error", got, err)
				}
				return
			}
			if err != nil || !strings.HasSuffix(got.String(), tc.wantSuffix) {
				t.Fatalf("options = %q/%v, want suffix %q", got.String(), err, tc.wantSuffix)
			}
		})
	}
}

func TestImageResizePublicBindingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*ImageDetails, *ImageResizeRequest)
		wantErr error
	}{
		{name: "uploaded same account and image permits native transform"},
		{name: "draft is no delivery proof", mutate: func(d *ImageDetails, _ *ImageResizeRequest) { d.Draft = true }, wantErr: core.ErrCloudflareResponse},
		{name: "private image cannot get unsigned resize", mutate: func(d *ImageDetails, _ *ImageResizeRequest) { d.RequireSignedURLs = true }, wantErr: core.ErrCloudflareResponse},
		{name: "absent variants do not prove account", mutate: func(d *ImageDetails, _ *ImageResizeRequest) { d.Variants = nil }, wantErr: core.ErrCloudflareBinding},
		{name: "valid foreign image cannot replace subject", mutate: func(_ *ImageDetails, r *ImageResizeRequest) { r.Source.Image = ImageID{value: "foreign"} }, wantErr: core.ErrCloudflareBinding},
		{name: "valid foreign account cannot replace authority", mutate: func(_ *ImageDetails, r *ImageResizeRequest) {
			r.Source.Account = ImageDeliveryAccount{value: "foreign"}
		}, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			details, err := imageDetailsFixture().details()
			if err != nil {
				t.Fatal(err)
			}
			request := imageResizeFixture(t)
			request.Source.Account = ImageDeliveryAccount{value: "account"}
			request.Source.Image = details.ID
			if tc.mutate != nil {
				tc.mutate(&details, &request)
			}
			got, err := details.PublicResize(request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("public resize = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if got != (core.HTTPEndpoint{}) {
					t.Fatalf("refused address = %v, want zero", got)
				}
				return
			}
			want, err := request.Address()
			if err != nil || got != want {
				t.Fatalf("public resize = %v, want %v/%v", got, want, err)
			}
		})
	}
}

func TestImageResizeCancellationJoinsStreamingDecoder(t *testing.T) {
	t.Parallel()
	backstop, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: testPolicy().OperationTimeout})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	ctx, cancel := context.WithCancel(backstop)
	defer cancel()
	started := make(chan struct{})
	client, err := NewImagesClient(testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{")); err != nil {
			t.Error(err)
			return
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
			return
		}
		close(started)
		<-r.Context().Done()
	}))
	if err != nil {
		t.Fatal(err)
	}
	request := imageResizeFixture(t)
	finished := make(chan imageInfoDecoded, 1)
	go func() {
		value, err := client.InspectResize(ctx, request, testPolicy())
		finished <- imageInfoDecoded{value: value, err: err}
	}()
	select {
	case <-started:
		cancel()
	case <-ctx.Done():
		t.Fatal("metadata stream never started")
	}
	var result imageInfoDecoded
	select {
	case result = <-finished:
	case <-backstop.Done():
		t.Fatal("metadata decoder did not join")
	}
	if result.value != (ImageInfo{}) || !errors.Is(result.err, context.Canceled) {
		t.Fatalf("cancelled result = %+v/%v, want zero and cancelled", result.value, result.err)
	}
}
