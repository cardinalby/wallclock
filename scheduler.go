package alarm

import (
	"sync"
	"time"
)

type scheduler struct {
	mu              sync.Mutex
	heap            *indexedHeap[*alarm]
	timer           *time.Timer
	loopDoneSignal  chan struct{}
	jumpForwardMon  jumpForwardMon
	jumpMonMaxDelay time.Duration
}

func newScheduler() *scheduler {
	scheduler := &scheduler{
		heap: newIndexedHeap[*alarm](func(x, y *alarm) bool {
			// keep alarms that fire earlier at the top of the heap
			return x.WallFireAt.Before(y.WallFireAt)
		}),
	}
	scheduler.jumpForwardMon = jumpForwardMon{
		OnJump: scheduler.onWcJumpForward,
	}
	return scheduler
}

var globalScheduler = newScheduler()

func (s *scheduler) AddFutureAlarm(a *alarm, remainingDuration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	isPushed, isTop, _ := s.heap.Push(a)
	if !isPushed {
		panic("alarm already exists in the scheduler")
	}
	if isTop {
		s.scheduleAlarm(remainingDuration, a.AllowedDelay)
		if s.heap.Len() == 1 {
			s.startLoop()
		}
	}
}

func (s *scheduler) RemoveAlarm(a *alarm) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	isDeleted, isTop := s.heap.Delete(a)
	if !isDeleted {
		// it's ok: while we were waiting for the lock, the alarm was already fired and removed
		return false
	}
	if s.heap.Len() == 0 {
		s.stopLoop()
		return true
	}
	if isTop {
		s.tick(time.Now())
	}
	return true
}

// thread-unsafe
func (s *scheduler) startLoop() {
	s.loopDoneSignal = make(chan struct{})
	if isTestingBuild {
		schedulerRunningLoops.Add(1)
	}
	go s.runLoop(s.loopDoneSignal)
}

// thread-unsafe
func (s *scheduler) runLoop(doneSignal chan struct{}) {
	if isTestingBuild {
		defer func() {
			schedulerRunningLoops.Add(-1)
		}()
	}
	for {
		select {
		case <-doneSignal:
			return
		case firedAt := <-s.timer.C:
			s.mu.Lock()
			s.tick(firedAt)
			s.mu.Unlock()
		}
	}
}

func (s *scheduler) onWcJumpForward() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loopDoneSignal == nil {
		return
	}
	// wall clock was adjusted to the future:
	// some alarms may turn out to be expired now or the timer need to be rescheduled to fire sooner
	s.tick(time.Now())
}

// thread-unsafe
// tick pops alarms from the heap:
// - fires alarms that are already expired
// - (re)schedules the timer to fire the alarm closest in the future and returns
// - stops the loop if there are no alarms left
func (s *scheduler) tick(now time.Time) {
	for s.heap.Len() > 0 {
		topAlarm := s.heap.Peek()
		remainingDuration := topAlarm.WallFireAt.Sub(now)
		if remainingDuration <= 0 {
			// alarm is already expired (time jumped over the scheduled wall time), fire it immediately
			topAlarm.c <- now
			if isTestingBuild {
				firedAlarms = append(firedAlarms, topAlarm)
			}
			s.heap.Pop() // remove topAlarm
		} else {
			s.scheduleAlarm(remainingDuration, topAlarm.AllowedDelay)
			return
		}
	}
	// heap is empty
	s.stopLoop()
}

// thread-unsafe
func (s *scheduler) scheduleAlarm(remainingDuration time.Duration, maxDelay time.Duration) {
	s.setTimer(remainingDuration)
	s.jumpForwardMon.Set(maxDelay)
}

// thread-unsafe
func (s *scheduler) stopLoop() {
	close(s.loopDoneSignal)
	s.loopDoneSignal = nil
	s.timer.Stop()
	s.jumpForwardMon.Stop()
}

// thread-unsafe
func (s *scheduler) setTimer(d time.Duration) {
	if s.timer == nil {
		s.timer = time.NewTimer(d)
	} else {
		s.timer.Stop()
		select {
		case <-s.timer.C:
		default:
		}
		s.timer.Reset(d)
	}
}
