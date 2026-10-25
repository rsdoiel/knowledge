package main

import (
	"bytes"
	"testing"
)

// The title typed on the command line goes through the same one-line rule as a
// project or concept name (DR-0046). A hard-wrapped paste used to be stored with
// its newline; a control character is a bad value on the command line.
func TestCmdSource_AddStoresATitleOnOneLine(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"add", "Line one\nline two"}, &out); err != nil {
		t.Fatalf("source add: %v", err)
	}
	sources, _ := kb.ListSources()
	if len(sources) != 1 || sources[0].Title != "Line one line two" {
		t.Errorf("sources = %+v, want one titled \"Line one line two\"", sources)
	}
}

func TestCmdSource_AddRefusesAControlCharacterWithTheBlankTitleClass(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	blank := cmdSource(kb, nil, false, []string{"add", " "}, &out)
	ctrl := cmdSource(kb, nil, false, []string{"add", "Esc\x1b[31m"}, &out)
	if blank == nil || ctrl == nil {
		t.Fatalf("blank: %v, control: %v; both must be refused", blank, ctrl)
	}
	if exitCodeFor(ctrl) != exitCodeFor(blank) {
		t.Errorf("control character exits %d, blank title %d; a bad title is one class", exitCodeFor(ctrl).Code, exitCodeFor(blank).Code)
	}
	if sources, _ := kb.ListSources(); len(sources) != 0 {
		t.Errorf("sources = %+v, want none", sources)
	}
}

func TestCmdSource_AddJSONReportsTheTitleAsStored(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, true, []string{"add", "Line one\nline two"}, &out); err != nil {
		t.Fatalf("source add: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"title": "Line one line two"`)) {
		t.Errorf("json = %s, want the stored title", out.String())
	}
}
