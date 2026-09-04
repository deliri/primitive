package github

import (
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestTarArchiveRequestExhaustsValidationEquivalenceClasses(t *testing.T) {
	t.Parallel()

	repository := parsedRepository(t, "owner/repository")
	commit := parsedCommit(t)
	minimum := byteCountFixture(t, 1)
	maximum := byteCountFixture(t, TarArchiveMaximumBytes)
	aboveMaximum := byteCountFixture(t, TarArchiveMaximumBytes+1)
	cases := []struct {
		name    string
		request TarArchiveRequest
		wantErr error
	}{
		{name: "minimum one-byte custody is admitted", request: TarArchiveRequest{Destination: io.Discard, Repository: repository, Commit: commit, MaximumBytes: minimum}},
		{name: "exact Primitive custody ceiling is admitted", request: TarArchiveRequest{Destination: io.Discard, Repository: repository, Commit: commit, MaximumBytes: maximum}},
		{name: "missing destination is rejected", request: TarArchiveRequest{Repository: repository, Commit: commit, MaximumBytes: minimum}, wantErr: core.ErrGitHubContract},
		{name: "missing repository is rejected", request: TarArchiveRequest{Destination: io.Discard, Commit: commit, MaximumBytes: minimum}, wantErr: core.ErrGitHubContract},
		{name: "missing immutable commit is rejected", request: TarArchiveRequest{Destination: io.Discard, Repository: repository, MaximumBytes: minimum}, wantErr: core.ErrGitHubContract},
		{name: "missing byte ceiling is rejected", request: TarArchiveRequest{Destination: io.Discard, Repository: repository, Commit: commit}, wantErr: core.ErrGitHubContract},
		{name: "one byte above Primitive custody ceiling is rejected", request: TarArchiveRequest{Destination: io.Discard, Repository: repository, Commit: commit, MaximumBytes: aboveMaximum}, wantErr: core.ErrGitHubContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.request.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("TarArchiveRequest.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestArchiveTransferStateExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	valid := [...]ArchiveTransferState{ArchiveTransferIncomplete, ArchiveTransferComplete}
	for _, state := range valid {
		if err := state.Validate(); err != nil || !state.IsValid() || state.String() == "" || state.String() == core.UnknownEnumDiagnostic {
			t.Fatalf("ArchiveTransferState(%d) = (Validate:%v IsValid:%t String:%q), want admitted diagnostic", state, err, state.IsValid(), state.String())
		}
		var offWire core.OffWireEnum = state
		offWire.OffWireEnum()
	}
	invalid := [...]ArchiveTransferState{
		ArchiveTransferUnknown,
		ArchiveTransferComplete + 1,
		ArchiveTransferComplete + 2,
		ArchiveTransferState(math.MaxUint8),
	}
	for _, state := range invalid {
		if !errors.Is(state.Validate(), core.ErrGitHubResponse) || state.IsValid() || state.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("ArchiveTransferState(%d) = (Validate:%v IsValid:%t String:%q), want rejected unknown", state, state.Validate(), state.IsValid(), state.String())
		}
	}
}

func TestGitHubArchiveLocationExhaustsHeaderAndTransportBoundaries(t *testing.T) {
	t.Parallel()

	location, err := core.ParseHTTPHeaderName(headerLocation)
	if err != nil {
		t.Fatalf("core.ParseHTTPHeaderName(Location) error = %v, want nil", err)
	}
	other, err := core.ParseHTTPHeaderName("X-Other")
	if err != nil {
		t.Fatalf("core.ParseHTTPHeaderName(X-Other) error = %v, want nil", err)
	}
	validHTTPS := "https://objects.example.test/temporary/archive?token=opaque"
	validLoopback := "http://127.0.0.1:8080/archive"
	cases := []struct {
		name    string
		headers exchange.CapturedHeaders
		want    string
		wantErr error
	}{
		{name: "absolute HTTPS temporary capability is admitted", headers: capturedHeaderFixture(t, location, validHTTPS), want: validHTTPS},
		{name: "loopback HTTP provider fixture is admitted", headers: capturedHeaderFixture(t, location, validLoopback), want: validLoopback},
		{name: "missing Location is rejected", wantErr: core.ErrGitHubBinding},
		{name: "wrong captured field is rejected", headers: capturedHeaderFixture(t, other, validHTTPS), wantErr: core.ErrGitHubBinding},
		{name: "duplicate captured fields are rejected", headers: exchange.CapturedHeaders{Values: []exchange.Header{capturedHeaderFixture(t, location, validHTTPS).Values[0], capturedHeaderFixture(t, other, validHTTPS).Values[0]}}, wantErr: core.ErrGitHubBinding},
		{name: "duplicate Location values are rejected", headers: capturedHeaderValuesFixture(t, location, validHTTPS, validHTTPS), wantErr: core.ErrGitHubBinding},
		{name: "foreign plain HTTP location is rejected", headers: capturedHeaderFixture(t, location, "http://objects.example.test/archive"), wantErr: core.ErrGitHubBinding},
		{name: "credential-bearing location is rejected", headers: capturedHeaderFixture(t, location, "https://user:secret@objects.example.test/archive"), wantErr: core.ErrGitHubBinding},
		{name: "fragment-bearing location is rejected", headers: capturedHeaderFixture(t, location, "https://objects.example.test/archive#fragment"), wantErr: core.ErrGitHubBinding},
		{name: "relative location is rejected", headers: capturedHeaderFixture(t, location, "/archive"), wantErr: core.ErrGitHubBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := archiveLocation(tc.headers, location)
			if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.want {
				t.Fatalf("archiveLocation() = (%q, %v), want (%q, %v)", got.String(), gotErr, tc.want, tc.wantErr)
			}
		})
	}
}

func capturedHeaderFixture(t testing.TB, name core.HTTPHeaderName, value string) exchange.CapturedHeaders {
	t.Helper()
	return capturedHeaderValuesFixture(t, name, value)
}

func capturedHeaderValuesFixture(t testing.TB, name core.HTTPHeaderName, values ...string) exchange.CapturedHeaders {
	t.Helper()
	typed := make([]exchange.HeaderValue, len(values))
	for index, value := range values {
		parsed, err := exchange.NewHeaderValue(value)
		if err != nil {
			t.Fatalf("exchange.NewHeaderValue(%q) error = %v, want nil", value, err)
		}
		typed[index] = parsed
	}
	return exchange.CapturedHeaders{Values: []exchange.Header{{Name: name, Values: typed}}}
}
