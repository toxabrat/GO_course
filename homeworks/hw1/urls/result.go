package urls

import "sync"

type aggregationResult struct {
	mu     sync.RWMutex
	counts map[int]int
	done   bool
}

func (r *aggregationResult) GetResponsesCount(code int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.counts[code]
}

func (r *aggregationResult) GetResult() map[int]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[int]int, len(r.counts))
	for code, count := range r.counts {
		result[code] = count
	}
	return result
}

func (r *aggregationResult) Done() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.done
}

func (r *aggregationResult) addResponse(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.done {
		r.counts[code]++
	}
}

func (r *aggregationResult) finish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = true
}
