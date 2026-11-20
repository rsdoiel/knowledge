
#
# Makefile for the knowledge website: pandoc builds an HTML page beside each Markdown
# page, the Markdown pages are published as they are, for BonBon browsers and
# cmarkbrowser, and toc.bash writes /.well-known/toc.opml.
#
# The pages are listed by name, not globbed, so the site does not change with
# whatever .md files happen to sit in the directory. site_test.go checks that
# every published page is listed and that the built files are current.
# README.md becomes index.html and index.md.
#
# Written by hand from cmarkbrowser's website.mak; do not regenerate it with cmt.
#
PROJECT = knowledge

# Where the site is published (GitHub Pages, gh-pages branch).
BASE_URL = https://rsdoiel.github.io/knowledge/

PANDOC = $(shell which pandoc)

PROGRAMS = kb

# The verb pages, kb-TOPIC.1.md; the same list as KB_TOPICS in the Makefile (site_test.go checks).
KB_TOPICS = project observation concept link unlink source search merge check-db stats story export import ingest record document index init model embed completion verbs version topics

# Every published page, in the order of the table of contents, README.md first:
# it is the home page. The files in NO_PUBLISH are not pages.
PAGE_ORDER = README.md user_manual.md kb.1.md $(addprefix kb-,$(addsuffix .1.md,$(KB_TOPICS))) INSTALL.md INSTALL_NOTES_macOS.md INSTALL_NOTES_Windows.md AGENTS.md CLAUDE.md CHANGES.md about.md agents-projects-layout-feature-request.md concept-tag-retrieval-feature-request.md frontmatter-generator-feature-request.md narrative-documents-feature-request.md wikilink-tagging-feature-request.md

# Internal notes and upstream drafts (issues, pull requests) stay unpublished; the drafts are
# untracked and deleted once submitted. index.md is a copy of README.md.
NO_PUBLISH = TODO.md ISSUE_DRAFT.md PR_DRAFT.md PR_REPLY.md index.md

MD_PAGES = $(filter-out $(NO_PUBLISH),$(shell ls -1 *.md))

MISSING = $(filter-out $(PAGE_ORDER),$(MD_PAGES))

HTML_PAGES = $(patsubst README.html,index.html,$(patsubst %.md,%.html,$(PAGE_ORDER)))

TOC = .well-known/toc.opml

build: check-order $(HTML_PAGES) index.md $(TOC) .nojekyll

check-order:
	@if [ -n "$(MISSING)" ]; then echo "website.mak: add to PAGE_ORDER: $(MISSING)" >&2; exit 1; fi

# Pandoc is optional; without it the existing pages are left as they are.
# Each HTML page names its Markdown page, for a reader that finds the HTML first.
$(HTML_PAGES): .FORCE
	@if [ -z "$(PANDOC)" ]; then echo "website: pandoc not found, skipping $@" >&2; else \
		SRC="$(patsubst index.html,README.html,$@)"; SRC="$${SRC%.html}"; \
		PUBLISHED="$${SRC}.md"; if [ "$${SRC}" = "README" ]; then PUBLISHED="index.md"; fi; \
		$(PANDOC) --metadata title="$${SRC}" -V markdown-source="$${PUBLISHED}" \
			-s --to html5 "$${SRC}.md" -o "$@" \
			--lua-filter=links-to-html.lua \
			--lua-filter=add-col-scope.lua \
			--template=page.tmpl; fi

# A BonBon browser asks for the site's address and gets index.md.
index.md: README.md
	cp README.md index.md

# GitHub Pages hides folders that start with a dot (and so /.well-known/)
# unless the site says it is not a Jekyll site.
.nojekyll:
	touch .nojekyll

$(TOC): $(MD_PAGES) toc.bash
	mkdir -p $(dir $(TOC))
	./toc.bash $(BASE_URL) $(PROJECT) $(PAGE_ORDER) > $(TOC).tmp
	mv $(TOC).tmp $(TOC)

clean:
	@rm -f $(HTML_PAGES)

.FORCE:
