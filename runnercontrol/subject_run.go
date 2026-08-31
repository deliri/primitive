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
	plan, err := CompileSubjectProcess(capability)
	if err != nil {
		return process.Result{}, err
	}
	request, err := plan.Bind(streams)
	if err != nil {
		return process.Result{}, err
	}
	if err := prepareSubjectNetwork(ctx, capability); err != nil {
		return process.Result{}, errors.Join(err, destroySubjectNetwork(capability))
	}
	result, runErr := process.Run(ctx, request)
	var stopErr error
	if ctx == nil || ctx.Err() == nil || result.Validate() != nil {
		return result, errors.Join(runErr, destroySubjectNetwork(capability))
	}
	stopErr = stopSubjectUnit(capability)
	return result, errors.Join(runErr, stopErr, destroySubjectNetwork(capability))
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

func destroySubjectNetwork(capability ExperimentCapability) error {
	if capability.Resources.Egress.Mode == EgressDenied {
		return nil
	}
	cleanupCtx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.Background(), Duration: capability.Execution.Process.WaitDelay})
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
	result, runErr := process.Run(ctx, request)
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
	switch o {
	case subjectNetworkPrepare:
		return "prepare"
	case subjectNetworkDestroy:
		return "destroy"
	default:
		return "unknown"
	}
}

func subjectNetworkControllerPlan(capability ExperimentCapability, operation subjectNetworkOperation) (process.Plan, error) {
	if capability.Execution.Subject.NetworkController == nil || capability.Execution.Subject.NetworkNamespace == nil || operation.String() == "unknown" {
		return process.Plan{}, errors.Join(core.ErrPrimitiveContract, errors.New("pinned subject network controller is incomplete"))
	}
	digest, err := capability.Resources.Egress.Digest()
	if err != nil || digest != capability.Execution.Subject.EgressPolicyIdentity {
		return process.Plan{}, errors.Join(core.ErrPrimitiveContract, err, errors.New("pinned subject network policy digest does not match execution authority"))
	}
	digestHex, err := digest.Hex()
	if err != nil {
		return process.Plan{}, err
	}
	arguments, err := process.ParseArguments([]string{operation.String(), capability.Execution.Subject.NetworkNamespace.String(), digestHex})
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
	workingDirectory, err := capability.Execution.Subject.NetworkController.Parent()
	if err != nil {
		return process.Plan{}, err
	}
	plan := process.Plan{
		SchemaVersion:    process.ExecutionPlanSchemaVersion,
		Command:          *capability.Execution.Subject.NetworkController,
		WorkingDirectory: workingDirectory,
		Arguments:        arguments, Environment: environment, OutputLimit: output,
		WaitDelay:   capability.Execution.Process.WaitDelay,
		Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalTerminate},
	}
	return plan, plan.Validate()
}

func stopSubjectUnit(capability ExperimentCapability) error {
	stopCtx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.Background(), Duration: capability.Execution.Process.WaitDelay})
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
	result, runErr := process.Run(stopCtx, request)
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

func subjectStopPlan(capability ExperimentCapability) (process.Plan, error) {
	arguments, err := process.ParseArguments([]string{"stop", "anvil-" + capability.Experiment.String() + ".service"})
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
