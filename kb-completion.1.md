%kb-completion(1) user manual | version 0.0.27 124d213
% R. S. Doiel
% 2026-10-25

# NAME

kb-completion — write or install a shell completion script

# SYNOPSIS

kb completion bash|powershell [-install|-uninstall|-check]

# DESCRIPTION

Writes a completion script for the named shell to standard output. The shell
name is case-insensitive and pwsh is accepted for powershell. The script
completes:

- the verbs and, after help, the help topics;
- the subverbs of project, observation, concept, source, link, record and
  document (and document review);
- each verb's flags, once a word beginning with a dash is started;
- paths after -db, -in, -out, -a, -b, -jsonl, -root and -dir;
- project names after --project, and after project show, concepts,
  set-status, set-description, rename and delete. They are read by running
  "kb project list" in the current directory, honouring a -db typed
  earlier on the line, so they work only where a workspace is found.

The verb list is read from the verbs kb itself registers, so a new
verb is completed without anyone adding it. The subverbs and flags are kept
in a table, which the tests compare with the code.

Neither form opens a database, so neither needs a workspace.

# OPTIONS

-install
: instead of writing the script to standard output, install it so it loads in
  every new shell, and print where it went. With bash the script goes to
  $XDG_DATA_HOME/bash-completion/completions/kb, or the same path under
  ~/.local/share, and one line that sources it is added once to the startup
  file a new shell reads: ~/.bashrc, or on macOS the first of ~/.bash_profile,
  ~/.bash_login and ~/.profile that exists (~/.bash_profile if none does). The
  line is what makes it work on macOS, whose /bin/bash 3.2 has no
  bash-completion loader to read the directory. A script already there is
  replaced only if kb wrote it; anything else is left alone and the
  command exits 73. With PowerShell the script is written as
  kb-completion.ps1 beside the profile (~/.config/powershell, or
  Documents\PowerShell on Windows), and one line that dot-sources it is added
  to Microsoft.PowerShell_profile.ps1 once. Windows PowerShell 5.1, whose
  profile is under Documents\WindowsPowerShell, is not covered.

-uninstall
: undo -install: remove the script if kb wrote it, and the line it added
  to the startup file or profile. Nothing else is touched. Exits 1 when there
  was nothing of kb's to remove.

-check
: say whether completion is set up for the shell: the script is there and
  current, and the startup file sources it. One line per finding, each problem
  with the command that fixes it, and for bash the two commands that show what
  a running shell has. Exits 1 when any check fails. It reads files only; it
  cannot see inside a shell already open, which has not read a line added after
  it started.

The global -db option does not apply and is refused (exit 2).

# EXAMPLES

Try it in the current bash session:

~~~shell
eval "$(kb completion bash)"
~~~

Use eval, not source <(...): macOS's bash 3.2 does nothing with the latter and
says nothing.

Install it for every session, check it, and take it out again:

~~~shell
kb completion bash -install
kb completion bash -check
kb completion bash -uninstall
kb completion powershell -install
~~~

If completion does not work in a new window, run -check, then in that window
type "echo $BASH_VERSION; complete -p kb". Nothing printed by the second
command means the shell never read the startup line.

# EXIT STATUS

The workspace convention, as described in kb(1). 0 success; 2 no shell
named, a shell that is not supported, a surplus argument or an unknown flag;
1 -uninstall found nothing to remove, or -check found a problem; 66 no home
directory found; 73 -install found a file that kb did not write, or could
not create the directory; 74 a write failed part way.
-install, -uninstall and -check together are a usage error (2).

# SEE ALSO

kb(1), kb-topics(1)

