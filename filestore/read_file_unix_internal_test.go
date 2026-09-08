//go:build darwin || linux

package filestore

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// nonblockingOpenBackstop bounds only a wedged open. A correct acquisition
// returns in microseconds, so expiry means the descriptor was parked waiting
// for a peer rather than that the machine is slow.
const nonblockingOpenBackstop = 30 * time.Second

const namedPipeOpenProbeRootEnvironment = "PRIMITIVE_FILESTORE_NAMED_PIPE_OPEN_PROBE_ROOT"

type regularReadPreparationOwner struct {
	prepareRegularReadFile func(*os.File) error
}

type fileDescriptorAccessor interface{ Fd() uintptr }

var (
	_ regularReadPreparationOwner = regularReadPreparationOwner{prepareRegularReadFile: prepareRegularReadFile}
	_ fileDescriptorAccessor      = (*os.File)(nil)
	_ syscall.Conn                = (*os.File)(nil)
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
	if err := syscall.Mkfifo(filepath.Join(rootDirectory, "source"), 0o600); err != nil {
		t.Fatalf("Mkfifo(source) error = %v, want nil", err)
	}
	duration, err := temporal.NewDuration(nonblockingOpenBackstop)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
	if err != nil {
		t.Fatal(err)
	}
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

	if flags := descriptorStatusFlags(t, file); flags&syscall.O_NONBLOCK == 0 {
		t.Fatalf("openReadFile(regular) status flags = %#x, want O_NONBLOCK (%#x) set", flags, syscall.O_NONBLOCK)
	}
}

// TestOpenRegularReadFileRestoresBlockingMode proves the acquired handle is
// handed to the streaming copy under the ordinary read contract, so the
// nonblocking acquisition is not observable past the identity proof.
func TestOpenRegularReadFileRestoresBlockingMode(t *testing.T) {
	t.Parallel()

	root, path := internalTestRegularFile(t, t.TempDir(), "source", "payload")
	file, _, err := openRegularReadFile(root, path)
	if err != nil {
		t.Fatalf("openRegularReadFile(regular) error = %v, want nil", err)
	}
	defer closeInternalTestFile(t, file)

	if flags := descriptorStatusFlags(t, file); flags&syscall.O_NONBLOCK != 0 {
		t.Fatalf("openRegularReadFile(regular) status flags = %#x, want O_NONBLOCK (%#x) cleared", flags, syscall.O_NONBLOCK)
	}
}

func TestMutableRegularHandlesRestoreBlockingModeBeforeOwnershipTransfer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		open     func(*testing.T, *os.Root, core.RelativePath) (*os.File, error)
		name     string
		existing bool
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
			if flags := descriptorStatusFlags(t, file); flags&syscall.O_NONBLOCK != 0 {
				t.Fatalf("mutable regular handle status flags = %#x, want O_NONBLOCK (%#x) cleared", flags, syscall.O_NONBLOCK)
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

	source, err := filestoreGoSources.ReadFile("read_file_unix.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "read_file_unix.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("ParseFile(read_file_unix.go) error = %v, want nil", err)
	}
	methods := prepareRegularReadFileMethodInventory(t, file)
	if methods.syscallConnectionCalls != 1 || methods.fileDescriptorCalls != 0 {
		t.Fatalf(
			"%s method calls = %+v, want one %s and zero %s calls",
			reflect.TypeFor[regularReadPreparationOwner]().Field(0).Name,
			methods,
			reflect.TypeFor[syscall.Conn]().Method(0).Name,
			reflect.TypeFor[fileDescriptorAccessor]().Method(0).Name,
		)
	}
}

type prepareReadFileMethodInventory struct {
	syscallConnectionCalls int
	fileDescriptorCalls    int
}

func prepareRegularReadFileMethodInventory(t *testing.T, file *ast.File) prepareReadFileMethodInventory {
	t.Helper()
	prepareRegularReadFileName := reflect.TypeFor[regularReadPreparationOwner]().Field(0).Name
	syscallConnectionMethodName := reflect.TypeFor[syscall.Conn]().Method(0).Name
	fileDescriptorMethodName := reflect.TypeFor[fileDescriptorAccessor]().Method(0).Name

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != prepareRegularReadFileName || function.Body == nil {
			continue
		}
		var inventory prepareReadFileMethodInventory
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
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

func TestRegularReadDescriptorSourceMatcherRejectsDisguisedCalls(t *testing.T) {
	t.Parallel()
	owner := reflect.TypeFor[regularReadPreparationOwner]().Field(0).Name
	connection := reflect.TypeFor[syscall.Conn]().Method(0).Name
	descriptor := reflect.TypeFor[fileDescriptorAccessor]().Method(0).Name
	for _, tc := range []struct {
		name  string
		body  string
		other string
		want  prepareReadFileMethodInventory
	}{
		{name: "absent calls cannot claim descriptor lifetime protection"},
		{name: "real connection acquisition is counted", body: "file." + connection + "()", want: prepareReadFileMethodInventory{syscallConnectionCalls: 1}},
		{name: "borrowed method value cannot impersonate a call", body: "_ = file." + connection},
		{name: "direct descriptor access remains visible", body: "file." + descriptor + "()", want: prepareReadFileMethodInventory{fileDescriptorCalls: 1}},
		{name: "safe call cannot hide an additional direct descriptor call", body: "file." + connection + "(); file." + descriptor + "()", want: prepareReadFileMethodInventory{syscallConnectionCalls: 1, fileDescriptorCalls: 1}},
		{name: "unrelated function cannot satisfy the owner", other: "func other() { file." + connection + "() }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := fmt.Sprintf("package filestore\nfunc %s() { %s }\n%s", owner, tc.body, tc.other)
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := prepareRegularReadFileMethodInventory(t, file); got != tc.want {
				t.Fatalf("method inventory = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestOpenRegularReadFileRefusesNamedPipeOnTheAcquiredHandle proves the refusal
// is decided by fstat on the descriptor this process holds, not by a path
// lookup another process can invalidate between the check and the open.
func TestOpenRegularReadFileRefusesNamedPipeOnTheAcquiredHandle(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(rootDirectory, "source"), 0o600); err != nil {
		t.Fatalf("Mkfifo(source) error = %v, want nil", err)
	}
	root := openInternalTestRoot(t, rootDirectory)
	file, _, err := openRegularReadFile(root, "source")
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
		value, _, errno := syscall.Syscall(syscall.SYS_FCNTL, descriptor, syscall.F_GETFL, 0)
		flags = int(value)
		if errno != 0 {
			flagsErr = errno
		}
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
