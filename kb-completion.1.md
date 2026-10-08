%kb-completion(1) user manual | version 0.0.16 362523e
% R. S. Doiel
% 2026-10-06

# NAME

kb-completion — write or install a shell completion script

# SYNOPSIS

kb completion bash|powershell [-install]

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
  ~/.local/share, which bash-completion loads on demand. A file already there
  is replaced only if kb wrote it; anything else is left alone and the
  command exits 73. With PowerShell the script is written as
  kb-completion.ps1 beside the profile (~/.config/powershell, or
  Documents\PowerShell on Windows), and one line that dot-sources it is added
  to Microsoft.PowerShell_profile.ps1 once. Windows PowerShell 5.1, whose
  profile is under Documents\WindowsPowerShell, is not covered.

The global -db option does not apply and is refused (exit 2).

# EXAMPLES

Try it in the current bash session:

~~~shell
source <(kb completion bash)
~~~

Install it for every session:

~~~shell
kb completion bash -install
kb completion powershell -install
~~~

# EXIT STATUS

The workspace convention, as described in kb(1). 0 success; 2 no shell
named, a shell that is not supported, a surplus argument or an unknown flag;
66 no home directory found for -install; 73 -install found a file that
kb did not write, or could not create the directory; 74 a write failed
part way.

# SEE ALSO

kb(1), kb-topics(1)

