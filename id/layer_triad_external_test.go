package id_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

func TestIdentityConstructionAndParsingLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive one request produces canonical UUIDv7 and ULID identities", func(t *testing.T) {
		t.Parallel()
		request := testRequest(t, 1, testEntropy())
		uuid, uuidErr := id.NewUUIDv7(request)
		ulid, ulidErr := id.NewULID(request)
		parsedUUID, parseUUIDErr := id.ParseUUIDv7(uuid.String())
		parsedULID, parseULIDErr := id.ParseULID(ulid.String())
		if uuidErr != nil || ulidErr != nil || parseUUIDErr != nil || parseULIDErr != nil ||
			parsedUUID != uuid || parsedULID != ulid {
			t.Fatalf("identity round trips = (uuid %v/%v/%v, ulid %v/%v/%v), want exact values and nil", uuid, parsedUUID, errors.Join(uuidErr, parseUUIDErr), ulid, parsedULID, errors.Join(ulidErr, parseULIDErr))
		}
	})

	t.Run("negative noncanonical identities return typed zero values", func(t *testing.T) {
		t.Parallel()
		uuid, uuidErr := id.ParseUUIDv7("00000000-0001-6000-8000-000000000001")
		ulid, ulidErr := id.ParseULID("0000000000000000000000000I")
		if !errors.Is(uuidErr, core.ErrIDContract) || !errors.Is(ulidErr, core.ErrIDContract) ||
			uuid != (id.UUIDv7{}) || ulid != (id.ULID{}) {
			t.Fatalf("noncanonical parses = (%v, %v, %v, %v), want typed errors and zero identities", uuid, uuidErr, ulid, ulidErr)
		}
	})

	t.Run("neutral zero identities project no plausible text or JSON", func(t *testing.T) {
		t.Parallel()
		var uuid id.UUIDv7
		var ulid id.ULID
		uuidJSON, uuidErr := uuid.MarshalJSON()
		ulidJSON, ulidErr := ulid.MarshalJSON()
		if uuid.String() != "" || ulid.String() != "" || uuidJSON != nil || ulidJSON != nil ||
			!errors.Is(uuidErr, core.ErrIDContract) || !errors.Is(ulidErr, core.ErrIDContract) {
			t.Fatalf("zero identity projections = (%q, %q, %v/%v, %v/%v), want empty text, nil JSON, typed errors", uuid.String(), ulid.String(), uuidJSON, uuidErr, ulidJSON, ulidErr)
		}
	})
}
