package objectstore

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzS3CredentialBudgetSemanticClosure(f *testing.F) {
	canonical := s3CredentialFixtureJSON(f)
	f.Add(canonical, uint64(len(canonical)))
	f.Add(canonical, uint64(len(canonical)-1))
	f.Add(canonical, uint64(len(canonical)+1))
	f.Add(append(bytes.Clone(canonical), ' '), uint64(len(canonical)))
	f.Add(canonical, uint64(0))
	f.Add([]byte("null"), uint64(4))
	f.Fuzz(func(t *testing.T, data []byte, rawBudget uint64) {
		// Bound the campaign's two independent decoders, not the public policy.
		if len(data) > 64*1024 {
			t.Skip("fuzz campaign input budget")
		}
		var budget core.ByteCount
		if rawBudget > 0 {
			budget = s3CredentialBudget(t, rawBudget)
		}
		probe := &s3CredentialProbe{source: bytes.NewReader(data)}
		got, gotErr := decodeS3BoundedCredentials(t.Context(), probe, budget)
		defer got.destroy()
		plain, plainErr := decodeS3FileCredentials(t.Context(), bytes.NewReader(data))
		defer plain.destroy()
		// The original strict decoder owns the grammar. Extent and native
		// representability independently determine the new budget agreement.
		limit, limitErr := s3CredentialReadLimit(budget)
		want := plainErr == nil && limitErr == nil && uint64(len(data)) <= rawBudget
		if want {
			if gotErr != nil || !bytes.Equal(got.AccessKeyID.value, plain.AccessKeyID.value) || !bytes.Equal(got.SecretAccessKey.value, plain.SecretAccessKey.value) || probe.read != len(data) {
				t.Fatalf("bounded credential acceptance error=%v sizes=(%d,%d) consumed=%d, want exact full credential of %d bytes", gotErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value), probe.read, len(data))
			}
			return
		}
		if !errors.Is(gotErr, core.ErrObjectStoreContract) || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 || (limitErr == nil && int64(probe.read) > limit) || (limitErr != nil && probe.read != 0) {
			t.Fatalf("bounded credential refusal error=%v sizes=(%d,%d) consumed=%d limit=%d/%v, want typed zero and bounded consumption", gotErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value), probe.read, limit, limitErr)
		}
		if plainErr == nil && limitErr == nil && !errors.Is(gotErr, core.ErrObjectStoreSize) {
			t.Fatalf("oversized valid credential error=%v, want stable extent refusal", gotErr)
		}
	})
}
