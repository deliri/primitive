package machineprobe

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	OutputMaximumBytes uint64 = 256 * 1024
	ScriptMaximumBytes uint64 = 128 * 1024
)

type Request struct {
	ObservationID    projectstandards.MachineObservationID `json:"observation_id"`
	GenerationID     projectstandards.MachineGenerationID  `json:"generation_id"`
	ObservedAt       temporal.Instant                      `json:"observed_at"`
	Collector        projectstandards.EvidenceAuthority    `json:"collector"`
	Bash             core.AbsolutePath                     `json:"bash"`
	Script           core.AbsolutePath                     `json:"script"`
	WorkingDirectory core.AbsolutePath                     `json:"working_directory"`
	Environment      process.Environment                   `json:"-"`
	WaitDelay        temporal.Duration                     `json:"wait_delay"`
}

func (r Request) Validate() error {
	if err := errors.Join(r.ObservationID.Validate(), r.GenerationID.Validate(), r.ObservedAt.Validate(), r.Collector.Validate(), r.Bash.Validate(), r.Script.Validate(), r.WorkingDirectory.Validate(), r.Environment.Validate(), r.WaitDelay.Validate()); err != nil {
		return errors.Join(core.ErrProjectStandardsContract, err)
	}
	if r.WaitDelay.IsZero() {
		return errors.Join(core.ErrProjectStandardsContract, errors.New("machine probe wait delay is zero"))
	}
	return nil
}

type FailureKind uint8

const (
	FailureUnknown FailureKind = iota
	FailureExit
	FailureOutput
	failureLimit
)

func (k FailureKind) Validate() error {
	if k <= FailureUnknown || k >= failureLimit {
		return errors.Join(core.ErrProjectStandardsContract, errors.New("machine probe failure kind is invalid"))
	}
	return nil
}

func (k FailureKind) IsValid() bool { return k.Validate() == nil }

func (k FailureKind) String() string {
	if !k.IsValid() {
		return "invalid"
	}
	return []string{"", "exit", "output"}[k]
}

func (k FailureKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *FailureKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(core.ErrJSONContract, core.ErrProjectStandardsContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	for candidate := FailureExit; candidate < failureLimit; candidate++ {
		if candidate.String() == value {
			*k = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrProjectStandardsContract)
}

type Failure struct {
	kind       FailureKind
	exitCode   int
	stderrHash core.SHA256Digest
	stderrSize core.ByteLength
	cause      error
}

type failureInput struct {
	kind     FailureKind
	exitCode int
	stderr   []byte
	cause    error
}

type executionFactInput struct {
	request      Request
	scriptDigest core.SHA256Digest
	scriptBytes  core.ByteLength
	limit        core.ByteCount
	result       process.Result
	stdout       []byte
	stderr       []byte
}

func (f Failure) Error() string                   { return "machine probe failed" }
func (f Failure) Unwrap() error                   { return f.cause }
func (f Failure) Kind() FailureKind               { return f.kind }
func (f Failure) ExitCode() int                   { return f.exitCode }
func (f Failure) StderrDigest() core.SHA256Digest { return f.stderrHash }
func (f Failure) StderrBytes() core.ByteLength    { return f.stderrSize }

func (f Failure) Validate() error {
	if err := f.kind.Validate(); err != nil {
		return err
	}
	if f.cause == nil {
		return errors.Join(core.ErrProjectStandardsContract, errors.New("machine probe failure cause is absent"))
	}
	return errors.Join(f.stderrHash.Validate(), f.stderrSize.Validate())
}

func Collect(ctx context.Context, request Request) (projectstandards.MachineObservation, error) {
	if err := request.Validate(); err != nil {
		return projectstandards.MachineObservation{}, err
	}
	execution, stdout, stderr, err := run(ctx, request)
	if err != nil {
		return projectstandards.MachineObservation{}, err
	}
	var report projectstandards.MachineProbeReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		return projectstandards.MachineObservation{}, newFailure(failureInput{kind: FailureOutput, stderr: stderr, cause: errors.Join(core.ErrHostFactsEvidence, err)})
	}
	fingerprint, err := report.Configuration.Fingerprint()
	if err != nil {
		return projectstandards.MachineObservation{}, newFailure(failureInput{kind: FailureOutput, stderr: stderr, cause: errors.Join(core.ErrHostFactsEvidence, err)})
	}
	observation := projectstandards.MachineObservation{
		SchemaVersion: projectstandards.MachineProbeSchemaVersion,
		ID:            request.ObservationID, GenerationID: request.GenerationID, ObservedAt: request.ObservedAt,
		Collector: request.Collector, Execution: execution, Configuration: report.Configuration,
		Runtime: report.Runtime, Fingerprint: fingerprint,
	}
	if err := observation.Validate(); err != nil {
		return projectstandards.MachineObservation{}, newFailure(failureInput{kind: FailureOutput, stderr: stderr, cause: errors.Join(core.ErrHostFactsEvidence, err)})
	}
	return observation, nil
}

func run(ctx context.Context, request Request) (projectstandards.MachineProbeExecution, []byte, []byte, error) {
	script, scriptBytes, err := readScript(ctx, request.Script)
	if err != nil {
		return projectstandards.MachineProbeExecution{}, nil, nil, err
	}
	if scriptBytes.Uint64() == 0 {
		return projectstandards.MachineProbeExecution{}, nil, nil, newFailure(failureInput{kind: FailureOutput, cause: core.ErrHostFactsEvidence})
	}
	argument, err := process.NewArgument("-s")
	if err != nil {
		return projectstandards.MachineProbeExecution{}, nil, nil, errors.Join(core.ErrProjectStandardsContract, err)
	}
	limit, err := core.NewByteCount(OutputMaximumBytes)
	if err != nil {
		return projectstandards.MachineProbeExecution{}, nil, nil, errors.Join(core.ErrProjectStandardsContract, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result, err := process.Run(ctx, process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(script), Stdout: &stdout, Stderr: &stderr},
		Command: request.Bash, WorkingDirectory: request.WorkingDirectory, Arguments: []process.Argument{argument},
		Environment: request.Environment, OutputLimit: limit, WaitDelay: request.WaitDelay,
	})
	if err != nil {
		return projectstandards.MachineProbeExecution{}, nil, nil, err
	}
	execution, err := executionFact(executionFactInput{
		request: request, scriptDigest: core.SHA256Of(script), scriptBytes: scriptBytes,
		limit: limit, result: result, stdout: stdout.Bytes(), stderr: stderr.Bytes(),
	})
	if err != nil {
		return projectstandards.MachineProbeExecution{}, nil, nil, err
	}
	return execution, stdout.Bytes(), stderr.Bytes(), nil
}

func readScript(ctx context.Context, path core.AbsolutePath) ([]byte, core.ByteLength, error) {
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return nil, core.ByteLength{}, err
	}
	limit, err := core.NewByteCount(ScriptMaximumBytes)
	if err != nil {
		closeErr := location.Root.Close()
		return nil, core.ByteLength{}, errors.Join(core.ErrProjectStandardsContract, err, closeErr)
	}
	var script bytes.Buffer
	count, readErr := filestore.Read(ctx, filestore.ReadRequest{Destination: &script, Location: location, MaximumBytes: limit})
	closeErr := location.Root.Close()
	if closeErr != nil {
		closeErr = errors.Join(core.ErrFilestoreCleanup, closeErr)
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, core.ByteLength{}, err
	}
	return script.Bytes(), count, nil
}

func executionFact(input executionFactInput) (projectstandards.MachineProbeExecution, error) {
	exit, err := input.result.ExitCode()
	if err != nil {
		return projectstandards.MachineProbeExecution{}, err
	}
	exitCode, err := exit.Int()
	if err != nil {
		return projectstandards.MachineProbeExecution{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	exitCodeObservation, err := core.CheckedInt32FromInt(exitCode)
	if err != nil {
		return projectstandards.MachineProbeExecution{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	if exitCode != 0 {
		return projectstandards.MachineProbeExecution{}, newFailure(failureInput{kind: FailureExit, exitCode: exitCode, stderr: input.stderr, cause: core.ErrHostFactsObservation})
	}
	cpu, err := input.result.CPUTime()
	if err != nil {
		return projectstandards.MachineProbeExecution{}, err
	}
	fact := projectstandards.MachineProbeExecution{
		Bash: input.request.Bash, Script: input.request.Script, ScriptDigest: input.scriptDigest, ScriptBytes: input.scriptBytes, OutputLimit: input.limit,
		ExitCode: exitCodeObservation, CPUTime: cpu,
		StdoutDigest: core.SHA256Of(input.stdout),
		StderrDigest: core.SHA256Of(input.stderr),
	}
	fact.StdoutBytes, err = byteLength(input.stdout)
	if err != nil {
		return projectstandards.MachineProbeExecution{}, err
	}
	fact.StderrBytes, err = byteLength(input.stderr)
	if err != nil {
		return projectstandards.MachineProbeExecution{}, err
	}
	if err := fact.Validate(); err != nil {
		return projectstandards.MachineProbeExecution{}, err
	}
	return fact, nil
}

func newFailure(input failureInput) error {
	stderrSize, err := byteLength(input.stderr)
	if err != nil {
		return errors.Join(core.ErrHostFactsObservation, err)
	}
	failure := Failure{kind: input.kind, exitCode: input.exitCode, stderrHash: core.SHA256Of(input.stderr), stderrSize: stderrSize, cause: input.cause}
	if err := failure.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsObservation, err)
	}
	return failure
}

func byteLength(data []byte) (core.ByteLength, error) {
	length, err := core.NewByteLength(uint64(len(data)))
	if err != nil {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return length, nil
}
