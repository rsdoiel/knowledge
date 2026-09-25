package knowledge

import (
	"bytes"
	"strings"
	"testing"
)

// `kb source add` used to accept anything: a blank title, --published
// notadate, --url 'not a url', --doi zzz. Nothing failed, so nothing was
// noticed, and a mistyped DOI is worse than a stored typo: check-retractions
// sends it to Retraction Watch and a miss reads as "not retracted". The guard
// lives in AddSource, not the CLI, so every caller gets it. The JSONL importer
// has its own importSource and is deliberately not held to it.

func TestAddSource_BlankTitleRejected(t *testing.T) {
	kb := openTestKB(t)
	before := countRows(t, kb, "sources")
	for _, title := range blankInputs {
		if _, err := kb.AddSource(Source{Title: title}); err == nil {
			t.Errorf("AddSource(title %q) succeeded, want an error", title)
		}
	}
	if got := countRows(t, kb, "sources"); got != before {
		t.Errorf("sources rows = %d, want %d (a rejected add must not insert)", got, before)
	}
}

func TestAddSource_TitleIsTrimmed(t *testing.T) {
	kb := openTestKB(t)
	id, err := kb.AddSource(Source{Title: "  SPARQL 1.1\t"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	sources, err := kb.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != id {
		t.Fatalf("ListSources = %+v, want the one source %d", sources, id)
	}
	if sources[0].Title != "SPARQL 1.1" {
		t.Errorf("stored title = %q, want it trimmed", sources[0].Title)
	}
}

func TestAddSource_PublishedDate(t *testing.T) {
	valid := []string{"", "2026", "2026-09", "2026-09-24", "2024-02-29", " 2026-09-24 "}
	invalid := []string{
		"notadate", "2026-13-45", "2026-13", "2026-00", "2026-09-31", "2023-02-29",
		"26", "2026/09/24", "09-24-2026", "2026-9-4", "20260924", "2026-09-24T10:00:00Z",
		"yesterday", "2026-09-24 extra",
	}
	for _, d := range valid {
		kb := openTestKB(t)
		if _, err := kb.AddSource(Source{Title: "T", PublishedDate: d}); err != nil {
			t.Errorf("AddSource(published %q) = %v, want it accepted", d, err)
		}
	}
	for _, d := range invalid {
		kb := openTestKB(t)
		if _, err := kb.AddSource(Source{Title: "T", PublishedDate: d}); err == nil {
			t.Errorf("AddSource(published %q) succeeded, want an error", d)
		}
		if n := countRows(t, kb, "sources"); n != 0 {
			t.Errorf("published %q was rejected but %d row(s) were inserted", d, n)
		}
	}
}

func TestAddSource_PublishedDateErrorNamesTheForms(t *testing.T) {
	kb := openTestKB(t)
	_, err := kb.AddSource(Source{Title: "T", PublishedDate: "notadate"})
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"notadate", "YYYY", "YYYY-MM", "YYYY-MM-DD"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestAddSource_URL(t *testing.T) {
	valid := []string{
		"https://example.org", "http://example.org/a/b?c=d#e", "https://example.org:8080/x",
		"ftp://example.org/file",
	}
	invalid := []string{
		"not a url", "example.org", "//example.org", "https://", "https:///path",
		"mailto:someone@example.org", "http://exa mple.org", "http://example.org/a b", "/relative/path", "10.1234/example",
	}
	for _, u := range valid {
		kb := openTestKB(t)
		if _, err := kb.AddSource(Source{Title: "T", IdentifierType: "url", IdentifierValue: u}); err != nil {
			t.Errorf("AddSource(url %q) = %v, want it accepted", u, err)
		}
	}
	for _, u := range invalid {
		kb := openTestKB(t)
		if _, err := kb.AddSource(Source{Title: "T", IdentifierType: "url", IdentifierValue: u}); err == nil {
			t.Errorf("AddSource(url %q) succeeded, want an error", u)
		}
		if n := countRows(t, kb, "sources"); n != 0 {
			t.Errorf("url %q was rejected but %d row(s) were inserted", u, n)
		}
	}
}

func TestAddSource_DOI(t *testing.T) {
	// Registrant codes in real DOIs have four or more digits, but this
	// module's own fixtures use 10.1/a, so the shape is the only thing checked.
	valid := []string{
		"10.1234/example", "10.1/a", "10.99/ex", "10.1000.10/abc", "10.1016/j.cell.2020.01.001",
		"10.1002/(SICI)1097-4571(199806)49:8<693::AID-ASI4>3.0.CO;2-O",
	}
	invalid := []string{
		"zzz", "doi:10.1234/example", "https://doi.org/10.1234/example", "10.1234", "10.1234/",
		"10./x", "11.1234/x", "10.1234/has space", "10.abc/x",
	}
	for _, d := range valid {
		kb := openTestKB(t)
		if _, err := kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: d}); err != nil {
			t.Errorf("AddSource(doi %q) = %v, want it accepted", d, err)
		}
	}
	for _, d := range invalid {
		kb := openTestKB(t)
		if _, err := kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: d}); err == nil {
			t.Errorf("AddSource(doi %q) succeeded, want an error", d)
		}
		if n := countRows(t, kb, "sources"); n != 0 {
			t.Errorf("doi %q was rejected but %d row(s) were inserted", d, n)
		}
	}
}

func TestAddSource_DOIErrorNamesTheShapeAndDoesNotNormalise(t *testing.T) {
	kb := openTestKB(t)
	_, err := kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: "https://doi.org/10.1234/example"})
	if err == nil {
		t.Fatal("a pasted https://doi.org/ URL should be refused, not silently rewritten")
	}
	for _, want := range []string{"https://doi.org/10.1234/example", "10."} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

// Only doi and url are checked: isbn, issn, arxiv and urn have no shape this
// change is prepared to police.
func TestAddSource_OtherIdentifierTypesUnchecked(t *testing.T) {
	kb := openTestKB(t)
	for _, typ := range []string{"isbn", "issn", "arxiv", "urn", "custom"} {
		if _, err := kb.AddSource(Source{Title: "T " + typ, IdentifierType: typ, IdentifierValue: "not checked at all"}); err != nil {
			t.Errorf("AddSource(type %q) = %v, want it accepted", typ, err)
		}
	}
}

// An identifier padded with whitespace must not become a second dedupe key.
func TestAddSource_IdentifierIsTrimmedBeforeDedupe(t *testing.T) {
	kb := openTestKB(t)
	want, err := kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: "10.1234/example"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	got, err := kb.AddSource(Source{Title: "T", IdentifierType: "doi", IdentifierValue: " 10.1234/example\n"})
	if err != nil {
		t.Fatalf("AddSource(padded): %v", err)
	}
	if got != want {
		t.Errorf("padded identifier gave id %d, want the existing id %d", got, want)
	}
	if n := countRows(t, kb, "sources"); n != 1 {
		t.Errorf("sources rows = %d, want 1", n)
	}
}

// An identifier-less source is still fine: nothing to check.
func TestAddSource_NoIdentifierNoDateStillWorks(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddSource(Source{Title: "Bare"}); err != nil {
		t.Errorf("AddSource(bare) = %v", err)
	}
}

// The importer is deliberately not held to AddSource's checks: an export from
// a database that already holds an odd value must still restore.
func TestImportSource_NotHeldToAddSourceChecks(t *testing.T) {
	src := openTestKB(t)
	if _, err := src.db.Exec(
		`INSERT INTO sources (title, identifier_type, identifier_value, published_date, uuid, origin_host)
		 VALUES ('Old', 'doi', 'zzz', 'notadate', '11111111-1111-7111-8111-111111111111', 'h')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var buf bytes.Buffer
	if err := ExportJSONL(src, &buf, ""); err != nil {
		t.Fatalf("ExportJSONL: %v", err)
	}
	dst := openTestKB(t)
	if _, err := ImportJSONL(dst, &buf); err != nil {
		t.Fatalf("ImportJSONL of a legacy odd source must still work: %v", err)
	}
	if n := countRows(t, dst, "sources"); n != 1 {
		t.Errorf("imported sources = %d, want 1", n)
	}
}
