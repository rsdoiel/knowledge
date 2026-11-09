package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// version and license are verbs as well as the -version and -license options,
// for the muscle memory of the go command (`go version`). They answer without
// a workspace and never open or create a database.

func TestVersionAndLicenseVerbs_SayWhatTheOptionsSay(t *testing.T) {
	for _, name := range []string{"version", "license"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("KB_DB", "")
			var vOut, vErr, oOut, oErr bytes.Buffer
			if code := mainRun([]string{name}, &vOut, &vErr); code != 0 {
				t.Fatalf("kb %s: exit %d, stderr %q", name, code, vErr.String())
			}
			if code := mainRun([]string{"-" + name}, &oOut, &oErr); code != 0 {
				t.Fatalf("kb -%s: exit %d", name, code)
			}
			if vOut.String() != oOut.String() || vOut.Len() == 0 {
				t.Errorf("kb %s = %q, want what kb -%s prints: %q", name, vOut.String(), name, oOut.String())
			}
			if vErr.Len() != 0 {
				t.Errorf("stderr = %q, want nothing", vErr.String())
			}
			if entries, _ := os.ReadDir("."); len(entries) != 0 {
				t.Errorf("kb %s left files behind: %v", name, entries)
			}
		})
	}
}

func TestVersionVerb_UnderJSONIsAnObject(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KB_DB", "")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--json", "version"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	var v struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		ReleaseHash string `json:"release_hash"`
	}
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("stdout is not one JSON object: %v (%q)", err, out.String())
	}
	if v.Name != "kb" || v.Version != knowledge.Version || v.ReleaseHash != knowledge.ReleaseHash {
		t.Errorf("version object = %+v", v)
	}
}

func TestLicenseVerb_UnderJSONIsAnObject(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KB_DB", "")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--json", "license"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	var l struct {
		License string `json:"license"`
	}
	if err := json.Unmarshal(out.Bytes(), &l); err != nil || l.License != knowledge.LicenseText {
		t.Errorf("license object: %v (%.80q)", err, out.String())
	}
}

func TestVersionAndLicenseVerbs_RefuseWhatIsNotTheirs(t *testing.T) {
	for _, args := range [][]string{
		{"version", "extra"},
		{"license", "extra"},
		{"version", "--bogus"},
		{"--db", "x.db", "version"},
		{"--db", "x.db", "license"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("KB_DB", "")
			var out, errOut bytes.Buffer
			if code := mainRun(append([]string{"--json"}, args...), &out, &errOut); code != 2 {
				t.Errorf("exit %d, want 2; stderr %q", code, errOut.String())
			}
			var env struct{ Class string }
			if err := json.Unmarshal(out.Bytes(), &env); err != nil || env.Class != "usage" {
				t.Errorf("stdout = %q (%v), want a usage envelope", out.String(), err)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("left files behind: %v", entries)
			}
		})
	}
}

func TestVersionAndLicenseVerbs_HaveAHelpPage(t *testing.T) {
	for _, name := range []string{"version", "license"} {
		var out bytes.Buffer
		if !printHelp(&out, name) || !strings.Contains(out.String(), "kb "+name) {
			t.Errorf("kb help %s: no page naming the verb", name)
		}
	}
}
