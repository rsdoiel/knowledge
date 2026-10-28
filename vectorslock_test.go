package knowledge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rsdoiel/knowledge/embedtest"
)

// Two writers must not build the same sidecar at once (semantic-search-plan.md
// T3.6): a one-row lock in the sidecar with a heartbeat, so it behaves the same
// on every platform and over a synced directory, and a crashed run's claim goes
// stale instead of blocking forever.

var t0 = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

// twoHandles opens the same sidecar twice, as two kb processes would.
func twoHandles(t *testing.T) (a, b *Vectors, clockA, clockB *time.Time) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "knowledge.db")
	a, err := CreateVectors(db, embedtest.New())
	if err != nil {
		t.Fatal(err)
	}
	b, err = OpenVectors(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close(); b.Close() })
	ca, cb := t0, t0
	a.now = func() time.Time { return ca }
	b.now = func() time.Time { return cb }
	return a, b, &ca, &cb
}

func TestLock_ASecondClaimIsErrLockedUntilTheFirstReleases(t *testing.T) {
	a, b, _, _ := twoHandles(t)
	la, err := a.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Lock(); !errors.Is(err, ErrLocked) {
		t.Fatalf("second Lock: err = %v, want ErrLocked", err)
	}
	if err := la.Release(); err != nil {
		t.Fatal(err)
	}
	lb, err := b.Lock()
	if err != nil {
		t.Fatalf("Lock after Release: %v", err)
	}
	lb.Release()
}

func TestLock_ReleasingTwiceIsHarmless(t *testing.T) {
	a, b, _, _ := twoHandles(t)
	la, _ := a.Lock()
	la.Release()
	lb, _ := b.Lock()
	if err := la.Release(); err != nil { // la no longer owns it; must not release lb's claim
		t.Errorf("second Release: %v", err)
	}
	if _, err := a.Lock(); !errors.Is(err, ErrLocked) {
		t.Errorf("b's claim was released by a's second Release: %v", err)
	}
	lb.Release()
}

func TestLock_AStaleClaimIsTakenOverAndTheOldOwnerFindsOut(t *testing.T) {
	a, b, _, clockB := twoHandles(t)
	la, _ := a.Lock()
	*clockB = t0.Add(LockStaleAfter - time.Second)
	if _, err := b.Lock(); !errors.Is(err, ErrLocked) {
		t.Fatalf("just before it goes stale: err = %v, want ErrLocked", err)
	}
	*clockB = t0.Add(LockStaleAfter + time.Second)
	lb, err := b.Lock()
	if err != nil {
		t.Fatalf("a stale claim was not taken over: %v", err)
	}
	if err := la.Heartbeat(); !errors.Is(err, ErrLocked) {
		t.Errorf("the old owner's Heartbeat: err = %v, want ErrLocked (it lost the lock)", err)
	}
	if err := la.Release(); err != nil {
		t.Errorf("the old owner's Release: %v", err)
	}
	if _, err := a.Lock(); !errors.Is(err, ErrLocked) {
		t.Error("the old owner's Release dropped the new owner's claim")
	}
	lb.Release()
}

func TestLock_AHeartbeatKeepsTheClaimFresh(t *testing.T) {
	a, b, clockA, clockB := twoHandles(t)
	la, _ := a.Lock()
	*clockA = t0.Add(4 * time.Minute)
	if err := la.Heartbeat(); err != nil {
		t.Fatal(err)
	}
	*clockB = t0.Add(7 * time.Minute) // 7 minutes since the claim, 3 since the heartbeat
	if _, err := b.Lock(); !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked: the heartbeat should have kept it fresh", err)
	}
}

func TestEmbedIndex_WhileAnotherRunHoldsTheLockIsErrLockedAndEmbedsNothing(t *testing.T) {
	f, v, e := indexFixture(t)
	other, err := OpenVectors(trimVectors(v.Path()))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	l, err := other.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{}); !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want ErrLocked", err)
	}
	if e.Calls() != 0 {
		t.Error("the embedder was called without the lock")
	}
}

func trimVectors(p string) string { return p[:len(p)-len(".vectors.db")] + ".db" }

func TestEmbedIndex_ReleasesTheLockWhenItEndsHoweverItEnds(t *testing.T) {
	f, v, e := indexFixture(t)
	other, _ := OpenVectors(trimVectors(v.Path()))
	defer other.Close()
	free := func(when string) {
		t.Helper()
		l, err := other.Lock()
		if err != nil {
			t.Fatalf("after %s the lock is still held: %v", when, err)
		}
		l.Release()
	}

	e.FailAfter(0)
	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{}); err == nil {
		t.Fatal("expected the embedder's failure")
	}
	free("a failed run")

	e.FailAfter(-1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.kb.EmbedIndex(ctx, v, e, EmbedOptions{})
	free("a cancelled run")

	if _, err := f.kb.EmbedIndex(context.Background(), v, e, EmbedOptions{}); err != nil {
		t.Fatal(err)
	}
	free("a finished run")
}

// lockProbe tries to take the lock from a second handle, with its own clock, in
// the middle of the first handle's run.
type lockProbe struct {
	*embedtest.Fake
	onCall func(n int)
	n      int
}

func (p *lockProbe) Embed(texts []string) ([][]float32, error) {
	p.n++
	p.onCall(p.n)
	return p.Fake.Embed(texts)
}

func TestEmbedIndex_HeartbeatsAfterEachBatch(t *testing.T) {
	f, v, _ := indexFixture(t)
	clock := t0
	v.now = func() time.Time { return clock }
	other, _ := OpenVectors(trimVectors(v.Path()))
	defer other.Close()
	other.now = func() time.Time { return t0.Add(6 * time.Minute) }

	var tookOver error
	probe := &lockProbe{Fake: embedtest.New()}
	probe.onCall = func(n int) {
		switch n {
		case 1:
			clock = t0.Add(4 * time.Minute) // time passes during the first batch; its heartbeat should record this
		case 2:
			// 6 minutes after the claim but 2 after the last heartbeat: still held.
			if l, err := other.Lock(); err == nil {
				l.Release()
				tookOver = errors.New("the claim went stale although the run was heartbeating")
			}
		}
	}
	if _, err := f.kb.EmbedIndex(context.Background(), v, probe, EmbedOptions{BatchSize: 2}); err != nil {
		t.Fatal(err)
	}
	if probe.n < 2 {
		t.Fatalf("only %d batch(es); the test needs two", probe.n)
	}
	if tookOver != nil {
		t.Error(tookOver)
	}
}

func TestEmbedIndex_StopsWhenItFindsItsLockWasTakenOver(t *testing.T) {
	f, v, _ := indexFixture(t)
	clock := t0
	v.now = func() time.Time { return clock }
	other, _ := OpenVectors(trimVectors(v.Path()))
	defer other.Close()
	other.now = func() time.Time { return t0.Add(LockStaleAfter + time.Minute) }

	probe := &lockProbe{Fake: embedtest.New()}
	probe.onCall = func(n int) {
		if n == 1 {
			if _, err := other.Lock(); err != nil { // the first run stalled; the other takes over
				t.Errorf("take over: %v", err)
			}
		}
	}
	_, err := f.kb.EmbedIndex(context.Background(), v, probe, EmbedOptions{BatchSize: 2})
	if !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked once the lock was lost", err)
	}
	if probe.n != 1 {
		t.Errorf("the run went on for %d batches after losing the lock", probe.n)
	}
}
