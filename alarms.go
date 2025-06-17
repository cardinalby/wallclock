package wallclock

import (
	"github.com/cardinalby/wallclock/internal/indexed_heap"
)

// alarms is not thread-safe and is used only by the scheduler
type alarms struct {
	fireAtHeap  *indexed_heap.IndexedHeap[*alarm]
	expTimeHeap *indexed_heap.IndexedHeap[*alarm]
}

func newAlarms() alarms {
	return alarms{
		fireAtHeap: indexed_heap.NewIndexedHeap[*alarm](
			func(x, y *alarm) bool {
				// keep alarms that fire earlier at the top of the heap
				return x.WallFireAt.Before(y.WallFireAt)
			},
		),
		expTimeHeap: indexed_heap.NewIndexedHeap[*alarm](
			func(x, y *alarm) bool {
				// keep alarms that expire earlier at the top of the heap
				switch x.wallExpiresAt().Compare(y.wallExpiresAt()) {
				case -1:
					return true
				case 0:
					// if expiration times are equal, prefer the one that has more lax allowed delay
					// (will require less frequent jump-forward checks)
					return x.AllowedDelay > y.AllowedDelay
				default:
					return false
				}
			},
		),
	}
}

func (a *alarms) Len() int {
	return a.fireAtHeap.Len()
}

func (a *alarms) GetWithSoonestFireAt() *alarm {
	return a.fireAtHeap.Peek()
}

func (a *alarms) GetWithSoonestExpTime() *alarm {
	return a.expTimeHeap.Peek()
}

func (a *alarms) Add(
	alarm *alarm,
) (
	isSoonestFireAt bool,
	isSoonestExpTime bool,
) {
	isPushed, isSoonestFireAt := a.fireAtHeap.Push(alarm)
	if isTestingBuild {
		if !isPushed {
			panic("alarm already exists")
		}
	}
	if alarm.AllowedDelay != 0 {
		// don't add to expTimeHeap if AllowedDelay is zero
		isPushed, isSoonestExpTime = a.expTimeHeap.Push(alarm)
		if isTestingBuild {
			if !isPushed {
				panic("alarm already exists")
			}
		}
	}
	return isSoonestFireAt, isSoonestExpTime
}

func (a *alarms) Delete(alarm *alarm) (
	isDeleted bool,
	isSoonestFireAt bool,
	isSoonestExpTime bool,
) {
	isDeleted, isSoonestFireAt = a.fireAtHeap.Delete(alarm)
	if !isDeleted {
		return false, false, false
	}
	isDeleted, isSoonestExpTime = a.expTimeHeap.Delete(alarm)
	// it's ok id isDeleted is false here, it means that the alarm was not in the expTimeHeap
	// because its AllowedDelay was zero
	return true, isSoonestFireAt, isSoonestExpTime
}
