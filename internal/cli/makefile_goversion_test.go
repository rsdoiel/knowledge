package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

/** goVersionCheck runs `make check-go-version` in a scratch directory holding the
 * repository's Makefile, a go.mod, a codemeta.json, an optional semantic/go.mod and a
 * stub `go` that reports toolchain as its version.
 *
 * Parameters:
 *   t (*testing.T) — the test
 *   toolchain (string) — what `go env GOVERSION` prints, such as "go1.27.3"
 *   goMod (string) — the go line of go.mod, such as "1.27.3"
 *   codemeta (string) — the version after "Go >= " in codemeta.json
 *   nested (string) — the go line of semantic/go.mod, or "" for none
 *
 * Returns:
 *   string — the combined output of make
 *   error — non-nil when make exited non-zero
 *
 * Example:
 *   out, err := goVersionCheck(t, "go1.27.3", "1.27.3", "1.27.3", "1.27.3")
 */
func goVersionCheck(t *testing.T, toolchain, goMod, codemeta, nested string) (string, error) {
	t.Helper()
	for _, tool := range []string{"make", "jq", "sort"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	dir := t.TempDir()
	write := func(name, body string, mode os.FileMode) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("Makefile", string(mk), 0o644)
	write("go.mod", "module example.com/x\n\ngo "+goMod+"\n", 0o644)
	write("codemeta.json", `{"version": "0.0.1", "softwareRequirements": ["Go >= `+codemeta+`", "git >= 2.0"]}`, 0o644)
	if nested != "" {
		write("semantic/go.mod", "module example.com/x/semantic\n\ngo "+nested+"\n", 0o644)
	}
	write("bin/go", "#!/bin/sh\n[ \"$1 $2\" = \"env GOVERSION\" ] && echo "+toolchain+"\n", 0o755)
	cmd := exec.Command("make", "-s", "-f", "Makefile", "--no-builtin-rules", "--no-builtin-variables", "check-go-version")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// The Go policy (kb observation 548): the minimum Go is the highest version the
// release was built with, and go.mod, codemeta.json and the nested module agree.
func TestCheckGoVersion(t *testing.T) {
	cases := []struct {
		name                               string
		toolchain, goMod, codemeta, nested string
		wantOK                             bool
		wantInOutput                       string
	}{
		{"all agree", "go1.27.3", "1.27.3", "1.27.3", "1.27.3", true, "consistent"},
		{"no nested module", "go1.27.3", "1.27.3", "1.27.3", "", true, "consistent"},
		{"go.mod lower than the Go in use", "go1.27.3", "1.27.0", "1.27.0", "1.27.0", false, "go.mod says go 1.27.0"},
		{"codemeta differs from go.mod", "go1.27.3", "1.27.3", "1.27.0", "1.27.3", false, "codemeta.json says Go >= 1.27.0"},
		{"nested module differs", "go1.27.3", "1.27.3", "1.27.3", "1.27.0", false, "semantic/go.mod says go 1.27.0"},
		{"a go line without a patch counts as .0", "go1.27.0", "1.27", "1.27.0", "1.27", true, "consistent"},
		{"development toolchain is not guessed at", "go1.28-devel_abc", "1.27.3", "1.27.3", "", false, "cannot read the Go version"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := goVersionCheck(t, c.toolchain, c.goMod, c.codemeta, c.nested)
			if c.wantOK && err != nil {
				t.Fatalf("want success, got %v\n%s", err, out)
			}
			if !c.wantOK && err == nil {
				t.Fatalf("want failure, got success\n%s", out)
			}
			if !strings.Contains(out, c.wantInOutput) {
				t.Errorf("output lacks %q:\n%s", c.wantInOutput, out)
			}
		})
	}
}
