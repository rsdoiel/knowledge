package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	knowledge "github.com/rsdoiel/knowledge"
)

// Workspace DR-0003 fixes one table of exit codes for every command-line tool,
// and knowledge DR-0047 applies it to kb. X0 of exit-codes-plan.md is the
// machinery: a class an error can carry, one function that turns an error into
// its class, and dispatch and the JSON error envelope using it. The individual
// error sites are classified later (X2), so most tests here build errors by hand.

// The table is the contract: a number changing silently would break every
// script that trusts it, so it is pinned row by row.
func TestExitClasses_MatchTheWorkspaceTable(t *testing.T) {
	for _, tc := range []struct {
		class exitClass
		name  string
		code  int
	}{
		{classOK, "ok", 0},
		{classNegative, "negative", 1},
		{classUsage, "usage", 2},
		{classData, "data", 65},
		{classNoInput, "no_input", 66},
		{classUnavailable, "unavailable", 69},
		{classInternal, "internal", 70},
		{classCantCreate, "cant_create", 73},
		{classIO, "io", 74},
		{classTempFail, "temp_fail", 75},
		{classNoPermission, "no_permission", 77},
		{classConfig, "config", 78},
	} {
		if tc.class.Name != tc.name || tc.class.Code != tc.code {
			t.Errorf("class %q = {%q, %d}, want {%q, %d}", tc.name, tc.class.Name, tc.class.Code, tc.name, tc.code)
		}
	}
}

// No class may use a number the convention reserves: 64 is left free on
// purpose, 126/127 and above 128 belong to the shell, and nothing exceeds 125.
func TestExitClasses_AvoidReservedNumbers(t *testing.T) {
	seen := map[int]string{}
	for _, c := range allExitClasses {
		if c.Code == 64 || c.Code == 126 || c.Code == 127 || c.Code > 125 {
			t.Errorf("class %q uses reserved code %d", c.Name, c.Code)
		}
		if other, dup := seen[c.Code]; dup {
			t.Errorf("classes %q and %q share code %d", other, c.Name, c.Code)
		}
		seen[c.Code] = c.Name
	}
	if len(allExitClasses) != 12 {
		t.Errorf("allExitClasses has %d entries, want the 12 rows of the table", len(allExitClasses))
	}
}

func TestClassify_Constructors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want exitClass
	}{
		{"usageErrorf", usageErrorf("usage: x %s", "y"), classUsage},
		{"negativef", negativef("still linked: %d", 3), classNegative},
		{"notFoundf", notFoundf("project %q not found", "p"), classNegative},
		{"dataErrorf", dataErrorf("bad record %s", "r"), classData},
		{"noInputf", noInputf("%s does not exist", "f"), classNoInput},
		{"cantCreatef", cantCreatef("%s exists", "f"), classCantCreate},
		{"unavailablef", unavailablef("cannot reach %s", "h"), classUnavailable},
		{"ioErrorf", ioErrorf("write %s failed", "f"), classIO},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, classified := classify(tc.err)
			if !classified || got != tc.want {
				t.Errorf("classify = %v, %v; want %v, true", got, classified, tc.want)
			}
		})
	}
}

// Classifying adds a class; it never changes what the user reads.
func TestClassify_MessageIsUnchanged(t *testing.T) {
	if got := notFoundf("project %q not found", "p").Error(); got != `project "p" not found` {
		t.Errorf("message = %q", got)
	}
	if got := dataErrorf("bad %d", 7).Error(); got != "bad 7" {
		t.Errorf("message = %q", got)
	}
}

// %w must keep the cause reachable, as it does for fmt.Errorf.
func TestClassify_WrapsCause(t *testing.T) {
	cause := errors.New("root cause")
	err := dataErrorf("record 3: %w", cause)
	if !errors.Is(err, cause) {
		t.Error("errors.Is cannot reach the wrapped cause")
	}
	var wrapped error = fmt.Errorf("outer: %w", err)
	if got, ok := classify(wrapped); !ok || got != classData {
		t.Errorf("classify through an fmt.Errorf wrapper = %v, %v; want data", got, ok)
	}
}

// The outermost class wins, so a caller can reclassify what a helper returned:
// a file verb wraps the library's invalid-value error as data (65) where the
// same error from an argument would be usage (2).
func TestClassify_OutermostClassWins(t *testing.T) {
	inner := usageErrorf("bad value")
	outer := dataErrorf("record 3: %w", inner)
	if got, _ := classify(outer); got != classData {
		t.Errorf("classify = %v, want data (the outer class)", got)
	}
	if got, _ := classify(classedAs(classIO, dataErrorf("x"))); got != classIO {
		t.Errorf("classedAs did not override the inner class")
	}
}

func TestClassify_StandardLibraryErrorsClassifyThemselves(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.db")
	_, realNotExist := os.Open(missing)
	_, realIsDir := os.ReadFile(t.TempDir())

	for _, tc := range []struct {
		name string
		err  error
		want exitClass
	}{
		{"fs.ErrNotExist", fs.ErrNotExist, classNoInput},
		{"a real open of a missing file", realNotExist, classNoInput},
		{"fs.ErrPermission", fs.ErrPermission, classNoPermission},
		{"EACCES", &fs.PathError{Op: "open", Path: "f", Err: syscall.EACCES}, classNoPermission},
		{"fs.ErrExist", fs.ErrExist, classCantCreate},
		{"EEXIST", &fs.PathError{Op: "mkdir", Path: "d", Err: syscall.EEXIST}, classCantCreate},
		{"a read failure", &fs.PathError{Op: "read", Path: "f", Err: syscall.EIO}, classIO},
		{"reading a directory", realIsDir, classIO},
		{"a link error", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.EIO}, classIO},
		{"a network operation error", &net.OpError{Op: "dial", Err: errors.New("refused")}, classUnavailable},
		{"a url error", &url.Error{Op: "Get", URL: "https://x", Err: errors.New("no route")}, classUnavailable},
		{"wrapped with %w", fmt.Errorf("checking: %w", fs.ErrNotExist), classNoInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, classified := classify(tc.err)
			if !classified || got != tc.want {
				t.Errorf("classify(%v) = %v, %v; want %v, true", tc.err, got, classified, tc.want)
			}
		})
	}
}

// An explicit class beats what the standard library would say: a mkdir that
// fails because the parent is missing is "cannot create", not "no input".
func TestClassify_ExplicitClassBeatsStandardLibrary(t *testing.T) {
	err := cantCreatef("create %s: %w", "d", fs.ErrNotExist)
	if got, _ := classify(err); got != classCantCreate {
		t.Errorf("classify = %v, want cant_create", got)
	}
}

func TestClassify_UnclassifiedIsReportedAsSuch(t *testing.T) {
	got, classified := classify(errors.New("something nobody classified"))
	if classified {
		t.Error("a plain error was reported as classified")
	}
	if got != classInternal {
		t.Errorf("an unclassified error's class = %v, want internal (70), never negative (1)", got)
	}
}

func TestClassify_NilIsOK(t *testing.T) {
	if got, classified := classify(nil); got != classOK || !classified {
		t.Errorf("classify(nil) = %v, %v; want ok, true", got, classified)
	}
}

// The old helpers keep working, so no existing call site changes in X0.
func TestUsageHelpersStillWork(t *testing.T) {
	if !isUsageError(usageErrorf("x")) {
		t.Error("isUsageError(usageErrorf) = false")
	}
	if !isUsageError(wrapUsage(errors.New("x"))) {
		t.Error("wrapUsage did not make a usage error")
	}
	if isUsageError(dataErrorf("x")) || isUsageError(errors.New("x")) {
		t.Error("isUsageError is true for a non-usage error")
	}
	if wrapUsage(nil) != nil {
		t.Error("wrapUsage(nil) != nil")
	}
	if got := wrapUsage(flag.ErrHelp); got != flag.ErrHelp {
		t.Errorf("wrapUsage(flag.ErrHelp) = %v, want it unchanged", got)
	}
}

// An error nothing classified exits 70, never 1 (DR-0047 item 4): a site nobody
// classified must show up, not hide as a negative answer. X0 kept the legacy 1
// while the sites were unclassified; X2 classified them and flipped this.
func TestExitCodeFor_UnclassifiedIsInternal(t *testing.T) {
	if got := exitCodeFor(errors.New("plain")); got != classInternal {
		t.Errorf("exitCodeFor(plain) = %v, want internal (70)", got)
	}
	if got := exitCodeFor(usageErrorf("x")); got != classUsage {
		t.Errorf("exitCodeFor(usage) = %v, want usage", got)
	}
	if got := exitCodeFor(notFoundf("x")); got != classNegative {
		t.Errorf("exitCodeFor(notFound) = %v, want negative", got)
	}
}

// ─── dispatch and the JSON envelope use the classifier ───────────────────────

func dispatchFailing(t *testing.T, jsonOut bool, failure error) (int, string, string) {
	t.Helper()
	vs := map[string]verbFunc{
		"boom": func(*knowledge.KnowledgeBase, *DebugLog, bool, []string, io.Writer) error { return failure },
	}
	var out, errOut bytes.Buffer
	code := dispatch(vs, nil, nil, jsonOut, []string{"boom"}, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestDispatch_ExitsWithTheClassOfTheError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		want    int
	}{
		{"usage", usageErrorf("bad"), 2},
		{"negative", notFoundf("nope"), 1},
		{"data", dataErrorf("bad content"), 65},
		{"no input", noInputf("missing"), 66},
		{"unavailable", unavailablef("down"), 69},
		{"cant create", cantCreatef("exists"), 73},
		{"io", ioErrorf("disk"), 74},
		{"a real missing file", fmt.Errorf("read: %w", fs.ErrNotExist), 66},
		{"a plain error", errors.New("plain"), 70},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, _ := dispatchFailing(t, false, tc.failure)
			if code != tc.want {
				t.Errorf("exit %d, want %d", code, tc.want)
			}
			if out != "" {
				t.Errorf("stdout = %q, want it empty on error", out)
			}
		})
	}
}

// A help request that reaches dispatch as flag.ErrHelp is answered with the
// verb's page (a real verb name, so there is a page) and exits 0.
func TestDispatch_HelpIsStillNotAnError(t *testing.T) {
	vs := map[string]verbFunc{
		"project": func(*knowledge.KnowledgeBase, *DebugLog, bool, []string, io.Writer) error { return flag.ErrHelp },
	}
	var out, errOut bytes.Buffer
	code := dispatch(vs, nil, nil, false, []string{"project"}, &out, &errOut)
	if code != 0 {
		t.Errorf("flag.ErrHelp exited %d, want 0", code)
	}
	if out.Len() == 0 || errOut.Len() != 0 {
		t.Errorf("want the verb's page on stdout and nothing on stderr; got %d bytes / %q", out.Len(), errOut.String())
	}
}

func TestPrintError_JSONEnvelopeCarriesClassAndCode(t *testing.T) {
	for _, tc := range []struct {
		failure error
		class   string
		code    int
	}{
		{usageErrorf("usage: x"), "usage", 2},
		{notFoundf("nope"), "negative", 1},
		{dataErrorf("bad"), "data", 65},
		{noInputf("gone"), "no_input", 66},
		{errors.New("plain"), "internal", 70},
	} {
		code, out, errOut := dispatchFailing(t, true, tc.failure)
		var env struct {
			Error string `json:"error"`
			Class string `json:"class"`
			Code  int    `json:"code"`
		}
		if err := json.Unmarshal([]byte(errOut), &env); err != nil {
			t.Fatalf("stderr is not JSON: %q (%v)", errOut, err)
		}
		if env.Error != tc.failure.Error() || env.Class != tc.class || env.Code != tc.code {
			t.Errorf("envelope = %+v, want error %q class %q code %d", env, tc.failure.Error(), tc.class, tc.code)
		}
		if code != env.Code {
			t.Errorf("exit code %d disagrees with the envelope's code %d", code, env.Code)
		}
		if out != "" {
			t.Errorf("stdout = %q, want it empty", out)
		}
	}
}

// The plain-text form is unchanged: "kb: <message>", nothing else added.
func TestPrintError_PlainTextIsUnchanged(t *testing.T) {
	_, _, errOut := dispatchFailing(t, false, dataErrorf("bad record"))
	if errOut != "kb: bad record\n" {
		t.Errorf("stderr = %q, want %q", errOut, "kb: bad record\n")
	}
}

// ─── X1: the library's sentinels and SQLite errors ───────────────────────────

func TestClassify_LibrarySentinels(t *testing.T) {
	kb := openTestKB(t)
	for _, tc := range []struct {
		name string
		err  error
		want exitClass
	}{
		{"not found", kb.SetProjectStatus("nosuch", "active"), classNegative},
		{"wrapped not found", fmt.Errorf("project: %w", kb.RenameConcept("nosuch", "x")), classNegative},
		{"a bad value", kb.SetProjectStatus("nosuch", "bogus"), classUsage},
		{"a concept still linked", &knowledge.ConceptInUseError{}, classNegative},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("setup: want an error")
			}
			got, classified := classify(tc.err)
			if !classified || got != tc.want {
				t.Errorf("classify(%v) = %v, %v; want %v, true", tc.err, got, classified, tc.want)
			}
		})
	}
}

// A file verb reclassifies the library's invalid-value error as content: the
// outermost class wins (DR-0047 item 3).
func TestClassify_FileVerbCanReclassifyInvalidAsData(t *testing.T) {
	kb := openTestKB(t)
	err := kb.SetProjectStatus("nosuch", "bogus")
	if got, _ := classify(err); got != classUsage {
		t.Fatalf("an argument's invalid value = %v, want usage", got)
	}
	if got, _ := classify(classedAs(classData, err)); got != classData {
		t.Errorf("the same error from file content = %v, want data", got)
	}
}

func TestSqliteClass_ByResultCode(t *testing.T) {
	for _, tc := range []struct {
		code int
		want exitClass
	}{
		{5, classTempFail},            // SQLITE_BUSY
		{6, classTempFail},            // SQLITE_LOCKED
		{5 | (1 << 8), classTempFail}, // an extended BUSY code (BUSY_RECOVERY)
		{11, classData},               // SQLITE_CORRUPT
		{26, classData},               // SQLITE_NOTADB
		{10, classIO},                 // SQLITE_IOERR
		{13, classIO},                 // SQLITE_FULL
		{14, classIO},                 // SQLITE_CANTOPEN
		{19, classIO},                 // SQLITE_CONSTRAINT: any other code is io
		{1, classIO},                  // SQLITE_ERROR
	} {
		if got := sqliteClass(tc.code); got != tc.want {
			t.Errorf("sqliteClass(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

// Real errors from the driver in use, not hand-built ones.
func TestClassify_RealSqliteErrors(t *testing.T) {
	dir := t.TempDir()

	notADB := filepath.Join(dir, "text.db")
	if err := os.WriteFile(notADB, []byte("this is not a database, just text, long enough to be read as a header....\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", notADB)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE t (a)`)
	db.Close()
	if err == nil {
		t.Fatal("writing to a text file as a database should fail")
	}
	if got, classified := classify(err); !classified || got != classData {
		t.Errorf("classify(not a database: %v) = %v, %v; want data", err, got, classified)
	}

	lockedPath := filepath.Join(dir, "locked.db")
	a, err := sql.Open("sqlite", lockedPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.SetMaxOpenConns(1)
	if _, err := a.Exec(`CREATE TABLE t (a)`); err != nil {
		t.Fatal(err)
	}
	holder, err := a.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := holder.ExecContext(context.Background(), `BEGIN EXCLUSIVE`); err != nil {
		t.Fatal(err)
	}
	b, err := sql.Open("sqlite", lockedPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, err = b.Exec(`INSERT INTO t VALUES (1)`)
	if err == nil {
		t.Fatal("writing while another connection holds an exclusive lock should fail")
	}
	if got, classified := classify(err); !classified || got != classTempFail {
		t.Errorf("classify(locked: %v) = %v, %v; want temp_fail", err, got, classified)
	}
}
