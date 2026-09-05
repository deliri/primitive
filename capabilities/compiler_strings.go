package capabilities

// Compiler-owned spellings for the Go and low-level substrate catalog. A
// selector shared by multiple standard packages remains one mechanical symbol
// spelling rather than copied text.
const (
	standardPackageBuiltin = "builtin"
	symbolError            = "Error"
	standardPackageSyscall = "syscall"
	testingPackagePath     = "testing"
	timeContractText       = "time"
	unixPackagePath        = "golang.org/x/sys/unix"
	windowsPackagePath     = "golang.org/x/sys/windows"

	symbolChdir        = "Chdir"
	symbolChmod        = "Chmod"
	symbolChown        = "Chown"
	symbolExit         = "Exit"
	symbolGetpid       = "Getpid"
	symbolGetppid      = "Getppid"
	symbolLchown       = "Lchown"
	symbolLink         = "Link"
	symbolListen       = "Listen"
	symbolLstat        = "Lstat"
	symbolMkdir        = "Mkdir"
	symbolNew          = "New"
	symbolNopCloser    = "NopCloser"
	symbolOpen         = "Open"
	symbolParse        = "Parse"
	symbolPipe         = "Pipe"
	symbolReadDir      = "ReadDir"
	symbolReadFile     = "ReadFile"
	symbolReadlink     = "Readlink"
	symbolRename       = "Rename"
	symbolServeFile    = "ServeFile"
	symbolServeFileFS  = "ServeFileFS"
	symbolStartProcess = "StartProcess"
	symbolStat         = "Stat"
	symbolSymlink      = "Symlink"
	symbolTruncate     = "Truncate"
	symbolWriteFile    = "WriteFile"
	symbolZipf         = "Zipf"
)
