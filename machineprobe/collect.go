package machineprobe

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/about"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	OutputMaximumBytes uint64 = 256 * 1024
	ScriptMaximumBytes uint64 = 128 * 1024
)

type Request struct {
	ObservationID    about.MachineObservationID `json:"observation_id"`
	GenerationID     about.MachineGenerationID  `json:"generation_id"`
	ObservedAt       temporal.Instant           `json:"observed_at"`
	Collector        about.EvidenceAuthority    `json:"collector"`
	Bash             core.AbsolutePath          `json:"bash"`
	Script           core.AbsolutePath          `json:"script"`
	WorkingDirectory core.AbsolutePath          `json:"working_directory"`
	Environment      process.Environment        `json:"-"`
	WaitDelay        temporal.Duration          `json:"wait_delay"`
}

func (r Request) Validate() error {
	if err := errors.Join(r.ObservationID.Validate(), r.GenerationID.Validate(), r.ObservedAt.Validate(), r.Collector.Validate(), r.Bash.Validate(), r.Script.Validate(), r.WorkingDirectory.Validate(), r.Environment.Validate(), r.WaitDelay.Validate()); err != nil {
		return errors.Join(core.ErrAboutContract, err)
	}
	if r.WaitDelay.IsZero() {
		return errors.Join(core.ErrAboutContract, errors.New("machine probe wait delay is zero"))
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
		return errors.Join(core.ErrAboutContract, errors.New("machine probe failure kind is invalid"))
	}
	return nil
}

type Failure struct {
	kind       FailureKind
	exitCode   int
	stderrHash core.SHA256Digest
	stderrSize core.ByteLength
	cause      error
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
		return errors.Join(core.ErrAboutContract, errors.New("machine probe failure cause is absent"))
	}
	return errors.Join(f.stderrHash.Validate(), f.stderrSize.Validate())
}

func Collect(ctx context.Context, request Request) (about.MachineObservation, error) {
	if err := request.Validate(); err != nil {
		return about.MachineObservation{}, err
	}
	execution, stdout, stderr, err := run(ctx, request)
	if err != nil {
		return about.MachineObservation{}, err
	}
	var report about.MachineProbeReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		return about.MachineObservation{}, newFailure(FailureOutput, 0, stderr, errors.Join(core.ErrHostFactsEvidence, err))
	}
	fingerprint, err := report.Configuration.Fingerprint()
	if err != nil {
		return about.MachineObservation{}, newFailure(FailureOutput, 0, stderr, errors.Join(core.ErrHostFactsEvidence, err))
	}
	observation := about.MachineObservation{
		SchemaVersion: about.MachineProbeSchemaVersion,
		ID:            request.ObservationID, GenerationID: request.GenerationID, ObservedAt: request.ObservedAt,
		Collector: request.Collector, Execution: execution, Configuration: report.Configuration,
		Runtime: report.Runtime, Fingerprint: fingerprint,
	}
	if err := observation.Validate(); err != nil {
		return about.MachineObservation{}, newFailure(FailureOutput, 0, stderr, errors.Join(core.ErrHostFactsEvidence, err))
	}
	return observation, nil
}

func run(ctx context.Context, request Request) (about.MachineProbeExecution, []byte, []byte, error) {
	script, scriptBytes, err := readScript(ctx, request.Script)
	if err != nil {
		return about.MachineProbeExecution{}, nil, nil, err
	}
	if scriptBytes.Uint64() == 0 {
		return about.MachineProbeExecution{}, nil, nil, newFailure(FailureOutput, 0, nil, core.ErrHostFactsEvidence)
	}
	argument, err := process.NewArgument("-s")
	if err != nil {
		return about.MachineProbeExecution{}, nil, nil, errors.Join(core.ErrAboutContract, err)
	}
	limit, err := core.NewByteCount(OutputMaximumBytes)
	if err != nil {
		return about.MachineProbeExecution{}, nil, nil, errors.Join(core.ErrAboutContract, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result, err := process.Run(ctx, process.Request{
		Streams: process.Streams{Stdin: bytes.NewReader(script), Stdout: &stdout, Stderr: &stderr},
		Command: request.Bash, WorkingDirectory: request.WorkingDirectory, Arguments: []process.Argument{argument},
		Environment: request.Environment, OutputLimit: limit, WaitDelay: request.WaitDelay,
	})
	if err != nil {
		return about.MachineProbeExecution{}, nil, nil, err
	}
	execution, err := executionFact(request, core.SHA256Of(script), scriptBytes, limit, result, stdout.Bytes(), stderr.Bytes())
	if err != nil {
		return about.MachineProbeExecution{}, nil, nil, err
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
		return nil, core.ByteLength{}, errors.Join(core.ErrAboutContract, err, closeErr)
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

func executionFact(request Request, scriptDigest core.SHA256Digest, scriptBytes core.ByteLength, limit core.ByteCount, result process.Result, stdout, stderr []byte) (about.MachineProbeExecution, error) {
	exit, err := result.ExitCode()
	if err != nil {
		return about.MachineProbeExecution{}, err
	}
	exitCode, err := exit.Int()
	if err != nil || exitCode < math.MinInt32 || exitCode > math.MaxInt32 {
		return about.MachineProbeExecution{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	if exitCode != 0 {
		return about.MachineProbeExecution{}, newFailure(FailureExit, exitCode, stderr, core.ErrHostFactsObservation)
	}
	cpu, err := result.CPUTime()
	if err != nil {
		return about.MachineProbeExecution{}, err
	}
	fact := about.MachineProbeExecution{
		Bash: request.Bash, Script: request.Script, ScriptDigest: scriptDigest, ScriptBytes: scriptBytes, OutputLimit: limit,
		ExitCode: int32(exitCode), CPUTime: cpu,
		StdoutDigest: core.SHA256Of(stdout),
		StderrDigest: core.SHA256Of(stderr),
	}
	fact.StdoutBytes, err = byteLength(stdout)
	if err != nil {
		return about.MachineProbeExecution{}, err
	}
	fact.StderrBytes, err = byteLength(stderr)
	if err != nil {
		return about.MachineProbeExecution{}, err
	}
	if err := fact.Validate(); err != nil {
		return about.MachineProbeExecution{}, err
	}
	return fact, nil
}

func newFailure(kind FailureKind, exitCode int, stderr []byte, cause error) error {
	stderrSize, err := byteLength(stderr)
	if err != nil {
		return errors.Join(core.ErrHostFactsObservation, err)
	}
	failure := Failure{kind: kind, exitCode: exitCode, stderrHash: core.SHA256Of(stderr), stderrSize: stderrSize, cause: cause}
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
