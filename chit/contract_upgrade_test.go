package chit

import (
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestChitZeroJSONValuesAndNilReceiversTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "entry name", run: chitZeroJSONContract[EntryName]},
		{name: "chit identity", run: chitZeroJSONContract[ChitID]},
		{name: "collection identity", run: chitZeroJSONContract[CollectionID]},
		{name: "partition", run: chitZeroJSONContract[Partition]},
		{name: "version", run: chitZeroJSONContract[Version]},
		{name: "payload", run: chitZeroJSONContract[Payload]},
		{name: "document", run: chitZeroJSONContract[Document]},
		{name: "query payload", run: chitZeroJSONContract[QueryPayload]},
		{name: "query commitment", run: chitZeroJSONContract[QueryCommitment]},
		{name: "query document", run: chitZeroJSONContract[QueryDocument]},
		{name: "custody state", run: chitZeroJSONContract[CustodyState]},
		{name: "cursor", run: chitZeroJSONContract[Cursor]},
		{name: "catalog payload", run: chitZeroJSONContract[CatalogPayload]},
		{name: "catalog document", run: chitZeroJSONContract[CatalogDocument]},
		{name: "signing domain", run: chitZeroJSONContract[SigningDomain]},
		{name: "object count", run: chitZeroJSONContract[ObjectCount]},
		{name: "entry sequence", run: chitZeroJSONContract[EntrySequence]},
		{name: "manifest digest", run: chitZeroJSONContract[ManifestDigest]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { t.Parallel(); tc.run(t) })
	}
}

func chitZeroJSONContract[T chitJSONValue, P interface {
	*T
	UnmarshalJSON([]byte) error
}](t *testing.T) {
	t.Helper()
	var zero T
	encoded, err := zero.MarshalJSON()
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrChitContract) || encoded != nil || zero.Validate() == nil {
		t.Fatalf("zero %T projection = %q, %v; want absent output and typed refusal", zero, encoded, err)
	}
	var receiver P
	err = receiver.UnmarshalJSON([]byte(`null`))
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrChitContract) {
		t.Fatalf("nil %T receiver = %v, want JSON and Chit refusal", receiver, err)
	}
}

func TestChitTaggedUnionExhaustiveTable(t *testing.T) {
	t.Parallel()
	identity := mustChitID(t, 0x38, 1)
	cursor := catalogCursorFixture(t, 0x38)
	// Each byte-domain discriminator is crossed with absent and present data.
	// Explicit legal arms form the oracle; production Validate never selects it.
	type unionCase struct {
		name      string
		value     core.ValidatedJSONMarshaler
		wantValid bool
	}
	cases := make([]unionCase, 0, 3*256*2)
	for raw := range 256 {
		for _, present := range []bool{false, true} {
			selection := Selection{Kind: core.CatalogSelectionKind(raw)}
			position := Position{Kind: core.CatalogPositionKind(raw)}
			continuation := Continuation{State: core.CatalogContinuationState(raw)}
			if present {
				selection.Chit = identity
				position.Cursor = cursor
				continuation.Cursor = cursor
			}
			suffix := strconv.Itoa(raw) + "/present=" + strconv.FormatBool(present)
			cases = append(cases,
				unionCase{name: "selection/" + suffix, value: selection, wantValid: (selection.Kind == core.CatalogSelectionAll && !present) || (selection.Kind == core.CatalogSelectionSpecific && present)},
				unionCase{name: "position/" + suffix, value: position, wantValid: (position.Kind == core.CatalogPositionStart && !present) || (position.Kind == core.CatalogPositionAfter && present)},
				unionCase{name: "continuation/" + suffix, value: continuation, wantValid: (continuation.State == core.CatalogContinuationEnd && !present) || (continuation.State == core.CatalogContinuationMore && present)},
			)
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.value.Validate()
			encoded, encodeErr := tc.value.MarshalJSON()
			if tc.wantValid {
				if err != nil || encodeErr != nil || len(encoded) == 0 {
					t.Fatalf("legal union = %v, %q, %v; want validated nonempty output", err, encoded, encodeErr)
				}
			} else if !errors.Is(err, core.ErrChitContract) || !errors.Is(encodeErr, core.ErrJSONContract) || encoded != nil {
				t.Fatalf("illegal union = %v, %q, %v; want typed refusal and nil output", err, encoded, encodeErr)
			}
		})
	}
}
