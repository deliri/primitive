package payment

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const paymentJSONStressWindow = 2 << 20

type paymentJSONReceiver[T paymentJSONValue] interface {
	*T
	UnmarshalJSON([]byte) error
}

func TestPaymentJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	f := paymentFixturesForFuzz(t)
	paymentJSONExtent[Payload](t, "receipt_payload", f.payload)
	paymentJSONExtent[Document](t, "receipt_document", f.document)
	paymentJSONExtent[QueryPayload](t, "query_payload", f.queryPayload)
	paymentJSONExtent[QueryDocument](t, "query_document", f.queryDocument)
	paymentJSONExtent[CatalogPayload](t, "catalog_payload", f.catalogPayload)
	paymentJSONExtent[CatalogDocument](t, "catalog_document", f.catalogDocument)
}

func paymentJSONExtent[T paymentJSONValue, P paymentJSONReceiver[T]](t *testing.T, name string, seed T) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		canonical, err := seed.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		gap := bytes.Repeat([]byte(" "), paymentJSONStressWindow)
		for _, tc := range []struct {
			name string
			data []byte
			want error
		}{
			{name: "positive canonical", data: canonical},
			{name: "positive whitespace before value crosses many windows", data: append(bytes.Clone(gap), canonical...)},
			{name: "positive whitespace after value crosses many windows", data: append(bytes.Clone(canonical), gap...)},
			{name: "positive whitespace inside object crosses many windows", data: append(append([]byte{'{'}, gap...), canonical[1:]...)},
			{name: "negative second value after many whitespace windows", data: append(append(bytes.Clone(canonical), gap...), []byte("{}")...), want: core.ErrJSONContract},
			{name: "negative truncation after large prefix", data: append(bytes.Clone(gap), canonical[:len(canonical)-1]...), want: core.ErrJSONContract},
			{name: "negative unknown field after large interior gap", data: append(append(append([]byte{'{'}, gap...), []byte(`"future":true,`)...), canonical[1:]...), want: core.ErrJSONContract},
			{name: "neutral whitespace contains no value", data: gap, want: core.ErrJSONContract},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := seed
				err := P(&got).UnmarshalJSON(tc.data)
				if !errors.Is(err, tc.want) {
					t.Fatalf("decode error = %v, want %v", err, tc.want)
				}
				encoded, marshalErr := got.MarshalJSON()
				if marshalErr != nil || !bytes.Equal(encoded, canonical) {
					t.Fatalf("decode changed exact facts: %v", marshalErr)
				}
				if tc.want != nil && !errors.Is(err, core.ErrPaymentContract) {
					t.Fatalf("missing payment identity: %v", err)
				}
			})
		}
	})
}

type paymentBoundaryWriter struct {
	count int
	cause error
	seen  []byte
	calls int
}

func (w *paymentBoundaryWriter) Write(p []byte) (int, error) {
	w.calls++
	w.seen = append(w.seen, p[:w.count]...)
	return w.count, w.cause
}

func TestPaymentCanonicalWriterLayerTriad(t *testing.T) {
	t.Parallel()
	f := paymentFixturesForFuzz(t)
	for _, body := range []struct {
		name           string
		valid, invalid attest.CanonicalBody[SigningDomain]
	}{
		{name: "receipt", valid: f.payload, invalid: Payload{}},
		{name: "query", valid: f.queryPayload, invalid: QueryPayload{}},
		{name: "catalog", valid: f.catalogPayload, invalid: CatalogPayload{}},
	} {
		t.Run(body.name, func(t *testing.T) {
			t.Parallel()
			var canonical bytes.Buffer
			if err := body.valid.WriteCanonical(&canonical); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(canonical.Bytes())
			for _, tc := range []struct {
				name        string
				count       int
				cause, want error
				invalid     bool
			}{
				{name: "positive full write", count: len(want)},
				{name: "negative no progress", want: io.ErrShortWrite},
				{name: "negative partial write", count: len(want) - 1, want: io.ErrShortWrite},
				{name: "negative native error before output", cause: io.ErrClosedPipe, want: io.ErrClosedPipe},
				{name: "negative native error after partial output", count: len(want) / 2, cause: io.ErrClosedPipe, want: io.ErrClosedPipe},
				{name: "negative native error after full output", count: len(want), cause: io.ErrClosedPipe, want: io.ErrClosedPipe},
				{name: "neutral invalid payload emits no bytes", invalid: true, want: core.ErrPaymentContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					sink := &paymentBoundaryWriter{count: tc.count, cause: tc.cause}
					candidate := body.valid
					calls := 1
					if tc.invalid {
						candidate = body.invalid
						calls = 0
					}
					err := candidate.WriteCanonical(sink)
					if !errors.Is(err, tc.want) || sink.calls != calls || !bytes.Equal(sink.seen, want[:tc.count]) {
						t.Fatalf("write = (%v,%d calls,%d bytes), want (%v,%d,%d)", err, sink.calls, len(sink.seen), tc.want, calls, tc.count)
					}
					if tc.want != nil && !errors.Is(err, core.ErrPaymentContract) {
						t.Fatalf("missing payment identity: %v", err)
					}
				})
			}
			for _, tc := range []struct {
				name        string
				destination io.Writer
			}{
				{name: "nil destination"},
				{name: "typed nil destination", destination: (*bytes.Buffer)(nil)},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					defer func() {
						if got := recover(); got != nil {
							t.Fatalf("nil writer panicked: %v", got)
						}
					}()
					if err := body.valid.WriteCanonical(tc.destination); !errors.Is(err, core.ErrPaymentContract) {
						t.Fatalf("nil writer error = %v", err)
					}
				})
			}
			t.Run("every strict prefix preserves native refusal", func(t *testing.T) {
				t.Parallel()
				for count := range len(want) {
					sink := &paymentBoundaryWriter{count: count, cause: io.ErrClosedPipe}
					err := body.valid.WriteCanonical(sink)
					if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrPaymentContract) || sink.calls != 1 || !bytes.Equal(sink.seen, want[:count]) {
						t.Fatalf("prefix %d: error %v bytes %d calls %d", count, err, len(sink.seen), sink.calls)
					}
				}
			})
		})
	}
}

type paymentMutatingSigner struct {
	private  ed25519.PrivateKey
	onPublic func()
	onSign   func()
}

func (s paymentMutatingSigner) Public() crypto.PublicKey {
	if s.onPublic != nil {
		s.onPublic()
	}
	return s.private.Public()
}
func (s paymentMutatingSigner) Sign(random io.Reader, data []byte, opts crypto.SignerOpts) ([]byte, error) {
	if s.onSign != nil {
		s.onSign()
	}
	return s.private.Sign(random, data, opts)
}

func TestPaymentCatalogIssuanceOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                            string
		duringPublic, duringSign, after bool
		entries                         uint16
	}{
		{name: "positive signed page owns exact entries", entries: 2},
		{name: "negative caller mutation after issuance", entries: 2, after: true},
		{name: "negative signer Public mutates caller storage", entries: 2, duringPublic: true},
		{name: "negative signer Sign mutates caller storage", entries: 2, duringSign: true},
		{name: "neutral empty end page remains present and empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x63, Entries: tc.entries})
			want := fixture.document
			want.Payload.Entries = append([]Document{}, fixture.payload.Entries...)
			mutate := func() { fixture.payload.Entries[0].Payload.Amount = mustPaymentAmount(t, 987) }
			signer := paymentMutatingSigner{private: fixture.private}
			if tc.duringPublic {
				signer.onPublic = mutate
			}
			if tc.duringSign {
				signer.onSign = mutate
			}
			got, err := IssueCatalog(CatalogIssuance{Signer: signer, Payload: fixture.payload})
			if tc.after {
				mutate()
			}
			if err != nil || !samePaymentCatalogDocument(got, want) {
				t.Fatalf("issuance lost original signed facts: %v", err)
			}
			proof, err := VerifyCatalog(CatalogVerification{Document: got, Request: fixture.request, TrustedKeys: fixture.trusted})
			if err != nil || !verifiedPaymentCatalogEqual(proof, want.Payload) {
				t.Fatalf("issued document cannot verify its original facts: %v", err)
			}
		})
	}
}
