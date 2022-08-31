package prepalert

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"
)

type Waiter struct {
	StartTime time.Time
	MinDelay  time.Duration
	MaxDelay  time.Duration
	Timeout   time.Duration
	Jitter    time.Duration

	delay time.Duration
	once  sync.Once
	rand  *rand.Rand
	timer *time.Timer
}

func (w *Waiter) Continue(ctx context.Context) bool {
	if time.Since(w.StartTime) >= w.Timeout {
		if w.timer != nil {
			w.timer.Stop()
			w.timer = nil
		}
		return false
	}
	if w.delay == 0 {
		w.delay = w.MinDelay
	} else {
		w.delay *= 2
		if w.delay > w.MaxDelay {
			w.delay = w.MaxDelay
		}
	}

	w.once.Do(func() {
		var seed int64
		if err := binary.Read(crand.Reader, binary.LittleEndian, &seed); err != nil {
			seed = time.Now().UnixNano() // fall back to timestamp
		}
		w.rand = rand.New(rand.NewSource(seed))
		w.timer = time.NewTimer(w.delay)
	})
	w.timer.Reset(w.delay + w.randomJitter())
	defer w.timer.Stop()
	select {
	case <-w.timer.C:
		return true
	case <-ctx.Done():
		w.timer = nil
		return false
	}
}

func (w *Waiter) randomJitter() time.Duration {
	jitter := int64(w.Jitter)
	if jitter == 0 {
		return 0
	}
	return time.Duration(w.rand.Int63n(jitter))
}
