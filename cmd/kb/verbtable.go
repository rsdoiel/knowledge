package main

/** writeClass says what a verb does to the knowledge base, and so how a
 * confirming interface treats it (knowledge DR-0067). The classes come from the
 * design brief for the TUI write flows; the TUI, `kb verbs` and the completion
 * and help code all read them from the verb table, never from a second list.
 */
type writeClass int

const (
	classNone      writeClass = iota // not set; a table row must name one
	classRead                        // reads only
	classAdditive                    // adds knowledge
	classChanging                    // changes something that exists
	classRemoving                    // removes knowledge or the links others rely on
	classDirect                      // writes files or an index, run without confirming
	classPlanApply                   // has a --dry-run plan, then applies
	classGuided                      // a multi-step workflow with a person in it
	classCLIOnly                     // not offered in the TUI
)

/** String returns the class's name as `kb verbs` prints it.
 *
 * Returns:
 *   string — "read", "additive", "changing", "removing", "direct", "plan_apply", "guided" or "cli_only".
 *
 * Example:
 *   classChanging.String() // "changing"
 */
func (c writeClass) String() string {
	switch c {
	case classRead:
		return "read"
	case classAdditive:
		return "additive"
	case classChanging:
		return "changing"
	case classRemoving:
		return "removing"
	case classDirect:
		return "direct"
	case classPlanApply:
		return "plan_apply"
	case classGuided:
		return "guided"
	case classCLIOnly:
		return "cli_only"
	}
	return ""
}

/** subverbSpec is one subverb of a verb in the table: its name, its class, and
 * what completion and the deprecation notes need to know about it.
 *
 * Fields:
 *   Name       (string)       — the subverb as typed.
 *   Class      (writeClass)   — what it does to the knowledge base.
 *   ProjectArg (bool)         — its first argument is a project name, which completion fills in.
 *   ScopeForm  (string)       — "list" or "ref" when --project and --workspace are deprecated aliases for it ("" when the flag is the way to name the scope).
 *   Subverbs   ([]subverbSpec) — its own subverbs (kb document review has list and promote).
 */
type subverbSpec struct {
	Name       string
	Class      writeClass
	ProjectArg bool
	ScopeForm  string
	Subverbs   []subverbSpec
}

/** verbSpec is one verb in the table: the single description of the command
 * language that completion, `kb verbs`, subverb help and the TUI menu read.
 *
 * Fields:
 *   Name     (string)       — the verb as typed.
 *   Summary  (string)       — one line saying what it does.
 *   Class    (writeClass)   — for a verb with no subverbs; ignored when Subverbs is set.
 *   Subverbs ([]subverbSpec) — in the order completion and help list them.
 *   Flags    ([]string)     — the flags the verb accepts, subverbs included, in completion order.
 */
type verbSpec struct {
	Name     string
	Summary  string
	Class    writeClass
	Subverbs []subverbSpec
	Flags    []string
}

// sub is shorthand for a subverb row in the table below.
func sub(name string, class writeClass) subverbSpec { return subverbSpec{Name: name, Class: class} }

// verbTable is the one description of kb's verbs (DR-0059). The order of the
// verbs, of each verb's subverbs and of its flags is the order completion and
// help list them, and completion_golden_test.go holds that order still.
var verbTable = []verbSpec{
	{Name: "check-db", Summary: "compare the database with its JSON-L dump", Class: classRead,
		Flags: []string{"--jsonl"}},
	{Name: "completion", Summary: "write or install a shell completion script", Class: classCLIOnly,
		Flags: []string{"--install"}},
	{Name: "concept", Summary: "manage concepts", Subverbs: []subverbSpec{
		sub("add", classAdditive), sub("list", classRead), sub("show", classRead), sub("recall", classRead),
		sub("rename", classChanging), sub("delete", classRemoving), sub("suggest", classRead),
	}, Flags: []string{"--identifier-type", "--identifier-value", "--project", "--limit", "--force", "--dry-run"}},
	{Name: "document", Summary: "ingest and review narrative documents at graduated abstraction levels", Subverbs: []subverbSpec{
		sub("ingest", classGuided), sub("draft", classGuided),
		{Name: "review", Subverbs: []subverbSpec{sub("list", classRead), sub("promote", classGuided)}},
		sub("list", classRead), sub("show", classRead), sub("tag", classPlanApply), sub("fuzzy-tag", classPlanApply),
		sub("frontmatter", classPlanApply), sub("delete", classRemoving),
	}, Flags: []string{"--project", "--concept", "--dry-run", "--accept", "--accept-keywords", "--set", "--by",
		"--confidence", "--status", "--title", "--format"}},
	{Name: "export", Summary: "write a portable JSON-L snapshot of the database", Class: classCLIOnly,
		Flags: []string{"--project", "--out"}},
	{Name: "format", Summary: "a fully assembled Markdown view of one project or every project", Class: classRead,
		Flags: []string{"--project"}},
	{Name: "import", Summary: "apply a JSON-L snapshot (from export) to the database", Class: classCLIOnly,
		Flags: []string{"--in"}},
	{Name: "index", Summary: "generate a decisions/index.md from a directory of records", Class: classDirect,
		Flags: []string{"--stdout", "--check", "--all"}},
	{Name: "ingest", Summary: "index a tree of decision records into the knowledge base", Class: classPlanApply,
		Flags: []string{"--root", "--dry-run"}},
	{Name: "init", Summary: "create a new, empty workspace", Class: classCLIOnly},
	{Name: "link", Summary: "link projects or observations to concepts", Subverbs: []subverbSpec{
		sub("project", classAdditive), sub("observation", classAdditive),
	}},
	{Name: "merge", Summary: "reconcile two knowledge.db files that drifted independently", Class: classCLIOnly,
		Flags: []string{"--a", "--b", "--out", "--force"}},
	{Name: "observation", Summary: "manage observations", Subverbs: []subverbSpec{
		sub("add", classAdditive), sub("list", classRead), sub("show", classRead), sub("update", classChanging),
		sub("sources", classRead), sub("delete", classRemoving),
	}, Flags: []string{"--project", "--source-doi"}},
	{Name: "project", Summary: "manage projects", Subverbs: []subverbSpec{
		sub("add", classAdditive), sub("list", classRead),
		{Name: "show", Class: classRead, ProjectArg: true},
		{Name: "concepts", Class: classRead, ProjectArg: true},
		{Name: "set-status", Class: classChanging, ProjectArg: true},
		{Name: "set-description", Class: classChanging, ProjectArg: true},
		{Name: "rename", Class: classChanging, ProjectArg: true},
		{Name: "delete", Class: classRemoving, ProjectArg: true},
	}, Flags: []string{"--status", "--root", "--dry-run"}},
	{Name: "record", Summary: "read and maintain decision records", Subverbs: []subverbSpec{
		{Name: "list", Class: classRead, ScopeForm: "list"},
		{Name: "pending", Class: classRead, ScopeForm: "list"},
		{Name: "show", Class: classRead, ScopeForm: "ref"},
		{Name: "set-status", Class: classChanging, ScopeForm: "ref"},
		{Name: "supersede", Class: classChanging, ScopeForm: "ref"},
		sub("fmt", classDirect), sub("new", classAdditive),
		{Name: "concepts", Class: classRead, ScopeForm: "ref"},
		{Name: "delete", Class: classRemoving, ScopeForm: "ref"},
		sub("fuzzy-tag", classPlanApply),
	}, Flags: []string{"--project", "--status", "--kind", "--trigger", "--initiative", "--since", "--root", "--dir",
		"--title", "--concept", "--workspace", "--partial", "--dry-run", "--write", "--all"}},
	{Name: "search", Summary: "full-text search across observations, projects, concepts and records", Class: classRead,
		Flags: []string{"--project"}},
	{Name: "source", Summary: "manage cited sources and retraction checking", Subverbs: []subverbSpec{
		sub("add", classAdditive), sub("list", classRead), sub("show", classRead), sub("remove", classRemoving),
		sub("retract", classChanging), sub("link", classAdditive), sub("check-retractions", classRead),
	}, Flags: []string{"--doi", "--url", "--authors", "--published", "--publisher", "--rights", "--version", "--relationship"}},
	{Name: "summary", Summary: "a formatted overview of every project and its most recent observations", Class: classRead,
		Flags: []string{"--project"}},
	{Name: "verbs", Summary: "print the verb table: every verb, subverb, flag and write class", Class: classCLIOnly},
	{Name: "unlink", Summary: "remove a link between a project or observation and a concept, or an observation and a source", Class: classRemoving},
}

/** subverbMap returns, for each verb that takes a subverb, the subverbs in table
 * order, keyed by the verb; a key of two words ("document review") lists a
 * subverb's own subverbs. Completion and the help-flag handling read it.
 *
 * Returns:
 *   map[string][]string — verb or "verb subverb" to its subverbs.
 *
 * Example:
 *   subverbMap()["record"] // [list pending show set-status ...]
 */
func subverbMap() map[string][]string {
	m := map[string][]string{}
	var add func(key string, subs []subverbSpec)
	add = func(key string, subs []subverbSpec) {
		if len(subs) == 0 {
			return
		}
		names := make([]string, len(subs))
		for i, s := range subs {
			names[i] = s.Name
		}
		m[key] = names
		for _, s := range subs {
			add(key+" "+s.Name, s.Subverbs)
		}
	}
	for _, v := range verbTable {
		add(v.Name, v.Subverbs)
	}
	return m
}

/** projectArgMap returns, for each verb, the subverbs whose first argument is a
 * project name, which completion fills in from the database.
 *
 * Returns:
 *   map[string][]string — verb to those subverbs; verbs with none are absent.
 *
 * Example:
 *   projectArgMap()["project"] // [show concepts set-status ...]
 */
func projectArgMap() map[string][]string {
	m := map[string][]string{}
	for _, v := range verbTable {
		for _, s := range v.Subverbs {
			if s.ProjectArg {
				m[v.Name] = append(m[v.Name], s.Name)
			}
		}
	}
	return m
}

/** flagMap returns each verb's flags in table order; verbs with none are absent.
 *
 * Returns:
 *   map[string][]string — verb to its flags.
 *
 * Example:
 *   flagMap()["ingest"] // [--root --dry-run]
 */
func flagMap() map[string][]string {
	m := map[string][]string{}
	for _, v := range verbTable {
		if len(v.Flags) > 0 {
			m[v.Name] = v.Flags
		}
	}
	return m
}

/** verbsTakingSubverbs returns the set of verbs whose first argument names a
 * subverb.
 *
 * Returns:
 *   map[string]bool — verb to true.
 *
 * Example:
 *   verbsTakingSubverbs()["record"] // true
 */
func verbsTakingSubverbs() map[string]bool {
	m := map[string]bool{}
	for _, v := range verbTable {
		if len(v.Subverbs) > 0 {
			m[v.Name] = true
		}
	}
	return m
}

/** recordScopeForms returns, for the record subverbs on which --project and
 * --workspace are deprecated aliases, the form that replaces them: "list" for a
 * scope argument, "ref" for a qualified reference.
 *
 * Returns:
 *   map[string]string — record subverb to its replacement form.
 *
 * Example:
 *   recordScopeForms()["set-status"] // "ref"
 */
func recordScopeForms() map[string]string {
	m := map[string]string{}
	for _, v := range verbTable {
		if v.Name != "record" {
			continue
		}
		for _, s := range v.Subverbs {
			if s.ScopeForm != "" {
				m[s.Name] = s.ScopeForm
			}
		}
	}
	return m
}
