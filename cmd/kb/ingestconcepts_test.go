package main

import (
	"strings"
	"testing"
)

// `kb ingest` says which concepts it created, or with --dry-run would create
// (knowledge DR-0068, v0.0.20 U4). A [[wikilink]] or a tag that matches no concept
// creates one, so a typo adds a concept without a word; the plan of an ingest has
// to show that before it is applied.

func conceptNames(t *testing.T, s ingestSummary) string {
	t.Helper()
	return strings.Join(s.ConceptsCreated, ", ")
}

func TestIngestConcepts_ADryRunSaysWhatItWouldCreateAndCreatesNothing(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	if _, err := kb.AddConcept("Existing", ""); err != nil {
		t.Fatal(err)
	}
	testRecord{ID: "0001", Project: "clasm", Body: "\nWe use [[NewThing]] and [[existing]] and [[Computr]].\n", Tags: []string{"brand-new"}}.write(t, dir)
	s := ingestJSON(t, kb, dir, "--dry-run")
	if got := conceptNames(t, s); got != "NewThing, Computr, brand-new" {
		t.Errorf("would create %q; want NewThing, Computr, brand-new (the existing one, in any case, is not new)", got)
	}
	if cs, _ := kb.Concepts(); len(cs) != 1 {
		t.Errorf("a dry run created concepts: %d in the database", len(cs))
	}
}

func TestIngestConcepts_ARealRunSaysWhatItCreatedAndTheyExist(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Body: "\nSee [[Alpha]] and [[alpha]] again, and [[Beta]].\n"}.write(t, dir)
	s := ingestJSON(t, kb, dir)
	if got := conceptNames(t, s); got != "Alpha, Beta" {
		t.Errorf("created %q; want Alpha, Beta (a case variant is the same concept)", got)
	}
	for _, name := range []string{"Alpha", "Beta"} {
		if ok, _ := kb.HasConcept(name); !ok {
			t.Errorf("concept %s was reported created but is missing", name)
		}
	}
	// the files are unchanged now, so a second run creates nothing
	if again := ingestJSON(t, kb, dir); len(again.ConceptsCreated) != 0 {
		t.Errorf("a second run reported %v", again.ConceptsCreated)
	}
}

func TestIngestConcepts_AConceptTwoRecordsMentionIsListedOnce(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Body: "\n[[Shared]]\n"}.write(t, dir)
	testRecord{ID: "0002", Project: "clasm", Body: "\n[[shared]] and [[Own]]\n"}.write(t, dir)
	s := ingestJSON(t, kb, dir, "--dry-run")
	if got := conceptNames(t, s); got != "Shared, Own" {
		t.Errorf("would create %q; want Shared, Own", got)
	}
}

// A [[DR-0012]] is a record reference, not a concept: it is warned about and is not
// a concept to create.
func TestIngestConcepts_ARecordReferenceIsNotAConcept(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Body: "\nSee [[DR-0012]] and [[Real]].\n"}.write(t, dir)
	s := ingestJSON(t, kb, dir, "--dry-run")
	if got := conceptNames(t, s); got != "Real" {
		t.Errorf("would create %q; want only Real", got)
	}
}

func TestIngestConcepts_TheTextSaysSo(t *testing.T) {
	kb, root := openWorkspaceKB(t)
	dir := root + "/clasm/decisions"
	testRecord{ID: "0001", Project: "clasm", Body: "\n[[Alpha]] [[Beta]]\n"}.write(t, dir)
	var sb strings.Builder
	if err := cmdIngest(kb, nil, false, []string{dir, "--dry-run"}, &sb); err != nil {
		t.Fatal(err)
	}
	if want := "concept: would create Alpha, Beta"; !strings.Contains(sb.String(), want) {
		t.Errorf("the dry-run text lacks %q:\n%s", want, sb.String())
	}
	sb.Reset()
	if err := cmdIngest(kb, nil, false, []string{dir}, &sb); err != nil {
		t.Fatal(err)
	}
	if want := "concept: created Alpha, Beta"; !strings.Contains(sb.String(), want) {
		t.Errorf("the text lacks %q:\n%s", want, sb.String())
	}
}
