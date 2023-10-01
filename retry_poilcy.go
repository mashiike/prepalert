package prepalert

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"

	"github.com/Songmu/flextime"
	"github.com/mashiike/canyon"
)

type RetryPolicy struct {
	randGenerator *rand.Rand
	once          sync.Once
	Interval      float64 `hcl:"interval,optional"`
	Jitter        float64 `hcl:"jitter,optional"`
	MaxInterval   float64 `hcl:"max_interval,optional"`
	BackoffFactor float64 `hcl:"backoff_factor,optional"`
}

func (rp *RetryPolicy) SetRetryAfter(w http.ResponseWriter, r *http.Request) {
	if rp == nil {
		return
	}
	if rp.Interval < 0 {
		return
	}
	rp.once.Do(func() {
		if rp.BackoffFactor == 0 {
			rp.BackoffFactor = 2
		}
		if rp.Interval > rp.MaxInterval {
			rp.Interval = rp.MaxInterval
		}
		rp.randGenerator = rand.New(rand.NewSource(flextime.Now().UnixNano()))
	})
	approxmateReceiveCount, err := strconv.Atoi(
		r.Header.Get(canyon.HeaderSQSAttribute("ApproximateReceiveCount")),
	)
	if err != nil {
		approxmateReceiveCount = 1
	}
	// exponential backoff
	// base * factor ^ (approxmateReceiveCount - 1)
	s := rp.Interval * math.Pow(rp.BackoffFactor, float64(approxmateReceiveCount-1))
	if s > rp.MaxInterval {
		s = rp.MaxInterval
		s -= rp.randGenerator.Float64() * rp.Jitter
	} else {
		s += rp.randGenerator.Float64() * rp.Jitter
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", int(s)))
}

func (rp *RetryPolicy) String() string {
	if rp == nil {
		return ""
	}
	return fmt.Sprintf(
		"interval:%ds jitter:%ds max_interval:%ds backoff_factor:%.2f",
		int(rp.Interval), int(rp.Jitter), int(rp.MaxInterval), rp.BackoffFactor)
}
