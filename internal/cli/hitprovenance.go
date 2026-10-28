package cli

import (
	"strings"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
)

// retractionMarker is the text a printed hit gets when a work it cites is
// retracted (DR-0080 item 4): one " [retracted source: TITLE]" for each such
// work, or "" when there is none. It reports; it never changes a hit's place in
// the results or hides it.
func retractionMarker(p knowledge.HitProvenance) string {
	var b strings.Builder
	for _, s := range p.Sources {
		if s.Retracted {
			b.WriteString(" [retracted source: " + s.Title + "]")
		}
	}
	return b.String()
}

// jsonSource and jsonMaterial are the --json shapes of a hit's provenance in
// the scoped search and in concept recall; the unscoped search prints the
// library's own structs, as it does for every other field.
type jsonSource struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	IdentifierType  string `json:"identifier_type,omitempty"`
	IdentifierValue string `json:"identifier_value,omitempty"`
	PublishedDate   string `json:"published_date,omitempty"`
	Version         string `json:"version,omitempty"`
	Rights          string `json:"rights,omitempty"`
	Retracted       bool   `json:"retracted"`
	RetractionNote  string `json:"retraction_note,omitempty"`
	Relationship    string `json:"relationship"`
	SourceVersion   string `json:"source_version,omitempty"`
	SourceHash      string `json:"source_hash,omitempty"`
}

type jsonMaterial struct {
	Path       string    `json:"path"`
	Checksum   string    `json:"checksum"`
	Format     string    `json:"format"`
	IngestedAt time.Time `json:"ingested_at"`
}

// jsonSources converts a hit's sources; an empty list is [] and not null.
func jsonSources(p knowledge.HitProvenance) []jsonSource {
	out := make([]jsonSource, 0, len(p.Sources))
	for _, s := range p.Sources {
		out = append(out, jsonSource{s.ID, s.Title, s.IdentifierType, s.IdentifierValue, s.PublishedDate, s.Version, s.Rights,
			s.Retracted, s.RetractionNote, s.Relationship, s.SourceVersion, s.SourceHash})
	}
	return out
}

func jsonMaterialOf(p knowledge.HitProvenance) *jsonMaterial {
	if p.Material == nil {
		return nil
	}
	return &jsonMaterial{p.Material.Path, p.Material.Checksum, p.Material.Format, p.Material.IngestedAt}
}
