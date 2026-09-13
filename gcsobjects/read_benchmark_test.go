package gcsobjects

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

// Measures the real SDK read, exact integrity verification, staged file and
// discard on a local HTTP provider. It does not measure remote GCS latency.
func BenchmarkGCSReadAndStage(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name string
		size int
	}{{"1KiB", 1 << 10}, {"1MiB", 1 << 20}} {
		b.Run(tc.name, func(b *testing.B) {
			directory := b.TempDir()
			payload := make([]byte, tc.size)
			for i := range payload {
				payload[i] = byte(i*37 + i/251)
			}
			client := bucketTestClient(b, &gcsReadProvider{t: b, payload: payload, metadataBytes: payload, disposition: gcsReadAvailable})
			destination, _ := gcsReadStageDestination(b, directory, uint64(len(payload)))
			request := GCSReadRequest{Generation: gcsReadGeneration(b), Destination: destination, Bucket: parsedGCSBucket(b, gcsProviderBucketText), Name: parsedGCSObjectName(b, gcsProviderObjectText), Integrity: gcsProviderIntegrity(b, payload, payload)}
			if err := request.Validate(); err != nil || bytes.Count(payload, []byte{0}) == len(payload) {
				b.Fatalf("benchmark request error = %v, want valid nonuniform bytes", err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			for b.Loop() {
				got, err := ReadGCSObject(b.Context(), client, request)
				if err != nil {
					b.Fatalf("ReadGCSObject error = %v, want nil", err)
				}
				staged, err := got.Staged()
				if err != nil {
					b.Fatalf("Staged error = %v, want nil", err)
				}
				if err := filestore.Discard(b.Context(), staged); err != nil {
					b.Fatalf("Discard error = %v, want nil", err)
				}
			}
		})
	}
}
