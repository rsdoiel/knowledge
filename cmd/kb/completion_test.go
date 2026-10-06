package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func completionScript(t *testing.T, shell string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", shell}, &out, &errOut); code != 0 {
		t.Fatalf("kb completion %s: exit %d, stderr %q", shell, code, errOut.String())
	}
	return out.String()
}

// The verb list is read from the verbs map, never typed a second time, so a
// verb added to kb is completed without anyone remembering to say so.
func TestCompletion_EveryRegisteredVerbIsCompleted(t *testing.T) {
	for _, shell := range []string{"bash", "powershell"} {
		script := completionScript(t, shell)
		for verb := range verbs {
			if !strings.Contains(script, verb) {
				t.Errorf("%s: registered verb %q is not in the completion script", shell, verb)
			}
		}
		for _, v := range []string{"help", "completion"} {
			if !strings.Contains(script, v) {
				t.Errorf("%s: %q missing from the completion script", shell, v)
			}
		}
	}
}

// Every verb that takes a subverb must have its subverbs listed, and the list
// must be the one the verb itself reports when run bare.
var usageList = regexp.MustCompile(`usage: [a-z ]+ <([a-z|-]+)>`)
var requiresList = regexp.MustCompile(`requires a subverb: ([a-z, -]+)`)

// subverbsReported reads the subverb list out of a verb's bare-invocation error,
// which comes in two phrasings.
func subverbsReported(msg string) []string {
	if m := usageList.FindStringSubmatch(msg); m != nil {
		return strings.Split(m[1], "|")
	}
	if m := requiresList.FindStringSubmatch(msg); m != nil {
		return strings.FieldsFunc(strings.ReplaceAll(m[1], " or ", ","), func(r rune) bool { return r == ',' || r == ' ' })
	}
	return nil
}

func TestCompletion_SubverbsMatchWhatTheVerbsAccept(t *testing.T) {
	for verb := range verbsWithSubverbs {
		listed := completionSubverbs[verb]
		if len(listed) == 0 {
			t.Errorf("verb %q has subverbs but none are listed for completion", verb)
			continue
		}
		var out, errOut bytes.Buffer
		dispatch(verbs, nil, nil, false, []string{verb}, &out, &errOut)
		want := subverbsReported(errOut.String())
		if want == nil {
			t.Errorf("verb %q: bare invocation gave no usage list to compare against: %q", verb, errOut.String())
			continue
		}
		got := append([]string(nil), listed...)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("verb %q: completion lists %v, the verb's usage says %v", verb, got, want)
		}
	}
	for key := range completionSubverbs {
		verb, _, _ := strings.Cut(key, " ")
		if !verbsWithSubverbs[verb] {
			t.Errorf("completionSubverbs has %q but %q has no subverbs", key, verb)
		}
	}
}

var sourceFlagPatterns = []*regexp.Regexp{
	regexp.MustCompile(`fs\.(?:String|Bool|Int|Float64|Duration)\("([a-z][a-z-]*)"`),
	regexp.MustCompile(`fs\.Var\([^,]+,\s*"([a-z][a-z-]*)"`),
	regexp.MustCompile(`"(--[a-z][a-z-]*)"\s*(?::|,|\))`),
	regexp.MustCompile(`case "(--[a-z][a-z-]*)"`),
}

// sourceFlags returns the flag names the verb source files declare, spelled
// --name. A flag added in code but not to completionVerbFlags fails here.
func sourceFlags(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	skip := map[string]bool{"main.go": true, "usage.go": true, "flagsplit.go": true, "completion.go": true}
	found := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || skip[f] {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, re := range sourceFlagPatterns {
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				name := "--" + strings.TrimPrefix(m[1], "--")
				found[name] = f
			}
		}
	}
	return found
}

func TestCompletion_FlagsMatchTheSource(t *testing.T) {
	listed := map[string]bool{}
	for verb, flags := range completionVerbFlags {
		if _, ok := verbs[verb]; !ok && verb != "completion" {
			t.Errorf("completionVerbFlags names %q, which is not a verb", verb)
		}
		for _, f := range flags {
			listed[f] = true
		}
	}
	src := sourceFlags(t)
	for name, file := range src {
		if !listed[name] {
			t.Errorf("%s declares %s but completionVerbFlags does not list it", file, name)
		}
	}
	for name := range listed {
		if _, ok := src[name]; !ok && name != "--install" {
			t.Errorf("completionVerbFlags lists %s but no verb source declares it", name)
		}
	}
}

func TestCompletion_BadShellAndBadArgsAreUsageErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"completion"},
		{"completion", "fish"},
		{"completion", "bash", "extra"},
		{"completion", "bash", "--bogus"},
	} {
		var out, errOut bytes.Buffer
		if code := mainRun(args, &out, &errOut); code != 2 {
			t.Errorf("kb %v: exit %d, want 2 (stderr %q)", args, code, errOut.String())
		}
		if out.Len() != 0 {
			t.Errorf("kb %v wrote to stdout: %q", args, out.String())
		}
	}
	if _, err := os.Stat("agents"); err == nil {
		t.Error("kb completion created ./agents; it must not open a database")
	}
}

func TestCompletion_DBOptionIsRefused(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"--db", "x.db", "completion", "bash"}, &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2 (stderr %q)", code, errOut.String())
	}
}

func TestCompletion_PwshAndCaseAreAccepted(t *testing.T) {
	for _, shell := range []string{"pwsh", "PowerShell", "BASH"} {
		var out, errOut bytes.Buffer
		if code := mainRun([]string{"completion", shell}, &out, &errOut); code != 0 {
			t.Errorf("kb completion %s: exit %d", shell, code)
		}
	}
}

func TestCompletion_InstallBash(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	path := filepath.Join(data, "bash-completion", "completions", "kb")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("script not written: %v", err)
	}
	if string(got) != completionScript(t, "bash") {
		t.Error("installed script differs from `kb completion bash`")
	}
	if !strings.Contains(out.String(), path) {
		t.Errorf("stdout %q does not say where the script went", out.String())
	}
	// Reinstalling over our own file is fine.
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 0 {
		t.Errorf("reinstall: exit %d", code)
	}
}

func TestCompletion_InstallBashFallsBackToHomeLocalShare(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "bash-completion", "completions", "kb")); err != nil {
		t.Errorf("script not in the default location: %v", err)
	}
}

func TestCompletion_InstallBashNeverOverwritesAForeignFile(t *testing.T) {
	data := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", data)
	path := filepath.Join(data, "bash-completion", "completions", "kb")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("# someone else's\n"), 0o644)
	var out, errOut bytes.Buffer
	if code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut); code != 73 {
		t.Errorf("exit %d, want 73 (cant_create); stderr %q", code, errOut.String())
	}
	if b, _ := os.ReadFile(path); string(b) != "# someone else's\n" {
		t.Errorf("foreign file was overwritten: %q", b)
	}
}

func TestCompletion_InstallBashUnwritableDirIsAnIOClass(t *testing.T) {
	data := t.TempDir()
	blocker := filepath.Join(data, "bash-completion")
	os.WriteFile(blocker, []byte("a file where a directory must go"), 0o644)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", data)
	var out, errOut bytes.Buffer
	code := mainRun([]string{"completion", "bash", "-install"}, &out, &errOut)
	if code != 73 && code != 74 {
		t.Errorf("exit %d, want 73 or 74; stderr %q", code, errOut.String())
	}
}

func TestCompletion_InstallPowerShellAddsOneProfileLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "powershell")
	profile := filepath.Join(dir, "Microsoft.PowerShell_profile.ps1")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(profile, []byte("# my profile"), 0o644) // no trailing newline
	for i := 0; i < 2; i++ {
		var out, errOut bytes.Buffer
		if code := mainRun([]string{"completion", "powershell", "-install"}, &out, &errOut); code != 0 {
			t.Fatalf("install %d: exit %d, stderr %q", i, code, errOut.String())
		}
	}
	script := filepath.Join(dir, "kb-completion.ps1")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("script not written: %v", err)
	}
	b, _ := os.ReadFile(profile)
	if n := strings.Count(string(b), script); n != 1 {
		t.Errorf("profile mentions the script %d times, want 1:\n%s", n, b)
	}
	if !strings.HasPrefix(string(b), "# my profile\n") {
		t.Errorf("existing profile content was not preserved: %q", b)
	}
}

func TestCompletion_HelpPage(t *testing.T) {
	var out bytes.Buffer
	if !printHelp(&out, "completion") || out.Len() == 0 {
		t.Fatal("kb help completion has no page")
	}
	for _, want := range []string{"bash", "powershell", "-install", "EXIT STATUS"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("completion page does not mention %q", want)
		}
	}
}

// bashComplete sources the generated script in a real bash and returns what
// completing the given words would offer.
func bashComplete(t *testing.T, dir string, words ...string) []string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	script := filepath.Join(dir, "kb.bash")
	os.WriteFile(script, []byte(completionScript(t, "bash")), 0o644)
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = "'" + w + "'"
	}
	prog := `source ` + script + `
COMP_WORDS=(` + strings.Join(quoted, " ") + `)
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
_kb
printf '%s\n' "${COMPREPLY[@]}"`
	cmd := exec.Command(bash, "-c", prog)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, b)
	}
	return strings.Fields(string(b))
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestCompletion_BashBehaviour(t *testing.T) {
	dir := t.TempDir()
	if got := bashComplete(t, dir, "kb", "pro"); len(got) != 1 || got[0] != "project" {
		t.Errorf("kb pro<TAB> = %v, want [project]", got)
	}
	if got := bashComplete(t, dir, "kb", "project", "se"); !has(got, "set-status") || !has(got, "set-description") {
		t.Errorf("kb project se<TAB> = %v", got)
	}
	if got := bashComplete(t, dir, "kb", "document", "review", ""); !has(got, "promote") || has(got, "ingest") {
		t.Errorf("kb document review <TAB> = %v, want review's subverbs", got)
	}
	if got := bashComplete(t, dir, "kb", "--json", "merge", "--"); !has(got, "--force") || !has(got, "--out") {
		t.Errorf("kb merge --<TAB> = %v", got)
	}
	if got := bashComplete(t, dir, "kb", "completion", ""); !has(got, "bash") || !has(got, "powershell") {
		t.Errorf("kb completion <TAB> = %v", got)
	}
	if got := bashComplete(t, dir, "kb", "help", "rec"); !has(got, "record") {
		t.Errorf("kb help rec<TAB> = %v", got)
	}
	if got := bashComplete(t, dir, "kb", "--"); !has(got, "--db") || !has(got, "--json") {
		t.Errorf("kb --<TAB> = %v, want the global options", got)
	}
}

// --project completes from the database, found the way kb itself would find it.
func TestCompletion_BashCompletesProjectNamesFromTheDatabase(t *testing.T) {
	if _, err := exec.LookPath("kb"); err != nil {
		// The script calls kb by name, as an installed script must.
		t.Skip("kb is not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("kb", args...)
		cmd.Dir = dir
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("kb %v: %v\n%s", args, err, b)
		}
	}
	run("init", ".")
	run("project", "add", "alphaproj")
	got := bashComplete(t, dir, "kb", "observation", "list", "--project", "alp")
	if !has(got, "alphaproj") {
		t.Errorf("--project alp<TAB> = %v, want alphaproj", got)
	}
	got = bashComplete(t, dir, "kb", "project", "show", "alp")
	if !has(got, "alphaproj") {
		t.Errorf("project show alp<TAB> = %v, want alphaproj", got)
	}
	// An unfilled template placeholder would surface as a command-not-found.
	if strings.Contains(completionScript(t, "bash"), "{") && strings.Contains(completionScript(t, "bash"), "{app_func}") {
		t.Error("the bash script still contains an unsubstituted {app_func}")
	}
}

func TestCompletion_PowerShellParses(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("no pwsh")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "kb.ps1")
	os.WriteFile(script, []byte(completionScript(t, "powershell")), 0o644)
	prog := `$e=$null; [void][System.Management.Automation.Language.Parser]::ParseFile('` + script + `',[ref]$null,[ref]$e); if ($e) { $e | Out-String; exit 1 }
. '` + script + `'
$r = TabExpansion2 'kb pro' 6
$r.CompletionMatches.CompletionText -join ' '`
	b, err := exec.Command(pwsh, "-NoProfile", "-Command", prog).CombinedOutput()
	if err != nil {
		t.Fatalf("pwsh: %v\n%s", err, b)
	}
	if !strings.Contains(string(b), "project") {
		t.Errorf("kb pro<TAB> in pwsh gave %q", b)
	}
}
