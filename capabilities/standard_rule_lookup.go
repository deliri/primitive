package capabilities

// Package dispatch constructs only the selected package's existing rules.
// Unknown packages allocate no rule table; all ownership policy stays typed.
func standardSymbolRules(importPath string) []standardSymbolRule {
	if rules := standardFunctionRules1(importPath); rules != nil {
		return rules
	}
	if rules := standardFunctionRules2(importPath); rules != nil {
		return rules
	}
	if rules := standardFunctionRules3(importPath); rules != nil {
		return rules
	}
	if rules := standardFunctionRules4(importPath); rules != nil {
		return rules
	}
	return nil
}
func standardFunctionRules1(importPath string) []standardSymbolRule {
	switch importPath {
	case "flag":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectProcess, effectSelectors: []string{symbolParse}, pureSelectors: []string{"Arg", "Args", "NArg", "NFlag", "NewFlagSet", "UnquoteUsage", "Visit", "VisitAll"}},
		}
	case "fmt":
		return []standardSymbolRule{
			{importPath: importPath, contextualSelectors: []string{"Errorf", "Sprint", "Sprintf", "Sprintln", "Fprint", "Fprintf", "Fprintln", "Fscan", "Fscanf", "Fscanln", "Print", "Printf", "Println", "Scan", "Scanf", "Scanln"}},
		}
	case "go/parser":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: []string{"ParseDir"}, pureSelectors: []string{"ParseExpr"}, contextualSelectors: []string{"ParseFile", "ParseExprFrom"}},
		}
	case "io":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"LimitReader", "MultiReader", "MultiWriter", "NewOffsetWriter", "NewSectionReader", symbolNopCloser, symbolPipe, "TeeReader"}},
		}
	case "io/fs":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"FileMode"}},
		}
	case "path/filepath":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: []string{"Abs", "EvalSymlinks", "Glob", "Walk", "WalkDir"}, pureSelectors: []string{"Base", "Clean", "Dir", "Ext", "FromSlash", "IsAbs", "IsLocal", symbolJoin, "Localize", "Match", "Rel", "Split", "SplitList", "ToSlash", "VolumeName"}},
		}
	case "text/template":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: []string{"ParseFiles", "ParseGlob"}, pureSelectors: []string{symbolNew, "Must"}},
		}
	case "os":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: osFilesystemSymbols(), pureSelectors: []string{"DevNull"}},
			{importPath: importPath, effect: EffectHost, effectSelectors: osHostSymbols()},
			{importPath: importPath, effect: EffectProcess, effectSelectors: []string{symbolExit, "FindProcess", symbolGetpid, symbolGetppid, symbolStartProcess, symbolPipe}},
		}
	}
	return nil
}
func standardFunctionRules2(importPath string) []standardSymbolRule {
	switch importPath {
	case catalogOsExec:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectProcess, effectSelectors: []string{"Command", "CommandContext", "LookPath"}},
		}
	case "os/signal":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectSignal, effectSelectors: []string{"Ignore", "Ignored", "Notify", "NotifyContext", symbolReset, symbolStop}},
		}
	case "crypto/rand":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectEntropy, effectSelectors: []string{symbolRead, "Int", "Prime", "Text"}},
		}
	case catalogMathRand:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectEntropy, effectSelectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt31, symbolInt31n, symbolInt63, symbolInt63n, symbolIntn, symbolNormFloat64, symbolPerm, symbolRead, symbolSeed, symbolShuffle, symbolUint32, symbolUint64}, pureSelectors: []string{symbolNew, "NewSource", symbolZipf}},
		}
	case catalogMathRandV2:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectEntropy, effectSelectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt32, symbolInt32N, symbolInt64, symbolInt64N, symbolIntN, symbolNormFloat64, symbolPerm, symbolShuffle, symbolUint, symbolUint32, symbolUint32N, symbolUint64, symbolUint64N, symbolUintN, "N"}, pureSelectors: []string{symbolNew, "NewPCG", "NewChaCha8", symbolZipf}},
		}
	case "io/ioutil":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: []string{symbolReadDir, symbolReadFile, symbolTempDir, "TempFile", symbolWriteFile}, pureSelectors: []string{symbolNopCloser}, contextualSelectors: []string{"ReadAll"}},
		}
	case "net":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectTransport, effectSelectors: netEffectSymbols(), pureSelectors: []string{"CIDRMask", "IPv4", "IPv4Mask", "JoinHostPort", "ParseCIDR", "ParseIP", symbolPipe, "ResolveUnixAddr", "SplitHostPort"}},
		}
	case catalogNetHttp:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectTransport, effectSelectors: httpEffectSymbols(), pureSelectors: httpPureSymbols(), secondary: EffectFilesystem, secondarySelectors: []string{symbolServeFile, symbolServeFileFS}},
		}
	}
	return nil
}
func standardFunctionRules3(importPath string) []standardSymbolRule {
	switch importPath {
	case "runtime":
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectHost, effectSelectors: []string{"CPUProfile", "GOMAXPROCS", "GOROOT", "MemProfile", "NumCPU", "NumCgoCall", "ReadMemStats", "SetCPUProfileRate", "StartTrace", "StopTrace", "ThreadCreateProfile"}},
		}
	case timeContractText:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectTime, effectSelectors: []string{"Since", "Until", symbolAfter, symbolAfterFunc, "NewTicker", "NewTimer", "Now", "Sleep", "Tick"}, pureSelectors: []string{symbolDate, "FixedZone", "LoadLocationFromTZData", symbolParse, "ParseDuration", "ParseInLocation", symbolUnix, "UnixMicro", "UnixMilli"}},
			{importPath: importPath, effect: EffectHost, effectSelectors: []string{"LoadLocation"}},
		}
	case standardPackageSyscall:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: syscallFilesystemSymbols()},
			{importPath: importPath, effect: EffectLocking, effectSelectors: syscallLockingSymbols()},
			{importPath: importPath, effect: EffectTransport, effectSelectors: syscallTransportSymbols()},
			{importPath: importPath, effect: EffectProcess, effectSelectors: syscallProcessSymbols()},
		}
	case unixPackagePath:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: syscallFilesystemSymbols()},
			{importPath: importPath, effect: EffectHost, effectSelectors: unixHostSymbols()},
			{importPath: importPath, effect: EffectLocking, effectSelectors: syscallLockingSymbols()},
			{importPath: importPath, effect: EffectTransport, effectSelectors: syscallTransportSymbols()},
			{importPath: importPath, effect: EffectProcess, effectSelectors: syscallProcessSymbols()},
		}
	case windowsPackagePath:
		return []standardSymbolRule{
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: syscallFilesystemSymbols()},
			{importPath: importPath, effect: EffectFilesystem, effectSelectors: windowsFilesystemSymbols()},
			{importPath: importPath, effect: EffectHost, effectSelectors: windowsHostSymbols()},
			{importPath: importPath, effect: EffectLocking, effectSelectors: windowsLockingSymbols()},
			{importPath: importPath, effect: EffectTransport, effectSelectors: syscallTransportSymbols()},
			{importPath: importPath, effect: EffectProcess, effectSelectors: syscallProcessSymbols()},
		}
	case standardPackageBuiltin:
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"append", "cap", "clear", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "real", "recover"}, contextualSelectors: []string{"close", "panic", "print", "println"}},
		}
	case "crypto/sha256":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"New", "New224", "Sum224", "Sum256"}},
		}
	case "crypto/sha512":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"New", "New384", "New512_224", "New512_256", "Sum384", "Sum512", "Sum512_224", "Sum512_256"}},
		}
	}
	return nil
}
func standardFunctionRules4(importPath string) []standardSymbolRule {
	switch importPath {
	case "context":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"Background", "TODO", "WithValue", "WithoutCancel"}, contextualSelectors: []string{"WithCancel", "WithCancelCause", "WithDeadline", "WithDeadlineCause", "WithTimeout", "WithTimeoutCause", symbolAfterFunc, "Cause"}},
		}
	case "errors":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"New", symbolJoin}, contextualSelectors: []string{"Is", "As", "AsType", "Unwrap"}},
		}
	case "encoding/json":
		return []standardSymbolRule{
			{importPath: importPath, contextualSelectors: []string{"Marshal", "MarshalIndent", "Unmarshal"}, pureSelectors: []string{"Valid", "NewDecoder", "NewEncoder"}},
		}
	case "reflect":
		return []standardSymbolRule{
			{importPath: importPath, pureSelectors: []string{"TypeOf", "TypeFor", "ValueOf"}},
		}
	}
	return nil
}
func standardMethodRules(importPath string) []standardMethodRule {
	if rules := standardReceiverRules1(importPath); rules != nil {
		return rules
	}
	if rules := standardReceiverRules2(importPath); rules != nil {
		return rules
	}
	return nil
}
func standardReceiverRules1(importPath string) []standardMethodRule {
	switch importPath {
	case standardPackageBuiltin:
		return []standardMethodRule{
			{importPath: importPath, receiver: "error", selectors: []string{symbolError}, disposition: StandardSymbolContextual},
		}
	case "os":
		return []standardMethodRule{
			{importPath: importPath, receiver: symbolFile, disposition: StandardSymbolEffect, effect: EffectFilesystem, selectors: []string{symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline}},
			{importPath: importPath, receiver: symbolRoot, disposition: StandardSymbolPure, selectors: []string{symbolName, "FS"}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolFile, effect: EffectFilesystem, selectors: []string{"Chdir", symbolChmod, symbolChown, symbolClose, symbolRead, "ReadAt", "ReadDir", symbolReadFrom, "Readdir", "Readdirnames", "Seek", symbolStat, symbolSync, "Truncate", symbolWrite, "WriteAt", "WriteString", symbolWriteTo}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolRoot, effect: EffectFilesystem, selectors: []string{symbolChmod, symbolChown, symbolClose, symbolCreate, symbolChtimes, "Lchown", "Link", "Lstat", "Mkdir", symbolMkdirAll, "Open", symbolOpenFile, symbolOpenRoot, symbolReadFile, "Readlink", symbolRemove, symbolRemoveAll, "Rename", symbolStat, "Symlink", symbolWriteFile}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Process", effect: EffectProcess, selectors: []string{symbolKill, "Release", "Signal", symbolWait, "WithHandle"}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolFile, effect: EffectHost, selectors: []string{"Fd", symbolSyscallConn}},
			{disposition: StandardSymbolPure, importPath: importPath, receiver: symbolFile, selectors: []string{symbolName}},
		}
	case "net":
		return []standardMethodRule{
			{importPath: importPath, receiver: receiverNetConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolRead, symbolWrite, symbolClose, symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline, "SetReadBuffer", "SetWriteBuffer"}},
			{importPath: importPath, receiver: receiverNetConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolFile}},
			{importPath: importPath, receiver: receiverNetConn, disposition: StandardSymbolPure, selectors: []string{symbolLocalAddr, symbolRemoteAddr}},
			{importPath: importPath, receiver: symbolConn, disposition: StandardSymbolPure, selectors: []string{symbolLocalAddr, symbolRemoteAddr}},
			{importPath: importPath, receiver: symbolPacketConn, disposition: StandardSymbolPure, selectors: []string{symbolLocalAddr}},
			{importPath: importPath, receiver: symbolTCPConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolMultipathTCP}},
			{importPath: importPath, receiver: symbolTCPConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
			{importPath: importPath, receiver: symbolUDPConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolReadFrom, "ReadFromUDP", "ReadFromUDPAddrPort", "ReadMsgUDP", "ReadMsgUDPAddrPort", "WriteMsgUDP", "WriteMsgUDPAddrPort", symbolWriteTo, "WriteToUDP", "WriteToUDPAddrPort"}},
			{importPath: importPath, receiver: symbolUDPConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
			{importPath: importPath, receiver: symbolUnixConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolCloseRead, symbolCloseWrite, symbolReadFrom, "ReadFromUnix", "ReadMsgUnix", "WriteMsgUnix", symbolWriteTo, "WriteToUnix"}},
			{importPath: importPath, receiver: symbolUnixConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
			{importPath: importPath, receiver: symbolIPConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolReadFrom, "ReadFromIP", "ReadMsgIP", "WriteMsgIP", symbolWriteTo, "WriteToIP"}},
			{importPath: importPath, receiver: symbolIPConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
			{importPath: importPath, receiver: symbolDialer, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolDialIP, symbolDialTCP, symbolDialUDP, symbolDialUnix}},
			{importPath: importPath, receiver: symbolDialer, disposition: StandardSymbolPure, selectors: []string{symbolMultipathTCP, symbolSetMultipathTCP}},
			{importPath: importPath, receiver: symbolListenConfig, disposition: StandardSymbolPure, selectors: []string{symbolMultipathTCP, symbolSetMultipathTCP}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolConn, effect: EffectTransport, selectors: []string{symbolClose, symbolRead, symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline, symbolWrite}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Listener", effect: EffectTransport, selectors: []string{symbolAccept, "Addr", symbolClose}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolPacketConn, effect: EffectTransport, selectors: []string{symbolClose, symbolReadFrom, symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline, symbolWriteTo}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolDialer, effect: EffectTransport, selectors: []string{symbolDial, "DialContext"}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolListenConfig, effect: EffectTransport, selectors: []string{"Listen", symbolListenPacket}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolTCPConn, effect: EffectTransport, selectors: []string{symbolCloseRead, symbolCloseWrite, symbolReadFrom, "SetKeepAlive", "SetKeepAliveConfig", "SetKeepAlivePeriod", "SetLinger", "SetNoDelay", symbolWriteTo}},
		}
	case catalogNetHttp:
		return []standardMethodRule{
			{importPath: importPath, receiver: symbolHeader, disposition: StandardSymbolContextual, selectors: []string{symbolWrite, "WriteSubset"}},
			{importPath: importPath, receiver: symbolResponseWriter, disposition: StandardSymbolPure, selectors: []string{symbolHeader}},
			{importPath: importPath, receiver: symbolServer, disposition: StandardSymbolPure, selectors: []string{"RegisterOnShutdown"}},
			{importPath: importPath, receiver: symbolServer, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{"SetKeepAlivesEnabled"}},
			{importPath: importPath, receiver: symbolTransport, disposition: StandardSymbolPure, selectors: []string{symbolClone, "RegisterProtocol"}},
			{importPath: importPath, receiver: symbolTransport, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{"NewClientConn"}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Client", effect: EffectTransport, selectors: []string{symbolCloseIdleConnections, "Do", "Get", symbolHead, symbolPost, symbolPostForm}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Flusher", effect: EffectTransport, selectors: []string{symbolFlush}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Hijacker", effect: EffectTransport, selectors: []string{symbolHijack}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolResponseWriter, effect: EffectTransport, selectors: []string{symbolWrite, "WriteHeader"}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "ResponseController", effect: EffectTransport, selectors: []string{"EnableFullDuplex", symbolFlush, symbolHijack, symbolSetReadDeadline, symbolSetWriteDeadline}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolRequest, effect: EffectTransport, selectors: []string{"FormFile", "FormValue", "ParseForm", "ParseMultipartForm", "PostFormValue"}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "RoundTripper", effect: EffectTransport, selectors: []string{symbolRoundTrip}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolServer, effect: EffectTransport, selectors: []string{symbolClose, symbolListenAndServe, symbolListenAndServeTLS, symbolServe, symbolServeTLS, symbolShutdown}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolTransport, effect: EffectTransport, selectors: []string{"CancelRequest", symbolCloseIdleConnections, symbolRoundTrip}},
			{disposition: StandardSymbolPure, importPath: importPath, receiver: symbolHeader, selectors: []string{"Get", "Set", "Add", "Del", "Values", symbolClone}},
		}
	case timeContractText:
		return []standardMethodRule{
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Ticker", effect: EffectTime, selectors: []string{symbolReset, symbolStop}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Timer", effect: EffectTime, selectors: []string{symbolReset, symbolStop}},
			{disposition: StandardSymbolPure, importPath: importPath, receiver: "Time", selectors: []string{symbolString, "Format", "AppendFormat", "IsZero", "Equal", "Before", symbolAfter, "Compare", "Add", "Sub", "AddDate", "UTC", "Local", "In", symbolDate, "Clock", symbolUnix, "UnixNano"}},
		}
	case catalogMathRandV2:
		return []standardMethodRule{
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "ChaCha8", effect: EffectEntropy, selectors: []string{symbolRead, symbolUint64}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "PCG", effect: EffectEntropy, selectors: []string{symbolUint64}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolRand, effect: EffectEntropy, selectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt32, symbolInt32N, symbolInt64, symbolInt64N, symbolIntN, symbolNormFloat64, symbolPerm, symbolShuffle, symbolUint, symbolUint32, symbolUint32N, symbolUint64, symbolUint64N, symbolUintN}},
		}
	case catalogOsExec:
		return []standardMethodRule{
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Cmd", effect: EffectHost, selectors: []string{symbolEnviron}},
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "Cmd", effect: EffectProcess, selectors: []string{"CombinedOutput", "Output", "Run", "Start", "StderrPipe", "StdinPipe", "StdoutPipe", symbolWait}},
			{disposition: StandardSymbolPure, importPath: importPath, receiver: "Cmd", selectors: []string{symbolString}},
		}
	case "syscall":
		return []standardMethodRule{
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: "RawConn", effect: EffectHost, selectors: []string{"Control", symbolRead, symbolWrite}},
		}
	}
	return nil
}
func standardReceiverRules2(importPath string) []standardMethodRule {
	switch importPath {
	case catalogMathRand:
		return []standardMethodRule{
			{disposition: StandardSymbolEffect, importPath: importPath, receiver: symbolRand, effect: EffectEntropy, selectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt31, symbolInt31n, symbolInt63, symbolInt63n, symbolIntn, symbolNormFloat64, symbolPerm, symbolRead, symbolSeed, symbolShuffle, symbolUint32, symbolUint64}},
		}
	}
	return nil
}
