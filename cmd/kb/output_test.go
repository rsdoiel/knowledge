package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
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
	var errOut bytes.Buffer
	printError(&errOut, false, errors.New("something broke"))
	if !strings.Contains(errOut.String(), "something broke") {
		t.Errorf("errOut = %q, want it to contain the error message", errOut.String())
	}
}

func TestPrintError_JSONMode(t *testing.T) {
	var errOut bytes.Buffer
	printError(&errOut, true, errors.New("something broke"))
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &envelope); err != nil {
		t.Fatalf("errOut is not valid JSON: %v (%q)", err, errOut.String())
	}
	if envelope.Error != "something broke" {
		t.Errorf("envelope.Error = %q, want %q", envelope.Error, "something broke")
	}
}
