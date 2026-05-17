package server

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// ipRateLimiter is a per-source-IP token bucket used to bound write-side
// abuse on the DCR endpoint. Buckets are evicted at random when the
// `maxIPs` cap is hit; for the single-operator deployments this binary
// targets, the steady-state population is small enough that the cap
// is never approached in practice.
type ipRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	rps     rate.Limit
	burst   int
	maxIPs  int
}

func newIPRateLimiter(rps rate.Limit, burst, maxIPs int) *ipRateLimiter {
	return &ipRateLimiter{
		buckets: make(map[string]*rate.Limiter),
		rps:     rps,
		burst:   burst,
		maxIPs:  maxIPs,
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[ip]
	if !ok {
		if len(l.buckets) >= l.maxIPs {
			// Random eviction — pick the first key the map iteration yields.
			// Avoids importing a heap; acceptable for the small-population
			// case this limiter targets.
			for k := range l.buckets {
				delete(l.buckets, k)
				break
			}
		}
		b = rate.NewLimiter(l.rps, l.burst)
		l.buckets[ip] = b
	}
	return b.Allow()
}

// clientIP extracts the caller's source IP, honouring X-Forwarded-For when
// present (operator is expected to terminate TLS at a trusted proxy that
// sets this header; if not, the bare RemoteAddr is used).
func clientIP(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-For"); h != "" {
		return strings.TrimSpace(strings.SplitN(h, ",", 2)[0])
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}
