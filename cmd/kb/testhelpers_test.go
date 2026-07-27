package main

import (
	"encoding/json"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func assertValidJSON(t *testing.T, data []byte) {
	t.Helper()
	if !json.Valid(data) {
		t.Errorf("output is not valid JSON: %q", data)
	}
}

func openTestKB(t *testing.T) *knowledge.KnowledgeBase {
	t.Helper()
	kb, err := knowledge.Open(knowledge.DefaultPath(t.TempDir()))
	if err != nil {
		t.Fatalf("knowledge.Open: %v", err)
	}
	t.Cleanup(func() { kb.Close() })
	return kb
}

func sourceStub(title string) knowledge.Source {
	return knowledge.Source{Title: title}
}
