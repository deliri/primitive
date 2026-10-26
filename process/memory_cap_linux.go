//go:build linux

package process

import (
	"strconv"

	"github.com/deliri/primitive/v2026/core"
)

// memoryCapLauncher creates the capped scope and then replaces itself with
// the command, so the child keeps its process identity and group.
const memoryCapLauncher = "/usr/bin/systemd-run"

// cappedCommand runs path inside a transient user scope whose MemoryMax is
// max and whose swap is zero; a zero max runs path as it is.
func cappedCommand(max core.ByteLength, path string, arguments []string) (string, []string, error) {
	if max.Uint64() == 0 {
		return path, arguments, nil
	}
	limit := strconv.FormatUint(max.Uint64(), 10)
	argv := append([]string{"--user", "--scope", "--quiet", "--collect", "-p", "MemoryMax=" + limit, "-p", "MemorySwapMax=0", "--", path}, arguments...)
	return memoryCapLauncher, argv, nil
}
