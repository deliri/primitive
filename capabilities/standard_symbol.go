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
)

func (d StandardSymbolDisposition) Validate() error {
	if d < StandardSymbolPure || d > StandardSymbolEffect {
		return contractError("standard symbol disposition is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether d belongs to the closed disposition domain.
func (d StandardSymbolDisposition) IsValid() bool { return d.Validate() == nil }

// OffWireEnum marks StandardSymbolDisposition as a compiler-only enum.
func (StandardSymbolDisposition) OffWireEnum() {}

// String returns the stable doctrine identity of a valid disposition.
func (d StandardSymbolDisposition) String() string {
	if !d.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{
		StandardSymbolPure:       "pure",
		StandardSymbolContextual: "contextual",
		StandardSymbolEffect:     "effect",
	}[d]
}

var _ core.OffWireEnum = StandardSymbolUnknown

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
}

func (s StandardSymbol) Validate() error {
	return errors.Join(s.ImportPath.Validate(), s.Selector.Validate())
}

// StandardSymbolFact is Primitive's ownership fact for one observed symbol.
// Secondary contains additional owners for composite helpers such as
// net/http.ServeFile, which touches both transport and filesystem.
type StandardSymbolFact struct {
	Symbol      StandardSymbol
	Secondary   []Effect
	Disposition StandardSymbolDisposition
	Effect      Effect
}

func (f StandardSymbolFact) Validate() error {
	if err := f.Symbol.Validate(); err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	if f.Disposition == StandardSymbolUnknown {
		return f.validateEffectFree()
	}
	if err := f.Disposition.Validate(); err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	if f.Disposition != StandardSymbolEffect {
		return f.validateEffectFree()
	}
	if err := f.Effect.Validate(); err != nil {
		return err
	}
	return f.validateSecondaryEffects()
}

func (f StandardSymbolFact) validateEffectFree() error {
	if f.Effect != EffectUnknown || len(f.Secondary) != 0 {
		return contractError("effect-free standard symbol carries effect ownership")
	}
	return nil
}

func (f StandardSymbolFact) validateSecondaryEffects() error {
	for index := range f.Secondary {
		if err := f.Secondary[index].Validate(); err != nil || f.Secondary[index] == f.Effect {
			return contractError("standard symbol secondary effect is invalid or duplicate")
		}
		for previous := range index {
			if f.Secondary[previous] == f.Secondary[index] {
				return contractError("standard symbol secondary effect is duplicated")
			}
		}
	}
	return nil
}

// ResolveStandardSymbol returns Primitive's compiled ownership fact. An
// unlisted valid symbol returns StandardSymbolUnknown with nil error so the
// syntax inspector can retain it as unresolved rather than invent ownership.
func ResolveStandardSymbol(symbol StandardSymbol) (StandardSymbolFact, error) {
	if err := symbol.Validate(); err != nil {
		return StandardSymbolFact{}, errors.Join(core.ErrCapabilitiesContract, err)
	}
	path := symbol.ImportPath.String()
	selector := symbol.Selector.String()
	for _, rule := range standardSymbolRules() {
		if rule.importPath != path {
			continue
		}
		fact, err := rule.resolve(symbol, selector)
		if err != nil || fact.Disposition != StandardSymbolUnknown {
			return fact, err
		}
	}
	return StandardSymbolFact{Symbol: symbol, Disposition: StandardSymbolUnknown}, nil
}

type standardSymbolRule struct {
	importPath          string
	effectSelectors     []string
	pureSelectors       []string
	contextualSelectors []string
	secondarySelectors  []string
	defaultDisposition  StandardSymbolDisposition
	effect              Effect
	secondary           Effect
}

func (r standardSymbolRule) resolve(symbol StandardSymbol, selector string) (StandardSymbolFact, error) {
	disposition := r.defaultDisposition
	if slices.Contains(r.effectSelectors, selector) {
		disposition = StandardSymbolEffect
	}
	if slices.Contains(r.pureSelectors, selector) {
		disposition = StandardSymbolPure
	}
	if slices.Contains(r.contextualSelectors, selector) {
		disposition = StandardSymbolContextual
	}
	fact := StandardSymbolFact{Symbol: symbol, Disposition: disposition}
	if disposition == StandardSymbolEffect {
		fact.Effect = r.effect
		if r.secondary != EffectUnknown && slices.Contains(r.secondarySelectors, selector) {
			fact.Secondary = []Effect{r.secondary}
		}
	}
	if disposition == StandardSymbolUnknown {
		return fact, nil
	}
	return fact, fact.Validate()
}

func standardSymbolRules() []standardSymbolRule {
	return append(effectSymbolRules(), purePackageRules()...)
}

func effectSymbolRules() []standardSymbolRule {
	return []standardSymbolRule{
		{importPath: "flag", effect: EffectProcess, effectSelectors: []string{symbolParse}, pureSelectors: []string{"Arg", "Args", "NArg", "NFlag", "NewFlagSet", "PrintDefaults", "UnquoteUsage", "Visit", "VisitAll"}},
		{importPath: "fmt", pureSelectors: []string{"Errorf", "Sprint", "Sprintf", "Sprintln"}},
		{importPath: "go/parser", effect: EffectFilesystem, effectSelectors: []string{"ParseDir"}, pureSelectors: []string{"ParseExpr"}, contextualSelectors: []string{"ParseFile", "ParseExprFrom"}},
		{importPath: "io", pureSelectors: []string{"LimitReader", "MultiReader", "MultiWriter", "NewOffsetWriter", "NewSectionReader", symbolNopCloser, symbolPipe, "TeeReader"}},
		{importPath: "io/fs", pureSelectors: []string{"FileMode"}},
		{importPath: "path/filepath", effect: EffectFilesystem, effectSelectors: []string{"Abs", "EvalSymlinks", "Glob", "Walk", "WalkDir"}, pureSelectors: []string{"Base", "Clean", "Dir", "Ext", "FromSlash", "IsAbs", "IsLocal", "Join", "Localize", "Match", "Rel", "Split", "SplitList", "ToSlash", "VolumeName"}},
		{importPath: "text/template", effect: EffectFilesystem, effectSelectors: []string{"ParseFiles", "ParseGlob"}, pureSelectors: []string{symbolNew, "Must"}},
		{importPath: "os", effect: EffectFilesystem, effectSelectors: osFilesystemSymbols(), pureSelectors: []string{"DevNull"}},
		{importPath: "os", effect: EffectHost, effectSelectors: osHostSymbols()},
		{importPath: "os", effect: EffectProcess, effectSelectors: []string{symbolExit, "FindProcess", symbolGetpid, symbolGetppid, symbolStartProcess}},
		{importPath: "os/exec", effect: EffectProcess, defaultDisposition: StandardSymbolEffect},
		{importPath: "os/signal", effect: EffectSignal, defaultDisposition: StandardSymbolEffect},
		{importPath: "crypto/rand", effect: EffectEntropy, defaultDisposition: StandardSymbolEffect},
		{importPath: "math/rand", effect: EffectEntropy, defaultDisposition: StandardSymbolEffect, pureSelectors: []string{symbolNew, "NewSource", symbolZipf}},
		{importPath: "math/rand/v2", effect: EffectEntropy, defaultDisposition: StandardSymbolEffect, pureSelectors: []string{symbolNew, "NewPCG", "NewChaCha8", symbolZipf}},
		{importPath: "io/ioutil", effect: EffectFilesystem, effectSelectors: []string{symbolReadDir, symbolReadFile, "TempDir", "TempFile", symbolWriteFile}, pureSelectors: []string{"ReadAll", symbolNopCloser}},
		{importPath: "net", effect: EffectTransport, effectSelectors: netEffectSymbols(), pureSelectors: []string{"CIDRMask", "IPv4", "IPv4Mask", "JoinHostPort", "ParseCIDR", "ParseIP", symbolPipe, "ResolveUnixAddr", "SplitHostPort"}},
		{importPath: "net/http", effect: EffectTransport, effectSelectors: httpEffectSymbols(), pureSelectors: httpPureSymbols(), secondary: EffectFilesystem, secondarySelectors: []string{symbolServeFile, symbolServeFileFS}},
		{importPath: "runtime", effect: EffectHost, effectSelectors: []string{"CPUProfile", "GOMAXPROCS", "GOROOT", "MemProfile", "NumCPU", "NumCgoCall", "ReadMemStats", "SetCPUProfileRate", "StartTrace", "StopTrace", "ThreadCreateProfile"}},
		{importPath: timeContractText, effect: EffectTime, effectSelectors: []string{"After", "AfterFunc", "NewTicker", "NewTimer", "Now", "Sleep", "Tick"}, pureSelectors: []string{"Date", "FixedZone", "LoadLocationFromTZData", symbolParse, "ParseDuration", "ParseInLocation", "Unix", "UnixMicro", "UnixMilli"}},
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

func purePackageRules() []standardSymbolRule {
	paths := []string{
		"bytes", "cmp", "context", "crypto", "crypto/md5", "crypto/sha1", "crypto/sha256", "crypto/sha512",
		"encoding", "encoding/base64", "encoding/binary", "encoding/csv", "encoding/hex", "encoding/json", "encoding/xml",
		"errors", "go/ast", "go/format", "go/token", "hash", "hash/crc32", "hash/crc64", "hash/fnv",
		"maps", "math", "math/big", "net/netip", "net/url", "path", "reflect", "regexp", "slices", "sort",
		"strconv", "strings", "sync", "sync/atomic", testingPackagePath, "text/scanner", "text/tabwriter", "unicode", "unicode/utf8",
	}
	rules := make([]standardSymbolRule, len(paths))
	for index, path := range paths {
		rules[index] = standardSymbolRule{importPath: path, defaultDisposition: StandardSymbolPure}
	}
	return rules
}

func osFilesystemSymbols() []string {
	return []string{symbolChdir, symbolChmod, symbolChown, "Create", "CreateTemp", symbolLchown, symbolLink, symbolLstat, symbolMkdir, "MkdirAll", "MkdirTemp", symbolOpen, "OpenFile", "OpenRoot", symbolReadDir, symbolReadFile, symbolReadlink, "Remove", "RemoveAll", symbolRename, symbolStat, symbolSymlink, symbolTruncate, symbolWriteFile}
}

func osHostSymbols() []string {
	return []string{"Clearenv", "Environ", "Executable", "ExpandEnv", "Getenv", "Getpagesize", "Getuid", "Geteuid", "Getgid", "Getegid", "Getgroups", "Getwd", "Hostname", "LookupEnv", "Setenv", "Unsetenv", "UserCacheDir", "UserConfigDir", "UserHomeDir"}
}

func netEffectSymbols() []string {
	return []string{"Dial", "DialIP", "DialTCP", "DialUDP", "DialUnix", symbolListen, "ListenIP", "ListenMulticastUDP", "ListenPacket", "ListenTCP", "ListenUDP", "ListenUnix", "LookupAddr", "LookupCNAME", "LookupHost", "LookupIP", "LookupMX", "LookupNS", "LookupPort", "LookupSRV", "LookupTXT", "ResolveIPAddr", "ResolveTCPAddr", "ResolveUDPAddr"}
}

func httpEffectSymbols() []string {
	return []string{"Error", "Get", "Head", "ListenAndServe", "ListenAndServeTLS", "NotFound", "Post", "PostForm", "Redirect", "Serve", "ServeContent", symbolServeFile, symbolServeFileFS, "ServeTLS", "SetCookie"}
}

func httpPureSymbols() []string {
	return []string{"CanonicalHeaderKey", "DetectContentType", "MaxBytesHandler", "MaxBytesReader", "NewFileTransportFS", "NewRequest", "NewRequestWithContext", "NewResponseController", "ParseCookie", "ParseHTTPVersion", "ParseSetCookie", "RedirectHandler", "StripPrefix", "TimeoutHandler"}
}

func syscallFilesystemSymbols() []string {
	return []string{"Access", symbolChdir, symbolChmod, symbolChown, "Close", "Creat", "Dup", "Fchmod", "Fchown", "Fstat", "Fstatat", "Fsync", "Ftruncate", "Getcwd", "Getdents", symbolLchown, symbolLink, symbolLstat, symbolMkdir, "Mkdirat", symbolOpen, "Openat", "Pread", "Pwrite", "Read", "ReadDirent", symbolReadlink, symbolRename, "Renameat", "Rmdir", symbolStat, symbolSymlink, "Sync", symbolTruncate, "Unlink", "Unlinkat", "Write"}
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
	return []string{"Accept", "Bind", "Connect", "Getpeername", "Getsockname", "GetsockoptInt", symbolListen, "Recvfrom", "Sendto", "SetsockoptInt", "Shutdown", "Socket", "Socketpair"}
}

func syscallProcessSymbols() []string {
	return []string{"Exec", symbolExit, "ForkExec", symbolGetpid, symbolGetppid, "Kill", symbolStartProcess, "Wait4"}
}
