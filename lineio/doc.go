// Package lineio streams exact LF-delimited byte fragments through Go's
// bufio.Reader and positioned UTF-8 characters through Go's text/scanner.
// It never imposes a line, token, file, or stream length quota.
package lineio
