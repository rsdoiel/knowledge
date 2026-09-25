package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"testing"
)

func TestSplitFlags_PositionalBeforeFlags(t *testing.T) {
	var project string
	var dryRun bool
	positional, err := splitFlags(
		[]string{"a.md", "--project", "alpha", "--dry-run"},
		map[string]*string{"--project": &project},
		map[string]*bool{"--dry-run": &dryRun},
	)
	if err != nil {
		t.Fatalf("splitFlags: %v", err)
	}
	if project != "alpha" || !dryRun {
		t.Errorf("project=%q dryRun=%v", project, dryRun)
	}
	if !reflect.DeepEqual(positional, []string{"a.md"}) {
		t.Errorf("positional = %v, want [a.md]", positional)
	}
}

func TestSplitFlags_FlagsBeforePositional(t *testing.T) {
	var project string
	positional, err := splitFlags(
		[]string{"--project", "alpha", "a.md"},
		map[string]*string{"--project": &project}, nil,
	)
	if err != nil {
		t.Fatalf("splitFlags: %v", err)
	}
	if project != "alpha" || !reflect.DeepEqual(positional, []string{"a.md"}) {
		t.Errorf("project=%q positional=%v", project, positional)
	}
}

func TestSplitFlags_InterleavedFlagsAndPositionals(t *testing.T) {
	var a, b string
	positional, err := splitFlags(
		[]string{"one", "--a", "1", "two", "--b", "2", "three"},
		map[string]*string{"--a": &a, "--b": &b}, nil,
	)
	if err != nil {
		t.Fatalf("splitFlags: %v", err)
	}
	if a != "1" || b != "2" {
		t.Errorf("a=%q b=%q", a, b)
	}
	if !reflect.DeepEqual(positional, []string{"one", "two", "three"}) {
		t.Errorf("positional = %v", positional)
	}
}

func TestSplitFlags_UnknownFlagErrors(t *testing.T) {
	_, err := splitFlags([]string{"--nope"}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for an unrecognized flag")
	}
}

func TestSplitFlags_MissingValueErrors(t *testing.T) {
	var project string
	_, err := splitFlags([]string{"--project"}, map[string]*string{"--project": &project}, nil)
	if err == nil {
		t.Fatal("expected an error for a flag missing its value")
	}
}

func TestSplitFlags_NilBoolFlagsIsSafe(t *testing.T) {
	positional, err := splitFlags([]string{"a"}, nil, nil)
	if err != nil {
		t.Fatalf("splitFlags: %v", err)
	}
	if !reflect.DeepEqual(positional, []string{"a"}) {
		t.Errorf("positional = %v", positional)
	}
}

func TestSplitFlags_NoArgsReturnsNoPositionals(t *testing.T) {
	positional, err := splitFlags(nil, nil, nil)
	if err != nil {
		t.Fatalf("splitFlags: %v", err)
	}
	if len(positional) != 0 {
		t.Errorf("positional = %v, want none", positional)
	}
}

// A bare help flag that is not a declared flag is a help request, reported as
// flag.ErrHelp so dispatch can answer with the verb's page. Any other unknown
// flag is still an ordinary error.
func TestSplitFlags_HelpFlagReportsErrHelp(t *testing.T) {
	for _, h := range []string{"-h", "-help", "--help"} {
		_, err := splitFlags([]string{"--dry-run", h}, nil, map[string]*bool{"--dry-run": new(bool)})
		if !errors.Is(err, flag.ErrHelp) {
			t.Errorf("splitFlags(--dry-run %s) error = %v, want flag.ErrHelp", h, err)
		}
	}
	if _, err := splitFlags([]string{"--bogus"}, nil, nil); err == nil || errors.Is(err, flag.ErrHelp) {
		t.Errorf("splitFlags(--bogus) error = %v, want an ordinary unknown-flag error", err)
	}
}

// `--` ends flag recognition, as it does for the verbs that already handle it
// (DR-0039, DR-0041): everything after it is positional, dashes included. Without
// it a source title or an ingest path that begins with a dash could not be given.
func TestSplitFlags_DoubleDashEndsFlags(t *testing.T) {
	var by string
	var force bool
	str := map[string]*string{"--by": &by}
	boo := map[string]*bool{"--force": &force}
	for _, tc := range []struct {
		name  string
		args  []string
		want  []string
		by    string
		force bool
	}{
		{"dash-leading value", []string{"--", "-1 considered harmful"}, []string{"-1 considered harmful"}, "", false},
		{"flags before it still apply", []string{"--by", "me", "--force", "--", "-x"}, []string{"-x"}, "me", true},
		{"a real flag after it is positional", []string{"a", "--", "--force", "--by"}, []string{"a", "--force", "--by"}, "", false},
		{"only the first one is consumed", []string{"--", "--", "b"}, []string{"--", "b"}, "", false},
		{"nothing after it", []string{"a", "--"}, []string{"a"}, "", false},
		{"help after it is positional", []string{"--", "--help"}, []string{"--help"}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			by, force = "", false
			got, err := splitFlags(tc.args, str, boo)
			if err != nil {
				t.Fatalf("splitFlags(%v) = %v", tc.args, err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("positional = %q, want %q", got, tc.want)
			}
			if by != tc.by || force != tc.force {
				t.Errorf("by=%q force=%v, want by=%q force=%v", by, force, tc.by, tc.force)
			}
		})
	}
}

// A flag's value is consumed before `--` is looked at, so `--by --` gives the
// flag the value "--", as any other string would.
func TestSplitFlags_DoubleDashAsFlagValue(t *testing.T) {
	var by string
	got, err := splitFlags([]string{"--by", "--", "x"}, map[string]*string{"--by": &by}, nil)
	if err != nil || by != "--" || fmt.Sprint(got) != "[x]" {
		t.Errorf("got %v, %v, by=%q; want [x], nil, by=--", got, err, by)
	}
}

func TestCmdSource_AddAcceptsDashLeadingTitleAfterDoubleDash(t *testing.T) {
	kb := openTestKB(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"add", "--", "-1 considered harmful"}, &out); err != nil {
		t.Fatalf("source add -- TITLE: %v", err)
	}
	sources, _ := kb.ListSources()
	if len(sources) != 1 || sources[0].Title != "-1 considered harmful" {
		t.Errorf("sources = %+v, want one titled \"-1 considered harmful\"", sources)
	}
	// Without `--` it is still a mistyped flag.
	out.Reset()
	if err := cmdSource(kb, nil, false, []string{"add", "-1 considered harmful"}, &out); err == nil {
		t.Error("a dash-leading title without -- must still be refused as an unknown flag")
	}
}
