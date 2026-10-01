package game

import (
	"math/rand/v2"
	"sync"
	"time"
)

// task is a scheduled job. Cancelling it, with the world locked, stops it for good.
type task struct {
	cancelled bool
	stop      chan struct{}
	once      sync.Once
}

func newTask() *task { return &task{stop: make(chan struct{})} }

func (t *task) cancel() {
	if t == nil {
		return
	}
	t.cancelled = true
	t.once.Do(func() { close(t.stop) })
}

// later runs fn once in d, with the world locked (ThreadPoolManager.schedule).
func (s *Server) later(d time.Duration, fn func()) *task {
	t := newTask()
	time.AfterFunc(d, func() {
		s.visMu.Lock()
		defer s.visMu.Unlock()
		if !t.cancelled {
			fn()
		}
	})
	return t
}

// every runs fn after delay and then each period, with the world locked, until it is cancelled
// (ThreadPoolManager.scheduleAtFixedRate).
func (s *Server) every(delay, period time.Duration, fn func()) *task {
	t := newTask()
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-t.stop:
			return
		}
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			s.visMu.Lock()
			if t.cancelled {
				s.visMu.Unlock()
				return
			}
			fn()
			s.visMu.Unlock()
			select {
			case <-ticker.C:
			case <-t.stop:
				return
			}
		}
	}()
	return t
}

// rnd is Rnd.get(min, max): a whole number from min to max, both included; min if max is lower.
func rnd(min, max int32) int32 {
	if max <= min {
		return min
	}
	return min + rand.Int32N(max-min+1)
}

// chance is Rnd.get(0, 100) < percent.
func chance(percent float64) bool {
	return float64(rnd(0, 100)) < percent
}
