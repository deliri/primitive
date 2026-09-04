package gitrepo

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestGitWorktreeLayerTriad(t *testing.T) {
	t.Parallel()

	configuration, err := DefaultConfiguration()
	if err != nil {
		t.Fatalf("DefaultConfiguration() error = %v, want nil", err)
	}
	capability, err := Open(t.Context(), configuration)
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}

	t.Run("positive tracked and repository-unignored paths cross the real Git boundary", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		initializeGitRepository(t, directory)
		writeWorktreeFile(t, directory, ".gitignore", "generated.go\n")
		writeWorktreeFile(t, directory, "tracked.go", "package tracked\n")
		writeWorktreeFile(t, directory, "untracked.go", "package untracked\n")
		writeWorktreeFile(t, directory, "generated.go", "package generated\n")
		runGitSetup(t, directory, "add", "--", ".gitignore", "tracked.go")

		var got streamCapture
		summary, gotErr := capability.StreamWorktree(t.Context(), worktreeRequest(t, directory), &got)
		want := []string{".gitignore", "tracked.go", "untracked.go"}
		wantBytes := uint64(len(".gitignore") + len("tracked.go") + len("untracked.go") + 3)
		slices.Sort(got.paths)
		if gotErr != nil || !slices.Equal(got.paths, want) || summary.Entries != uint64(len(want)) || summary.Bytes.Uint64() != wantBytes {
			t.Fatalf("StreamWorktree(real repository) = (paths %q, summary %+v, %v), want (%q, %d entries/%d bytes, nil)", got.paths, summary, gotErr, want, len(want), wantBytes)
		}
	})

	t.Run("negative nonrepository root retains Git execution identity and emits no paths", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		var got streamCapture
		summary, gotErr := capability.StreamWorktree(t.Context(), worktreeRequest(t, directory), &got)
		if !errors.Is(gotErr, core.ErrGitRepositoryExecution) || len(got.paths) != 0 || summary != (WorktreeSummary{}) {
			t.Fatalf("StreamWorktree(nonrepository) = (paths %q, summary %+v, %v), want no paths, zero summary, and errors.Is(..., %v)", got.paths, summary, gotErr, core.ErrGitRepositoryExecution)
		}
	})

	t.Run("neutral empty repository emits a sealed zero summary without invented paths", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		initializeGitRepository(t, directory)
		var got streamCapture
		summary, gotErr := capability.StreamWorktree(t.Context(), worktreeRequest(t, directory), &got)
		if gotErr != nil || len(got.paths) != 0 || summary.Entries != 0 || summary.Bytes.Uint64() != 0 {
			t.Fatalf("StreamWorktree(empty repository) = (paths %q, summary %+v, %v), want no paths, zero summary, and nil", got.paths, summary, gotErr)
		}
	})
}

func TestWorktreeEntryWriterPressuresExternalGitPathStream(t *testing.T) {
	t.Parallel()

	consumerRefusal := errors.New("consumer refused provisional entry")
	longest := strings.Repeat("a", core.SourcePathMaximumBytes)
	tests := []struct {
		consumer  WorktreeConsumer
		wantErr   error
		name      string
		chunks    []string
		want      []string
		wantBytes uint64
	}{
		{name: "ordinary file is admitted", chunks: []string{"main.go\x00"}, want: []string{"main.go"}, wantBytes: 8},
		{name: "nested file is admitted", chunks: []string{"cmd/hammer/main.go\x00"}, want: []string{"cmd/hammer/main.go"}, wantBytes: 19},
		{name: "internal spaces remain exact", chunks: []string{"docs/design note.md\x00"}, want: []string{"docs/design note.md"}, wantBytes: 20},
		{name: "UTF-8 path remains exact", chunks: []string{"docs/café.go\x00"}, want: []string{"docs/café.go"}, wantBytes: 14},
		{name: "dot-prefixed path remains exact", chunks: []string{".github/workflows/gate.yml\x00"}, want: []string{".github/workflows/gate.yml"}, wantBytes: 27},
		{name: "dash-prefixed path remains an operand", chunks: []string{"-generated.go\x00"}, want: []string{"-generated.go"}, wantBytes: 14},
		{name: "case-distinct paths preserve lexical order", chunks: []string{"A.go\x00a.go\x00"}, want: []string{"A.go", "a.go"}, wantBytes: 10},
		{name: "multiple suffix periods remain exact", chunks: []string{"schema.test.golden.json\x00"}, want: []string{"schema.test.golden.json"}, wantBytes: 24},
		{name: "delimiter in its own write completes the record", chunks: []string{"split.go", "\x00"}, want: []string{"split.go"}, wantBytes: 9},
		{name: "UTF-8 code point split across writes remains exact", chunks: []string{"caf\xc3", "\xa9.go\x00"}, want: []string{"café.go"}, wantBytes: 9},
		{name: "empty record is refused", chunks: []string{"\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "repository root record is refused", chunks: []string{".\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "absolute path is refused", chunks: []string{"/main.go\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "parent traversal is refused", chunks: []string{"../main.go\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "native separator is refused", chunks: []string{"cmd\\main.go\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "line-delimited impostor is refused", chunks: []string{"main.go\nnext.go\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "invalid UTF-8 path is refused", chunks: []string{"bad\xff.go\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "duplicate observations remain exact for caller policy", chunks: []string{"same.go\x00same.go\x00"}, want: []string{"same.go", "same.go"}, wantBytes: 16},
		{name: "descending observations preserve Git stream order", chunks: []string{"z.go\x00a.go\x00"}, want: []string{"z.go", "a.go"}, wantBytes: 10},
		{name: "truncated final record is refused", chunks: []string{"partial.go"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "consumer refusal remains exact", chunks: []string{"main.go\x00"}, consumer: refusingConsumer{err: consumerRefusal}, wantErr: consumerRefusal},
		{name: "one-byte path crosses the lower length boundary", chunks: []string{"a\x00"}, want: []string{"a"}, wantBytes: 2},
		{name: "one below source ceiling is admitted", chunks: []string{longest[:len(longest)-1] + "\x00"}, want: []string{longest[:len(longest)-1]}, wantBytes: uint64(len(longest))},
		{name: "exact source ceiling is admitted", chunks: []string{longest + "\x00"}, want: []string{longest}, wantBytes: uint64(len(longest) + 1)},
		{name: "one above source ceiling is refused before allocation grows", chunks: []string{longest + "b\x00"}, wantErr: core.ErrGitRepositoryOutput},
		{name: "zero records preserve neutral accounting", chunks: nil, want: []string{}, wantBytes: 0},
		{name: "two records account for both delimiters", chunks: []string{"a\x00b\x00"}, want: []string{"a", "b"}, wantBytes: 4},
		{name: "three records split at record boundaries remain ordered", chunks: []string{"a\x00", "b\x00", "c\x00"}, want: []string{"a", "b", "c"}, wantBytes: 6},
		{name: "one-byte writes reconstruct the complete stream", chunks: []string{"a", "/", "b", ".", "g", "o", "\x00"}, want: []string{"a/b.go"}, wantBytes: 7},
		{name: "ascending adjacent bytes remain distinct", chunks: []string{"a\x00b\x00"}, want: []string{"a", "b"}, wantBytes: 4},
		{name: "prefix sorts before its descendant", chunks: []string{"a\x00a/b\x00"}, want: []string{"a", "a/b"}, wantBytes: 6},
		{name: "punctuation ordering remains byte lexical", chunks: []string{"a-b\x00a.b\x00a_b\x00"}, want: []string{"a-b", "a.b", "a_b"}, wantBytes: 12},
		{name: "upper lexical edge remains admitted", chunks: []string{"~\x00"}, want: []string{"~"}, wantBytes: 2},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			capture := &streamCapture{}
			consumer := WorktreeConsumer(capture)
			if testCase.consumer != nil {
				consumer = testCase.consumer
			}
			writer := &worktreeEntryWriter{consumer: consumer}
			var gotErr error
			for _, chunk := range testCase.chunks {
				_, gotErr = writer.Write([]byte(chunk))
				if gotErr != nil {
					break
				}
			}
			var summary WorktreeSummary
			if gotErr == nil {
				summary, gotErr = writer.finish()
			}
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) || summary != (WorktreeSummary{}) || !slices.Equal(capture.paths, testCase.want) {
					t.Fatalf("worktree stream = (paths %q, summary %+v, %v), want (%q, zero, errors.Is(..., %v))", capture.paths, summary, gotErr, testCase.want, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || !slices.Equal(capture.paths, testCase.want) || summary.Entries != uint64(len(testCase.want)) || summary.Bytes.Uint64() != testCase.wantBytes {
				t.Fatalf("worktree stream = (paths %q, summary %+v, %v), want (%q, %d entries/%d bytes, nil)", capture.paths, summary, gotErr, testCase.want, len(testCase.want), testCase.wantBytes)
			}
		})
	}
}

func TestExactGitEnvironmentRejectsAmbientGitControl(t *testing.T) {
	t.Parallel()

	ambient, err := process.ParseExactEnvironment([]string{
		"PATH=/usr/bin",
		"HOME=/example",
		"GIT_DIR=/attacker/repository",
		"GIT_CONFIG_GLOBAL=/attacker/config",
	})
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment(seed) error = %v, want nil", err)
	}
	got, gotErr := exactGitEnvironment(ambient)
	if gotErr != nil {
		t.Fatalf("exactGitEnvironment() error = %v, want nil", gotErr)
	}
	values, gotErr := got.Strings()
	if gotErr != nil {
		t.Fatalf("Environment.Strings() error = %v, want nil", gotErr)
	}
	for _, forbidden := range []string{"GIT_DIR=/attacker/repository", "GIT_CONFIG_GLOBAL=/attacker/config"} {
		if slices.Contains(values, forbidden) {
			t.Fatalf("exact Git environment contains %q = %t, want false", forbidden, true)
		}
	}
	for _, want := range []string{"PATH=/usr/bin", "HOME=/example", gitConfigNoSystem, gitConfigGlobal + os.DevNull, gitAttributesNoSystem, gitOptionalLocksOff, gitTerminalPromptOff} {
		if !slices.Contains(values, want) {
			t.Fatalf("exact Git environment contains %q = %t, want true; environment %q", want, false, values)
		}
	}
}

func FuzzWorktreeEntryWriterSemanticClosure(f *testing.F) {
	path, err := core.ParseSourcePath("cmd/hammer/main.go")
	if err != nil {
		f.Fatalf("core.ParseSourcePath(seed) error = %v, want nil", err)
	}
	f.Add([]byte(path.String() + "\x00"))
	f.Add([]byte{})
	f.Add([]byte("a\x00b\x00"))
	f.Add([]byte("b\x00a\x00"))
	f.Add([]byte("partial"))

	f.Fuzz(func(t *testing.T, data []byte) {
		capture := &streamCapture{}
		writer := &worktreeEntryWriter{consumer: capture}
		written, writeErr := writer.Write(data)
		summary, finishErr := writer.finish()
		gotErr := errors.Join(writeErr, finishErr)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitRepositoryOutput) || summary != (WorktreeSummary{}) || written > len(data) {
				t.Fatalf("worktree parser rejection = (written %d/%d, paths %q, summary %+v, %v), want bounded write, zero summary, and errors.Is(..., %v)", written, len(data), capture.paths, summary, gotErr, core.ErrGitRepositoryOutput)
			}
			return
		}
		if written != len(data) || summary.Entries != uint64(len(capture.paths)) || summary.Bytes.Uint64() != uint64(len(data)) {
			t.Fatalf("worktree parser acceptance = (written %d/%d, paths %q, summary %+v), want exact byte and entry conservation", written, len(data), capture.paths, summary)
		}
		var canonical bytes.Buffer
		for _, value := range capture.paths {
			path, pathErr := core.ParseSourcePath(value)
			if pathErr != nil || path.String() == "." {
				t.Fatalf("accepted path = (%q, %v), want canonical non-root source identity", value, pathErr)
			}
			canonical.WriteString(value)
			canonical.WriteByte(0)
		}
		if !bytes.Equal(canonical.Bytes(), data) {
			t.Fatalf("canonical worktree projection = %q, want accepted input %q", canonical.Bytes(), data)
		}
	})
}

type streamCapture struct{ paths []string }

func (c *streamCapture) ConsumeWorktreeEntry(entry WorktreeEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	c.paths = append(c.paths, entry.Path.String())
	return nil
}

type refusingConsumer struct{ err error }

func (c refusingConsumer) ConsumeWorktreeEntry(WorktreeEntry) error { return c.err }

func worktreeRequest(t testing.TB, directory string) WorktreeRequest {
	t.Helper()
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", directory, err)
	}
	return WorktreeRequest{Root: root, Selection: WorktreeSelectionTrackedAndUnignored}
}

func initializeGitRepository(t testing.TB, directory string) {
	t.Helper()
	runGitSetup(t, directory, "init", "--quiet")
}

func runGitSetup(t testing.TB, directory string, arguments ...string) {
	t.Helper()
	commandArguments := append([]string{"-C", directory}, arguments...)
	command := exec.CommandContext(t.Context(), "git", commandArguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git setup %q = (%q, %v), want nil", arguments, output, err)
	}
}

func writeWorktreeFile(t testing.TB, directory, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, path), []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}
