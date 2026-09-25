package knowledge

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Moved from cmd/kb/documentfrontmatter.go (library-lift-plan.md L4, DR-0035).
// The proposers, scoring and YAML helpers are a verbatim move, except that
// the three proposers take a Provenance instead of calling git directly.
// ProposeFrontmatter and ApplyFrontmatter are the orchestration that used to
// sit inline in cmdDocumentFrontmatter; the caller reads and writes the file.

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

/** FrontmatterSignal is one candidate value with its provenance label, for
 * the author field's multi-signal report (design decision 4: show every
 * signal when they disagree, not just the winner). It has no JSON tags, so
 * its keys are Source and Value, as `kb document frontmatter --json` has
 * always printed them.
 *
 * Fields:
 *   Source (string) — where the value came from: byline, git, or git config.
 *   Value  (string) — the candidate author.
 *
 * Example:
 *   // {Source: "byline", Value: "Jane Doe"}
 */
type FrontmatterSignal struct {
	Source string
	Value  string
}

// proposeAuthor proposes an author from a layered signal: a byline in the
// document's own prose wins when present, falling back to git's
// earliest-commit author, then git config user.name. signals always
// carries every signal that actually fired, in that priority order, so a
// caller can show a disagreement rather than hide it behind one guess.
func proposeAuthor(prov Provenance, path, body string) (winner string, signals []FrontmatterSignal) {
	if byline, ok := detectByline(body); ok {
		signals = append(signals, FrontmatterSignal{Source: "byline", Value: byline})
	}
	if author, _, ok := prov.FirstCommit(path); ok {
		signals = append(signals, FrontmatterSignal{Source: "git", Value: author})
	}
	if name, ok := prov.ConfigUserName(filepath.Dir(path)); ok {
		signals = append(signals, FrontmatterSignal{Source: "git config", Value: name})
	}
	if len(signals) == 0 {
		return "", nil
	}
	return signals[0].Value, signals
}

// proposeDateCreated proposes dateCreated from git's first commit touching
// path, falling back to filesystem birth/mod time.
func proposeDateCreated(prov Provenance, path string) (value, source string) {
	if _, date, ok := prov.FirstCommit(path); ok {
		return date, "git"
	}
	if t, err := prov.FileTime(path); err == nil {
		return t.Format(time.RFC3339), "filesystem"
	}
	return "", ""
}

// proposeDateModified proposes dateModified from git's most recent commit
// touching path, falling back to filesystem mtime. Always computed,
// regardless of any existing value -- the caller (FM5) decides whether to
// write it, per design decision 4's "not absent-only" exception for this
// one field.
func proposeDateModified(prov Provenance, path string) (value, source string) {
	if date, ok := prov.LastCommit(path); ok {
		return date, "git"
	}
	if t, err := prov.FileTime(path); err == nil {
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
// (EligibleTagConcepts's own >1-occurrence, code-span-excluded threshold, the
// same one tag/fuzzy-tag apply) that aren't already in currentKeywords
// (case-insensitive), so only genuinely new matches are proposed.
func knownKeywordProposals(kb *KnowledgeBase, text string, currentKeywords []string) ([]string, error) {
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, err
	}
	var allNames []string
	for _, c := range concepts {
		allNames = append(allNames, c.Name)
	}
	eligible, err := kb.EligibleTagConcepts(text, allNames)
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
func scoreDocumentCandidateTerms(targetText string, comparisonItems []string, known map[string]bool) []ConceptCandidate {
	occurrences := map[string]int{}
	for _, tok := range CandidateTerms(targetText) {
		if known[tok] {
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
		for _, tok := range CandidateTerms(item) {
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
	var out []ConceptCandidate
	for term, occ := range occurrences {
		if occ < 2 {
			continue
		}
		idf := math.Log(n / float64(df[term]+1))
		if idf <= 0 {
			continue
		}
		out = append(out, ConceptCandidate{Term: term, Occurrences: occ, Items: df[term], Score: float64(occ) * idf})
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
func projectNameByID(kb *KnowledgeBase, id int64) (string, error) {
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
func comparisonScope(kb *KnowledgeBase, doc *Document) ([]string, error) {
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

	records, err := kb.ListRecords(RecordFilter{Project: projectName})
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
		return nil, 0, false, invalidf("knowledge: frontmatter is not a mapping")
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

// absentOnlyCurrent returns key's existing value and true only when it is
// present *and* non-empty. An explicit empty value (e.g. a template's
// `title: ""`) is treated the same as absent -- found reviewing this code
// before release: without this, mappingStringValue's ok=true for an empty
// string permanently blocked the absent-only proposal branch from ever
// running, with no way to fill the field in and no diagnostic explaining
// why.
func absentOnlyCurrent(node *yaml.Node, key string) (string, bool) {
	v, ok := mappingStringValue(node, key)
	if !ok || v == "" {
		return "", false
	}
	return v, true
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

// frontmatterScalarFields are the field names --accept and --set recognize.
// keywords is handled separately by --accept-keywords, since it's list-
// valued and diffed against two candidate sets, not a single accept/reject.
// frontmatterFieldOrder is the canonical order the scalar fields are applied and,
// for a field not yet in the block, appended in.
var frontmatterFieldOrder = []string{"title", "author", "dateCreated", "dateModified"}

var frontmatterScalarFields = map[string]bool{
	"title": true, "author": true, "dateCreated": true, "dateModified": true,
}

/** IsFrontmatterScalarField reports whether name is one of the four scalar
 * fields ApplyFrontmatter can accept or set: title, author, dateCreated,
 * dateModified. keywords is list-valued and handled through
 * FrontmatterAccept.Keywords instead.
 *
 * Parameters:
 *   name (string) — a frontmatter field name.
 *
 * Returns:
 *   bool — true for a recognized scalar field.
 *
 * Example:
 *   knowledge.IsFrontmatterScalarField("title") // true
 */
func IsFrontmatterScalarField(name string) bool { return frontmatterScalarFields[name] }

/** FrontmatterFieldReport is one scalar field's outcome in a
 * FrontmatterResult, and the per-field JSON shape of `kb document
 * frontmatter --json`, so its tags are a compatibility contract.
 *
 * Fields:
 *   Field    (string)               — title, author, dateCreated or dateModified.
 *   Current  (string)               — the existing value; for an absent-only
 *                                     field, its presence means nothing is proposed.
 *   Proposed (string)               — the proposed value, or the value that was
 *                                     just set by an explicit Set.
 *   Signals  ([]FrontmatterSignal)  — author only: every signal that fired.
 *   Accepted (bool)                 — whether this call selected the field.
 *
 * Example:
 *   // {Field: "title", Proposed: "A Real Title", Accepted: true}
 */
type FrontmatterFieldReport struct {
	Field    string              `json:"field"`
	Current  string              `json:"current,omitempty"`
	Proposed string              `json:"proposed,omitempty"`
	Signals  []FrontmatterSignal `json:"signals,omitempty"`
	Accepted bool                `json:"accepted"`
}

/** FrontmatterKeywordReport is the keywords outcome in a FrontmatterResult.
 *
 * Fields:
 *   Current  ([]string)          — the keywords already in the file.
 *   Known    ([]string)          — known concepts mentioned in the text and not
 *                                  yet listed; order is not specified.
 *   New      ([]ConceptCandidate) — terms distinctive to this document, scored
 *                                  against the rest of its project or corpus.
 *   Accepted ([]string)          — the names this call accepted.
 *
 * Example:
 *   // {Current: [], Known: ["chunking"], New: [{Term: "gronkulator", ...}], Accepted: []}
 */
type FrontmatterKeywordReport struct {
	Current  []string           `json:"current"`
	Known    []string           `json:"known_proposals"`
	New      []ConceptCandidate `json:"new_candidate_proposals"`
	Accepted []string           `json:"accepted"`
}

/** FrontmatterResult is one file's whole report, and the JSON shape of `kb
 * document frontmatter --json`.
 *
 * Fields:
 *   Path     (string)                   — the path as passed in.
 *   DryRun   (bool)                     — true when the call was a preview.
 *   Fields   ([]FrontmatterFieldReport) — title, author, dateCreated, dateModified.
 *   Keywords (FrontmatterKeywordReport) — the keyword proposals and acceptances.
 *   Written  (bool)                     — true when ApplyFrontmatter's returned
 *                                         bytes differ from its input and are
 *                                         meant to be written (never on a dry run).
 *
 * Example:
 *   res, _ := kb.ProposeFrontmatter("a.md", raw, nil)
 */
type FrontmatterResult struct {
	Path     string                   `json:"path"`
	DryRun   bool                     `json:"dry_run"`
	Fields   []FrontmatterFieldReport `json:"fields"`
	Keywords FrontmatterKeywordReport `json:"keywords"`
	Written  bool                     `json:"written"`
}

/** FrontmatterAccept says what ApplyFrontmatter should write.
 *
 * Fields:
 *   Fields   ([]string)          — scalar fields whose proposal to accept
 *                                  (see IsFrontmatterScalarField). title, author
 *                                  and dateCreated are absent-only; dateModified
 *                                  is always refreshed.
 *   Keywords ([]string)          — keyword names to add. Each must be a known
 *                                  concept or one of the report's proposed new
 *                                  candidates; a new candidate's concept is
 *                                  created (except on a dry run).
 *   Set      (map[string]string) — explicit values that bypass signal detection
 *                                  and the absent-only rule, including an
 *                                  explicit empty value.
 *
 * Example:
 *   acc := knowledge.FrontmatterAccept{Fields: []string{"title"}, Set: map[string]string{"author": "Jane"}}
 */
type FrontmatterAccept struct {
	Fields   []string
	Keywords []string
	Set      map[string]string
}

// spliceFrontmatter returns raw with newBlock (frontmatter YAML text, no
// delimiters) in place of the original frontmatter block -- or, when
// !hadBlock, with a fresh block prepended. The document body is never
// touched. Moved from cmd/kb's writeDocumentFile, which also did the atomic
// file write; that half stays in cmd/kb, since the caller owns the write.
// bodyOffset is frontmatterNode's own return value, ignored when !hadBlock.
func spliceFrontmatter(raw []byte, hadBlock bool, bodyOffset int, newBlock string) []byte {
	if hadBlock {
		return append([]byte("---\n"+newBlock+"\n---"), raw[bodyOffset:]...)
	}
	return append([]byte("---\n"+newBlock+"\n---\n\n"), raw...)
}

/** ProposeFrontmatter is the read-only half of `kb document frontmatter`: it
 * reports what it would propose for a file's title, author, dateCreated,
 * dateModified and keywords, deriving signals from the file's own prose and
 * from prov. It writes nothing, to the file or the database.
 *
 * Parameters:
 *   path (string)     — the file's path, used for provenance lookups and to
 *                       find the document's project in the knowledge base.
 *   raw  ([]byte)     — the file's current bytes; the caller reads them.
 *   prov (Provenance) — the source of authorship and dates; nil means GitProvenance.
 *
 * Returns:
 *   FrontmatterResult — the report, with nothing accepted.
 *   error             — on unparseable frontmatter or a database failure.
 *
 * Example:
 *   raw, _ := os.ReadFile("a.md")
 *   res, err := kb.ProposeFrontmatter("a.md", raw, nil)
 */
func (kb *KnowledgeBase) ProposeFrontmatter(path string, raw []byte, prov Provenance) (FrontmatterResult, error) {
	_, res, err := kb.frontmatterRun(path, raw, prov, FrontmatterAccept{}, false)
	return res, err
}

/** ApplyFrontmatter proposes, then applies exactly what accept selects, and
 * returns the file's new bytes for the caller to write. The document body is
 * never modified: only the frontmatter block is spliced, via a YAML node edit
 * that leaves every other key, value and comment in it alone.
 *
 * Every accepted keyword is validated before any concept is created or any
 * byte is produced, so one bad name aborts the whole call. On a real run a
 * new-candidate keyword's concept is created in the database before this
 * returns, which is why the caller should write the bytes it gets back; on a
 * dry run the database is never touched and the input bytes come back.
 *
 * Parameters:
 *   path   (string)            — as for ProposeFrontmatter.
 *   raw    ([]byte)            — the file's current bytes; never modified.
 *   prov   (Provenance)        — nil means GitProvenance.
 *   accept (FrontmatterAccept) — what to write.
 *   dryRun (bool)              — preview only: report, but change nothing.
 *
 * Returns:
 *   []byte            — the new file contents when result.Written is true,
 *                       otherwise raw unchanged; nil on error.
 *   FrontmatterResult — the report, with Accepted marks set.
 *   error             — for an unknown field, a rejected keyword, unparseable
 *                       frontmatter, or a database failure.
 *
 * Example:
 *   next, res, err := kb.ApplyFrontmatter("a.md", raw, nil, knowledge.FrontmatterAccept{Fields: []string{"title"}}, false)
 *   if err == nil && res.Written { os.WriteFile("a.md", next, 0o644) }
 */
func (kb *KnowledgeBase) ApplyFrontmatter(path string, raw []byte, prov Provenance, accept FrontmatterAccept, dryRun bool) ([]byte, FrontmatterResult, error) {
	return kb.frontmatterRun(path, raw, prov, accept, dryRun)
}

func (kb *KnowledgeBase) frontmatterRun(path string, raw []byte, prov Provenance, accept FrontmatterAccept, dryRun bool) ([]byte, FrontmatterResult, error) {
	if prov == nil {
		prov = GitProvenance{}
	}
	setValues := map[string]string{}
	setFields := make([]string, 0, len(accept.Set))
	for field := range accept.Set {
		setFields = append(setFields, field)
	}
	sort.Strings(setFields) // so an error names the same field on every run (DR-0037)
	for _, field := range setFields {
		value := accept.Set[field]
		if !frontmatterScalarFields[field] {
			return nil, FrontmatterResult{}, invalidf("unknown frontmatter field %q", field)
		}
		setValues[field] = value
	}
	acceptedFields := map[string]bool{}
	for _, f := range accept.Fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !frontmatterScalarFields[f] {
			return nil, FrontmatterResult{}, invalidf("unknown frontmatter field %q", f)
		}
		acceptedFields[f] = true
	}
	var acceptKeywordNames []string
	for _, n := range accept.Keywords {
		if n = strings.TrimSpace(n); n != "" {
			acceptKeywordNames = append(acceptKeywordNames, n)
		}
	}

	node, bodyOffset, hadBlock, err := frontmatterNode(raw)
	if err != nil {
		return nil, FrontmatterResult{}, err
	}

	body := string(raw)
	if hadBlock {
		body = string(raw[bodyOffset:])
	}

	doc, err := kb.DocumentByPath(path)
	if err != nil {
		return nil, FrontmatterResult{}, err
	}

	titleReport := FrontmatterFieldReport{Field: "title"}
	if v, ok := absentOnlyCurrent(node, "title"); ok {
		titleReport.Current = v
	} else {
		titleReport.Proposed = proposeTitle(body)
	}

	authorReport := FrontmatterFieldReport{Field: "author"}
	if v, ok := absentOnlyCurrent(node, "author"); ok {
		authorReport.Current = v
	} else {
		winner, signals := proposeAuthor(prov, path, body)
		authorReport.Proposed = winner
		authorReport.Signals = signals
	}

	dateCreatedReport := FrontmatterFieldReport{Field: "dateCreated"}
	if v, ok := absentOnlyCurrent(node, "dateCreated"); ok {
		dateCreatedReport.Current = v
	} else {
		value, _ := proposeDateCreated(prov, path)
		dateCreatedReport.Proposed = value
	}

	dateModifiedReport := FrontmatterFieldReport{Field: "dateModified"}
	if v, ok := mappingStringValue(node, "dateModified"); ok {
		dateModifiedReport.Current = v
	}
	dmValue, _ := proposeDateModified(prov, path)
	dateModifiedReport.Proposed = dmValue

	fieldReports := map[string]*FrontmatterFieldReport{
		"title": &titleReport, "author": &authorReport,
		"dateCreated": &dateCreatedReport, "dateModified": &dateModifiedReport,
	}

	currentKeywords := mappingStringListValue(node, "keywords")
	concepts, err := kb.Concepts()
	if err != nil {
		return nil, FrontmatterResult{}, err
	}
	known := map[string]bool{}
	for _, c := range concepts {
		known[strings.ToLower(c.Name)] = true
	}
	knownProposals, err := knownKeywordProposals(kb, body, currentKeywords)
	if err != nil {
		return nil, FrontmatterResult{}, err
	}
	items, err := comparisonScope(kb, doc)
	if err != nil {
		return nil, FrontmatterResult{}, err
	}
	newCandidates := scoreDocumentCandidateTerms(body, items, known)
	keywordReport := FrontmatterKeywordReport{Current: currentKeywords, Known: knownProposals, New: newCandidates}

	// Every --accept-keywords name must be a known concept or one of the report's
	// proposed new candidates, checked before any concept is created or any
	// byte is written -- the same check-before-write rule tag's --concept holds
	// to (DR-0033). Anything else would mint a concept, and write a keyword,
	// for a typo or an arbitrary string (found live 2026-09-23, DR-0036).
	proposedNew := map[string]bool{}
	for _, c := range newCandidates {
		proposedNew[strings.ToLower(c.Term)] = true
	}
	for _, name := range acceptKeywordNames {
		if !known[strings.ToLower(name)] && !proposedNew[strings.ToLower(name)] {
			return nil, FrontmatterResult{}, fmt.Errorf("keyword %q is neither a known concept nor a proposed new candidate; nothing was written (run `kb concept add %s` first to create it)", name, name)
		}
	}

	changed := false
	// applyProposed only fires when there's a genuine (non-empty) proposal
	// to accept -- correct for --accept, which never invents a value out
	// of nothing. applySet writes unconditionally, even an explicit empty
	// string: --set is a human's direct assertion, not a proposal, so
	// `--set title=` clearing a field must actually write, not silently
	// no-op the way it did before this was found in review -- and it
	// updates the report's Proposed so "accepted: %s" reflects what was
	// actually written, not a stale/absent proposal from before --set ran.
	applyProposed := func(name string) {
		r := fieldReports[name]
		if r.Proposed == "" {
			return
		}
		if err := setMappingField(node, name, r.Proposed); err != nil {
			return
		}
		r.Accepted = true
		changed = true
	}
	applySet := func(name, value string) {
		if err := setMappingField(node, name, value); err != nil {
			return
		}
		r := fieldReports[name]
		r.Proposed = value
		r.Accepted = true
		changed = true
	}
	// One pass in canonical field order, so new keys are appended to the block in
	// the same order on every run whatever order they were asked for in
	// (DR-0037). An explicit --set wins over an accepted proposal for the same
	// field, exactly as before.
	for _, field := range frontmatterFieldOrder {
		if value, isSet := setValues[field]; isSet {
			applySet(field, value)
		} else if acceptedFields[field] {
			applyProposed(field)
		}
	}

	var acceptedKeywords []string
	for _, name := range acceptKeywordNames {
		if !known[strings.ToLower(name)] {
			// A dry run must not touch the database: report the keyword as
			// accepted, but only create its concept on a real run.
			if !dryRun {
				if _, err := kb.AddConcept(name, ""); err != nil {
					return nil, FrontmatterResult{}, err
				}
			}
			known[strings.ToLower(name)] = true
		}
		acceptedKeywords = append(acceptedKeywords, name)
	}
	if len(acceptedKeywords) > 0 {
		merged := append(append([]string(nil), currentKeywords...), acceptedKeywords...)
		if err := setMappingField(node, "keywords", merged); err != nil {
			return nil, FrontmatterResult{}, err
		}
		keywordReport.Accepted = acceptedKeywords
		changed = true
	}

	result := FrontmatterResult{
		Path:     path,
		DryRun:   dryRun,
		Fields:   []FrontmatterFieldReport{titleReport, authorReport, dateCreatedReport, dateModifiedReport},
		Keywords: keywordReport,
		Written:  changed && !dryRun,
	}

	next := raw
	if changed && !dryRun {
		rendered, err := renderFrontmatter(node)
		if err != nil {
			return nil, FrontmatterResult{}, err
		}
		next = spliceFrontmatter(raw, hadBlock, bodyOffset, rendered)
	}
	return next, result, nil
}
