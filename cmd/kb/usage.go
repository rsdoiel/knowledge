package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

/** usageErrorf is fmt.Errorf that marks its result as a usage error: a mistake
 * in the command line itself (a missing or surplus argument, an unknown subverb
 * or flag, a flag without its value, a bad value passed on the command line, or
 * a missing required flag). Nothing was attempted. dispatch exits 2 for these
 * (workspace DR-0003, kb DR-0047). A command line that parsed and then failed
 * (not found, no results, a database error) is not a usage error. %w works as
 * it does in fmt.Errorf, so a wrapped cause stays reachable.
 *
 * Parameters:
 *   format (string) — a fmt.Errorf format.
 *   a      (...any) — the format's arguments.
 *
 * Returns:
 *   error — a *classedError of classUsage.
 *
 * Example:
 *   return usageErrorf("invalid observation id %q", args[0])
 */
func usageErrorf(format string, a ...any) error {
	return classErrorf(classUsage, format, a...)
}

/** wrapUsage marks an existing error as a usage error and leaves nil alone. An
 * error that is already flag.ErrHelp is returned unchanged: a help request is
 * answered with a page, not reported as a mistake.
 *
 * Parameters:
 *   err (error) — the error to mark, or nil.
 *
 * Returns:
 *   error — nil, err itself for flag.ErrHelp or an existing usage error, or a
 *           usage-class error wrapping err.
 *
 * Example:
 *   if err := fs.Parse(args); err != nil {
 *       return wrapUsage(err)
 *   }
 */
func wrapUsage(err error) error {
	if err == nil || errors.Is(err, flag.ErrHelp) || isUsageError(err) {
		return err
	}
	return classedAs(classUsage, err)
}

/** isUsageError reports whether err, or anything it wraps, is a usage error.
 *
 * Parameters:
 *   err (error) — the error to test; may be nil.
 *
 * Returns:
 *   bool — true if err's exit class is usage.
 *
 * Example:
 *   if isUsageError(err) { return 2 }
 */
func isUsageError(err error) bool {
	class, classified := classify(err)
	return classified && class == classUsage
}

/** parseFlags is fs.Parse for a verb's FlagSet, with a parse failure marked as
 * a usage error. flag.ErrHelp passes through untouched so dispatch can answer
 * it with the verb's page.
 *
 * Parameters:
 *   fs   (*flag.FlagSet) — the verb's flag set.
 *   args ([]string)      — the arguments to parse.
 *
 * Returns:
 *   error — nil, flag.ErrHelp, or a usage error for an unknown flag, a missing
 *           value or a value that does not parse.
 *
 * Example:
 *   if err := parseFlags(fs, args); err != nil {
 *       return err
 *   }
 */
func parseFlags(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(args)
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		if name, ok := strings.CutPrefix(err.Error(), "flag provided but not defined: "); ok {
			if h := globalOptionHint(name); h != "" {
				return usageErrorf("%v (%s)", err, h)
			}
		}
	}
	return wrapUsage(err)
}

/** looksLikeFlag reports whether arg would be read as a flag: it starts with
 * "-" and is longer than the bare "-" that conventionally means stdin.
 *
 * Parameters:
 *   arg (string) — a single command-line argument.
 *
 * Returns:
 *   bool — true for "-x", "--x" and "-1"; false for "-", "" and "x".
 *
 * Example:
 *   looksLikeFlag("--bogus") // true
 */
func looksLikeFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

/** plainArgs validates the arguments of a verb that has no flags of its own
 * and returns them as positionals. Until now such a verb checked only that it
 * had enough, so a bogus flag or a surplus argument was dropped and the
 * command exited 0.
 *
 * A "--" ends flag recognition, as it does for concept delete: what follows is
 * positional however it looks, which is how a name that starts with a dash is
 * given. Before it, only the first fixed arguments are checked for flag shape;
 * anything after those is free text the verb joins (a description, a note) and
 * a "-x" there is text.
 *
 * Parameters:
 *   args  ([]string) — the verb's arguments, after the subverb.
 *   min   (int)      — fewest positionals allowed.
 *   max   (int)      — most positionals allowed; negative means no limit
 *                      (free text after the fixed ones).
 *   fixed (int)      — how many leading arguments are checked for flag shape.
 *   usage (string)   — the usage line reported on a wrong count.
 *
 * Returns:
 *   []string — the positionals, with any "--" removed.
 *   error    — a usage error for an unknown flag or a wrong number of
 *              positionals.
 *
 * Example:
 *   pos, err := plainArgs(args, 2, 2, 2, "usage: project set-status NAME STATUS")
 */
func plainArgs(args []string, min, max, fixed int, usage string) ([]string, error) {
	positional := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if i < fixed && looksLikeFlag(arg) {
			return nil, withGlobalHint(fmt.Sprintf("unknown flag %q; %s", arg, usage), args)
		}
		positional = append(positional, arg)
	}
	if len(positional) < min || (max >= 0 && len(positional) > max) {
		return nil, withGlobalHint(usage, args)
	}
	return positional, nil
}

/** noExtraArgs reports a usage error when a FlagSet-based verb, which takes
 * no positional arguments, was given any after its flags were parsed.
 *
 * Parameters:
 *   fs    (*flag.FlagSet) — the verb's flag set, already parsed.
 *   usage (string)        — the usage line reported on a surplus argument.
 *
 * Returns:
 *   error — a usage error naming the first surplus argument, or nil.
 *
 * Example:
 *   if err := noExtraArgs(fs, "usage: export [--project P] [--out FILE]"); err != nil {
 *       return err
 *   }
 */
func noExtraArgs(fs *flag.FlagSet, usage string) error {
	if fs.NArg() > 0 {
		return usageErrorf("unexpected argument %q; %s", fs.Arg(0), usage)
	}
	return nil
}

/** globalOptionHint explains where a global option goes when it turns up after
 * the verb, where it used to be swallowed silently (so `project list --json`
 * printed plain text and exited 0). It returns "" for anything that is not one
 * of the global options --json, --db and --debug, in either dash form.
 *
 * Parameters:
 *   arg (string) — a single command-line argument.
 *
 * Returns:
 *   string — a one-line explanation, or "" if arg is not a global option.
 *
 * Example:
 *   globalOptionHint("--json") // "--json is a global option; put it before the verb, as in kb --json VERB ..."
 */
func globalOptionHint(arg string) string {
	if !strings.HasPrefix(arg, "-") {
		return ""
	}
	switch strings.TrimLeft(arg, "-") {
	case "json", "db", "debug":
		return fmt.Sprintf("%s is a global option; put it before the verb, as in kb %s VERB ...", arg, arg)
	}
	return ""
}

// withGlobalHint appends the hint for the first global option among args to a
// usage message, so a misplaced --json is told where it belongs rather than
// just refused.
func withGlobalHint(msg string, args []string) error {
	for _, a := range args {
		if a == "--" {
			break
		}
		if h := globalOptionHint(a); h != "" {
			return usageErrorf("%s (%s)", msg, h)
		}
	}
	return usageErrorf("%s", msg)
}
