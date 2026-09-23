package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"

	knowledge "github.com/rsdoiel/knowledge"
)

func init() {
	verbs["concept"] = cmdConcept
}

func cmdConcept(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: concept <add|list|rename|suggest> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return cmdConceptAdd(kb, dl, jsonOut, rest, out)
	case "list":
		return cmdConceptList(kb, dl, jsonOut, rest, out)
	case "rename":
		return cmdConceptRename(kb, dl, jsonOut, rest, out)
	case "suggest":
		return cmdConceptSuggest(kb, dl, jsonOut, rest, out)
	default:
		return fmt.Errorf("unknown concept subcommand %q", sub)
	}
}

func cmdConceptAdd(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("concept add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	identifierType := fs.String("identifier-type", "", "e.g. doi, orcid, ror")
	identifierValue := fs.String("identifier-value", "", "normalized identifier value")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("usage: concept add NAME [DESCRIPTION] [--identifier-type T --identifier-value V]")
	}
	name := rest[0]
	desc := strings.Join(rest[1:], " ")
	var id int64
	var err error
	if *identifierType != "" || *identifierValue != "" {
		id, err = logKBCall(dl, "AddConceptWithIdentifier", map[string]any{"name": name, "identifier_type": *identifierType, "identifier_value": *identifierValue}, func() (int64, error) {
			return kb.AddConceptWithIdentifier(name, desc, *identifierType, *identifierValue)
		})
	} else {
		id, err = logKBCall(dl, "AddConcept", map[string]any{"name": name}, func() (int64, error) {
			return kb.AddConcept(name, desc)
		})
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{ID: id, Name: name})
	}
	fmt.Fprintf(out, "concept %q added (id=%d)\n", name, id)
	return nil
}

// cmdConceptRename implements `concept rename OLD NEW` (DR-0024). Unlike
// project rename, nothing refuses this beyond a name collision: a concept
// has no corpus of external files to desync.
func cmdConceptRename(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: concept rename OLD NEW")
	}
	old, new := args[0], args[1]
	err := logKBCallErr(dl, "RenameConcept", map[string]any{"old": old, "new": new}, func() error {
		return kb.RenameConcept(old, new)
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Old string `json:"old"`
			New string `json:"new"`
		}{Old: old, New: new})
	}
	fmt.Fprintf(out, "concept %q renamed to %q\n", old, new)
	return nil
}

func cmdConceptList(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	concepts, err := logKBCall(dl, "Concepts", nil, kb.Concepts)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, concepts)
	}
	if len(concepts) == 0 {
		fmt.Fprintln(out, "(no concepts)")
		return nil
	}
	for _, c := range concepts {
		fmt.Fprintf(out, "%-4d  %s\n", c.ID, c.Name)
	}
	return nil
}

// candidateTerm is one term-frequency suggestion for `kb concept suggest`
// (TODO.md's programmatic-corpus-improvement item), ranked by corpus-wide
// distinctiveness rather than raw frequency. Variants is v0.0.11 item 5's
// fuzzy clustering addition: spelling variants of Term (the cluster's
// highest-occurrence member, its canonical spelling) merged into this one
// candidate rather than scored separately -- omitted from JSON, and empty
// in Go, for a singleton (the common case), so existing output/consumers
// see no change.
type candidateTerm struct {
	Term        string   `json:"term"`
	Occurrences int      `json:"occurrences"`
	Items       int      `json:"items"`
	Score       float64  `json:"score"`
	Variants    []string `json:"variants,omitempty"`
}

// candidateTermPattern extracts single-word candidate terms: a letter
// followed by two or more letters/digits/hyphens/underscores, i.e. a
// minimum length of 3 -- short tokens are almost never a usable concept
// name and are pure noise at corpus scale.
var candidateTermPattern = regexp.MustCompile(`[a-z][a-z0-9_-]{2,}`)

// recordIDShapedPattern matches a bare record reference like dr-0013 or
// adr-0004: a letters-only prefix, a hyphen, then digits only. It is
// filtered outright rather than scored -- a record reference is a
// legitimate statistical signal (distinctive, often repeated) but never a
// usable concept name, so leaving it in would put the same kind of noise
// in every single run's output, not just an occasional false positive.
var recordIDShapedPattern = regexp.MustCompile(`^[a-z]+-[0-9]+$`)

// suggestStopwords is a small built-in list of common English function
// words, excluded from candidacy outright rather than left to the
// distinctiveness score alone -- at corpus scale they are frequent enough
// in nearly every item that idf would usually zero them anyway, but a
// smaller or lopsided corpus could let one slip through, and there is no
// reason to spend a candidate slot on "with" or "about".
var suggestStopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the and for that this with from into
		through during before after above below under again further then
		once here there when where why how all any both each few more most
		other some such nor not only own same than too very just but not
		are was were been being have has had does did doing will would
		shall should may might must can could you your yours they them
		their theirs she her hers him his its our ours who whom which what
		about over out off down up`) {
		suggestStopwords[w] = true
	}
}

// scoreCandidateTerms scores single-word candidate terms across a corpus
// of items (each item's raw text -- a record body or document section
// body), for `kb concept suggest`. A term is a candidate for a new
// concept when it is distinctive: mentioned several times overall but
// confined to relatively few items, the classic TF-IDF shape, rather than
// spread evenly across nearly every item (idf collapses to zero, filtered
// out as not distinctive) or mentioned only once anywhere in the whole
// corpus (below the >1 occurrence floor -- the same threshold DR-0027's
// density-linking uses, for the same reason: a single incidental mention
// is too weak a signal on its own).
//
// known is the set of already-existing concept names, lowercased --
// suggesting one again wastes a candidate slot. Code spans and fenced code
// blocks are stripped before matching (stripCodeSpans, shared with
// document ingest's density-linking, DR-0027), for the same reason: a
// short, common word colliding with a word used in a different sense
// inside quoted code is exactly the false-positive shape found there.
//
// Returned sorted by score descending, term ascending on a tie, for
// deterministic output; nil for an empty corpus or when nothing survives
// the filters.
func scoreCandidateTerms(items []string, known map[string]bool) (candidates []candidateTerm, nearExisting []nearExistingMatch) {
	if len(items) == 0 {
		return nil, nil
	}
	occurrences, itemCounts := tokenizeCandidateItems(items, known)

	// v0.0.11 item 5: a token fuzzy-close to an already-known concept is
	// excluded from candidacy entirely (fuzzy-tag's job, not this
	// command's), and clustering runs on the raw, pre-filter occurrence
	// map -- a variant individually below the occ<2/idf<=0 floor only
	// survives merged into a cluster, not scored on its own.
	survivors, nearExisting := excludeNearExisting(occurrences, known)
	clusters := clusterCandidateTerms(occurrences, itemCounts, survivors)

	n := float64(len(items))
	var out []candidateTerm
	for _, c := range clusters {
		if c.Occurrences < 2 {
			continue
		}
		df := len(c.Items)
		idf := math.Log(n / float64(df))
		if idf <= 0 {
			continue
		}
		out = append(out, candidateTerm{
			Term: c.Seed, Variants: c.Variants,
			Occurrences: c.Occurrences, Items: df, Score: float64(c.Occurrences) * idf,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Term < out[j].Term
	})
	return out, nearExisting
}

// tokenizeCandidateItems is scoreCandidateTerms's shared tokenization pass,
// factored out so fuzzy clustering (FC4) can build its own occurrence/
// item-index maps identically before scoreCandidateTerms's filter loop
// runs. itemCounts is a set of item indices per term (FC2, design decision
// 7), not a bare count -- needed so a cluster's df can be computed as the
// size of a *union* across members, not a sum; two mentions of the same
// term within one item still count once toward that term's df.
func tokenizeCandidateItems(items []string, known map[string]bool) (occurrences map[string]int, itemCounts map[string]map[int]bool) {
	occurrences = map[string]int{}
	itemCounts = map[string]map[int]bool{}
	for i, item := range items {
		text := strings.ToLower(stripCodeSpans(item))
		seenInItem := map[string]bool{}
		for _, tok := range candidateTermPattern.FindAllString(text, -1) {
			if known[tok] || suggestStopwords[tok] || recordIDShapedPattern.MatchString(tok) {
				continue
			}
			occurrences[tok]++
			if !seenInItem[tok] {
				seenInItem[tok] = true
				if itemCounts[tok] == nil {
					itemCounts[tok] = map[int]bool{}
				}
				itemCounts[tok][i] = true
			}
		}
	}
	return occurrences, itemCounts
}

// cmdConceptSuggest implements `kb concept suggest [--project NAME]
// [--limit N]`: a read-only scan of every record body and document
// section body -- scoped to one project when --project is given -- that
// prints candidate new concepts ranked by corpus-wide distinctiveness. It
// never writes anything; a suggestion becomes a real concept only when a
// human runs `kb concept add`, the same mechanical-signal-then-human-
// curates pattern DR-0027's density-linking and document summary review
// both already use.
func cmdConceptSuggest(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("concept suggest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	projectName := fs.String("project", "", "scope to one project")
	limit := fs.Int("limit", 20, "maximum number of suggestions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: concept suggest [--project NAME] [--limit N]")
	}

	var projectID int64
	if *projectName != "" {
		p, err := kb.ProjectByName(*projectName)
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("unknown project %q", *projectName)
		}
		projectID = p.ID
	}

	concepts, err := kb.Concepts()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, c := range concepts {
		known[strings.ToLower(c.Name)] = true
	}

	records, err := kb.ListRecords(knowledge.RecordFilter{Project: *projectName})
	if err != nil {
		return err
	}
	var items []string
	for _, r := range records {
		items = append(items, r.Body)
	}

	docs, err := kb.Documents(projectID)
	if err != nil {
		return err
	}
	for _, d := range docs {
		sections, err := kb.DocumentSections(d.ID)
		if err != nil {
			return err
		}
		for _, s := range sections {
			if s.Level == "section" {
				items = append(items, s.Body)
			}
		}
	}

	dl.Log("concept suggest", map[string]any{"project": *projectName, "items": len(items)})
	candidates, nearExisting := scoreCandidateTerms(items, known)
	if *limit > 0 && len(candidates) > *limit {
		candidates = candidates[:*limit]
	}

	if jsonOut {
		return printJSON(out, map[string]any{"candidates": candidates, "near_existing": nearExisting})
	}
	if len(candidates) == 0 {
		fmt.Fprintln(out, "no candidate concepts found")
	}
	for i, c := range candidates {
		name := c.Term
		if len(c.Variants) > 0 {
			name = fmt.Sprintf("%s (+%s)", c.Term, strings.Join(c.Variants, ", "))
		}
		fmt.Fprintf(out, "%2d. %-24s  score=%.2f  occurrences=%d  items=%d\n", i+1, name, c.Score, c.Occurrences, c.Items)
	}
	if len(nearExisting) > 0 {
		fmt.Fprintln(out, "\nnear-existing (excluded from candidates):")
		for _, m := range nearExisting {
			fmt.Fprintf(out, "  %s  ~  %s  (distance %d)\n", m.Token, m.Concept, m.Distance)
		}
	}
	return nil
}
