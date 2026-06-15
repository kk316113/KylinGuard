package logtrace

import "sync"

type Store struct {
	mu     sync.Mutex
	traces []ToolTrace
}

func NewStore() *Store {
	return &Store{traces: []ToolTrace{}}
}

func (s *Store) Add(trace ToolTrace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces = append(s.traces, trace)
}

func (s *Store) List() []ToolTrace {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make([]ToolTrace, len(s.traces))
	copy(copied, s.traces)
	return copied
}
