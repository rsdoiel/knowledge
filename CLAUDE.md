# CLAUDE.md

Read [AGENTS.md](AGENTS.md): what `kb` is, the rules for using it safely, the verbs and the exit codes.

## Makefile: verb man pages

The `Makefile` was scaffolded by `cmt` and has since been extended. Do not regenerate it. Its `KB_TOPICS` variable and `kb-topics-help` target generate one man page per `kb` verb. When adding a verb, add it to `KB_TOPICS` and to `kb(1)`; `cmd/kb/help_dispatch_test.go` enforces both. Losing `KB_TOPICS` fails quietly: `MAN_PAGES_1` is a glob over existing `.1.md` files, so `make man` would still report success.
