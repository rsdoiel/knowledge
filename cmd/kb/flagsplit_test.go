package main

import (
	"errors"
	"flag"
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
