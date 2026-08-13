package objectstore

import (
	"fmt"

	"github.com/deliri/primitive/v2026/core"
)

var (
	_ core.Validatable = Client{}
	_ core.Validatable = Transfer{}
	_ core.Validatable = VendorSpec{}
	_ fmt.Formatter    = SignedURL{}
	_ fmt.Formatter    = SignedHeader{}
	_ fmt.Formatter    = SignedHeaders{}
	_ fmt.Formatter    = UploadTarget{}
	_ fmt.Formatter    = DownloadTarget{}
)
