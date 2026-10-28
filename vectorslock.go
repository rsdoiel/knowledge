package knowledge

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	sqlite "github.com/glebarez/go-sqlite"
)

/** ErrLocked marks an error raised because another run holds the vector index:
 * two kb embed runs would write the same file. Trying again later may work, so
 * kb exits 75 for it (workspace DR-0003 class "temp_fail"). Match it with
 * errors.Is.
 *
 * Example:
 *   if _, err := v.Lock(); errors.Is(err, knowledge.ErrLocked) {
 *       // another kb embed is running; try again later
 *   }
 */
var ErrLocked = errors.New("vector index locked")

/** LockStaleAfter is how long a claim on the vector index may go without a
 * heartbeat before another run may take it over: a run that crashed or was
 * killed does not block the index for ever. A live run refreshes its claim after
 * every batch, which takes seconds.
 */
const LockStaleAfter = 5 * time.Minute

/** VectorsLock is a claim on a vector sidecar, held by one run. The claim is a
 * row in the sidecar itself, not an operating-system lock, so it behaves the
 * same on every platform and in a directory that is synced or on a network.
 */
type VectorsLock struct {
	v     *Vectors
	owner string
}

/** Lock claims the sidecar for writing. A claim made by another run blocks it
 * until that run releases or its heartbeat is older than LockStaleAfter.
 *
 * Returns:
 *   *VectorsLock — the claim; call Heartbeat as work goes on and Release when done
 *   error — ErrLocked, with who holds it and since when; a database error otherwise
 *
 * Example:
 *   l, err := v.Lock()
 *   if err != nil { return err }
 *   defer l.Release()
 */
func (v *Vectors) Lock() (*VectorsLock, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("knowledge: vector index lock: %w", err)
	}
	owner := hex.EncodeToString(raw[:])
	now := v.now()
	res, err := v.db.Exec(`
		INSERT INTO vectors_lock(id, owner, claimed_at, heartbeat_at) VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET owner = excluded.owner, claimed_at = excluded.claimed_at,
		                              heartbeat_at = excluded.heartbeat_at
		WHERE vectors_lock.heartbeat_at < ?`,
		owner, now.UnixMilli(), now.UnixMilli(), now.Add(-LockStaleAfter).UnixMilli())
	if err != nil {
		var se *sqlite.Error
		if errors.As(err, &se) && (se.Code()&0xff == 5 || se.Code()&0xff == 6) { // BUSY, LOCKED
			return nil, markedf(ErrLocked, "the vector index %s is busy: another kb is writing it; try again later", v.path)
		}
		return nil, fmt.Errorf("knowledge: vector index lock: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var claimed, beat int64
		if v.db.QueryRow(`SELECT claimed_at, heartbeat_at FROM vectors_lock WHERE id = 1`).Scan(&claimed, &beat) == nil {
			return nil, markedf(ErrLocked, "another kb embed has been building %s since %s (last active %s): try again later",
				v.path, time.UnixMilli(claimed).Format("15:04:05"), time.UnixMilli(beat).Format("15:04:05"))
		}
		return nil, markedf(ErrLocked, "another kb embed is building %s: try again later", v.path)
	}
	return &VectorsLock{v: v, owner: owner}, nil
}

/** Heartbeat refreshes the claim so it does not go stale, and confirms this run
 * still holds it.
 *
 * Returns:
 *   error — ErrLocked when the claim was taken over after it went stale (the
 *     run should stop without writing); a database error otherwise
 *
 * Example:
 *   if err := l.Heartbeat(); err != nil { return err }
 */
func (l *VectorsLock) Heartbeat() error {
	res, err := l.v.db.Exec(`UPDATE vectors_lock SET heartbeat_at = ? WHERE id = 1 AND owner = ?`,
		l.v.now().UnixMilli(), l.owner)
	if err != nil {
		return fmt.Errorf("knowledge: vector index lock: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return markedf(ErrLocked, "this run lost its claim on %s: another kb embed took it over after it stalled", l.v.path)
	}
	return nil
}

/** Release gives up the claim. It only drops a claim this run still owns, so
 * releasing after the claim was taken over, or releasing twice, leaves the new
 * owner's claim alone.
 *
 * Returns:
 *   error — a database error
 *
 * Example:
 *   defer l.Release()
 */
func (l *VectorsLock) Release() error {
	if _, err := l.v.db.Exec(`DELETE FROM vectors_lock WHERE id = 1 AND owner = ?`, l.owner); err != nil {
		return fmt.Errorf("knowledge: vector index lock: %w", err)
	}
	return nil
}
