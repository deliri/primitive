package gcsobjects

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Both public read doors execute the official SDK against a local provider.
// The oracle is independent byte equality, exact destination contents and no
// proof on refusal, including when metadata agrees with only a source prefix.
func FuzzGCSReadExtentSemanticBoundary(f *testing.F) {
	for _, data := range [][]byte{nil, {0x7f}, []byte("distinct provider payload")} {
		for mutation := uint8(0); mutation < 4; mutation++ {
			f.Add(data, mutation, false)
			f.Add(data, mutation, true)
		}
	}
	f.Fuzz(func(t *testing.T, input []byte, mutation uint8, chunked bool) {
		if len(input) > 4096 {
			input = input[:4096]
		}
		payload := bytes.Clone(input)
		expected := bytes.Clone(input)
		switch mutation % 4 {
		case 0:
		case 1:
			payload = append(payload, 0x7f)
		case 2:
			expected = append(expected, 0x7f)
		case 3:
			if len(payload) != 0 {
				payload[0] ^= 0xff
			} else {
				payload = []byte{0xff}
			}
		}
		wantAccepted := bytes.Equal(payload, expected)
		directory := t.TempDir()
		destination, root := gcsReadStageDestination(t, directory, uint64(len(expected)))
		provider := &gcsReadProvider{t: t, payload: payload, metadataBytes: expected, chunked: chunked, disposition: gcsReadAvailable}
		client := bucketTestClient(t, provider)
		integrity := gcsProviderIntegrity(t, expected, expected)
		request := GCSReadRequest{Destination: destination, Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), Integrity: integrity}
		if err := request.Validate(); err != nil {
			t.Fatalf("typed fuzz request error = %v, want nil", err)
		}
		got, gotErr := ReadGCSObject(t.Context(), client, request)
		if wantAccepted {
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("exact provider read = (%v, %v), want validated proof", got, gotErr)
			}
			var content bytes.Buffer
			_, err := filestore.Read(t.Context(), filestore.ReadRequest{Destination: &content, Location: filestore.Location{Root: root, Path: destination.Temporary.Path}})
			if err != nil || !bytes.Equal(content.Bytes(), expected) {
				t.Fatalf("staged bytes = (%v, %v), want exact %v", content.Bytes(), err, expected)
			}
			staged, err := got.Staged()
			if err != nil {
				t.Fatalf("Staged error = %v, want nil", err)
			}
			if err := filestore.Discard(t.Context(), staged); err != nil {
				t.Fatalf("Discard error = %v, want nil", err)
			}
		} else {
			if !errors.Is(gotErr, core.ErrObjectStoreIntegrity) || got != (GCSReadResult{}) {
				t.Fatalf("mutated provider read = (%v, %v), want zero and integrity refusal", got, gotErr)
			}
			var content bytes.Buffer
			_, err := filestore.Read(t.Context(), filestore.ReadRequest{Destination: &content, Location: filestore.Location{Root: root, Path: destination.Temporary.Path}})
			if !errors.Is(err, fs.ErrNotExist) || content.Len() != 0 {
				t.Fatalf("rejected stage = (%d bytes, %v), want absent", content.Len(), err)
			}
		}
		listedProvider := &gcsListReadProvider{t: t, payload: expected, mediaOverride: &payload, chunked: chunked}
		listedClient := bucketTestClient(t, listedProvider)
		var listed GCSObjectMetadata
		var visits int
		listErr := ListGCSObjects(t.Context(), listedClient, GCSListRequest{Bucket: request.Bucket, Prefix: parsedGCSObjectPrefix(t, "users/01/evidence/"), MaxObjects: gcsProviderMaximum(t, 1)}, func(value GCSObjectMetadata) error { listed = value; visits++; return nil })
		if listErr != nil || visits != 1 || listed.Validate() != nil || listed.Length() != integrity.Length || listed.CRC32C() != integrity.CRC32C {
			t.Fatalf("listed producer = (%v, %d, %v), want one exact validated metadata fact", listed, visits, listErr)
		}
		var output bytes.Buffer
		read, readErr := ReadListedGCSObject(t.Context(), listedClient, GCSListedReadRequest{Destination: &output, Object: listed, Maximum: gcsProviderMaximum(t, uint64(max(1, len(expected))))})
		if output.Len() > len(expected) {
			t.Fatalf("listed output = %d bytes, want at most %d", output.Len(), len(expected))
		}
		if wantAccepted {
			if readErr != nil || read != listed || !bytes.Equal(output.Bytes(), expected) {
				t.Fatalf("listed exact read = (%v, %v, %v), want exact producer proof and bytes", read, output.Bytes(), readErr)
			}
			return
		}
		if !errors.Is(readErr, core.ErrObjectStoreIntegrity) || read != (GCSObjectMetadata{}) {
			t.Fatalf("listed mutated read = (%v, %v), want zero and integrity refusal", read, readErr)
		}
	})
}
