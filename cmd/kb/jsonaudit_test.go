package main

import (
	"bytes"
	"fmt"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// jsonAuditFixture seeds a fresh KnowledgeBase with one project, one
// concept (linked to the project), one observation (linked to the
// concept), and one source (linked to the observation) -- enough real
// data for every read-oriented verb in the table below to have something
// to show, and returns the ids so args can reference them by number.
type jsonAuditFixture struct {
	kb          *knowledge.KnowledgeBase
	projectID   int64
	conceptID   int64
	obsID       int64
	sourceID    int64
	projectName string
	conceptName string
}

func newJSONAuditFixture(t *testing.T) jsonAuditFixture {
	t.Helper()
	kb := openTestKB(t)
	pid, err := kb.AddProject("fixture-project", "a fixture project")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	cid, err := kb.AddConcept("fixture-concept", "a fixture concept")
	if err != nil {
		t.Fatalf("AddConcept: %v", err)
	}
	if err := kb.LinkProjectConcept(pid, cid); err != nil {
		t.Fatalf("LinkProjectConcept: %v", err)
	}
	oid, err := kb.AddObservation(pid, "note", "a fixture observation body")
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}
	if err := kb.LinkObservationConcept(oid, cid); err != nil {
		t.Fatalf("LinkObservationConcept: %v", err)
	}
	sid, err := kb.AddSource(sourceStub("Fixture Source"))
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := kb.LinkObservationSource(oid, sid, "cited"); err != nil {
		t.Fatalf("LinkObservationSource: %v", err)
	}
	return jsonAuditFixture{kb: kb, projectID: pid, conceptID: cid, obsID: oid, sourceID: sid, projectName: "fixture-project", conceptName: "fixture-concept"}
}

// TestJSONMode_EveryVerbProducesValidJSON exercises every verb+subcommand
// against a fresh copy of the fixture, in both text mode (must not error)
// and --json mode (output must be valid JSON). Verbs that mutate state in
// a way that conflicts with reuse (source remove, which requires an
// unlinked source) are covered by their own dedicated tests instead --
// see source_test.go.
func TestJSONMode_EveryVerbProducesValidJSON(t *testing.T) {
	// args is built per-case against a fresh fixture so ids referenced by
	// number (observation/concept/source ids) are always valid.
	cases := []struct {
		name string
		args func(f jsonAuditFixture) []string
	}{
		{"project add", func(f jsonAuditFixture) []string { return []string{"project", "add", "new-project", "desc"} }},
		{"project list", func(f jsonAuditFixture) []string { return []string{"project", "list"} }},
		{"project show", func(f jsonAuditFixture) []string { return []string{"project", "show", f.projectName} }},
		{"project concepts", func(f jsonAuditFixture) []string { return []string{"project", "concepts", f.projectName} }},
		{"observation add", func(f jsonAuditFixture) []string {
			return []string{"observation", "add", "--project", f.projectName, "note", "another body"}
		}},
		{"observation list", func(f jsonAuditFixture) []string { return []string{"observation", "list", "--project", f.projectName} }},
		{"observation show", func(f jsonAuditFixture) []string { return []string{"observation", "show", fmt.Sprint(f.obsID)} }},
		{"observation sources", func(f jsonAuditFixture) []string { return []string{"observation", "sources", fmt.Sprint(f.obsID)} }},
		{"concept add", func(f jsonAuditFixture) []string { return []string{"concept", "add", "new-concept"} }},
		{"concept list", func(f jsonAuditFixture) []string { return []string{"concept", "list"} }},
		{"link project", func(f jsonAuditFixture) []string { return []string{"link", "project", f.projectName, f.conceptName} }},
		{"link observation", func(f jsonAuditFixture) []string {
			return []string{"link", "observation", fmt.Sprint(f.obsID), f.conceptName}
		}},
		{"source add", func(f jsonAuditFixture) []string { return []string{"source", "add", "New Source"} }},
		{"source list", func(f jsonAuditFixture) []string { return []string{"source", "list"} }},
		{"source show", func(f jsonAuditFixture) []string { return []string{"source", "show", fmt.Sprint(f.sourceID)} }},
		{"source retract", func(f jsonAuditFixture) []string {
			return []string{"source", "retract", fmt.Sprint(f.sourceID), "reason"}
		}},
		{"source link", func(f jsonAuditFixture) []string {
			return []string{"source", "link", fmt.Sprint(f.obsID), fmt.Sprint(f.sourceID)}
		}},
		{"source check-retractions", func(f jsonAuditFixture) []string { return []string{"source", "check-retractions"} }},
		{"search", func(f jsonAuditFixture) []string { return []string{"search", "fixture"} }},
		{"summary", func(f jsonAuditFixture) []string { return []string{"summary"} }},
		{"format", func(f jsonAuditFixture) []string { return []string{"format"} }},
	}

	for _, c := range cases {
		t.Run(c.name+"/text", func(t *testing.T) {
			f := newJSONAuditFixture(t)
			var out bytes.Buffer
			if err := dispatchArgs(f.kb, false, c.args(f), &out); err != nil {
				t.Fatalf("%s (text mode): %v", c.name, err)
			}
		})
		t.Run(c.name+"/json", func(t *testing.T) {
			f := newJSONAuditFixture(t)
			var out bytes.Buffer
			if err := dispatchArgs(f.kb, true, c.args(f), &out); err != nil {
				t.Fatalf("%s (json mode): %v", c.name, err)
			}
			assertValidJSON(t, out.Bytes())
		})
	}
}

// TestMainRun_ErrorPathsNeverWriteToStdout runs a real verb error (a
// nonexistent project) through the full mainRun path in both output
// modes, confirming stdout stays empty, stderr gets the message, and the
// exit code is 1 -- the plan's stated acceptance criterion for this
// audit pass, checked end-to-end rather than just at the dispatch level
// (see TestDispatch_HandlerErrorReturnsExitCode1 for that narrower check).
func TestMainRun_ErrorPathsNeverWriteToStdout(t *testing.T) {
	dbPath := newJSONAuditFixture(t).kb.Path()

	for _, jsonFlag := range []string{"", "--json"} {
		var args []string
		if jsonFlag != "" {
			args = append(args, jsonFlag)
		}
		args = append(args, "--db", dbPath, "project", "show", "nonexistent")

		var out, errOut bytes.Buffer
		code := mainRun(args, &out, &errOut)
		if code != 1 {
			t.Errorf("args=%v: exit code = %d, want 1", args, code)
		}
		if out.Len() != 0 {
			t.Errorf("args=%v: stdout = %q, want empty", args, out.String())
		}
		if errOut.Len() == 0 {
			t.Errorf("args=%v: stderr is empty, want the error message", args)
		}
		if jsonFlag == "--json" {
			assertValidJSON(t, errOut.Bytes())
		}
	}
}

// dispatchArgs is a small helper: look up args[0] in the real verbs table
// and call it directly (skipping dispatch's exit-code plumbing, which is
// already covered by TestDispatch_* in dispatch_test.go).
func dispatchArgs(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out *bytes.Buffer) error {
	fn, ok := verbs[args[0]]
	if !ok {
		return fmt.Errorf("no such verb %q registered", args[0])
	}
	return fn(kb, jsonOut, args[1:], out)
}
