package filestore

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// AppendAccess selects Go's native descriptor capabilities. The zero value
// preserves the ordinary write-only append contract; reads require intent.
type AppendAccess uint8

const (
	AppendAccessWriteOnly AppendAccess = iota
	AppendAccessReadWrite
)

func (a AppendAccess) IsValid() bool { return a == AppendAccessWriteOnly || a == AppendAccessReadWrite }
func (a AppendAccess) Validate() error {
	if !a.IsValid() {
		return core.ErrFilestoreContract
	}
	return nil
}
func (AppendAccess) OffWireEnum() {}
func (a AppendAccess) String() string {
	switch a {
	case AppendAccessWriteOnly:
		return "write-only"
	case AppendAccessReadWrite:
		return "read-write"
	default:
		return core.UnknownEnumDiagnostic
	}
}

func (a AppendAccess) nativeFlags() int {
	if a == AppendAccessReadWrite {
		return os.O_RDWR
	}
	return os.O_WRONLY
}
