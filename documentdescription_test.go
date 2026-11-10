package knowledge

import "testing"

// Workspace DR-0010: a description is the author's statement, made after the
// draft settles. A tool never fills one with a stand-in, and a missing one is
// shown as missing. These tests hold knowledge to items 2 and 3 of the record.

func gistAfterIngest(t *testing.T, name, content string) DocumentSection {
	t.Helper()
	kb := openTestKB(t)
	pid, _ := kb.AddProject("alpha", "")
	path := writeTempFile(t, name, content)
	if _, err := kb.IngestDocument(pid, path, DocumentIngestOptions{}); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	_, _, gist := ingestSections(t, kb, path)
	return gist
}

// A document with no description has a gist that says nothing: not the first
// paragraph, not a truncation.
func TestDR0010_AMissingDescriptionLeavesTheGistUnsummarized(t *testing.T) {
	gist := gistAfterIngest(t, "none.md", "---\ntitle: \"No description\"\n---\n\nThe first paragraph is long enough to be tempting to copy.\n\n## Part\n\nMore text.\n")
	if gist.SummaryBody != "" || gist.SummaryStatus != "unsummarized" {
		t.Errorf("gist = %q (%s), want an empty, unsummarized gist", gist.SummaryBody, gist.SummaryStatus)
	}
}

// The author's description is kept as written, and it waits for the author's
// review: a drafted gist is not a reviewed one.
func TestDR0010_AnAuthorDescriptionSeedsADraftNotAReviewedSummary(t *testing.T) {
	gist := gistAfterIngest(t, "desc.md", "---\ntitle: \"With description\"\ndescription: \"What the author says it is.\"\n---\n\nBody.\n")
	if gist.SummaryBody != "What the author says it is." || gist.SummaryStatus != "drafted" {
		t.Errorf("gist = %q (%s), want the description as a drafted gist", gist.SummaryBody, gist.SummaryStatus)
	}
}

// `abstract` is not read: a field that often holds a copy of the opening
// paragraph (65 of 133 on the blog corpus) must not become the gist unseen.
func TestDR0010_AnAbstractIsNotTakenForTheDescription(t *testing.T) {
	gist := gistAfterIngest(t, "abs.md", "---\ntitle: \"With abstract\"\nabstract: \"Possibly the first paragraph again.\"\n---\n\nPossibly the first paragraph again.\n")
	if gist.SummaryBody != "" || gist.SummaryStatus != "unsummarized" {
		t.Errorf("gist = %q (%s), want it left unsummarized", gist.SummaryBody, gist.SummaryStatus)
	}
}
