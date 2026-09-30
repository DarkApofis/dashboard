package paymentref

import "testing"

// The state machine is the core business rule of this domain, so it gets an
// exhaustive test: every pair of statuses, allowed or not, is asserted.
func TestStatusCanTransitionTo(t *testing.T) {
	allowed := map[Status]map[Status]bool{
		StatusPending:   {StatusPaid: true, StatusCancelled: true, StatusExpired: true},
		StatusExpired:   {StatusPaid: true},
		StatusPaid:      {},
		StatusCancelled: {},
	}

	for _, from := range AllStatuses() {
		for _, to := range AllStatuses() {
			want := allowed[from][to]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s: got %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestStatusIsTerminal(t *testing.T) {
	cases := map[Status]bool{
		StatusPending:   false,
		StatusExpired:   false, // an expired reference can still be paid
		StatusPaid:      true,
		StatusCancelled: true,
	}

	for status, want := range cases {
		if got := status.IsTerminal(); got != want {
			t.Errorf("%s.IsTerminal(): got %v, want %v", status, got, want)
		}
	}
}

func TestParseStatus(t *testing.T) {
	for _, valid := range AllStatuses() {
		got, err := ParseStatus(string(valid))
		if err != nil {
			t.Errorf("ParseStatus(%q): unexpected error %v", valid, err)
		}
		if got != valid {
			t.Errorf("ParseStatus(%q): got %q", valid, got)
		}
	}

	for _, invalid := range []string{"", "PENDING", "refunded", "paid "} {
		if _, err := ParseStatus(invalid); err == nil {
			t.Errorf("ParseStatus(%q): expected an error", invalid)
		}
	}
}
