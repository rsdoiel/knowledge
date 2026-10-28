package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// `kb stats`: the shape of a database without SQL. A read verb, like check-db.

func statsDB(t *testing.T) string {
	t.Helper()
	noTerminal(t)
	db := filepath.Join(t.TempDir(), "kb.db")
	for _, a := range [][]string{
		{"project", "add", "P", "d"}, {"concept", "add", "C"}, {"concept", "add", "Lonely"},
		{"observation", "add", "--project", "P", "note", "body"}, {"link", "observation", "1", "C"},
	} {
		var out, errOut bytes.Buffer
		if code := mainRun(append([]string{"--db", db}, a...), &out, &errOut); code != 0 {
			t.Fatalf("%v: exit %d: %s", a, code, errOut.String())
		}
	}
	return db
}

func TestStats_HumanOutput(t *testing.T) {
	db := statsDB(t)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--db", db, "stats"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	for _, want := range []string{"observation_concepts", "concepts_without_links", "observations_without_project", "P "} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	// The counts are right where they are checkable by eye.
	for _, line := range strings.Split(out.String(), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "concepts_without_links" && f[1] != "1" {
			t.Errorf("line %q, want 1 (Lonely)", line)
		}
		if len(f) == 2 && f[0] == "observation_concepts" && f[1] != "1" {
			t.Errorf("line %q, want 1", line)
		}
	}
}

func TestStats_JSON(t *testing.T) {
	db := statsDB(t)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--db", db, "--json", "stats"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var got struct {
		Rows     map[string]int `json:"rows"`
		Links    map[string]int `json:"links"`
		Checks   map[string]int `json:"checks"`
		Projects []struct {
			Name         string `json:"name"`
			Observations int    `json:"observations"`
			Concepts     int    `json:"concepts"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got.Rows["concepts"] != 2 || got.Links["observation_concepts"] != 1 || got.Checks["concepts_without_links"] != 1 {
		t.Errorf("counts = %+v", got)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "P" || got.Projects[0].Observations != 1 || got.Projects[0].Concepts != 1 {
		t.Errorf("projects = %+v", got.Projects)
	}
}

func TestStats_IsARegisteredReadVerb(t *testing.T) {
	if _, ok := verbs["stats"]; !ok {
		t.Fatal("stats is not registered")
	}
	for _, v := range verbTable {
		if v.Name == "stats" {
			if v.Class != classRead {
				t.Errorf("class = %v, want read", v.Class)
			}
			return
		}
	}
	t.Error("stats is not in the verb table")
}

func TestConceptShow_SaysWhichProjectsItSpans(t *testing.T) {
	kb := showFixture(t)
	beta, _ := kb.AddProject("beta", "")
	ob, _ := kb.AddObservation(beta, "note", "mentions toast")
	cs, _ := kb.Concepts()
	for _, c := range cs {
		if c.Name == "toast" {
			kb.LinkObservationConcept(ob, c.ID)
		}
	}
	out, err := runConceptCmd(t, kb, false, "show", "toast")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "spans 2 projects") || !strings.Contains(out, "beta") {
		t.Errorf("no spans section:\n%s", out)
	}
	j, err := runConceptCmd(t, kb, true, "show", "toast")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Spans []struct {
			Project      string `json:"project"`
			Tagged       bool   `json:"tagged"`
			Observations int    `json:"observations"`
		} `json:"spans"`
	}
	if err := json.Unmarshal([]byte(j), &got); err != nil || len(got.Spans) != 2 {
		t.Fatalf("spans in JSON: %v %s", err, j)
	}
}
