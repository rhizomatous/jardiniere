package proxy

import (
	"sync"
	"testing"
)

func entry(host string, allowed bool) Entry {
	return Entry{Target: Target{Host: host, Port: 443}, Allowed: allowed}
}

func TestLogStampsEachEntry(t *testing.T) {
	l := NewLog(10)
	first := l.Record(entry("a.test", true))
	second := l.Record(entry("b.test", false))

	if first.Seq == 0 || second.Seq <= first.Seq {
		t.Errorf("sequences = %d, %d; want increasing and non-zero", first.Seq, second.Seq)
	}
	if first.At.IsZero() {
		t.Error("an entry with no time cannot be shown in order")
	}
}

func TestLogKeepsOnlyTheMostRecent(t *testing.T) {
	// the daemon outlives many sandboxes, so the log has to be bounded or its
	// memory grows with how chatty the agent is.
	l := NewLog(3)
	for _, host := range []string{"a", "b", "c", "d", "e"} {
		l.Record(entry(host, true))
	}

	got := l.Recent(0)
	if len(got) != 3 {
		t.Fatalf("kept %d entries, want the limit of 3", len(got))
	}
	if got[0].Target.Host != "c" || got[2].Target.Host != "e" {
		t.Errorf("kept %v, want the last three", []string{got[0].Target.Host, got[1].Target.Host, got[2].Target.Host})
	}
}

func TestLogSinceReturnsOnlyWhatIsNew(t *testing.T) {
	// this is what lets the dashboard re-read on a tick without redrawing
	// everything it already had.
	l := NewLog(10)
	l.Record(entry("a.test", true))
	mark := l.Record(entry("b.test", true))
	l.Record(entry("c.test", false))

	got := l.Since(mark.Seq)
	if len(got) != 1 || got[0].Target.Host != "c.test" {
		t.Errorf("Since(%d) = %+v, want only c.test", mark.Seq, got)
	}
	if all := l.Since(0); len(all) != 3 {
		t.Errorf("Since(0) returned %d entries, want everything", len(all))
	}
}

func TestLogSinceSurvivesEvictedEntries(t *testing.T) {
	// a reader that was away longer than the buffer is deep asks for a
	// sequence that has been dropped. It should get what remains, not nothing
	// and not a panic.
	l := NewLog(2)
	first := l.Record(entry("a.test", true))
	l.Record(entry("b.test", true))
	l.Record(entry("c.test", true))

	got := l.Since(first.Seq)
	if len(got) != 2 {
		t.Errorf("Since(%d) = %d entries, want the two still held", first.Seq, len(got))
	}
}

func TestLogRecentClampsToWhatItHas(t *testing.T) {
	l := NewLog(10)
	l.Record(entry("a.test", true))

	if got := l.Recent(50); len(got) != 1 {
		t.Errorf("Recent(50) = %d entries, want the one there is", len(got))
	}
	if got := l.Recent(-1); len(got) != 1 {
		t.Errorf("Recent(-1) = %d entries, want it treated as everything", len(got))
	}
}

func TestLogIsSafeUnderConcurrentUse(t *testing.T) {
	// the proxy records from one goroutine per connection while the dashboard
	// reads on its own tick.
	l := NewLog(100)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 50 {
				l.Record(entry("a.test", true))
			}
		}()
		go func() {
			defer wg.Done()
			for range 50 {
				_ = l.Since(0)
				_ = l.Recent(10)
			}
		}()
	}
	wg.Wait()

	if got := l.Recent(0); len(got) != 100 {
		t.Errorf("kept %d entries, want the limit of 100", len(got))
	}
}

func TestNewLogRejectsAnUnusableLimit(t *testing.T) {
	if NewLog(0).limit != defaultLogLimit {
		t.Error("a zero limit would keep nothing at all")
	}
	if NewLog(-5).limit != defaultLogLimit {
		t.Error("a negative limit should fall back to the default")
	}
}
