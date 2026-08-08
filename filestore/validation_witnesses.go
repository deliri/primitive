package filestore

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.OffWireEnum = InstallUnknown
	_ core.OffWireEnum = AppendUnknown
	_ core.OffWireEnum = WalkDirective(0)
	_ core.OffWireEnum = WalkOrder(0)
	_ core.OffWireEnum = SharingUnknown
)
