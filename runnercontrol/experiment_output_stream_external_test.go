package runnercontrol_test

import (
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

type nativeExperimentCompiler uint8

const (
	nativeExperimentGo nativeExperimentCompiler = iota
	nativeExperimentJavaScript
	nativeExperimentSuite
	nativeExperimentTool
)

// The child uses native writes, independently of Primitive's stream accounting.
// Its process is reaped by the production process runner in the parent.
func TestExperimentOutputNativeChild(t *testing.T) {
	for index, argument := range os.Args {
		if argument != "--" || index+1 >= len(os.Args) {
			continue
		}
		extent, err := strconv.ParseUint(os.Args[index+1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var chunk [32 << 10]byte
		for index := range chunk {
			chunk[index] = byte(index % 251)
		}
		for remaining := extent; remaining != 0; {
			part := chunk[:min(remaining, uint64(len(chunk)))]
			for _, destination := range [...]*os.File{os.Stdout, os.Stderr} {
				if written, err := destination.Write(part); err != nil || written != len(part) {
					os.Exit(17)
				}
			}
			remaining -= uint64(len(part))
		}
		// Avoid the testing driver's PASS text becoming unrequested stream bytes.
		os.Exit(0)
	}
	t.Parallel()
}

type nativeExperimentDigest struct {
	digest hash.Hash
	count  uint64
}

func (w *nativeExperimentDigest) Write(data []byte) (int, error) {
	written, err := w.digest.Write(data)
	w.count += uint64(written)
	return written, err
}

func nativeCompiledExperiment(t testing.TB, compiler nativeExperimentCompiler, command, root core.AbsolutePath) process.Plan {
	t.Helper()
	if compiler == nativeExperimentGo {
		request := baseAcceptanceGoPlanRequest(t)
		request.Command, request.WorkingDirectory = command, root
		request.Subject.SourceRoot = root
		compiled, err := runnercontrol.CompileGoPlan(request)
		if err != nil {
			t.Fatal(err)
		}
		return compiled.Process
	}
	base := externalPlanBaseFixture(t, deniedExternalEgress())
	base.Command, base.WorkingDirectory, base.Subject.SourceRoot = command, root, root
	var compiled runnercontrol.ExperimentExecution
	var err error
	switch compiler {
	case nativeExperimentJavaScript:
		request := javaScriptPlanFixture(t)
		request.Base = base
		compiled, err = runnercontrol.CompileJavaScriptPlan(request)
	case nativeExperimentSuite:
		request := suitePlanFixture(t, deniedExternalEgress())
		request.Base = base
		compiled, err = runnercontrol.CompileSuitePlan(request)
	case nativeExperimentTool:
		request := toolPlanFixture(t)
		request.Base = base
		compiled, err = runnercontrol.CompileToolPlan(request)
	default:
		t.Fatalf("unknown native compiler %d", compiler)
	}
	if err != nil {
		t.Fatal(err)
	}
	return compiled.Process
}

func nativeExperimentCommand(t testing.TB, directory string, extent uint64) core.AbsolutePath {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "native-subject")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestExperimentOutputNativeChild$ -- " + strconv.FormatUint(extent, 10) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return mustProfileAbsolutePath(t, path)
}

func TestCompiledExperimentsConserveNativeOutputAcrossFormerCeiling(t *testing.T) {
	t.Parallel()
	for _, compiler := range [...]nativeExperimentCompiler{nativeExperimentGo, nativeExperimentJavaScript, nativeExperimentSuite, nativeExperimentTool} {
		for _, extent := range [...]uint64{0, 1, 65535, 65536, 65537, 8<<20 - 1, 8 << 20, 8<<20 + 1} {
			t.Run(strconv.Itoa(int(compiler))+"/"+strconv.FormatUint(extent, 10), func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root := mustProfileAbsolutePath(t, directory)
				plan := nativeCompiledExperiment(t, compiler, nativeExperimentCommand(t, directory, extent), root)
				var stdout = nativeExperimentDigest{digest: sha256.New()}
				var stderr = nativeExperimentDigest{digest: sha256.New()}
				request, err := plan.Bind(process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
				if err != nil {
					t.Fatal(err)
				}
				execution, beginErr := process.Begin(t.Context(), request)
				if beginErr != nil {
					t.Fatal(beginErr)
				}
				result, runErr := execution.Wait()
				if runErr != nil {
					runErr = errors.Join(runErr, execution.Sweep())
				}
				exit, exitErr := result.ExitCode()
				success, successErr := exit.Success()
				if runErr != nil || exitErr != nil || successErr != nil || !success || stdout.count != extent || stderr.count != extent {
					t.Fatalf("native output = stdout %d stderr %d, success %v, errors (%v,%v,%v); want %d bytes on each stream and reaped success", stdout.count, stderr.count, success, runErr, exitErr, successErr, extent)
				}
				oracle := sha256.New()
				var chunk [32 << 10]byte
				for index := range chunk {
					chunk[index] = byte(index % 251)
				}
				for remaining := extent; remaining != 0; {
					part := chunk[:min(remaining, uint64(len(chunk)))]
					_, _ = oracle.Write(part)
					remaining -= uint64(len(part))
				}
				want := [32]byte(oracle.Sum(nil))
				if [32]byte(stdout.digest.Sum(nil)) != want || [32]byte(stderr.digest.Sum(nil)) != want {
					t.Fatal("stream digests differ from the independently generated native byte sequence")
				}
			})
		}
	}
}

type nativeExperimentRefusal struct{}

func (nativeExperimentRefusal) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCompiledExperimentPreservesSinkRefusalAndReapsChild(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root := mustProfileAbsolutePath(t, directory)
	plan := nativeCompiledExperiment(t, nativeExperimentGo, nativeExperimentCommand(t, directory, 64<<20), root)
	request, err := plan.Bind(process.Streams{Stdin: strings.NewReader(""), Stdout: nativeExperimentRefusal{}, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	execution, beginErr := process.Begin(t.Context(), request)
	if beginErr != nil {
		t.Fatal(beginErr)
	}
	result, runErr := execution.Wait()
	if runErr != nil {
		runErr = errors.Join(runErr, execution.Sweep())
	}
	if !errors.Is(runErr, io.ErrClosedPipe) || result.Validate() != nil {
		t.Fatalf("sink refusal = result %+v error %v; want reaped result and io.ErrClosedPipe", result, runErr)
	}
}
