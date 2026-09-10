package retrieval

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/core"
)

// Historical extents identify removed regressions, never production quotas.
const (
	retrievalFormerRequestPayloadBytes  = 32 << 10
	retrievalFormerRequestDocumentBytes = 64 << 10
	retrievalFormerGrantPayloadBytes    = 128 << 10
	retrievalFormerGrantDocumentBytes   = 256 << 10
	retrievalWhitespaceProbeBytes       = 1<<20 + 1
)

func TestRetrievalJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	requestFixture := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: StartAll()})
	request := issueRetrievalRequestFixture(t, requestFixture)
	grantFixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{1}})
	projection, err := IssueGrant(GrantIssuance{Signer: grantFixture.private, Capability: grantFixture.capability, Payload: grantFixture.grantPayload, Entry: grantFixture.membership, Chit: grantFixture.chit, Request: grantFixture.request})
	if err != nil {
		t.Fatal(err)
	}
	for _, door := range []struct {
		name    string
		former  int
		marshal func() ([]byte, error)
		decode  func([]byte, bool) (bool, error)
	}{
		{name: "request_payload", former: retrievalFormerRequestPayloadBytes, marshal: request.Payload.MarshalJSON, decode: func(data []byte, accept bool) (bool, error) {
			got := request.Payload
			if accept {
				got = RequestPayload{}
			}
			err := got.UnmarshalJSON(data)
			return got == request.Payload, err
		}},
		{name: "request_document", former: retrievalFormerRequestDocumentBytes, marshal: request.MarshalJSON, decode: func(data []byte, accept bool) (bool, error) {
			got := request
			if accept {
				got = RequestDocument{}
			}
			err := got.UnmarshalJSON(data)
			return got == request, err
		}},
		{name: "grant_payload", former: retrievalFormerGrantPayloadBytes, marshal: projection.Payload.MarshalJSON, decode: func(data []byte, accept bool) (bool, error) {
			got := projection.Payload
			if accept {
				got = GrantPayload{}
			}
			err := got.UnmarshalJSON(data)
			return got == projection.Payload, err
		}},
		{name: "grant_document", former: retrievalFormerGrantDocumentBytes, marshal: projection.MarshalJSON, decode: func(data []byte, accept bool) (bool, error) {
			got := grantFixture.document
			if accept {
				got = GrantDocument{}
			}
			err := got.UnmarshalJSON(data)
			return sameGrantDocument(got, grantFixture.document), err
		}},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := door.marshal()
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name string
				data []byte
				want error
			}{
				{name: "positive_one_past_removed_ceiling", data: retrievalPadJSON(encoded, door.former+1)},
				{name: "positive_large_whitespace_inside_object", data: append(append([]byte{'{'}, bytes.Repeat([]byte{' '}, retrievalWhitespaceProbeBytes)...), encoded[1:]...)},
				{name: "negative_second_document_after_large_whitespace", data: append(retrievalPadJSON(encoded, retrievalWhitespaceProbeBytes), '{', '}'), want: core.ErrJSONContract},
				{name: "negative_truncation_after_large_whitespace", data: append(append([]byte{'{'}, bytes.Repeat([]byte{' '}, retrievalWhitespaceProbeBytes)...), encoded[1:len(encoded)-1]...), want: core.ErrJSONContract},
				{name: "neutral_empty_input_preserves_receiver", want: core.ErrJSONContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					exact, err := door.decode(tc.data, tc.want == nil)
					if !exact || !errors.Is(err, tc.want) || (tc.want != nil && !errors.Is(err, core.ErrRetrievalContract)) {
						t.Fatalf("decode exact=%t error=%v, want exact receiver and %v", exact, err, tc.want)
					}
				})
			}
		})
	}
}

type retrievalBoundaryWriter struct {
	output bytes.Buffer
	count  int
	err    error
	calls  int
}

func (w *retrievalBoundaryWriter) Write(data []byte) (int, error) {
	w.calls++
	count := min(max(w.count, 0), len(data))
	_, _ = w.output.Write(data[:count]) // bytes.Buffer.Write always succeeds.
	return w.count, w.err
}

func TestRetrievalCanonicalWriterLayerTriad(t *testing.T) {
	t.Parallel()
	request := newRetrievalRequestFixture(t, retrievalRequestFixtureRequest{Selection: StartAll()}).payload
	grant := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{1}}).grantPayload
	for _, door := range []struct {
		name    string
		marshal func() ([]byte, error)
		write   func(io.Writer) error
		invalid func(io.Writer) error
	}{
		{name: "request", marshal: request.MarshalJSON, write: request.WriteCanonical, invalid: (RequestPayload{}).WriteCanonical},
		{name: "grant", marshal: grant.MarshalJSON, write: grant.WriteCanonical, invalid: (GrantPayload{}).WriteCanonical},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			canonical, err := door.marshal()
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name    string
				count   int
				cause   error
				want    error
				invalid bool
				calls   int
			}{
				{name: "positive_exact_bytes_once", count: len(canonical), calls: 1},
				{name: "negative_error_before_any_byte", cause: io.ErrClosedPipe, want: io.ErrClosedPipe, calls: 1},
				{name: "negative_error_after_prefix", count: len(canonical) / 2, cause: io.ErrUnexpectedEOF, want: io.ErrUnexpectedEOF, calls: 1},
				{name: "negative_full_count_does_not_erase_error", count: len(canonical), cause: io.ErrClosedPipe, want: io.ErrClosedPipe, calls: 1},
				{name: "negative_impossible_negative_count", count: -1, want: io.ErrShortWrite, calls: 1},
				{name: "negative_impossible_excess_count", count: len(canonical) + 1, want: io.ErrShortWrite, calls: 1},
				{name: "neutral_invalid_body_emits_nothing", invalid: true, want: core.ErrRetrievalContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					writer := retrievalBoundaryWriter{count: tc.count, err: tc.cause}
					write := door.write
					if tc.invalid {
						write = door.invalid
					}
					err := write(&writer)
					count := min(max(tc.count, 0), len(canonical))
					if tc.invalid {
						count = 0
					}
					if !errors.Is(err, tc.want) || (tc.want != nil && !errors.Is(err, core.ErrRetrievalContract)) || writer.calls != tc.calls || !bytes.Equal(writer.output.Bytes(), canonical[:count]) {
						t.Fatalf("write error=%v calls=%d bytes=%d, want %v calls=%d exact prefix=%d", err, writer.calls, writer.output.Len(), tc.want, tc.calls, count)
					}
				})
			}
			t.Run("negative_every_short_prefix", func(t *testing.T) {
				t.Parallel()
				for count := range len(canonical) {
					writer := retrievalBoundaryWriter{count: count}
					err := door.write(&writer)
					if !errors.Is(err, io.ErrShortWrite) || !errors.Is(err, core.ErrRetrievalContract) || writer.calls != 1 || !bytes.Equal(writer.output.Bytes(), canonical[:count]) {
						t.Fatalf("prefix=%d error=%v calls=%d bytes=%d, want exact short write once", count, err, writer.calls, writer.output.Len())
					}
				}
			})
			for _, tc := range []struct {
				name        string
				destination io.Writer
			}{
				{name: "negative_nil_interface"},
				{name: "negative_typed_nil", destination: (*bytes.Buffer)(nil)},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					defer func() {
						if p := recover(); p != nil {
							t.Errorf("WriteCanonical panicked=%v, want typed refusal", p)
						}
					}()
					if err := door.write(tc.destination); !errors.Is(err, core.ErrRetrievalContract) {
						t.Fatalf("WriteCanonical error=%v, want %v", err, core.ErrRetrievalContract)
					}
				})
			}
		})
	}
}

func TestDownloadCallDestinationLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: []byte{1}})
	for _, tc := range []struct {
		name        string
		destination io.Writer
		want        error
	}{
		{name: "positive_discard_is_real_destination", destination: io.Discard},
		{name: "negative_typed_nil", destination: (*bytes.Buffer)(nil), want: core.ErrRetrievalContract},
		{name: "neutral_absent_destination_produces_no_call", want: core.ErrRetrievalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := DownloadCallRequest{Destination: tc.destination}
			validation := request.Validate()
			call, err := fixture.grant.DownloadCall(request)
			if !errors.Is(validation, tc.want) || !errors.Is(err, tc.want) {
				t.Fatalf("validate=%v projection=%v, want %v", validation, err, tc.want)
			}
			if tc.want != nil {
				if !downloadCallIsZero(call) {
					t.Fatalf("rejected projection=%v, want zero", call)
				}
			} else if call.Validate() != nil || call.Destination != tc.destination {
				t.Fatalf("projection=%v, want exact destination", call)
			}
		})
	}
}

func TestSelectionJSONLayerTriad(t *testing.T) {
	t.Parallel()
	sequence, err := chit.NewEntrySequence(1)
	if err != nil {
		t.Fatal(err)
	}
	specific, err := Specific(sequence)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		want    Selection
		wantErr error
	}{
		{name: "positive_specific_excludes_traversal", data: mustSelectionJSON(t, specific), want: specific},
		{name: "negative_missing_position", data: retrievalMissingSelectionPosition(t), want: StartAll(), wantErr: core.ErrJSONContract},
		{name: "negative_unknown_member", data: bytes.Replace(mustSelectionJSON(t, StartAll()), []byte("{"), []byte(`{"foreign":1,`), 1), want: StartAll(), wantErr: core.ErrJSONContract},
		{name: "negative_duplicate_member", data: append(append([]byte("{"), mustSelectionJSON(t, StartAll())[1:len(mustSelectionJSON(t, StartAll()))-1]...), append([]byte(","), mustSelectionJSON(t, StartAll())[1:]...)...), want: StartAll(), wantErr: core.ErrJSONContract},
		{name: "neutral_null_preserves_receiver", data: []byte("null"), want: StartAll(), wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := StartAll()
			err := got.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) || got != tc.want || (tc.wantErr != nil && !errors.Is(err, core.ErrRetrievalContract)) {
				t.Fatalf("selection=%v error=%v, want %v/%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
func mustSelectionJSON(t testing.TB, value Selection) []byte {
	t.Helper()
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func FuzzSelectionExternalDecoder(f *testing.F) {
	specific, err := Specific(grantSequenceForFuzz(f, 1))
	if err != nil {
		f.Fatal(err)
	}
	continued, err := ContinueAll(grantSequenceForFuzz(f, 1))
	if err != nil {
		f.Fatal(err)
	}
	fuzzRetrievalExternalDoor(f, retrievalExternalDoor[Selection]{
		Seed: StartAll(), Mutations: []Selection{specific, continued},
		Marshal:   func(value Selection) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *Selection, data []byte) error { return value.UnmarshalJSON(data) },
		Validate:  func(value Selection) error { return value.Validate() },
	})
}
func grantSequenceForFuzz(t testing.TB, value uint64) chit.EntrySequence {
	t.Helper()
	sequence, err := chit.NewEntrySequence(value)
	if err != nil {
		t.Fatal(err)
	}
	return sequence
}

func retrievalMissingSelectionPosition(t testing.TB) []byte {
	t.Helper()
	encoded, err := core.MarshalCanonicalJSONDocument(struct {
		Kind core.CatalogSelectionKind `json:"kind"`
	}{Kind: core.CatalogSelectionAll})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestRetrievalNilReceiverBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decode func([]byte) error
	}{
		{name: "selection", decode: (*Selection)(nil).UnmarshalJSON},
		{name: "request_payload", decode: (*RequestPayload)(nil).UnmarshalJSON},
		{name: "request_document", decode: (*RequestDocument)(nil).UnmarshalJSON},
		{name: "request_commitment", decode: (*RequestCommitment)(nil).UnmarshalJSON},
		{name: "grant_payload", decode: (*GrantPayload)(nil).UnmarshalJSON},
		{name: "grant_document", decode: (*GrantDocument)(nil).UnmarshalJSON},
		{name: "signing_domain", decode: (*SigningDomain)(nil).UnmarshalJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.decode(nil); !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrRetrievalContract) {
				t.Fatalf("nil receiver=%v, want JSON and Retrieval refusal", err)
			}
		})
	}
}
