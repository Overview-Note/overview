package openapi_test

import (
	"encoding/json"
	"testing"

	"github.com/Overview-Note/overview/internal/openapi"
)

func TestDocumentParses(t *testing.T) {
	spec, err := openapi.Document()
	if err != nil {
		t.Fatal(err)
	}
	if spec.Info.Title == "" || spec.Info.Version == "" {
		t.Fatalf("missing info: %+v", spec.Info)
	}
	if _, ok := spec.Paths["/note"]["put"]; !ok {
		t.Error("missing PUT /note")
	}
	if _, ok := spec.Paths["/tree"]["get"]; !ok {
		t.Error("missing GET /tree")
	}
	// Newly documented endpoints.
	for _, p := range []string{"/export", "/import", "/settings/ai"} {
		if _, ok := spec.Paths[p]; !ok {
			t.Errorf("missing path %s", p)
		}
	}
}

func TestJSONValid(t *testing.T) {
	raw, err := openapi.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("spec is not valid JSON: %v", err)
	}
}
