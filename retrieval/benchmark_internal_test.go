package retrieval

import (
	"bytes"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func BenchmarkRetrievalDocuments(b *testing.B) {
	b.ReportAllocs()
	fixture := newDownloadCallFixture(b, downloadCallFixtureRequest{Payload: []byte{1}})
	request := issueRetrievalRequestFixture(b, newRetrievalRequestFixture(b, retrievalRequestFixtureRequest{Selection: StartAll()}))
	requestBytes, err := request.MarshalJSON()
	if err != nil {
		b.Fatalf("benchmark operation error=%v, want nil", err)
	}
	grant, err := IssueGrant(GrantIssuance{Signer: fixture.private, Capability: fixture.capability, Payload: fixture.grantPayload, Entry: fixture.membership, Chit: fixture.chit, Request: fixture.request})
	if err != nil {
		b.Fatalf("benchmark operation error=%v, want nil", err)
	}
	grantBytes, err := grant.MarshalJSON()
	if err != nil {
		b.Fatalf("benchmark operation error=%v, want nil", err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "request", data: requestBytes},
		{name: "request_whitespace_32KiB", data: retrievalPadJSON(requestBytes, 32<<10)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var got RequestDocument
			b.ReportAllocs()
			for b.Loop() {
				if err := got.UnmarshalJSON(tc.data); err != nil {
					b.Fatalf("benchmark operation error=%v, want nil", err)
				}
			}
			if got != request {
				b.Fatalf("decoded request=%v, want signed fixture=%v", got, request)
			}
		})
	}
	b.Run("grant", func(b *testing.B) {
		var got GrantDocument
		b.ReportAllocs()
		for b.Loop() {
			if err := got.UnmarshalJSON(grantBytes); err != nil {
				b.Fatalf("benchmark operation error=%v, want nil", err)
			}
		}
		if !sameGrantDocument(got, fixture.document) {
			b.Fatalf("decoded grant=%v, want signed fixture=%v", got, fixture.document)
		}
	})
	for _, tc := range []struct {
		name  string
		write func(io.Writer) error
		want  []byte
	}{
		{name: "request_canonical", write: request.Payload.WriteCanonical},
		{name: "grant_canonical", write: grant.Payload.WriteCanonical},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var destination bytes.Buffer
			if err := tc.write(&destination); err != nil {
				b.Fatalf("benchmark operation error=%v, want nil", err)
			}
			want := bytes.Clone(destination.Bytes())
			b.ReportAllocs()
			for b.Loop() {
				destination.Reset()
				if err := tc.write(&destination); err != nil {
					b.Fatalf("benchmark operation error=%v, want nil", err)
				}
			}
			if !bytes.Equal(destination.Bytes(), want) {
				b.Fatalf("canonical output=%x, want %x", destination.Bytes(), want)
			}
		})
	}
}

func BenchmarkRetrievalDownloadFile(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "32KiB", size: 32 << 10},
		{name: "4MiB", size: 4 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			payload := bytes.Repeat([]byte{0x6a}, tc.size)
			fixture := newDownloadCallFixture(b, downloadCallFixtureRequest{Payload: payload})
			root := retrievalFileRoot(b, b.TempDir())
			request := FileDownloadRequest{
				Client:     retrievalObjectstoreClient(b, payload),
				Activation: retrievalActivation(b, retrievalActivationRequest{Root: root, Size: uint64(tc.size), Install: filestore.InstallReplace}),
			}
			b.ReportAllocs()
			b.SetBytes(int64(tc.size))
			for b.Loop() {
				recovery, transfer, err := fixture.grant.DownloadFile(b.Context(), request)
				if err != nil || recovery.Validate() == nil || transfer.Validate() != nil || transfer.Bytes() != fixture.addition.Entry.Evidence.Payload.Body.Extent || transfer.SHA256() != fixture.addition.Entry.Evidence.Payload.Body.SHA256 {
					b.Fatalf("download=%v/%v/%v", recovery, transfer, err)
				}
			}
			if got := readRetrievalTarget(b, root, "target"); !bytes.Equal(got, payload) {
				b.Fatalf("activated file extent=%d, want exact authenticated extent=%d", len(got), len(payload))
			}
		})
	}
}
