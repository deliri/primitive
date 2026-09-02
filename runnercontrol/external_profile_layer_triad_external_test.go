package runnercontrol_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/standard"
)

func TestJavaScriptProfileCompilerLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive Bun plan owns exact test selection concurrency and JUnit evidence", func(t *testing.T) {
		t.Parallel()
		request := javaScriptPlanFixture(t)
		got, gotErr := runnercontrol.CompileJavaScriptPlan(request)
		arguments := planArguments(t, got.Process)
		wantArguments := []string{"test", "web/session.test.ts", "--reporter=junit", "--reporter-outfile=/workspace/output/bun-junit.xml", "--max-concurrency=4"}
		if gotErr != nil || !slices.Equal(arguments, wantArguments) || got.Observation.Format != runnercontrol.ObservationJUnitXML || len(got.Artifacts) != 1 || got.Artifacts[0].Kind != runnercontrol.ArtifactReport {
			t.Fatalf("CompileJavaScriptPlan() = (arguments %q, observation %+v, artifacts %+v, error %v), want (%q, junit-xml, one report, nil)", arguments, got.Observation, got.Artifacts, gotErr, wantArguments)
		}
	})

	t.Run("negative non-JavaScript target is refused before process authority exists", func(t *testing.T) {
		t.Parallel()
		request := javaScriptPlanFixture(t)
		request.Test.File = mustProfileSourcePath(t, "web/session.go")
		got, gotErr := runnercontrol.CompileJavaScriptPlan(request)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Process.SchemaVersion != 0 {
			t.Fatalf("CompileJavaScriptPlan(non-JavaScript target) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral default deny egress cannot acquire a namespace or extra artifacts", func(t *testing.T) {
		t.Parallel()
		request := javaScriptPlanFixture(t)
		got, gotErr := runnercontrol.CompileJavaScriptPlan(request)
		if gotErr != nil || got.Subject.NetworkNamespace != nil || got.Subject.NetworkController != nil || len(got.Artifacts) != 1 {
			t.Fatalf("CompileJavaScriptPlan(deny egress) = (namespace %v, controller %v, artifacts %d, error %v), want (nil, nil, one JUnit report, nil)", got.Subject.NetworkNamespace, got.Subject.NetworkController, len(got.Artifacts), gotErr)
		}
	})
}

func TestSuiteProfileCompilerLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive pinned smoke suite binds the reviewed network policy and exact suite", func(t *testing.T) {
		t.Parallel()
		request := suitePlanFixture(t, pinnedSuiteEgress(t))
		got, gotErr := runnercontrol.CompileSuitePlan(request)
		arguments := planArguments(t, got.Process)
		if gotErr != nil || !slices.Equal(arguments, []string{"--suite=public-api"}) || got.Subject.NetworkNamespace == nil || got.Subject.NetworkController == nil {
			t.Fatalf("CompileSuitePlan(pinned suite) = (arguments %q, namespace %v, controller %v, error %v), want ([--suite=public-api], prepared namespace/controller, nil)", arguments, got.Subject.NetworkNamespace, got.Subject.NetworkController, gotErr)
		}
	})

	t.Run("negative changed egress policy cannot reuse subject execution authority", func(t *testing.T) {
		t.Parallel()
		request := suitePlanFixture(t, pinnedSuiteEgress(t))
		request.Base.Egress = deniedExternalEgress()
		got, gotErr := runnercontrol.CompileSuitePlan(request)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Process.SchemaVersion != 0 {
			t.Fatalf("CompileSuitePlan(changed egress) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral smoke suite with no admitted services retains deny-all egress", func(t *testing.T) {
		t.Parallel()
		request := suitePlanFixture(t, deniedExternalEgress())
		got, gotErr := runnercontrol.CompileSuitePlan(request)
		if gotErr != nil || got.Subject.NetworkNamespace != nil || len(got.Artifacts) != 0 || got.Observation.Format != runnercontrol.ObservationOpaque {
			t.Fatalf("CompileSuitePlan(no services) = (namespace %v, artifacts %d, observation %v, error %v), want (nil, 0, opaque, nil)", got.Subject.NetworkNamespace, len(got.Artifacts), got.Observation.Format, gotErr)
		}
	})
}

func TestToolProfileCompilerLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive reviewed tool executable receives no caller argument surface", func(t *testing.T) {
		t.Parallel()
		request := toolPlanFixture(t)
		got, gotErr := runnercontrol.CompileToolPlan(request)
		arguments := planArguments(t, got.Process)
		if gotErr != nil || len(arguments) != 0 || got.Process.Command != request.Base.Command {
			t.Fatalf("CompileToolPlan() = (command %v, arguments %q, error %v), want (%v, empty, nil)", got.Process.Command, arguments, gotErr, request.Base.Command)
		}
	})

	t.Run("negative tool cannot receive pinned network authority", func(t *testing.T) {
		t.Parallel()
		request := toolPlanFixture(t)
		request.Base = externalPlanBaseFixture(t, pinnedSuiteEgress(t))
		got, gotErr := runnercontrol.CompileToolPlan(request)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Process.SchemaVersion != 0 {
			t.Fatalf("CompileToolPlan(pinned egress) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral tool creates no fabricated measurement artifact", func(t *testing.T) {
		t.Parallel()
		got, gotErr := runnercontrol.CompileToolPlan(toolPlanFixture(t))
		if gotErr != nil || len(got.Artifacts) != 0 || got.Observation.Format != runnercontrol.ObservationOpaque {
			t.Fatalf("CompileToolPlan(no outputs) = (artifacts %d, observation %v, error %v), want (0, opaque, nil)", len(got.Artifacts), got.Observation.Format, gotErr)
		}
	})
}

func javaScriptPlanFixture(t testing.TB) runnercontrol.JavaScriptPlanRequest {
	t.Helper()
	return runnercontrol.JavaScriptPlanRequest{
		Base: externalPlanBaseFixture(t, deniedExternalEgress()),
		Test: runnercontrol.JavaScriptTestPlan{
			File: mustProfileSourcePath(t, "web/session.test.ts"), Timeout: mustProfileDuration(t, 60_000_000_000),
			Concurrency: 4, RepeatCount: runnercontrol.JavaScriptRepeatCount, Report: mustProfileRelativePath(t, "bun-junit.xml"),
		},
		ArtifactLimit: mustProfileByteCount(t, 8<<20), ExpectedUnits: 1,
	}
}

func suitePlanFixture(t testing.TB, egress runnercontrol.EgressPolicy) runnercontrol.SuitePlanRequest {
	t.Helper()
	suite, err := standard.NewIdentifier("public-api")
	if err != nil {
		t.Fatalf("standard.NewIdentifier(smoke suite) error = %v, want nil", err)
	}
	return runnercontrol.SuitePlanRequest{Base: externalPlanBaseFixture(t, egress), Suite: runnercontrol.SuitePlan{Suite: suite, Timeout: mustProfileDuration(t, 120_000_000_000)}}
}

func toolPlanFixture(t testing.TB) runnercontrol.ToolPlanRequest {
	t.Helper()
	base := externalPlanBaseFixture(t, deniedExternalEgress())
	base.Command = mustProfileAbsolutePath(t, "/usr/libexec/fixture-tool")
	return runnercontrol.ToolPlanRequest{Base: base, Tool: runnercontrol.ToolPlan{Timeout: mustProfileDuration(t, 60_000_000_000)}}
}

func externalPlanBaseFixture(t testing.TB, egress runnercontrol.EgressPolicy) runnercontrol.ExternalPlanBase {
	t.Helper()
	source := mustProfileAbsolutePath(t, "/source")
	subject := subjectExecutionFixture(t, source)
	digest, err := egress.Digest()
	if err != nil {
		t.Fatalf("EgressPolicy.Digest(external profile fixture) error = %v, want nil", err)
	}
	subject.EgressPolicyIdentity = digest
	if egress.Mode == runnercontrol.EgressPinned {
		namespace := mustProfileAbsolutePath(t, "/run/netns/primitive-smoke")
		controller := mustProfileAbsolutePath(t, "/usr/libexec/primitive-network")
		subject.NetworkNamespace = &namespace
		subject.NetworkController = &controller
	}
	return runnercontrol.ExternalPlanBase{
		Command: mustProfileAbsolutePath(t, "/usr/local/bin/bun"), WorkingDirectory: source,
		WorkspaceRoot: mustProfileAbsolutePath(t, "/workspace"), OutputDirectory: mustProfileRelativePath(t, "output"),
		Environment: runnercontrol.ExternalExecutionEnvironment{
			Home: mustProfileAbsolutePath(t, "/workspace/home"), Cache: mustProfileAbsolutePath(t, "/workspace/cache"), Temporary: mustProfileAbsolutePath(t, "/workspace/tmp"),
		},
		OutputLimit: mustProfileByteCount(t, 8<<20), WaitDelay: mustProfileDuration(t, 5_000_000_000), Subject: subject, Egress: egress,
	}
}

func deniedExternalEgress() runnercontrol.EgressPolicy {
	return runnercontrol.EgressPolicy{Mode: runnercontrol.EgressDenied, Rules: []runnercontrol.EgressRule{}, DNSPolicy: core.SHA256Of([]byte("deny-all-dns"))}
}

func pinnedSuiteEgress(t testing.TB) runnercontrol.EgressPolicy {
	t.Helper()
	service, serviceErr := standard.NewIdentifier("public-api")
	endpoint, endpointErr := core.ParseHTTPEndpoint("https://api.example.test")
	if err := errors.Join(serviceErr, endpointErr); err != nil {
		t.Fatalf("pinned smoke egress fixture error = %v, want nil", err)
	}
	return runnercontrol.EgressPolicy{
		Mode:      runnercontrol.EgressPinned,
		Rules:     []runnercontrol.EgressRule{{Service: service, Endpoint: endpoint, Protocol: runnercontrol.NetworkTCP, Port: 443, Certificate: core.SHA256Of([]byte("api certificate"))}},
		DNSPolicy: core.SHA256Of([]byte("pinned smoke DNS")),
	}
}
