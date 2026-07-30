package objectstore

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.Validatable = Client{}
	_ core.Validatable = Transfer{}
	_ core.Validatable = VendorSpec{}
)
