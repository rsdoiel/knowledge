package knowledge

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

/** scaleChapters ingests n scenes as Markdown chapters of 25, one dated scene
 * per day with one learns each, into a new project, and returns its id.
 *
 * Parameters:
 *   t (*testing.T) — the test
 *   kb (*KnowledgeBase) — an open knowledge base
 *   n (int) — the number of scenes
 *
 * Returns:
 *   int64 — the project id
 *
 * Example:
 *   pid := scaleChapters(t, kb, 1000)
 */
func scaleChapters(t *testing.T, kb *KnowledgeBase, n int) int64 {
	t.Helper()
	pid, err := kb.AddProject("scale", "")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for c := 0; c*25 < n; c++ {
		var sb strings.Builder
		sb.WriteString("---\ntitle: chapter\n---\n\n")
		for s := c * 25; s < (c+1)*25 && s < n; s++ {
			fmt.Fprintf(&sb, "## Scene %d {#S%d}\n<!-- story: story_time=%s; learns=Anna: fact number %d -->\n\ntext\n\n",
				s, s, day.AddDate(0, 0, s).Format("2006-01-02"), s)
		}
		ingestStory(t, kb, pid, writeTempFile(t, fmt.Sprintf("c%d.md", c), sb.String()))
	}
	return pid
}

// TestScale_StoryVerbs times the story verbs on a long work. It is skipped
// unless KB_SCALE names the scene counts (KB_SCALE=300,1000,3000), so the
// ordinary suite stays fast.
func TestScale_StoryVerbs(t *testing.T) {
	spec := os.Getenv("KB_SCALE")
	if spec == "" {
		t.Skip("set KB_SCALE=300,1000,3000 to time the story verbs")
	}
	for _, f := range strings.Split(spec, ",") {
		var n int
		fmt.Sscan(f, &n)
		t.Run(f, func(t *testing.T) {
			kb := openTestKB(t)
			pid := scaleChapters(t, kb, n)
			time1 := func(name string, fn func() error) {
				start := time.Now()
				if err := fn(); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				t.Logf("%d scenes: %-10s %v", n, name, time.Since(start))
			}
			time1("timeline", func() error { _, err := kb.StoryTimeline(pid, false); return err })
			time1("check", func() error { _, err := kb.StoryCheck(pid); return err })
			time1("who-knows", func() error { _, err := kb.WhoKnows(pid, "Anna", ""); return err })
		})
	}
}
