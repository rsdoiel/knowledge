package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestPrintJSON_ValidOutput(t *testing.T) {
	var out bytes.Buffer
	type row struct {
		Name string `json:"name"`
	}
	if err := printJSON(&out, row{Name: "alpha"}); err != nil {
		t.Fatalf("printJSON: %v", err)
	}
	var got row
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, out.String())
	}
	if got.Name != "alpha" {
		t.Errorf("got.Name = %q, want %q", got.Name, "alpha")
	}
}

func TestPrintError_TextMode(t *testing.T) {
	var out, errOut bytes.Buffer
	printError(&out, &errOut, false, errors.New("something broke"))
	if errOut.String() != "kb: something broke\n" {
		t.Errorf("errOut = %q, want the one plain line", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want nothing in text mode", out.String())
	}
}

// In JSON mode the human line still goes to standard error, and the error is
// also an object on standard output (workspace DR-0011).
func TestPrintError_JSONModeIsPlainOnStderrAndAnObjectOnStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	printError(&out, &errOut, true, usageErrorf("something broke"))
	if errOut.String() != "kb: something broke\n" {
		t.Errorf("errOut = %q, want the plain line, not JSON", errOut.String())
	}
	var envelope struct {
		Error string `json:"error"`
		Class string `json:"class"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("out is not one JSON object: %v (%q)", err, out.String())
	}
	if envelope.Error != "something broke" || envelope.Class != "usage" || envelope.Code != 2 {
		t.Errorf("envelope = %+v, want error, class usage, code 2", envelope)
	}
}
