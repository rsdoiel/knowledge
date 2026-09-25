package main

import (
	"bytes"
	"strings"
	"testing"
)

// The library refuses a blank project or concept name and trims a padded one
// (blankname_test.go at the module root). These tests pin the CLI half: the
// verbs must report the name that was actually stored, and `project rename`,
// which rewrites files before it calls the library, must clean NEW first.
// Otherwise a padded NEW lands in every record's project: frontmatter while
// the database row holds the trimmed name, which is the exact desync DR-0026
// exists to prevent.

func TestCmdProject_AddBlankNameFails(t *testing.T) {
	kb := openTestKB(t)
	for _, name := range []string{"", " ", "\t"} {
		var out bytes.Buffer
		if err := cmdProject(kb, nil, false, []string{"add", name}, &out); err == nil {
			t.Errorf("project add %q succeeded, want an error", name)
		}
	}
	ps, err := kb.Projects()
	if err != nil || len(ps) != 0 {
		t.Errorf("Projects() = %+v, %v; want none", ps, err)
	}
}

func TestCmdProject_AddReportsTrimmedName(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"add", "  padded "}, &out); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if !strings.Contains(out.String(), `project "padded" added`) {
		t.Errorf("output = %q, want it to name the stored (trimmed) project", out.String())
	}
	out.Reset()
	if err := cmdProject(kb, nil, true, []string{"add", "  padded2 "}, &out); err != nil {
		t.Fatalf("project add --json: %v", err)
	}
	if !strings.Contains(out.String(), `"name": "padded2"`) && !strings.Contains(out.String(), `"name":"padded2"`) {
		t.Errorf("json = %q, want the trimmed name", out.String())
	}
}

func TestCmdConcept_AddBlankNameFails(t *testing.T) {
	kb := openTestKB(t)
	for _, name := range []string{"", " ", "\t"} {
		var out bytes.Buffer
		if err := cmdConcept(kb, nil, false, []string{"add", name}, &out); err == nil {
			t.Errorf("concept add %q succeeded, want an error", name)
		}
	}
	cs, err := kb.Concepts()
	if err != nil || len(cs) != 0 {
		t.Errorf("Concepts() = %+v, %v; want none", cs, err)
	}
}

func TestCmdConcept_AddReportsTrimmedName(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"add", " padded "}, &out); err != nil {
		t.Fatalf("concept add: %v", err)
	}
	if !strings.Contains(out.String(), `concept "padded" added`) {
		t.Errorf("output = %q, want it to name the stored (trimmed) concept", out.String())
	}
}

func TestCmdProject_RenameBlankNewNameFailsWithoutRewriting(t *testing.T) {
	kb, root := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"})
	before := readFixture(t, root, "hasrecords", "0001")
	for _, name := range []string{"", "  "} {
		// A dry run must refuse too: it would otherwise promise a rewrite
		// that no real run can perform.
		for _, args := range [][]string{
			{"rename", "hasrecords", name},
			{"rename", "--dry-run", "hasrecords", name},
		} {
			var out bytes.Buffer
			err := cmdProject(kb, nil, false, args, &out)
			if err == nil {
				t.Errorf("project %v succeeded, want an error", args)
				continue
			}
			if !strings.Contains(err.Error(), "must not be empty") {
				t.Errorf("project %v error = %v, want the empty-name error, not a later failure", args, err)
			}
		}
	}
	if after := readFixture(t, root, "hasrecords", "0001"); after != before {
		t.Errorf("record file changed by a refused rename:\n%s", after)
	}
	if p, err := kb.ProjectByName("hasrecords"); err != nil || p == nil {
		t.Errorf("project hasrecords should be untouched: %v, %v", p, err)
	}
}

func TestCmdProject_RenameTrimsNewNameInFilesAndDatabase(t *testing.T) {
	kb, root := fixtureWorkspace(t, "hasrecords", testRecord{ID: "0001"})
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "hasrecords", "  newname "}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}
	if p, err := kb.ProjectByName("newname"); err != nil || p == nil {
		t.Fatalf("ProjectByName(newname) = %v, %v", p, err)
	}
	line := frontmatterLine(t, readFixture(t, root, "hasrecords", "0001"), "project")
	val := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "project:")), `"'`)
	if val != "newname" {
		t.Errorf("frontmatter %q holds %q, want exactly \"newname\" so the file and the database agree", line, val)
	}
	if !strings.Contains(out.String(), `to "newname"`) {
		t.Errorf("output = %q, want it to report the trimmed name", out.String())
	}
}

func TestCmdProject_RenameNoRecordsReportsTrimmedName(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddProject("oldname", ""); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "oldname", " newname "}, &out); err != nil {
		t.Fatalf("project rename: %v", err)
	}
	if !strings.Contains(out.String(), `renamed to "newname"`) {
		t.Errorf("output = %q, want the trimmed name", out.String())
	}
}

func TestCmdProject_RenameOntoExistingNameIsCaughtAfterTrimming(t *testing.T) {
	kb := openTestKB(t)
	for _, n := range []string{"oldname", "taken"} {
		if _, err := kb.AddProject(n, ""); err != nil {
			t.Fatalf("AddProject: %v", err)
		}
	}
	var out bytes.Buffer
	if err := cmdProject(kb, nil, false, []string{"rename", "oldname", " taken "}, &out); err == nil {
		t.Error("renaming onto an existing name (padded) succeeded, want an error")
	}
}

func TestCmdConcept_RenameBlankNewNameFails(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "oldname", " "}, &out); err == nil {
		t.Error("concept rename to a blank name succeeded, want an error")
	}
}

func TestCmdConcept_RenameReportsTrimmedName(t *testing.T) {
	kb := openTestKB(t)
	if _, err := kb.AddConcept("oldname", ""); err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	var out bytes.Buffer
	if err := cmdConcept(kb, nil, false, []string{"rename", "oldname", " newname "}, &out); err != nil {
		t.Fatalf("concept rename: %v", err)
	}
	if !strings.Contains(out.String(), `renamed to "newname"`) {
		t.Errorf("output = %q, want the trimmed name", out.String())
	}
}
