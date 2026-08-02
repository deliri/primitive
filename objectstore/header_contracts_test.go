package objectstore

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// TestOwnedProtocolConstantsAreParsableHeaderNames closes the failure mode in
// which a mistyped protocol constant becomes the zero header name. A zero name
// matches nothing, so it would silently drop an emitted field and silently stop
// guarding a caller-signed field.
func TestOwnedProtocolConstantsAreParsableHeaderNames(t *testing.T) {
	t.Parallel()

	constants := []struct {
		name  string
		value string
	}{
		{name: "If-None-Match", value: headerIfNoneMatch},
		{name: "S3 CRC32C checksum", value: headerS3ChecksumCRC32C},
		{name: "S3 checksum mode", value: headerS3ChecksumMode},
		{name: "S3 checksum type", value: headerS3ChecksumType},
		{name: "S3 version identity", value: headerS3Version},
		{name: "GCS hash", value: headerGCSHash},
		{name: "GCS generation precondition", value: headerGCSGenerationMatch},
		{name: "GCS generation identity", value: headerGCSGeneration},
		{name: "Content-Range", value: headerContentRange},
		{name: "Content-Disposition", value: headerContentDisposition},
		{name: "Host", value: headerHost},
		{name: "Range", value: headerRange},
	}
	for _, tc := range constants {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := headerName(tc.value)
			if gotErr != nil {
				t.Fatalf(
					"headerName(%q) error = %v, want nil",
					tc.value,
					gotErr,
				)
			}
			if got.String() != tc.value {
				t.Fatalf(
					"headerName(%q) = %q, want the exact declared constant",
					tc.value,
					got.String(),
				)
			}
		})
	}

	t.Run("an unparsable constant fails loudly instead of becoming the zero name", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"", "Bad Header", "X-A\nB", "X-A:B"} {
			got, gotErr := headerName(value)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) ||
				got != (core.HTTPHeaderName{}) {
				t.Fatalf(
					"headerName(%q) = (%q, %v), want zero name and %v",
					value,
					got.String(),
					gotErr,
					core.ErrObjectStoreContract,
				)
			}
		}
	})
}

// TestOwnedHeaderSetIsExactlyTheDeclaredFields pins the one set of fields
// Objectstore sets itself. Every name in the set must be rejected as a
// caller-signed field, and every field Objectstore actually emits must be in it.
func TestOwnedHeaderSetIsExactlyTheDeclaredFields(t *testing.T) {
	t.Parallel()

	owned, gotErr := objectstoreOwnedHeaderNames()
	if gotErr != nil {
		t.Fatalf("objectstoreOwnedHeaderNames() error = %v, want nil", gotErr)
	}
	wantValues := []string{
		core.HTTPHeaderContentType().String(),
		core.HTTPHeaderContentLength().String(),
		core.HTTPHeaderAcceptEncoding().String(),
		core.HTTPHeaderContentEncoding().String(),
		core.HTTPHeaderAccept().String(),
		headerAuthorization,
		core.HTTPHeaderIdempotencyKey().String(),
		headerContentRange,
		headerHost,
		headerRange,
		headerIfNoneMatch,
		headerS3ChecksumCRC32C,
		headerS3ChecksumMode,
		headerGCSHash,
		headerGCSGenerationMatch,
	}
	gotValues := make([]string, 0, len(owned))
	for _, name := range owned {
		gotValues = append(gotValues, name.String())
	}
	if !slices.Equal(gotValues, wantValues) {
		t.Fatalf(
			"objectstore-owned fields = %q, want exactly %q",
			gotValues,
			wantValues,
		)
	}

	t.Run("every owned field is refused as a caller-signed field", func(t *testing.T) {
		t.Parallel()

		for _, name := range owned {
			_, err := NewSignedHeader(name, "value")
			if !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf(
					"NewSignedHeader(%q) error = %v, want %v",
					name.String(),
					err,
					core.ErrObjectStoreContract,
				)
			}
		}
	})

	t.Run("every emitted request field is an owned field", func(t *testing.T) {
		t.Parallel()

		emitted := []string{
			headerIfNoneMatch, headerS3ChecksumCRC32C,
			headerGCSGenerationMatch, headerGCSHash, headerS3ChecksumMode,
		}
		for _, value := range emitted {
			name, err := headerName(value)
			if err != nil {
				t.Fatalf("headerName(%q) error = %v, want nil", value, err)
			}
			got, ownedErr := objectstoreOwnedHeader(name)
			if ownedErr != nil || !got {
				t.Fatalf(
					"objectstoreOwnedHeader(%q) = (%t, %v), want (true, nil)",
					value,
					got,
					ownedErr,
				)
			}
		}
	})

	t.Run("an unrelated field is not owned", func(t *testing.T) {
		t.Parallel()

		name, err := headerName("X-Caller-Signed")
		if err != nil {
			t.Fatalf("headerName() error = %v, want nil", err)
		}
		got, ownedErr := objectstoreOwnedHeader(name)
		if ownedErr != nil || got {
			t.Fatalf(
				"objectstoreOwnedHeader(X-Caller-Signed) = (%t, %v), want (false, nil)",
				got,
				ownedErr,
			)
		}
	})
}

func TestSignedHeaderDeclarationTokenTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		declaration string
		token       string
		want        bool
	}{
		{name: "single exact token matches", declaration: "host", token: "host", want: true},
		{name: "first token matches", declaration: "host;if-none-match", token: "host", want: true},
		{name: "last token matches", declaration: "host;if-none-match", token: "if-none-match", want: true},
		{name: "interior token matches", declaration: "a;host;b", token: "host", want: true},
		{name: "token match is case insensitive", declaration: "Host", token: "host", want: true},
		{name: "declaration match is case insensitive", declaration: "host", token: "HOST", want: true},
		{name: "prefix of a token does not match", declaration: "hostname", token: "host"},
		{name: "suffix of a token does not match", declaration: "xhost", token: "host"},
		{name: "absent token does not match", declaration: "host;range", token: "if-none-match"},
		{name: "empty declaration does not match", declaration: "", token: "host"},
		{name: "empty token never matches an empty declaration", declaration: "", token: ""},
		{name: "empty token never matches a trailing separator", declaration: "host;", token: ""},
		{name: "empty token never matches a leading separator", declaration: ";host", token: ""},
		{name: "empty token never matches an interior separator", declaration: "host;;range", token: ""},
		{name: "empty token never matches a lone separator", declaration: ";", token: ""},
		{name: "whitespace padded token does not match", declaration: "host", token: " host"},
		{name: "whitespace padded declaration does not match", declaration: " host", token: "host"},
		{name: "comma separated declaration is not a token list", declaration: "host,range", token: "host"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := semicolonTokenContains(
				tc.declaration,
				tc.token,
			); got != tc.want {
				t.Fatalf(
					"semicolonTokenContains(%q, %q) = %t, want %t",
					tc.declaration,
					tc.token,
					got,
					tc.want,
				)
			}
		})
	}
}

func TestSignedHeaderDeclarationIsCanonicalAndBounded(t *testing.T) {
	t.Parallel()

	valid := []string{
		"host",
		"host;if-none-match;x-amz-checksum-crc32c",
		"accept;accept-encoding;host;x-goog-meta-run",
	}
	for _, declaration := range valid {
		declaration := declaration
		t.Run("valid "+declaration, func(t *testing.T) {
			t.Parallel()
			got, err := parseSignedHeaderDeclaration(declaration)
			if err != nil || got.count == 0 {
				t.Fatalf("parseSignedHeaderDeclaration(%q) = (%v, %v), want nonempty and nil",
					declaration, got, err)
			}
		})
	}
	invalid := []string{
		"",
		"Host",
		"host;",
		";host",
		"host;;range",
		"host;host",
		"x-goog-meta-run;host",
		"host;x goog meta run",
	}
	for _, declaration := range invalid {
		declaration := declaration
		t.Run("invalid "+declaration, func(t *testing.T) {
			t.Parallel()
			if _, err := parseSignedHeaderDeclaration(declaration); !errors.Is(
				err, core.ErrObjectStoreContract,
			) {
				t.Fatalf("parseSignedHeaderDeclaration(%q) error = %v, want %v",
					declaration, err, core.ErrObjectStoreContract)
			}
		})
	}

	boundary := make([]string, signedHeaderDeclarationMaximumCount+1)
	for index := range boundary {
		boundary[index] = fmt.Sprintf("x-objectstore-%02d", index)
	}
	exact := strings.Join(
		boundary[:signedHeaderDeclarationMaximumCount],
		signedHeaderTokenSeparator,
	)
	got, exactErr := parseSignedHeaderDeclaration(exact)
	if exactErr != nil || int(got.count) != signedHeaderDeclarationMaximumCount {
		t.Fatalf(
			"parseSignedHeaderDeclaration(exact maximum) = (count %d, %v), want (%d, nil)",
			got.count,
			exactErr,
			signedHeaderDeclarationMaximumCount,
		)
	}
	over := strings.Join(boundary, signedHeaderTokenSeparator)
	if _, err := parseSignedHeaderDeclaration(over); !errors.Is(
		err,
		core.ErrObjectStoreContract,
	) {
		t.Fatalf(
			"parseSignedHeaderDeclaration(one above maximum) error = %v, want %v",
			err,
			core.ErrObjectStoreContract,
		)
	}
}

func TestAmazonS3DataHostDomainBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		want bool
	}{
		{host: "s3.amazonaws.com", want: true},
		{host: "bucket.s3.us-west-2.amazonaws.com", want: true},
		{host: "s3.dualstack.us-west-2.amazonaws.com", want: true},
		{host: "access-123.s3-accesspoint.us-west-2.amazonaws.com", want: true},
		{host: "bucket.s3.cn-north-1.amazonaws.com.cn", want: true},
		{host: "amazonaws.com"},
		{host: "sts.amazonaws.com"},
		{host: "s3.amazonaws.com.attacker.example"},
		{host: "attacker.example"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.host, func(t *testing.T) {
			t.Parallel()
			if got := amazonS3DataHost(tc.host); got != tc.want {
				t.Fatalf("amazonS3DataHost(%q) = %t, want %t", tc.host, got, tc.want)
			}
		})
	}
}

func TestProviderSignedHeaderContractsAreClosed(t *testing.T) {
	t.Parallel()

	t.Run("signature-bearing providers declare a query and required fields", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			wantQuery    string
			wantRequired []string
			provider     Provider
		}{
			{
				provider:     ProviderAmazonS3,
				wantQuery:    queryS3SignedHeaders,
				wantRequired: []string{headerIfNoneMatch, headerS3ChecksumCRC32C},
			},
			{
				provider:     ProviderGoogleCloudStorage,
				wantQuery:    queryGCSSignedHeaders,
				wantRequired: []string{headerGCSGenerationMatch, headerGCSHash},
			},
		}
		for _, tc := range cases {
			gotQuery, gotErr := providerSignedHeadersQuery(tc.provider)
			if gotErr != nil || gotQuery != tc.wantQuery {
				t.Fatalf(
					"providerSignedHeadersQuery(%v) = (%q, %v), want (%q, nil)",
					tc.provider,
					gotQuery,
					gotErr,
					tc.wantQuery,
				)
			}
			gotRequired, gotRequiredErr := providerRequiredSignedHeaders(
				tc.provider,
			)
			if gotRequiredErr != nil ||
				!slices.Equal(gotRequired, tc.wantRequired) {
				t.Fatalf(
					"providerRequiredSignedHeaders(%v) = (%q, %v), want (%q, nil)",
					tc.provider,
					gotRequired,
					gotRequiredErr,
					tc.wantRequired,
				)
			}
		}
	})

	t.Run("one-time capability providers declare no signed-header query", func(t *testing.T) {
		t.Parallel()

		_, gotErr := providerSignedHeadersQuery(ProviderCloudflareImages)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"providerSignedHeadersQuery(cloudflare) error = %v, want %v",
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
		gotRequired, gotRequiredErr := providerRequiredSignedHeaders(
			ProviderCloudflareImages,
		)
		if gotRequiredErr != nil || len(gotRequired) != 0 {
			t.Fatalf(
				"providerRequiredSignedHeaders(cloudflare) = (%q, %v), want (none, nil)",
				gotRequired,
				gotRequiredErr,
			)
		}
	})

	t.Run("providers outside the closed domain are typed contract failures", func(t *testing.T) {
		t.Parallel()

		for _, provider := range []Provider{
			ProviderUnknown, providerLimit, providerLimit + 1, Provider(200),
		} {
			if _, err := providerSignedHeadersQuery(provider); !errors.Is(
				err,
				core.ErrObjectStoreContract,
			) {
				t.Fatalf(
					"providerSignedHeadersQuery(%d) error = %v, want %v",
					provider,
					err,
					core.ErrObjectStoreContract,
				)
			}
			if _, err := providerRequiredSignedHeaders(provider); !errors.Is(
				err,
				core.ErrObjectStoreContract,
			) {
				t.Fatalf(
					"providerRequiredSignedHeaders(%d) error = %v, want %v",
					provider,
					err,
					core.ErrObjectStoreContract,
				)
			}
			if _, err := providerVersionHeader(provider); !errors.Is(
				err,
				core.ErrObjectStoreContract,
			) {
				t.Fatalf(
					"providerVersionHeader(%d) error = %v, want %v",
					provider,
					err,
					core.ErrObjectStoreContract,
				)
			}
		}
	})
}

func TestResponseSelectionByProviderAndDirection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		want      []string
		provider  Provider
		direction Direction
	}{
		{
			name:     "S3 upload captures only object identity",
			provider: ProviderAmazonS3, direction: DirectionUpload,
			want: []string{headerS3Version},
		},
		{
			name:     "S3 download captures identity, checksum type, and range",
			provider: ProviderAmazonS3, direction: DirectionDownload,
			want: []string{
				headerS3Version,
				headerS3ChecksumCRC32C,
				headerS3ChecksumType,
				headerContentRange,
			},
		},
		{
			name:     "GCS upload captures only generation identity",
			provider: ProviderGoogleCloudStorage, direction: DirectionUpload,
			want: []string{headerGCSGeneration},
		},
		{
			name:     "GCS download captures generation, hash, and range",
			provider: ProviderGoogleCloudStorage, direction: DirectionDownload,
			want: []string{
				headerGCSGeneration, headerGCSHash, headerContentRange,
			},
		},
		{
			name:     "Cloudflare upload captures nothing",
			provider: ProviderCloudflareImages, direction: DirectionUpload,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := responseSelection(tc.provider, tc.direction)
			if gotErr != nil {
				t.Fatalf(
					"responseSelection(%v, %v) error = %v, want nil",
					tc.provider,
					tc.direction,
					gotErr,
				)
			}
			gotValues := make([]string, 0, len(got.Names))
			for _, name := range got.Names {
				gotValues = append(gotValues, name.String())
			}
			if len(gotValues) != len(tc.want) ||
				(len(tc.want) != 0 && !slices.Equal(gotValues, tc.want)) {
				t.Fatalf(
					"responseSelection(%v, %v) = %q, want %q",
					tc.provider,
					tc.direction,
					gotValues,
					tc.want,
				)
			}
		})
	}

	t.Run("an unknown direction cannot select provider metadata", func(t *testing.T) {
		t.Parallel()

		for _, direction := range []Direction{
			DirectionUnknown, directionLimit, Direction(200),
		} {
			if _, err := responseSelection(
				ProviderAmazonS3,
				direction,
			); !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf(
					"responseSelection(s3, %d) error = %v, want %v",
					direction,
					err,
					core.ErrObjectStoreContract,
				)
			}
		}
	})

	t.Run("an unknown provider cannot select provider metadata", func(t *testing.T) {
		t.Parallel()

		for _, provider := range []Provider{ProviderUnknown, providerLimit} {
			if _, err := responseSelection(
				provider,
				DirectionUpload,
			); !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf(
					"responseSelection(%d, upload) error = %v, want %v",
					provider,
					err,
					core.ErrObjectStoreContract,
				)
			}
		}
	})
}

func capturedHeader(t testing.TB, name string, values ...string) exchange.Header {
	t.Helper()

	parsed, err := headerName(name)
	if err != nil {
		t.Fatalf("headerName(%q) setup error = %v, want nil", name, err)
	}
	return exchange.Header{Name: parsed, Values: values}
}

func TestProviderDownloadCRC32CProjectionTable(t *testing.T) {
	t.Parallel()

	// The base64 CRC32C of the four bytes "test" under Castagnoli.
	const encoded = "SUYRpg=="
	var want core.CRC32C
	gotErr := want.UnmarshalText([]byte(encoded))
	if gotErr != nil {
		t.Fatalf("core.CRC32C.UnmarshalText() setup error = %v, want nil", gotErr)
	}
	cases := []struct {
		wantErr     error
		name        string
		headers     []exchange.Header
		provider    Provider
		wantPresent bool
	}{
		{
			name:     "S3 supplies one legacy checksum without a type",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
			},
			wantPresent: true,
		},
		{
			name:     "S3 supplies one full-object checksum",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
				capturedHeader(
					t,
					headerS3ChecksumType,
					headerS3ChecksumFullObject,
				),
			},
			wantPresent: true,
		},
		{
			name:     "S3 composite checksum is not a whole-stream witness",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
				capturedHeader(
					t,
					headerS3ChecksumType,
					headerS3ChecksumComposite,
				),
			},
		},
		{
			name:     "GCS supplies one crc32c component",
			provider: ProviderGoogleCloudStorage,
			headers: []exchange.Header{
				capturedHeader(t, headerGCSHash, headerGCSChecksumPrefix+encoded),
			},
			wantPresent: true,
		},
		{
			name:     "GCS crc32c component alongside md5",
			provider: ProviderGoogleCloudStorage,
			headers: []exchange.Header{
				capturedHeader(
					t,
					headerGCSHash,
					"md5=1B2M2Y8AsgTpgAmY7PhCfg==,"+
						headerGCSChecksumPrefix+encoded,
				),
			},
			wantPresent: true,
		},
		{
			name:     "GCS crc32c component with surrounding whitespace",
			provider: ProviderGoogleCloudStorage,
			headers: []exchange.Header{
				capturedHeader(
					t,
					headerGCSHash,
					"md5=1B2M2Y8AsgTpgAmY7PhCfg== , "+
						headerGCSChecksumPrefix+encoded,
				),
			},
			wantPresent: true,
		},
		{
			name:     "S3 supplies no checksum",
			provider: ProviderAmazonS3,
			headers:  nil,
		},
		{
			name:     "GCS supplies only md5",
			provider: ProviderGoogleCloudStorage,
			headers: []exchange.Header{
				capturedHeader(t, headerGCSHash, "md5=1B2M2Y8AsgTpgAmY7PhCfg=="),
			},
		},
		{
			name:     "Cloudflare exposes no download checksum",
			provider: ProviderCloudflareImages,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
			},
		},
		{
			name:     "S3 repeats the checksum field",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name:     "S3 supplies two checksum values in one field",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded, encoded),
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name:     "GCS repeats the crc32c component",
			provider: ProviderGoogleCloudStorage,
			headers: []exchange.Header{
				capturedHeader(
					t,
					headerGCSHash,
					headerGCSChecksumPrefix+encoded+","+
						headerGCSChecksumPrefix+encoded,
				),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "S3 checksum is not base64",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, "not-base64!!"),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "S3 full-object checksum is not base64",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(
					t,
					headerS3ChecksumCRC32C,
					"not-base64!!",
				),
				capturedHeader(
					t,
					headerS3ChecksumType,
					headerS3ChecksumFullObject,
				),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "S3 unknown checksum type is rejected",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
				capturedHeader(t, headerS3ChecksumType, "FUTURE"),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "S3 repeated checksum type is contradictory",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, encoded),
				capturedHeader(
					t,
					headerS3ChecksumType,
					headerS3ChecksumFullObject,
				),
				capturedHeader(
					t,
					headerS3ChecksumType,
					headerS3ChecksumFullObject,
				),
			},
			wantErr: core.ErrObjectStoreContract,
		},
		{
			name:     "S3 checksum is the wrong width",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, "AAAA"),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "S3 checksum is empty",
			provider: ProviderAmazonS3,
			headers: []exchange.Header{
				capturedHeader(t, headerS3ChecksumCRC32C, ""),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "GCS crc32c component is not base64",
			provider: ProviderGoogleCloudStorage,
			headers: []exchange.Header{
				capturedHeader(
					t,
					headerGCSHash,
					headerGCSChecksumPrefix+"not-base64!!",
				),
			},
			wantErr: core.ErrObjectStoreIntegrity,
		},
		{
			name:     "provider outside the closed domain",
			provider: ProviderUnknown,
			headers:  nil,
			wantErr:  core.ErrObjectStoreContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotPresent, gotErr := providerDownloadCRC32C(
				exchange.CapturedHeaders{Values: tc.headers},
				tc.provider,
			)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || gotPresent {
					t.Fatalf(
						"providerDownloadCRC32C() = (%v, %t, %v), want absent and %v",
						got,
						gotPresent,
						gotErr,
						tc.wantErr,
					)
				}
				return
			}
			if gotErr != nil || gotPresent != tc.wantPresent {
				t.Fatalf(
					"providerDownloadCRC32C() present/error = (%t, %v), want (%t, nil)",
					gotPresent,
					gotErr,
					tc.wantPresent,
				)
			}
			if tc.wantPresent && got != want {
				t.Fatalf(
					"providerDownloadCRC32C() = %v, want the provider checksum %v",
					got,
					want,
				)
			}
		})
	}
}

// TestMultipartFilePartCarriesCallerMediaType proves the declared object media
// type reaches the provider's part header instead of being validated and then
// discarded in favour of a fixed default.
func TestMultipartFilePartCarriesCallerMediaType(t *testing.T) {
	t.Parallel()

	partType := core.HTTPMediaTypeOctetStream()
	header, gotErr := multipartFileHeader(partType)
	if gotErr != nil {
		t.Fatalf("multipartFileHeader() error = %v, want nil", gotErr)
	}
	gotType := header.Get(core.HTTPHeaderContentType().String())
	if gotType != partType.String() {
		t.Fatalf(
			"multipart part content type = %q, want the caller media type %q",
			gotType,
			partType.String(),
		)
	}
	gotDisposition := header.Get(headerContentDisposition)
	if !strings.Contains(gotDisposition, `name="`+cloudflareImagesFormField+`"`) {
		t.Fatalf(
			"multipart part disposition = %q, want the %q form field",
			gotDisposition,
			cloudflareImagesFormField,
		)
	}
}
