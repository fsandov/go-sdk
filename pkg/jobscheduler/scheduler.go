package jobscheduler

import (
	"sync"

	"github.com/robfig/cron/v3"
)

type JobFunc func()

type Scheduler interface {
	Add(spec string, job JobFunc) (id cron.EntryID, err error)
	Remove(id cron.EntryID)
	Start()
	Stop()
	List() []cron.Entry
}

type memoryScheduler struct {
	c  *cron.Cron
	mu sync.RWMutex
}

func NewMemoryScheduler() Scheduler {
	return &memoryScheduler{
		c: cron.New(),
	}
}

func (s *memoryScheduler) Add(spec string, job JobFunc) (cron.EntryID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.c.AddFunc(spec, job)
}

func (s *memoryScheduler) Remove(id cron.EntryID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.Remove(id)
}

func (s *memoryScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.Start()
}

func (s *memoryScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := s.c.Stop()
	<-ctx.Done()
}

func (s *memoryScheduler) List() []cron.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.c.Entries()
}
