package api

import (
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// limiter allows each client a burst of attempts, then one more every
// interval (a token bucket per client).
type limiter struct {
	mu        sync.Mutex
	every     rate.Limit
	burst     int
	clients   map[string]*rate.Limiter
	lastSweep time.Time
}

func newLimiter(interval time.Duration, burst int) *limiter {
	return &limiter{every: rate.Every(interval), burst: burst, clients: map[string]*rate.Limiter{}}
}

// wait returns 0 if the client may go ahead and uses up one attempt, or how
// long it has to wait otherwise.
func (l *limiter) wait(client string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.sweep(now)
	lim := l.clients[client]
	if lim == nil {
		lim = rate.NewLimiter(l.every, l.burst)
		l.clients[client] = lim
	}
	r := lim.ReserveN(now, 1)
	if d := r.DelayFrom(now); d > 0 {
		// The attempt is refused, so it doesn't count.
		r.CancelAt(now)
		return d
	}
	return 0
}

// sweep forgets clients whose bucket is full again, since a new bucket would
// be the same. This keeps memory from growing with every IP ever seen.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	for client, lim := range l.clients {
		if lim.TokensAt(now) >= float64(l.burst) {
			delete(l.clients, client)
		}
	}
	l.lastSweep = now
}

// limitAttempts wraps handlers that check a password, so nobody can try
// passwords faster than the login limit allows.
func (s *Server) limitAttempts(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d := s.loginLimiter.wait(clientKey(clientIP(r, s.trustedProxies))); d > 0 {
			tooMany(w, d, "too many attempts, try again later")
			return
		}
		next(w, r)
	}
}

// tooMany answers 429 with how long to wait, in whole seconds.
func tooMany(w http.ResponseWriter, wait time.Duration, msg string) {
	w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
	writeCodedError(w, http.StatusTooManyRequests, "too_many_requests", msg)
}

// clientIP returns the address the request came from. Behind a reverse proxy
// every request comes from the proxy, which puts the real address in
// X-Forwarded-For. Anyone can send that header, so it is only believed when
// the request comes from a trusted proxy, and only as far back as the hops
// are trusted proxies too.
func clientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	addrPort, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	ip := addrPort.Addr().Unmap()
	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	// The rightmost entry was added by the nearest proxy, so walk from the
	// right until an address isn't one of our proxies.
	for i := len(hops) - 1; i >= 0 && isTrusted(ip, trusted); i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		ip = hop.Unmap()
	}
	return ip
}

func isTrusted(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// clientKey groups IPv6 addresses by their /64 network: one home or server
// usually gets a whole /64, so counting single addresses would let a client
// switch to a fresh address for every attempt.
func clientKey(ip netip.Addr) string {
	if ip.Is6() {
		p, _ := ip.Prefix(64)
		return p.String()
	}
	return ip.String()
}
