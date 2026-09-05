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
	return err
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
	return resolveFunctionRules(symbol, standardSymbolRules())
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

func standardSymbolRules() []standardSymbolRule {
	return append(effectSymbolRules(), purePackageRules()...)
}

func effectSymbolRules() []standardSymbolRule {
	return []standardSymbolRule{
		{importPath: "flag", effect: EffectProcess, effectSelectors: []string{symbolParse}, pureSelectors: []string{"Arg", "Args", "NArg", "NFlag", "NewFlagSet", "UnquoteUsage", "Visit", "VisitAll"}},
		{importPath: "fmt", contextualSelectors: []string{"Errorf", "Sprint", "Sprintf", "Sprintln", "Fprint", "Fprintf", "Fprintln", "Fscan", "Fscanf", "Fscanln", "Print", "Printf", "Println", "Scan", "Scanf", "Scanln"}},
		{importPath: "go/parser", effect: EffectFilesystem, effectSelectors: []string{"ParseDir"}, pureSelectors: []string{"ParseExpr"}, contextualSelectors: []string{"ParseFile", "ParseExprFrom"}},
		{importPath: "io", pureSelectors: []string{"LimitReader", "MultiReader", "MultiWriter", "NewOffsetWriter", "NewSectionReader", symbolNopCloser, symbolPipe, "TeeReader"}},
		{importPath: "io/fs", pureSelectors: []string{"FileMode"}},
		{importPath: "path/filepath", effect: EffectFilesystem, effectSelectors: []string{"Abs", "EvalSymlinks", "Glob", "Walk", "WalkDir"}, pureSelectors: []string{"Base", "Clean", "Dir", "Ext", "FromSlash", "IsAbs", "IsLocal", symbolJoin, "Localize", "Match", "Rel", "Split", "SplitList", "ToSlash", "VolumeName"}},
		{importPath: "text/template", effect: EffectFilesystem, effectSelectors: []string{"ParseFiles", "ParseGlob"}, pureSelectors: []string{symbolNew, "Must"}},
		{importPath: "os", effect: EffectFilesystem, effectSelectors: osFilesystemSymbols(), pureSelectors: []string{"DevNull"}},
		{importPath: "os", effect: EffectHost, effectSelectors: osHostSymbols()},
		{importPath: "os", effect: EffectProcess, effectSelectors: []string{symbolExit, "FindProcess", symbolGetpid, symbolGetppid, symbolStartProcess, symbolPipe}},
		{importPath: catalogOsExec, effect: EffectProcess, effectSelectors: []string{"Command", "CommandContext", "LookPath"}},
		{importPath: "os/signal", effect: EffectSignal, effectSelectors: []string{"Ignore", "Ignored", "Notify", "NotifyContext", symbolReset, symbolStop}},
		{importPath: "crypto/rand", effect: EffectEntropy, effectSelectors: []string{symbolRead, "Int", "Prime", "Text"}},
		{importPath: catalogMathRand, effect: EffectEntropy, effectSelectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt31, symbolInt31n, symbolInt63, symbolInt63n, symbolIntn, symbolNormFloat64, symbolPerm, symbolRead, symbolSeed, symbolShuffle, symbolUint32, symbolUint64}, pureSelectors: []string{symbolNew, "NewSource", symbolZipf}},
		{importPath: catalogMathRandV2, effect: EffectEntropy, effectSelectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt32, symbolInt32N, symbolInt64, symbolInt64N, symbolIntN, symbolNormFloat64, symbolPerm, symbolShuffle, symbolUint, symbolUint32, symbolUint32N, symbolUint64, symbolUint64N, symbolUintN, "N"}, pureSelectors: []string{symbolNew, "NewPCG", "NewChaCha8", symbolZipf}},
		{importPath: "io/ioutil", effect: EffectFilesystem, effectSelectors: []string{symbolReadDir, symbolReadFile, symbolTempDir, "TempFile", symbolWriteFile}, pureSelectors: []string{symbolNopCloser}, contextualSelectors: []string{"ReadAll"}},
		{importPath: "net", effect: EffectTransport, effectSelectors: netEffectSymbols(), pureSelectors: []string{"CIDRMask", "IPv4", "IPv4Mask", "JoinHostPort", "ParseCIDR", "ParseIP", symbolPipe, "ResolveUnixAddr", "SplitHostPort"}},
		{importPath: catalogNetHttp, effect: EffectTransport, effectSelectors: httpEffectSymbols(), pureSelectors: httpPureSymbols(), secondary: EffectFilesystem, secondarySelectors: []string{symbolServeFile, symbolServeFileFS}},
		{importPath: "runtime", effect: EffectHost, effectSelectors: []string{"CPUProfile", "GOMAXPROCS", "GOROOT", "MemProfile", "NumCPU", "NumCgoCall", "ReadMemStats", "SetCPUProfileRate", "StartTrace", "StopTrace", "ThreadCreateProfile"}},
		{importPath: timeContractText, effect: EffectTime, effectSelectors: []string{"Since", "Until", symbolAfter, symbolAfterFunc, "NewTicker", "NewTimer", "Now", "Sleep", "Tick"}, pureSelectors: []string{symbolDate, "FixedZone", "LoadLocationFromTZData", symbolParse, "ParseDuration", "ParseInLocation", symbolUnix, "UnixMicro", "UnixMilli"}},
		{importPath: timeContractText, effect: EffectHost, effectSelectors: []string{"LoadLocation"}},
		{importPath: standardPackageSyscall, effect: EffectFilesystem, effectSelectors: syscallFilesystemSymbols()},
		{importPath: standardPackageSyscall, effect: EffectLocking, effectSelectors: syscallLockingSymbols()},
		{importPath: standardPackageSyscall, effect: EffectTransport, effectSelectors: syscallTransportSymbols()},
		{importPath: standardPackageSyscall, effect: EffectProcess, effectSelectors: syscallProcessSymbols()},
		{importPath: unixPackagePath, effect: EffectFilesystem, effectSelectors: syscallFilesystemSymbols()},
		{importPath: unixPackagePath, effect: EffectHost, effectSelectors: unixHostSymbols()},
		{importPath: unixPackagePath, effect: EffectLocking, effectSelectors: syscallLockingSymbols()},
		{importPath: unixPackagePath, effect: EffectTransport, effectSelectors: syscallTransportSymbols()},
		{importPath: unixPackagePath, effect: EffectProcess, effectSelectors: syscallProcessSymbols()},
		{importPath: windowsPackagePath, effect: EffectFilesystem, effectSelectors: syscallFilesystemSymbols()},
		{importPath: windowsPackagePath, effect: EffectFilesystem, effectSelectors: windowsFilesystemSymbols()},
		{importPath: windowsPackagePath, effect: EffectHost, effectSelectors: windowsHostSymbols()},
		{importPath: windowsPackagePath, effect: EffectLocking, effectSelectors: windowsLockingSymbols()},
		{importPath: windowsPackagePath, effect: EffectTransport, effectSelectors: syscallTransportSymbols()},
		{importPath: windowsPackagePath, effect: EffectProcess, effectSelectors: syscallProcessSymbols()},
	}
}

// Only explicitly reviewed functions classify; package membership is not proof.
func purePackageRules() []standardSymbolRule {
	return []standardSymbolRule{
		{importPath: standardPackageBuiltin, pureSelectors: []string{"append", "cap", "clear", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "real", "recover"}, contextualSelectors: []string{"close", "panic", "print", "println"}},
		{importPath: "crypto/sha256", pureSelectors: []string{"New", "New224", "Sum224", "Sum256"}},
		{importPath: "crypto/sha512", pureSelectors: []string{"New", "New384", "New512_224", "New512_256", "Sum384", "Sum512", "Sum512_224", "Sum512_256"}},
		{importPath: "context", pureSelectors: []string{"Background", "TODO", "WithValue", "WithoutCancel"}, contextualSelectors: []string{"WithCancel", "WithCancelCause", "WithDeadline", "WithDeadlineCause", "WithTimeout", "WithTimeoutCause", symbolAfterFunc, "Cause"}},
		{importPath: "errors", pureSelectors: []string{"New", symbolJoin}, contextualSelectors: []string{"Is", "As", "AsType", "Unwrap"}},
		{importPath: "encoding/json", contextualSelectors: []string{"Marshal", "MarshalIndent", "Unmarshal"}, pureSelectors: []string{"Valid", "NewDecoder", "NewEncoder"}},
		{importPath: "reflect", pureSelectors: []string{"TypeOf", "TypeFor", "ValueOf"}},
	}
}

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
