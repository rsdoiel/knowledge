package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleRecord is a minimal well-formed record file in canonical form.
const sampleRecord = `---
id: "0008"
title: "Config resolution is YAML"
date: "2026-06-18"
status: accepted
kind: decision
trigger: implementation
project: CMTools
phase: "0.0.46"
supersedes: ["0003"]
superseded_by: []
relates_to: ["0003", "0009"]
initiative: ""
session: ""
decisions: ["YAML, not JSON, as the config file format"]
tags: [config, yaml, precedence]
uuid: "01a03af1-e5cb-73c6-a794-931c837a1c2e"
origin_host: "MACMINI-RD.local"
---

**Context.** Config had no defined resolution order.

**Decision.** YAML, merged from cwd up to $HOME.
`

// withField returns sampleRecord with the named frontmatter field replaced by
// the given full line, so a test can vary one field at a time.
func withField(t *testing.T, field, line string) string {
	t.Helper()
	lines := strings.Split(sampleRecord, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, field+":") {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	t.Fatalf("field %q not present in sampleRecord", field)
	return ""
}

// withoutField returns sampleRecord with the named frontmatter line removed.
func withoutField(t *testing.T, field string) string {
	t.Helper()
	lines := strings.Split(sampleRecord, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, field+":") {
			return strings.Join(append(lines[:i:i], lines[i+1:]...), "\n")
		}
	}
	t.Fatalf("field %q not present in sampleRecord", field)
	return ""
}

func mustParse(t *testing.T, src string) *RecordFile {
	t.Helper()
	rf, err := ParseRecord([]byte(src), "decisions/0008-config-resolution.md")
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	return rf
}

func TestParseRecord_Fields(t *testing.T) {
	rf := mustParse(t, sampleRecord)

	for _, tc := range []struct{ field, got, want string }{
		{"RecordID", rf.Record.RecordID, "0008"},
		{"Title", rf.Record.Title, "Config resolution is YAML"},
		{"Date", rf.Record.Date, "2026-06-18"},
		{"Status", rf.Record.Status, "accepted"},
		{"Kind", rf.Record.Kind, "decision"},
		{"Trigger", rf.Record.Trigger, "implementation"},
		{"Phase", rf.Record.Phase, "0.0.46"},
		{"Initiative", rf.Record.Initiative, ""},
		{"Session", rf.Record.Session, ""},
		{"UUID", rf.Record.UUID, "01a03af1-e5cb-73c6-a794-931c837a1c2e"},
		{"OriginHost", rf.Record.OriginHost, "MACMINI-RD.local"},
		{"ProjectName", rf.ProjectName, "CMTools"},
		{"Scope", rf.Record.Scope, "project"},
		{"Path", rf.Record.Path, "decisions/0008-config-resolution.md"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}

	for _, tc := range []struct {
		field string
		got   []string
		want  []string
	}{
		{"Supersedes", rf.Supersedes, []string{"0003"}},
		{"SupersededBy", rf.SupersededBy, nil},
		{"RelatesTo", rf.RelatesTo, []string{"0003", "0009"}},
		{"Decisions", rf.Decisions, []string{"YAML, not JSON, as the config file format"}},
		{"Tags", rf.Tags, []string{"config", "yaml", "precedence"}},
	} {
		if len(tc.got) != len(tc.want) {
			t.Errorf("%s = %v, want %v", tc.field, tc.got, tc.want)
			continue
		}
		for i := range tc.want {
			if tc.got[i] != tc.want[i] {
				t.Errorf("%s[%d] = %q, want %q", tc.field, i, tc.got[i], tc.want[i])
			}
		}
	}
}

// The workspace tier is identified by an empty project.
func TestParseRecord_EmptyProjectIsWorkspaceScope(t *testing.T) {
	rf := mustParse(t, withField(t, "project", `project: ""`))
	if rf.Record.Scope != "workspace" {
		t.Errorf("Scope = %q, want %q for a record with an empty project", rf.Record.Scope, "workspace")
	}
	if rf.ProjectName != "" {
		t.Errorf("ProjectName = %q, want empty", rf.ProjectName)
	}
}

// Body is everything after the closing fence, verbatim.
func TestParseRecord_BodyIsVerbatim(t *testing.T) {
	rf := mustParse(t, sampleRecord)
	want := "\n**Context.** Config had no defined resolution order.\n\n**Decision.** YAML, merged from cwd up to $HOME.\n"
	if rf.Record.Body != want {
		t.Errorf("Body = %q,\nwant %q", rf.Record.Body, want)
	}
}

func TestParseRecord_ChecksumIsOverRawBytes(t *testing.T) {
	rf := mustParse(t, sampleRecord)
	if rf.Record.Checksum == "" {
		t.Fatal("Checksum is empty")
	}
	changed := mustParse(t, strings.Replace(sampleRecord, "resolution order", "resolution ordering", 1))
	if changed.Record.Checksum == rf.Record.Checksum {
		t.Error("Checksum did not change when the body changed; ingest could not detect the edit")
	}
	same := mustParse(t, sampleRecord)
	if same.Record.Checksum != rf.Record.Checksum {
		t.Error("Checksum is not stable across two parses of identical bytes")
	}
}

// Plan finding 2: trigger: "" is valid on a converted record.
func TestParseRecord_EmptyTriggerIsValid(t *testing.T) {
	rf, err := ParseRecord([]byte(withField(t, "trigger", `trigger: ""`)), "decisions/0008-x.md")
	if err != nil {
		t.Fatalf("an empty trigger must parse, got: %v", err)
	}
	for _, w := range rf.Warnings {
		if strings.Contains(w, "trigger") {
			t.Errorf("an empty trigger must not warn, got %q", w)
		}
	}
}

// Plan finding 1: superseded_by does not imply status: superseded.
func TestParseRecord_AcceptedWithSupersededBy(t *testing.T) {
	src := withField(t, "superseded_by", `superseded_by: ["0159"]`)
	rf, err := ParseRecord([]byte(src), "decisions/0160-x.md")
	if err != nil {
		t.Fatalf("accepted + superseded_by must parse, got: %v", err)
	}
	if rf.Record.Status != "accepted" {
		t.Errorf("Status = %q, want accepted — parsing must not derive status from superseded_by", rf.Record.Status)
	}
	if len(rf.SupersededBy) != 1 || rf.SupersededBy[0] != "0159" {
		t.Errorf("SupersededBy = %v, want [0159]", rf.SupersededBy)
	}
}

func TestParseRecord_MissingRequiredFieldIsAnError(t *testing.T) {
	for _, field := range []string{"id", "title", "date", "status", "kind"} {
		if _, err := ParseRecord([]byte(withoutField(t, field)), "decisions/0008-x.md"); err == nil {
			t.Errorf("a record missing %q parsed without error", field)
		}
	}
}

func TestParseRecord_NoFrontmatterIsAnError(t *testing.T) {
	for _, src := range []string{
		"**Context.** no frontmatter at all\n",
		"---\nid: \"0001\"\nnever closed\n",
		"",
	} {
		if _, err := ParseRecord([]byte(src), "decisions/0001-x.md"); err == nil {
			t.Errorf("malformed input parsed without error: %q", src)
		}
	}
}

// Vocabularies are closed in the format doc but enforcement would turn a typo
// into a failed run, so an unknown value is reported and carried, matching the
// treatment of unresolvable references and unquoted scalars.
func TestParseRecord_UnknownVocabularyWarnsButParses(t *testing.T) {
	for _, tc := range []struct{ field, line, bad string }{
		{"status", `status: agreed`, "agreed"},
		{"kind", `kind: architecture`, "architecture"},
		{"trigger", `trigger: brainstorm`, "brainstorm"},
	} {
		rf, err := ParseRecord([]byte(withField(t, tc.field, tc.line)), "decisions/0008-x.md")
		if err != nil {
			t.Errorf("%s: an out-of-vocabulary value must not fail the parse, got: %v", tc.field, err)
			continue
		}
		if !warnsAbout(rf.Warnings, tc.bad) {
			t.Errorf("%s: expected a warning naming %q, got %v", tc.field, tc.bad, rf.Warnings)
		}
	}
}

func TestParseRecord_KnownVocabularyDoesNotWarn(t *testing.T) {
	for _, tc := range []struct{ field, line string }{
		{"status", `status: proposed`},
		{"status", `status: superseded`},
		{"status", `status: rejected`},
		{"kind", `kind: correction`},
		{"kind", `kind: refinement`},
		{"trigger", `trigger: plan-review`},
		{"trigger", `trigger: release-review`},
		{"trigger", `trigger: external`},
		{"trigger", `trigger: request`},
		{"trigger", `trigger: design`},
		{"trigger", `trigger: live-test`},
	} {
		rf, err := ParseRecord([]byte(withField(t, tc.field, tc.line)), "decisions/0008-x.md")
		if err != nil {
			t.Fatalf("%s: %v", tc.line, err)
		}
		if len(rf.Warnings) != 0 {
			t.Errorf("%s produced warnings %v, want none", tc.line, rf.Warnings)
		}
	}
}

// An unquoted id or date parses (yaml.v3 hands back the source text, so no
// padding is lost) but is reported, so a hand edit that dropped the quotes is
// visible rather than silently re-quoted on the next render.
func TestParseRecord_UnquotedScalarIsReportedNotFatal(t *testing.T) {
	for _, tc := range []struct{ field, line, want string }{
		{"id", `id: 0008`, "id"},
		{"date", `date: 2026-06-18`, "date"},
		{"phase", `phase: 20.51`, "phase"},
	} {
		rf, err := ParseRecord([]byte(withField(t, tc.field, tc.line)), "decisions/0008-x.md")
		if err != nil {
			t.Errorf("%s: an unquoted scalar must not fail the parse, got: %v", tc.field, err)
			continue
		}
		if !warnsAbout(rf.Warnings, tc.want) {
			t.Errorf("%s: expected a warning naming %q, got %v", tc.field, tc.want, rf.Warnings)
		}
	}
}

func TestParseRecord_UnquotedIDKeepsItsPadding(t *testing.T) {
	rf, err := ParseRecord([]byte(withField(t, "id", `id: 0008`)), "decisions/0008-x.md")
	if err != nil {
		t.Fatalf("ParseRecord: %v", err)
	}
	if rf.Record.RecordID != "0008" {
		t.Errorf("RecordID = %q, want %q — zero padding must survive an unquoted scalar", rf.Record.RecordID, "0008")
	}
}

func TestParseRecord_UnknownFieldsArePreserved(t *testing.T) {
	src := strings.Replace(sampleRecord, "tags: [config, yaml, precedence]",
		"tags: [config, yaml, precedence]\nreviewer: rsdoiel", 1)
	rf, err := ParseRecord([]byte(src), "decisions/0008-x.md")
	if err != nil {
		t.Fatalf("an unknown field must not fail the parse, got: %v", err)
	}
	if !warnsAbout(rf.Warnings, "reviewer") {
		t.Errorf("expected a warning naming the unknown field, got %v", rf.Warnings)
	}
	out, err := RenderRecordFile(rf)
	if err != nil {
		t.Fatalf("RenderRecordFile: %v", err)
	}
	if !strings.Contains(string(out), "reviewer: rsdoiel") {
		t.Errorf("unknown field was dropped on render:\n%s", out)
	}
}

// The canonical rendering, field by field. This is the format specification.
func TestRenderRecordFile_CanonicalQuoting(t *testing.T) {
	rf := mustParse(t, sampleRecord)
	out, err := RenderRecordFile(rf)
	if err != nil {
		t.Fatalf("RenderRecordFile: %v", err)
	}
	got := string(out)

	for _, want := range []string{
		`id: "0008"`,
		`title: "Config resolution is YAML"`,
		`date: "2026-06-18"`,
		`status: accepted`,
		`kind: decision`,
		`trigger: implementation`,
		`project: CMTools`,
		`phase: "0.0.46"`,
		`supersedes: ["0003"]`,
		`superseded_by: []`,
		`relates_to: ["0003", "0009"]`,
		`initiative: ""`,
		`session: ""`,
		`decisions: ["YAML, not JSON, as the config file format"]`,
		`tags: [config, yaml, precedence]`,
		`uuid: "01a03af1-e5cb-73c6-a794-931c837a1c2e"`,
		`origin_host: "MACMINI-RD.local"`,
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("rendered output is missing line:\n  %s\ngot:\n%s", want, got)
		}
	}
}

// phase is quoted whether or not its value happens to look like a float.
// Plain yaml.Marshal quotes "20.51" and leaves 0.0.46 bare, which would make
// the rendering value-dependent.
func TestRenderRecordFile_PhaseAlwaysQuoted(t *testing.T) {
	for _, phase := range []string{"0.0.46", "20.51", "0.0.45", ""} {
		rf := mustParse(t, withField(t, "phase", `phase: "`+phase+`"`))
		out, err := RenderRecordFile(rf)
		if err != nil {
			t.Fatalf("RenderRecordFile: %v", err)
		}
		want := `phase: "` + phase + `"` + "\n"
		if !strings.Contains(string(out), want) {
			t.Errorf("phase %q rendered without quotes; want line %s", phase, strings.TrimSpace(want))
		}
	}
}

func TestRenderRecordFile_IsIdempotent(t *testing.T) {
	rf := mustParse(t, sampleRecord)
	first, err := RenderRecordFile(rf)
	if err != nil {
		t.Fatalf("RenderRecordFile: %v", err)
	}
	again, err := ParseRecord(first, rf.Record.Path)
	if err != nil {
		t.Fatalf("re-parsing rendered output: %v", err)
	}
	second, err := RenderRecordFile(again)
	if err != nil {
		t.Fatalf("second RenderRecordFile: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("render is not a fixed point:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// clasm DR-0164's title embeds escaped quotes; others carry backticks and
// angle brackets.
func TestRenderRecordFile_AwkwardTitlesRoundTrip(t *testing.T) {
	for _, title := range []string{
		`clasm DR-0164 embeds "escaped quotes" here`,
		"a title with `backticks` and a ~/path",
		"angle <brackets> and a colon: here",
		`a backslash \ and a trailing quote "`,
		"$HOME and a #hash",
	} {
		rf := mustParse(t, sampleRecord)
		rf.Record.Title = title
		out, err := RenderRecordFile(rf)
		if err != nil {
			t.Fatalf("RenderRecordFile(%q): %v", title, err)
		}
		back, err := ParseRecord(out, rf.Record.Path)
		if err != nil {
			t.Fatalf("re-parsing %q: %v", title, err)
		}
		if back.Record.Title != title {
			t.Errorf("title did not survive the round trip:\n got %q\nwant %q", back.Record.Title, title)
		}
		if n := strings.Count(strings.SplitN(string(out), "---\n", 3)[1], "\ntitle:"); n != 1 {
			t.Errorf("title %q was not emitted as exactly one line", title)
		}
	}
}

// A long title must stay on one line: the format keeps it greppable, and
// yaml.v3's emitter folds long scalars in some configurations.
func TestRenderRecordFile_LongValuesAreNotFolded(t *testing.T) {
	rf := mustParse(t, sampleRecord)
	rf.Record.Title = "Restore OpenSearch Snapshot from S3: the restore index prefix must be editable, not silently derived from the target's own tags, because a live restore proved otherwise"
	rf.Decisions = []string{
		"A fairly long one-line summary of what was decided, with `backticks` and a ~/path in it",
		"A second one, equally long, so that the flow sequence comfortably exceeds any folding width",
		"A third, to be certain the emitter is not wrapping at eighty columns",
	}
	out, err := RenderRecordFile(rf)
	if err != nil {
		t.Fatalf("RenderRecordFile: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, " ") && strings.TrimSpace(line) != "" {
			t.Errorf("emitter folded a long value onto a continuation line: %q", line)
		}
	}
	if !strings.Contains(string(out), `title: "`+rf.Record.Title+`"`) {
		t.Error("long title was not emitted on a single line")
	}
}

func warnsAbout(warnings []string, needle string) bool {
	for _, w := range warnings {
		if strings.Contains(w, needle) {
			return true
		}
	}
	return false
}

// corpusDirs returns the live decision-record directories present on this
// machine, skipping the test when none are.
func corpusDirs(t *testing.T) []string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	candidates := []string{
		filepath.Join(home, "WorkLab", "clasm", "decisions"),
		filepath.Join(home, "WorkLab", "CMTools", "decisions"),
		filepath.Join(home, "WorkLab", "cold", "decisions"),
		filepath.Join(home, "WorkLab", "agents", "decisions"),
		filepath.Join(home, "Laboratory", "knowledge", "decisions"),
	}
	var out []string
	for _, d := range candidates {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		t.Skip("no decision-record corpora on this machine")
	}
	return out
}

// normalizesOnRender lists records known to differ from canonical form. It is
// deliberately empty: it once held the six CMTools records whose decisions[]
// used a block sequence, which DECISION_RECORD_FORMAT.md forbids ("inline
// lists only"), and `kb record fmt` normalised them in W5. Every record in
// every corpus now round-trips byte-identically.
//
// The map stays rather than being inlined as a zero check, because it is the
// mechanism that made the normalisation visible: the test asserts this set
// exactly, so a record that starts diverging fails, and so does one listed
// here that stops.
var normalizesOnRender = map[string]bool{}

// The phase's real acceptance test: parse then render every live record and
// compare bytes.
func TestParseRecordFile_RoundTripsEveryLiveRecord(t *testing.T) {
	dirs := corpusDirs(t)

	var total int
	diverged := map[string]bool{}

	for _, dir := range dirs {
		corpus := filepath.Base(filepath.Dir(dir))
		files, err := filepath.Glob(filepath.Join(dir, "[0-9]*.md"))
		if err != nil {
			t.Fatalf("globbing %s: %v", dir, err)
		}
		for _, path := range files {
			total++
			key := corpus + "/" + filepath.Base(path)[:4]

			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			rf, err := ParseRecord(raw, path)
			if err != nil {
				t.Errorf("%s: parse failed: %v", key, err)
				continue
			}
			out, err := RenderRecordFile(rf)
			if err != nil {
				t.Errorf("%s: render failed: %v", key, err)
				continue
			}

			if string(out) != string(raw) {
				diverged[key] = true
				// Whatever the divergence, rendering must be a fixed point.
				again, err := ParseRecord(out, path)
				if err != nil {
					t.Errorf("%s: re-parsing rendered output failed: %v", key, err)
					continue
				}
				second, err := RenderRecordFile(again)
				if err != nil {
					t.Errorf("%s: second render failed: %v", key, err)
					continue
				}
				if string(second) != string(out) {
					t.Errorf("%s: render is not a fixed point", key)
				}
			}
		}
	}

	if total < 190 {
		t.Fatalf("only found %d records across %d corpora; expected ~198", total, len(dirs))
	}

	for key := range diverged {
		if !normalizesOnRender[key] {
			t.Errorf("%s changed on render but is not a known normalization case", key)
		}
	}
	for key := range normalizesOnRender {
		if !diverged[key] {
			t.Errorf("%s was expected to normalize on render but round-tripped unchanged", key)
		}
	}
	t.Logf("round-tripped %d records; %d normalized as expected", total, len(diverged))
}

// The record-level assertion that normalisation inlines decisions[] without
// touching the body now lives in TestCmdRecord_FmtNormalisesAndReports, which
// uses a fixture. The corpus-based version that stood here became vacuous once
// kb record fmt normalised CMTools in W5 and left nothing divergent to check.

// bodyOf returns everything after the second --- fence.
func bodyOf(s string) string {
	parts := strings.SplitN(s, "---\n", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
