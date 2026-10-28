package main

import (
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Workspace DR-0003: a tool documents the codes it can return under EXIT STATUS
// in its manual and has a test that gives each documented code. These tests keep
// the manual pages and the verb tests in step (v0.0.28, release-plan-v0.0.28.md).
//
// Which codes a page lists: 0 and 2 always (every verb succeeds and every verb
// can be called wrongly), 1 where the verb has a negative answer, and the
// classes below only where the verb handles a file of its own. 65, 66, 74 and 77
// can come from opening the database, so a verb that touches only the database
// leaves them to the table in kb(1).

// exitStatusPages are the topics whose page must have an EXIT STATUS section.
func exitStatusPages(t *testing.T) []string {
	t.Helper()
	return makefileTopics(t)
}

// fileVerbs handle a file or directory the caller names, so their pages list the
// file classes (65, 66, 73, 74, 77) they provoke as well as 0, 1 and 2.
var fileVerbs = map[string]bool{
	"export": true, "import": true, "merge": true, "index": true, "ingest": true, "init": true,
}

// genericClasses can come from any verb that opens the database. They are listed
// per verb only for fileVerbs.
var genericClasses = map[int]bool{65: true, 66: true, 74: true, 75: true, 77: true}

// unprovable are classes no verb test can provoke: 75 needs a second writer
// holding the database, and is tested at the classifier (exitcode_test.go).
var unprovable = map[int]bool{75: true}

// pageOfVerb is the topic whose page documents a verb: summary and format are
// described in kb-search(1).
func pageOfVerb(verb string) string {
	switch verb {
	case "summary", "format":
		return "search"
	}
	return verb
}

// helpPage returns the text of `kb help TOPIC`.
func helpPage(t *testing.T, topic string) string {
	t.Helper()
	var out bytes.Buffer
	if !printHelp(&out, topic) {
		t.Fatalf("no help topic %q", topic)
	}
	return out.String()
}

// sectionOf returns the body of the "# HEADING" section of a page, up to the next
// top-level heading, and whether the page has it.
func sectionOf(page, heading string) (string, bool) {
	_, rest, ok := strings.Cut(page, "\n# "+heading+"\n")
	if !ok {
		return "", false
	}
	body, _, _ := strings.Cut(rest, "\n# ")
	return body, true
}

var (
	drRef     = regexp.MustCompile(`\bDR-\d+`)
	manRef    = regexp.MustCompile(`[\w-]+\(\d\)`)
	codeToken = regexp.MustCompile(`\b\d+\b`)
)

// codesIn returns the exit codes a section names: every whole number in it, after
// removing the decision record references (DR-0003) and manual references (kb(1)),
// which are numbers of another kind.
func codesIn(section string) []int {
	section = drRef.ReplaceAllString(section, "")
	section = manRef.ReplaceAllString(section, "")
	seen := map[int]bool{}
	for _, tok := range codeToken.FindAllString(section, -1) {
		n, err := strconv.Atoi(tok)
		if err == nil {
			seen[n] = true
		}
	}
	var out []int
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func TestManPages_EveryPageHasExitStatus(t *testing.T) {
	for _, topic := range exitStatusPages(t) {
		t.Run(topic, func(t *testing.T) {
			if _, ok := sectionOf(helpPage(t, topic), "EXIT STATUS"); !ok {
				t.Errorf("kb-%s(1) has no EXIT STATUS section (workspace DR-0003)", topic)
			}
		})
	}
}

func TestManPages_EveryPageHasSeeAlso(t *testing.T) {
	for _, topic := range exitStatusPages(t) {
		t.Run(topic, func(t *testing.T) {
			if _, ok := sectionOf(helpPage(t, topic), "SEE ALSO"); !ok {
				t.Errorf("kb-%s(1) has no SEE ALSO section", topic)
			}
		})
	}
}

// A code outside the workspace table, or one that belongs to the shell, is a
// typo or a leftover from before the convention (64 was usage until DR-0003).
func TestManPages_ExitStatusNamesOnlyDocumentedCodes(t *testing.T) {
	allowed := map[int]bool{}
	for _, c := range allExitClasses {
		allowed[c.Code] = true
	}
	for _, topic := range exitStatusPages(t) {
		t.Run(topic, func(t *testing.T) {
			section, ok := sectionOf(helpPage(t, topic), "EXIT STATUS")
			if !ok {
				t.Skip("no EXIT STATUS section; TestManPages_EveryPageHasExitStatus says so")
			}
			for _, n := range codesIn(section) {
				if !allowed[n] {
					t.Errorf("kb-%s(1) EXIT STATUS names %d, which is not in the workspace table (DR-0003)", topic, n)
				}
			}
		})
	}
}

// provokedCodes reads the classes each verb is provoked into, from the tables in
// exitcodes_verbs_test.go and from the verbs whose class is proved in a test of
// their own.
func provokedCodes() map[string]map[int]bool {
	got := map[string]map[int]bool{}
	add := func(verb string, code int) {
		page := pageOfVerb(verb)
		if got[page] == nil {
			got[page] = map[int]bool{}
		}
		got[page][code] = true
	}
	for _, group := range []struct {
		cases []verbCase
		class exitClass
	}{
		{negativeVerbCases, classNegative}, {usageVerbCases, classUsage}, {dataVerbCases, classData},
		{noInputVerbCases, classNoInput}, {cantCreateVerbCases, classCantCreate},
	} {
		for _, tc := range group.cases {
			if len(tc.args) > 0 {
				add(tc.args[0], group.class.Code)
			}
		}
	}
	// One-off cases in other tests, each named so the proof can be found.
	for verb, codes := range provedElsewhere {
		for _, c := range codes {
			add(verb, c)
		}
	}
	return got
}

// provedElsewhere lists the classes that a verb is provoked into by a test outside
// the tables, with that test's name. TestVerbs_IOIsExit74 and
// TestVerbs_PermissionIsExit77 are single cases, listed here too.
var provedElsewhere = map[string][]int{
	"export": {74, 77}, // TestVerbs_ExportWriteFailures
	"import": {74, 77}, // TestVerbs_IOIsExit74, TestVerbs_PermissionIsExit77
	"init":   {77},     // TestVerbs_PermissionIsExit77
	"ingest": {65},     // TestCmdIngest_MalformedRecordsExit65AfterIngestingTheGoodOnes
	"source": {69},     // TestCheckRetractions_UnreachableServiceExits69AfterCheckingTheRest
	"index":  {1},      // TestIndexAll_StaleOnlyIsNegative
	"search": {1},      // TestVerbs_NegativeIsExit1
	"merge":  {65},     // TestMerge_CollisionIsExit65
}

// checkedAgainstVerbTests are the pages written for v0.0.28. The older pages (concept,
// record, check-db, stats, story, completion, document) name codes that tests of
// their own provoke (conceptshowrecall_test.go and others) and not the tables the
// check reads, and record omits 73; bringing them under this check is
// release-plan-v0.0.28.md, T4, follow-up.
var checkedAgainstVerbTests = []string{
	"export", "import", "index", "ingest", "init", "link", "merge",
	"observation", "project", "search", "source", "unlink", "topics",
}

// The page and the tests agree on every code but 0 and 2: a code a page names is
// provoked by a test, and a class a test provokes is on the page. Adding a way for
// a verb to fail is a documentation change as well, and a code in the manual with
// no test behind it is a claim nobody checks.
func TestManPages_ExitStatusMatchesTheVerbTests(t *testing.T) {
	proved := provokedCodes()
	for _, topic := range checkedAgainstVerbTests {
		t.Run(topic, func(t *testing.T) {
			section, ok := sectionOf(helpPage(t, topic), "EXIT STATUS")
			if !ok {
				t.Skip("no EXIT STATUS section; TestManPages_EveryPageHasExitStatus says so")
			}
			named := map[int]bool{}
			for _, n := range codesIn(section) {
				named[n] = true
			}
			for n := range named {
				if n == 0 || n == 2 || unprovable[n] {
					continue
				}
				if !proved[topic][n] {
					t.Errorf("kb-%s(1) names exit %d, but no verb test provokes it: add a case to exitcodes_verbs_test.go", topic, n)
				}
			}
			for n := range proved[topic] {
				if genericClasses[n] && !fileVerbs[topic] {
					continue
				}
				if !named[n] {
					t.Errorf("kb-%s(1) EXIT STATUS does not name %d, which a verb test provokes", topic, n)
				}
			}
		})
	}
}
