package jobscheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNewMemoryScheduler(t *testing.T) {
	s := NewMemoryScheduler()
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

func TestAddAndListJobs(t *testing.T) {
	s := NewMemoryScheduler()

	id1, err := s.Add("@every 1h", func() {})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	id2, err := s.Add("@every 2h", func() {})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries := s.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].ID != id1 || entries[1].ID != id2 {
		t.Error("entry IDs don't match")
	}
}

func TestRemoveJob(t *testing.T) {
	s := NewMemoryScheduler()

	id, _ := s.Add("@every 1h", func() {})
	s.Remove(id)

	entries := s.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(entries))
	}
}

func TestInvalidCronSpec(t *testing.T) {
	s := NewMemoryScheduler()
	_, err := s.Add("not a valid cron", func() {})
	if err == nil {
		t.Fatal("expected error for invalid cron spec")
	}
}

func TestSchedulerExecutesJob(t *testing.T) {
	s := NewMemoryScheduler()
	var counter int32

	_, err := s.Add("@every 1s", func() {
		atomic.AddInt32(&counter, 1)
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	s.Start()

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("job did not execute within 5s, counter=%d", atomic.LoadInt32(&counter))
		case <-ticker.C:
			if atomic.LoadInt32(&counter) >= 1 {
				s.Stop()
				return
			}
		}
	}
}

func TestStopPreventsExecution(t *testing.T) {
	s := NewMemoryScheduler()
	var counter int32

	_, _ = s.Add("@every 1s", func() {
		atomic.AddInt32(&counter, 1)
	})

	s.Start()

	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("job did not execute before stop, counter=%d", atomic.LoadInt32(&counter))
		case <-ticker.C:
			if atomic.LoadInt32(&counter) >= 1 {
				goto stopPhase
			}
		}
	}

stopPhase:
	s.Stop()
	afterStop := atomic.LoadInt32(&counter)

	time.Sleep(2 * time.Second)
	afterWait := atomic.LoadInt32(&counter)

	if afterWait > afterStop+1 {
		t.Errorf("expected no significant executions after stop: afterStop=%d afterWait=%d", afterStop, afterWait)
	}
}

func TestNoDuplicateIDs(t *testing.T) {
	s := NewMemoryScheduler()

	ids := make(map[int]bool)
	for i := 0; i < 10; i++ {
		id, err := s.Add("@every 1h", func() {})
		if err != nil {
			t.Fatalf("Add #%d failed: %v", i, err)
		}
		if ids[int(id)] {
			t.Fatalf("duplicate entry ID: %d", id)
		}
		ids[int(id)] = true
	}
}
