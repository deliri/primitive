package hostfacts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// hostnameMaximumBytes bounds a reported hostname at the DNS name ceiling,
// the one extent every platform's answer fits inside.
const hostnameMaximumBytes = 253

// Hostname is the platform's reported name for this host, bounded and free
// of control bytes. It is a label for diagnostics and device records, never
// a network identity claim: nothing here resolved it or proved it reachable.
type Hostname struct {
	value string
}

// Validate rejects the unset value.
func (h Hostname) Validate() error {
	if h.value == "" {
		return errors.Join(core.ErrHostFactsContract, errors.New("hostname is unset"))
	}
	return nil
}

// String returns the reported name.
func (h Hostname) String() string {
	return h.value
}

// admitHostname admits one platform-reported host name, or reports that the
// answer is outside the form a device record can carry. An empty, oversized,
// non-UTF-8, or control-carrying answer is a failed observation of the host,
// never a value, because a label built from it would carry bytes no admission
// downstream should ever meet.
func admitHostname(value string) (Hostname, error) {
	if value == "" || len(value) > hostnameMaximumBytes || !utf8.ValidString(value) ||
		strings.ContainsFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return Hostname{}, errors.Join(core.ErrHostFactsObservation, errors.New("platform hostname is outside the admitted form"))
	}
	return Hostname{value: value}, nil
}

// admitObservedPath admits one platform-reported base path. A host answer that
// is not an absolute path is a failed observation of the host, not a contract
// breach: nothing the caller controls produced it, so it carries the same
// observation identity an unusable hostname does rather than an ingress
// rejection the caller could have avoided.
func admitObservedPath(value string) (core.AbsolutePath, error) {
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return path, nil
}

// ObserveHostname reports the platform's name for this host. An empty,
// oversized, or control-carrying answer is a failed observation rather than
// a value, because a device record built from it would carry bytes no label
// admission downstream should ever meet.
func ObserveHostname() (Hostname, error) {
	value, err := os.Hostname()
	if err != nil {
		return Hostname{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return admitHostname(value)
}

// UserConfigDirectory reports the platform's per-user configuration base:
// the XDG, HOME, or AppData rules the standard library already encodes,
// admitted as an absolute path. Products place their own named subdirectory
// below it; this door owns only where the base is.
func UserConfigDirectory() (core.AbsolutePath, error) {
	value, err := os.UserConfigDir()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return admitObservedPath(value)
}

// UserHomeDirectory reports the platform's home directory for the current
// user: the HOME, USERPROFILE, or account-database rules the standard
// library already encodes, admitted as an absolute path. It is a base for
// platform-conventional entries a product must resolve, such as a tool
// cache a host tool already keeps there; a product's own data belongs
// under the configuration or cache base, not loose in the home.
func UserHomeDirectory() (core.AbsolutePath, error) {
	value, err := os.UserHomeDir()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return admitObservedPath(value)
}

// UserCacheDirectory reports the platform's per-user cache base: the XDG,
// Library, or AppData rules the standard library already encodes, admitted
// as an absolute path. Products place their own named subdirectory below
// it; this door owns only where the base is. Entries below it are
// disposable by the platform's own convention, so nothing load-bearing
// belongs there.
func UserCacheDirectory() (core.AbsolutePath, error) {
	value, err := os.UserCacheDir()
	if err != nil {
		return core.AbsolutePath{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return admitObservedPath(value)
}

// TemporaryDirectory reports the platform's temporary-file base for this
// process, admitted as an absolute path. It is where a product builds its
// own uniquely named scratch entries; uniqueness is the caller's to supply
// from keygen, because a name this door invented would be a hidden entropy
// source.
func TemporaryDirectory() (core.AbsolutePath, error) {
	return admitObservedPath(filepath.Clean(os.TempDir()))
}
