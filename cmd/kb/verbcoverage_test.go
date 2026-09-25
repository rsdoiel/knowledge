package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// X4 of exit-codes-plan.md. DR-0040 admitted that nothing forced a new verb to
// classify its errors: "the guarantee is the survey", and a verb returning a plain
// fmt.Errorf for a bad argument would exit 1 again until someone noticed. This
// test makes the guarantee permanent. It derives the registry of command paths
// (verb, and subverb where there is one) from each verb's SYNOPSIS, so a new
// documented command is covered without touching this file, and it checks that
// registry against the verbs actually registered. Every path is then run with a
// bogus flag, and with a surplus argument, and must exit 2. Nothing may exit 70.
//
// A path whose arguments end in free text (an observation body, a project
// description, a retraction note, a search term) takes trailing words as they are,
// dashes included (DR-0041 item 2), so a surplus word or a trailing flag-looking
// word is text there, not a mistake. Those paths are exempt from the "trailing"
// checks and still get the flag-first check, where a bogus flag is refused.

type commandPath struct {
	words     []string // ["document", "review", "list"]
	positions []string // required positionals, as placeholders: ["NAME"]
	flags     [][2]string
	// freeText is true when any placeholder is trailing free text.
	freeText bool
	line     string
}

var (
	wordRE        = regexp.MustCompile(`^[a-z][a-z-]*$`)
	placeholderRE = regexp.MustCompile(`^[A-Z][A-Z_]*(\.\.\.)?$`)
	freeTextNames = map[string]bool{"BODY": true, "DESCRIPTION": true, "NOTE": true, "TERM": true}
)

// synopsisPaths returns the command paths documented in a verb's help page.
func synopsisPaths(t *testing.T, verb string) []commandPath {
	t.Helper()
	var buf bytes.Buffer
	if !printHelp(&buf, verb) {
		t.Fatalf("no help page for verb %q", verb)
	}
	var paths []commandPath
	inSynopsis := false
	for _, line := range strings.Split(buf.String(), "\n") {
		switch {
		case strings.HasPrefix(line, "# SYNOPSIS"):
			inSynopsis = true
			continue
		case strings.HasPrefix(line, "# ") && inSynopsis:
			inSynopsis = false
		}
		if !inSynopsis || !strings.HasPrefix(line, "kb ") {
			continue
		}
		p := parseSynopsisLine(line)
		if len(p.words) > 0 && p.words[0] == verb {
			paths = append(paths, p)
		}
	}
	return paths
}

// parseSynopsisLine splits `kb VERB [SUB] ARGS...` into the command path and the
// arguments that are not optional: placeholders and flags outside [] and ().
// Where a required choice is given, `(--project P | --workspace)`, the first
// alternative is taken.
func parseSynopsisLine(line string) commandPath {
	p := commandPath{line: line}
	fields := strings.Fields(strings.TrimPrefix(line, "kb "))
	i := 0
	for i < len(fields) && wordRE.MatchString(fields[i]) {
		p.words = append(p.words, fields[i])
		i++
	}
	depth := 0
	inGroup, groupDone := false, false
	var tokens []string
	for ; i < len(fields); i++ {
		tok := fields[i]
		startsGroup := strings.HasPrefix(tok, "(")
		if depth == 0 && startsGroup {
			inGroup, groupDone = true, false
			tok = strings.TrimPrefix(tok, "(")
		}
		clean := strings.Trim(tok, "[]()")
		if clean == "|" {
			groupDone = true
		}
		if placeholderRE.MatchString(clean) {
			name := strings.TrimSuffix(clean, "...")
			if freeTextNames[name] || strings.HasSuffix(clean, "...") {
				p.freeText = true
			}
		}
		opensBracket := strings.Count(tok, "[")
		closesBracket := strings.Count(tok, "]")
		if depth == 0 && opensBracket == 0 && (!inGroup || !groupDone) && clean != "" && clean != "|" {
			tokens = append(tokens, clean)
		}
		depth += opensBracket - closesBracket
		if inGroup && strings.HasSuffix(tok, ")") {
			inGroup = false
		}
	}
	for j := 0; j < len(tokens); j++ {
		tok := tokens[j]
		switch {
		case strings.HasPrefix(tok, "-"):
			if j+1 < len(tokens) && placeholderRE.MatchString(tokens[j+1]) {
				p.flags = append(p.flags, [2]string{tok, tokens[j+1]})
				j++
			} else {
				p.flags = append(p.flags, [2]string{tok, ""})
			}
		case placeholderRE.MatchString(tok):
			p.positions = append(p.positions, strings.TrimSuffix(tok, "..."))
		}
	}
	return p
}

var idPlaceholders = map[string]bool{"ID": true, "OBS_ID": true, "SOURCE_ID": true, "SECTION_ID": true, "RECORD_ID": true}

// minimalArgs is the shortest well-formed invocation of a path: its required
// flags with dummy values, then its required positionals.
func (p commandPath) minimalArgs() []string {
	args := append([]string{}, p.words...)
	for _, f := range p.flags {
		args = append(args, f[0])
		if f[1] != "" {
			args = append(args, "x"+strings.TrimLeft(f[0], "-"))
		}
	}
	for _, pos := range p.positions {
		if idPlaceholders[pos] {
			args = append(args, "1")
		} else {
			args = append(args, "x"+strings.ToLower(pos))
		}
	}
	return args
}

func allCommandPaths(t *testing.T) []commandPath {
	t.Helper()
	var verbNames []string
	for v := range verbs {
		verbNames = append(verbNames, v)
	}
	sort.Strings(verbNames)
	var paths []commandPath
	seen := map[string]bool{}
	for _, v := range verbNames {
		for _, p := range synopsisPaths(t, v) {
			key := strings.Join(p.words, " ") + "|" + p.line
			if !seen[key] {
				seen[key] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func runPath(t *testing.T, args []string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	// merge, index and init refuse --db; every other verb needs a database.
	pre := []string{"--json"}
	switch args[0] {
	case "merge", "index", "init":
	default:
		pre = append(pre, "--db", filepath.Join(dir, "kb.db"))
	}
	var out, errOut bytes.Buffer
	code := mainRun(append(pre, args...), &out, &errOut)
	return code, errOut.String()
}

// The registry is complete: every registered verb is documented with at least
// one synopsis path, and every documented path names a registered verb. A verb
// without a documented path would escape the checks below, so it is a failure.
func TestCommandRegistry_MatchesTheRegisteredVerbs(t *testing.T) {
	documented := map[string]bool{}
	for _, p := range allCommandPaths(t) {
		documented[p.words[0]] = true
	}
	for v := range verbs {
		if !documented[v] {
			t.Errorf("verb %q is registered but no SYNOPSIS documents a path for it, so the exit-code checks cannot reach it", v)
		}
	}
	if n := len(allCommandPaths(t)); n < 40 {
		t.Errorf("only %d command paths were found in the help pages; the parser has drifted from the synopsis format", n)
	}
}

func TestParseSynopsisLine(t *testing.T) {
	for _, tc := range []struct {
		line      string
		words     []string
		positions []string
		flags     [][2]string
		freeText  bool
	}{
		{"kb project show NAME", []string{"project", "show"}, []string{"NAME"}, nil, false},
		{"kb project list", []string{"project", "list"}, nil, nil, false},
		{"kb project add [--status concept|active] NAME [DESCRIPTION]", []string{"project", "add"}, []string{"NAME"}, nil, true},
		{"kb observation add --project NAME KIND BODY [--source-doi DOI]", []string{"observation", "add"}, []string{"KIND", "BODY"}, [][2]string{{"--project", "NAME"}}, true},
		{"kb document review list [--project P] [--status S]", []string{"document", "review", "list"}, nil, nil, false},
		{"kb merge -a PATH -b PATH -out PATH [-force]", []string{"merge"}, nil, [][2]string{{"-a", "PATH"}, {"-b", "PATH"}, {"-out", "PATH"}}, false},
		{"kb record new --title T --trigger G (--project P | --workspace) [--kind K]", []string{"record", "new"}, nil, [][2]string{{"--title", "T"}, {"--trigger", "G"}, {"--project", "P"}}, false},
		{"kb observation update ID BODY...", []string{"observation", "update"}, []string{"ID", "BODY"}, nil, true},
		{"kb index ROOT --all [--check]", []string{"index"}, []string{"ROOT"}, [][2]string{{"--all", ""}}, false},
		{"kb init [PATH]", []string{"init"}, nil, nil, false},
	} {
		got := parseSynopsisLine(tc.line)
		if strings.Join(got.words, " ") != strings.Join(tc.words, " ") ||
			strings.Join(got.positions, ",") != strings.Join(tc.positions, ",") ||
			len(got.flags) != len(tc.flags) || got.freeText != tc.freeText {
			t.Errorf("%q: got words %v positions %v flags %v freeText %v; want %v %v %v %v",
				tc.line, got.words, got.positions, got.flags, got.freeText, tc.words, tc.positions, tc.flags, tc.freeText)
			continue
		}
		for i := range tc.flags {
			if got.flags[i] != tc.flags[i] {
				t.Errorf("%q: flag %d = %v, want %v", tc.line, i, got.flags[i], tc.flags[i])
			}
		}
	}
}

// A bogus flag straight after the command path is refused by every command,
// free-text ones included: only the words after the fixed arguments are text.
func TestEveryCommand_BogusFlagFirstIsUsage(t *testing.T) {
	for _, p := range allCommandPaths(t) {
		p := p
		t.Run(strings.Join(p.words, "_"), func(t *testing.T) {
			args := append(append([]string{}, p.words...), "--bogus-flag-zzz")
			args = append(args, p.minimalArgs()[len(p.words):]...)
			code, errOut := runPath(t, args)
			if code != 2 {
				t.Errorf("kb %s: exit %d, want 2 (usage); stderr %.160s", strings.Join(args, " "), code, errOut)
			}
		})
	}
}

// A bogus flag after a complete, valid invocation is refused, not dropped: the
// mistake DR-0041 found in 15 verbs.
func TestEveryCommand_TrailingBogusFlagIsUsage(t *testing.T) {
	for _, p := range allCommandPaths(t) {
		if p.freeText {
			continue // trailing words are text here (DR-0041 item 2)
		}
		p := p
		t.Run(strings.Join(p.words, "_"), func(t *testing.T) {
			args := append(p.minimalArgs(), "--bogus-flag-zzz")
			code, errOut := runPath(t, args)
			if code != 2 {
				t.Errorf("kb %s: exit %d, want 2 (usage); stderr %.160s", strings.Join(args, " "), code, errOut)
			}
		})
	}
}

// A surplus argument after a complete, valid invocation is refused, not dropped:
// the other half of DR-0041's survey (19 verbs). If a command ignored it and
// carried on, the dummy values would give not-found (1) or success (0), never 2.
func TestEveryCommand_SurplusArgumentIsUsage(t *testing.T) {
	for _, p := range allCommandPaths(t) {
		if p.freeText {
			continue
		}
		p := p
		t.Run(strings.Join(p.words, "_"), func(t *testing.T) {
			// Two surplus words: init takes one optional PATH, so one would be it.
			args := append(p.minimalArgs(), "surplus-zzz", "surplus2-zzz")
			code, errOut := runPath(t, args)
			if code != 2 {
				t.Errorf("kb %s: exit %d, want 2 (usage); stderr %.160s", strings.Join(args, " "), code, errOut)
			}
		})
	}
}

// Nothing in the registry reaches an unclassified error: run every path in its
// minimal form and assert the exit is never 70. (The result may be 0, 1, 2, 65 or
// 66 depending on the dummy values; 70 is always a gap.)
func TestEveryCommand_MinimalInvocationIsNeverInternal(t *testing.T) {
	for _, p := range allCommandPaths(t) {
		p := p
		t.Run(strings.Join(p.words, "_"), func(t *testing.T) {
			code, errOut := runPath(t, p.minimalArgs())
			if code == 70 {
				t.Errorf("kb %s: exit 70, an error nothing classified: %.200s", strings.Join(p.minimalArgs(), " "), errOut)
			}
		})
	}
}
