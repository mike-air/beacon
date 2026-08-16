package experiments

import (
	"testing"

	"github.com/google/uuid"
)

// Assign is pure, which is exactly what makes it testable without a database,
// a clock, or a running server. These are the four properties the chapter
// claims for it, each checked against 2,000 synthetic users.
func TestAssign(t *testing.T) {
	fiftyFifty := []Variant{{Name: "control", Weight: 50}, {Name: "treatment", Weight: 50}}

	ids := make([]string, 2000)
	for i := range ids {
		ids[i] = uuid.NewString()
	}

	t.Run("splits roughly evenly", func(t *testing.T) {
		counts := map[string]int{}
		for _, id := range ids {
			counts[Assign(id, "new_board_ui", fiftyFifty)]++
		}
		t.Logf("control=%d treatment=%d", counts["control"], counts["treatment"])
		// 45–55% is a generous band; a broken hash lands far outside it.
		for name, n := range counts {
			if n < 900 || n > 1100 {
				t.Errorf("variant %q got %d of 2000, expected roughly 1000", name, n)
			}
		}
	})

	t.Run("is stable for the same user", func(t *testing.T) {
		// The whole point. A user who flickers between variants makes the
		// experiment measure nothing.
		for _, id := range ids {
			first := Assign(id, "new_board_ui", fiftyFifty)
			for k := 0; k < 20; k++ {
				if got := Assign(id, "new_board_ui", fiftyFifty); got != first {
					t.Fatalf("user %s got %q then %q", id, first, got)
				}
			}
		}
	})

	t.Run("de-correlates across experiments", func(t *testing.T) {
		// The experiment key goes into the hash so that a user who lands in
		// treatment here doesn't automatically land in treatment everywhere.
		same := 0
		for _, id := range ids {
			if Assign(id, "new_board_ui", fiftyFifty) == Assign(id, "other_experiment", fiftyFifty) {
				same++
			}
		}
		t.Logf("same arm in both experiments: %d/2000", same)
		if same < 850 || same > 1150 {
			t.Errorf("experiments look correlated: %d/2000 users matched", same)
		}
	})

	t.Run("honours uneven weights", func(t *testing.T) {
		ninetyTen := []Variant{{Name: "control", Weight: 90}, {Name: "treatment", Weight: 10}}
		counts := map[string]int{}
		for _, id := range ids {
			counts[Assign(id, "new_board_ui", ninetyTen)]++
		}
		t.Logf("control=%d treatment=%d", counts["control"], counts["treatment"])
		if counts["treatment"] < 140 || counts["treatment"] > 260 {
			t.Errorf("treatment got %d of 2000, expected roughly 200", counts["treatment"])
		}
	})

	t.Run("survives a misconfigured experiment", func(t *testing.T) {
		// Weights that don't sum to 100 are a data bug, not a runtime condition
		// to crash on. Fall back to the first variant.
		short := []Variant{{Name: "control", Weight: 10}, {Name: "treatment", Weight: 10}}
		for _, id := range ids {
			if got := Assign(id, "k", short); got != "control" && got != "treatment" {
				t.Fatalf("got %q for %s", got, id)
			}
		}
		if got := Assign("someone", "k", nil); got != "" {
			t.Errorf("no variants should give %q, got %q", "", got)
		}
	})
}

// The null separator in the hash exists so that concatenating the two inputs
// cannot collide: experiment "abc" + user "def" must not hash the same as
// experiment "ab" + user "cdef". Without the separator both are the byte string
// "abcdef" and two unrelated experiments share one assignment.
//
// Checked with 100 one-percent bands, so the bucket itself is observable
// through Assign's return value rather than inferred.
func TestAssignSeparatorPreventsCollision(t *testing.T) {
	bands := make([]Variant, 100)
	for i := range bands {
		bands[i] = Variant{Name: string(rune('a'+i%26)) + string(rune('0'+i/26)), Weight: 1}
	}

	a := Assign("def", "abc", bands) // user "def", experiment "abc"
	b := Assign("cdef", "ab", bands) // user "cdef", experiment "ab"
	if a == b {
		t.Errorf("split inputs hashed to the same bucket (%q) — the separator is missing", a)
	}
}
