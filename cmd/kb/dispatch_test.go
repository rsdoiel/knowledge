package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	knowledge "github.com/rsdoiel/knowledge"
)

func TestDispatch_UnknownVerbReturnsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := dispatch(map[string]verbFunc{}, nil, nil, false, []string{"bogus"}, &out, &errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), `unknown verb "bogus"`) {
		t.Errorf("errOut = %q, want it to mention the unknown verb", errOut.String())
	}
}

func TestDispatch_CallsMatchedVerbWithRemainingArgs(t *testing.T) {
	var gotArgs []string
	var gotJSON bool
	verbs := map[string]verbFunc{
		"stub": func(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
			gotArgs = args
			gotJSON = jsonOut
			return nil
		},
	}
	var out, errOut bytes.Buffer
	code := dispatch(verbs, nil, nil, true, []string{"stub", "arg1", "arg2"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !gotJSON {
		t.Error("expected jsonOut=true to reach the handler")
	}
	if len(gotArgs) != 2 || gotArgs[0] != "arg1" || gotArgs[1] != "arg2" {
		t.Errorf("handler args = %v, want [arg1 arg2]", gotArgs)
	}
}

func TestDispatch_HandlerErrorReturnsExitCode1(t *testing.T) {
	verbs := map[string]verbFunc{
		"boom": func(kb *knowledge.KnowledgeBase, dl *DebugLog, jsonOut bool, args []string, out io.Writer) error {
			return errors.New("boom")
		},
	}
	var out, errOut bytes.Buffer
	code := dispatch(verbs, nil, nil, false, []string{"boom"}, &out, &errOut)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "boom") {
		t.Errorf("errOut = %q, want it to contain the handler's error", errOut.String())
	}
}
