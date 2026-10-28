package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// A retracted citation is reported on the hit, never used to rank or hide it
// (DR-0080 item 4).

type retractWorld struct {
	kb        *knowledge.KnowledgeBase
	alpha     int64
	retracted int64
	obs, rec  int64
}

func newRetractWorld(t *testing.T) *retractWorld {
	t.Helper()
	kb := openTestKB(t)
	w := &retractWorld{kb: kb}
	w.alpha, _ = kb.AddProject("alpha", "")
	w.retracted, _ = kb.AddSource(knowledge.Source{Title: "SPARQL 1.1", IdentifierType: "doi", IdentifierValue: "10.1234/sparql"})
	good, _ := kb.AddSource(knowledge.Source{Title: "A sound page", IdentifierType: "url", IdentifierValue: "https://example.org/a"})
	w.obs, _ = kb.AddObservation(w.alpha, "finding", "the marmalade result is reproducible")
	kb.AddObservation(w.alpha, "note", "another marmalade note, nothing cited")
	w.rec, _ = kb.AddRecord(knowledge.Record{RecordID: "0001", ProjectID: w.alpha, Scope: "project", Path: "a/0001.md",
		Title: "Marmalade policy", Date: "2026-10-01", Status: "accepted", Kind: "decision", Body: "\n**Decision.** Marmalade on toast.\n", Checksum: "c1"})
	for _, err := range []error{
		kb.LinkObservationSource(w.obs, w.retracted, "cited"),
		kb.LinkRecordSource(w.rec, good, "cited", "", ""),
		kb.LinkRecordSource(w.rec, w.retracted, "cited", "", ""),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	c, _ := kb.AddConcept("marmalade-study", "")
	kb.LinkObservationConcept(w.obs, c)
	return w
}

func (w *retractWorld) retract(t *testing.T) {
	t.Helper()
	if err := w.kb.RetractSource(w.retracted, "withdrawn by the publisher"); err != nil {
		t.Fatal(err)
	}
}

const marker = "[retracted source: SPARQL 1.1]"

func TestSearch_MarksAHitThatCitesARetractedSource(t *testing.T) {
	w := newRetractWorld(t)
	var before bytes.Buffer
	if err := cmdSearch(w.kb, nil, false, []string{"marmalade"}, &before); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(before.String(), "retracted") {
		t.Errorf("nothing is retracted yet:\n%s", before.String())
	}
	w.retract(t)
	var after bytes.Buffer
	if err := cmdSearch(w.kb, nil, false, []string{"marmalade"}, &after); err != nil {
		t.Fatalf("a retracted citation must not fail the search: %v", err)
	}
	if n := strings.Count(after.String(), marker); n != 2 {
		t.Errorf("marker appears %d times, want on the observation and the record:\n%s", n, after.String())
	}
	// The untouched note carries no marker.
	for _, line := range strings.Split(after.String(), "\n") {
		if strings.Contains(line, "nothing cited") && strings.Contains(line, "retracted") {
			t.Errorf("an uncited hit is marked: %q", line)
		}
	}
}

func TestSearchScoped_MarksAndTheOrderIsUnchanged(t *testing.T) {
	w := newRetractWorld(t)
	order := func() []int64 {
		res, err := w.kb.SearchScoped("marmalade", knowledge.SearchScope{Project: "alpha"})
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, r := range res.Results {
			ids = append(ids, r.SourceID)
		}
		return ids
	}
	before := order()
	w.retract(t)
	after := order()
	if len(before) != len(after) {
		t.Fatalf("a retraction changed the number of hits: %v -> %v", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("a retraction changed the ranking: %v -> %v", before, after)
			break
		}
	}
	var out bytes.Buffer
	if err := cmdSearch(w.kb, nil, false, []string{"alpha/", "marmalade"}, &out); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), marker); n != 2 {
		t.Errorf("scoped output marks %d hits, want 2:\n%s", n, out.String())
	}
}

func TestSearchJSON_CarriesTheProvenanceFields(t *testing.T) {
	w := newRetractWorld(t)
	w.retract(t)

	var scoped bytes.Buffer
	if err := cmdSearch(w.kb, nil, true, []string{"alpha/", "marmalade"}, &scoped); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Results []struct {
			Type      string `json:"source_type"`
			Retracted bool   `json:"retracted"`
			Sources   []struct {
				Title          string `json:"title"`
				Retracted      bool   `json:"retracted"`
				RetractionNote string `json:"retraction_note"`
				Relationship   string `json:"relationship"`
			} `json:"sources"`
			Material *struct {
				Path     string `json:"path"`
				Checksum string `json:"checksum"`
				Format   string `json:"format"`
			} `json:"material"`
		} `json:"results"`
	}
	if err := json.Unmarshal(scoped.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, r := range doc.Results {
		switch r.Type {
		case "record":
			seen++
			if !r.Retracted || len(r.Sources) != 2 || r.Material == nil || r.Material.Path != "a/0001.md" || r.Material.Checksum != "c1" {
				t.Errorf("record = %+v", r)
			}
		case "observation":
			if len(r.Sources) == 1 && r.Sources[0].Title == "SPARQL 1.1" {
				seen++
				if !r.Retracted || r.Sources[0].RetractionNote != "withdrawn by the publisher" || r.Material != nil {
					t.Errorf("observation = %+v", r)
				}
			} else if r.Sources == nil || len(r.Sources) != 0 {
				t.Errorf("an uncited observation lists %v, want []", r.Sources)
			}
		}
	}
	if seen != 2 {
		t.Errorf("saw %d cited hits in %s", seen, scoped.String())
	}

	var plain bytes.Buffer
	if err := cmdSearch(w.kb, nil, true, []string{"marmalade"}, &plain); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain.String(), `"Retracted": true`) || !strings.Contains(plain.String(), `"Sources"`) {
		t.Errorf("unscoped json lacks the library's provenance fields:\n%s", plain.String())
	}
}

func TestConceptRecall_MarksAndCarriesProvenanceInJSON(t *testing.T) {
	w := newRetractWorld(t)
	w.retract(t)
	out, err := runConceptCmd(t, w.kb, false, "recall", "--concept", "marmalade-study")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, marker) {
		t.Errorf("recall output lacks the marker:\n%s", out)
	}
	js, err := runConceptCmd(t, w.kb, true, "recall", "--concept", "marmalade-study")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Hits []struct {
			Retracted bool `json:"retracted"`
			Sources   []struct {
				Title string `json:"title"`
			} `json:"sources"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(js), &v); err != nil || len(v.Hits) != 1 || !v.Hits[0].Retracted ||
		len(v.Hits[0].Sources) != 1 || v.Hits[0].Sources[0].Title != "SPARQL 1.1" {
		t.Errorf("recall json = %+v, %v\n%s", v, err, js)
	}
}

func TestRetractionMarker_Helper(t *testing.T) {
	var p knowledge.HitProvenance
	if got := retractionMarker(p); got != "" {
		t.Errorf("no sources: %q", got)
	}
	p.Sources = []knowledge.SourceRef{{Title: "Fine"}, {Title: "Bad one", Retracted: true}, {Title: "Worse", Retracted: true}}
	p.Retracted = true
	if got := retractionMarker(p); got != " [retracted source: Bad one] [retracted source: Worse]" {
		t.Errorf("marker = %q", got)
	}
}

func TestHelp_DescribesTheRetractionMarkerAndProvenanceFields(t *testing.T) {
	for name, text := range map[string]string{"search": SearchHelpText, "concept": ConceptHelpText} {
		for _, want := range []string{"[retracted source: TITLE]", "DR-0080"} {
			if !strings.Contains(text, want) {
				t.Errorf("kb %s help does not mention %q", name, want)
			}
		}
	}
	if !strings.Contains(SearchHelpText, "material") || !strings.Contains(SearchHelpText, "never") && !strings.Contains(SearchHelpText, "does not rank") {
		t.Errorf("kb search help does not describe the provenance fields or that ranking is unchanged")
	}
}
