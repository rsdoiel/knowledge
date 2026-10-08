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
 * the subverb's own subverbs. completion_test.go compares each verb's list with
 * the usage line the verb itself reports, so a subverb added to the verb and
 * not here fails the build.
 */
var completionSubverbs = map[string][]string{
	"project":         {"add", "list", "show", "concepts", "set-status", "set-description", "rename", "delete"},
	"observation":     {"add", "list", "show", "update", "sources", "delete"},
	"concept":         {"add", "list", "show", "recall", "rename", "delete", "suggest"},
	"source":          {"add", "list", "show", "remove", "retract", "link", "check-retractions"},
	"link":            {"project", "observation"},
	"record":          {"list", "pending", "show", "set-status", "supersede", "fmt", "new", "concepts", "delete", "fuzzy-tag"},
	"document":        {"ingest", "draft", "review", "list", "show", "tag", "fuzzy-tag", "frontmatter", "delete"},
	"document review": {"list", "promote"},
}

/** completionProjectArgs names the subverbs whose first argument is a project
 * name, which completion fills in from the database.
 */
var completionProjectArgs = map[string][]string{
	"project": {"show", "concepts", "set-status", "set-description", "rename", "delete"},
}

/** completionVerbFlags lists the flags each verb accepts, subverbs included.
 * The flags are declared in several ways (flag.FlagSet, splitFlags maps, a
 * switch), so there is no single registry to read; TestCompletion_FlagsMatchTheSource
 * scans the source for declared flags and fails when this table and the code
 * disagree.
 */
var completionVerbFlags = map[string][]string{
	"project":     {"--status", "--root", "--dry-run"},
	"observation": {"--project", "--source-doi"},
	"concept":     {"--identifier-type", "--identifier-value", "--project", "--limit", "--force", "--dry-run"},
	"source":      {"--doi", "--url", "--authors", "--published", "--publisher", "--rights", "--version", "--relationship"},
	"record": {"--project", "--status", "--kind", "--trigger", "--initiative", "--since", "--root", "--dir",
		"--title", "--concept", "--workspace", "--partial", "--dry-run", "--write", "--all"},
	"document": {"--project", "--concept", "--dry-run", "--accept", "--accept-keywords", "--set", "--by",
		"--confidence", "--status", "--title", "--format"},
	"ingest":     {"--root", "--dry-run"},
	"index":      {"--stdout", "--check", "--all"},
	"merge":      {"--a", "--b", "--out", "--force"},
	"export":     {"--project", "--out"},
	"import":     {"--in"},
	"check-db":   {"--jsonl"},
	"search":     {"--project"},
	"summary":    {"--project"},
	"format":     {"--project"},
	"completion": {"--install"},
}

// completionGlobalFlags are the options that go before the verb.
var completionGlobalFlags = []string{"--db", "--debug", "--help", "--json", "--license", "--version"}

// completionBoolFlags take no value; every other flag does, so completion
// skips the word after it when looking for the verb's arguments.
var completionBoolFlags = map[string]bool{
	"--dry-run": true, "--force": true, "--stdout": true, "--check": true, "--all": true,
	"--workspace": true, "--partial": true, "--write": true, "--install": true,
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
# Load with:  source <({app_name} completion bash)
# Or install: {app_name} completion bash -install
_{app_func}_projects() {
    {app_name} "${dbarg[@]}" project list 2>/dev/null | awk '$1 ~ /^[0-9]+$/ { print $2 }'
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
{ps_subverb_cases}            default      { $null }
        }
    }

    if ($null -eq $candidates) { return Get-Files }
    $candidates | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`

/** InstallCompletion installs the completion script for a shell so it loads in
 * every new session, and returns the path of the script it wrote.
 *
 * For bash the script goes in the bash-completion user directory
 * ($XDG_DATA_HOME/bash-completion/completions/NAME, or the same under
 * ~/.local/share), which bash-completion loads on demand. An existing file
 * there is replaced only if kb wrote it.
 *
 * For PowerShell the script is written beside the profile and the profile gets
 * one line that dot-sources it, added once.
 *
 * Parameters:
 *   appName  (string) — binary name to register; a .exe suffix is ignored.
 *   shell    (string) — "bash" or "powershell" ("pwsh" is accepted).
 *   home     (string) — the user's home directory.
 *   dataHome (string) — $XDG_DATA_HOME, or "" for the default.
 *   goos     (string) — runtime.GOOS, which picks the PowerShell profile location.
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
	if normalizeShell(shell) == "bash" {
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		path := filepath.Join(dataHome, "bash-completion", "completions", name)
		if old, err := os.ReadFile(path); err == nil {
			if !strings.HasPrefix(string(old), "# bash completion for "+name+"\n") {
				return "", cantCreatef("%s exists and was not written by %s; not overwriting it", path, name)
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return path, writeFileIn(path, script.Bytes())
	}
	profileDir := filepath.Join(home, ".config", "powershell")
	if goos == "windows" {
		profileDir = filepath.Join(home, "Documents", "PowerShell")
	}
	path := filepath.Join(profileDir, name+"-completion.ps1")
	if err := writeFileIn(path, script.Bytes()); err != nil {
		return "", err
	}
	profile := filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
	cur, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if strings.Contains(string(cur), path) {
		return path, nil
	}
	if len(cur) > 0 && !bytes.HasSuffix(cur, []byte("\n")) {
		cur = append(cur, '\n')
	}
	line := fmt.Sprintf(". '%s' # %s completion\n", path, name)
	return path, os.WriteFile(profile, append(cur, line...), 0o644)
}

// writeFileIn writes data to path, creating its directory first.
func writeFileIn(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// cmdCompletion is `kb completion SHELL [-install]`. It never touches a
// database: mainRun routes it past the open.
func cmdCompletion(_ *knowledge.KnowledgeBase, _ *DebugLog, jsonOut bool, args []string, out io.Writer) error {
	var install bool
	positional, err := splitFlags(args, nil, map[string]*bool{"--install": &install, "-install": &install})
	if err != nil {
		return err
	}
	const usage = "usage: completion bash|powershell [-install]"
	if len(positional) != 1 {
		return usageErrorf("%s", usage)
	}
	shell := positional[0]
	if !install {
		return WriteCompletion(out, appName, shell)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return noInputf("cannot find your home directory: %v", err)
	}
	path, err := InstallCompletion(appName, shell, home, os.Getenv("XDG_DATA_HOME"), runtime.GOOS)
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
	return nil
}
