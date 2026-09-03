package matching

import "testing"

func TestSwipeDecision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		decision   SwipeDecision
		valid      bool
		interested bool
	}{
		{decision: SwipeSkip, valid: true},
		{decision: SwipeLike, valid: true, interested: true},
		{decision: SwipeSuperLike, valid: true, interested: true},
		{decision: SwipeDecision("maybe")},
	}
	for _, test := range tests {
		if got := test.decision.Valid(); got != test.valid {
			t.Fatalf("Valid(%q) = %v, want %v", test.decision, got, test.valid)
		}
		if got := test.decision.Interested(); got != test.interested {
			t.Fatalf("Interested(%q) = %v, want %v", test.decision, got, test.interested)
		}
	}
}
