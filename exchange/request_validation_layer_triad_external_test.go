package exchange_test

import (
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestRequestFamilyValidationLayerTriad(t *testing.T) {
	t.Parallel()

	target := mustEndpoint(t, "https://example.test")
	status := mustHTTPStatus(t, 200)
	oneByte := mustByteCount(t, 1)
	single := exchange.RequestSemantics{
		Method: core.HTTPMethodGet,
		Replay: exchange.ReplaySingleAttempt,
	}

	t.Run("positive every request family admits one complete typed value", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			validate func() error
			name     string
		}{
			{
				name: "typed JSON request",
				validate: func() error {
					return (exchange.JSONRequest[transportDocument]{
						Target: target, Body: transportDocument{Message: "ready"},
						Semantics: single, ExpectedStatus: status,
					}).Validate()
				},
			},
			{
				name: "body-absent request",
				validate: func() error {
					return (exchange.NoBodyRequest{
						Target: target, Semantics: single, ExpectedStatus: status,
					}).Validate()
				},
			},
			{
				name: "body-absent bounded request",
				validate: func() error {
					return (exchange.NoBodyBoundedRequest{
						Target: target, Semantics: single, ExpectedStatus: status,
					}).Validate()
				},
			},
			{
				name: "aggregate byte request",
				validate: func() error {
					return (exchange.BoundedRequest{
						Target: target, Semantics: single,
						RequestContentType: core.HTTPMediaTypeOctetStream(),
						ExpectedStatus:     status,
					}).Validate()
				},
			},
			{
				name: "stream upload request",
				validate: func() error {
					return (exchange.UploadRequest{
						Target: target, Source: strings.NewReader("payload"),
						Semantics: single,
						ContentLength: core.NewByteLength(
							uint64(len("payload")),
						),
						ContentType:    core.HTTPMediaTypeOctetStream(),
						ExpectedStatus: status,
					}).Validate()
				},
			},
			{
				name: "stream download request",
				validate: func() error {
					return (exchange.DownloadRequest{
						Target: target, Destination: io.Discard,
						Semantics:         single,
						ResponseBodyLimit: oneByte,
						ExpectedStatus:    status,
					}).Validate()
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if gotErr := tc.validate(); gotErr != nil {
					t.Fatalf("request Validate() error = %v, want nil", gotErr)
				}
			})
		}
	})

	t.Run("negative missing capabilities and conflicting contracts are refused", func(t *testing.T) {
		t.Parallel()

		replayable := exchange.RequestSemantics{
			Method: core.HTTPMethodGet,
			Replay: exchange.ReplaySafe,
		}
		cases := []struct {
			wantErr  error
			validate func() error
			name     string
		}{
			{
				name: "JSON request refuses an absent owning body",
				validate: func() error {
					return (exchange.JSONRequest[*transportDocument]{
						Target: target, Body: nil,
						Semantics: single, ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "body-absent request refuses an absent target",
				validate: func() error {
					return (exchange.NoBodyRequest{
						Semantics: single, ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "bounded request refuses an absent request content type",
				validate: func() error {
					return (exchange.BoundedRequest{
						Target: target, Semantics: single,
						ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeContentType,
			},
			{
				name: "upload refuses an absent source",
				validate: func() error {
					return (exchange.UploadRequest{
						Target: target, Semantics: single,
						ContentType:    core.HTTPMediaTypeOctetStream(),
						ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "upload refuses a replayable declaration",
				validate: func() error {
					return (exchange.UploadRequest{
						Target: target, Source: strings.NewReader("payload"),
						Semantics: replayable,
						ContentLength: core.NewByteLength(
							uint64(len("payload")),
						),
						ContentType:    core.HTTPMediaTypeOctetStream(),
						ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "upload refuses an extent outside net/http signed range",
				validate: func() error {
					return (exchange.UploadRequest{
						Target: target, Source: strings.NewReader("payload"),
						Semantics:      single,
						ContentLength:  core.NewByteLength(math.MaxUint64),
						ContentType:    core.HTTPMediaTypeOctetStream(),
						ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "upload refuses an absent content type",
				validate: func() error {
					return (exchange.UploadRequest{
						Target: target, Source: strings.NewReader("payload"),
						Semantics: single,
						ContentLength: core.NewByteLength(
							uint64(len("payload")),
						),
						ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeContentType,
			},
			{
				name: "download refuses an absent destination",
				validate: func() error {
					return (exchange.DownloadRequest{
						Target: target, Semantics: single,
						ResponseBodyLimit: oneByte,
						ExpectedStatus:    status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "download refuses a replayable declaration",
				validate: func() error {
					return (exchange.DownloadRequest{
						Target: target, Destination: io.Discard,
						Semantics:         replayable,
						ResponseBodyLimit: oneByte,
						ExpectedStatus:    status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
			{
				name: "download refuses a zero response bound",
				validate: func() error {
					return (exchange.DownloadRequest{
						Target: target, Destination: io.Discard,
						Semantics:      single,
						ExpectedStatus: status,
					}).Validate()
				},
				wantErr: core.ErrExchangeRequest,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gotErr := tc.validate()
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("request Validate() error = %v, want %v", gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero-length payloads preserve structural body presence", func(t *testing.T) {
		t.Parallel()

		boundedErr := (exchange.BoundedRequest{
			Target: target, Semantics: single, Body: []byte{},
			RequestContentType: core.HTTPMediaTypeOctetStream(),
			ExpectedStatus:     status,
		}).Validate()
		uploadErr := (exchange.UploadRequest{
			Target: target, Source: strings.NewReader(""),
			Semantics: single, ContentLength: core.NewByteLength(0),
			ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: status,
		}).Validate()
		if boundedErr != nil || uploadErr != nil {
			t.Fatalf(
				"zero-length bounded/upload validation errors = (%v, %v), want (nil, nil)",
				boundedErr,
				uploadErr,
			)
		}
	})
}
