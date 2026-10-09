package main

import (
	"sort"
	"strings"
)

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

/** menuEntry says how a verb or subverb appears in the TUI's menus (DR-0066). The
 * verb table carries it, so the menus are derived from the same description as
 * completion and `kb verbs`, and the table is also the roadmap: an entry that is
 * not Built is shown dimmed with the release that brings it.
 *
 * Fields:
 *   Label      (string) — the text in the menu.
 *   Desc       (string) — one line saying what choosing it does.
 *   Order      (int)    — position within its menu, lowest first.
 *   Built      (bool)   — there is a TUI screen for it now. For a verb with subverbs it is derived: true when any leaf is built.
 *   Since      (string) — the release that brings it, shown while it is not built.
 *   Equivalent (string) — the command line that does the same, shown when it is chosen but not built, and after it runs.
 */
type menuEntry struct {
	Label      string
	Desc       string
	Order      int
	Built      bool
	Since      string
	Equivalent string
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
	Menu       *menuEntry // the leaf in its group's menu; nil when the subverb acts on a selected item
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
	Menu     *menuEntry // the entry in the top menu; nil when the verb has none
}

// mi builds a menu entry; it is built when no release is named.
func mi(label, desc string, order int, since, equivalent string) *menuEntry {
	return &menuEntry{Label: label, Desc: desc, Order: order, Built: since == "", Since: since, Equivalent: equivalent}
}

// leaf is a subverb row that is a leaf of its group's menu.
func leaf(name string, class writeClass, m *menuEntry) subverbSpec {
	return subverbSpec{Name: name, Class: class, Menu: m}
}

// sub is shorthand for a subverb row in the table below.
func sub(name string, class writeClass) subverbSpec { return subverbSpec{Name: name, Class: class} }

// verbTable is the one description of kb's verbs (DR-0059). The order of the
// verbs, of each verb's subverbs and of its flags is the order completion and
// help list them, and completion_golden_test.go holds that order still.
var verbTable = []verbSpec{
	{Name: "check-db", Summary: "compare the database with its JSON-L dump", Menu: mi("Check", "compare the database with its JSONL dump", 10, "v0.0.20", "kb check-db"), Class: classRead,
		Flags: []string{"--jsonl"}},
	{Name: "completion", Summary: "write or install a shell completion script", Class: classCLIOnly,
		Flags: []string{"--install"}},
	{Name: "concept", Summary: "manage concepts", Menu: mi("Concepts", "named ideas that span projects", 4, "", "kb concept"), Subverbs: []subverbSpec{
		leaf("add", classAdditive, mi("New concept…", "add a concept", 2, "v0.0.20", "kb concept add NAME DESCRIPTION")), leaf("list", classRead, mi("Browse", "every concept", 1, "v0.0.20", "kb concept list")), sub("show", classRead), leaf("recall", classRead, mi("Recall…", "what is known about the concepts in some text", 4, "v0.0.20", "kb concept recall TEXT")),
		sub("rename", classChanging), sub("delete", classRemoving), leaf("suggest", classRead, mi("Suggest…", "propose new concepts from the corpus", 3, "v0.0.20", "kb concept suggest")),
	}, Flags: []string{"--identifier-type", "--identifier-value", "--project", "--limit", "--force", "--dry-run"}},
	{Name: "document", Summary: "ingest and review narrative documents at graduated abstraction levels", Menu: mi("Documents", "narratives, their summaries and review", 6, "", "kb document"), Subverbs: []subverbSpec{
		leaf("ingest", classGuided, mi("Ingest a document…", "bring a narrative in and summarise it", 2, "v0.0.20", "kb document ingest PATH --project P")), sub("draft", classGuided),
		{Name: "review", Subverbs: []subverbSpec{leaf("list", classRead, mi("Review queue", "summaries waiting for a person", 3, "v0.0.20", "kb document review list")), sub("promote", classGuided)}},
		leaf("list", classRead, mi("Browse", "every document", 1, "v0.0.20", "kb document list")), sub("show", classRead), leaf("tag", classPlanApply, mi("Tag…", "insert explicit concept links", 4, "v0.0.20", "kb document tag --project P")), leaf("fuzzy-tag", classPlanApply, mi("Fuzzy-tag…", "footnote near-miss concept names", 5, "v0.0.20", "kb document fuzzy-tag --project P")),
		leaf("frontmatter", classPlanApply, mi("Frontmatter…", "propose frontmatter for a file", 6, "v0.0.20", "kb document frontmatter PATH")), sub("delete", classRemoving),
	}, Flags: []string{"--project", "--concept", "--dry-run", "--accept", "--accept-keywords", "--set", "--by",
		"--confidence", "--status", "--title", "--format"}},
	{Name: "export", Summary: "write a portable JSON-L snapshot of the database", Class: classCLIOnly,
		Flags: []string{"--project", "--out"}},
	{Name: "format", Summary: "a fully assembled Markdown view of one project or every project", Class: classRead,
		Flags: []string{"--project"}},
	{Name: "import", Summary: "apply a JSON-L snapshot (from export) to the database", Class: classCLIOnly,
		Flags: []string{"--in"}},
	{Name: "index", Summary: "generate a decisions/index.md from a directory of records", Menu: mi("Index", "regenerate a decisions/index.md", 9, "", "kb index PATH"), Class: classDirect,
		Flags: []string{"--stdout", "--check", "--all"}},
	{Name: "ingest", Summary: "index a tree of decision records into the knowledge base", Menu: mi("Ingest", "bring record files into the database", 8, "v0.0.20", "kb ingest PATH"), Class: classPlanApply,
		Flags: []string{"--root", "--dry-run"}},
	{Name: "init", Summary: "create a new, empty workspace", Class: classCLIOnly},
	{Name: "link", Summary: "link projects or observations to concepts", Subverbs: []subverbSpec{
		sub("project", classAdditive), sub("observation", classAdditive),
	}},
	{Name: "merge", Summary: "reconcile two knowledge.db files that drifted independently", Class: classCLIOnly,
		Flags: []string{"--a", "--b", "--out", "--force"}},
	{Name: "observation", Summary: "manage observations", Menu: mi("Observations", "notes, findings, decisions, questions", 3, "", "kb observation"), Subverbs: []subverbSpec{
		leaf("add", classAdditive, mi("New observation…", "record a note, finding or decision", 2, "v0.0.20", "kb observation add --project P KIND BODY")), leaf("list", classRead, mi("Browse", "a project's observations", 1, "v0.0.20", "kb observation list --project P")), sub("show", classRead), sub("update", classChanging),
		sub("sources", classRead), sub("delete", classRemoving),
	}, Flags: []string{"--project", "--source-doi"}},
	{Name: "project", Summary: "manage projects", Menu: mi("Projects", "browse projects, their notes, concepts, records", 1, "", "kb project"), Subverbs: []subverbSpec{
		leaf("add", classAdditive, mi("New project…", "add a project", 2, "v0.0.20", "kb project add NAME DESCRIPTION")), leaf("list", classRead, mi("Browse", "the project list, with each project's observations, concepts and records", 1, "", "kb project list")),
		{Name: "show", Class: classRead, ProjectArg: true},
		{Name: "concepts", Class: classRead, ProjectArg: true},
		{Name: "set-status", Class: classChanging, ProjectArg: true},
		{Name: "set-description", Class: classChanging, ProjectArg: true},
		{Name: "rename", Class: classChanging, ProjectArg: true},
		{Name: "delete", Class: classRemoving, ProjectArg: true},
	}, Flags: []string{"--status", "--root", "--dry-run"}},
	{Name: "record", Summary: "read and maintain decision records", Menu: mi("Records", "decision records: browse, pending, …", 2, "", "kb record"), Subverbs: []subverbSpec{
		{Name: "list", Class: classRead, ScopeForm: "list", Menu: mi("Browse", "all records in scope, newest first", 1, "", "kb record list")},
		{Name: "pending", Class: classRead, ScopeForm: "list", Menu: mi("Pending", "proposed records, oldest first", 2, "", "kb record pending")},
		{Name: "show", Class: classRead, ScopeForm: "ref"},
		{Name: "set-status", Class: classChanging, ScopeForm: "ref"},
		{Name: "supersede", Class: classChanging, ScopeForm: "ref"},
		leaf("fmt", classDirect, mi("Format files…", "canonicalise record files", 4, "", "kb record fmt DIR")), leaf("new", classAdditive, mi("New record…", "scaffold a record", 3, "v0.0.20", "kb record new --title T --trigger G")),
		{Name: "concepts", Class: classRead, ScopeForm: "ref"},
		{Name: "delete", Class: classRemoving, ScopeForm: "ref"},
		leaf("fuzzy-tag", classPlanApply, mi("Fuzzy-tag…", "near-miss concept tags", 5, "v0.0.20", "kb record fuzzy-tag --project P")),
	}, Flags: []string{"--project", "--status", "--kind", "--trigger", "--initiative", "--since", "--root", "--dir",
		"--title", "--concept", "--workspace", "--partial", "--dry-run", "--write", "--all"}},
	{Name: "search", Summary: "full-text search across observations, projects, concepts and records", Menu: mi("Search", "full-text search across everything", 7, "", "kb search TERM"), Class: classRead,
		Flags: []string{"--project"}},
	{Name: "source", Summary: "manage cited sources and retraction checking", Menu: mi("Sources", "cited works and retraction checks", 5, "", "kb source"), Subverbs: []subverbSpec{
		leaf("add", classAdditive, mi("New source…", "add a cited work", 2, "v0.0.20", "kb source add TITLE")), leaf("list", classRead, mi("Browse", "every source", 1, "v0.0.20", "kb source list")), sub("show", classRead), sub("remove", classRemoving),
		sub("retract", classChanging), sub("link", classAdditive), leaf("check-retractions", classRead, mi("Check retractions…", "look sources up for retractions", 3, "v0.0.20", "kb source check-retractions")),
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

/** menuItem is one row of a TUI menu, derived from the verb table.
 *
 * Fields:
 *   Verb, Sub  (string) — the verb, and for a group menu the subverb it runs.
 *   Label, Desc (string) — what the row says.
 *   Built      (bool)   — there is a screen for it; otherwise it is dimmed.
 *   Since      (string) — the release that brings it, while it is not built.
 *   Equivalent (string) — the command line that does the same.
 *   Group      (bool)   — in the top menu: choosing it opens a group menu.
 */
type menuItem struct {
	Verb, Sub   string
	Label, Desc string
	Built       bool
	Since       string
	Equivalent  string
	Group       bool
}

/** topMenu returns the top menu: one row per verb that has a menu entry, in
 * order. A verb with leaves is a group and counts as built when any leaf is.
 *
 * Returns:
 *   []menuItem — the rows, lowest Order first.
 *
 * Example:
 *   for _, it := range topMenu() { fmt.Println(it.Label) }
 */
func topMenu() []menuItem {
	var items []menuItem
	order := map[string]int{}
	for _, v := range verbTable {
		if v.Menu == nil {
			continue
		}
		it := menuItem{Verb: v.Name, Label: v.Menu.Label, Desc: v.Menu.Desc, Built: v.Menu.Built,
			Since: v.Menu.Since, Equivalent: v.Menu.Equivalent, Group: len(v.Subverbs) > 0}
		if it.Group {
			// A group is built when any leaf is; when none is, it names the release
			// of its first leaf, so a dimmed group row says when it arrives.
			it.Built = false
			leaves := groupMenu(v.Name)
			for _, l := range leaves {
				if l.Built {
					it.Built = true
				}
			}
			if !it.Built && len(leaves) > 0 {
				it.Since = leaves[0].Since
			}
		}
		order[v.Name] = v.Menu.Order
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool { return order[items[i].Verb] < order[items[j].Verb] })
	return items
}

/** groupMenu returns a group's menu: the subverbs with a menu entry, in order.
 *
 * Parameters:
 *   verb (string) — the group, such as "record"
 *
 * Returns:
 *   []menuItem — the rows, lowest Order first; nil when the verb has none.
 *
 * Example:
 *   groupMenu("record") // Browse, Pending, New record…
 */
func groupMenu(verb string) []menuItem {
	var items []menuItem
	order := map[string]int{}
	var walk func(prefix string, subs []subverbSpec)
	walk = func(prefix string, subs []subverbSpec) {
		for _, sv := range subs {
			name := strings.TrimSpace(prefix + " " + sv.Name)
			if sv.Menu != nil {
				items = append(items, menuItem{Verb: verb, Sub: name, Label: sv.Menu.Label, Desc: sv.Menu.Desc,
					Built: sv.Menu.Built, Since: sv.Menu.Since, Equivalent: sv.Menu.Equivalent})
				order[name] = sv.Menu.Order
			}
			walk(name, sv.Subverbs)
		}
	}
	for _, v := range verbTable {
		if v.Name == verb {
			walk("", v.Subverbs)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return order[items[i].Sub] < order[items[j].Sub] })
	return items
}
