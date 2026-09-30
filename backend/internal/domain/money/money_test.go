package money

import (
	"errors"
	"testing"
)

func TestFromString(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		wantErr error
	}{
		{raw: "125000.50", want: "125000.50"},
		{raw: "0", want: "0.00"},
		{raw: "1.5", want: "1.50"},
		{raw: " 10 ", want: "10.00"},
		{raw: "999999999999.99", want: "999999999999.99"},
		{raw: "-1", wantErr: ErrNegative},
		{raw: "1.005", wantErr: ErrScale},
		{raw: "1000000000000.00", wantErr: ErrTooLarge},
		{raw: "abc", wantErr: ErrInvalidFormat},
		{raw: "", wantErr: ErrInvalidFormat},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := FromString(tc.raw)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("FromString(%q): got error %v, want %v", tc.raw, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromString(%q): unexpected error %v", tc.raw, err)
			}
			if got.String() != tc.want {
				t.Errorf("FromString(%q).String(): got %q, want %q", tc.raw, got.String(), tc.want)
			}
		})
	}
}

// The exactness of the representation is the whole reason this type exists: the
// webhook has to decide whether the notified amount equals the expected one.
func TestEqualIsExact(t *testing.T) {
	a := MustFromString("125000.50")
	b := MustFromString("125000.5")
	c := MustFromString("125000.51")

	if !a.Equal(b) {
		t.Error("125000.50 and 125000.5 must be equal")
	}
	if a.Equal(c) {
		t.Error("125000.50 and 125000.51 must not be equal")
	}
}

func TestSignHelpers(t *testing.T) {
	if !MustFromString("0.01").IsPositive() {
		t.Error("0.01 must be positive")
	}
	if Zero().IsPositive() {
		t.Error("zero must not be positive")
	}
	if !Zero().IsZero() {
		t.Error("zero must report IsZero")
	}
	if Zero().String() != "0.00" {
		t.Errorf("zero renders as %q, want 0.00", Zero().String())
	}
}
