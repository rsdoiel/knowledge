package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readJSONLLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening log file: %v", err)
	}
	defer f.Close()

	var records []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("line %q is not valid JSON: %v", scanner.Text(), err)
		}
		records = append(records, record)
	}
	return records
}

func TestLog_WritesOneJSONObjectPerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}

	dl.Log("kb_call", map[string]any{"method": "Projects"})
	dl.Log("kb_call", map[string]any{"method": "AddProject"})

	if err := dl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	records := readJSONLLines(t, path)
	if len(records) != 2 {
		t.Fatalf("got %d lines, want 2: %v", len(records), records)
	}
	if records[0]["event"] != "kb_call" {
		t.Errorf("event = %v, want kb_call", records[0]["event"])
	}
	if records[0]["method"] != "Projects" {
		t.Errorf("method = %v, want Projects", records[0]["method"])
	}
	if _, ok := records[0]["time"]; !ok {
		t.Errorf("record missing a time field: %v", records[0])
	}
}

func TestLog_NilReceiverIsSafe(t *testing.T) {
	var dl *DebugLog
	dl.Log("kb_call", map[string]any{"method": "Projects"})
	if dl.Path() != "" {
		t.Errorf("Path() = %q, want empty", dl.Path())
	}
	if err := dl.Close(); err != nil {
		t.Errorf("Close() on nil = %v, want nil", err)
	}
}

func TestDefaultDebugLogPath_MatchesExpectedFormat(t *testing.T) {
	p := DefaultDebugLogPath()
	if !strings.HasPrefix(p, "kb-debug-") || !strings.HasSuffix(p, ".jsonl") {
		t.Errorf("DefaultDebugLogPath() = %q, want kb-debug-<timestamp>.jsonl", p)
	}
}

// TestDefaultDebugLogPath_UniqueAcrossRapidCalls guards against the real
// bug found during manual verification: a second-only timestamp produces
// the same filename (and NewDebugLog's O_TRUNC silently destroys the
// earlier run's log) when two kb --debug invocations happen within the
// same second -- a likely occurrence for a CLI whose verbs are typically
// many short-lived processes, unlike clasm's long-running sessions.
func TestDefaultDebugLogPath_UniqueAcrossRapidCalls(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		p := DefaultDebugLogPath()
		if seen[p] {
			t.Fatalf("DefaultDebugLogPath() produced a duplicate: %q", p)
		}
		seen[p] = true
	}
}

func TestLogKBCall_LogsMethodParamsAndResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}

	result, err := logKBCall(dl, "Projects", map[string]any{"x": 1}, func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("logKBCall: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want ok (must pass through unchanged)", result)
	}
	dl.Close()

	records := readJSONLLines(t, path)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %v", len(records), records)
	}
	r := records[0]
	if r["method"] != "Projects" {
		t.Errorf("method = %v, want Projects", r["method"])
	}
	if r["result"] != "ok" {
		t.Errorf("result field = %v, want ok", r["result"])
	}
	if _, ok := r["error"]; ok {
		t.Errorf("expected no error field on success, got %v", r["error"])
	}
	if _, ok := r["duration_ms"]; !ok {
		t.Errorf("expected a duration_ms field, got %v", r)
	}
}

func TestLogKBCall_LogsErrorInsteadOfResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}

	wantErr := errors.New("boom")
	_, gotErr := logKBCall(dl, "AddProject", nil, func() (string, error) {
		return "", wantErr
	})
	if gotErr != wantErr {
		t.Errorf("logKBCall error = %v, want it passed through unchanged", gotErr)
	}
	dl.Close()

	records := readJSONLLines(t, path)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %v", len(records), records)
	}
	r := records[0]
	if r["error"] != "boom" {
		t.Errorf("error field = %v, want boom", r["error"])
	}
	if _, ok := r["result"]; ok {
		t.Errorf("expected no result field on error, got %v", r["result"])
	}
}

func TestLogKBCallErr_LogsSuccessAndError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	dl, err := NewDebugLog(path)
	if err != nil {
		t.Fatalf("NewDebugLog: %v", err)
	}

	if err := logKBCallErr(dl, "LinkProjectConcept", map[string]any{"a": 1}, func() error { return nil }); err != nil {
		t.Errorf("logKBCallErr (success case): %v", err)
	}
	wantErr := errors.New("boom")
	if gotErr := logKBCallErr(dl, "LinkProjectConcept", nil, func() error { return wantErr }); gotErr != wantErr {
		t.Errorf("logKBCallErr error = %v, want %v", gotErr, wantErr)
	}
	dl.Close()

	records := readJSONLLines(t, path)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2: %v", len(records), records)
	}
	if _, ok := records[0]["error"]; ok {
		t.Errorf("expected no error field on success, got %v", records[0])
	}
	if records[1]["error"] != "boom" {
		t.Errorf("error field = %v, want boom", records[1]["error"])
	}
}

func TestLogKBCall_NilDebugLogStillReturnsCallResult(t *testing.T) {
	result, err := logKBCall[string](nil, "Projects", nil, func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("logKBCall with nil dl: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want ok", result)
	}
}
