// Package lineio streams LF-delimited input in fixed-memory fragments through
// Go's bufio.Reader. It never imposes a line or stream length quota.
package lineio
