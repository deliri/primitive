package chit

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCatalogRefusalPrecedesSignerCallbacksTable(t *testing.T) {
	t.Parallel()
	fixture := newCatalogFixture(t, 0x42, 1)
	foreign := newCatalogFixture(t, 0x73, 2)
	validEntries := catalogHistoryEntries(t, fixture, core.CatalogPageMaximumEntries+1)
	cases := []struct {
		name    string
		count   int
		mutate  func(*CatalogPayload)
		wantErr error
	}{
		{name: "nil entries are absence", mutate: func(p *CatalogPayload) { p.Entries = nil }, wantErr: core.ErrChitContract},
		{name: "valid empty terminal page", count: 0},
		{name: "minimum nonempty page", count: 1},
		{name: "one below page ceiling", count: core.CatalogPageMaximumEntries - 1},
		{name: "exact page ceiling", count: core.CatalogPageMaximumEntries},
		{name: "valid entries one above ceiling", count: core.CatalogPageMaximumEntries + 1, wantErr: core.ErrChitContract},
		{name: "entry has foreign principal", count: 1, mutate: func(p *CatalogPayload) { p.Entries[0].Chit.Payload.Scope.Principal = foreign.payload.Scope.Principal }, wantErr: core.ErrChitConflict},
		{name: "entry has foreign offering", count: 1, mutate: func(p *CatalogPayload) { p.Entries[0].Chit.Payload.Scope.Offering = foreign.payload.Scope.Offering }, wantErr: core.ErrChitConflict},
		{name: "watermark has foreign scope", count: 1, mutate: func(p *CatalogPayload) { p.Watermark = foreign.payload.Watermark }, wantErr: core.ErrChitConflict},
		{name: "adjacent identities reversed", count: 2, mutate: func(p *CatalogPayload) { p.Entries[0], p.Entries[1] = p.Entries[1], p.Entries[0] }, wantErr: core.ErrChitConflict},
		{name: "duplicate final identity", count: 2, mutate: func(p *CatalogPayload) { p.Entries[1] = p.Entries[0] }, wantErr: core.ErrChitConflict},
		{name: "empty page claims continuation", count: 0, mutate: func(p *CatalogPayload) {
			var err error
			p.Continuation, err = More(catalogCursorFixture(t, 0x42))
			if err != nil {
				t.Fatal(err)
			}
		}, wantErr: core.ErrChitConflict},
		{name: "tail commitment belongs to preceding entry", count: 2, mutate: func(p *CatalogPayload) {
			cursor, err := CursorFor(p.Entries[0].Chit.Payload.Identity)
			if err != nil {
				t.Fatal(err)
			}
			p.Continuation, err = More(cursor)
			if err != nil {
				t.Fatal(err)
			}
		}, wantErr: core.ErrChitConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := fixture.payload
			payload.Entries = make([]CatalogEntry, tc.count)
			copy(payload.Entries, validEntries[:tc.count])
			payload.Continuation = End()
			before := cloneCatalogPayload(payload)
			if tc.mutate != nil {
				tc.mutate(&payload)
				if catalogPayloadsEqual(payload, before) {
					t.Fatalf("mutated page = %+v, want distinct from %+v", payload, before)
				}
			}
			publicCalls := 0
			signer := &chitMutationSigner{Signer: fixture.private, mutate: func() {}, mutatePublic: func() { publicCalls++ }}
			document, err := IssueCatalog(CatalogIssuance{Signer: signer, Payload: payload})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || signer.calls != 0 || publicCalls != 0 || !catalogDocumentsEqual(document, CatalogDocument{}) {
					t.Fatalf("refused issuance = %v, Sign %d, Public %d, document %+v; want %v and no callbacks/output", err, signer.calls, publicCalls, document, tc.wantErr)
				}
				return
			}
			if err != nil || signer.calls != 1 || publicCalls == 0 || !catalogPayloadsEqual(document.Payload, payload) {
				t.Fatalf("valid issuance = %v, Sign %d, Public %d; want exact signed page", err, signer.calls, publicCalls)
			}
		})
	}
}
