// Copyright 2020 The Go Authors. All rights reserved.
// Derived from golang.org/x/tools/internal/typesinternal.SetUsesCgo at
// 18332fec72972efbb8ab9881984fec2d8cfc2b58. See cgo_config_license.txt.

package gotoolchain

import (
	"go/types"
	"reflect"
)

// configureCgo enables the same Go checker mode used by x/tools for original
// cgo syntax paired with cmd/cgo's generated declarations. Go 1.27 keeps this
// setting private. A changed field shape is refused, never silently replaced
// with FakeImportC, which would suppress checking of missing C declarations.
func configureCgo(configuration *types.Config) bool {
	if configuration == nil {
		return false
	}
	field := reflect.ValueOf(configuration).Elem().FieldByName("go115UsesCgo")
	if !field.IsValid() || field.Kind() != reflect.Bool || !field.CanAddr() {
		return false
	}
	*(*bool)(field.Addr().UnsafePointer()) = true
	return true
}
