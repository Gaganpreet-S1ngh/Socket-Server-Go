package ratelimit

import "golang.org/x/time/rate"

// Limiter wraps a token-bucket limiter for a single connection.
// Each client gets its own instance so one noisy/malicious client
// can't starve others, and the server rejects/disconnects clients
// that exceed their allotted message rate.
type Limiter struct {
	l *rate.Limiter
}

// New creates a limiter allowing `perSecond` messages/sec sustained,
// with a burst capacity of `burst`.
func New(perSecond float64, burst int) *Limiter {
	return &Limiter{l: rate.NewLimiter(rate.Limit(perSecond), burst)}
}

// Allow reports whether a message may be processed right now.
// Non-blocking - the caller decides what to do on rejection
// (e.g. drop the message, warn, or disconnect after repeat offenses).
func (r *Limiter) Allow() bool {
	return r.l.Allow()
}
