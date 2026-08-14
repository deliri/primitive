package objectstore_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
)

// TestObjectstoreClientAuthorityLayerTriad proves Objectstore cannot
// become a second net/http owner. Customized transport is admitted by Exchange
// first; this boundary accepts only that typed capability.
func TestObjectstoreClientAuthorityLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive validated exchange capability crosses the owner boundary", func(t *testing.T) {
		t.Parallel()

		client, err := exchange.NewStandardClient()
		if err != nil {
			t.Fatalf("exchange.NewStandardClient() error = %v, want nil", err)
		}
		got, gotErr := objectstore.NewClient(client)
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("objectstore.NewClient(valid exchange client) = (%v, %v), want valid capability and nil", got, gotErr)
		}
	})

	t.Run("negative zero exchange authority is rejected without a fabricated capability", func(t *testing.T) {
		t.Parallel()

		got, gotErr := objectstore.NewClient(exchange.Client{})
		if !errors.Is(gotErr, core.ErrObjectStoreContract) ||
			!errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf(
				"objectstore.NewClient(zero exchange client) error = %v, want %v/%v",
				gotErr, core.ErrObjectStoreContract, core.ErrExchangeContract,
			)
		}
		if got != (objectstore.Client{}) {
			t.Fatalf("objectstore.NewClient(zero exchange client) = %+v, want zero client", got)
		}
		if gotValidateErr := got.Validate(); !errors.Is(gotValidateErr, core.ErrObjectStoreContract) {
			t.Fatalf("zero objectstore.Client.Validate() error = %v, want %v", gotValidateErr, core.ErrObjectStoreContract)
		}
	})

	t.Run("neutral absent transport customization uses the standard owner without noise", func(t *testing.T) {
		t.Parallel()

		client, err := objectstore.NewStandardClient()
		if err != nil || client.Validate() != nil {
			t.Fatalf("objectstore.NewStandardClient() = (%v, %v), want valid capability and nil", client, err)
		}
	})
}
