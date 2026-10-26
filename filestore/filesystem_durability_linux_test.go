//go:build linux && (amd64 || arm64)

package filestore

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

type filesystemDurabilityCase uint8

const (
	filesystemJoinedWriter filesystemDurabilityCase = iota + 1
	filesystemEmptyUse
	filesystemOpaqueLinks
	filesystemRenamedDirectory
	filesystemUnlinkedDirectory
	filesystemCallbackRefusal
	filesystemCallbackCancellation
	filesystemBeforeCancellation
	filesystemNativeDescriptorFault
	filesystemCallbackAndCloseFault
	filesystemCallbackPanic
	filesystemAfterAcknowledgementCancellation
	filesystemCaseLimit
)

// The isolated child owns every descriptor involved in this proof. strace
// observes native acknowledgements, not replacements for production calls.
func TestFilesystemDurabilityNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name             string
		mode             filesystemDurabilityCase
		wantSync         bool
		wantAcknowledged bool
	}{
		{name: "joined writer precedes native filesystem acknowledgement", mode: filesystemJoinedWriter, wantSync: true, wantAcknowledged: true},
		{name: "empty use still acknowledges the requested filesystem", mode: filesystemEmptyUse, wantSync: true, wantAcknowledged: true},
		{name: "dangling external and cyclic links remain opaque", mode: filesystemOpaqueLinks, wantSync: true, wantAcknowledged: true},
		{name: "renaming and replacing the pathname retains the original descriptor", mode: filesystemRenamedDirectory, wantSync: true, wantAcknowledged: true},
		{name: "unlinked directory stays held through acknowledgement", mode: filesystemUnlinkedDirectory, wantSync: true, wantAcknowledged: true},
		{name: "callback refusal closes without filesystem writeback", mode: filesystemCallbackRefusal},
		{name: "callback cancellation closes without filesystem writeback", mode: filesystemCallbackCancellation},
		{name: "cancelled caller never enters use", mode: filesystemBeforeCancellation},
		{name: "native descriptor fault retains sync and close errors", mode: filesystemNativeDescriptorFault, wantSync: true},
		{name: "callback refusal retains the native close failure", mode: filesystemCallbackAndCloseFault},
		{name: "callback panic propagates through descriptor closure", mode: filesystemCallbackPanic},
		{name: "cancellation after native acknowledgement remains terminal", mode: filesystemAfterAcknowledgementCancellation, wantSync: true, wantAcknowledged: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			trace := filepath.Join(directory, "native.trace")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := newNativeBackstop(t.Context(), t, 2*time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, "strace", "-f", "-qq", "-yy", "-e", "trace=openat,openat2,close,write,syncfs", "-o", trace, executable, "-test.run=^TestFilesystemDurabilityNativeChild$", "-test.v", "--", strconv.Itoa(int(row.mode)), trace)
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Run(); err != nil {
				t.Fatalf("native durability child = %v, output %q; want joined success", err, output.String())
			}
			data, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("native child output:\n%s\nnative syscall trace:\n%s", output.Bytes(), data)
			lines := strings.Split(string(data), "\n")
			use, acknowledgement, returned, closed, attempts := -1, -1, -1, -1, 0
			fd := -1
			if match := regexp.MustCompile(`filesystem-scope-use-fd=([0-9]+)`).FindSubmatch(output.Bytes()); match != nil {
				fd, err = strconv.Atoi(string(match[1]))
				if err != nil {
					t.Fatal(err)
				}
			}
			for index, line := range lines {
				if strings.Contains(line, "filesystem-scope-use-done") {
					use = index
				}
				if strings.Contains(line, "filesystem-scope-returned") {
					returned = index
				}
				if strings.Contains(line, "syncfs(") {
					attempts++
					if !strings.Contains(line, "syncfs("+strconv.Itoa(fd)) {
						t.Fatalf("syncfs descriptor trace = %q, want held descriptor %d", line, fd)
					}
					if strings.Contains(line, "= 0") {
						acknowledgement = index
					}
					if row.mode == filesystemRenamedDirectory && !strings.Contains(line, "held-moved") {
						t.Fatalf("renamed descriptor trace = %q, want original held-moved inode", line)
					}
					if row.mode == filesystemUnlinkedDirectory && !strings.Contains(line, "deleted") {
						t.Fatalf("unlinked descriptor trace = %q, want native deleted-directory descriptor", line)
					}
				}
				if use >= 0 && closed < 0 && fd >= 0 && strings.Contains(line, "close("+strconv.Itoa(fd)+"<") && strings.Contains(line, "held") {
					closed = index
				}
			}
			if (attempts == 1) != row.wantSync || attempts > 1 || (acknowledgement >= 0) != row.wantAcknowledged {
				t.Fatalf("native sync attempts=%d acknowledgement=%d, want sync=%t acknowledged=%t", attempts, acknowledgement, row.wantSync, row.wantAcknowledged)
			}
			if row.wantAcknowledged && !(use >= 0 && acknowledgement > use && closed > acknowledgement && returned > closed) {
				t.Fatalf("native ordering use=%d sync=%d close=%d return=%d, want joined Use then acknowledged sync then close then return", use, acknowledgement, closed, returned)
			}
			if returned < 0 {
				t.Fatalf("native return marker=%d, want completed caller observation", returned)
			}
		})
	}
}

// This context cancels a real child context only after strace has recorded the
// kernel's successful syncfs return. It exercises the post-acknowledgement
// context boundary without a sleep or a replacement synchronization function.
type filesystemAcknowledgementContext struct {
	context.Context
	cancel context.CancelCauseFunc
	trace  string
}

func (ctx filesystemAcknowledgementContext) Err() error {
	data, _ := os.ReadFile(ctx.trace)
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if bytes.Contains(line, []byte("syncfs(")) && bytes.Contains(line, []byte("= 0")) {
			ctx.cancel(core.ErrFilestoreConflict)
		}
	}
	return ctx.Context.Err()
}

func TestFilesystemDurabilityNativeChild(t *testing.T) {
	t.Parallel()
	arguments := flag.Args()
	if len(arguments) == 0 {
		t.Skip("owned native child invoked only by its parent proof")
	}
	if len(arguments) != 2 {
		t.Fatalf("child arguments=%q, want mode and native trace", arguments)
	}
	raw, err := strconv.ParseUint(arguments[0], 10, 8)
	mode := filesystemDurabilityCase(raw)
	if err != nil || mode < filesystemJoinedWriter || mode >= filesystemCaseLimit {
		t.Fatalf("child mode=%d/%v, want admitted native fixture", raw, err)
	}
	directory := t.TempDir()
	held := filepath.Join(directory, "held")
	if err := os.Mkdir(held, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path, err := core.ParseRelativePath("held")
	if err != nil {
		t.Fatal(err)
	}
	base, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	var ctx context.Context = base
	if mode == filesystemBeforeCancellation {
		cancel(core.ErrFilestoreConflict)
	}
	if mode == filesystemAfterAcknowledgementCancellation {
		ctx = filesystemAcknowledgementContext{Context: base, cancel: cancel, trace: arguments[1]}
	}
	fd, uses := -1, 0
	var observedErr error
	func() {
		defer func() {
			if cause := recover(); cause != nil {
				failure, ok := cause.(error)
				if mode != filesystemCallbackPanic || !ok || !errors.Is(failure, core.ErrFilestoreConflict) {
					panic(cause)
				}
				observedErr = failure
			}
		}()
		observedErr = WithFilesystemDurability(ctx, FilesystemDurabilityRequest{Location: Location{Root: root, Path: path}, Use: func(ctx context.Context) error {
			uses++
			fd = filesystemOwnedDescriptor(t, held)
			fmt.Fprintf(os.Stdout, "filesystem-scope-use-fd=%d\n", fd)
			if mode == filesystemJoinedWriter {
				command, parseErr := core.ParseAbsolutePath("/bin/sh")
				working, workErr := core.ParseAbsolutePath(held)
				if parseErr != nil || workErr != nil {
					return errors.Join(parseErr, workErr)
				}
				var argv []process.Argument
				for _, text := range []string{"-c", "printf joined-native-writer > payload"} {
					argument, parseErr := process.NewArgument(text)
					if parseErr != nil {
						return parseErr
					}
					argv = append(argv, argument)
				}
				environment, parseErr := process.ParseExactEnvironment(nil)
				if parseErr != nil {
					return parseErr
				}
				result, runErr := process.Run(ctx, process.Request{Command: command, WorkingDirectory: working, Arguments: argv, Environment: environment, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard}})
				if runErr != nil {
					return runErr
				}
				exit, exitErr := result.ExitCode()
				code, codeErr := exit.Int()
				if exitErr != nil || codeErr != nil || code != 0 {
					return errors.Join(exitErr, codeErr, core.ErrFilestoreSource)
				}
			} else if mode != filesystemEmptyUse {
				if err := os.WriteFile(filepath.Join(held, "payload"), []byte("retained-native-bytes"), 0o600); err != nil {
					return err
				}
			}
			switch mode {
			case filesystemOpaqueLinks:
				for _, pair := range [][2]string{{"missing", "dangling"}, {"/primitive-unowned-missing/target", "external"}, {".", "cycle"}} {
					if err := os.Symlink(pair[0], filepath.Join(held, pair[1])); err != nil {
						return err
					}
				}
			case filesystemRenamedDirectory:
				if err := os.Rename(held, held+"-moved"); err != nil {
					return err
				}
				if err := os.Mkdir(held, 0o700); err != nil {
					return err
				}
			case filesystemUnlinkedDirectory:
				if err := os.RemoveAll(held); err != nil {
					return err
				}
			case filesystemCallbackCancellation:
				cancel(core.ErrFilestoreConflict)
			case filesystemNativeDescriptorFault, filesystemCallbackAndCloseFault:
				if err := syscall.Close(fd); err != nil {
					return err
				}
			case filesystemJoinedWriter, filesystemEmptyUse, filesystemCallbackRefusal, filesystemCallbackPanic, filesystemAfterAcknowledgementCancellation:
			case filesystemBeforeCancellation, filesystemCaseLimit:
				return core.ErrFilestoreContract
			default:
				return core.ErrFilestoreContract
			}
			fmt.Fprintln(os.Stdout, "filesystem-scope-use-done")
			if mode == filesystemCallbackPanic {
				panic(core.ErrFilestoreConflict)
			}
			if mode == filesystemCallbackRefusal || mode == filesystemCallbackAndCloseFault {
				return core.ErrFilestoreConflict
			}
			return nil
		}})
	}()
	fmt.Fprintln(os.Stdout, "filesystem-scope-returned")
	if mode == filesystemBeforeCancellation {
		if uses != 0 {
			t.Fatalf("precancelled callback uses=%d, want zero", uses)
		}
	} else if uses != 1 {
		t.Fatalf("callback uses=%d, want one owned native population", uses)
	}
	switch mode {
	case filesystemCallbackRefusal, filesystemCallbackPanic:
		if !errors.Is(observedErr, core.ErrFilestoreConflict) {
			t.Fatalf("callback cause=%v, want preserved typed refusal", observedErr)
		}
	case filesystemCallbackCancellation, filesystemBeforeCancellation, filesystemAfterAcknowledgementCancellation:
		if !errors.Is(observedErr, context.Canceled) || !errors.Is(observedErr, core.ErrFilestoreConflict) {
			t.Fatalf("cancelled scope=%v, want Go cancellation and native context cause", observedErr)
		}
	case filesystemNativeDescriptorFault:
		if !errors.Is(observedErr, core.ErrFilestoreActivation) || !errors.Is(observedErr, core.ErrFilestoreCleanup) || !errors.Is(observedErr, syscall.EBADF) {
			t.Fatalf("faulted native scope=%v, want sync, close and EBADF causes", observedErr)
		}
	case filesystemCallbackAndCloseFault:
		if !errors.Is(observedErr, core.ErrFilestoreConflict) || !errors.Is(observedErr, core.ErrFilestoreCleanup) || !errors.Is(observedErr, syscall.EBADF) || errors.Is(observedErr, core.ErrFilestoreActivation) {
			t.Fatalf("refused faulted scope=%v, want callback and close causes without sync", observedErr)
		}
	case filesystemJoinedWriter, filesystemEmptyUse, filesystemOpaqueLinks, filesystemRenamedDirectory, filesystemUnlinkedDirectory:
		if observedErr != nil {
			t.Fatalf("native scope=%v, want acknowledged success", observedErr)
		}
	case filesystemCaseLimit:
		t.Fatalf("child mode=%d, want admitted native fixture", mode)
	default:
		t.Fatalf("child mode=%d, want admitted native fixture", mode)
	}
	if fd >= 0 {
		var info syscall.Stat_t
		if err := syscall.Fstat(fd, &info); !errors.Is(err, syscall.EBADF) {
			t.Fatalf("owned descriptor after scope=%+v/%v, want EBADF after native close", info, err)
		}
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatalf("borrowed root after scope=%v, want still owned by caller", err)
	}
	if mode == filesystemJoinedWriter {
		data, err := os.ReadFile(filepath.Join(held, "payload"))
		if err != nil || string(data) != "joined-native-writer" {
			t.Fatalf("joined child bytes=%q/%v, want exact completed writer output", data, err)
		}
	}
	if mode == filesystemOpaqueLinks {
		for _, pair := range [][2]string{{"missing", "dangling"}, {"/primitive-unowned-missing/target", "external"}, {".", "cycle"}} {
			target, err := os.Readlink(filepath.Join(held, pair[1]))
			if err != nil || target != pair[0] {
				t.Fatalf("opaque link %s=%q/%v, want exact target %q", pair[1], target, err, pair[0])
			}
		}
	}
}

func filesystemOwnedDescriptor(t testing.TB, directory string) int {
	t.Helper()
	want, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	found := -1
	for _, entry := range entries {
		got, err := os.Stat(filepath.Join("/proc/self/fd", entry.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(want, got) {
			if found >= 0 {
				t.Fatalf("held directory descriptors=%d and %s, want exactly one acquired scope handle", found, entry.Name())
			}
			found, err = strconv.Atoi(entry.Name())
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if found < 0 {
		t.Fatalf("held directory descriptor=%d, want native acquisition before callback", found)
	}
	return found
}

// FilesystemDurabilityNativeProofForTest binds the public capability inventory
// without adding a production hook or replacing the native acknowledgement.
func FilesystemDurabilityNativeProofForTest(t *testing.T) {
	t.Helper()
	t.Run("native", TestFilesystemDurabilityNativeLayerTriad)
	t.Run("admission", TestFilesystemDurabilityAdmission)
}

func FuzzFilesystemDurabilityNativeCustody(f *testing.F) {
	for _, seed := range []struct {
		data []byte
		mode uint8
	}{
		{[]byte("completed bytes"), 0}, {nil, 0}, {[]byte{0, 255, 1}, 1}, {[]byte("cancelled"), 2},
	} {
		f.Add(seed.data, seed.mode)
	}
	f.Fuzz(func(t *testing.T, data []byte, raw uint8) {
		directory := t.TempDir()
		held := filepath.Join(directory, "held")
		if err := os.Mkdir(held, 0o700); err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		path, err := core.ParseRelativePath("held")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		mode := raw % 3
		uses, fd := 0, -1
		got := WithFilesystemDurability(ctx, FilesystemDurabilityRequest{Location: Location{Root: root, Path: path}, Use: func(context.Context) error {
			uses++
			fd = filesystemOwnedDescriptor(t, held)
			if err := os.WriteFile(filepath.Join(held, "payload"), data, 0o600); err != nil {
				return err
			}
			if err := os.Symlink("missing", filepath.Join(held, "link")); err != nil {
				return err
			}
			if mode == 1 {
				return core.ErrFilestoreConflict
			}
			if mode == 2 {
				cancel(core.ErrFilestoreConflict)
			}
			return nil
		}})
		if uses != 1 {
			t.Fatalf("callback uses=%d, want one native scope", uses)
		}
		if mode == 0 && got != nil {
			t.Fatalf("completed scope=%v, want success", got)
		}
		if mode != 0 && !errors.Is(got, core.ErrFilestoreConflict) {
			t.Fatalf("failed scope=%v, want original refusal cause", got)
		}
		if (mode == 2) != errors.Is(got, context.Canceled) {
			t.Fatalf("cancelled scope=%v, want cancellation exactly for terminal callback", got)
		}
		observed, err := os.ReadFile(filepath.Join(held, "payload"))
		if err != nil || !bytes.Equal(observed, data) {
			t.Fatalf("native payload=%q/%v, want exact input %q", observed, err, data)
		}
		target, err := os.Readlink(filepath.Join(held, "link"))
		if err != nil || target != "missing" {
			t.Fatalf("opaque link=%q/%v, want missing", target, err)
		}
		var stat syscall.Stat_t
		if err := syscall.Fstat(fd, &stat); !errors.Is(err, syscall.EBADF) {
			t.Fatalf("owned fd after scope=%v, want native closure", err)
		}
		if _, err := root.Stat("."); err != nil {
			t.Fatalf("borrowed root=%v, want caller ownership", err)
		}
	})
}
