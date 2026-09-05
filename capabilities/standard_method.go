package capabilities

import "slices"

// standardMethodRule binds the declaring receiver type, never a variable name
// or an imported package alias. A zero effect explicitly marks pure methods.
type standardMethodRule struct {
	importPath  string
	receiver    string
	selectors   []string
	effect      Effect
	disposition StandardSymbolDisposition
}

func resolveStandardMethod(symbol StandardSymbol) (StandardSymbolFact, error) {
	return resolveMethodRules(symbol, standardMethodRules())
}

func resolveMethodRules(symbol StandardSymbol, rules []standardMethodRule) (StandardSymbolFact, error) {
	fact := StandardSymbolFact{Symbol: symbol, Disposition: StandardSymbolUnresolved}
	for _, rule := range rules {
		if rule.importPath != symbol.ImportPath.String() || rule.receiver != symbol.Receiver.String() || !slices.Contains(rule.selectors, symbol.Selector.String()) {
			continue
		}
		candidate := Classification{Effect: rule.effect, Disposition: rule.disposition}
		if err := mergeClassification(&fact.Classification, candidate); err != nil {
			return StandardSymbolFact{}, err
		}
	}
	return fact, fact.Validate()
}

func standardMethodRules() []standardMethodRule {
	return []standardMethodRule{
		{importPath: standardPackageBuiltin, receiver: "error", selectors: []string{symbolError}, disposition: StandardSymbolContextual},
		{importPath: "os", receiver: symbolFile, disposition: StandardSymbolEffect, effect: EffectFilesystem, selectors: []string{symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline}},
		{importPath: "os", receiver: symbolRoot, disposition: StandardSymbolPure, selectors: []string{symbolName, "FS"}},
		{importPath: "net", receiver: receiverNetConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolRead, symbolWrite, symbolClose, symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline, "SetReadBuffer", "SetWriteBuffer"}},
		{importPath: "net", receiver: receiverNetConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolFile}},
		{importPath: "net", receiver: receiverNetConn, disposition: StandardSymbolPure, selectors: []string{symbolLocalAddr, symbolRemoteAddr}},
		{importPath: "net", receiver: symbolConn, disposition: StandardSymbolPure, selectors: []string{symbolLocalAddr, symbolRemoteAddr}},
		{importPath: "net", receiver: symbolPacketConn, disposition: StandardSymbolPure, selectors: []string{symbolLocalAddr}},
		{importPath: "net", receiver: symbolTCPConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolMultipathTCP}},
		{importPath: "net", receiver: symbolTCPConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
		{importPath: "net", receiver: symbolUDPConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolReadFrom, "ReadFromUDP", "ReadFromUDPAddrPort", "ReadMsgUDP", "ReadMsgUDPAddrPort", "WriteMsgUDP", "WriteMsgUDPAddrPort", symbolWriteTo, "WriteToUDP", "WriteToUDPAddrPort"}},
		{importPath: "net", receiver: symbolUDPConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
		{importPath: "net", receiver: symbolUnixConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolCloseRead, symbolCloseWrite, symbolReadFrom, "ReadFromUnix", "ReadMsgUnix", "WriteMsgUnix", symbolWriteTo, "WriteToUnix"}},
		{importPath: "net", receiver: symbolUnixConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
		{importPath: "net", receiver: symbolIPConn, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolReadFrom, "ReadFromIP", "ReadMsgIP", "WriteMsgIP", symbolWriteTo, "WriteToIP"}},
		{importPath: "net", receiver: symbolIPConn, disposition: StandardSymbolEffect, effect: EffectHost, selectors: []string{symbolSyscallConn}},
		{importPath: "net", receiver: symbolDialer, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{symbolDialIP, symbolDialTCP, symbolDialUDP, symbolDialUnix}},
		{importPath: "net", receiver: symbolDialer, disposition: StandardSymbolPure, selectors: []string{symbolMultipathTCP, symbolSetMultipathTCP}},
		{importPath: "net", receiver: symbolListenConfig, disposition: StandardSymbolPure, selectors: []string{symbolMultipathTCP, symbolSetMultipathTCP}},
		{importPath: catalogNetHttp, receiver: symbolHeader, disposition: StandardSymbolContextual, selectors: []string{symbolWrite, "WriteSubset"}},
		{importPath: catalogNetHttp, receiver: symbolResponseWriter, disposition: StandardSymbolPure, selectors: []string{symbolHeader}},
		{importPath: catalogNetHttp, receiver: symbolServer, disposition: StandardSymbolPure, selectors: []string{"RegisterOnShutdown"}},
		{importPath: catalogNetHttp, receiver: symbolServer, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{"SetKeepAlivesEnabled"}},
		{importPath: catalogNetHttp, receiver: symbolTransport, disposition: StandardSymbolPure, selectors: []string{symbolClone, "RegisterProtocol"}},
		{importPath: catalogNetHttp, receiver: symbolTransport, disposition: StandardSymbolEffect, effect: EffectTransport, selectors: []string{"NewClientConn"}},
		{disposition: StandardSymbolEffect, importPath: "os", receiver: symbolFile, effect: EffectFilesystem, selectors: []string{"Chdir", symbolChmod, symbolChown, symbolClose, symbolRead, "ReadAt", "ReadDir", symbolReadFrom, "Readdir", "Readdirnames", "Seek", symbolStat, symbolSync, "Truncate", symbolWrite, "WriteAt", "WriteString", symbolWriteTo}},
		{disposition: StandardSymbolEffect, importPath: "os", receiver: symbolRoot, effect: EffectFilesystem, selectors: []string{symbolChmod, symbolChown, symbolClose, symbolCreate, symbolChtimes, "Lchown", "Link", "Lstat", "Mkdir", symbolMkdirAll, "Open", symbolOpenFile, symbolOpenRoot, symbolReadFile, "Readlink", symbolRemove, symbolRemoveAll, "Rename", symbolStat, "Symlink", symbolWriteFile}},
		{disposition: StandardSymbolEffect, importPath: "os", receiver: "Process", effect: EffectProcess, selectors: []string{symbolKill, "Release", "Signal", symbolWait, "WithHandle"}},
		{disposition: StandardSymbolEffect, importPath: "net", receiver: symbolConn, effect: EffectTransport, selectors: []string{symbolClose, symbolRead, symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline, symbolWrite}},
		{disposition: StandardSymbolEffect, importPath: "net", receiver: "Listener", effect: EffectTransport, selectors: []string{symbolAccept, "Addr", symbolClose}},
		{disposition: StandardSymbolEffect, importPath: "net", receiver: symbolPacketConn, effect: EffectTransport, selectors: []string{symbolClose, symbolReadFrom, symbolSetDeadline, symbolSetReadDeadline, symbolSetWriteDeadline, symbolWriteTo}},
		{disposition: StandardSymbolEffect, importPath: "net", receiver: symbolDialer, effect: EffectTransport, selectors: []string{symbolDial, "DialContext"}},
		{disposition: StandardSymbolEffect, importPath: "net", receiver: symbolListenConfig, effect: EffectTransport, selectors: []string{"Listen", symbolListenPacket}},
		{disposition: StandardSymbolEffect, importPath: "net", receiver: symbolTCPConn, effect: EffectTransport, selectors: []string{symbolCloseRead, symbolCloseWrite, symbolReadFrom, "SetKeepAlive", "SetKeepAliveConfig", "SetKeepAlivePeriod", "SetLinger", "SetNoDelay", symbolWriteTo}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: "Client", effect: EffectTransport, selectors: []string{symbolCloseIdleConnections, "Do", "Get", symbolHead, symbolPost, symbolPostForm}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: "Flusher", effect: EffectTransport, selectors: []string{symbolFlush}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: "Hijacker", effect: EffectTransport, selectors: []string{symbolHijack}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: symbolResponseWriter, effect: EffectTransport, selectors: []string{symbolWrite, "WriteHeader"}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: "ResponseController", effect: EffectTransport, selectors: []string{"EnableFullDuplex", symbolFlush, symbolHijack, symbolSetReadDeadline, symbolSetWriteDeadline}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: symbolRequest, effect: EffectTransport, selectors: []string{"FormFile", "FormValue", "ParseForm", "ParseMultipartForm", "PostFormValue"}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: "RoundTripper", effect: EffectTransport, selectors: []string{symbolRoundTrip}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: symbolServer, effect: EffectTransport, selectors: []string{symbolClose, symbolListenAndServe, symbolListenAndServeTLS, symbolServe, symbolServeTLS, symbolShutdown}},
		{disposition: StandardSymbolEffect, importPath: catalogNetHttp, receiver: symbolTransport, effect: EffectTransport, selectors: []string{"CancelRequest", symbolCloseIdleConnections, symbolRoundTrip}},
		{disposition: StandardSymbolEffect, importPath: timeContractText, receiver: "Ticker", effect: EffectTime, selectors: []string{symbolReset, symbolStop}},
		{disposition: StandardSymbolEffect, importPath: timeContractText, receiver: "Timer", effect: EffectTime, selectors: []string{symbolReset, symbolStop}},
		{disposition: StandardSymbolEffect, importPath: catalogMathRandV2, receiver: "ChaCha8", effect: EffectEntropy, selectors: []string{symbolRead, symbolUint64}},
		{disposition: StandardSymbolEffect, importPath: catalogMathRandV2, receiver: "PCG", effect: EffectEntropy, selectors: []string{symbolUint64}},
		{disposition: StandardSymbolEffect, importPath: catalogMathRandV2, receiver: symbolRand, effect: EffectEntropy, selectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt32, symbolInt32N, symbolInt64, symbolInt64N, symbolIntN, symbolNormFloat64, symbolPerm, symbolShuffle, symbolUint, symbolUint32, symbolUint32N, symbolUint64, symbolUint64N, symbolUintN}},
		{disposition: StandardSymbolEffect, importPath: "os", receiver: symbolFile, effect: EffectHost, selectors: []string{"Fd", symbolSyscallConn}},
		{disposition: StandardSymbolEffect, importPath: catalogOsExec, receiver: "Cmd", effect: EffectHost, selectors: []string{symbolEnviron}},
		{disposition: StandardSymbolEffect, importPath: catalogOsExec, receiver: "Cmd", effect: EffectProcess, selectors: []string{"CombinedOutput", "Output", "Run", "Start", "StderrPipe", "StdinPipe", "StdoutPipe", symbolWait}},
		{disposition: StandardSymbolEffect, importPath: "syscall", receiver: "RawConn", effect: EffectHost, selectors: []string{"Control", symbolRead, symbolWrite}},
		{disposition: StandardSymbolEffect, importPath: catalogMathRand, receiver: symbolRand, effect: EffectEntropy, selectors: []string{symbolExpFloat64, symbolFloat32, symbolFloat64, "Int", symbolInt31, symbolInt31n, symbolInt63, symbolInt63n, symbolIntn, symbolNormFloat64, symbolPerm, symbolRead, symbolSeed, symbolShuffle, symbolUint32, symbolUint64}},
		{disposition: StandardSymbolPure, importPath: "os", receiver: symbolFile, selectors: []string{symbolName}},
		{disposition: StandardSymbolPure, importPath: catalogOsExec, receiver: "Cmd", selectors: []string{symbolString}},
		{disposition: StandardSymbolPure, importPath: catalogNetHttp, receiver: symbolHeader, selectors: []string{"Get", "Set", "Add", "Del", "Values", symbolClone}},
		{disposition: StandardSymbolPure, importPath: timeContractText, receiver: "Time", selectors: []string{symbolString, "Format", "AppendFormat", "IsZero", "Equal", "Before", symbolAfter, "Compare", "Add", "Sub", "AddDate", "UTC", "Local", "In", symbolDate, "Clock", symbolUnix, "UnixNano"}},
	}
}
