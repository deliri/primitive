package chit

import (
	"bytes"
	"context"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"io"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type chitCanonicalFixture struct {
	name string
	body interface {
		WriteCanonical(io.Writer) error
		MarshalJSON() ([]byte, error)
	}
}

func chitCanonicalFixtures(t testing.TB) []chitCanonicalFixture {
	t.Helper()
	catalog := newCatalogFixture(t, 0x42, 1)
	return []chitCanonicalFixture{
		{name: "chit", body: catalog.payload.Entries[0].Chit.Payload},
		{name: "catalog", body: catalog.payload},
		{name: "query", body: catalog.request},
	}
}

type chitWriteResult struct {
	amount int
	err    error
	calls  int
	bytes  []byte
}

func (w *chitWriteResult) Write(data []byte) (int, error) {
	w.calls++
	w.bytes = bytes.Clone(data)
	return w.amount, w.err
}

type chitNilWriter func([]byte) (int, error)

func (w chitNilWriter) Write(p []byte) (int, error) { return len(p), nil }

type chitNilPointerWriter struct{}

func (*chitNilPointerWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestChitCanonicalWriterBoundaryTable(t *testing.T) {
	t.Parallel()
	for _, fixture := range chitCanonicalFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			canonical, err := fixture.body.MarshalJSON()
			if err != nil || len(canonical) == 0 {
				t.Fatalf("canonical fixture = %d bytes, %v", len(canonical), err)
			}
			cases := []struct {
				name           string
				amount         int
				cause, wantErr error
			}{
				{name: "exact complete write", amount: len(canonical)},
				{name: "zero progress", amount: 0, wantErr: io.ErrShortWrite},
				{name: "one byte prefix", amount: 1, wantErr: io.ErrShortWrite},
				{name: "one byte short", amount: len(canonical) - 1, wantErr: io.ErrShortWrite},
				{name: "negative count", amount: -1, wantErr: io.ErrShortWrite},
				{name: "one byte overreported", amount: len(canonical) + 1, wantErr: io.ErrShortWrite},
				{name: "zero write preserves cancellation", cause: context.Canceled, wantErr: context.Canceled},
				{name: "partial write preserves deadline", amount: 1, cause: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
				{name: "full write still preserves error", amount: len(canonical), cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
				{name: "wrapped writer cause survives", amount: len(canonical) - 1, cause: errors.Join(core.ErrChitConflict, io.ErrClosedPipe), wantErr: io.ErrClosedPipe},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					destination := &chitWriteResult{amount: tc.amount, err: tc.cause}
					gotErr := fixture.body.WriteCanonical(destination)
					if !errors.Is(gotErr, tc.wantErr) || (tc.wantErr != nil && !errors.Is(gotErr, core.ErrChitContract)) || destination.calls != 1 || !bytes.Equal(destination.bytes, canonical) {
						t.Fatalf("WriteCanonical = %v, %d calls, exact bytes %t; want %v, 1, true", gotErr, destination.calls, bytes.Equal(destination.bytes, canonical), tc.wantErr)
					}
				})
			}
			absent := []struct {
				name        string
				destination io.Writer
				wantErr     error
			}{
				{name: "nil interface", wantErr: core.ErrChitContract},
				{name: "typed nil pointer with callable method", destination: (*chitNilPointerWriter)(nil), wantErr: core.ErrChitContract},
				{name: "typed nil function with callable method", destination: chitNilWriter(nil), wantErr: core.ErrChitContract},
			}
			for _, tc := range absent {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					if err := fixture.body.WriteCanonical(tc.destination); !errors.Is(err, tc.wantErr) {
						t.Fatalf("WriteCanonical(absent) = %v, want %v", err, tc.wantErr)
					}
				})
			}
		})
	}
}

func TestChitInvalidCanonicalBodyHasNoWriteEffect(t *testing.T) {
	t.Parallel()
	cases := []chitCanonicalFixture{{name: "unset chit", body: Payload{}}, {name: "unset query", body: QueryPayload{}}, {name: "unset catalog", body: CatalogPayload{}}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := &chitWriteResult{err: io.ErrClosedPipe}
			err := tc.body.WriteCanonical(destination)
			if !errors.Is(err, core.ErrChitContract) || errors.Is(err, io.ErrClosedPipe) || destination.calls != 0 || destination.bytes != nil {
				t.Fatalf("invalid body = %v, calls %d, bytes %v; want contract refusal before write", err, destination.calls, destination.bytes)
			}
		})
	}
}

type chitMutationSigner struct {
	crypto.Signer
	mutate       func()
	mutatePublic func()
	calls        int
}

func (s *chitMutationSigner) Public() crypto.PublicKey {
	if s.mutatePublic != nil {
		s.mutatePublic()
	}
	return s.Signer.Public()
}

func (s *chitMutationSigner) Sign(random io.Reader, digest []byte, options crypto.SignerOpts) ([]byte, error) {
	s.calls++
	s.mutate()
	return s.Signer.Sign(random, digest, options)
}

func TestIssueCatalogOwnsSignedEntriesTable(t *testing.T) {
	t.Parallel()
	fixture := newCatalogFixture(t, 0x44, 1)
	other := newCatalogFixture(t, 0x65, 2)
	cases := []struct {
		name   string
		mutate func(*CatalogEntry)
	}{
		{name: "custody availability", mutate: func(e *CatalogEntry) { e.State = CustodyStateDeleted }},
		{name: "chit identity", mutate: func(e *CatalogEntry) { e.Chit.Payload.Identity = other.payload.Entries[0].Chit.Payload.Identity }},
		{name: "collection identity", mutate: func(e *CatalogEntry) { e.Chit.Payload.Collection = other.payload.Entries[0].Chit.Payload.Collection }},
		{name: "partition", mutate: func(e *CatalogEntry) { e.Chit.Payload.Partition = other.payload.Entries[0].Chit.Payload.Partition }},
		{name: "manifest digest", mutate: func(e *CatalogEntry) {
			e.Chit.Payload.Manifest.Digest = other.payload.Entries[0].Chit.Payload.Manifest.Digest
		}},
		{name: "version", mutate: func(e *CatalogEntry) { e.Chit.Payload.Version = other.payload.Entries[0].Chit.Payload.Version }},
		{name: "acceptance", mutate: func(e *CatalogEntry) { e.Chit.Payload.AcceptedAt = other.payload.Entries[0].Chit.Payload.AcceptedAt }},
		{name: "retention", mutate: func(e *CatalogEntry) { e.Chit.Payload.RetainUntil = other.payload.Entries[0].Chit.Payload.RetainUntil }},
		{name: "signature", mutate: func(e *CatalogEntry) {
			e.Chit.Attestation.Signature = other.payload.Entries[0].Chit.Attestation.Signature
		}},
		{name: "signed digest", mutate: func(e *CatalogEntry) {
			e.Chit.Attestation.BodySHA256 = other.payload.Entries[0].Chit.Attestation.BodySHA256
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, phase := range []struct {
				name                     string
				duringSign, duringPublic bool
			}{
				{name: "after signing"}, {name: "inside Sign", duringSign: true}, {name: "inside Public", duringPublic: true},
			} {
				t.Run(phase.name, func(t *testing.T) {
					t.Parallel()
					input := cloneCatalogPayload(fixture.payload)
					// This is a current stored observation, so the availability mutation is real.
					input.Entries[0].State = CustodyStateStored
					want := cloneCatalogPayload(input)
					signer := &chitMutationSigner{Signer: fixture.private, mutate: func() {
						if phase.duringSign {
							tc.mutate(&input.Entries[0])
						}
					}}
					signer.mutatePublic = func() {
						if phase.duringPublic {
							tc.mutate(&input.Entries[0])
						}
					}
					document, err := IssueCatalog(CatalogIssuance{Signer: signer, Payload: input})
					if !phase.duringSign && !phase.duringPublic {
						tc.mutate(&input.Entries[0])
					}
					if catalogPayloadsEqual(input, want) {
						t.Fatalf("mutated input = %+v, want distinct from %+v", input, want)
					}
					if err != nil || signer.calls != 1 || !catalogPayloadsEqual(document.Payload, want) {
						t.Fatalf("IssueCatalog(phase=%s) = %v, signs %d, original payload %t", phase.name, err, signer.calls, catalogPayloadsEqual(document.Payload, want))
					}
					verified, verifyErr := VerifyCatalog(CatalogVerification{Document: document, Request: fixture.request, TrustedKeys: fixture.trusted})
					if verifyErr != nil || !verifiedCatalogPayloadsEqual(verified, want) {
						t.Fatalf("issued catalog lost its exact authenticated content: %v", verifyErr)
					}
				})
			}
		})
	}
}

func FuzzChitCanonicalWriterResults(f *testing.F) {
	fixtures := chitCanonicalFixtures(f)
	for index, fixture := range fixtures {
		canonical, err := fixture.body.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		for _, n := range []int{-1, 0, 1, len(canonical) - 1, len(canonical), len(canonical) + 1} {
			f.Add(uint8(index), n, false)
			f.Add(uint8(index), n, true)
		}
	}
	f.Fuzz(func(t *testing.T, selector uint8, amount int, failed bool) {
		fixture := fixtures[int(selector)%len(fixtures)]
		canonical, err := fixture.body.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		destination := &chitWriteResult{amount: amount}
		var wantErr error
		if failed {
			destination.err = context.Canceled
			wantErr = context.Canceled
		} else if amount != len(canonical) {
			wantErr = io.ErrShortWrite
		}
		gotErr := fixture.body.WriteCanonical(destination)
		if !errors.Is(gotErr, wantErr) || destination.calls != 1 || !bytes.Equal(destination.bytes, canonical) {
			t.Fatalf("write result %v, calls %d; want %v and exact single write", gotErr, destination.calls, wantErr)
		}
	})
}

func TestChitJSONScalarExtentTable(t *testing.T) {
	t.Parallel()
	fixtures := chitFixturesForFuzz(t)
	cases := []struct {
		name          string
		seed          chitJSONValue
		rejectPadding bool
	}{
		{name: "version", seed: fixtures.version, rejectPadding: true},
		{name: "object count", seed: fixtures.objectCount, rejectPadding: true},
		{name: "entry sequence", seed: fixtures.entrySequence, rejectPadding: true},
		{name: "entry name", seed: fixtures.entryName},
		{name: "chit identity", seed: fixtures.chitID},
		{name: "collection identity", seed: fixtures.collectionID},
		{name: "partition", seed: fixtures.partition},
		{name: "signing domain", seed: fixtures.signingDomain},
		{name: "custody state", seed: fixtures.custodyState},
		{name: "cursor", seed: fixtures.cursor},
		{name: "manifest digest", seed: fixtures.manifestDigest},
		{name: "query commitment", seed: fixtures.queryCommitment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			canonical, err := tc.seed.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			for _, size := range []int{core.JSONDocumentMaximumBytes - 1, core.JSONDocumentMaximumBytes, core.JSONDocumentMaximumBytes + 1} {
				data := append(bytes.Repeat([]byte{' '}, size-len(canonical)), canonical...)
				switch seed := tc.seed.(type) {
				case Version:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case ObjectCount:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case EntrySequence:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case EntryName:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case ChitID:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case CollectionID:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case Partition:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case SigningDomain:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case CustodyState:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case Cursor:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case ManifestDigest:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				case QueryCommitment:
					chitScalarExtent(t, seed, data, size, tc.rejectPadding)
				default:
					t.Fatalf("unhandled scalar type %T", seed)
				}
			}
		})
	}
}

func chitScalarExtent[T interface {
	chitJSONValue
	comparable
}, P interface {
	*T
	UnmarshalJSON([]byte) error
}](t *testing.T, seed T, data []byte, size int, rejectPadding bool) {
	t.Helper()
	got := seed
	decoder := P(&got)
	err := decoder.UnmarshalJSON(data)
	if rejectPadding || size > core.JSONDocumentMaximumBytes {
		if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrChitContract) || got != seed {
			t.Fatalf("%T scalar at %d bytes = %v, %v; want preserved receiver and typed refusal", got, size, got, err)
		}
	} else if err != nil || got != seed {
		t.Fatalf("bounded %T scalar = %v, %v; want %v, nil", got, got, err, seed)
	}
}

func TestChitNumericExtentPrecedesParsingTable(t *testing.T) {
	t.Parallel()
	fixtures := chitFixturesForFuzz(t)
	cases := []struct {
		name string
		seed chitJSONValue
	}{
		{name: "version", seed: fixtures.version},
		{name: "object count", seed: fixtures.objectCount},
		{name: "entry sequence", seed: fixtures.entrySequence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, size := range []int{core.JSONDocumentMaximumBytes - 1, core.JSONDocumentMaximumBytes, core.JSONDocumentMaximumBytes + 1} {
				t.Run(strconv.Itoa(size), func(t *testing.T) {
					t.Parallel()
					// Overflow exposes a typed parser failure only when admission permits parsing.
					overflow := strconv.FormatUint(math.MaxUint64, 10) + "0"
					data := append(bytes.Repeat([]byte{' '}, size-len(overflow)), overflow...)
					switch seed := tc.seed.(type) {
					case Version:
						chitNumericExtentPrecedesParsing(t, seed, data)
					case ObjectCount:
						chitNumericExtentPrecedesParsing(t, seed, data)
					case EntrySequence:
						chitNumericExtentPrecedesParsing(t, seed, data)
					default:
						t.Fatalf("unhandled numeric scalar type %T", seed)
					}
				})
			}
		})
	}
}

func chitNumericExtentPrecedesParsing[T interface {
	chitJSONValue
	comparable
}, P interface {
	*T
	UnmarshalJSON([]byte) error
}](t *testing.T, seed T, data []byte) {
	t.Helper()
	got := seed
	err := P(&got).UnmarshalJSON(data)
	var parserErr *json.SemanticError
	parsed := errors.As(err, &parserErr)
	wantParsed := len(data) <= core.JSONDocumentMaximumBytes
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrChitContract) || got != seed || parsed != wantParsed {
		t.Fatalf("%T at %d bytes: receiver %v, error %v, parser failure %t; want preserved receiver, typed refusal, parser failure %t", got, len(data), got, err, parsed, wantParsed)
	}
}
