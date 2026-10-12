//go:build linux || darwin

package hostfacts

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"testing"
)

func TestEffectiveUserNativeObservationLayerTriad(t *testing.T) {
	t.Parallel()
	got, err := ObserveEffectiveUser(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	account, err := user.LookupId(strconv.Itoa(os.Geteuid()))
	if err != nil {
		t.Fatal(err)
	}
	reference, err := exec.CommandContext(t.Context(), "whoami").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != account.Username || got.String() != strings.TrimSpace(string(reference)) || got.Validate() != nil {
		t.Fatal("effective user disagrees with native account and whoami observations")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if value, err := ObserveEffectiveUser(ctx); value != (UserName{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled observation=%v/%v, want zero typed refusal", value, err)
	}
	if value, err := ObserveEffectiveUser(nil); value != (UserName{}) || !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("absent context observation=%v/%v, want zero typed refusal", value, err)
	}
	if err := (UserName{}).Validate(); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("absent user validation=%v, want typed refusal", err)
	}
}
