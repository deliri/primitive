package capabilities

import (
	"errors"
	"go/token"
	"slices"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// StandardSymbolDisposition describes what Primitive knows about one
// standard-library or low-level substrate symbol.
type StandardSymbolDisposition uint8

const (
	StandardSymbolUnknown StandardSymbolDisposition = iota
	// StandardSymbolPure performs no real-world effect.
	StandardSymbolPure
	// StandardSymbolContextual requires the syntax inspector to classify its arguments.
	StandardSymbolContextual
	// StandardSymbolEffect performs a Primitive-owned real-world effect.
	StandardSymbolEffect
	// StandardSymbolUnresolved is an admitted outcome with no claimed effect knowledge.
	StandardSymbolUnresolved
)

func (d StandardSymbolDisposition) Validate() error {
	if d < StandardSymbolPure || d > StandardSymbolUnresolved {
		return contractError("standard symbol disposition is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether d belongs to the closed disposition domain.
func (d StandardSymbolDisposition) IsValid() bool { return d.Validate() == nil }

// String returns the stable doctrine identity of a valid disposition.
func (d StandardSymbolDisposition) String() string {
	if !d.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{
		StandardSymbolPure:       "pure",
		StandardSymbolContextual: "contextual",
		StandardSymbolEffect:     "effect",
		StandardSymbolUnresolved: "unresolved",
	}[d]
}

// SymbolName is one compiler-admitted Go selector.
type SymbolName struct{ value string }

// ParseSymbolName admits one Go selector identifier.
func ParseSymbolName(value string) (SymbolName, error) {
	name := SymbolName{value: value}
	if err := name.Validate(); err != nil {
		return SymbolName{}, err
	}
	return name, nil
}

func (n SymbolName) Validate() error {
	if !token.IsIdentifier(n.value) || token.Lookup(n.value).IsKeyword() || n.value == "_" {
		return contractError("standard symbol name is not a Go identifier")
	}
	return nil
}

func (n SymbolName) String() string {
	if n.Validate() != nil {
		return ""
	}
	return n.value
}

// StandardSymbol identifies one package-qualified low-level symbol.
type StandardSymbol struct {
	ImportPath gomodule.ImportPath
	Selector   SymbolName
	// Receiver is absent for package functions and names the compiler-resolved
	// declaring type for methods. It never names a caller's variable or alias.
	Receiver *SymbolName
}

func (s StandardSymbol) Validate() error {
	err := errors.Join(s.ImportPath.Validate(), s.Selector.Validate())
	if s.Receiver != nil {
		err = errors.Join(err, s.Receiver.Validate())
	}
	if err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	return nil
}

// StandardSymbolFact is Primitive's ownership fact for one observed symbol.
// Secondary contains additional owners for composite helpers such as
// net/http.ServeFile, which touches both transport and filesystem.
type StandardSymbolFact struct {
	Symbol StandardSymbol
	Classification
}

func (f StandardSymbolFact) Validate() error {
	return errors.Join(f.Symbol.Validate(), f.Classification.Validate())
}

// ResolveStandardSymbol returns Primitive's compiled ownership fact. An
// unlisted valid symbol returns StandardSymbolUnresolved with nil error so the
// syntax inspector can retain it as unresolved rather than invent ownership.
func ResolveStandardSymbol(symbol StandardSymbol) (StandardSymbolFact, error) {
	if err := symbol.Validate(); err != nil {
		return StandardSymbolFact{}, errors.Join(core.ErrCapabilitiesContract, err)
	}
	fact, err := resolveSymbolRules(symbol)
	if err != nil {
		return StandardSymbolFact{}, err
	}
	fact.Operation, err = fact.Replacement()
	if err != nil {
		return StandardSymbolFact{}, err
	}
	return fact, fact.Validate()
}

func resolveSymbolRules(symbol StandardSymbol) (StandardSymbolFact, error) {
	if symbol.Receiver != nil {
		return resolveStandardMethod(symbol)
	}
	return resolveFunctionRules(symbol, standardSymbolRules(symbol.ImportPath.String()))
}

func resolveFunctionRules(symbol StandardSymbol, rules []standardSymbolRule) (StandardSymbolFact, error) {
	result := StandardSymbolFact{Symbol: symbol, Disposition: StandardSymbolUnresolved}
	path := symbol.ImportPath.String()
	selector := symbol.Selector.String()
	for _, rule := range rules {
		if rule.importPath != path {
			continue
		}
		fact, err := rule.resolve(symbol, selector)
		if err != nil {
			return StandardSymbolFact{}, err
		}
		if err := mergeClassification(&result.Classification, fact.Classification); err != nil {
			return StandardSymbolFact{}, err
		}
	}
	return result, result.Validate()
}

type standardSymbolRule struct {
	importPath          string
	effectSelectors     []string
	pureSelectors       []string
	contextualSelectors []string
	secondarySelectors  []string
	effect              Effect
	secondary           Effect
}

func (r standardSymbolRule) resolve(symbol StandardSymbol, selector string) (StandardSymbolFact, error) {
	disposition := StandardSymbolUnresolved
	groups := []struct {
		selectors   []string
		disposition StandardSymbolDisposition
	}{
		{r.effectSelectors, StandardSymbolEffect}, {r.pureSelectors, StandardSymbolPure}, {r.contextualSelectors, StandardSymbolContextual},
	}
	for _, group := range groups {
		if !slices.Contains(group.selectors, selector) {
			continue
		}
		if disposition != StandardSymbolUnresolved {
			return StandardSymbolFact{}, contractError("symbol has contradictory dispositions")
		}
		disposition = group.disposition
	}
	fact := StandardSymbolFact{Symbol: symbol, Disposition: disposition}
	if disposition == StandardSymbolEffect {
		fact.Effect = r.effect
		if r.secondary != EffectUnknown && slices.Contains(r.secondarySelectors, selector) {
			fact.Secondary = []Effect{r.secondary}
		}
	}

	return fact, fact.Validate()
}

// Only explicitly reviewed functions classify; package membership is not proof.

func osFilesystemSymbols() []string {
	return []string{symbolChdir, symbolChmod, symbolChown, "CopyFS", symbolChtimes, "OpenInRoot", symbolCreate, "CreateTemp", symbolLchown, symbolLink, symbolLstat, symbolMkdir, symbolMkdirAll, "MkdirTemp", symbolOpen, symbolOpenFile, symbolOpenRoot, symbolReadDir, symbolReadFile, symbolReadlink, symbolRemove, symbolRemoveAll, symbolRename, symbolStat, symbolSymlink, symbolTruncate, symbolWriteFile}
}

func osHostSymbols() []string {
	return []string{symbolTempDir, "Clearenv", symbolEnviron, "Executable", "ExpandEnv", "Getenv", "Getpagesize", "Getuid", "Geteuid", "Getgid", "Getegid", "Getgroups", "Getwd", "Hostname", "LookupEnv", "Setenv", "Unsetenv", "UserCacheDir", "UserConfigDir", "UserHomeDir"}
}

func netEffectSymbols() []string {
	return []string{"DialTimeout", "FileConn", "FileListener", "FilePacketConn", "ListenUnixgram", symbolDial, symbolDialIP, symbolDialTCP, symbolDialUDP, symbolDialUnix, symbolListen, "ListenIP", "ListenMulticastUDP", symbolListenPacket, "ListenTCP", "ListenUDP", "ListenUnix", "LookupAddr", "LookupCNAME", "LookupHost", "LookupIP", "LookupMX", "LookupNS", "LookupPort", "LookupSRV", "LookupTXT", "ResolveIPAddr", "ResolveTCPAddr", "ResolveUDPAddr"}
}

func httpEffectSymbols() []string {
	return []string{symbolError, "Get", symbolHead, symbolListenAndServe, symbolListenAndServeTLS, "NotFound", symbolPost, symbolPostForm, "Redirect", symbolServe, "ServeContent", symbolServeFile, symbolServeFileFS, symbolServeTLS, "SetCookie"}
}

func httpPureSymbols() []string {
	return []string{"CanonicalHeaderKey", "DetectContentType", "MaxBytesHandler", "MaxBytesReader", "NewFileTransportFS", "NewRequest", "NewRequestWithContext", "NewResponseController", "ParseCookie", "ParseHTTPVersion", "ParseSetCookie", "RedirectHandler", "StripPrefix", "TimeoutHandler"}
}

func syscallFilesystemSymbols() []string {
	return []string{"Mkfifo", "Access", symbolChdir, symbolChmod, symbolChown, symbolClose, "Creat", "Dup", "Fchmod", "Fchown", "Fstat", "Fstatat", "Fsync", "Ftruncate", "Getcwd", "Getdents", symbolLchown, symbolLink, symbolLstat, symbolMkdir, "Mkdirat", symbolOpen, "Openat", "Pread", "Pwrite", symbolRead, "ReadDirent", symbolReadlink, symbolRename, "Renameat", "Rmdir", symbolStat, symbolSymlink, symbolSync, symbolTruncate, "Unlink", "Unlinkat", symbolWrite}
}

func unixHostSymbols() []string {
	return []string{"Fstatfs", "IoctlGetWinsize", "Statfs", "SysctlUint64", "Sysinfo"}
}

func windowsFilesystemSymbols() []string {
	return []string{"GetFileInformationByHandle"}
}

func windowsHostSymbols() []string {
	return []string{"GetConsoleScreenBufferInfo", "GetDiskFreeSpaceEx", "GetFinalPathNameByHandle"}
}

func syscallLockingSymbols() []string {
	return []string{"Flock"}
}

func windowsLockingSymbols() []string {
	return []string{"LockFileEx", "UnlockFileEx"}
}

func syscallTransportSymbols() []string {
	return []string{symbolAccept, "Bind", "Connect", "Getpeername", "Getsockname", "GetsockoptInt", symbolListen, "Recvfrom", "Sendto", "SetsockoptInt", symbolShutdown, "Socket", "Socketpair"}
}

func syscallProcessSymbols() []string {
	return []string{"Exec", symbolExit, "ForkExec", symbolGetpid, symbolGetppid, symbolKill, symbolStartProcess, "Wait4"}
}
