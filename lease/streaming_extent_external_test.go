package lease_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

type extentValue interface {
	comparable
	Validate() error
	MarshalJSON() ([]byte, error)
}
type extentPointer[T extentValue] interface {
	*T
	UnmarshalJSON([]byte) error
}
type extentDoor struct {
	name          string
	canonical     []byte
	formerMaximum int
	decode        func([]byte) (bool, error)
}

// extentDoorFor returns observed equality, leaving every got/want check in the table.
func extentDoorFor[T extentValue, P extentPointer[T]](t testing.TB, name string, seed T, formerMaximum int) extentDoor {
	t.Helper()
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return extentDoor{name: name, canonical: canonical, formerMaximum: formerMaximum, decode: func(data []byte) (bool, error) {
		got := seed
		err := P(&got).UnmarshalJSON(data)
		return got == seed, err
	}}
}
func leaseExtentDoors(t testing.TB) []extentDoor {
	t.Helper()
	f := leaseFixturesForFuzz(t)
	return []extentDoor{
		extentDoorFor[lease.EntitlementID](t, "entitlement", f.entitlement, lease.IdentifierCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.DeviceID](t, "device", f.device, lease.IdentifierCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.Generation](t, "generation", f.generation, lease.GenerationCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.Revision](t, "revision", f.revision, lease.RevisionCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.Outcome](t, "outcome", f.outcome, lease.OutcomeCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.RevocationReason](t, "reason", f.reason, lease.RevocationReasonCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.Subject](t, "subject", f.subject, lease.SubjectCanonicalJSONMaximumBytes+1024),
		extentDoorFor[lease.Grant](t, "grant", f.grant, lease.GrantCanonicalJSONMaximumBytes+1024),
		extentDoorFor[lease.Refusal](t, "refusal", f.refusal, lease.RefusalCanonicalJSONMaximumBytes+1024),
		extentDoorFor[lease.Revocation](t, "revocation", f.revocation, lease.RevocationCanonicalJSONMaximumBytes+256),
		extentDoorFor[lease.Decision](t, "decision", f.decision, lease.DecisionCanonicalJSONMaximumBytes+4096),
		extentDoorFor[lease.Document](t, "document", f.document, lease.DocumentCanonicalJSONMaximumBytes+8192),
	}
}

// These are historical whitespace budgets, retained only as regression coordinates.
func TestJSONWhitespaceExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, door := range leaseExtentDoors(t) {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name    string
				padding int
				tail    []byte
				wantErr error
			}{
				{name: "canonical fixed point"},
				{name: "one byte below former document quota", padding: door.formerMaximum - len(door.canonical) - 1},
				{name: "exact former document quota", padding: door.formerMaximum - len(door.canonical)},
				{name: "one byte beyond former document quota", padding: door.formerMaximum - len(door.canonical) + 1},
				{name: "many windows remain valid", padding: (1 << 20) + 1},
				{name: "large trailing value stays malformed", padding: 32769, tail: []byte("false"), wantErr: core.ErrJSONContract},
				{name: "large trailing NUL stays malformed", padding: 32769, tail: []byte{0}, wantErr: core.ErrJSONContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					data := append(bytes.Repeat([]byte{' '}, tc.padding), door.canonical...)
					data = append(data, tc.tail...)
					same, err := door.decode(data)
					if !errors.Is(err, tc.wantErr) || !same {
						t.Fatalf("decoded equality/error = %v/%v, want true/%v", same, err, tc.wantErr)
					}
				})
			}
		})
	}
}

func TestSignedDocumentNestedWhitespaceLayerTriad(t *testing.T) {
	t.Parallel()
	f := leaseFixturesForFuzz(t)
	canonical, err := f.document.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		padding  int
		truncate bool
		wantErr  error
	}{
		{name: "canonical authentic agreement"},
		{name: "every nested object beyond old limits", padding: 32769},
		{name: "truncated padded agreement yields no replacement", padding: 32769, truncate: true, wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// The compiler-produced fixture has no braces inside its string values.
			data := bytes.ReplaceAll(canonical, []byte{'{'}, append([]byte{'{'}, bytes.Repeat([]byte{' '}, tc.padding)...))
			if tc.truncate {
				data = data[:len(data)-1]
			}
			got := f.document
			err := got.UnmarshalJSON(data)
			if !errors.Is(err, tc.wantErr) || got != f.document {
				t.Fatalf("document/error = %v/%v, want exact seed/%v", got, err, tc.wantErr)
			}
			if err != nil {
				return
			}
			proof, err := lease.Verify(lease.VerifyRequest{Document: got, TrustedKeys: f.authority.trusted, ExpectedSubject: f.signedSubject})
			if err != nil || proof.Validate() != nil {
				t.Fatalf("padded verification = %v/%v, want authentic", proof, err)
			}
		})
	}
}

type prefixWriter struct {
	buffer bytes.Buffer
	count  int
	err    error
	calls  int
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.calls++
	// bytes.Buffer.Write always returns the full count and nil.
	if w.count >= 0 && w.count <= len(p) {
		_, _ = w.buffer.Write(p[:w.count])
	}
	return w.count, w.err
}

type nilWriter struct{}

func (*nilWriter) Write([]byte) (int, error) { panic("nil writer reached effect") }

func TestCanonicalWriterExactPrefixLayerTriad(t *testing.T) {
	t.Parallel()
	f := leaseFixturesForFuzz(t)
	canonical, err := f.decision.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for count := -1; count <= len(canonical)+1; count++ {
		for _, cause := range []error{nil, io.ErrClosedPipe} {
			t.Run(fmt.Sprintf("count_%d_cause_%v", count, cause), func(t *testing.T) {
				t.Parallel()
				writer := &prefixWriter{count: count, err: cause}
				err := f.decision.WriteCanonical(writer)
				wantErr := cause
				if wantErr == nil && count != len(canonical) {
					wantErr = io.ErrShortWrite
				}
				if !errors.Is(err, wantErr) || (err != nil && !errors.Is(err, core.ErrLeaseContract)) {
					t.Fatalf("writer error = %v, want %v and lease identity", err, wantErr)
				}
				want := []byte(nil)
				if count >= 0 && count <= len(canonical) {
					want = canonical[:count]
				}
				if writer.calls != 1 || !bytes.Equal(writer.buffer.Bytes(), want) {
					t.Fatalf("calls/prefix = %d/%x, want 1/%x", writer.calls, writer.buffer.Bytes(), want)
				}
			})
		}
	}
}

func TestCanonicalWriterNilIngress(t *testing.T) {
	t.Parallel()
	f := leaseFixturesForFuzz(t)
	for _, tc := range []struct {
		name        string
		destination io.Writer
	}{
		{name: "nil interface"},
		{name: "typed nil pointer", destination: (*nilWriter)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := f.decision.WriteCanonical(tc.destination)
			if !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("nil writer error = %v, want lease contract", err)
			}
		})
	}
}

func FuzzLeaseWhitespaceSemanticClosure(f *testing.F) {
	doors := leaseExtentDoors(f)
	for index := range doors {
		f.Add(uint8(index), uint16(32769), false)
	}
	f.Fuzz(func(t *testing.T, index uint8, padding uint16, malformed bool) {
		door := doors[int(index)%len(doors)]
		data := append(bytes.Repeat([]byte{' '}, int(padding)), door.canonical...)
		if malformed {
			data = append(data, 0)
		}
		same, err := door.decode(data)
		wantErr := error(nil)
		if malformed {
			wantErr = core.ErrJSONContract
		}
		if !errors.Is(err, wantErr) || !same {
			t.Fatalf("whitespace equality/error = %v/%v, want true/%v", same, err, wantErr)
		}
	})
}

func FuzzLeaseDomainTextSemanticClosure(f *testing.F) {
	canonical, err := lease.DomainDecisionV1.MarshalText()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add(append(bytes.Clone(canonical), 0))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := lease.DomainUnknown.ParseCanonicalText(data)
		wantErr := error(nil)
		want := lease.DomainDecisionV1
		if !bytes.Equal(data, canonical) {
			wantErr = core.ErrLeaseContract
			want = lease.DomainUnknown
		}
		if !errors.Is(err, wantErr) || got != want {
			t.Fatalf("domain/error = %v/%v, want %v/%v", got, err, want, wantErr)
		}
		if err != nil {
			return
		}
		encoded, err := got.MarshalText()
		if err != nil || got.Validate() != nil || !bytes.Equal(encoded, canonical) {
			t.Fatalf("domain projection/error = %q/%v, want %q", encoded, err, canonical)
		}
	})
}
