package gitrepo

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func TestRepositoryIdentityConservesNativeRootBranchAndTag(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ordinary", "space 雪", "line\nbreak"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), name)
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			initializeGitRepository(t, dir)
			runGitSetup(t, dir, "-c", "user.name=Native proof", "-c", "user.email=native@example.invalid", "commit", "--allow-empty", "-m", "native identity")
			runGitSetup(t, dir, "checkout", "-b", "topic/雪")
			runGitSetup(t, dir, "tag", "proof/v1")
			child := filepath.Join(dir, "subdirectory")
			if err := os.Mkdir(child, 0o700); err != nil {
				t.Fatal(err)
			}
			c, request := statusFixtureCapability(t, child)
			root, err := c.ObserveRoot(t.Context(), RootRequest{Directory: request.Root})
			if err != nil || root.Root.String() != dir || root.Validate() != nil {
				t.Fatalf("root=%+v err=%v", root, err)
			}
			branch, err := c.ObserveBranch(t.Context(), BranchRequest{Root: root.Root})
			if err != nil || branch.Branch.String() != "topic/雪" || branch.Validate() != nil {
				t.Fatalf("branch=%+v err=%v", branch, err)
			}
			tag, err := c.ObserveExactTag(t.Context(), ExactTagRequest{Root: root.Root})
			if err != nil || tag.Tag.String() != "proof/v1" || tag.Validate() != nil {
				t.Fatalf("tag=%+v err=%v", tag, err)
			}
			runGitSetup(t, dir, "checkout", "--detach")
			branch, err = c.ObserveBranch(t.Context(), BranchRequest{Root: root.Root})
			if err != nil || branch.Branch.String() != "HEAD" {
				t.Fatalf("detached=%+v err=%v", branch, err)
			}
		})
	}
}

func TestRepositoryIdentityRefusesNativeExitWarningAndPartialOutput(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing", "missing-directory", "untagged", "canceled", "warning", "multiline", "unterminated", "oversized-multiline", "long-label"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			c, request := statusFixtureCapability(t, dir)
			ctx := t.Context()
			want := core.ErrGitRepositoryExecution
			switch mode {
			case "missing-directory":
				var err error
				request.Root, err = core.ParseAbsolutePath(filepath.Join(dir, "absent"))
				if err != nil {
					t.Fatal(err)
				}
			case "untagged":
				initializeGitRepository(t, dir)
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = core.ErrGitRepositoryContract
			case "warning", "multiline", "unterminated", "oversized-multiline", "long-label":
				want = core.ErrGitRepositoryOutput
				data := "topic\n"
				script := "#!/bin/sh\ncat output\n"
				switch mode {
				case "warning":
					script += "printf warning >&2\n"
				case "multiline":
					data += "foreign\n"
				case "unterminated":
					data = "topic"
				case "oversized-multiline":
					data = strings.Repeat("a", 4<<20+1) + "\nforeign\n"
				case "long-label":
					data = strings.Repeat("a", 65537) + "\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "output"), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
				command := filepath.Join(dir, "producer")
				if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
				var err error
				c.command, err = core.ParseAbsolutePath(command)
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := c.ObserveExactTag(ctx, ExactTagRequest{Root: request.Root})
			if mode == "long-label" {
				if err != nil || len(got.Tag.String()) != 65537 || got.Validate() != nil {
					t.Fatalf("extent=%d err=%v", len(got.Tag.String()), err)
				}
				return
			}
			if got != (ExactTagObservation{}) || !errors.Is(err, want) {
				t.Fatalf("fact=%+v err=%v want=%v", got, err, want)
			}
			if mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if mode == "missing-directory" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("native filesystem cause lost: %v", err)
			}
			if mode == "missing" || mode == "untagged" {
				var exit CommandExitError
				if !errors.As(err, &exit) || exit.Validate() != nil || exit.Process.ExitCode == 0 {
					t.Fatalf("native exit lost: %v", err)
				}
			}
		})
	}
}

func TestReferenceLabelByteDomainAgreesWithNativeGit(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		t.Run(strconv.Itoa(value), func(t *testing.T) { assertReferenceLabelNative(t, "topic"+string([]byte{byte(value)})+"end") })
	}
	for _, value := range []string{"", "@", "HEAD", "雪", "a/b", "a//b", ".hidden", "a.lock", "a..b", "a@{b", "/a", "a/", "a.", "-a", "a/.b"} {
		assertReferenceLabelNative(t, value)
	}
}

func assertReferenceLabelNative(t *testing.T, value string) {
	t.Helper()
	label, err := ParseReferenceLabel(value)
	// The independent Git executable owns the reference grammar. UTF-8 is the
	// explicit label contract; Git itself can also store non-UTF-8 names.
	valid := false
	if value != "" && value != "@" && !strings.ContainsRune(value, 0) {
		command := exec.Command("git", "check-ref-format", "refs/tags/"+value)
		valid = command.Run() == nil && utf8.ValidString(value)
	}
	if (err == nil) != valid {
		t.Fatalf("label=%q admitted=%v native=%v err=%v", value, err == nil, valid, err)
	}
	if valid && (label.String() != value || label.Validate() != nil) {
		t.Fatalf("label changed %q", value)
	}
	if !valid && (label != (ReferenceLabel{}) || !errors.Is(err, core.ErrGitRepositoryOutput)) {
		t.Fatalf("invalid label published: %v %v", label, err)
	}
}

func FuzzReferenceLabelConservesNativeGitGrammar(f *testing.F) {
	for _, value := range []string{"topic/雪", "HEAD", "a..b", "", "@", "a.lock"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) { assertReferenceLabelNative(t, value) })
}

func FuzzRepositoryIdentityNativeProducerConservesCompletedLabel(f *testing.F) {
	for _, data := range []string{"topic/雪\n", "HEAD\n", "foreign\nsecond\n", "unterminated", "\n", ""} {
		f.Add([]byte(data))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		c, request := statusFixtureCapability(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "output"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		command := filepath.Join(dir, "producer")
		if err := os.WriteFile(command, []byte("#!/bin/sh\ncat output\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		var err error
		c.command, err = core.ParseAbsolutePath(command)
		if err != nil {
			t.Fatal(err)
		}
		got, err := c.ObserveBranch(t.Context(), BranchRequest{Root: request.Root})
		text := string(data)
		valid := strings.HasSuffix(text, "\n") && strings.Count(text, "\n") == 1 && len(text) > 1
		if valid {
			text = strings.TrimSuffix(text, "\n")
			valid = text != "@" && !strings.ContainsRune(text, 0) && utf8.ValidString(text)
			if valid {
				valid = exec.Command("git", "check-ref-format", "refs/heads/"+text).Run() == nil
			}
		}
		if (err == nil) != valid {
			t.Fatalf("wire=%q admitted=%v native=%v err=%v", data, err == nil, valid, err)
		}
		if valid && (got.Branch.String() != text || got.Process.StdoutBytes.Uint64() != uint64(len(data)) || got.Validate() != nil) {
			t.Fatalf("native bytes changed: %+v wire=%q", got, data)
		}
		if !valid && got != (BranchObservation{}) {
			t.Fatalf("partial fact admitted: %+v", got)
		}
	})
}

func BenchmarkIdentityMalformedRecordWorkingWindow(b *testing.B) {
	for _, extent := range []int64{65537, 64 << 20} {
		b.Run(strconv.FormatInt(extent, 10), func(b *testing.B) {
			file, err := os.CreateTemp(b.TempDir(), "native-identity")
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := file.Close(); err != nil {
					b.Error(err)
				}
			})
			chunk := strings.Repeat("a", 65536)
			for remaining := extent; remaining > 0; {
				width := min(remaining, int64(len(chunk)))
				if _, err := io.WriteString(file, chunk[:width]); err != nil {
					b.Fatal(err)
				}
				remaining -= width
			}
			if _, err := io.WriteString(file, "\nforeign\n"); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				text, err := identityText(b.Context(), io.NewSectionReader(file, 0, extent+9), extent+9)
				if text != "" || !errors.Is(err, core.ErrGitRepositoryOutput) {
					b.Fatalf("malformed record admitted: %q %v", text, err)
				}
			}
		})
	}
}
