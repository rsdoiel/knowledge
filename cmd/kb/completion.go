package main

import (
	"bytes"
	"fmt"
	knowledge "github.com/rsdoiel/knowledge"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

func init() {
	verbs["completion"] = cmdCompletion
}

/** completionSubverbs lists, for each verb that takes a subverb, the subverbs
 * completion offers. A key of two words ("document review") is the list for
 * the subverb's own subverbs. It is read from the verb table (verbtable.go);
 * completion_test.go compares each verb's list with the usage line the verb
 * itself reports, so a subverb added to the verb and not to the table fails the
 * build.
 */
var completionSubverbs = subverbMap()

/** completionProjectArgs names the subverbs whose first argument is a project
 * name, which completion fills in from the database. Read from the verb table.
 */
var completionProjectArgs = projectArgMap()

/** completionVerbFlags lists the flags each verb accepts, subverbs included,
 * read from the verb table. The flags are declared in several ways (flag.FlagSet,
 * splitFlags maps, a switch), so the table cannot be generated from the code;
 * TestCompletion_FlagsMatchTheSource scans the source for declared flags and
 * fails when the table and the code disagree.
 */
var completionVerbFlags = flagMap()

// completionGlobalFlags are the options that go before the verb.
var completionGlobalFlags = []string{"--db", "--debug", "--help", "-i", "--json", "--license", "--version"}

// completionBoolFlags take no value; every other flag does, so completion
// skips the word after it when looking for the verb's arguments.
var completionBoolFlags = map[string]bool{
	"--dry-run": true, "--force": true, "--stdout": true, "--check": true, "--all": true,
	"--workspace": true, "--partial": true, "--write": true, "--install": true,
	"--uninstall": true,
}

// completionFileFlags take a path.
var completionFileFlags = []string{"--db", "--in", "--out", "--a", "--b", "--jsonl", "--root", "--dir"}

// completionShells are the shells kb can generate for.
var completionShells = []string{"bash", "powershell"}

// completionTopics are help topics that are not verbs.
var completionTopics = []string{"topics"}

// completionOwnHelp are verbs handled before the verbs map is consulted.
var completionOwnHelp = []string{"help"}

/** completionVerbs returns every verb completion offers, sorted: the registered
 * verbs, read from the verbs map so a new verb is completed without being named
 * here, plus help.
 *
 * Returns:
 *   []string — verb names.
 *
 * Example:
 *   for _, v := range completionVerbs() { fmt.Println(v) }
 */
func completionVerbs() []string {
	names := append([]string(nil), completionOwnHelp...)
	for v := range verbs {
		names = append(names, v)
	}
	sort.Strings(names)
	return names
}

func completionHelpTopics() []string {
	return append(completionVerbs(), completionTopics...)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func valueFlags() []string {
	seen := map[string]bool{}
	for _, flags := range completionVerbFlags {
		for _, f := range flags {
			if !completionBoolFlags[f] {
				seen[f] = true
			}
		}
	}
	for _, f := range completionGlobalFlags {
		if f == "--db" {
			seen[f] = true
		}
	}
	return sortedKeys(seen)
}

func psList(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = "'" + s + "'"
	}
	return strings.Join(q, ", ")
}

var nonIdent = regexp.MustCompile(`[^A-Za-z0-9_]`)

// bashSubverbCases emits the case arms that complete a verb's subverbs and the
// project-name arguments after them.
func bashSubverbCases(projectsFn string) string {
	var b strings.Builder
	for _, verb := range sortedKeys(completionSubverbs) {
		if strings.Contains(verb, " ") {
			continue
		}
		fmt.Fprintf(&b, "        %s)\n", verb)
		fmt.Fprintf(&b, "            if [[ $n -eq 1 ]]; then COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") )\n", strings.Join(completionSubverbs[verb], " "))
		for _, nested := range sortedKeys(completionSubverbs) {
			if v, sub, ok := strings.Cut(nested, " "); ok && v == verb {
				fmt.Fprintf(&b, "            elif [[ $n -eq 2 && \"${pos[1]}\" == %s ]]; then COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") )\n", sub, strings.Join(completionSubverbs[nested], " "))
			}
		}
		if subs := completionProjectArgs[verb]; len(subs) > 0 {
			fmt.Fprintf(&b, "            elif [[ $n -eq 2 && \"${pos[1]}\" =~ ^(%s)$ ]]; then COMPREPLY=( $(compgen -W \"$(%s)\" -- \"$cur\") )\n", strings.Join(subs, "|"), projectsFn)
		}
		b.WriteString("            fi ;;\n")
	}
	return b.String()
}

func bashFlagCases() string {
	var b strings.Builder
	for _, verb := range sortedKeys(completionVerbFlags) {
		fmt.Fprintf(&b, "        %s) COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") ) ;;\n", verb, strings.Join(completionVerbFlags[verb], " "))
	}
	return b.String()
}

func psSubverbCases() string {
	var b strings.Builder
	for _, verb := range sortedKeys(completionSubverbs) {
		if strings.Contains(verb, " ") {
			continue
		}
		fmt.Fprintf(&b, "            '%s' {\n", verb)
		fmt.Fprintf(&b, "                if ($n -eq 1) { @(%s) }\n", psList(completionSubverbs[verb]))
		for _, nested := range sortedKeys(completionSubverbs) {
			if v, sub, ok := strings.Cut(nested, " "); ok && v == verb {
				fmt.Fprintf(&b, "                elseif ($n -eq 2 -and $pos[1] -eq '%s') { @(%s) }\n", sub, psList(completionSubverbs[nested]))
			}
		}
		if subs := completionProjectArgs[verb]; len(subs) > 0 {
			fmt.Fprintf(&b, "                elseif ($n -eq 2 -and $pos[1] -in %s) { Get-KbProjects }\n", psList(subs))
		}
		b.WriteString("                else { $null }\n            }\n")
	}
	return b.String()
}

func psFlagCases() string {
	var b strings.Builder
	for _, verb := range sortedKeys(completionVerbFlags) {
		fmt.Fprintf(&b, "            '%s' { @(%s) }\n", verb, psList(completionVerbFlags[verb]))
	}
	return b.String()
}

/** WriteCompletion writes a shell completion script for the named shell. The
 * verbs come from the registered verbs, the subverbs and flags from the tables
 * above. Nothing is written when the shell is not supported.
 *
 * Parameters:
 *   w       (io.Writer) — destination for the script.
 *   appName (string)    — binary name to register; a .exe suffix is ignored.
 *   shell   (string)    — "bash" or "powershell" (case-insensitive; "pwsh" is accepted).
 *
 * Returns:
 *   error — a usage error for an unsupported shell, or the write's error.
 *
 * Example:
 *   err := WriteCompletion(os.Stdout, "kb", "bash")
 */
func WriteCompletion(w io.Writer, appName, shell string) error {
	name := strings.TrimSuffix(appName, ".exe")
	var script string
	switch normalizeShell(shell) {
	case "bash":
		script = bashCompletion
	case "powershell":
		script = powershellCompletion
	default:
		return usageErrorf("unsupported shell %q; use one of: %s", shell, strings.Join(completionShells, ", "))
	}
	r := strings.NewReplacer(
		"{app_name}", name,
		"{app_func}", nonIdent.ReplaceAllString(name, "_"),
		"{verbs}", strings.Join(completionVerbs(), " "),
		"{global_flags}", strings.Join(completionGlobalFlags, " "),
		"{topics}", strings.Join(completionHelpTopics(), " "),
		"{shells}", strings.Join(completionShells, " "),
		"{value_flags}", strings.Join(valueFlags(), "|"),
		"{file_flags}", strings.Join(completionFileFlags, "|"),
		"{bash_subverb_cases}", bashSubverbCases("_"+nonIdent.ReplaceAllString(name, "_")+"_projects"),
		"{bash_flag_cases}", bashFlagCases(),
		"{search_kinds}", strings.Join(knowledge.SearchKinds, " "),
		"{ps_search_kinds}", psList(knowledge.SearchKinds),
		"{ps_verbs}", psList(completionVerbs()),
		"{ps_global_flags}", psList(completionGlobalFlags),
		"{ps_topics}", psList(completionHelpTopics()),
		"{ps_shells}", psList(completionShells),
		"{ps_value_flags}", psList(valueFlags()),
		"{ps_file_flags}", psList(completionFileFlags),
		"{ps_subverb_cases}", psSubverbCases(),
		"{ps_flag_cases}", psFlagCases(),
	)
	_, err := io.WriteString(w, r.Replace(script))
	return err
}

func normalizeShell(shell string) string {
	switch s := strings.ToLower(strings.TrimSpace(shell)); s {
	case "pwsh":
		return "powershell"
	default:
		return s
	}
}

const bashCompletion = `# bash completion for {app_name}
# Load with:  eval "$({app_name} completion bash)"
# Or install: {app_name} completion bash -install
# (Plain "source" of a process substitution does nothing, silently, in macOS bash 3.2.)
_{app_func}_projects() {
    {app_name} "${dbarg[@]}" project list 2>/dev/null | awk '$1 ~ /^[0-9]+$/ { print $2 }'
}
_{app_func}_search_scopes() {
    local p k
    for p in "" workspace $(_{app_func}_projects); do
        echo "$p/"
        for k in {search_kinds}; do
            # projects is searched at the global scope only (DR-0073).
            [[ -n "$p" && "$k" == projects ]] && continue
            echo "$p/$k"
        done
    done
}
_{app_func}() {
    local cur prev i w n verb
    local -a pos=() dbarg=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    # The words before the cursor that are neither options nor option values.
    for (( i=1; i<COMP_CWORD; i++ )); do
        w="${COMP_WORDS[i]}"
        case "$w" in
            --db|-db) dbarg=(--db "${COMP_WORDS[i+1]}"); (( i++ )) ;;
            {value_flags}) (( i++ )) ;;
            -*) ;;
            *) pos+=("$w") ;;
        esac
    done
    case "$prev" in
        {file_flags})
            COMPREPLY=( $(compgen -f -- "$cur") )
            return 0 ;;
        --project)
            COMPREPLY=( $(compgen -W "$(_{app_func}_projects)" -- "$cur") )
            return 0 ;;
    esac
    n=${#pos[@]}
    verb="${pos[0]}"
    if [[ -z "$verb" ]]; then
        if [[ "$cur" == -* ]]; then
            COMPREPLY=( $(compgen -W "{global_flags}" -- "$cur") )
        else
            COMPREPLY=( $(compgen -W "{verbs}" -- "$cur") )
        fi
        return 0
    fi
    if [[ "$cur" == -* ]]; then
        case "$verb" in
{bash_flag_cases}        esac
        return 0
    fi
    case "$verb" in
        help)       [[ $n -eq 1 ]] && COMPREPLY=( $(compgen -W "{topics}" -- "$cur") ) ;;
        completion) [[ $n -eq 1 ]] && COMPREPLY=( $(compgen -W "{shells}" -- "$cur") ) ;;
        search)     [[ $n -eq 1 ]] && COMPREPLY=( $(compgen -W "$(_{app_func}_search_scopes)" -- "$cur") ) ;;
{bash_subverb_cases}    esac
    return 0
}
complete -o default -F _{app_func} {app_name}
`

const powershellCompletion = `# PowerShell completion for {app_name}
# Load with:  {app_name} completion powershell | Out-String | Invoke-Expression
# Or install: {app_name} completion powershell -install
Register-ArgumentCompleter -Native -CommandName {app_name},{app_name}.exe -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $verbs      = @({ps_verbs})
    $globals    = @({ps_global_flags})
    $topics     = @({ps_topics})
    $shells     = @({ps_shells})
    $valueFlags = @({ps_value_flags})
    $fileFlags  = @({ps_file_flags})

    function Get-KbProjects {
        & {app_name} @script:dbArgs project list 2>$null | ForEach-Object {
            if ($_ -match '^\s*\d+\s+(\S+)') { $Matches[1] }
        }
    }
    function Get-KbSearchScopes {
        $kinds = @({ps_search_kinds})
        foreach ($p in @('', 'workspace') + @(Get-KbProjects)) {
            "$p/"
            # projects is searched at the global scope only (DR-0073).
            foreach ($k in $kinds) { if ($p -ne '' -and $k -eq 'projects') { continue }; "$p/$k" }
        }
    }
    function Get-Files {
        Get-ChildItem -Path "$wordToComplete*" -ErrorAction SilentlyContinue | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_.Name, $_.Name, 'ProviderItem', $_.Name)
        }
    }

    # Words typed after the command name, not counting the word being completed.
    $words = @($commandAst.CommandElements | Select-Object -Skip 1 | ForEach-Object { $_.ToString() })
    if ($wordToComplete -ne '' -and $words.Count -gt 0) {
        $words = @($words | Select-Object -SkipLast 1)
    }
    $prev = if ($words.Count -gt 0) { $words[-1] } else { '' }

    # The words that are neither options nor option values.
    $script:dbArgs = @()
    $pos = @()
    for ($i = 0; $i -lt $words.Count; $i++) {
        $w = $words[$i]
        if ($w -in '--db', '-db') { $script:dbArgs = @('--db', $words[$i + 1]); $i++; continue }
        if ($w -in $valueFlags) { $i++; continue }
        if ($w.StartsWith('-')) { continue }
        $pos += $w
    }
    $n = $pos.Count

    if ($prev -in $fileFlags) { return Get-Files }
    $candidates = $null
    if ($prev -eq '--project') {
        $candidates = Get-KbProjects
    } elseif ($n -eq 0) {
        $candidates = if ($wordToComplete.StartsWith('-')) { $globals } else { $verbs }
    } elseif ($wordToComplete.StartsWith('-')) {
        $candidates = switch ($pos[0]) {
{ps_flag_cases}            default { $null }
        }
    } else {
        $candidates = switch ($pos[0]) {
            'help'       { if ($n -eq 1) { $topics } }
            'completion' { if ($n -eq 1) { $shells } }
            'search'     { if ($n -eq 1) { Get-KbSearchScopes } }
{ps_subverb_cases}            default      { $null }
        }
    }

    if ($null -eq $candidates) { return Get-Files }
    $candidates | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`

/** bashStartupFile returns the file a new bash session reads that kb adds its
 * completion line to. On Linux that is ~/.bashrc. On macOS Terminal starts a
 * login shell, which reads the first of ~/.bash_profile, ~/.bash_login and
 * ~/.profile that exists, so the line goes there: creating ~/.bash_profile
 * beside an existing ~/.profile would hide the .profile from bash.
 *
 * Parameters:
 *   home (string) — the user's home directory.
 *   goos (string) — runtime.GOOS.
 *
 * Returns:
 *   string — the startup file's path, which may not exist yet.
 *
 * Example:
 *   rc := bashStartupFile(home, "darwin") // ~/.bash_profile
 */
func bashStartupFile(home, goos string) string {
	if goos != "darwin" {
		return filepath.Join(home, ".bashrc")
	}
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		if _, err := os.Stat(filepath.Join(home, name)); err == nil {
			return filepath.Join(home, name)
		}
	}
	return filepath.Join(home, ".bash_profile")
}

// completionTargets returns where the completion script for a shell lives and
// which startup file refers to it, and the line that file carries.
func completionTargets(name, shell, home, dataHome, goos string) (script, startup, line string) {
	if normalizeShell(shell) == "bash" {
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		script = filepath.Join(dataHome, "bash-completion", "completions", name)
		return script, bashStartupFile(home, goos), fmt.Sprintf("[ -f '%s' ] && . '%s' # %s completion", script, script, name)
	}
	dir := filepath.Join(home, ".config", "powershell")
	if goos == "windows" {
		dir = filepath.Join(home, "Documents", "PowerShell")
	}
	script = filepath.Join(dir, name+"-completion.ps1")
	return script, filepath.Join(dir, "Microsoft.PowerShell_profile.ps1"), fmt.Sprintf(". '%s' # %s completion", script, name)
}

// addStartupLine appends line to the startup file unless a line mentioning the
// script is already there. A missing file is created.
func addStartupLine(startup, script, line string) error {
	cur, err := os.ReadFile(startup)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(cur), script) {
		return nil
	}
	if len(cur) > 0 && !bytes.HasSuffix(cur, []byte("\n")) {
		cur = append(cur, '\n')
	}
	return writeFileIn(startup, append(cur, line+"\n"...))
}

// removeStartupLines deletes the lines kb added to the startup file for the
// script, reporting whether any were there. Other lines are kept as they are.
func removeStartupLines(startup, script, name string) (bool, error) {
	cur, err := os.ReadFile(startup)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	marker := "# " + name + " completion"
	var kept []string
	removed := false
	for _, l := range strings.SplitAfter(string(cur), "\n") {
		if strings.Contains(l, "'"+script+"'") && strings.HasSuffix(strings.TrimRight(l, "\r\n"), marker) {
			removed = true
			continue
		}
		kept = append(kept, l)
	}
	if !removed {
		return false, nil
	}
	return true, os.WriteFile(startup, []byte(strings.Join(kept, "")), 0o644)
}

/** InstallCompletion installs the completion script for a shell so it loads in
 * every new session, and returns the path of the script it wrote.
 *
 * The script goes in the bash-completion user directory
 * ($XDG_DATA_HOME/bash-completion/completions/NAME, or the same under
 * ~/.local/share) for bash and beside the profile for PowerShell. An existing
 * bash file there is replaced only if kb wrote it. The shell's startup file
 * (bashStartupFile, or the PowerShell profile) gets one line that sources the
 * script, added once. The line is what makes it work where no bash-completion
 * loader reads the directory, as with macOS's /bin/bash 3.2.
 *
 * Parameters:
 *   appName  (string) — binary name to register; a .exe suffix is ignored.
 *   shell    (string) — "bash" or "powershell" ("pwsh" is accepted).
 *   home     (string) — the user's home directory.
 *   dataHome (string) — $XDG_DATA_HOME, or "" for the default.
 *   goos     (string) — runtime.GOOS, which picks the startup file and PowerShell profile.
 *
 * Returns:
 *   (string, error) — the installed script path; a usage error for an
 *   unsupported shell, a cant_create error for a foreign file in the way, or the
 *   failed write's error.
 *
 * Example:
 *   path, err := InstallCompletion("kb", "bash", home, "", "linux")
 */
func InstallCompletion(appName, shell, home, dataHome, goos string) (string, error) {
	name := strings.TrimSuffix(appName, ".exe")
	var script bytes.Buffer
	if err := WriteCompletion(&script, appName, shell); err != nil {
		return "", err
	}
	path, startup, line := completionTargets(name, shell, home, dataHome, goos)
	if normalizeShell(shell) == "bash" {
		if old, err := os.ReadFile(path); err == nil {
			if !strings.HasPrefix(string(old), "# bash completion for "+name+"\n") {
				return "", cantCreatef("%s exists and was not written by %s; not overwriting it", path, name)
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if err := writeFileIn(path, script.Bytes()); err != nil {
		return "", err
	}
	return path, addStartupLine(startup, path, line)
}

/** UninstallCompletion undoes InstallCompletion: it removes the script if kb
 * wrote it and the line kb added to the startup file. A file kb did not write
 * is left alone.
 *
 * Parameters:
 *   appName, shell, home, dataHome, goos — as for InstallCompletion.
 *
 * Returns:
 *   ([]string, error) — what was removed, one description per item, empty when
 *   nothing of kb's was found; a usage error for an unsupported shell, or the
 *   failed read or write's error.
 *
 * Example:
 *   removed, err := UninstallCompletion("kb", "bash", home, "", "linux")
 */
func UninstallCompletion(appName, shell, home, dataHome, goos string) ([]string, error) {
	name := strings.TrimSuffix(appName, ".exe")
	if s := normalizeShell(shell); s != "bash" && s != "powershell" {
		return nil, usageErrorf("unsupported shell %q; use one of: %s", shell, strings.Join(completionShells, ", "))
	}
	path, startup, _ := completionTargets(name, shell, home, dataHome, goos)
	var removed []string
	if old, err := os.ReadFile(path); err == nil {
		if strings.HasPrefix(string(old), completionHeader(shell, name)) {
			if err := os.Remove(path); err != nil {
				return removed, err
			}
			removed = append(removed, "script "+path)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// The line may be in a startup file other than the one install would pick
	// today (a ~/.bash_profile created since), so look in all of them.
	files := []string{startup}
	if normalizeShell(shell) == "bash" {
		for _, n := range []string{".bashrc", ".bash_profile", ".bash_login", ".profile"} {
			files = append(files, filepath.Join(home, n))
		}
	}
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		if ok, err := removeStartupLines(f, path, name); err != nil {
			return removed, err
		} else if ok {
			removed = append(removed, "line in "+f)
		}
	}
	return removed, nil
}

// completionHeader is the first line of a script kb wrote, which is how kb
// knows a file is its own.
func completionHeader(shell, name string) string {
	if normalizeShell(shell) == "bash" {
		return "# bash completion for " + name + "\n"
	}
	return "# PowerShell completion for " + name + "\n"
}

/** CheckCompletion reports, one line per finding, whether completion is set up
 * for a shell and what to do about it. It looks at the files only: it cannot see
 * inside the shell a Terminal window is running, so the report ends with the
 * command that does.
 *
 * Parameters:
 *   w, appName, shell, home, dataHome, goos — as for InstallCompletion.
 *
 * Returns:
 *   (bool, error) — true when every check passed; a usage error for an
 *   unsupported shell, or a failed read.
 *
 * Example:
 *   ok, err := CheckCompletion(os.Stdout, "kb", "bash", home, "", "darwin")
 */
func CheckCompletion(w io.Writer, appName, shell, home, dataHome, goos string) (bool, error) {
	name := strings.TrimSuffix(appName, ".exe")
	var want bytes.Buffer
	if err := WriteCompletion(&want, appName, shell); err != nil {
		return false, err
	}
	path, startup, _ := completionTargets(name, shell, home, dataHome, goos)
	ok := true
	report := func(good bool, format string, a ...any) {
		mark := "ok     "
		if !good {
			mark, ok = "PROBLEM", false
		}
		fmt.Fprintf(w, "%s %s\n", mark, fmt.Sprintf(format, a...))
	}
	old, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		report(false, "not installed: no script at %s; run: %s completion %s -install", path, name, normalizeShell(shell))
	case err != nil:
		return false, err
	case !strings.HasPrefix(string(old), completionHeader(shell, name)):
		report(false, "%s exists but %s did not write it", path, name)
	case string(old) != want.String():
		report(false, "script at %s is out of date; run: %s completion %s -install", path, name, normalizeShell(shell))
	default:
		report(true, "script is installed and current: %s", path)
	}
	files := []string{startup}
	if normalizeShell(shell) == "bash" {
		files = []string{bashStartupFile(home, goos)}
	}
	found := false
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil && strings.Contains(string(b), "'"+path+"'") {
			found = true
			report(true, "%s sources it", f)
		}
	}
	if !found {
		report(false, "%s does not source the script; run: %s completion %s -install", files[0], name, normalizeShell(shell))
	}
	if normalizeShell(shell) == "bash" {
		fmt.Fprintf(w, "\nA window opened before the install has not read the new line: open a new one, or run: . %s\n", path)
		fmt.Fprintf(w, "To see what the running shell has, type:\n  echo $BASH_VERSION; complete -p %s\n", name)
		fmt.Fprintf(w, "It should print: complete -o default -F _%s %s\n", nonIdent.ReplaceAllString(name, "_"), name)
		fmt.Fprintf(w, "If it prints nothing, add the line shown by:  %s completion bash -install\nor trace a new shell's startup with:  bash -lixc exit 2>&1 | grep -n %s\n", name, name)
	}
	return ok, nil
}

// writeFileIn writes data to path, creating its directory first.
func writeFileIn(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// cmdCompletion is `kb completion SHELL [-install|-uninstall|-check]`. It never
// touches a database: mainRun routes it past the open.
func cmdCompletion(_ *knowledge.KnowledgeBase, _ *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	var install, uninstall, check bool
	positional, err := splitFlags(args, nil, map[string]*bool{
		"--install": &install, "-install": &install,
		"--uninstall": &uninstall, "-uninstall": &uninstall,
		"--check": &check, "-check": &check,
	})
	if err != nil {
		return err
	}
	const usage = "usage: completion bash|powershell [-install|-uninstall|-check]"
	if len(positional) != 1 {
		return usageErrorf("%s", usage)
	}
	n := 0
	for _, b := range []bool{install, uninstall, check} {
		if b {
			n++
		}
	}
	if n > 1 {
		return usageErrorf("-install, -uninstall and -check are exclusive; %s", usage)
	}
	shell := positional[0]
	if n == 0 {
		return WriteCompletion(out, appName, shell)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return noInputf("cannot find your home directory: %v", err)
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	switch {
	case uninstall:
		removed, err := UninstallCompletion(appName, shell, home, dataHome, runtime.GOOS)
		if err != nil {
			return err
		}
		if jsonOut {
			if removed == nil {
				removed = []string{}
			}
			if err := printJSON(out, struct {
				Shell   string   `json:"shell"`
				Removed []string `json:"removed"`
			}{normalizeShell(shell), removed}); err != nil {
				return err
			}
		} else {
			for _, r := range removed {
				fmt.Fprintf(out, "removed %s\n", r)
			}
		}
		if len(removed) == 0 {
			return negativef("nothing to uninstall: no %s completion written by %s was found", normalizeShell(shell), appName)
		}
		return nil
	case check:
		var report io.Writer = out
		if jsonOut {
			report = io.Discard
		}
		ok, err := CheckCompletion(report, appName, shell, home, dataHome, runtime.GOOS)
		if err != nil {
			return err
		}
		if jsonOut {
			if err := printJSON(out, struct {
				Shell string `json:"shell"`
				OK    bool   `json:"ok"`
			}{normalizeShell(shell), ok}); err != nil {
				return err
			}
		}
		if !ok {
			return negativef("completion for %s is not set up correctly (see above)", normalizeShell(shell))
		}
		return nil
	}
	path, err := InstallCompletion(appName, shell, home, dataHome, runtime.GOOS)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(out, struct {
			Shell string `json:"shell"`
			Path  string `json:"path"`
		}{normalizeShell(shell), path})
	}
	fmt.Fprintf(out, "installed %s completion: %s\n", normalizeShell(shell), path)
	if normalizeShell(shell) == "bash" {
		fmt.Fprintf(out, "open a new terminal window, or run: . %s\n", path)
	}
	return nil
}
