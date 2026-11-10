%kb-version(1) user manual | version 0.0.31 e413620
% R. S. Doiel
% 2026-11-09

# NAME

kb-version, kb-license — print the version, or the license

# SYNOPSIS

kb version

kb license

kb -json version

kb -json license

# DESCRIPTION

kb version prints the program name, its version and the release hash it was
built from, on one line: the same as kb -version. kb license prints the
license text: the same as kb -license. They are verbs as well as options for
the habit of the go command (go version).

With the global -json option, version prints an object with "name", "version" and
"release_hash", and license an object with "license", so standard output is JSON in
that mode as for every verb.

Neither opens a database, so both work anywhere, and --db does not apply to them.
The global options go before the verb: kb -json version.

# EXIT STATUS

The workspace convention, as described in kb(1): 0 success; 2 an unknown
flag, a surplus argument, or -db, which does not apply to these verbs.

# SEE ALSO

kb(1), kb-verbs(1)

