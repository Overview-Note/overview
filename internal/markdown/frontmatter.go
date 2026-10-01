package markdown

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

const delimiter = "---"

// Frontmatter is the YAML metadata block at the top of a note.
type Frontmatter struct {
	ID      string   `yaml:"id,omitempty"`
	Title   string   `yaml:"title,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Icon    string   `yaml:"icon,omitempty"`
	Public  bool     `yaml:"public,omitempty"`
	Created string   `yaml:"created,omitempty"`
	Updated string   `yaml:"updated,omitempty"`
}

// Document is a note split into its metadata and body.
type Document struct {
	Meta Frontmatter
	Body string
}

// Parse splits a markdown document into YAML frontmatter and body. Documents
// without a leading frontmatter block are returned with an empty Meta.
func Parse(raw string) Document {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, delimiter+"\n") {
		return Document{Body: normalized}
	}

	rest := normalized[len(delimiter)+1:]
	idx := strings.Index(rest, "\n"+delimiter)
	if idx < 0 {
		return Document{Body: normalized}
	}

	header := rest[:idx]
	body := rest[idx+1+len(delimiter):]
	body = strings.TrimLeft(body, "\n")

	var meta Frontmatter
	if err := yaml.Unmarshal([]byte(header), &meta); err != nil {
		return Document{Body: normalized}
	}
	return Document{Meta: meta, Body: body}
}

// String renders the document back into markdown with its frontmatter block.
func (d Document) String() string {
	var buf bytes.Buffer
	buf.WriteString(delimiter)
	buf.WriteString("\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(d.Meta)
	_ = enc.Close()
	buf.WriteString(delimiter)
	buf.WriteString("\n\n")
	buf.WriteString(strings.TrimLeft(d.Body, "\n"))
	return buf.String()
}
