package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The knowledge skills and the agent document ship in this repository (TODO,
// filed 2026-09-25; knowledge DR-0076). These tests keep them from drifting from
// the release: a skill whose kb_version is not this release's, or an agent
// document that does not name a verb, fails the build, so release prep cannot
// skip them.

var skillNames = []string{"setup-knowledge-base", "update-knowledge-base", "review-knowledge-base"}

func repoFile(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func codemetaVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(repoFile("codemeta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cm struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &cm); err != nil || cm.Version == "" {
		t.Fatalf("codemeta.json version: %v", err)
	}
	return cm.Version
}

func TestSkills_EachIsCompleteAndPinnedToThisRelease(t *testing.T) {
	want := codemetaVersion(t)
	for _, name := range skillNames {
		t.Run(name, func(t *testing.T) {
			dir := repoFile("agents", "skills", name)
			raw, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
			if err != nil {
				t.Fatalf("no SKILL.md: %v", err)
			}
			parts := strings.SplitN(string(raw), "---", 3)
			if len(parts) < 3 {
				t.Fatal("SKILL.md has no frontmatter")
			}
			var fm struct {
				Name          string `yaml:"name"`
				Compatibility string `yaml:"compatibility"`
				Metadata      struct {
					Version   string `yaml:"version"`
					KBVersion string `yaml:"kb_version"`
				} `yaml:"metadata"`
			}
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
				t.Fatalf("frontmatter: %v", err)
			}
			if fm.Name != name {
				t.Errorf("name = %q, want %q (the directory)", fm.Name, name)
			}
			if fm.Metadata.KBVersion != want {
				t.Errorf("kb_version = %q, want %q (codemeta.json): bump it, after running the skill against this kb", fm.Metadata.KBVersion, want)
			}
			if fm.Metadata.Version == "" || !strings.Contains(fm.Compatibility, "claude-code") || !strings.Contains(fm.Compatibility, "harvey") {
				t.Errorf("version %q / compatibility %q incomplete", fm.Metadata.Version, fm.Compatibility)
			}
			for _, script := range []string{"compiled.bash", "compiled.ps1"} {
				if _, err := os.Stat(filepath.Join(dir, "scripts", script)); err != nil {
					t.Errorf("missing scripts/%s", script)
				}
			}
		})
	}
}

func TestAgentsDocument_NamesEveryVerbAndEveryExitCode(t *testing.T) {
	raw, err := os.ReadFile(repoFile("AGENTS.md"))
	if err != nil {
		t.Fatalf("no AGENTS.md: %v", err)
	}
	doc := string(raw)
	for _, v := range verbTable {
		if !strings.Contains(doc, "`"+v.Name+"`") && !strings.Contains(doc, "kb "+v.Name) {
			t.Errorf("AGENTS.md does not mention the verb %q", v.Name)
		}
	}
	for _, code := range []string{"| 0 ", "| 1 ", "| 2 ", "| 65 ", "| 66 ", "| 69 ", "| 70 ", "| 73 ", "| 74 ", "| 75 ", "| 77 ", "| 78 "} {
		if !strings.Contains(doc, code) {
			t.Errorf("AGENTS.md has no exit-code row %q", strings.TrimSpace(strings.Trim(code, "|")))
		}
	}
	for _, rule := range []string{"never accept", "raw SQL", "--json"} {
		if !strings.Contains(strings.ToLower(doc), strings.ToLower(rule)) {
			t.Errorf("AGENTS.md does not state %q", rule)
		}
	}
}

func TestClaudeMD_PointsAtAgentsMD(t *testing.T) {
	raw, err := os.ReadFile(repoFile("CLAUDE.md"))
	if err != nil {
		t.Fatalf("no CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(raw), "AGENTS.md") {
		t.Error("CLAUDE.md does not point at AGENTS.md")
	}
}

func TestMakefile_ShipsAndInstallsTheSkills(t *testing.T) {
	raw, err := os.ReadFile(repoFile("Makefile"))
	if err != nil {
		t.Skip("no Makefile")
	}
	mk := string(raw)
	for _, line := range strings.Split(mk, "\n") {
		if strings.Contains(line, "zip -r") && !strings.Contains(line, " agents") {
			t.Errorf("a release zip leaves out agents/: %s", strings.TrimSpace(line))
		}
	}
	for _, target := range []string{"install-skills:", "check-skills:"} {
		if !strings.Contains(mk, "\n"+target) {
			t.Errorf("Makefile has no %s target", target)
		}
	}
	if strings.Contains(strings.SplitN(mk, "\ninstall:", 2)[1][:400], "install-skills") {
		t.Error("`make install` must not install skills: they go in the user's agent configuration, so that is opt-in")
	}
}

// The skills describe the story verbs, concept kinds and kb stats as released
// (T4 of v0.0.25); a phrase going missing fails the build.
func TestSkills_DescribeTheV0025Features(t *testing.T) {
	want := map[string][]string{
		"update-knowledge-base": {"story who-knows", "story who-learns", "story timeline", "story check", "concept set-kind", "--kind", "source link --document", "learns", "knows", "{#12A}"},
		"review-knowledge-base": {"kb stats", "story check", "story:<project>"},
		"setup-knowledge-base":  {"story_events", "story_knowledge", "document_sources", "concepts.kind"},
	}
	for name, phrases := range want {
		raw, err := os.ReadFile(repoFile("agents", "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range phrases {
			if !strings.Contains(string(raw), p) {
				t.Errorf("%s does not mention %q", name, p)
			}
		}
	}
}
