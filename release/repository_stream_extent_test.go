package release

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const nativeIndexRecord = "H native.bin\x00"

func nativeIndexRefusal(data []byte) error {
	for len(data) != 0 {
		record, tail, complete := bytes.Cut(data, []byte{0})
		if len(record) == 0 || record[0] != 'H' {
			return errRepositoryStatusObserved
		}
		if !complete || len(record) < 3 || record[1] != ' ' {
			return core.ErrReleaseContract
		}
		data = tail
	}
	return nil
}

func FuzzRepositoryIndexNativeFraming(f *testing.F) {
	for _, seed := range [...]string{"", nativeIndexRecord, "H a\x00H b\x00", "S hidden\x00", "H ", "H \x00", "H a\xff\x00", "Hx\x00"} {
		f.Add([]byte(seed), uint16(1))
	}
	f.Fuzz(func(t *testing.T, data []byte, width uint16) {
		writer := repositoryIndexWriter{}
		var gotErr error
		chunk := max(1, int(width))
		for offset := 0; offset < len(data); {
			end := offset + min(chunk, len(data)-offset)
			_, gotErr = writer.Write(data[offset:end])
			if gotErr != nil {
				break
			}
			offset = end
		}
		if gotErr == nil {
			gotErr = writer.finish()
		}
		wantErr := nativeIndexRefusal(data)
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("native index refusal = %v, want %v for %q", gotErr, wantErr, data)
		}
	})
}

func BenchmarkRepositoryIndexFixedBuffer(b *testing.B) {
	block := bytes.Repeat([]byte(nativeIndexRecord), 65536/len(nativeIndexRecord))
	b.ReportAllocs()
	b.SetBytes(int64(len(block)))
	for b.Loop() {
		writer := repositoryIndexWriter{}
		n, err := writer.Write(block)
		if err != nil || n != len(block) || writer.finish() != nil {
			b.Fatalf("native index write = (%d,%v), want (%d,nil) and closed frame", n, err, len(block))
		}
	}
}

func TestRepositoryIndexNativeChild(t *testing.T) {
	for index, argument := range os.Args {
		if argument != "--" || index+1 >= len(os.Args) {
			continue
		}
		count, err := strconv.ParseUint(os.Args[index+1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var block [65536 / len(nativeIndexRecord) * len(nativeIndexRecord)]byte
		for offset := 0; offset < len(block); offset += len(nativeIndexRecord) {
			copy(block[offset:], nativeIndexRecord)
		}
		for remaining := count; remaining != 0; {
			records := min(remaining, uint64(len(block)/len(nativeIndexRecord)))
			part := block[:records*uint64(len(nativeIndexRecord))]
			if n, err := os.Stdout.Write(part); err != nil || n != len(part) {
				os.Exit(17)
			}
			remaining -= records
		}
		os.Exit(0)
	}
	t.Parallel()
}

func TestRepositoryIndexProcessContinuesBeyondFormerByteCeiling(t *testing.T) {
	t.Parallel()
	const formerCeiling = 256 << 20
	for _, count := range [...]uint64{0, 1, uint64(formerCeiling / len(nativeIndexRecord)), uint64(formerCeiling/len(nativeIndexRecord) + 1)} {
		t.Run(strconv.FormatUint(count, 10), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "native-git")
			quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
			script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestRepositoryIndexNativeChild$ -- " + strconv.FormatUint(count, 10) + "\n"
			if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			root, rootErr := core.ParseAbsolutePath(directory)
			command, commandErr := core.ParseAbsolutePath(path)
			environment, environmentErr := process.ParseExactEnvironment(nil)
			wait, waitErr := temporal.DurationFromSeconds(5)
			if err := errors.Join(rootErr, commandErr, environmentErr, waitErr); err != nil {
				t.Fatal(err)
			}
			writer := &repositoryIndexWriter{}
			result, runErr := runRepositoryGit(t.Context(), repositoryGitRequest{
				verification: RepositoryVerificationRequest{Root: root, GitExecutable: command, Environment: environment, WaitDelay: wait},
				arguments:    []string{repositoryGitListFiles, repositoryGitVerboseFlags, repositoryGitNullTerminate},
				stdout:       writer, outputPolicy: repositoryGitOutputIndex,
			})
			observed, observedErr := result.StdoutBytes()
			if err := errors.Join(runErr, requireRepositoryGitSuccess(result), writer.finish(), observedErr); err != nil || observed.Uint64() != count*uint64(len(nativeIndexRecord)) {
				t.Fatalf("native index = (%d bytes,%v), want (%d,nil) with all records closed", observed.Uint64(), err, count*uint64(len(nativeIndexRecord)))
			}
		})
	}
}
