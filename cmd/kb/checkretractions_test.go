package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

// `source check-retractions` used to print "[skip]" for a source it could not
// look up, exit 0, and say it had checked them all. Now it checks every source,
// prints the counts, and exits 69 (unavailable) when any lookup failed, so a
// script never reads "no retractions" from a service that was not reached.

func withChecker(t *testing.T, f func(doi string) (bool, string, error)) {
	t.Helper()
	old := retractionChecker
	retractionChecker = f
	t.Cleanup(func() { retractionChecker = old })
}

func twoDOISources(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb := openTestKB(t)
	kb.AddSource(knowledge.Source{Title: "Unreachable", IdentifierType: "doi", IdentifierValue: "10.1/a"})
	kb.AddSource(knowledge.Source{Title: "Fine", IdentifierType: "doi", IdentifierValue: "10.1/b"})
	return kb
}

func TestCheckRetractions_UnreachableServiceExits69AfterCheckingTheRest(t *testing.T) {
	withChecker(t, func(doi string) (bool, string, error) {
		if doi == "10.1/a" {
			return false, "", &url.Error{Op: "Get", URL: "https://x", Err: errors.New("no route to host")}
		}
		return false, "", nil
	})
	kb := twoDOISources(t)
	var out bytes.Buffer
	err := cmdSource(kb, nil, false, []string{"check-retractions"}, &out)
	if err == nil {
		t.Fatal("want an error when a lookup failed")
	}
	if got := exitCodeFor(err); got != classUnavailable {
		t.Errorf("class = %v, want unavailable (69)", got)
	}
	for _, want := range []string{"[skip]", "10.1/a", "[ok]", "10.1/b", "could not be checked"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q should mention %q", out.String(), want)
		}
	}
}

func TestCheckRetractions_EveryLookupWorkingExitsZero(t *testing.T) {
	withChecker(t, func(string) (bool, string, error) { return false, "", nil })
	kb := twoDOISources(t)
	var out bytes.Buffer
	if err := cmdSource(kb, nil, false, []string{"check-retractions"}, &out); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "Checked 2") {
		t.Errorf("output = %q", out.String())
	}
}

func TestCheckRetractions_JSONCarriesTheFailureCount(t *testing.T) {
	withChecker(t, func(string) (bool, string, error) { return false, "", errors.New("down") })
	kb := twoDOISources(t)
	var out, errOut bytes.Buffer
	code := dispatch(verbs, kb, nil, true, []string{"source", "check-retractions"}, &out, &errOut)
	if code != 69 {
		t.Errorf("exit %d, want 69; stderr %q", code, errOut.String())
	}
	var res struct {
		Checked, Updated, Failed int
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout is not JSON: %q", out.String())
	}
	if res.Failed != 2 || res.Checked != 0 {
		t.Errorf("result = %+v, want failed 2 and checked 0", res)
	}
	var env struct{ Class string }
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil || env.Class != "unavailable" {
		t.Errorf("stderr = %q, want an unavailable envelope", errOut.String())
	}
}
