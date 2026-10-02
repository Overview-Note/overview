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

// Spec is a minimal projection of the OpenAPI document used to derive tool
// surfaces (e.g. the MCP server) from the published schema.
type Spec struct {
	Info  Info                     `yaml:"info"`
	Paths map[string]map[string]Op `yaml:"paths"`
}

// Info is the OpenAPI info block.
type Info struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

// Op is a single operation (HTTP method) on a path.
type Op struct {
	Summary     string       `yaml:"summary"`
	Description string       `yaml:"description"`
	Tags        []string     `yaml:"tags"`
	Parameters  []Parameter  `yaml:"parameters"`
	RequestBody *RequestBody `yaml:"requestBody"`
	OperationID string       `yaml:"operationId"`
}

// Parameter is an OpenAPI parameter (query/path/header).
type Parameter struct {
	Name     string         `yaml:"name"`
	In       string         `yaml:"in"`
	Required bool           `yaml:"required"`
	Schema   map[string]any `yaml:"schema"`
}

// RequestBody is an OpenAPI request body.
type RequestBody struct {
	Required bool                 `yaml:"required"`
	Content  map[string]MediaType `yaml:"content"`
}

// MediaType holds the schema for a content type.
type MediaType struct {
	Schema map[string]any `yaml:"schema"`
}

var (
	specOnce sync.Once
	parsed   Spec
	specErr  error
)

// Document parses and returns the embedded specification.
func Document() (Spec, error) {
	specOnce.Do(func() {
		specErr = yaml.Unmarshal(specYAML, &parsed)
	})
	return parsed, specErr
}
