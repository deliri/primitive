package objectstore_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

// TestClientAdmissionKeepsHTTPMechanicsInsideObjectstore proves the clean
// package boundary introduced for every real transfer caller. Callers hand
// Objectstore the standard-library capability they own; Objectstore alone
// admits it through Exchange and preserves both stable error identities.
func TestClientAdmissionKeepsHTTPMechanicsInsideObjectstore(t *testing.T) {
	t.Parallel()

	t.Run("positive timeout-free standard clients cross the owner boundary", func(t *testing.T) {
		t.Parallel()

		for _, client := range []*http.Client{
			{},
			{Transport: http.DefaultTransport},
		} {
			got, gotErr := objectstore.NewClient(client)
			if gotErr != nil {
				t.Fatalf("objectstore.NewClient(timeout-free client) error = %v, want nil", gotErr)
			}
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf("admitted objectstore.Client.Validate() error = %v, want nil", gotValidateErr)
			}
			if client.Timeout != 0 {
				t.Fatalf("caller http.Client.Timeout = %v after admission, want unchanged zero", client.Timeout)
			}
		}
	})

	t.Run("negative competing timeout ownership is rejected transactionally", func(t *testing.T) {
		t.Parallel()

		for _, timeout := range []time.Duration{
			time.Duration(-1 << 63), -time.Hour, -1, 1, time.Hour, time.Duration(1<<63 - 1),
		} {
			client := &http.Client{Timeout: timeout}
			got, gotErr := objectstore.NewClient(client)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) ||
				!errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf(
					"objectstore.NewClient(timeout %v) error = %v, want %v/%v",
					timeout, gotErr, core.ErrObjectStoreContract, core.ErrExchangeContract,
				)
			}
			if got != (objectstore.Client{}) {
				t.Fatalf("objectstore.NewClient(timeout %v) = %+v, want zero client", timeout, got)
			}
			if client.Timeout != timeout {
				t.Fatalf("refused caller timeout = %v, want preserved %v", client.Timeout, timeout)
			}
		}
	})

	t.Run("neutral absent client returns no fabricated capability", func(t *testing.T) {
		t.Parallel()

		got, gotErr := objectstore.NewClient(nil)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) ||
			!errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf(
				"objectstore.NewClient(nil) error = %v, want %v/%v",
				gotErr, core.ErrObjectStoreContract, core.ErrExchangeContract,
			)
		}
		if got != (objectstore.Client{}) {
			t.Fatalf("objectstore.NewClient(nil) = %+v, want zero client", got)
		}
		if gotValidateErr := got.Validate(); !errors.Is(gotValidateErr, core.ErrObjectStoreContract) {
			t.Fatalf("zero objectstore.Client.Validate() error = %v, want %v", gotValidateErr, core.ErrObjectStoreContract)
		}
	})
}
