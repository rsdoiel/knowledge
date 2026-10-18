package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// kb source link, list, show, remove and unlink for documents and records
// (DR-0081 items 3, 4 and 7).

type citeWorld struct {
	kb       *knowledge.KnowledgeBase
	pid, doc int64
	rec, src int64
	obs      int64
}

func newCiteWorld(t *testing.T) *citeWorld {
	t.Helper()
	kb := openTestKB(t)
	w := &citeWorld{kb: kb}
	w.pid, _ = kb.AddProject("alpha", "")
	w.doc, _ = kb.AddDocument(knowledge.Document{ProjectID: w.pid, Title: "A Story", Format: "markdown", Path: "story.md", Checksum: "dk"})
	w.rec, _ = kb.AddRecord(knowledge.Record{RecordID: "0001", ProjectID: w.pid, Scope: "project", Path: "a/0001.md", Title: "R",
		Date: "2026-10-18", Status: "accepted", Kind: "decision", Body: "b", Checksum: "c"})
	w.src, _ = kb.AddSource(knowledge.Source{Title: "SPARQL 1.1", IdentifierType: "doi", IdentifierValue: "10.1234/sparql"})
	w.obs, _ = kb.AddObservation(w.pid, "note", "cites the paper")
	return w
}

func (w *citeWorld) source(t *testing.T, jsonOut bool, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmdSource(w.kb, nil, jsonOut, args, &out)
	return out.String(), err
}

func TestSourceLink_DocumentRecordAndObservationForms(t *testing.T) {
	w := newCiteWorld(t)
	src := fmt.Sprint(w.src)

	out, err := w.source(t, false, "link", "--document", fmt.Sprint(w.doc), src, "--relationship", "retrieved", "--version", "2nd ed", "--hash", "sha256:aa")
	if err != nil {
		t.Fatalf("link --document: %v", err)
	}
	if !strings.Contains(out, "linked document") || !strings.Contains(out, "(retrieved)") {
		t.Errorf("output = %q", out)
	}
	dl, _ := w.kb.DocumentSources(w.doc)
	if len(dl) != 1 || dl[0].Relationship != "retrieved" || dl[0].SourceVersion != "2nd ed" || dl[0].SourceHash != "sha256:aa" {
		t.Errorf("document links = %+v", dl)
	}

	if _, err := w.source(t, false, "link", "--record", "alpha/DR-0001", src); err != nil {
		t.Fatalf("link --record: %v", err)
	}
	rl, _ := w.kb.RecordSources(w.rec)
	if len(rl) != 1 || rl[0].Relationship != "cited" {
		t.Errorf("record links = %+v", rl)
	}

	if _, err := w.source(t, false, "link", fmt.Sprint(w.obs), src, "--version", "v3", "--hash", "sha256:cc"); err != nil {
		t.Fatalf("link OBS: %v", err)
	}
	ol, _ := w.kb.ObservationSourceLinks(w.obs)
	if len(ol) != 1 || ol[0].SourceVersion != "v3" || ol[0].SourceHash != "sha256:cc" {
		t.Errorf("observation links = %+v", ol)
	}

	js, err := w.source(t, true, "link", "--document", fmt.Sprint(w.doc), src)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if json.Unmarshal([]byte(js), &v) != nil || v["document_id"] == nil || v["source_id"] == nil {
		t.Errorf("json = %s", js)
	}
}

func TestSourceLink_ErrorClasses(t *testing.T) {
	w := newCiteWorld(t)
	src, doc := fmt.Sprint(w.src), fmt.Sprint(w.doc)
	for _, tc := range []struct {
		name string
		args []string
		want exitClass
	}{
		{"unknown document", []string{"link", "--document", "999", src}, classNegative},
		{"unknown record", []string{"link", "--record", "alpha/DR-9999", src}, classNegative},
		{"unknown source", []string{"link", "--document", doc, "999"}, classNegative},
		{"unknown observation", []string{"link", "999", src}, classNegative},
		{"document and record together", []string{"link", "--document", doc, "--record", "alpha/DR-0001", src}, classUsage},
		{"document with an observation id", []string{"link", "--document", doc, src, "5"}, classUsage},
		{"bad document id", []string{"link", "--document", "abc", src}, classUsage},
		{"blank version", []string{"link", "--document", doc, src, "--version", "  "}, classUsage},
		{"blank hash", []string{"link", "--document", doc, src, "--hash", ""}, classUsage},
		{"missing source id", []string{"link", "--document", doc}, classUsage},
		{"unknown flag", []string{"link", "--document", doc, src, "--bogus", "x"}, classUsage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := w.source(t, false, tc.args...)
			if err == nil {
				t.Fatalf("kb source %v succeeded, want %v", tc.args, tc.want)
			}
			if got := exitCodeFor(err); got != tc.want {
				t.Errorf("kb source %v: class %v (%v), want %v", tc.args, got, err, tc.want)
			}
		})
	}
	if l, _ := w.kb.DocumentSources(w.doc); len(l) != 0 {
		t.Errorf("a refused link wrote %+v", l)
	}
}

func TestSourceListAndShow_ReportCitationsOfEveryKind(t *testing.T) {
	w := newCiteWorld(t)
	w.kb.LinkObservationSource(w.obs, w.src, "cited")
	w.kb.LinkDocumentSource(w.doc, w.src, "cited", "", "")
	w.kb.LinkRecordSource(w.rec, w.src, "cited", "", "")
	spare, _ := w.kb.AddSource(knowledge.Source{Title: "Spare"})

	list, err := w.source(t, false, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, "1 observation(s), 1 document(s), 1 record(s)") {
		t.Errorf("list = %q, want the citations of SPARQL", list)
	}
	for _, line := range strings.Split(list, "\n") {
		if strings.Contains(line, "Spare") && strings.Contains(line, "cited by") {
			t.Errorf("an uncited source says it is cited: %q", line)
		}
	}
	show, err := w.source(t, false, "show", fmt.Sprint(w.src))
	if err != nil || !strings.Contains(show, "cited by 1 observation(s), 1 document(s), 1 record(s)") {
		t.Errorf("show = %q, %v", show, err)
	}

	js, _ := w.source(t, true, "list")
	var rows []struct {
		ID        int64
		Title     string
		Citations struct {
			Observations, Documents, Records int
		} `json:"citations"`
	}
	if err := json.Unmarshal([]byte(js), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("list json: %v\n%s", err, js)
	}
	for _, r := range rows {
		want := 0
		if r.ID == w.src {
			want = 1
		}
		if r.Citations.Observations != want || r.Citations.Documents != want || r.Citations.Records != want {
			t.Errorf("row %d (%s) citations = %+v, want %d of each (spare is %d)", r.ID, r.Title, r.Citations, want, spare)
		}
	}
	sj, _ := w.source(t, true, "show", fmt.Sprint(w.src))
	if !strings.Contains(sj, `"citations"`) || !strings.Contains(sj, `"Title": "SPARQL 1.1"`) {
		t.Errorf("show json = %s", sj)
	}
}

func TestSourceRemove_RefusesASourceACitingDocumentOrRecordHolds(t *testing.T) {
	w := newCiteWorld(t)
	w.kb.LinkDocumentSource(w.doc, w.src, "cited", "", "")
	_, err := w.source(t, false, "remove", fmt.Sprint(w.src))
	if err == nil || exitCodeFor(err) != classNegative || !strings.Contains(err.Error(), "1 document(s)") {
		t.Errorf("remove = %v (class %v), want a negative refusal naming the document", err, exitCodeFor(err))
	}
}

func TestUnlinkSource_DocumentAndRecordForms(t *testing.T) {
	w := newCiteWorld(t)
	w.kb.LinkDocumentSource(w.doc, w.src, "cited", "", "")
	w.kb.LinkRecordSource(w.rec, w.src, "cited", "", "")
	run := func(args ...string) error {
		var out bytes.Buffer
		return cmdUnlink(w.kb, nil, false, args, &out)
	}
	if err := run("source", "--document", fmt.Sprint(w.doc), fmt.Sprint(w.src)); err != nil {
		t.Fatalf("unlink --document: %v", err)
	}
	if err := run("source", "--record", "alpha/DR-0001", fmt.Sprint(w.src)); err != nil {
		t.Fatalf("unlink --record: %v", err)
	}
	if c, _ := w.kb.SourceCitations(w.src); c.Total() != 0 {
		t.Errorf("citations left: %+v", c)
	}
	// A link that is not there is the negative class, and a bad shape is usage.
	if err := run("source", "--document", fmt.Sprint(w.doc), fmt.Sprint(w.src)); exitCodeFor(err) != classNegative {
		t.Errorf("unlinking a missing link: %v", err)
	}
	if err := run("source", "--document", "1", "--record", "alpha/DR-0001", "1"); exitCodeFor(err) != classUsage {
		t.Errorf("both targets: %v", err)
	}
}

func TestPlanSourceRemove_NamesEveryKindThatCitesIt(t *testing.T) {
	w := newCiteWorld(t)
	w.kb.LinkDocumentSource(w.doc, w.src, "cited", "", "")
	w.kb.LinkRecordSource(w.rec, w.src, "cited", "", "")
	p, err := planSourceRemove(w.kb, w.src, "SPARQL 1.1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Refusal, "1 document(s)") || !strings.Contains(p.Refusal, "1 record(s)") || p.Apply != nil {
		t.Errorf("plan = %+v, want a refusal naming the document and record", p)
	}
	w.kb.UnlinkDocumentSource(w.doc, w.src)
	w.kb.UnlinkRecordSource(w.rec, w.src)
	p, _ = planSourceRemove(w.kb, w.src, "SPARQL 1.1")
	if p.Refusal != "" || p.Apply == nil {
		t.Errorf("an uncited source should be removable: %+v", p)
	}
}

func TestVerbTable_SourceLinkFlagsAreListed(t *testing.T) {
	var flags []string
	for _, v := range verbTable {
		if v.Name == "source" {
			flags = v.Flags
		}
	}
	for _, want := range []string{"--document", "--record", "--version", "--hash", "--relationship"} {
		found := false
		for _, f := range flags {
			found = found || f == want
		}
		if !found {
			t.Errorf("source flags %v lack %s", flags, want)
		}
	}
}

func TestHelp_DescribesCitingFromDocumentsAndRecords(t *testing.T) {
	for _, want := range []string{"--document DOC_ID", "--record REF", "--hash H", "cited by", "DR-0081"} {
		if !strings.Contains(SourceHelpText, want) {
			t.Errorf("kb source help does not mention %q", want)
		}
	}
	for _, want := range []string{"--document DOC_ID SOURCE_ID", "--record REF SOURCE_ID"} {
		if !strings.Contains(UnlinkHelpText, want) {
			t.Errorf("kb unlink help does not mention %q", want)
		}
	}
}
