package paymentref

import "paymentlinks/internal/domain/errs"

// Status is the lifecycle state of a payment reference.
type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// ErrInvalidStatus is returned when an unknown status reaches the domain.
var ErrInvalidStatus = errs.Validation("invalid_status", "status must be one of pending, paid, expired, cancelled")

// transitions is the authoritative state machine. Reading it top to bottom:
//
//	pending   -> paid | cancelled | expired
//	expired   -> paid              (money arrived late; recorded for review)
//	paid      -> terminal
//	cancelled -> terminal          (a payment for it is a conflict, not a transition)
var transitions = map[Status][]Status{
	StatusPending:   {StatusPaid, StatusCancelled, StatusExpired},
	StatusExpired:   {StatusPaid},
	StatusPaid:      {},
	StatusCancelled: {},
}

// AllStatuses lists every valid status, in lifecycle order.
func AllStatuses() []Status {
	return []Status{StatusPending, StatusPaid, StatusExpired, StatusCancelled}
}

// ParseStatus validates external input (query strings, payloads, database rows).
func ParseStatus(raw string) (Status, error) {
	candidate := Status(raw)
	if !candidate.IsValid() {
		return "", ErrInvalidStatus.WithDetail("status", raw)
	}
	return candidate, nil
}

// IsValid reports whether s is a known status.
func (s Status) IsValid() bool {
	_, ok := transitions[s]
	return ok
}

func (s Status) String() string { return string(s) }

// IsTerminal reports whether no further transition is possible.
func (s Status) IsTerminal() bool {
	return len(transitions[s]) == 0
}

// CanTransitionTo reports whether the s -> target move is allowed.
func (s Status) CanTransitionTo(target Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}
