package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["verbs"] = cmdVerbs
}

// verbsJSONMenu is the menu entry of a verb or subverb in the output of kb --json
// verbs: the fields the TUI's menus are drawn from. Since is set only while the
// entry is not built.
type verbsJSONMenu struct {
	Label      string `json:"label"`
	Desc       string `json:"desc"`
	Order      int    `json:"order"`
	Built      bool   `json:"built"`
	Since      string `json:"since,omitempty"`
	Equivalent string `json:"equivalent"`
}

func verbsJSONMenuOf(m *menuEntry) *verbsJSONMenu {
	if m == nil {
		return nil
	}
	return &verbsJSONMenu{Label: m.Label, Desc: m.Desc, Order: m.Order, Built: m.Built, Since: m.Since, Equivalent: m.Equivalent}
}

// verbsJSONSub is one subverb in the output of kb --json verbs.
type verbsJSONSub struct {
	Name     string         `json:"name"`
	Class    string         `json:"class,omitempty"`
	TUI      bool           `json:"tui"`
	Menu     *verbsJSONMenu `json:"menu,omitempty"`
	Subverbs []verbsJSONSub `json:"subverbs,omitempty"`
}

// verbsJSONVerb is one verb in the output of kb --json verbs.
type verbsJSONVerb struct {
	Name     string         `json:"name"`
	Summary  string         `json:"summary"`
	Class    string         `json:"class,omitempty"`
	TUI      bool           `json:"tui"`
	Flags    []string       `json:"flags,omitempty"`
	Menu     *verbsJSONMenu `json:"menu,omitempty"`
	Subverbs []verbsJSONSub `json:"subverbs,omitempty"`
}

func verbsJSONSubs(subs []subverbSpec) []verbsJSONSub {
	var out []verbsJSONSub
	for _, s := range subs {
		out = append(out, verbsJSONSub{Name: s.Name, Class: s.Class.String(), TUI: len(s.Subverbs) > 0 || s.Class != classCLIOnly,
			Menu: verbsJSONMenuOf(s.Menu), Subverbs: verbsJSONSubs(s.Subverbs)})
	}
	return out
}

/** cmdVerbs implements `kb verbs`: it prints the verb table, so a model or a
 * script has one description of the command language (knowledge DR-0059). Each
 * verb comes with its summary, its flags, and its subverbs with their write
 * classes; "tui" says whether the interactive interface offers it. Plain text
 * lists one row per verb or subverb; the global --json prints the table as JSON.
 * It opens no database.
 *
 * Parameters:
 *   _ (*knowledge.KnowledgeBase) — unused; the verb needs no database
 *   _ (*DebugLog) — unused
 *   jsonOut (bool) — print JSON
 *   args ([]string) — must be empty
 *   out (io.Writer) — where the table goes
 *
 * Returns:
 *   error — a usage error for a flag or argument, else nil or a write failure
 *
 * Example:
 *   err := cmdVerbs(nil, nil, true, nil, os.Stdout)
 */
func cmdVerbs(_ *knowledge.KnowledgeBase, _ *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	positional, err := splitFlags(args, nil, nil)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return usageErrorf("usage: verbs (it takes no arguments; --json is a global option, as in kb --json verbs)")
	}
	if jsonOut {
		doc := struct {
			Verbs []verbsJSONVerb `json:"verbs"`
		}{}
		// A verb's built state and release are derived from its leaves, as the
		// top menu derives them, so the output agrees with what the TUI draws.
		top := map[string]menuItem{}
		for _, it := range topMenu() {
			top[it.Verb] = it
		}
		for _, v := range verbTable {
			var menu *verbsJSONMenu
			if v.Menu != nil {
				menu = verbsJSONMenuOf(v.Menu)
				it := top[v.Name]
				menu.Built, menu.Since = it.Built, it.Since
			}
			doc.Verbs = append(doc.Verbs, verbsJSONVerb{
				Name: v.Name, Summary: v.Summary, Class: v.Class.String(),
				TUI:   len(v.Subverbs) > 0 || v.Class != classCLIOnly,
				Flags: v.Flags, Menu: menu, Subverbs: verbsJSONSubs(v.Subverbs),
			})
		}
		return printJSON(out, doc)
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	var rows func(path string, subs []subverbSpec)
	rows = func(path string, subs []subverbSpec) {
		for _, s := range subs {
			p := path + " " + s.Name
			if len(s.Subverbs) > 0 {
				rows(p, s.Subverbs)
				continue
			}
			fmt.Fprintf(tw, "  %s\t%s\t\n", p, s.Class)
		}
	}
	for _, v := range verbTable {
		if len(v.Subverbs) == 0 {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", v.Name, v.Class, v.Summary)
			continue
		}
		fmt.Fprintf(tw, "%s\t\t%s\n", v.Name, v.Summary)
		rows(v.Name, v.Subverbs)
	}
	return tw.Flush()
}
