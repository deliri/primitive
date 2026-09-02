package runnercontrol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const subjectControllerOutputMaximumBytes uint64 = 32 * 1024

type subjectNetworkOperation uint8

const (
	subjectNetworkPrepare subjectNetworkOperation = iota + 1
	subjectNetworkDestroy
)

// RunSubject executes one capability through its reviewed isolation engine.
// When cancellation reaches a started transient service, Primitive stops the
// exact compiler-owned unit before returning the interruption to product code.
func RunSubject(ctx context.Context, capability ExperimentCapability, streams process.Streams) (process.Result, error) {
	if ctx == nil {
		return process.Result{}, errors.Join(core.ErrProcessContract, errors.New("subject context is absent"))
	}
	plan, err := CompileSubjectProcess(capability)
	if err != nil {
		return process.Result{}, err
	}
	request, err := plan.Bind(streams)
	if err != nil {
		return process.Result{}, err
	}
	if err := prepareSubjectNetwork(ctx, capability); err != nil {
		return process.Result{}, errors.Join(err, destroySubjectNetwork(ctx, capability))
	}
	result, runErr := runSubjectGroup(ctx, request)
	var stopErr error
	if ctx == nil || ctx.Err() == nil || result.Validate() != nil {
		return result, errors.Join(runErr, destroySubjectNetwork(ctx, capability))
	}
	stopErr = stopSubjectUnit(ctx, capability)
	return result, errors.Join(runErr, stopErr, destroySubjectNetwork(ctx, capability))
}

func prepareSubjectNetwork(ctx context.Context, capability ExperimentCapability) error {
	if capability.Resources.Egress.Mode == EgressDenied {
		return nil
	}
	encoded, err := core.MarshalCanonicalJSONDocument(capability.Resources.Egress)
	if err != nil {
		return fmt.Errorf("marshal pinned subject egress policy: %w", err)
	}
	return runSubjectNetworkController(ctx, capability, subjectNetworkPrepare, bytes.NewReader(encoded))
}

func destroySubjectNetwork(parent context.Context, capability ExperimentCapability) error {
	if capability.Resources.Egress.Mode == EgressDenied {
		return nil
	}
	cleanupCtx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.WithoutCancel(parent), Duration: capability.Execution.Process.WaitDelay})
	if err != nil {
		return fmt.Errorf("create pinned subject network cleanup context: %w", errors.Join(core.ErrProcessContract, err))
	}
	defer cancel()
	return runSubjectNetworkController(cleanupCtx, capability, subjectNetworkDestroy, bytes.NewReader(nil))
}

func runSubjectNetworkController(ctx context.Context, capability ExperimentCapability, operation subjectNetworkOperation, stdin io.Reader) error {
	plan, err := subjectNetworkControllerPlan(capability, operation)
	if err != nil {
		return err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	request, err := plan.Bind(process.Streams{Stdin: stdin, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		return fmt.Errorf("bind subject network %s operation: %w", operation.String(), err)
	}
	result, runErr := runSubjectGroup(ctx, request)
	if validationErr := result.Validate(); validationErr != nil {
		return fmt.Errorf("subject network %s did not return a reaped controller result: %w", operation.String(), errors.Join(core.ErrProcessContract, runErr, validationErr))
	}
	exit, exitErr := result.ExitCode()
	success, successErr := exit.Success()
	if runErr != nil || exitErr != nil || successErr != nil || !success || stdout.Len() != 0 || stderr.Len() != 0 {
		return fmt.Errorf("subject network %s failed with stdout = %q and stderr = %q: %w", operation.String(), stdout.String(), stderr.String(), errors.Join(core.ErrProcessContract, runErr, exitErr, successErr))
	}
	return nil
}

func (o subjectNetworkOperation) String() string {
	if o == subjectNetworkPrepare {
		return "prepare"
	}
	if o == subjectNetworkDestroy {
		return "destroy"
	}
	return invalidEnumString()
}

func (o subjectNetworkOperation) IsValid() bool {
	return o == subjectNetworkPrepare || o == subjectNetworkDestroy
}

func subjectNetworkControllerPlan(capability ExperimentCapability, operation subjectNetworkOperation) (process.Plan, error) {
	controller, namespace, digestHex, err := subjectNetworkControllerAuthority(capability, operation)
	if err != nil {
		return process.Plan{}, err
	}
	arguments, err := process.ParseArguments([]string{operation.String(), namespace.String(), digestHex})
	if err != nil {
		return process.Plan{}, err
	}
	environment, err := process.ParseExactEnvironment(nil)
	if err != nil {
		return process.Plan{}, err
	}
	output, err := core.NewByteCount(subjectControllerOutputMaximumBytes)
	if err != nil {
		return process.Plan{}, err
	}
	workingDirectory, err := controller.Parent()
	if err != nil {
		return process.Plan{}, err
	}
	plan := process.Plan{
		SchemaVersion:    process.ExecutionPlanSchemaVersion,
		Command:          controller,
		WorkingDirectory: workingDirectory,
		Arguments:        arguments, Environment: environment, OutputLimit: output,
		WaitDelay:   capability.Execution.Process.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	return plan, plan.Validate()
}

func subjectNetworkControllerAuthority(capability ExperimentCapability, operation subjectNetworkOperation) (core.AbsolutePath, core.AbsolutePath, string, error) {
	subject := capability.Execution.Subject
	if subject.NetworkController == nil || subject.NetworkNamespace == nil || !operation.IsValid() {
		return core.AbsolutePath{}, core.AbsolutePath{}, "", errors.Join(core.ErrPrimitiveContract, errors.New("pinned subject network controller is incomplete"))
	}
	digest, err := capability.Resources.Egress.Digest()
	if err != nil || digest != subject.EgressPolicyIdentity {
		return core.AbsolutePath{}, core.AbsolutePath{}, "", errors.Join(core.ErrPrimitiveContract, err, errors.New("pinned subject network policy digest does not match execution authority"))
	}
	digestHex, err := digest.Hex()
	if err != nil {
		return core.AbsolutePath{}, core.AbsolutePath{}, "", err
	}
	return *subject.NetworkController, *subject.NetworkNamespace, digestHex, nil
}

func stopSubjectUnit(parent context.Context, capability ExperimentCapability) error {
	stopCtx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.WithoutCancel(parent), Duration: capability.Execution.Process.WaitDelay})
	if err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	defer cancel()
	plan, err := subjectStopPlan(capability)
	if err != nil {
		return err
	}
	request, err := plan.Bind(process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		return err
	}
	result, runErr := runSubjectGroup(stopCtx, request)
	if result.Validate() != nil {
		return errors.Join(core.ErrProcessContract, runErr)
	}
	exit, exitErr := result.ExitCode()
	success, successErr := exit.Success()
	if exitErr != nil || successErr != nil || !success {
		return errors.Join(core.ErrProcessContract, errors.New("subject controller did not stop the interrupted transient service"), runErr, exitErr, successErr)
	}
	return runErr
}

// runSubjectGroup owns the complete lifecycle of one group-contained subject
// controller. A clean leader wait is the success path and must not sweep:
// the transient service group may intentionally outlive its supervisor. A
// failed wait leaves completion uncertain, so the still-owned execution
// handle sweeps the group before the error crosses into product policy.
func runSubjectGroup(ctx context.Context, request process.Request) (process.Result, error) {
	if request.Containment.Isolation != process.IsolationGroup {
		return process.Result{}, errors.Join(core.ErrProcessContract, errors.New("subject execution requires group containment"))
	}
	execution, err := process.Begin(ctx, request)
	if err != nil {
		return process.Result{}, err
	}
	result, waitErr := execution.Wait()
	if waitErr == nil {
		return result, nil
	}
	return result, errors.Join(waitErr, execution.Sweep())
}

func subjectStopPlan(capability ExperimentCapability) (process.Plan, error) {
	arguments, err := process.ParseArguments([]string{"stop", subjectUnitName(capability.Experiment) + ".service"})
	if err != nil {
		return process.Plan{}, err
	}
	environment, err := process.ParseExactEnvironment(nil)
	if err != nil {
		return process.Plan{}, err
	}
	output, err := core.NewByteCount(subjectControllerOutputMaximumBytes)
	if err != nil {
		return process.Plan{}, err
	}
	plan := process.Plan{
		SchemaVersion: process.ExecutionPlanSchemaVersion,
		Command:       capability.Execution.Subject.Controller, WorkingDirectory: capability.Execution.Subject.SourceRoot,
		Arguments: arguments, Environment: environment, OutputLimit: output, WaitDelay: capability.Execution.Process.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	return plan, plan.Validate()
}
