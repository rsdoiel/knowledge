/*
knowledge is a SQLite3-backed knowledge base for experiments.
Copyright (C) 2026 R. S. Doiel

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package knowledge

import (
	"bytes"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The documentation is a Markdown site published as HTML and as Markdown for
// BonBon browsers and cmarkbrowser. These tests keep it that way: every page is
// in the page order, the pages link to each other as Markdown, the table of
// contents lists them, and the built HTML points at its Markdown.

// siteNoPublish are the Markdown files that are not website pages.
var siteNoPublish = map[string]bool{
	"TODO.md": true, "ISSUE_DRAFT.md": true, "PR_DRAFT.md": true, "PR_REPLY.md": true, "index.md": true,
}

// siteVar returns the value of a variable set on one line in a make file.
func siteVar(t *testing.T, file, name string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^` + name + ` *= *(.*)$`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("%s has no %s", file, name)
	}
	return strings.TrimSpace(string(m[1]))
}

// sitePageOrder is PAGE_ORDER with its one computed part, the verb pages built
// from KB_TOPICS, written out.
func sitePageOrder(t *testing.T) []string {
	t.Helper()
	var out []string
	expr := regexp.MustCompile(`\$\(addprefix.*?\(KB_TOPICS\)\)\)`)
	for _, f := range strings.Fields(expr.ReplaceAllString(siteVar(t, "website.mak", "PAGE_ORDER"), "")) {
		out = append(out, f)
	}
	// Put the verb pages where the addprefix expression stands: after kb.1.md.
	var topics []string
	for _, topic := range strings.Fields(siteVar(t, "website.mak", "KB_TOPICS")) {
		topics = append(topics, "kb-"+topic+".1.md")
	}
	var ordered []string
	for _, f := range out {
		ordered = append(ordered, f)
		if f == "kb.1.md" {
			ordered = append(ordered, topics...)
		}
	}
	return ordered
}

func sitePublishedPages(t *testing.T) []string {
	t.Helper()
	all, _ := filepath.Glob("*.md")
	var out []string
	for _, f := range all {
		if !siteNoPublish[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

func siteReadText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSiteEveryPublishedPageIsInThePageOrder(t *testing.T) {
	order := append([]string(nil), sitePageOrder(t)...)
	sort.Strings(order)
	if got, want := strings.Join(order, " "), strings.Join(sitePublishedPages(t), " "); got != want {
		t.Errorf("PAGE_ORDER in website.mak\n  %s\nbut the published pages are\n  %s", got, want)
	}
	if o := sitePageOrder(t); o[0] != "README.md" {
		t.Errorf("the home page must come first: %v", o)
	}
}

// KB_TOPICS is written in the Makefile and in website.mak; the two lists must
// agree, or a verb's manual page would be built and not published.
func TestSiteTopicsMatchTheMakefile(t *testing.T) {
	if a, b := siteVar(t, "Makefile", "KB_TOPICS"), siteVar(t, "website.mak", "KB_TOPICS"); a != b {
		t.Errorf("KB_TOPICS differs\n  Makefile:    %s\n  website.mak: %s", a, b)
	}
}

// The Markdown is the truth: a page links to another page as .md, and the
// HTML build turns that into .html (links-to-html.lua).
func TestSitePagesLinkToEachOtherAsMarkdown(t *testing.T) {
	link := regexp.MustCompile(`\]\(([^)\s]+)\)`)
	published := map[string]bool{}
	for _, p := range sitePageOrder(t) {
		published[p] = true
	}
	for _, page := range sitePublishedPages(t) {
		for _, m := range link.FindAllStringSubmatch(siteReadText(t, page), -1) {
			dest := m[1]
			if strings.Contains(dest, "://") || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "mailto:") {
				continue
			}
			file := strings.SplitN(dest, "#", 2)[0]
			switch {
			case strings.HasSuffix(file, ".html"):
				t.Errorf("%s links to %s: link to the .md page, the HTML build rewrites it", page, dest)
			case strings.HasSuffix(file, ".md") && !published[file] && !siteNoPublish[file]:
				t.Errorf("%s links to %s, which is not a published page", page, dest)
			}
		}
	}
}

// A pandoc "%" title block and JSON front matter show as text in a reader
// that follows CommonMark; YAML front matter is stripped.
func TestSitePagesStartLikeCommonMark(t *testing.T) {
	for _, page := range sitePublishedPages(t) {
		first := strings.SplitN(siteReadText(t, page), "\n", 2)[0]
		if strings.HasPrefix(first, "%") || strings.HasPrefix(first, "{") {
			t.Errorf("%s starts with %q, not YAML front matter or text", page, first)
		}
	}
}

func siteRunToc(t *testing.T) []byte {
	t.Helper()
	args := append([]string{"./toc.bash", siteVar(t, "website.mak", "BASE_URL"), siteVar(t, "website.mak", "PROJECT")}, sitePageOrder(t)...)
	out, err := exec.Command("bash", args...).Output()
	if err != nil {
		t.Fatalf("toc.bash: %v", err)
	}
	return out
}

type siteOutline struct {
	Text string        `xml:"text,attr"`
	URL  string        `xml:"url,attr"`
	Kids []siteOutline `xml:"outline"`
}

func (o siteOutline) leaves() []siteOutline {
	if len(o.Kids) == 0 {
		return []siteOutline{o}
	}
	var out []siteOutline
	for _, k := range o.Kids {
		out = append(out, k.leaves()...)
	}
	return out
}

func TestSiteTocListsThePagesInOrder(t *testing.T) {
	var doc struct {
		Body struct {
			Outlines []siteOutline `xml:"outline"`
		} `xml:"body"`
	}
	if err := xml.Unmarshal(siteRunToc(t), &doc); err != nil {
		t.Fatal(err)
	}
	var got []string
	titles := map[string]string{}
	for _, o := range doc.Body.Outlines {
		for _, l := range o.leaves() {
			got = append(got, l.URL)
			titles[l.URL] = l.Text
		}
	}
	var want []string
	base := siteVar(t, "website.mak", "BASE_URL")
	for _, p := range sitePageOrder(t) {
		if p == "README.md" {
			p = "index.md"
		}
		want = append(want, base+p)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("toc urls\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for url, title := range titles {
		if title == "" || title == "NAME" || strings.Contains(title, "*") || strings.HasSuffix(title, ".md") {
			t.Errorf("%s has the toc title %q: give the page a YAML title or a heading", url, title)
		}
	}
}

// The head's dateModified is the date of the last commit that touched a page,
// so committing the toc changes what a rebuild writes: that one line is left
// out of the comparison, or the test could never pass after a commit.
func TestSiteCommittedTocIsCurrent(t *testing.T) {
	withoutDate := func(s string) string {
		var keep []string
		for _, line := range strings.Split(s, "\n") {
			if !strings.Contains(line, "<dateModified>") {
				keep = append(keep, line)
			}
		}
		return strings.Join(keep, "\n")
	}
	if got, want := withoutDate(siteReadText(t, ".well-known/toc.opml")), withoutDate(string(siteRunToc(t))); got != want {
		t.Errorf(".well-known/toc.opml is out of date: run `make website`")
	}
}

func TestSiteIndexMdIsReadmeAndSiteIsNotJekyll(t *testing.T) {
	if siteReadText(t, "index.md") != siteReadText(t, "README.md") {
		t.Error("index.md differs from README.md: run `make website`")
	}
	if _, err := os.Stat(".nojekyll"); err != nil {
		t.Error("no .nojekyll: GitHub Pages would hide /.well-known/")
	}
}

// Each HTML page names its Markdown, for a reader that finds the HTML first.
func TestSiteHTMLPagesPointAtTheirMarkdown(t *testing.T) {
	for _, md := range sitePageOrder(t) {
		html := strings.TrimSuffix(md, ".md") + ".html"
		want := md
		if md == "README.md" {
			html, want = "index.html", "index.md"
		}
		page := siteReadText(t, html)
		if !strings.Contains(page, `<link rel="alternate" type="text/markdown" href="`+want+`">`) {
			t.Errorf("%s has no alternate link to %s: run `make website`", html, want)
		}
	}
}

func TestSiteLinksToHTMLFilter(t *testing.T) {
	pandoc, err := exec.LookPath("pandoc")
	if err != nil {
		t.Skip("no pandoc")
	}
	src := "[a](user_manual.md#keys) [b](README.md) [c](https://example.org/x.md) [d](mailto:a@b.c) [e](#top) [f](other.html) [g](/abs.md)\n"
	cmd := exec.Command(pandoc, "--from", "markdown", "--to", "markdown", "--wrap=none", "--lua-filter=links-to-html.lua")
	cmd.Stdin = strings.NewReader(src)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	want := "[a](user_manual.html#keys) [b](index.html) [c](https://example.org/x.md) [d](mailto:a@b.c) [e](#top) [f](other.html) [g](/abs.md)"
	if got := strings.TrimSpace(out.String()); got != want {
		t.Errorf("links-to-html.lua\n got %s\nwant %s", got, want)
	}
}
