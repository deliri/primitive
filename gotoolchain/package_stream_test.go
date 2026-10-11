package gotoolchain

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const nativePackageJSON = `{"ImportPath":"fmt","Name":"fmt","Standard":true}` + "\n"

type nativePackageBytes struct{}

func (nativePackageBytes) ReadAt(data []byte, offset int64) (int, error) {
	for index := range data {
		data[index] = nativePackageJSON[(offset+int64(index))%int64(len(nativePackageJSON))]
	}
	return len(data), nil
}
func nativePackageSource(count uint64) io.Reader {
	return io.NewSectionReader(nativePackageBytes{}, 0, int64(count)*int64(len(nativePackageJSON)))
}

func TestPackageVisitorNativeChild(t *testing.T) {
	for index, argument := range os.Args {
		if argument != "--" || index+1 >= len(os.Args) {
			continue
		}
		count, err := strconv.ParseUint(os.Args[index+1], 10, 64)
		if err != nil {
			os.Exit(17)
		}
		if _, err := io.CopyBuffer(os.Stdout, nativePackageSource(count), make([]byte, 65536)); err != nil {
			os.Exit(18)
		}
		if _, err := io.CopyBuffer(os.Stderr, nativePackageSource(count), make([]byte, 65536)); err != nil {
			os.Exit(19)
		}
		os.Exit(0)
	}
}

func nativePackageCapability(t *testing.T, count uint64) (Capability, core.AbsolutePath) {
	t.Helper()
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "native-go")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestPackageVisitorNativeChild$ -- " + strconv.FormatUint(count, 10) + "\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	root, rootErr := core.ParseAbsolutePath(directory)
	command, commandErr := core.ParseAbsolutePath(path)
	environment, environmentErr := process.ParseExactEnvironment(nil)
	limits, limitsErr := DefaultLimits()
	if err := errors.Join(rootErr, commandErr, environmentErr, limitsErr); err != nil {
		t.Fatal(err)
	}
	return Capability{command: command, environment: environment, configuration: Configuration{Limits: limits, Workspace: WorkspaceModeDisabled, Environment: &environment}}, root
}

func TestPackageVisitorNativeExtentAndConsumerRefusal(t *testing.T) {
	t.Parallel()
	for _, count := range [...]uint64{0, 1, 4095, 4096, 4097, 131073} {
		t.Run(strconv.FormatUint(count, 10), func(t *testing.T) {
			t.Parallel()
			capability, root := nativePackageCapability(t, count)
			observed := uint64(0)
			err := capability.VisitPackages(t.Context(), PackageVisitRequest{Selection: ListRequest{WorkingDirectory: root, Pattern: "./..."}, Visit: func(_ context.Context, p Package) error {
				if p.ImportPath.String() != "fmt" || p.Name.String() != "fmt" || !p.Standard || p.Module != nil {
					t.Errorf("observed = %+v, want exact native fmt fact", p)
				}
				observed++
				return nil
			}})
			if err != nil || observed != count {
				t.Fatalf("native child visitor = (%d,%v), want (%d,nil)", observed, err, count)
			}
		})
	}
	t.Run("consumer refusal reaps a blocked child", func(t *testing.T) {
		t.Parallel()
		capability, root := nativePackageCapability(t, 1<<30)
		observed := uint64(0)
		err := capability.VisitPackages(t.Context(), PackageVisitRequest{Selection: ListRequest{WorkingDirectory: root, Pattern: "./..."}, Visit: func(context.Context, Package) error {
			observed++
			if observed == 4097 {
				return io.ErrClosedPipe
			}
			return nil
		}})
		if !errors.Is(err, io.ErrClosedPipe) || observed != 4097 {
			t.Fatalf("consumer refusal = (%d,%v), want (4097,io.ErrClosedPipe)", observed, err)
		}
	})
}

type nativePackageFailure struct{}

func (nativePackageFailure) Read([]byte) (int, error) {
	return 0, errors.Join(io.EOF, io.ErrUnexpectedEOF)
}

func TestPackageVisitorRetainsJoinedSourceFailure(t *testing.T) {
	t.Parallel()
	observed := 0
	err := visitPackageStream(t.Context(), io.MultiReader(nativePackageSource(1), nativePackageFailure{}), func(context.Context, Package) error { observed++; return nil })
	if observed != 1 || !errors.Is(err, io.ErrUnexpectedEOF) || !errors.Is(err, core.ErrGoToolchainOutput) {
		t.Fatalf("source closure = (%d,%v), want one fact and both native and Core refusal", observed, err)
	}
}

func FuzzPackageVisitorNativeCountAndRefusal(f *testing.F) {
	for _, seed := range [...][2]uint16{{0, 0}, {1, 0}, {4096, 0}, {4097, 4097}, {65535, 65535}} {
		f.Add(seed[0], seed[1], false)
	}
	f.Fuzz(func(t *testing.T, count, refuse uint16, sourceFailure bool) {
		source := nativePackageSource(uint64(count))
		if sourceFailure {
			source = io.MultiReader(source, nativePackageFailure{})
		}
		observed := uint64(0)
		err := visitPackageStream(t.Context(), source, func(_ context.Context, p Package) error {
			if p.ImportPath.String() != "fmt" || p.Name.String() != "fmt" || !p.Standard || p.Module != nil {
				t.Fatalf("native observation = %+v, want fmt", p)
			}
			observed++
			if refuse != 0 && observed == uint64(refuse) {
				return io.ErrClosedPipe
			}
			return nil
		})
		want := uint64(count)
		var wantErr error
		if refuse != 0 && refuse <= count {
			want = uint64(refuse)
			wantErr = io.ErrClosedPipe
		} else if sourceFailure {
			wantErr = io.ErrUnexpectedEOF
		}
		if observed != want || !errors.Is(err, wantErr) {
			t.Fatalf("visitor = (%d,%v), want (%d,%v)", observed, err, want, wantErr)
		}
	})
}

func BenchmarkPackageVisitorFixedObservation(b *testing.B) {
	for _, count := range [...]uint64{1, 4097, 131073} {
		b.Run(strconv.FormatUint(count, 10), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(count) * int64(len(nativePackageJSON)))
			for b.Loop() {
				observed := uint64(0)
				err := visitPackageStream(b.Context(), nativePackageSource(count), func(context.Context, Package) error { observed++; return nil })
				if err != nil || observed != count {
					b.Fatalf("visitor = (%d,%v), want (%d,nil)", observed, err, count)
				}
			}
		})
	}
}
