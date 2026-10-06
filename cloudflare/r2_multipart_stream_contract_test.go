package cloudflare

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestR2MultipartScalarReceiptsHaveNoInventedExtentQuota(t *testing.T) {
	t.Parallel()
	// These are regression probes around deleted SDK quotas, not new limits.
	for _, size := range []int{1023, 1024, 1025, 4095, 4096, 4097, 8193} {
		value := strings.Repeat("v", size)
		id, err := ParseR2UploadID(value)
		if err != nil || id.String() != value {
			t.Fatalf("opaque ID of %d bytes = %v, want exact value and nil", size, err)
		}
		part := R2CompletedPart{PartNumber: 1, ETag: `"` + value + `"`}
		if err := part.Validate(); err != nil {
			t.Fatalf("quoted ETag of %d bytes = %v, want nil", size, err)
		}
	}
}

// Source cannot yield the second receipt until a real TLS peer has decoded the
// first. Pre-reading, pre-counting or buffering the manifest cannot pass.
func TestR2MultipartCompletionAppliesBackpressureThroughTLS(t *testing.T) {
	t.Parallel()
	acknowledged := make(chan struct{})
	var observed atomic.Uint32
	peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		decoder := xml.NewDecoder(r.Body)
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Error(err)
				return
			}
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != core.CloudflareR2MultipartPartElement {
				continue
			}
			var part R2CompletedPart
			if err := decoder.DecodeElement(&part, &start); err != nil {
				t.Error(err)
				return
			}
			number := observed.Add(1)
			if uint32(part.PartNumber) != number || part.ETag != `"receipt"` {
				t.Errorf("received part = %+v, want exact receipt %d", part, number)
			}
			if number == 1 {
				close(acknowledged)
			}
		}
		if r.ContentLength != -1 || observed.Load() != core.CloudflareR2MultipartMaximumParts {
			t.Errorf("declared/observed = %d/%d, want unknown length and complete native sequence", r.ContentLength, observed.Load())
		}
		result := multipartCompletedFixture{Bucket: "media-bucket", Key: "video/owner/large file.mp4", ETag: `"completed"`}
		if err := xml.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	})
	client, err := NewR2Client(peer)
	if err != nil {
		t.Fatal(err)
	}
	server := testR2Server(t, R2JurisdictionDefault)
	grant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartComplete))
	if err != nil {
		t.Fatal(err)
	}
	var yielded atomic.Uint32
	parts := func(ctx context.Context, yield func(R2CompletedPart) error) error {
		for number := uint16(1); number <= core.CloudflareR2MultipartMaximumParts; number++ {
			if number == 2 {
				select {
				case <-acknowledged:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if err := yield(R2CompletedPart{PartNumber: number, ETag: `"receipt"`}); err != nil {
				return err
			}
			yielded.Add(1)
		}
		return nil
	}
	got, err := client.CompleteMultipart(t.Context(), grant, parts, testPolicy())
	if err != nil || got.ETag != `"completed"` || yielded.Load() != observed.Load() {
		t.Fatalf("completion = %+v/%v, produced/received %d/%d, want exact complete stream", got, err, yielded.Load(), observed.Load())
	}
}

func TestR2MultipartCompletionSourceAndDestinationFailures(t *testing.T) {
	t.Parallel()
	sourceFailure := errors.New("receipt source failed")
	destinationFailure := errors.New("completion sink failed")
	for _, tc := range []struct {
		name        string
		source      R2CompletedParts
		destination io.Writer
		wantErr     error
	}{
		{name: "source failure before any receipt", source: func(context.Context, func(R2CompletedPart) error) error { return sourceFailure }, destination: io.Discard, wantErr: sourceFailure},
		{name: "source failure after a valid receipt", source: func(_ context.Context, yield func(R2CompletedPart) error) error {
			return errors.Join(yield(R2CompletedPart{PartNumber: 1, ETag: `"one"`}), sourceFailure)
		}, destination: io.Discard, wantErr: sourceFailure},
		{name: "destination refusal preserves identity", source: multipartTestParts(R2CompletedPart{PartNumber: 1, ETag: `"one"`}), destination: multipartFailingSink{cause: destinationFailure}, wantErr: destinationFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := writeMultipartCompletion(t.Context(), tc.destination, tc.source); !errors.Is(err, tc.wantErr) {
				t.Fatalf("stream error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

type multipartFailingSink struct{ cause error }

func (s multipartFailingSink) Write([]byte) (int, error) { return 0, s.cause }

func TestR2MultipartResponseContinuesAcrossFormerQuota(t *testing.T) {
	t.Parallel()
	// About 3 MiB of comments, written in a fixed 32 KiB fixture window.
	// The SDK must parse past the former 2 MiB refusal without retaining them.
	comment := "<!--" + strings.Repeat("x", exchange.TransferBufferBytes-7) + "-->"
	var transferred atomic.Int64
	peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		for range 96 {
			n, err := io.WriteString(w, comment)
			transferred.Add(int64(n))
			if err != nil {
				t.Error(err)
				return
			}
		}
		if _, err := w.Write(multipartCreatedBytes(t)); err != nil {
			t.Error(err)
		}
	})
	client, err := NewR2Client(peer)
	if err != nil {
		t.Fatal(err)
	}
	server := testR2Server(t, R2JurisdictionDefault)
	grant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartCreate))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.CreateMultipart(t.Context(), grant, testPolicy())
	if err != nil || got.UploadID.String() != "opaque+/upload==" || transferred.Load() != int64(96*len(comment)) {
		t.Fatalf("receipt = %+v/%v, comment bytes = %d, want exact receipt after %d streamed bytes", got, err, transferred.Load(), 96*len(comment))
	}
}

func TestR2MultipartPeerRefusalCancelsAndJoinsSource(t *testing.T) {
	t.Parallel()
	peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).EnableFullDuplex(); err != nil {
			t.Error(err)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
	})
	client, err := NewR2Client(peer)
	if err != nil {
		t.Fatal(err)
	}
	server := testR2Server(t, R2JurisdictionDefault)
	grant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartComplete))
	if err != nil {
		t.Fatal(err)
	}
	var exited atomic.Bool
	parts := func(ctx context.Context, yield func(R2CompletedPart) error) error {
		defer exited.Store(true)
		if err := yield(R2CompletedPart{PartNumber: 1, ETag: `"one"`}); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	}
	got, err := client.CompleteMultipart(t.Context(), grant, parts, testPolicy())
	if !errors.Is(err, context.Canceled) || got != (R2MultipartResult{}) || !exited.Load() {
		t.Fatalf("refusal = %+v/%v, source exited %t, want zero, cancellation and joined source", got, err, exited.Load())
	}
}
