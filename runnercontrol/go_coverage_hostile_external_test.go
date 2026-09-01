package runnercontrol_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

type coverageCase struct {
	name         string
	class        string
	setup        func() [][]byte
	wantMode     runnercontrol.CoverageMode
	wantTotal    uint64
	wantCovered  uint64
	wantBasis    uint16
	wantErr      error
	wantWriteErr error
}

func TestGoCoverageCompilerHostileEvidenceMatrix(t *testing.T) {
	t.Parallel()

	cases := coverageCases()
	classes := map[string]int{}
	for index := range cases {
		classes[cases[index].class]++
	}
	if len(cases) != 40 || classes["valid"] != 10 || classes["rejection"] != 10 || classes["boundary"] != 20 {
		t.Fatalf("go coverage matrix classes = %v across %d cases, want valid:10 rejection:10 boundary:20 across 40 earned cases", classes, len(cases))
	}
	for index := range cases {
		tc := cases[index]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler := runnercontrol.NewGoCoverageCompiler()
			for chunkIndex, chunk := range tc.setup() {
				gotWritten, gotWriteErr := compiler.Write(chunk)
				if tc.wantWriteErr != nil {
					if !errors.Is(gotWriteErr, tc.wantWriteErr) || gotWritten >= len(chunk) {
						t.Fatalf("GoCoverageCompiler.Write(%s chunk %d) = (%d, %v), want partial write and errors.Is(..., %v)", tc.name, chunkIndex, gotWritten, gotWriteErr, tc.wantWriteErr)
					}
					break
				}
				if gotWriteErr != nil || gotWritten != len(chunk) {
					t.Fatalf("GoCoverageCompiler.Write(%s chunk %d) = (%d, %v), want (%d, nil) while retaining the full artifact", tc.name, chunkIndex, gotWritten, gotWriteErr, len(chunk))
				}
			}
			got, gotErr := compiler.Seal()
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (runnercontrol.GoCoverageObservation{}) {
					t.Fatalf("GoCoverageCompiler.Seal(%s) = (%+v, %v), want zero and errors.Is(..., %v)", tc.name, got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("GoCoverageCompiler.Seal(%s) error = %v, want nil", tc.name, gotErr)
			}
			if got.Mode != tc.wantMode || got.Statements != tc.wantTotal || got.Covered != tc.wantCovered || got.BasisPoints != tc.wantBasis {
				t.Fatalf("GoCoverageCompiler.Seal(%s) = mode %s/statements %d/covered %d/basis %d, want %s/%d/%d/%d", tc.name, got.Mode.String(), got.Statements, got.Covered, got.BasisPoints, tc.wantMode.String(), tc.wantTotal, tc.wantCovered, tc.wantBasis)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("GoCoverageCompiler.Seal(%s).Validate() error = %v, want nil", tc.name, err)
			}
		})
	}
}

func coverageCases() []coverageCase {
	valid := []coverageCase{
		coverageSuccess("set mode admits one covered statement", "valid", "mode: set\na.go:1.1,1.2 1 1\n", runnercontrol.CoverageSet, 1, 1),
		coverageSuccess("count mode admits execution counts above one", "valid", "mode: count\na.go:1.1,1.2 2 7\n", runnercontrol.CoverageCount, 2, 2),
		coverageSuccess("atomic mode admits concurrent count evidence", "valid", "mode: atomic\na.go:1.1,1.2 3 2\n", runnercontrol.CoverageAtomic, 3, 3),
		coverageSuccess("zero execution count remains uncovered", "valid", "mode: set\na.go:1.1,1.2 4 0\n", runnercontrol.CoverageSet, 4, 0),
		coverageSuccess("mixed records weight statement counts", "valid", "mode: set\na.go:1.1,1.2 2 1\na.go:2.1,2.2 3 0\n", runnercontrol.CoverageSet, 5, 2),
		coverageSuccess("different files share one exact total", "valid", "mode: atomic\na.go:1.1,1.2 2 1\nb.go:1.1,1.2 2 1\n", runnercontrol.CoverageAtomic, 4, 4),
		coverageSuccess("nested source coordinate remains one record", "valid", "mode: set\nexample.com/p/a.go:1.1,1.2 5 1\n", runnercontrol.CoverageSet, 5, 5),
		coverageSuccess("Windows-shaped source coordinate retains its colon", "valid", "mode: set\nC:/source/a.go:1.1,1.2 1 1\n", runnercontrol.CoverageSet, 1, 1),
		coverageSuccess("final record may omit trailing newline", "valid", "mode: count\na.go:1.1,1.2 2 1", runnercontrol.CoverageCount, 2, 2),
		coverageSuccess("large execution count does not inflate statements", "valid", "mode: atomic\na.go:1.1,1.2 6 18446744073709551615\n", runnercontrol.CoverageAtomic, 6, 6),
	}
	rejections := []coverageCase{
		coverageFailureCase("empty artifact has no mode or statements", "rejection", ""),
		coverageFailureCase("record before mode header is refused", "rejection", "a.go:1.1,1.2 1 1\n"),
		coverageFailureCase("unknown mode is refused", "rejection", "mode: future\na.go:1.1,1.2 1 1\n"),
		coverageFailureCase("mode without any record is vacuous", "rejection", "mode: set\n"),
		coverageFailureCase("empty record line is refused", "rejection", "mode: set\n\n"),
		coverageFailureCase("record missing execution count is refused", "rejection", "mode: set\na.go:1.1,1.2 1\n"),
		coverageFailureCase("zero statement extent is refused", "rejection", "mode: set\na.go:1.1,1.2 0 1\n"),
		coverageFailureCase("nonnumeric execution count is refused", "rejection", "mode: set\na.go:1.1,1.2 1 many\n"),
		coverageFailureCase("location without source separator is refused", "rejection", "mode: set\na.go 1 1\n"),
		coverageFailureCase("extra record field is refused", "rejection", "mode: set\na.go:1.1,1.2 1 1 extra\n"),
	}
	return append(append(valid, rejections...), coverageBoundaryCases()...)
}

func coverageBoundaryCases() []coverageCase {
	return []coverageCase{
		coverageSuccess("zero percent is represented exactly", "boundary", "mode: set\na.go:1.1,1.2 1 0\n", runnercontrol.CoverageSet, 1, 0),
		coverageSuccess("one hundred percent is represented exactly", "boundary", "mode: set\na.go:1.1,1.2 1 1\n", runnercontrol.CoverageSet, 1, 1),
		coverageSuccess("one third floors deterministically", "boundary", "mode: set\na.go:1.1,1.2 1 1\na.go:2.1,2.2 2 0\n", runnercontrol.CoverageSet, 3, 1),
		coverageSuccess("two thirds floors deterministically", "boundary", "mode: set\na.go:1.1,1.2 2 1\na.go:2.1,2.2 1 0\n", runnercontrol.CoverageSet, 3, 2),
		coverageSuccess("maximum uint32 statement record stays bounded", "boundary", "mode: count\na.go:1.1,1.2 4294967295 1\n", runnercontrol.CoverageCount, 4294967295, 4294967295),
		coverageSuccess("maximum uint32 uncovered record stays zero", "boundary", "mode: count\na.go:1.1,1.2 4294967295 0\n", runnercontrol.CoverageCount, 4294967295, 0),
		coverageSuccess("single-byte chunks preserve framing", "boundary", "mode: atomic\na.go:1.1,1.2 1 1\n", runnercontrol.CoverageAtomic, 1, 1, splitCoverageBytes),
		coverageSuccess("header and record chunks preserve state", "boundary", "mode: set\na.go:1.1,1.2 1 1\n", runnercontrol.CoverageSet, 1, 1, splitCoverageLines),
		coverageFailureCase("carriage return cannot change mode spelling", "boundary", "mode: set\r\na.go:1.1,1.2 1 1\n"),
		coverageFailureCase("leading mode whitespace is refused", "boundary", " mode: set\na.go:1.1,1.2 1 1\n"),
		coverageFailureCase("trailing mode whitespace is refused", "boundary", "mode: set \na.go:1.1,1.2 1 1\n"),
		coverageFailureCase("duplicate mode header cannot masquerade as a record", "boundary", "mode: set\nmode: set\na.go:1.1,1.2 1 1\n"),
		coverageFailureCase("negative statement count is refused", "boundary", "mode: set\na.go:1.1,1.2 -1 1\n"),
		coverageFailureCase("statement count above uint32 is refused", "boundary", "mode: set\na.go:1.1,1.2 4294967296 1\n"),
		coverageFailureCase("negative execution count is refused", "boundary", "mode: set\na.go:1.1,1.2 1 -1\n"),
		coverageSuccess("disjoint counts conserve exact arithmetic", "boundary", "mode: atomic\na.go:1.1,1.2 7 0\na.go:2.1,2.2 11 1\na.go:3.1,3.2 13 0\n", runnercontrol.CoverageAtomic, 31, 11),
		coverageSuccess("reordered independent records preserve totals", "boundary", "mode: atomic\nb.go:2.1,2.2 3 0\na.go:1.1,1.2 2 1\n", runnercontrol.CoverageAtomic, 5, 2),
		coverageSuccess("adjacent covered records add without deduplication", "boundary", "mode: count\na.go:1.1,1.2 1 1\na.go:1.1,1.2 1 1\n", runnercontrol.CoverageCount, 2, 2),
		coverageExactLineCase(),
		coverageAboveLineCase(),
	}
}

func coverageSuccess(name, class, input string, mode runnercontrol.CoverageMode, total, covered uint64, transforms ...func([]byte) [][]byte) coverageCase {
	setup := func() [][]byte { return [][]byte{[]byte(input)} }
	if len(transforms) == 1 {
		setup = func() [][]byte { return transforms[0]([]byte(input)) }
	}
	return coverageCase{name: name, class: class, setup: setup, wantMode: mode, wantTotal: total, wantCovered: covered, wantBasis: uint16(covered * 10_000 / total)}
}

func coverageFailureCase(name, class, input string) coverageCase {
	return coverageCase{name: name, class: class, setup: func() [][]byte { return [][]byte{[]byte(input)} }, wantErr: core.ErrPrimitiveContract}
}

func splitCoverageBytes(input []byte) [][]byte {
	chunks := make([][]byte, len(input))
	for index := range input {
		chunks[index] = []byte{input[index]}
	}
	return chunks
}

func splitCoverageLines(input []byte) [][]byte {
	index := strings.IndexByte(string(input), '\n') + 1
	return [][]byte{append([]byte(nil), input[:index]...), append([]byte(nil), input[index:]...)}
}

func coverageExactLineCase() coverageCase {
	prefix := "a"
	suffix := ":1.1,1.2 1 1\n"
	filler := strings.Repeat("x", int(runnercontrol.GoCoverageLineMaximumBytes)-len(prefix)-len(suffix)+1)
	input := "mode: set\n" + prefix + filler + suffix
	return coverageSuccess("record exactly at line byte ceiling is admitted", "boundary", input, runnercontrol.CoverageSet, 1, 1)
}

func coverageAboveLineCase() coverageCase {
	return coverageCase{
		name: "record one byte above line ceiling is refused", class: "boundary", wantErr: core.ErrPrimitiveContract, wantWriteErr: core.ErrJSONContract,
		setup: func() [][]byte {
			line := strings.Repeat("x", int(runnercontrol.GoCoverageLineMaximumBytes)+1)
			return [][]byte{[]byte(fmt.Sprintf("mode: set\n%s", line))}
		},
	}
}

func FuzzGoCoverageCompilerSemanticClosure(f *testing.F) {
	f.Add([]byte("mode: atomic\nexample.com/p/a.go:1.1,1.2 2 1\n"))
	f.Add([]byte{})
	f.Add([]byte("mode: future\n"))
	f.Add([]byte("mode: set\na.go:1.1,1.2 0 1\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		compiler := runnercontrol.NewGoCoverageCompiler()
		gotWritten, gotWriteErr := compiler.Write(data)
		if gotWriteErr != nil {
			if !errors.Is(gotWriteErr, core.ErrJSONContract) || gotWritten >= len(data) {
				t.Fatalf("GoCoverageCompiler.Write(fuzz input) = (%d, %v), want a typed partial-write refusal", gotWritten, gotWriteErr)
			}
			got, gotErr := compiler.Seal()
			if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != (runnercontrol.GoCoverageObservation{}) {
				t.Fatalf("GoCoverageCompiler.Seal(write-refused fuzz input) = (%+v, %v), want zero and typed refusal", got, gotErr)
			}
			return
		}
		if gotWritten != len(data) {
			t.Fatalf("GoCoverageCompiler.Write(fuzz input) = (%d, %v), want (%d, nil) so the raw artifact remains retainable", gotWritten, gotWriteErr, len(data))
		}
		got, gotErr := compiler.Seal()
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != (runnercontrol.GoCoverageObservation{}) {
				t.Fatalf("GoCoverageCompiler.Seal(rejected fuzz input) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("GoCoverageCompiler.Seal(accepted fuzz input).Validate() error = %v, want nil", err)
		}
		if got.Covered > got.Statements || uint64(got.BasisPoints) != got.Covered*10_000/got.Statements {
			t.Fatalf("GoCoverageCompiler.Seal(accepted fuzz input) = %+v, want conserved exact statement arithmetic", got)
		}
	})
}
