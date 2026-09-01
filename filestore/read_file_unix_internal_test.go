//go:build darwin || linux

package filestore

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

// nonblockingOpenBackstop bounds only a wedged open. A correct acquisition
// returns in microseconds, so expiry means the descriptor was parked waiting
// for a peer rather than that the machine is slow.
const nonblockingOpenBackstop = 30 * time.Second

const namedPipeOpenProbeRootEnvironment = "PRIMITIVE_FILESTORE_NAMED_PIPE_OPEN_PROBE_ROOT"

const (
	prepareRegularReadFileName  = "prepareRegularReadFile"
	fileDescriptorMethodName    = "Fd"
	syscallConnectionMethodName = "SyscallConn"
)

// TestOpenReadFileAcquiresNamedPipeWithoutBlocking proves the mechanism the
// stat-then-open preflight could not provide: the open itself never parks. A
// blocking open of a writerless FIFO waits forever, so a build that drops
// O_NONBLOCK from the read flags expires this backstop instead of passing.
func TestOpenReadFileAcquiresNamedPipeWithoutBlocking(t *testing.T) {
	if rootDirectory := os.Getenv(namedPipeOpenProbeRootEnvironment); rootDirectory != "" {
		runNamedPipeOpenProbe(t, rootDirectory)
		return
	}
	t.Parallel()

	rootDirectory := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(rootDirectory, "source"), 0o600); err != nil {
		t.Fatalf("Mkfifo(source) error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), nonblockingOpenBackstop)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOpenReadFileAcquiresNamedPipeWithoutBlocking$")
	command.Env = append(os.Environ(), namedPipeOpenProbeRootEnvironment+"="+rootDirectory)
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("openReadFile(named pipe) did not return within %v: the open is blocking", nonblockingOpenBackstop)
		}
		t.Fatalf("named-pipe open probe error = %v, want nil", err)
	}
}

func runNamedPipeOpenProbe(t *testing.T, rootDirectory string) {
	t.Helper()

	root := openInternalTestRoot(t, rootDirectory)
	file, err := openReadFile(root, "source")
	if err != nil {
		t.Fatalf("openReadFile(named pipe) error = %v, want nil", err)
	}
	closeInternalTestFile(t, file)
}

// TestOpenReadFileRequestsNonblockingAcquisition pins the flag that removes the
// race. Without it the regular-file proof would still pass while the open it
// guards could park on an identity swapped in after the check.
func TestOpenReadFileRequestsNonblockingAcquisition(t *testing.T) {
	t.Parallel()

	root, path := internalTestRegularFile(t, t.TempDir(), "source", "payload")
	file, err := openReadFile(root, path)
	if err != nil {
		t.Fatalf("openReadFile(regular) error = %v, want nil", err)
	}
	defer closeInternalTestFile(t, file)

	if flags := descriptorStatusFlags(t, file); flags&unix.O_NONBLOCK == 0 {
		t.Fatalf("openReadFile(regular) status flags = %#x, want O_NONBLOCK (%#x) set", flags, unix.O_NONBLOCK)
	}
}

// TestOpenRegularReadFileRestoresBlockingMode proves the acquired handle is
// handed to the streaming copy under the ordinary read contract, so the
// nonblocking acquisition is not observable past the identity proof.
func TestOpenRegularReadFileRestoresBlockingMode(t *testing.T) {
	t.Parallel()

	root, path := internalTestRegularFile(t, t.TempDir(), "source", "payload")
	file, err := openRegularReadFile(root, path)
	if err != nil {
		t.Fatalf("openRegularReadFile(regular) error = %v, want nil", err)
	}
	defer closeInternalTestFile(t, file)

	if flags := descriptorStatusFlags(t, file); flags&unix.O_NONBLOCK != 0 {
		t.Fatalf("openRegularReadFile(regular) status flags = %#x, want O_NONBLOCK (%#x) cleared", flags, unix.O_NONBLOCK)
	}
}

func TestMutableRegularHandlesRestoreBlockingModeBeforeOwnershipTransfer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		existing bool
		open     func(*testing.T, *os.Root, core.RelativePath) (*os.File, error)
	}{
		{name: "existing append handle", existing: true, open: func(t *testing.T, root *os.Root, path core.RelativePath) (*os.File, error) {
			return OpenAppend(t.Context(), AppendRequest{
				Location: Location{Root: root, Path: path}, Mode: 0o600, Append: AppendExisting,
			})
		}},
		{name: "new append handle", open: func(t *testing.T, root *os.Root, path core.RelativePath) (*os.File, error) {
			return OpenAppend(t.Context(), AppendRequest{
				Location: Location{Root: root, Path: path}, Mode: 0o600, Append: AppendCreate,
			})
		}},
		{name: "lock handle", existing: true, open: func(t *testing.T, root *os.Root, path core.RelativePath) (*os.File, error) {
			return OpenLockFile(t.Context(), LockFileRequest{Location: Location{Root: root, Path: path}, Mode: 0o600})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var root *os.Root
			text := "source"
			if tc.existing {
				root, text = internalTestRegularFile(t, t.TempDir(), text, "payload")
			} else {
				root = openInternalTestRoot(t, t.TempDir())
			}
			path, err := core.ParseRelativePath(text)
			if err != nil {
				t.Fatalf("core.ParseRelativePath(%q) error = %v, want nil", text, err)
			}
			file, err := tc.open(t, root, path)
			if err != nil {
				t.Fatalf("open mutable regular handle error = %v, want nil", err)
			}
			defer closeInternalTestFile(t, file)
			if flags := descriptorStatusFlags(t, file); flags&unix.O_NONBLOCK != 0 {
				t.Fatalf("mutable regular handle status flags = %#x, want O_NONBLOCK (%#x) cleared", flags, unix.O_NONBLOCK)
			}
		})
	}
}

// TestPrepareRegularReadFileKeepsDescriptorInsideSyscallConn ratchets the
// compiler-visible mechanism that a regular-file read cannot expose
// behaviorally. File.Fd permanently disables the handle's deadline methods;
// the effect leaf must instead keep descriptor access inside SyscallConn.
func TestPrepareRegularReadFileKeepsDescriptorInsideSyscallConn(t *testing.T) {
	t.Parallel()

	_, testFilename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) ok = false, want true")
	}
	productionFilename := filepath.Join(filepath.Dir(testFilename), "read_file_unix.go")
	file, err := parser.ParseFile(token.NewFileSet(), productionFilename, nil, 0)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v, want nil", productionFilename, err)
	}
	methods := prepareRegularReadFileMethodInventory(t, file)
	if methods.syscallConnectionCalls != 1 || methods.fileDescriptorCalls != 0 {
		t.Fatalf(
			"%s method calls = %+v, want one %s and zero %s calls",
			prepareRegularReadFileName,
			methods,
			syscallConnectionMethodName,
			fileDescriptorMethodName,
		)
	}
}

type prepareReadFileMethodInventory struct {
	syscallConnectionCalls int
	fileDescriptorCalls    int
}

func prepareRegularReadFileMethodInventory(t *testing.T, file *ast.File) prepareReadFileMethodInventory {
	t.Helper()

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != prepareRegularReadFileName || function.Body == nil {
			continue
		}
		var inventory prepareReadFileMethodInventory
		ast.Inspect(function.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case syscallConnectionMethodName:
				inventory.syscallConnectionCalls++
			case fileDescriptorMethodName:
				inventory.fileDescriptorCalls++
			}
			return true
		})
		return inventory
	}
	t.Fatalf("%s declaration present = false, want true", prepareRegularReadFileName)
	return prepareReadFileMethodInventory{}
}

// TestOpenRegularReadFileRefusesNamedPipeOnTheAcquiredHandle proves the refusal
// is decided by fstat on the descriptor this process holds, not by a path
// lookup another process can invalidate between the check and the open.
func TestOpenRegularReadFileRefusesNamedPipeOnTheAcquiredHandle(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(rootDirectory, "source"), 0o600); err != nil {
		t.Fatalf("Mkfifo(source) error = %v, want nil", err)
	}
	root := openInternalTestRoot(t, rootDirectory)
	file, err := openRegularReadFile(root, "source")
	if file != nil {
		closeInternalTestFile(t, file)
		t.Fatalf("openRegularReadFile(named pipe) file = %v, want nil", file)
	}
	if !errors.Is(err, core.ErrFilestoreSource) || !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf(
			"openRegularReadFile(named pipe) error = %v, want %v and %v",
			err,
			core.ErrFilestoreSource,
			fs.ErrInvalid,
		)
	}
}

func descriptorStatusFlags(t *testing.T, file *os.File) int {
	t.Helper()

	connection, err := file.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn() error = %v, want nil", err)
	}
	var flags int
	var flagsErr error
	if err := connection.Control(func(descriptor uintptr) {
		flags, flagsErr = unix.FcntlInt(descriptor, unix.F_GETFL, 0)
	}); err != nil {
		t.Fatalf("Control() error = %v, want nil", err)
	}
	if flagsErr != nil {
		t.Fatalf("FcntlInt(F_GETFL) error = %v, want nil", flagsErr)
	}
	return flags
}

func internalTestRegularFile(
	t *testing.T,
	rootDirectory string,
	name string,
	content string,
) (*os.Root, string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(rootDirectory, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
	}
	return openInternalTestRoot(t, rootDirectory), name
}

func openInternalTestRoot(t *testing.T, rootDirectory string) *os.Root {
	t.Helper()

	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatalf("OpenRoot(%s) error = %v, want nil", rootDirectory, err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Close(root) error = %v, want nil", err)
		}
	})
	return root
}

func closeInternalTestFile(t *testing.T, file *os.File) {
	t.Helper()

	if err := file.Close(); err != nil {
		t.Errorf("Close(file) error = %v, want nil", err)
	}
}
