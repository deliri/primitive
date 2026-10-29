//go:build linux

package process

import (
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func observeMemoryCap(owner memoryCapOwner) MemoryCapObservation {
	unknown := MemoryCapObservation{Limit: owner.limit}
	membership, ok := memoryCapMembership(owner)
	if !ok {
		return unknown
	}
	directory, ok := openMemoryCapDirectory(membership)
	if !ok {
		return unknown
	}
	defer directory.Close()
	maximum, ok := readMemoryCapFile(directory, "memory.max")
	if !ok || maximum != strconv.FormatUint(owner.limit.Uint64(), 10)+"\n" {
		return unknown
	}
	file, err := openMemoryCapFile(directory, "memory.events.local")
	if err != nil {
		return unknown
	}
	events, valid := readMemoryEvents(file)
	closeErr := file.Close()
	if !valid || closeErr != nil {
		return unknown
	}
	// Verify membership again while the command still has this exact owned
	// scope, rather than trusting a stale PID or another cgroup's counters.
	maximumAfter, sameLimit := readMemoryCapFile(directory, "memory.max")
	if !sameLimit || maximumAfter != maximum {
		return unknown
	}
	after, ok := memoryCapMembership(owner)
	if !ok || after != membership {
		return unknown
	}
	return MemoryCapObservation{Limit: owner.limit, State: MemoryCapObserved, LocalOOMEvents: events.oom, LocalOOMKills: events.kills}
}
func memoryCapMembership(owner memoryCapOwner) (string, bool) {
	if owner.pid <= 0 || owner.unit == "" {
		return "", false
	}
	file, err := os.Open("/proc/" + strconv.Itoa(int(owner.pid)) + "/cgroup")
	if err != nil {
		return "", false
	}
	raw, ok := readMemoryCapScalar(file)
	closeErr := file.Close()
	if !ok || closeErr != nil || !strings.HasPrefix(raw, "0::/") || !strings.HasSuffix(raw, "/"+string(owner.unit)+"\n") || strings.Count(raw, "\n") != 1 {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(raw, "0::"), "\n"), true
}
func openMemoryCapDirectory(path string) (*os.File, bool) {
	if !strings.HasPrefix(path, "/") {
		return nil, false
	}
	root, err := os.Open("/sys/fs/cgroup")
	if err != nil {
		return nil, false
	}
	var facts syscall.Statfs_t
	if syscall.Fstatfs(int(root.Fd()), &facts) != nil || facts.Type != 0x63677270 {
		root.Close()
		return nil, false
	}
	directory := root
	for component := range strings.SplitSeq(strings.TrimPrefix(path, "/"), "/") {
		if component == "" || component == "." || component == ".." {
			directory.Close()
			return nil, false
		}
		fd, err := syscall.Openat(int(directory.Fd()), component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		directory.Close()
		if err != nil {
			return nil, false
		}
		directory = os.NewFile(uintptr(fd), component)
	}
	return directory, true
}
func openMemoryCapFile(directory *os.File, name string) (*os.File, error) {
	fd, err := syscall.Openat(int(directory.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
func readMemoryCapFile(directory *os.File, name string) (string, bool) {
	file, err := openMemoryCapFile(directory, name)
	if err != nil {
		return "", false
	}
	raw, ok := readMemoryCapScalar(file)
	closeErr := file.Close()
	return raw, ok && closeErr == nil
}
func readMemoryCapScalar(source io.Reader) (string, bool) {
	// Linux proc cgroup paths are PATH_MAX bounded. One additional byte rejects
	// truncation; this never retains input proportional to source extent.
	var buffer [4097]byte
	n, err := io.ReadFull(source, buffer[:])
	if err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", false
	}
	return string(buffer[:n]), true
}
