package knowledge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// SearchScoped applies a scope (a project, or the workspace tier) and a set of
// kinds in the query itself (workspace DR-0073 and knowledge DR-0074), so a limit
// never hides a match in scope behind a better-ranked one outside it.

// scopedFixture is two projects and the workspace tier, each with something that
// matches "cache", and concepts attached in each of the three ways.
type scopedFixture struct {
	kb                         *KnowledgeBase
	alpha, beta                int64
	obsAlpha, obsBeta          int64
	recAlpha, recBeta, recWork int64
	conceptAlpha, conceptBoth  int64
	conceptWork, conceptOrphan int64
}

func newScopedFixture(t *testing.T) *scopedFixture {
	t.Helper()
	kb := openTestKB(t)
	f := &scopedFixture{kb: kb}
	var err error
	must := func(id int64, err error) int64 {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.alpha = must(kb.AddProject("alpha", "a cache service"))
	f.beta = must(kb.AddProject("beta", "something else"))
	f.obsAlpha = must(kb.AddObservation(f.alpha, "finding", "cache invalidation is the hard part"))
	f.obsBeta = must(kb.AddObservation(f.beta, "note", "cache warmup takes a minute"))
	f.recAlpha = must(kb.AddRecord(Record{RecordID: "0001", ProjectID: f.alpha, Scope: "project", Path: "a/0001.md",
		Title: "Alpha cache layer", Date: "2026-10-01", Status: "accepted", Kind: "decision", Body: "the cache layer sits in front"}))
	f.recBeta = must(kb.AddRecord(Record{RecordID: "0001", ProjectID: f.beta, Scope: "project", Path: "b/0001.md",
		Title: "Beta cache policy", Date: "2026-10-02", Status: "accepted", Kind: "decision", Body: "beta evicts its cache"}))
	f.recWork = must(kb.AddRecord(Record{RecordID: "0002", Scope: "workspace", Path: "w/0002.md",
		Title: "Workspace cache rule", Date: "2026-10-03", Status: "accepted", Kind: "decision", Body: "a cache rule for every project"}))

	f.conceptAlpha = must(kb.AddConcept("alpha-cache-idea", "a cache concept tagged on the project"))
	f.conceptBoth = must(kb.AddConcept("caching", "the cache concept that spans projects"))
	f.conceptWork = must(kb.AddConcept("cache-policy", "a cache concept from a workspace record"))
	f.conceptOrphan = must(kb.AddConcept("orphan-cache", "a cache concept nobody uses"))
	if err = kb.LinkProjectConcept(f.alpha, f.conceptAlpha); err != nil {
		t.Fatal(err)
	}
	if err = kb.LinkObservationConcept(f.obsAlpha, f.conceptBoth); err != nil { // reaches alpha through an observation
		t.Fatal(err)
	}
	if err = kb.LinkRecordConcept(f.recBeta, f.conceptBoth); err != nil { // and beta through a record
		t.Fatal(err)
	}
	if err = kb.LinkRecordConcept(f.recWork, f.conceptWork); err != nil {
		t.Fatal(err)
	}
	return f
}

// scopedLabels returns "type:id" for each result, sorted.
func scopedIDs(r *ScopedSearchResult) []string {
	var out []string
	for _, x := range r.Results {
		out = append(out, fmt.Sprintf("%s:%d", x.SourceType, x.SourceID))
	}
	sort.Strings(out)
	return out
}

func wantIDs(t *testing.T, got *ScopedSearchResult, want ...string) {
	t.Helper()
	sort.Strings(want)
	g := scopedIDs(got)
	if strings.Join(g, ",") != strings.Join(want, ",") {
		t.Errorf("results = %v, want %v", g, want)
	}
}

func TestSearchScoped_NoScopeIsTheGlobalSearch(t *testing.T) {
	f := newScopedFixture(t)
	plain, err := f.kb.Search("cache")
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.kb.SearchScoped("cache", SearchScope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != len(plain) || got.Total != len(plain) {
		t.Errorf("scoped with no scope: %d results, total %d; Search gave %d", len(got.Results), got.Total, len(plain))
	}
}

func TestSearchScoped_ProjectScope(t *testing.T) {
	f := newScopedFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got,
		fmt.Sprintf("observation:%d", f.obsAlpha),
		fmt.Sprintf("record:%d", f.recAlpha),
		fmt.Sprintf("concept:%d", f.conceptAlpha), // tagged on the project
		fmt.Sprintf("concept:%d", f.conceptBoth),  // tagged on one of its observations
	)
}

func TestSearchScoped_ConceptsAreTheUnionOfThreeLinks(t *testing.T) {
	f := newScopedFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Project: "beta", Kinds: []string{"concepts"}})
	if err != nil {
		t.Fatal(err)
	}
	// "caching" reaches beta only through a record; the project-tagged concept is alpha's.
	wantIDs(t, got, fmt.Sprintf("concept:%d", f.conceptBoth))
}

func TestSearchScoped_WorkspaceIsTheTierOnly(t *testing.T) {
	f := newScopedFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Workspace: true})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got,
		fmt.Sprintf("record:%d", f.recWork),
		fmt.Sprintf("concept:%d", f.conceptWork), // linked to a workspace-tier record
	)
}

func TestSearchScoped_KindsNarrow(t *testing.T) {
	f := newScopedFixture(t)
	got, err := f.kb.SearchScoped("cache", SearchScope{Kinds: []string{"records"}})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got,
		fmt.Sprintf("record:%d", f.recAlpha), fmt.Sprintf("record:%d", f.recBeta), fmt.Sprintf("record:%d", f.recWork))
	got, err = f.kb.SearchScoped("cache", SearchScope{Kinds: []string{"projects"}})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got, fmt.Sprintf("project:%d", f.alpha)) // only alpha's description says cache
}

func TestSearchScoped_ImpossibleScopeAndKindIsErrNoMembers(t *testing.T) {
	f := newScopedFixture(t)
	for name, sc := range map[string]SearchScope{
		"workspace observations": {Workspace: true, Kinds: []string{"observations"}},
		"workspace projects":     {Workspace: true, Kinds: []string{"projects"}},
		"project projects":       {Project: "alpha", Kinds: []string{"projects"}},
		"only impossible kinds":  {Workspace: true, Kinds: []string{"observations", "projects"}},
	} {
		_, err := f.kb.SearchScoped("cache", sc)
		if !errors.Is(err, ErrNoMembers) {
			t.Errorf("%s: err = %v, want ErrNoMembers", name, err)
		}
	}
	// One possible kind among impossible ones is a search; the impossible one adds nothing.
	got, err := f.kb.SearchScoped("cache", SearchScope{Workspace: true, Kinds: []string{"observations", "records"}})
	if err != nil {
		t.Fatalf("mixed kinds: %v", err)
	}
	wantIDs(t, got, fmt.Sprintf("record:%d", f.recWork))
}

func TestSearchScoped_UnknownProjectAndKind(t *testing.T) {
	f := newScopedFixture(t)
	if _, err := f.kb.SearchScoped("cache", SearchScope{Project: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown project: err = %v, want ErrNotFound", err)
	}
	if _, err := f.kb.SearchScoped("cache", SearchScope{Kinds: []string{"sources"}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("sources is not a kind yet: err = %v, want ErrInvalid", err)
	}
	if _, err := f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Workspace: true}); !errors.Is(err, ErrInvalid) {
		t.Errorf("project and workspace together: err = %v, want ErrInvalid", err)
	}
}

func TestSearchScoped_TheScopeIsAppliedBeforeTheLimit(t *testing.T) {
	f := newScopedFixture(t)
	// Beta's observations repeat the term, so they outrank alpha's. A limit of 1 on
	// alpha must still return alpha's observation, not an empty list.
	for i := 0; i < 6; i++ {
		if _, err := f.kb.AddObservation(f.beta, "note", "cache cache cache cache cache cache"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Kinds: []string{"observations"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got, fmt.Sprintf("observation:%d", f.obsAlpha))
	if got.Total != 1 {
		t.Errorf("total = %d, want 1", got.Total)
	}
}

func TestSearchScoped_TotalAndCountsSurviveTheLimit(t *testing.T) {
	f := newScopedFixture(t)
	for i := 0; i < 4; i++ {
		if _, err := f.kb.AddObservation(f.alpha, "note", fmt.Sprintf("another cache note %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 2 {
		t.Errorf("%d results, want the limit, 2", len(got.Results))
	}
	if got.Total != 8 { // 5 observations, 1 record, 2 concepts; the project's own row is not a member of its scope
		t.Errorf("total = %d, want 8", got.Total)
	}
	want := map[string]int{"observations": 5, "records": 1, "concepts": 2}
	for k, n := range want {
		if got.Counts[k] != n {
			t.Errorf("counts[%s] = %d, want %d (all: %v)", k, got.Counts[k], n, got.Counts)
		}
	}
}

func TestSearchScoped_ConceptsShowTheProjectsTheySpan(t *testing.T) {
	f := newScopedFixture(t)
	got, err := f.kb.SearchScoped("caching", SearchScope{Kinds: []string{"concepts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 {
		t.Fatalf("%d results, want the one concept", len(got.Results))
	}
	if s := strings.Join(got.Spans[got.Results[0].SourceID], ","); s != "alpha,beta" {
		t.Errorf("projects = %q, want alpha,beta", s)
	}
}

func TestSearchScoped_ADocumentSummaryIsFoundInItsProject(t *testing.T) {
	f := newScopedFixture(t)
	docID, err := f.kb.AddDocument(Document{ProjectID: f.alpha, Title: "A Post", Format: "text", Path: "p.txt"})
	if err != nil {
		t.Fatal(err)
	}
	secID, err := f.kb.AddDocumentSection(DocumentSection{DocumentID: docID, Level: "section", Heading: "Intro", Body: "text"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.kb.DraftDocumentSummary(secID, "a summary of the cache post", "human", nil); err != nil {
		t.Fatal(err)
	}
	// A drafted summary is not trusted, so it is not in the index yet.
	got, _ := f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Kinds: []string{"documents"}})
	if got != nil && len(got.Results) != 0 {
		t.Fatalf("a drafted summary was found: %v", scopedIDs(got))
	}
	if err := f.kb.PromoteDocumentSummary(secID); err != nil {
		t.Fatal(err)
	}
	got, err = f.kb.SearchScoped("cache", SearchScope{Project: "alpha", Kinds: []string{"documents"}})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got, fmt.Sprintf("document_summary:%d", secID))
	got, err = f.kb.SearchScoped("cache", SearchScope{Project: "beta", Kinds: []string{"documents"}})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs(t, got)
}

func TestSearchScoped_ATermThatIsNotQuerySyntaxIsAPhrase(t *testing.T) {
	f := newScopedFixture(t)
	if _, err := f.kb.AddObservation(f.alpha, "note", "use map-reduce for the batch"); err != nil {
		t.Fatal(err)
	}
	got, err := f.kb.SearchScoped("map-reduce", SearchScope{Project: "alpha"})
	if err != nil {
		t.Fatalf("a hyphenated term failed: %v", err)
	}
	if len(got.Results) != 1 || got.Total != 1 {
		t.Errorf("%d results, total %d, want 1", len(got.Results), got.Total)
	}
}

func TestSearchScoped_NothingFoundIsAnEmptyResultNotAnError(t *testing.T) {
	f := newScopedFixture(t)
	got, err := f.kb.SearchScoped("zzzznothing", SearchScope{Project: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.Results) != 0 || got.Total != 0 {
		t.Errorf("got %+v, want an empty result", got)
	}
}
