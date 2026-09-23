package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	knowledge "github.com/rsdoiel/knowledge"
	"gopkg.in/yaml.v3"
)

// runGit runs git with args, cwd at dir, returning trimmed stdout. Any
// exec.Command failure (non-repo, no commits yet, git not on PATH) is
// reported as an error -- callers treat that uniformly with "found nothing"
// as provenance-source failure (design decision 3: detected from the
// command's own failure, not a .git pre-check).
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// splitNonEmptyLines splits s on newlines, dropping empty lines -- git log
// output with -z-free formatting.
func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// gitFirstCommit returns the author and date of the oldest commit touching
// path, or ok=false if path isn't in any git history (not a repo, or
// genuinely untracked). Tries --diff-filter=A first (isolates the add
// commit); if that yields nothing -- e.g. a moved/renamed file, where the
// add may not appear under the current name -- falls back to the oldest
// --follow entry with no filter.
func gitFirstCommit(path string) (author, date string, ok bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	line, found := firstGitLogLine(dir, base, true)
	if !found {
		line, found = firstGitLogLine(dir, base, false)
	}
	if !found {
		return "", "", false
	}
	parts := strings.SplitN(line, "\x1f", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// firstGitLogLine runs git log --reverse (oldest first) for base, optionally
// with --diff-filter=A, and returns its first output line.
func firstGitLogLine(dir, base string, addOnly bool) (string, bool) {
	args := []string{"log", "--follow", "--reverse", "--format=%an\x1f%aI"}
	if addOnly {
		args = append(args, "--diff-filter=A")
	}
	args = append(args, "--", base)
	out, err := runGit(dir, args...)
	if err != nil {
		return "", false
	}
	lines := splitNonEmptyLines(out)
	if len(lines) == 0 {
		return "", false
	}
	return lines[0], true
}

// gitLastCommit returns the date of the most recent commit touching path, or
// ok=false if path isn't in any git history.
func gitLastCommit(path string) (date string, ok bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	out, err := runGit(dir, "log", "-1", "--format=%aI", "--", base)
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}

// gitConfigUserName returns git's configured user.name, run with cwd at dir
// (so a repo-local override is honored, not just the global config). Not
// the plan's literal zero-argument signature: it structurally needs to know
// which directory's config to query.
func gitConfigUserName(dir string) (string, bool) {
	out, err := runGit(dir, "config", "user.name")
	if err != nil || out == "" {
		return "", false
	}
	return out, true
}

// fsBirthOrModTime returns path's modification time. Real OS-level birth
// time (creation time) isn't exposed portably by Go's standard library
// without per-platform build tags (darwin/windows expose it via extended
// stat fields, Linux only via statx, not through os.FileInfo at all) --
// deliberately not implemented here, mtime is used unconditionally. Worth
// revisiting only if this proves too imprecise in practice.
func fsBirthOrModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// firstH1TitlePattern is cmd/kb's own copy of documents.go's unexported
// markdownH1Pattern, the same "own copy, not a shared internal" precedent
// firstH1HeadingPattern (documenttag.go) already sets.
var firstH1TitlePattern = regexp.MustCompile(`^#\s+(.*)$`)

// proposeTitle proposes a title from body's own first H1 heading, the same
// fallback DR-0027 already uses at ingest time (firstH1Heading,
// documents.go), called here for a command a human runs deliberately
// instead.
func proposeTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if m := firstH1TitlePattern.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// signal is one candidate value with its provenance label, for author's
// multi-signal report (design decision 4: show every signal when they
// disagree, not just the winner).
type signal struct {
	Source string
	Value  string
}

// proposeAuthor proposes an author from a layered signal: a byline in the
// document's own prose wins when present, falling back to git's
// earliest-commit author, then git config user.name. signals always
// carries every signal that actually fired, in that priority order, so a
// caller can show a disagreement rather than hide it behind one guess.
func proposeAuthor(path, body string) (winner string, signals []signal) {
	if byline, ok := detectByline(body); ok {
		signals = append(signals, signal{Source: "byline", Value: byline})
	}
	if author, _, ok := gitFirstCommit(path); ok {
		signals = append(signals, signal{Source: "git", Value: author})
	}
	if name, ok := gitConfigUserName(filepath.Dir(path)); ok {
		signals = append(signals, signal{Source: "git config", Value: name})
	}
	if len(signals) == 0 {
		return "", nil
	}
	return signals[0].Value, signals
}

// proposeDateCreated proposes dateCreated from git's first commit touching
// path, falling back to filesystem birth/mod time.
func proposeDateCreated(path string) (value, source string) {
	if _, date, ok := gitFirstCommit(path); ok {
		return date, "git"
	}
	if t, err := fsBirthOrModTime(path); err == nil {
		return t.Format(time.RFC3339), "filesystem"
	}
	return "", ""
}

// proposeDateModified proposes dateModified from git's most recent commit
// touching path, falling back to filesystem mtime. Always computed,
// regardless of any existing value -- the caller (FM5) decides whether to
// write it, per design decision 4's "not absent-only" exception for this
// one field.
func proposeDateModified(path string) (value, source string) {
	if date, ok := gitLastCommit(path); ok {
		return date, "git"
	}
	if t, err := fsBirthOrModTime(path); err == nil {
		return t.Format(time.RFC3339), "filesystem"
	}
	return "", ""
}

// bylineLeadIns are the lead-in phrases detectByline recognizes, in the
// document's own prose immediately after its first H1 (design decision 4).
var bylineLeadIns = []string{"By ", "Author: ", "Written by "}

// detectByline scans body for a byline in the small window immediately
// after the first H1 heading, up to the next blank line or heading: one of
// bylineLeadIns, case-sensitively (a deliberately narrow, high-confidence
// signal -- the author's own chosen wording, not a guess).
func detectByline(body string) (author string, ok bool) {
	lines := strings.Split(body, "\n")
	afterH1 := false
	for _, line := range lines {
		if !afterH1 {
			// firstH1HeadingPattern (documenttag.go) is already cmd/kb's own
			// copy of documents.go's unexported markdownH1Pattern -- reused
			// here rather than a second copy.
			if firstH1HeadingPattern.MatchString(line) {
				afterH1 = true
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// sectionHeadingPattern (documentfuzzytag.go) is cmd/kb's own copy
		// of documents.go's markdownHeadingPattern.
		if sectionHeadingPattern.MatchString(trimmed) {
			return "", false
		}
		for _, lead := range bylineLeadIns {
			if strings.HasPrefix(trimmed, lead) {
				return strings.TrimSpace(strings.TrimPrefix(trimmed, lead)), true
			}
		}
		return "", false
	}
	return "", false
}

// knownKeywordProposals returns known concept names mentioned in text
// (eligibleConcepts's own >1-occurrence, code-span-excluded threshold, the
// same one tag/fuzzy-tag apply) that aren't already in currentKeywords
// (case-insensitive), so only genuinely new matches are proposed.
func knownKeywordProposals(kb *knowledge.KnowledgeBase, text string, currentKeywords []string) ([]string, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	var allNames []string
	for _, c := range concepts {
		allNames = append(allNames, c.Name)
	}
	eligible, err := eligibleConcepts(kb, text, allNames)
	if err != nil {
		return nil, err
	}
	current := map[string]bool{}
	for _, k := range currentKeywords {
		current[strings.ToLower(k)] = true
	}
	var out []string
	for _, name := range eligible {
		if !current[strings.ToLower(name)] {
			out = append(out, name)
		}
	}
	return out, nil
}

// scoreDocumentCandidateTerms is scoreCandidateTerms (concept.go) adapted to
// a single target document's scope: occurrences are counted from targetText
// alone, but idf is computed from comparisonItems, which the caller has
// already excluded the target document from (comparisonScope). This fixes
// the idf-degeneracy bug a naive same-scope calculation would have: with
// the target counted as its own single comparison item, every term's df
// would trivially equal the corpus size, collapsing idf to 0 for
// everything. The smoothing form log((len(comparisonItems)+1) / (df+1))
// lets a term absent from every comparison item still score, rather than
// dividing by zero -- a sibling function to scoreCandidateTerms, not a
// modification of it, so that function's existing single-corpus contract
// and tests stay untouched.
func scoreDocumentCandidateTerms(targetText string, comparisonItems []string, known map[string]bool) []candidateTerm {
	occurrences := map[string]int{}
	for _, tok := range candidateTermPattern.FindAllString(strings.ToLower(stripCodeSpans(targetText)), -1) {
		if known[tok] || suggestStopwords[tok] || recordIDShapedPattern.MatchString(tok) {
			continue
		}
		occurrences[tok]++
	}
	if len(occurrences) == 0 {
		return nil
	}

	df := map[string]int{}
	for _, item := range comparisonItems {
		seen := map[string]bool{}
		for _, tok := range candidateTermPattern.FindAllString(strings.ToLower(stripCodeSpans(item)), -1) {
			if _, ok := occurrences[tok]; !ok {
				continue
			}
			if !seen[tok] {
				seen[tok] = true
				df[tok]++
			}
		}
	}

	n := float64(len(comparisonItems) + 1)
	var out []candidateTerm
	for term, occ := range occurrences {
		if occ < 2 {
			continue
		}
		idf := math.Log(n / float64(df[term]+1))
		if idf <= 0 {
			continue
		}
		out = append(out, candidateTerm{Term: term, Occurrences: occ, Items: df[term], Score: float64(occ) * idf})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Term < out[j].Term
	})
	return out
}

// projectNameByID returns the name of the project with the given id, or ""
// if id is 0 or no such project exists.
func projectNameByID(kb *knowledge.KnowledgeBase, id int64) (string, error) {
	if id == 0 {
		return "", nil
	}
	projects, err := kb.Projects()
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if p.ID == id {
			return p.Name, nil
		}
	}
	return "", nil
}

// comparisonScope gathers the text items scoreDocumentCandidateTerms
// compares a target document's candidate terms against: the project's other
// records and document sections when doc belongs to one, or the whole
// corpus otherwise -- always excluding doc itself, mirroring
// cmdConceptSuggest's own gather loop (concept.go) minus the target. doc
// may be nil (the target file hasn't been ingested yet).
func comparisonScope(kb *knowledge.KnowledgeBase, doc *knowledge.Document) ([]string, error) {
	var projectID int64
	var projectName string
	if doc != nil && doc.ProjectID != 0 {
		projectID = doc.ProjectID
		name, err := projectNameByID(kb, projectID)
		if err != nil {
			return nil, err
		}
		projectName = name
	}

	records, err := kb.ListRecords(knowledge.RecordFilter{Project: projectName})
	if err != nil {
		return nil, err
	}
	var items []string
	for _, r := range records {
		items = append(items, r.Body)
	}

	docs, err := kb.Documents(projectID)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		if doc != nil && d.ID == doc.ID {
			continue
		}
		sections, err := kb.DocumentSections(d.ID)
		if err != nil {
			return nil, err
		}
		for _, s := range sections {
			if s.Level == "section" {
				items = append(items, s.Body)
			}
		}
	}
	return items, nil
}

// frontmatterNode locates raw's leading YAML frontmatter block (reusing
// frontmatterBlockPattern, documenttag.go's own positional block detector,
// rather than splitFrontmatter's string-based split, which discards byte
// offsets a surgical splice needs) and parses it into a *yaml.Node mapping
// for in-place editing. bodyOffset is the byte offset into raw where
// everything after the block begins -- the caller never touches raw before
// that point except to replace the block itself. hadBlock=false when no
// block exists; node is then a fresh, empty mapping the caller can still
// append fields to (writeDocumentFile prepends a new block in that case).
func frontmatterNode(raw []byte) (node *yaml.Node, bodyOffset int, hadBlock bool, err error) {
	loc := frontmatterBlockPattern.FindIndex(raw)
	if loc == nil {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, 0, false, nil
	}
	inner := raw[loc[0]+4 : loc[1]-4] // strip the "---\n" prefix and "\n---" suffix
	var doc yaml.Node
	if err := yaml.Unmarshal(inner, &doc); err != nil {
		return nil, 0, false, fmt.Errorf("knowledge: parse frontmatter: %w", err)
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, loc[1], true, nil
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, 0, false, fmt.Errorf("knowledge: frontmatter is not a mapping")
	}
	return doc.Content[0], loc[1], true, nil
}

// setMappingField finds key in node's top-level mapping and overwrites its
// value in place, or appends a new key/value pair if key isn't present yet
// -- every other key, value, and comment in node is left untouched. value
// is a string (a scalar field) or []string (keywords).
func setMappingField(node *yaml.Node, key string, value any) error {
	valueNode, err := frontmatterValueNode(value)
	if err != nil {
		return err
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = valueNode
			return nil
		}
	}
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		valueNode,
	)
	return nil
}

// frontmatterValueNode builds a *yaml.Node for a scalar string or a
// []string sequence. Tag is always explicitly "!!str" on scalars so a
// date-shaped value (dateCreated, dateModified) round-trips as a string,
// not an implicitly-resolved YAML timestamp.
func frontmatterValueNode(value any) (*yaml.Node, error) {
	switch v := value.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}, nil
	case []string:
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range v {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item})
		}
		return seq, nil
	default:
		return nil, fmt.Errorf("knowledge: unsupported frontmatter value type %T", value)
	}
}

// renderFrontmatter re-encodes node (the mapping node itself, not a
// document wrapper) back to YAML text, with no surrounding "---" delimiters
// -- writeDocumentFile adds those.
func renderFrontmatter(node *yaml.Node) (string, error) {
	out, err := yaml.Marshal(node)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// writeDocumentFile splices newBlock (frontmatter YAML text, no
// delimiters) into raw in place of the original frontmatter block -- or,
// when !hadBlock, prepends a fresh block -- and writes the result
// atomically: a temp file in path's own directory, then os.Rename over the
// original, so a crash mid-write never leaves a half-written file.
// bodyOffset is frontmatterNode's own return value, ignored when !hadBlock.
func writeDocumentFile(path string, raw []byte, hadBlock bool, bodyOffset int, newBlock string) error {
	var next []byte
	if hadBlock {
		next = append([]byte("---\n"+newBlock+"\n---"), raw[bodyOffset:]...)
	} else {
		next = append([]byte("---\n"+newBlock+"\n---\n\n"), raw...)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".frontmatter-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(next); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// mappingStringValue returns the value of key in node's top-level mapping,
// or ("", false) if absent. Reads directly from the parsed yaml.Node rather
// than through documents.go's unexported documentFrontmatter struct, which
// cmd/kb cannot reference across the package boundary anyway.
func mappingStringValue(node *yaml.Node, key string) (string, bool) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1].Value, true
		}
	}
	return "", false
}

// mappingStringListValue returns the list value of key in node's top-level
// mapping, or nil if absent or not a sequence.
func mappingStringListValue(node *yaml.Node, key string) []string {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value != key {
			continue
		}
		var out []string
		for _, item := range node.Content[i+1].Content {
			out = append(out, item.Value)
		}
		return out
	}
	return nil
}

// stringSliceFlag implements flag.Value for a repeatable string flag
// (--set FIELD=VALUE, one occurrence per pair) -- no precedent for this in
// cmd/kb yet, so this is knowledge's first repeatable-flag type.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// frontmatterScalarFields are the field names --accept and --set recognize.
// keywords is handled separately by --accept-keywords, since it's list-
// valued and diffed against two candidate sets, not a single accept/reject.
var frontmatterScalarFields = map[string]bool{
	"title": true, "author": true, "dateCreated": true, "dateModified": true,
}

// frontmatterFieldReport is one scalar field's outcome in
// cmdDocumentFrontmatter's report: its current value (if any, meaning
// nothing was proposed -- decision 6's absent-only rule), its proposed
// value (if absent), author's multi-signal breakdown when they disagree,
// and whether this run selected it for writing (via --accept or --set;
// true even under --dry-run, which only gates the actual file write, not
// this report).
type frontmatterFieldReport struct {
	Field    string   `json:"field"`
	Current  string   `json:"current,omitempty"`
	Proposed string   `json:"proposed,omitempty"`
	Signals  []signal `json:"signals,omitempty"`
	Accepted bool     `json:"accepted"`
}

// frontmatterKeywordReport is keywords' outcome: the current list, the two
// candidate sets (design decision 4's known-concept and new-candidate
// proposals), and which names this run accepted.
type frontmatterKeywordReport struct {
	Current  []string        `json:"current"`
	Known    []string        `json:"known_proposals"`
	New      []candidateTerm `json:"new_candidate_proposals"`
	Accepted []string        `json:"accepted"`
}

// documentFrontmatterResult is cmdDocumentFrontmatter's whole report for one
// file.
type documentFrontmatterResult struct {
	Path     string                   `json:"path"`
	DryRun   bool                     `json:"dry_run"`
	Fields   []frontmatterFieldReport `json:"fields"`
	Keywords frontmatterKeywordReport `json:"keywords"`
	Written  bool                     `json:"written"`
}

/** cmdDocumentFrontmatter implements `kb document frontmatter PATH [--accept
 * FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]`
 * (v0.0.11 item 2, frontmatter-generator-plan.md): propose-then-accept
 * title/author/dateCreated/dateModified/keywords for one document, deriving
 * mechanical signals from the document's own prose and git/filesystem
 * provenance -- no model call, no new dependency beyond git itself.
 *
 * title/author/dateCreated are absent-only: never overwritten once present.
 * dateModified is the one exception, always recomputed and rewritten when
 * accepted, since a stale value is worse than a missing one. keywords is a
 * diff against two candidate sets: known concepts already mentioned in the
 * text (plain write on accept) and new candidates distinctive to this
 * document (kb.AddConcept'd before being written, mirroring `kb document
 * tag --concept`'s checked-before-any-write discipline). --set FIELD=VALUE
 * bypasses signal detection and the absent-only rule entirely -- a human's
 * explicit assertion, not a proposal.
 *
 * A bare invocation (no --accept/--accept-keywords/--set) is read-only by
 * construction: nothing is selected, so nothing is written. Editing is
 * surgical, via frontmatterNode/setMappingField/writeDocumentFile -- the
 * document body is never read into anything but raw bytes either side of
 * the frontmatter block.
 *
 * Parameters:
 *   kb      (*knowledge.KnowledgeBase) — the open knowledge base.
 *   jsonOut (bool)                     — emit results as JSON.
 *   args    ([]string)                 — PATH, --accept, --accept-keywords,
 *                                        --set, --dry-run.
 *   out     (io.Writer)                — where results are written.
 *
 * Returns:
 *   error — on a usage error, an unknown field name, or a failed read/write.
 *
 * Example:
 *   err := cmdDocumentFrontmatter(kb, false, []string{"a.md", "--accept", "title"}, os.Stdout)
 */
func cmdDocumentFrontmatter(kb *knowledge.KnowledgeBase, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("document frontmatter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	acceptList := fs.String("accept", "", "comma-separated fields to accept: title,author,dateCreated,dateModified")
	acceptKeywordsList := fs.String("accept-keywords", "", "comma-separated keyword names to accept")
	var setFlags stringSliceFlag
	fs.Var(&setFlags, "set", "FIELD=VALUE, repeatable, bypasses signal detection and the absent-only rule")
	dryRun := fs.Bool("dry-run", false, "preview the write without applying it")
	if len(args) == 0 {
		return fmt.Errorf("usage: document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]")
	}
	// PATH comes first, flags after -- Go's flag package stops parsing at
	// the first non-flag token, so PATH must be peeled off before Parse
	// ever sees it, not left for fs.Args() to return afterward.
	path := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: document frontmatter PATH [--accept FIELD,...] [--accept-keywords NAME,...] [--set FIELD=VALUE] [--dry-run]")
	}

	setValues := map[string]string{}
	for _, kv := range setFlags {
		i := strings.Index(kv, "=")
		if i <= 0 {
			return fmt.Errorf("invalid --set value %q, want FIELD=VALUE", kv)
		}
		field, value := kv[:i], kv[i+1:]
		if !frontmatterScalarFields[field] {
			return fmt.Errorf("unknown --set field %q", field)
		}
		setValues[field] = value
	}

	acceptedFields := map[string]bool{}
	if *acceptList != "" {
		for _, f := range strings.Split(*acceptList, ",") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			if !frontmatterScalarFields[f] {
				return fmt.Errorf("unknown --accept field %q", f)
			}
			acceptedFields[f] = true
		}
	}

	var acceptKeywordNames []string
	if *acceptKeywordsList != "" {
		for _, n := range strings.Split(*acceptKeywordsList, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				acceptKeywordNames = append(acceptKeywordNames, n)
			}
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	node, bodyOffset, hadBlock, err := frontmatterNode(raw)
	if err != nil {
		return err
	}

	body := string(raw)
	if hadBlock {
		body = string(raw[bodyOffset:])
	}

	doc, err := kb.DocumentByPath(path)
	if err != nil {
		return err
	}

	titleReport := frontmatterFieldReport{Field: "title"}
	if v, ok := mappingStringValue(node, "title"); ok {
		titleReport.Current = v
	} else {
		titleReport.Proposed = proposeTitle(body)
	}

	authorReport := frontmatterFieldReport{Field: "author"}
	if v, ok := mappingStringValue(node, "author"); ok {
		authorReport.Current = v
	} else {
		winner, signals := proposeAuthor(path, body)
		authorReport.Proposed = winner
		authorReport.Signals = signals
	}

	dateCreatedReport := frontmatterFieldReport{Field: "dateCreated"}
	if v, ok := mappingStringValue(node, "dateCreated"); ok {
		dateCreatedReport.Current = v
	} else {
		value, _ := proposeDateCreated(path)
		dateCreatedReport.Proposed = value
	}

	dateModifiedReport := frontmatterFieldReport{Field: "dateModified"}
	if v, ok := mappingStringValue(node, "dateModified"); ok {
		dateModifiedReport.Current = v
	}
	dmValue, _ := proposeDateModified(path)
	dateModifiedReport.Proposed = dmValue

	fieldReports := map[string]*frontmatterFieldReport{
		"title": &titleReport, "author": &authorReport,
		"dateCreated": &dateCreatedReport, "dateModified": &dateModifiedReport,
	}

	currentKeywords := mappingStringListValue(node, "keywords")
	concepts, err := kb.Concepts()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, c := range concepts {
		known[strings.ToLower(c.Name)] = true
	}
	knownProposals, err := knownKeywordProposals(kb, body, currentKeywords)
	if err != nil {
		return err
	}
	items, err := comparisonScope(kb, doc)
	if err != nil {
		return err
	}
	newCandidates := scoreDocumentCandidateTerms(body, items, known)
	keywordReport := frontmatterKeywordReport{Current: currentKeywords, Known: knownProposals, New: newCandidates}

	changed := false
	applyField := func(name, value string) {
		if value == "" {
			return
		}
		if err := setMappingField(node, name, value); err != nil {
			return
		}
		fieldReports[name].Accepted = true
		changed = true
	}
	for field, value := range setValues {
		applyField(field, value)
	}
	for field := range acceptedFields {
		if _, isSet := setValues[field]; isSet {
			continue
		}
		applyField(field, fieldReports[field].Proposed)
	}

	var acceptedKeywords []string
	for _, name := range acceptKeywordNames {
		if !known[strings.ToLower(name)] {
			if _, err := kb.AddConcept(name, ""); err != nil {
				return err
			}
			known[strings.ToLower(name)] = true
		}
		acceptedKeywords = append(acceptedKeywords, name)
	}
	if len(acceptedKeywords) > 0 {
		merged := append(append([]string(nil), currentKeywords...), acceptedKeywords...)
		if err := setMappingField(node, "keywords", merged); err != nil {
			return err
		}
		keywordReport.Accepted = acceptedKeywords
		changed = true
	}

	result := documentFrontmatterResult{
		Path:     path,
		DryRun:   *dryRun,
		Fields:   []frontmatterFieldReport{titleReport, authorReport, dateCreatedReport, dateModifiedReport},
		Keywords: keywordReport,
		Written:  changed && !*dryRun,
	}

	if changed && !*dryRun {
		rendered, err := renderFrontmatter(node)
		if err != nil {
			return err
		}
		if err := writeDocumentFile(path, raw, hadBlock, bodyOffset, rendered); err != nil {
			return err
		}
	}

	return reportDocumentFrontmatter(jsonOut, result, out)
}

// reportDocumentFrontmatter renders cmdDocumentFrontmatter's outcome, plain
// text or --json.
func reportDocumentFrontmatter(jsonOut bool, result documentFrontmatterResult, out io.Writer) error {
	if jsonOut {
		return printJSON(out, result)
	}
	for _, f := range result.Fields {
		switch {
		case f.Current != "" && f.Proposed != "":
			// dateModified only: the one field that's always recomputed
			// even when a value already exists (design decision 4), so
			// both need to be shown -- current alone would silently hide
			// the fresh proposal a human needs to decide whether to accept.
			fmt.Fprintf(out, "%s: current: %s. proposing %q.\n", f.Field, f.Current, f.Proposed)
		case f.Current != "":
			fmt.Fprintf(out, "%s: already set: %s\n", f.Field, f.Current)
		case f.Proposed != "":
			if len(f.Signals) > 1 {
				var parts []string
				for _, s := range f.Signals {
					parts = append(parts, fmt.Sprintf("%s: %q", s.Source, s.Value))
				}
				fmt.Fprintf(out, "%s: missing. signals: %s. proposing %q.\n", f.Field, strings.Join(parts, "; "), f.Proposed)
			} else {
				fmt.Fprintf(out, "%s: missing. proposing %q.\n", f.Field, f.Proposed)
			}
		default:
			fmt.Fprintf(out, "%s: no signal available\n", f.Field)
		}
		if f.Accepted {
			fmt.Fprintf(out, "  accepted: %s\n", f.Proposed)
		}
	}
	kw := result.Keywords
	fmt.Fprintf(out, "keywords: current: %s\n", strings.Join(kw.Current, ", "))
	if len(kw.Known) > 0 {
		fmt.Fprintf(out, "  known-concept proposals: %s\n", strings.Join(kw.Known, ", "))
	}
	for _, c := range kw.New {
		fmt.Fprintf(out, "  new candidate: %s (score %.2f)\n", c.Term, c.Score)
	}
	if len(kw.Accepted) > 0 {
		fmt.Fprintf(out, "  accepted: %s\n", strings.Join(kw.Accepted, ", "))
	}
	if result.Written {
		fmt.Fprintln(out, "written.")
	}
	return nil
}
