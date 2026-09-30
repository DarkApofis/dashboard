// Package clock implements the time port. It exists so that no use case ever
// calls time.Now() directly: every time-dependent rule (expiration, above all)
// can then be driven from a test with a fixed instant.
package clock

import (
	"time"

	apppaymentref "paymentlinks/internal/application/paymentref"
)

// System is the real clock. It always answers in UTC, which is also how every
// timestamp is stored and serialised.
type System struct{}

var _ apppaymentref.Clock = System{}

// New returns the system clock.
func New() System { return System{} }

// Now is the current instant in UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Fixed is a clock stopped at a chosen instant. It lives next to the real one
// because it is the other half of the same port: the expiration rules are only
// testable if "now" can be moved at will.
type Fixed struct {
	Instant time.Time
}

var _ apppaymentref.Clock = (*Fixed)(nil)

// NewFixed returns a clock stopped at instant.
func NewFixed(instant time.Time) *Fixed { return &Fixed{Instant: instant.UTC()} }

// Now is the configured instant.
func (c *Fixed) Now() time.Time { return c.Instant }

// Advance moves the clock forward.
func (c *Fixed) Advance(d time.Duration) { c.Instant = c.Instant.Add(d) }
