package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// The completion scripts are generated from the verb table (v0.0.19 T0, knowledge
// DR-0059). Moving that data from hand-kept maps into one table changed no
// behaviour, and this characterisation test is the proof: the scripts as they
// were before the move are kept in testdata/ and the generated ones must match
// byte for byte. A deliberate change to completion (a new verb, a new flag)
// regenerates them with `go test ./cmd/kb -run CompletionGolden -update`.

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata/")

func TestCompletionGolden(t *testing.T) {
	for shell, file := range map[string]string{"bash": "completion.bash.golden", "powershell": "completion.ps1.golden"} {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteCompletion(&out, "kb", shell); err != nil {
				t.Fatalf("WriteCompletion(%s): %v", shell, err)
			}
			path := filepath.Join("testdata", file)
			if *updateGolden {
				if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (create it with -update)", err)
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Errorf("the %s completion script differs from %s; if the change is deliberate, regenerate with -update", shell, path)
			}
		})
	}
}
