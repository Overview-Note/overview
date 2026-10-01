// Package openapi embeds the OpenAPI specification and serves it as JSON.
package openapi

import (
	_ "embed"
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed spec.yaml
var specYAML []byte

var (
	once     sync.Once
	jsonSpec []byte
	convErr  error
)

// JSON returns the specification converted from YAML to JSON.
func JSON() ([]byte, error) {
	once.Do(func() {
		var doc any
		if err := yaml.Unmarshal(specYAML, &doc); err != nil {
			convErr = err
			return
		}
		jsonSpec, convErr = json.Marshal(doc)
	})
	return jsonSpec, convErr
}
