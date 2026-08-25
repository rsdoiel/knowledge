package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// RecordStatuses, RecordKinds and RecordTriggers are the vocabularies from
// DECISION_RECORD_FORMAT.md. They are documented and reported against, not
// enforced: a value outside them parses and is carried, with a warning.
// Rejecting one would turn a typo in a file several harnesses write into a
// failed run rather than a fixable row.
var (
	RecordStatuses = []string{"proposed", "accepted", "superseded", "rejected"}
	RecordKinds    = []string{"decision", "correction", "refinement"}
	RecordTriggers = []string{
		"design", "plan-review", "implementation", "live-test",
		"release-review", "request", "external",
	}
)

// qstr is a string that always emits double-quoted, whatever its value.
//
// Plain yaml.Marshal quotes a string only when leaving it bare would change
// how it reparses, which makes the rendering value-dependent: phase "20.51"
// comes out quoted because it would otherwise resolve as a float, while
// "0.0.46" comes out bare because it would not. The file format requires both
// quoted. Decoding takes the node's value verbatim, so an unquoted scalar in a
// hand-edited file still yields its source text and a zero-padded id keeps its
// padding.
type qstr string

// MarshalYAML emits the value as a double-quoted scalar.
func (q qstr) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Style: yaml.DoubleQuotedStyle,
		Value: string(q),
	}, nil
}

// UnmarshalYAML takes the scalar's value as written, so that a bare 0142 does
// not become the integer 142 and a bare 2026-08-19 does not become a
// timestamp.
func (q *qstr) UnmarshalYAML(n *yaml.Node) error {
	*q = qstr(n.Value)
	return nil
}

// recordFrontmatter is the canonical on-disk shape of a decision record's
// frontmatter. Field order is the emitted key order, and matches all 198 live
// records. The struct tags are the format specification: qstr fields render
// double-quoted, plain string fields render bare when populated and as "" when
// empty, and every sequence is flow-styled per the format's "inline lists
// only" rule.
type recordFrontmatter struct {
	ID           qstr     `yaml:"id"`
	Title        qstr     `yaml:"title"`
	Date         qstr     `yaml:"date"`
	Status       string   `yaml:"status"`
	Kind         string   `yaml:"kind"`
	Trigger      string   `yaml:"trigger"`
	Project      string   `yaml:"project"`
	Phase        qstr     `yaml:"phase"`
	Supersedes   []qstr   `yaml:"supersedes,flow"`
	SupersededBy []qstr   `yaml:"superseded_by,flow"`
	RelatesTo    []qstr   `yaml:"relates_to,flow"`
	Initiative   qstr     `yaml:"initiative"`
	Session      string   `yaml:"session"`
	Decisions    []qstr   `yaml:"decisions,flow"`
	Tags         []string `yaml:"tags,flow"`
	UUID         qstr     `yaml:"uuid"`
	OriginHost   qstr     `yaml:"origin_host"`
}

// quotedFields are the frontmatter keys the format requires double-quoted.
// A value written any other way is reported so that a hand edit which dropped
// the quotes is visible, rather than being silently re-quoted on the next
// render.
var quotedFields = []string{"id", "title", "date", "phase", "initiative", "uuid", "origin_host"}

// requiredFields must be present. id, title, date, status and kind must also
// be non-empty; project is required but empty at the workspace tier.
var requiredFields = []string{"id", "title", "date", "status", "kind", "project"}

// unknownField is a frontmatter key this version does not model, kept so that
// a round trip does not silently discard it.
type unknownField struct {
	key   *yaml.Node
	value *yaml.Node
}

/** RecordFile is one parsed decision record: the database-bound fields in
 * Record, plus the file-level fields that are not columns of the records
 * table.
 *
 * Supersedes, SupersededBy and RelatesTo hold raw frontmatter entries — bare
 * ids, or `scope:id` for a cross-tier reference — and are resolved into
 * record_relations at ingest, not here. Decisions and Tags have no column yet
 * and are carried so that a round trip preserves them.
 *
 * Fields:
 *   Record       (Record)   — the fields that map onto the records table.
 *   ProjectName  (string)   — frontmatter "project"; empty at the workspace tier.
 *   Supersedes   ([]string) — ids this record replaces.
 *   SupersededBy ([]string) — ids that replace this one.
 *   RelatesTo    ([]string) — non-superseding cross-references.
 *   Decisions    ([]string) — one-line summaries for a multi-decision episode.
 *   Tags         ([]string) — free-form tags.
 *   Warnings     ([]string) — non-fatal problems found while parsing.
 *
 * Example:
 *   rf, err := ParseRecordFile("decisions/0008-config-resolution.md")
 *   for _, w := range rf.Warnings {
 *       fmt.Fprintln(os.Stderr, w)
 *   }
 */
type RecordFile struct {
	Record       Record
	ProjectName  string
	Supersedes   []string
	SupersededBy []string
	RelatesTo    []string
	Decisions    []string
	Tags         []string
	Warnings     []string

	unknown []unknownField
}

/** ParseRecordFile reads and parses the decision record at path.
 *
 * Parameters:
 *   path (string) — path to the record file.
 *
 * Returns:
 *   *RecordFile — the parsed record, with Path set to path.
 *   error       — on a read failure, missing frontmatter, or a missing
 *                 required field. Vocabulary and quoting problems are
 *                 reported in Warnings instead.
 *
 * Example:
 *   rf, err := ParseRecordFile("decisions/0160-iam-instance-profile.md")
 *   fmt.Println(rf.Record.Title)
 */
func ParseRecordFile(path string) (*RecordFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read record %s: %w", path, err)
	}
	return ParseRecord(data, path)
}

/** ParseRecord parses a decision record from bytes already in memory.
 *
 * The body is everything after the closing fence, kept verbatim: the format
 * uses bold-lead sections rather than headings precisely so that conversion
 * copies bodies byte for byte.
 *
 * Parameters:
 *   data ([]byte) — the whole file, frontmatter and body.
 *   path (string) — the path to record on the parsed Record.
 *
 * Returns:
 *   *RecordFile — the parsed record.
 *   error       — on missing or malformed frontmatter, or a missing required
 *                 field.
 *
 * Example:
 *   rf, err := ParseRecord(data, "decisions/0008-config-resolution.md")
 */
func ParseRecord(data []byte, path string) (*RecordFile, error) {
	front, body, err := splitFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("knowledge: %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(front), &doc); err != nil {
		return nil, fmt.Errorf("knowledge: %s: parse frontmatter: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("knowledge: %s: frontmatter is not a mapping", path)
	}
	mapping := doc.Content[0]

	var fm recordFrontmatter
	if err := mapping.Decode(&fm); err != nil {
		return nil, fmt.Errorf("knowledge: %s: decode frontmatter: %w", path, err)
	}

	rf := &RecordFile{
		Record: Record{
			RecordID:   string(fm.ID),
			Path:       path,
			Title:      string(fm.Title),
			Date:       string(fm.Date),
			Status:     fm.Status,
			Kind:       fm.Kind,
			Trigger:    fm.Trigger,
			Phase:      string(fm.Phase),
			Initiative: string(fm.Initiative),
			Session:    fm.Session,
			Body:       body,
			Checksum:   checksum(data),
			UUID:       string(fm.UUID),
			OriginHost: string(fm.OriginHost),
		},
		ProjectName:  fm.Project,
		Supersedes:   fromQstr(fm.Supersedes),
		SupersededBy: fromQstr(fm.SupersededBy),
		RelatesTo:    fromQstr(fm.RelatesTo),
		Decisions:    fromQstr(fm.Decisions),
		Tags:         fm.Tags,
	}
	rf.Record.Scope = "project"
	if rf.ProjectName == "" {
		rf.Record.Scope = "workspace"
	}

	present, err := inspectMapping(rf, mapping)
	if err != nil {
		return nil, fmt.Errorf("knowledge: %s: %w", path, err)
	}
	for _, name := range requiredFields {
		if !present[name] {
			return nil, fmt.Errorf("knowledge: %s: required frontmatter field %q is missing", path, name)
		}
	}
	for _, tc := range []struct{ name, value string }{
		{"id", rf.Record.RecordID},
		{"title", rf.Record.Title},
		{"date", rf.Record.Date},
		{"status", rf.Record.Status},
		{"kind", rf.Record.Kind},
	} {
		if tc.value == "" {
			return nil, fmt.Errorf("knowledge: %s: required frontmatter field %q is empty", path, tc.name)
		}
	}

	rf.checkVocabulary("status", rf.Record.Status, RecordStatuses, false)
	rf.checkVocabulary("kind", rf.Record.Kind, RecordKinds, false)
	rf.checkVocabulary("trigger", rf.Record.Trigger, RecordTriggers, true)

	return rf, nil
}

// inspectMapping walks the frontmatter mapping to record which keys are
// present, warn about scalars the format requires quoted, and capture keys
// this version does not model. It returns the set of keys seen.
func inspectMapping(rf *RecordFile, mapping *yaml.Node) (map[string]bool, error) {
	if len(mapping.Content)%2 != 0 {
		return nil, fmt.Errorf("malformed frontmatter mapping")
	}
	known := knownFieldNames()
	quoted := make(map[string]bool, len(quotedFields))
	for _, f := range quotedFields {
		quoted[f] = true
	}

	present := make(map[string]bool, len(mapping.Content)/2)
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		present[key.Value] = true

		if !known[key.Value] {
			rf.unknown = append(rf.unknown, unknownField{key: key, value: value})
			rf.Warnings = append(rf.Warnings, fmt.Sprintf(
				"unknown frontmatter field %q preserved but not indexed", key.Value))
			continue
		}
		if quoted[key.Value] && value.Kind == yaml.ScalarNode && value.Style != yaml.DoubleQuotedStyle {
			rf.Warnings = append(rf.Warnings, fmt.Sprintf(
				"field %q is written as %s but the format requires it double-quoted; it will be re-quoted on the next write",
				key.Value, describeScalar(value)))
		}
	}
	return present, nil
}

// describeScalar names how a scalar was written, for a warning message.
func describeScalar(n *yaml.Node) string {
	switch n.Tag {
	case "!!int", "!!float", "!!timestamp", "!!bool", "!!null":
		return "an unquoted " + strings.TrimPrefix(n.Tag, "!!")
	}
	if n.Style == yaml.SingleQuotedStyle {
		return "a single-quoted string"
	}
	return "an unquoted string"
}

// knownFieldNames returns the frontmatter keys this version models, taken from
// recordFrontmatter's own tags so the two cannot drift.
func knownFieldNames() map[string]bool {
	out := make(map[string]bool, 17)
	for _, tag := range []string{
		"id", "title", "date", "status", "kind", "trigger", "project", "phase",
		"supersedes", "superseded_by", "relates_to", "initiative", "session",
		"decisions", "tags", "uuid", "origin_host",
	} {
		out[tag] = true
	}
	return out
}

// checkVocabulary reports a value outside its documented vocabulary. Empty is
// permitted only where allowEmpty says so — a converted record may carry
// trigger: "" and that is preferred to a guess.
func (rf *RecordFile) checkVocabulary(field, value string, vocabulary []string, allowEmpty bool) {
	if value == "" {
		if !allowEmpty {
			rf.Warnings = append(rf.Warnings, fmt.Sprintf("field %q is empty", field))
		}
		return
	}
	for _, v := range vocabulary {
		if v == value {
			return
		}
	}
	rf.Warnings = append(rf.Warnings, fmt.Sprintf(
		"field %q has value %q, which is outside the documented vocabulary (%s)",
		field, value, strings.Join(vocabulary, ", ")))
}

/** RenderRecordFile writes a record back out in canonical form: the
 * frontmatter in the documented key order with the documented quoting, then
 * the body verbatim.
 *
 * Rendering is a fixed point — parsing the output and rendering again produces
 * identical bytes — which is what lets ingest and `kb record` share one
 * writer without a file's original formatting leaking downstream.
 *
 * Parameters:
 *   rf (*RecordFile) — the record to render.
 *
 * Returns:
 *   []byte — the complete file contents.
 *   error  — if the frontmatter cannot be marshalled.
 *
 * Example:
 *   out, err := RenderRecordFile(rf)
 *   err = os.WriteFile(rf.Record.Path, out, 0o644)
 */
func RenderRecordFile(rf *RecordFile) ([]byte, error) {
	fm := recordFrontmatter{
		ID:           qstr(rf.Record.RecordID),
		Title:        qstr(rf.Record.Title),
		Date:         qstr(rf.Record.Date),
		Status:       rf.Record.Status,
		Kind:         rf.Record.Kind,
		Trigger:      rf.Record.Trigger,
		Project:      rf.ProjectName,
		Phase:        qstr(rf.Record.Phase),
		Supersedes:   toQstr(rf.Supersedes),
		SupersededBy: toQstr(rf.SupersededBy),
		RelatesTo:    toQstr(rf.RelatesTo),
		Initiative:   qstr(rf.Record.Initiative),
		Session:      rf.Record.Session,
		Decisions:    toQstr(rf.Decisions),
		Tags:         nonNil(rf.Tags),
		UUID:         qstr(rf.Record.UUID),
		OriginHost:   qstr(rf.Record.OriginHost),
	}

	front, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("knowledge: render frontmatter: %w", err)
	}

	var extra []byte
	if len(rf.unknown) > 0 {
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, u := range rf.unknown {
			node.Content = append(node.Content, u.key, u.value)
		}
		extra, err = yaml.Marshal(node)
		if err != nil {
			return nil, fmt.Errorf("knowledge: render preserved fields: %w", err)
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(front)
	b.Write(extra)
	b.WriteString("---\n")
	b.WriteString(rf.Record.Body)
	return []byte(b.String()), nil
}

// splitFrontmatter separates the YAML frontmatter from the body. The body is
// everything after the closing fence line, reconstructed exactly.
func splitFrontmatter(s string) (front, body string, err error) {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return "", "", fmt.Errorf("no frontmatter: file does not start with ---")
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", fmt.Errorf("no frontmatter: closing --- not found")
}

// checksum returns a stable digest of the raw file bytes, so that ingest can
// skip a file that has not changed.
func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// fromQstr converts a decoded qstr slice to plain strings.
func fromQstr(in []qstr) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

// toQstr converts plain strings to qstr, returning an empty (never nil) slice
// so the emitter writes [] rather than null.
func toQstr(in []string) []qstr {
	out := make([]qstr, len(in))
	for i, v := range in {
		out[i] = qstr(v)
	}
	return out
}

// nonNil returns an empty slice in place of a nil one, so the emitter writes
// [] rather than null.
func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
