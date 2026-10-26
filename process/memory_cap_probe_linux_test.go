//go:build linux

package process_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

// Probe: a user systemd scope with MemoryMax kills a shell holding ~300 MB
// under a 64 MiB cap and lets the same shell finish under 1 GiB.
func TestMemoryCapProbeUserScope(t *testing.T) {
	for _, row := range []struct {
		max      string
		wantPass bool
	}{{"64M", false}, {"1G", true}} {
		var stderr bytes.Buffer
		request := processRequest(t, "copy", process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: &stderr})
		request.Command = absolutePath(t, "/usr/bin/systemd-run")
		request.Arguments = arguments(t, "--user", "--scope", "--quiet", "--collect", "-p", "MemoryMax="+row.max, "-p", "MemorySwapMax=0", "--", "/bin/sh", "-c", "head -c 300000000 /dev/zero | tail -c 300000000 > /dev/null")
		result, err := process.Run(t.Context(), request)
		t.Logf("MemoryMax=%s: result=%+v err=%v stderr=%q", row.max, result, err, stderr.String())
		code, codeErr := result.ExitCode()
		success, _ := code.Success()
		passed := err == nil && codeErr == nil && success
		if passed != row.wantPass {
			t.Errorf("MemoryMax=%s passed=%v, want %v", row.max, passed, row.wantPass)
		}
	}
}
