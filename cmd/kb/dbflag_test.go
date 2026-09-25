package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// init, index and merge never open the ambient database (see mainRun), so a
// --db given to them names something they do not use. It used to be dropped
// silently: `kb --db rt.db init` reported success while creating
// ./agents/knowledge.db, and rt.db never existed. An option that changes
// nothing is a mistake worth reporting, and the message says where the target
// really goes.

func TestMainRun_DBOptionRefusedByVerbsThatIgnoreIt(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string // a phrase the message must contain
	}{
		{[]string{"--db", "rt.db", "init"}, "kb init PATH"},
		{[]string{"-db", "rt.db", "init"}, "kb init PATH"},
		{[]string{"--db", "rt.db", "init", "somedir"}, "kb init PATH"},
		{[]string{"--db", "rt.db", "index", "somedir"}, "record files"},
		{[]string{"--db", "rt.db", "merge", "-a", "a.db", "-b", "b.db", "-out", "c.db"}, "-a, -b and -out"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			var out, errOut bytes.Buffer
			code := mainRun(tc.args, &out, &errOut)
			if code != 2 {
				t.Errorf("exit %d, want 2; stdout=%.100q stderr=%q", code, out.String(), errOut.String())
			}
			if !strings.Contains(errOut.String(), "--db") || !strings.Contains(errOut.String(), tc.want) {
				t.Errorf("stderr = %q, want it to name --db and contain %q", errOut.String(), tc.want)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("a refused command left files behind: %v", entries)
			}
		})
	}
}

func TestMainRun_DBOptionRefusalUnderJSONIsAnEnvelope(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code := mainRun([]string{"--json", "--db", "rt.db", "init"}, &out, &errOut)
	if code != 2 || out.Len() != 0 {
		t.Fatalf("exit %d, stdout %q; want 2 and nothing on stdout", code, out.String())
	}
	var env struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil || env.Error == "" {
		t.Errorf("stderr = %q, want a {\"error\": ...} envelope (%v)", errOut.String(), err)
	}
}

// Without --db these verbs are unchanged, and the verbs that do use the
// database still accept it.
func TestMainRun_DBOptionStillWorksWhereItIsUsed(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"init"}, &out, &errOut); code != 0 {
		t.Fatalf("plain init: exit %d: %s", code, errOut.String())
	}
	if _, err := os.Stat("agents/knowledge.db"); err != nil {
		t.Errorf("plain init did not create agents/knowledge.db: %v", err)
	}
	out.Reset()
	errOut.Reset()
	if code := mainRun([]string{"--db", "explicit.db", "project", "add", "p"}, &out, &errOut); code != 0 {
		t.Errorf("--db with a verb that uses it: exit %d: %s", code, errOut.String())
	}
	if _, err := os.Stat("explicit.db"); err != nil {
		t.Errorf("--db explicit.db was not honoured by project add: %v", err)
	}
	out.Reset()
	errOut.Reset()
	if code := mainRun([]string{"index", "."}, &out, &errOut); code != 0 {
		t.Errorf("plain index: exit %d: %s", code, errOut.String())
	}
}
