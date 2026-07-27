package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// debugLogPathCounter guarantees DefaultDebugLogPath is unique even when
// called repeatedly within one process faster than the clock ticks (see
// the comment on DefaultDebugLogPath) -- a real failure mode found by
// TestDefaultDebugLogPath_UniqueAcrossRapidCalls, not a hypothetical one.
var debugLogPathCounter int64

// DebugLog appends JSONL records to an open file, for --debug. Every
// method is safe to call on a nil *DebugLog (a no-op), so --debug=false
// costs one nil check per call site rather than an "if debug" conditional
// scattered through the code -- same pattern as harvey's own DebugLog and
// clasm's internal/debuglog, which this mirrors.
type DebugLog struct {
	mu   sync.Mutex
	file *os.File
}

// NewDebugLog opens path for writing (creating or truncating it).
func NewDebugLog(path string) (*DebugLog, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return &DebugLog{file: f}, nil
}

// DefaultDebugLogPath returns a timestamped JSONL path in the current
// working directory, e.g. "kb-debug-20260727-153012-42-1.jsonl" (PID 42,
// 1st call this process). Includes the PID and a per-process call counter
// alongside the timestamp -- not clasm's original second-only format --
// because kb's verbs are typically many short-lived processes rather than
// one long session like clasm's, so two invocations within the same
// second are a real, likely occurrence. Timestamp precision alone isn't a
// reliable guarantee here regardless of how many digits are requested:
// TestDefaultDebugLogPath_UniqueAcrossRapidCalls found that even
// microsecond formatting collides under a tight loop on this hardware's
// clock resolution, and NewDebugLog opens with O_TRUNC, so a collision
// silently destroys the earlier run's log rather than erroring. The
// counter is the only part of this that's unconditionally unique
// regardless of clock behavior; PID and timestamp are included for
// readability/traceability, not as the uniqueness guarantee itself.
func DefaultDebugLogPath() string {
	n := atomic.AddInt64(&debugLogPathCounter, 1)
	return fmt.Sprintf("kb-debug-%s-%d-%d.jsonl", time.Now().Format("20060102-150405"), os.Getpid(), n)
}

// Log writes one JSON object containing fields plus "time" (RFC3339Nano,
// UTC) and "event", followed by a newline. A nil *DebugLog, a nil fields
// map, or a marshal error are all silently ignored -- this is a
// debugging aid, not a path any workflow should fail on.
func (dl *DebugLog) Log(event string, fields map[string]any) {
	if dl == nil {
		return
	}
	record := make(map[string]any, len(fields)+2)
	maps.Copy(record, fields)
	record["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	record["event"] = event

	line, err := json.Marshal(record)
	if err != nil {
		return
	}
	line = append(line, '\n')

	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.file.Write(line)
}

// Path returns the path of the open log file, or "" for a nil *DebugLog.
func (dl *DebugLog) Path() string {
	if dl == nil {
		return ""
	}
	return dl.file.Name()
}

// Close closes the underlying file. A nil *DebugLog returns nil.
func (dl *DebugLog) Close() error {
	if dl == nil {
		return nil
	}
	return dl.file.Close()
}

// logKBCall runs call, logs one "kb_call" record to dl (method, params,
// duration_ms, and either result or error), and returns call's result
// unchanged. A nil dl makes this transparent -- one nil check inside
// Log, no branch here at all.
func logKBCall[T any](dl *DebugLog, method string, params any, call func() (T, error)) (T, error) {
	start := time.Now()
	result, err := call()

	fields := map[string]any{
		"method":      method,
		"params":      params,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if err != nil {
		fields["error"] = err.Error()
	} else {
		fields["result"] = result
	}
	dl.Log("kb_call", fields)

	return result, err
}

// logKBCallErr is logKBCall for methods that return only an error, so
// call sites don't need to fabricate a dummy result type. Logs "result":
// true on success (no error) instead of omitting the field entirely, so
// every kb_call record has a consistent success/failure shape to filter
// or grep on.
func logKBCallErr(dl *DebugLog, method string, params any, call func() error) error {
	_, err := logKBCall(dl, method, params, func() (bool, error) {
		return true, call()
	})
	return err
}
