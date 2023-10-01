package prepalert_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mashiike/canyon"
	"github.com/mashiike/prepalert"
	"github.com/stretchr/testify/require"
)

func TestRetryPolicySetRetryAfter__Nil(t *testing.T) {
	t.Parallel()
	var rp *prepalert.RetryPolicy
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rp.SetRetryAfter(w, r)
	require.Equal(t, "", w.Header().Get("Retry-After"))
}

func TestRetryPolicySetRetryAfter__IntervalLessThanZero(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval: -1,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rp.SetRetryAfter(w, r)
	require.Equal(t, "", w.Header().Get("Retry-After"))
}

func TestRetryPolicySetRetryAfter__First(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval:      5,
		MaxInterval:   300,
		BackoffFactor: 2,
		Jitter:        0,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(canyon.HeaderSQSAttribute("ApproximateReceiveCount"), "1")
	rp.SetRetryAfter(w, r)
	require.Equal(t, "5", w.Header().Get("Retry-After"))
}

func TestRetryPolicySetRetryAfter__Second(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval:      5,
		MaxInterval:   300,
		BackoffFactor: 2,
		Jitter:        0,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(canyon.HeaderSQSAttribute("ApproximateReceiveCount"), "2")
	rp.SetRetryAfter(w, r)
	require.Equal(t, "10", w.Header().Get("Retry-After"))
}

func TestRetryPolicySetRetryAfter__Third(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval:      5,
		MaxInterval:   300,
		BackoffFactor: 2,
		Jitter:        0,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(canyon.HeaderSQSAttribute("ApproximateReceiveCount"), "3")
	rp.SetRetryAfter(w, r)
	require.Equal(t, "20", w.Header().Get("Retry-After"))
}

func TestRetryPolicySetRetryAfter__MaxInterval(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval:      5,
		MaxInterval:   300,
		BackoffFactor: 2,
		Jitter:        0,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(canyon.HeaderSQSAttribute("ApproximateReceiveCount"), "100")
	rp.SetRetryAfter(w, r)
	require.Equal(t, "300", w.Header().Get("Retry-After"))
}

func TestRetryPolicySetRetryAfter__FirstWithJittr(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval:      5,
		MaxInterval:   300,
		BackoffFactor: 2,
		Jitter:        15,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(canyon.HeaderSQSAttribute("ApproximateReceiveCount"), "1")
	rp.SetRetryAfter(w, r)
	retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err)
	t.Log("Retry-After:", retryAfter)
	require.GreaterOrEqual(t, retryAfter, 5)
	require.LessOrEqual(t, retryAfter, 20)
}

func TestRetryPolicySetRetryAfter__MaxIntervalWithJittr(t *testing.T) {
	t.Parallel()
	rp := &prepalert.RetryPolicy{
		Interval:      5,
		MaxInterval:   300,
		BackoffFactor: 2,
		Jitter:        15,
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(canyon.HeaderSQSAttribute("ApproximateReceiveCount"), "100")
	rp.SetRetryAfter(w, r)
	retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err)
	t.Log("Retry-After:", retryAfter)
	require.GreaterOrEqual(t, retryAfter, 285)
	require.LessOrEqual(t, retryAfter, 300)
}
