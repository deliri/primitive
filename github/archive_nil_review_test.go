package github

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestArchiveTypedNilDestinationHasNoEffectLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		destination io.Writer
	}{
		{name: "missing destination"},
		{name: "typed nil bytes buffer", destination: (*bytes.Buffer)(nil)},
		{name: "typed nil native file", destination: (*os.File)(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(http.StatusBadRequest) }))
			defer server.Close()
			client := clientFixture(t, server.URL)
			request := TarArchiveRequest{Destination: tc.destination, Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t)}
			validationErr := request.Validate()
			got, err := client.ReadTarArchive(t.Context(), request)
			if !errors.Is(validationErr, core.ErrGitHubContract) || !errors.Is(err, core.ErrGitHubContract) || got != (TarArchiveObservation{}) || calls.Load() != 0 {
				t.Fatalf("validation/effect = %v/%v, observation=%+v, requests=%d; want contract errors, zero observation and no HTTP", validationErr, err, got, calls.Load())
			}
		})
	}
}
