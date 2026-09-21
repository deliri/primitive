package upgradereport

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzAcknowledgmentAuthenticationAndExactRequestBinding(f *testing.F) {
	r, i, keys, _ := reportFixture(f)
	seed := value[Response](f)(SignAcknowledgment(Acknowledgment{Attempt: r.Payload.Attempt, RequestDigest: value[core.SHA256Digest](f)(r.Digest()), RecordedAt: r.Payload.ObservedAt}, i.AuthorityPrivate))
	canonical := value[[]byte](f)(seed.MarshalJSON())
	forged := seed
	forged.Payload.RequestDigest = core.SHA256Of([]byte("a different request"))
	f.Add(canonical)
	f.Add(value[[]byte](f)(forged.MarshalJSON()))
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(canonical[:len(canonical)-1])
	f.Add(append(bytes.Clone(canonical), canonical...))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if (!errors.Is(err, core.ErrReportContract) && !errors.Is(err, core.ErrReportBinding)) || got != seed {
				t.Fatalf("rejected acknowledgment = (%v,%v), want typed refusal and unchanged receiver", got, err)
			}
			return
		}
		encoded := value[[]byte](t)(got.MarshalJSON())
		var again Response
		if err := again.UnmarshalJSON(encoded); err != nil || again != got {
			t.Fatalf("acknowledgment closure = (%v,%v), want admitted response", again, err)
		}
		if !bytes.Equal(value[[]byte](t)(again.MarshalJSON()), encoded) {
			t.Fatalf("second canonical acknowledgment=%q, want %q", value[[]byte](t)(again.MarshalJSON()), encoded)
		}
		err := got.Verify(keys, r)
		if got == seed {
			if err != nil {
				t.Fatalf("genuine acknowledgment = %v, want nil", err)
			}
		} else if !errors.Is(err, core.ErrReportAuthentication) && !errors.Is(err, core.ErrReportBinding) {
			t.Fatalf("unsigned acknowledgment recombination = %v, want authentication/binding refusal", err)
		}
	})
}
