package distribution_test

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"io"
	"strconv"
	"testing"
)

type canonicalBoundaryWriter struct {
	err   error
	got   []byte
	count int
	calls int
}

func (w *canonicalBoundaryWriter) Write(p []byte) (int, error) {
	w.calls++
	w.got = bytes.Clone(p)
	return w.count, w.err
}

func TestCanonicalWriterLayerTriad(t *testing.T) {
	t.Parallel()
	publication := newPublicationExchangeFixture(t)
	update := newUpdateExchangeFixture(t)
	upgrade := newUpgradeExchangeFixture(t)
	completion := completedPublicationDocument(t, publication, 0)
	bodies := []struct {
		body    attest.CanonicalBody[distribution.SigningDomain]
		marshal func() ([]byte, error)
		name    string
	}{
		{name: "publication request", body: publication.request, marshal: publication.request.MarshalJSON},
		{name: "publication grant", body: publication.grantPayload, marshal: publication.grantPayload.MarshalJSON},
		{name: "publication completion", body: completion.Payload, marshal: completion.Payload.MarshalJSON},
		{name: "update request", body: update.request, marshal: update.request.MarshalJSON},
		{name: "update response", body: update.responseDoc.Payload, marshal: update.responseDoc.Payload.MarshalJSON},
		{name: "upgrade request", body: upgrade.request, marshal: upgrade.request.MarshalJSON},
		{name: "upgrade grant", body: upgrade.grantDoc.Payload, marshal: upgrade.grantDoc.Payload.MarshalJSON},
	}
	for _, b := range bodies {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			wire, err := b.marshal()
			if err != nil || len(wire) == 0 {
				t.Fatalf("MarshalJSON()=(%d,%v), want nonempty canonical bytes", len(wire), err)
			}
			cases := []struct {
				err     error
				wantErr error
				name    string
				n       int
			}{
				{name: "complete write", n: len(wire), err: nil, wantErr: nil},
				{name: "zero accepted bytes", n: 0, err: nil, wantErr: io.ErrShortWrite},
				{name: "last byte missing", n: len(wire) - 1, err: nil, wantErr: io.ErrShortWrite},
				{name: "negative writer count", n: -1, err: nil, wantErr: io.ErrShortWrite},
				{name: "impossible extra byte", n: len(wire) + 1, err: nil, wantErr: io.ErrShortWrite},
				{name: "refusal before output", n: 0, err: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
				{name: "refusal after prefix", n: len(wire) - 1, err: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
				{name: "refusal despite full count", n: len(wire), err: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					w := &canonicalBoundaryWriter{count: tc.n, err: tc.err}
					got := b.body.WriteCanonical(w)
					if !errors.Is(got, tc.wantErr) || w.calls != 1 || !bytes.Equal(w.got, wire) {
						t.Fatalf("WriteCanonical()=(%v,%d,%d bytes), want (%v,1,%d exact bytes)", got, w.calls, len(w.got), tc.wantErr, len(wire))
					}
					if tc.wantErr != nil && !errors.Is(got, core.ErrDistributionContract) {
						t.Fatalf("WriteCanonical() error=%v, want %v", got, core.ErrDistributionContract)
					}
				})
			}
			nils := []struct {
				writer io.Writer
				name   string
			}{{name: "absent destination"}, {name: "typed nil destination", writer: (*bytes.Buffer)(nil)}}
			for _, tc := range nils {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					defer func() {
						if got := recover(); got != nil {
							t.Fatalf("WriteCanonical() panic=%v, want typed refusal", got)
						}
					}()
					got := b.body.WriteCanonical(tc.writer)
					if !errors.Is(got, core.ErrDistributionContract) {
						t.Fatalf("WriteCanonical() error=%v, want %v", got, core.ErrDistributionContract)
					}
				})
			}
		})
	}
}

func TestPublicationSourceRejectsEveryAbsentReader(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reader  io.Reader
		wantErr error
		name    string
	}{
		{name: "absent reader", wantErr: core.ErrDistributionContract},
		{name: "typed nil reader", reader: (*bytes.Reader)(nil), wantErr: core.ErrDistributionContract},
		{name: "empty stream remains valid intent", reader: bytes.NewReader(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := (distribution.PublicationSource{Reader: tc.reader}).Validate()
			if !errors.Is(got, tc.wantErr) {
				t.Fatalf("PublicationSource.Validate()=%v, want %v", got, tc.wantErr)
			}
		})
	}
}

func TestCompletionVerifierBindsEveryEvidenceCapabilityToItsGrant(t *testing.T) {
	t.Parallel()
	f := newPublicationExchangeFixture(t)
	doc := completedPublicationDocument(t, f, 0)
	for index := range f.grantPayload.Commitments {
		t.Run("publication slot "+strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			payload := f.grantPayload
			// The independently signed grant changes only one destination commitment.
			// Content, request, authorization, lifetime and completion remain identical.
			foreign, _ := uploadCapabilityProjection(t, index+len(f.grantPayload.Commitments))
			commitment, err := foreign.Commitment()
			if err != nil || commitment == payload.Commitments[index] {
				t.Fatalf("foreign capability=(%v,%v), want distinct valid commitment", commitment, err)
			}
			cases := []struct {
				wantErr error
				name    string
				foreign bool
			}{
				{name: "same signed upload"},
				{name: "same bytes under foreign signed destination", foreign: true, wantErr: core.ErrDistributionBinding},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					input := publicationCompletionExpectation(f, doc)
					if tc.foreign {
						input.Grant.Commitments[index] = commitment
					}
					envelope, err := attest.Sign(attest.SignRequest[distribution.SigningDomain]{Body: input.Grant, Signer: f.authorityKey})
					if err != nil {
						t.Fatalf("Sign(grant)=%v, want nil", err)
					}
					input.GrantAttestation = envelope
					got, gotErr := distribution.VerifyPublicationCompletion(input)
					if !errors.Is(gotErr, tc.wantErr) {
						t.Fatalf("VerifyPublicationCompletion()=%v, want %v", gotErr, tc.wantErr)
					}
					if tc.wantErr != nil {
						if got != (distribution.VerifiedPublicationCompletion{}) {
							t.Fatalf("refused completion=%v, want zero proof", got)
						}
					} else {
						payload, payloadErr := got.Payload()
						if payloadErr != nil || payload != doc.Payload {
							t.Fatalf("completion=(%v,%v), want exact signed payload", payload, payloadErr)
						}
					}
				})
			}
		})
	}
}
