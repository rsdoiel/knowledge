package knowledge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// The corpus is the rows semantic search embeds: the text kb_fts indexes for
// observations, records, reviewed document summaries and concepts, narrowed by
// the same scope lexical search uses (semantic-search-plan.md T3.3).

func corpusKeys(rows []CorpusRow) string {
	var out []string
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%s:%d", r.SourceType, r.SourceID))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestEmbedCorpus_HoldsOnlyTheEmbeddedKinds(t *testing.T) {
	f := newScopedFixture(t)
	if _, err := f.kb.AddSource(Source{Title: "A cited paper", IdentifierType: "doi", IdentifierValue: "10.1/x"}); err != nil {
		t.Fatal(err)
	}
	rows, err := f.kb.EmbedCorpus(SearchScope{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		switch r.SourceType {
		case "observation", "record", "concept", "document_summary":
		default:
			t.Errorf("row of type %q is in the corpus", r.SourceType)
		}
		if strings.TrimSpace(r.Text) == "" {
			t.Errorf("%s:%d has no text", r.SourceType, r.SourceID)
		}
	}
	want := fmt.Sprintf("observation:%d,observation:%d,record:%d,record:%d,record:%d,concept:%d,concept:%d,concept:%d,concept:%d",
		f.obsAlpha, f.obsBeta, f.recAlpha, f.recBeta, f.recWork, f.conceptAlpha, f.conceptBoth, f.conceptWork, f.conceptOrphan)
	wantSorted := strings.Split(want, ",")
	sort.Strings(wantSorted)
	if got := corpusKeys(rows); got != strings.Join(wantSorted, ",") {
		t.Errorf("corpus = %s\nwant     %s", got, strings.Join(wantSorted, ","))
	}
}

func TestEmbedCorpus_TextIsWhatLexicalSearchIndexes(t *testing.T) {
	f := newScopedFixture(t)
	rows, _ := f.kb.EmbedCorpus(SearchScope{})
	for _, r := range rows {
		var body string
		err := f.kb.db.QueryRow(`SELECT body FROM kb_fts WHERE source_type = ? AND CAST(source_id AS INTEGER) = ?`,
			r.SourceType, r.SourceID).Scan(&body)
		if err != nil {
			t.Fatalf("%s:%d: %v", r.SourceType, r.SourceID, err)
		}
		if r.Text != body {
			t.Errorf("%s:%d text = %q, kb_fts body = %q", r.SourceType, r.SourceID, r.Text, body)
		}
	}
}

func TestEmbedCorpus_ADraftedSummaryIsNotInItButAReviewedOneIs(t *testing.T) {
	f := newScopedFixture(t)
	docID, _ := f.kb.AddDocument(Document{ProjectID: f.alpha, Title: "A Post", Format: "text", Path: "p.txt"})
	secID, _ := f.kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Heading: "Intro", Body: "text"})
	if err := f.kb.DraftDocumentSummary(secID, "a summary of the cache post", "human", nil); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("document_summary:%d", secID)
	rows, _ := f.kb.EmbedCorpus(SearchScope{})
	if strings.Contains(corpusKeys(rows), key) {
		t.Error("an unreviewed summary is in the corpus")
	}
	if err := f.kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatal(err)
	}
	rows, _ = f.kb.EmbedCorpus(SearchScope{})
	if !strings.Contains(corpusKeys(rows), key) {
		t.Error("a reviewed summary is not in the corpus")
	}
}

func TestEmbedCorpus_ScopeNarrowsTheRowsAsLexicalSearchDoes(t *testing.T) {
	f := newScopedFixture(t)
	rows, err := f.kb.EmbedCorpus(SearchScope{Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		fmt.Sprintf("observation:%d", f.obsAlpha), fmt.Sprintf("record:%d", f.recAlpha),
		fmt.Sprintf("concept:%d", f.conceptAlpha), fmt.Sprintf("concept:%d", f.conceptBoth),
	}
	sort.Strings(want)
	if got := corpusKeys(rows); got != strings.Join(want, ",") {
		t.Errorf("project alpha = %s, want %s", got, strings.Join(want, ","))
	}

	rows, _ = f.kb.EmbedCorpus(SearchScope{Workspace: true})
	want = []string{fmt.Sprintf("record:%d", f.recWork), fmt.Sprintf("concept:%d", f.conceptWork)}
	sort.Strings(want)
	if got := corpusKeys(rows); got != strings.Join(want, ",") {
		t.Errorf("workspace = %s, want %s", got, strings.Join(want, ","))
	}

	rows, _ = f.kb.EmbedCorpus(SearchScope{Project: "alpha", Kinds: []string{"records"}})
	if got := corpusKeys(rows); got != fmt.Sprintf("record:%d", f.recAlpha) {
		t.Errorf("alpha records = %s", got)
	}
}

func TestEmbedCorpus_AKindThatIsNotEmbeddedIsInvalidAndNamesTheChoices(t *testing.T) {
	f := newScopedFixture(t)
	for _, kind := range []string{"sources", "projects"} {
		_, err := f.kb.EmbedCorpus(SearchScope{Kinds: []string{kind}})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("kind %q: err = %v, want ErrInvalid", kind, err)
		}
		for _, k := range EmbeddedKinds {
			if !strings.Contains(err.Error(), k) {
				t.Errorf("error %q does not name %q", err, k)
			}
		}
	}
	if _, err := f.kb.EmbedCorpus(SearchScope{Kinds: []string{"bogus"}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: err = %v, want ErrInvalid", err)
	}
}

func TestEmbedCorpus_UnknownProjectIsNotFound(t *testing.T) {
	f := newScopedFixture(t)
	if _, err := f.kb.EmbedCorpus(SearchScope{Project: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestEmbedCorpus_ASupersededOrDeletedRowLeavesIt(t *testing.T) {
	f := newScopedFixture(t)
	if _, err := f.kb.DeleteObservation(f.obsBeta, true); err != nil {
		t.Fatal(err)
	}
	rows, _ := f.kb.EmbedCorpus(SearchScope{})
	if strings.Contains(corpusKeys(rows), fmt.Sprintf("observation:%d", f.obsBeta)) {
		t.Error("a deleted observation is still in the corpus")
	}
}
