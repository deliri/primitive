//go:build unix

package runnercontrol_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestRunSubjectProcessLifecycleLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real subject path starts waits and returns reaped evidence", func(t *testing.T) {
		t.Parallel()
		capability := runnableSubjectCapability(t, "/usr/bin/true")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		got, gotErr := runnercontrol.RunSubject(t.Context(), capability, process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr,
		})
		if gotErr != nil {
			t.Fatalf("RunSubject(real supervisor) error = %v, want nil", gotErr)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("RunSubject(real supervisor) result validation error = %v, want nil", err)
		}
		exit, err := got.ExitCode()
		if err != nil {
			t.Fatalf("RunSubject(real supervisor) ExitCode() error = %v, want nil", err)
		}
		success, err := exit.Success()
		if err != nil || !success || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("RunSubject(real supervisor) = success %t/stdout %q/stderr %q/error %v, want true/empty/empty/nil", success, stdout.String(), stderr.String(), err)
		}
	})

	t.Run("positive pinned egress runs prepare subject and destroy through owned groups", func(t *testing.T) {
		t.Parallel()
		capability := pinnedNetworkSubjectCapability(t)
		got, gotErr := runnercontrol.RunSubject(t.Context(), capability, process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
		})
		if gotErr != nil {
			t.Fatalf("RunSubject(pinned network controllers) error = %v, want nil", gotErr)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("RunSubject(pinned network controllers) result validation error = %v, want nil", err)
		}
	})

	t.Run("negative real start failure preserves typed process identity", func(t *testing.T) {
		t.Parallel()
		capability := runnableSubjectCapability(t, "/primitive-test/missing-supervisor")
		got, gotErr := runnercontrol.RunSubject(t.Context(), capability, process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
		})
		if !errors.Is(gotErr, core.ErrProcessStart) || got != (process.Result{}) {
			t.Fatalf("RunSubject(missing supervisor) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrProcessStart)
		}
	})

	t.Run("negative cancellation retains reaped evidence and stop controller start refusal", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		capability := runnableSubjectCapability(t, "/usr/bin/yes")
		capability.Execution.Subject.Controller = mustProfileAbsolutePath(t, "/primitive-test/missing-controller")
		if err := capability.Validate(); err != nil {
			t.Fatalf("ExperimentCapability.Validate(cancelled subject) error = %v, want nil", err)
		}
		got, gotErr := runnercontrol.RunSubject(ctx, capability, process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &subjectCancellationWriter{cancel: cancel}, Stderr: io.Discard,
		})
		if !errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrProcessStart) {
			t.Fatalf("RunSubject(cancelled subject with missing stop controller) error = %v, want errors.Is %v and %v", gotErr, context.Canceled, core.ErrProcessStart)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("RunSubject(cancelled subject) result validation error = %v, want reaped evidence", err)
		}
	})

	t.Run("neutral cancelled ingress emits no process evidence", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		capability := runnableSubjectCapability(t, "/usr/bin/true")
		got, gotErr := runnercontrol.RunSubject(ctx, capability, process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
		})
		if !errors.Is(gotErr, context.Canceled) || got != (process.Result{}) {
			t.Fatalf("RunSubject(cancelled ingress) = (%v, %v), want zero and errors.Is %v", got, gotErr, context.Canceled)
		}
	})
}

func runnableSubjectCapability(t testing.TB, supervisor string) runnercontrol.ExperimentCapability {
	t.Helper()
	capability := experimentObservationRequestFixture(t).Capability
	capability.Execution.Subject.Supervisor = mustProfileAbsolutePath(t, supervisor)
	capability.Execution.Process.WaitDelay = mustProfileDuration(t, 1_000_000_000)
	if err := capability.Validate(); err != nil {
		t.Fatalf("ExperimentCapability.Validate(runnable subject) error = %v, want nil", err)
	}
	return capability
}

func pinnedNetworkSubjectCapability(t testing.TB) runnercontrol.ExperimentCapability {
	t.Helper()
	capability := runnableSubjectCapability(t, "/usr/bin/true")
	service, serviceErr := runprotocol.NewIdentifier("subject-api")
	endpoint, endpointErr := core.ParseHTTPEndpoint("https://127.0.0.1:8443")
	if err := errors.Join(serviceErr, endpointErr); err != nil {
		t.Fatalf("pinned subject network fixture error = %v, want nil", err)
	}
	capability.Resources.Egress = runnercontrol.EgressPolicy{
		Mode: runnercontrol.EgressPinned, DNSPolicy: core.SHA256Of([]byte("pinned-dns")),
		Rules: []runnercontrol.EgressRule{{
			Service: service, Endpoint: endpoint, Protocol: runnercontrol.NetworkTCP, Port: 8443,
			Certificate: core.SHA256Of([]byte("subject-api-certificate")),
		}},
	}
	digest, err := capability.Resources.Egress.Digest()
	if err != nil {
		t.Fatalf("EgressPolicy.Digest(pinned subject network) error = %v, want nil", err)
	}
	namespace := mustProfileAbsolutePath(t, "/run/netns/primitive-test")
	controller := mustProfileAbsolutePath(t, "/usr/bin/true")
	capability.Execution.Subject.EgressPolicyIdentity = digest
	capability.Execution.Subject.NetworkNamespace = &namespace
	capability.Execution.Subject.NetworkController = &controller
	if err := capability.Validate(); err != nil {
		t.Fatalf("ExperimentCapability.Validate(pinned subject network) error = %v, want nil", err)
	}
	return capability
}

type subjectCancellationWriter struct {
	cancel context.CancelFunc
	once   sync.Once
}

func (w *subjectCancellationWriter) Write(data []byte) (int, error) {
	w.once.Do(w.cancel)
	return 0, context.Canceled
}
