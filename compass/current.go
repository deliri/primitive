package compass

import (
	_ "embed"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

//go:embed config.json
var currentConfigurationJSON string

// Configuration is the reusable project-only Compass document. A product that
// owns additional human declarations defines its own validated document with a
// Project field and decodes it through Decode.
type Configuration struct {
	Project Project `json:"project"`
}

// Validate proves every authored section before the configuration escapes.
func (c Configuration) Validate() error { return c.Project.Validate() }

// Current returns Primitive's configuration from the sole authored source at
// compass/config.json. The embedded bytes make the same declaration available
// after build without a generated version constant or runtime path.
func Current() (Configuration, error) {
	return Decode[Configuration](strings.NewReader(currentConfigurationJSON))
}

var _ core.Validatable = Configuration{}
