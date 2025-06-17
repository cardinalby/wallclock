package wallclock

import (
	"sync"
	"time"
)

type scheduler struct {
	mu              sync.Mutex
	alarms          alarms
	timer           *time.Timer
	loopDoneSignal  chan struct{}
	jumpForwardMon  jumpForwardMon
	jumpMonMaxDelay time.Duration
}

func newScheduler() *scheduler {
	scheduler := &scheduler{
		alarms: newAlarms(),
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

	isSoonestFireAt, isSoonestExpiresAt := s.alarms.Add(a)
	if isSoonestFireAt {
		s.setTimer(remainingDuration)
		if s.alarms.Len() == 1 {
			s.startLoop()
		}
	}
	if isSoonestExpiresAt {
		s.jumpForwardMon.Set(a.AllowedDelay)
	}
}

func (s *scheduler) RemoveAlarm(a *alarm) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	isDeleted, isSoonestFireAt, isSoonestExpiresAt := s.alarms.Delete(a)
	if !isDeleted {
		// it's ok: while we were waiting for the lock, the alarm was already fired and removed
		return false
	}
	if s.alarms.Len() == 0 {
		s.stopLoop()
		return true
	}
	if isSoonestFireAt {
		s.tick(gotTime(time.Now()))
	}
	if isSoonestExpiresAt {
		if alarm := s.alarms.GetWithSoonestExpTime(); alarm != nil {
			s.jumpForwardMon.Set(alarm.AllowedDelay)
		} else if isTestingBuild {
			panic("isSoonestExpiresAt is true, but there are no alarms left")
		}
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
			s.tick(gotTime(firedAt))
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
	s.tick(gotTime(time.Now()))
}

// thread-unsafe
// tick pops alarms from the alarmsByFireAtHeap:
// - fires alarms that are already expired
// - (re)schedules the timer to fire the alarm closest in the future and returns
// - stops the loop if there are no alarms left
func (s *scheduler) tick(now time.Time) {
	shouldResetJumpForwardMon := false
	for s.alarms.Len() > 0 {
		topAlarm := s.alarms.GetWithSoonestFireAt()
		remainingDuration := topAlarm.WallFireAt.Sub(now)
		if remainingDuration <= 0 {
			// alarm is already expired (time jumped over the scheduled wall time), fire it immediately
			// remove topAlarm
			if _, _, isSoonestExpTime := s.alarms.Delete(topAlarm); isSoonestExpTime {
				shouldResetJumpForwardMon = true
			}
			if isTestingBuild {
				firedAlarms = append(firedAlarms, topAlarm)
			}
			topAlarm.c <- now
		} else {
			// is in the future, schedule it
			s.setTimer(remainingDuration)
			if shouldResetJumpForwardMon {
				if withSoonestExpTime := s.alarms.GetWithSoonestExpTime(); withSoonestExpTime != nil {
					s.jumpForwardMon.Set(withSoonestExpTime.AllowedDelay)
				} else if isTestingBuild {
					panic("shouldResetJumpForwardMon is true, but there are no alarms left")
				}
			}
			return
		}
	}
	// alarms is empty
	s.stopLoop()
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
