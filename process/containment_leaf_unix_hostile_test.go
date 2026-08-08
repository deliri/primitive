//go:build unix

package process

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestApplyContainmentRefusesAnIsolationOutsideTheAdmittedDomain pins the
// platform leaf to the fail-closed discriminator contract: the leaf receives a
// three-state enum and must not read it as a bool. Before this ratchet an
// unknown isolation silently rode the direct arm, so a path that ever handed
// the leaf an unvalidated containment would start an uncontained child instead
// of refusing. Every path today resolves orDefault and validates first, which
// is exactly why this is a ratchet: the leaf must stay correct even when a
// future caller is not.
func TestApplyContainmentRefusesAnIsolationOutsideTheAdmittedDomain(t *testing.T) {
	t.Parallel()

	for _, isolation := range []Isolation{IsolationUnknown, Isolation(200)} {
		command := exec.Command("true")
		err := applyContainment(command, Containment{Isolation: isolation, CancelSignal: CancelSignalKill})
		if !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("applyContainment(isolation %d) error = %v, want errors.Is %v",
				isolation, err, core.ErrProcessContract)
		}
		if command.SysProcAttr != nil {
			t.Fatalf("applyContainment(isolation %d) configured SysProcAttr = %+v alongside the refusal, want none",
				isolation, command.SysProcAttr)
		}
	}
}

// TestDeliverSignalRefusesAnIsolationOutsideTheAdmittedDomain proves the
// delivery leaf refuses an unknown isolation before it addresses any process:
// the refusal must arrive with no held process touched, which is why the
// delivery carries no process at all and the test would panic if the leaf
// reached for one.
func TestDeliverSignalRefusesAnIsolationOutsideTheAdmittedDomain(t *testing.T) {
	t.Parallel()

	for _, isolation := range []Isolation{IsolationUnknown, Isolation(200)} {
		err := deliverSignal(signalDelivery{
			containment: Containment{Isolation: isolation, CancelSignal: CancelSignalKill},
			signal:      CancelSignalKill,
		})
		if !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("deliverSignal(isolation %d) error = %v, want errors.Is %v",
				isolation, err, core.ErrProcessContract)
		}
	}
}
