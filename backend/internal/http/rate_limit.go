package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type ClientRateLimitOptions struct {
	RatePerSecond float64
	Burst         int
	MaxClients    int
	ClientTTL     time.Duration
	Now           func() time.Time
}

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type ClientRateLimiter struct {
	mu            sync.Mutex
	ratePerSecond float64
	burst         float64
	maxClients    int
	clientTTL     time.Duration
	now           func() time.Time
	lastCleanup   time.Time
	clients       map[string]*clientBucket
	overflow      clientBucket
}

func NewClientRateLimiter(options ClientRateLimitOptions) *ClientRateLimiter {
	if options.RatePerSecond <= 0 {
		options.RatePerSecond = 1
	}
	if options.Burst <= 0 {
		options.Burst = 1
	}
	if options.MaxClients <= 0 {
		options.MaxClients = 1
	}
	if options.ClientTTL <= 0 {
		options.ClientTTL = 10 * time.Minute
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	now := options.Now()
	return &ClientRateLimiter{
		ratePerSecond: options.RatePerSecond,
		burst:         float64(options.Burst),
		maxClients:    options.MaxClients,
		clientTTL:     options.ClientTTL,
		now:           options.Now,
		lastCleanup:   now,
		clients:       make(map[string]*clientBucket),
		overflow: clientBucket{
			tokens:     float64(options.Burst),
			lastRefill: now,
			lastSeen:   now,
		},
	}
}

func (limiter *ClientRateLimiter) Allow(client string) (bool, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.now()
	limiter.cleanup(now)
	bucket, exists := limiter.clients[client]
	if !exists {
		if len(limiter.clients) >= limiter.maxClients {
			bucket = &limiter.overflow
		} else {
			bucket = &clientBucket{tokens: limiter.burst, lastRefill: now, lastSeen: now}
			limiter.clients[client] = bucket
		}
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed > 0 {
		bucket.tokens = math.Min(limiter.burst, bucket.tokens+elapsed*limiter.ratePerSecond)
		bucket.lastRefill = now
	}
	bucket.lastSeen = now
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}
	retryAfter := time.Duration(math.Ceil((1-bucket.tokens)/limiter.ratePerSecond*float64(time.Second))) * time.Nanosecond
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return false, retryAfter
}

func (limiter *ClientRateLimiter) ClientCount() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.clients)
}

func (limiter *ClientRateLimiter) cleanup(now time.Time) {
	if now.Sub(limiter.lastCleanup) < limiter.clientTTL {
		return
	}
	cutoff := now.Add(-limiter.clientTTL)
	for client, bucket := range limiter.clients {
		if bucket.lastSeen.Before(cutoff) {
			delete(limiter.clients, client)
		}
	}
	limiter.lastCleanup = now
}

func RateLimitMiddleware(limiter *ClientRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter := limiter.Allow(clientIP(r))
			if !allowed {
				seconds := int(math.Ceil(retryAfter.Seconds()))
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				writeHTTPError(w, http.StatusTooManyRequests, "请求过于频繁")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ConcurrencyLimitMiddleware(maxInFlight int) func(http.Handler) http.Handler {
	if maxInFlight <= 0 {
		maxInFlight = 1
	}
	semaphore := make(chan struct{}, maxInFlight)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Retry-After", "1")
				writeHTTPError(w, http.StatusServiceUnavailable, "服务繁忙，请稍后重试")
			}
		})
	}
}
