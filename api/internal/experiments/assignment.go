// Chapter 32 — assignment. The one function in the package that matters, and
// the one place people reach for math/rand and get it wrong.
//
// A coin flip per request would move a user between variants on every page
// load: they'd see the new board, then the old one, then the new one, and your
// experiment would be measuring nothing at all. Assignment has to be a function
// OF THE USER — same user, same experiment, same answer, forever, with no
// clock, no randomness and no I/O.
//
// [verbatim ch32]
package experiments

import (
	"hash/fnv"
)

// Variant is one arm of an experiment. Weight is an integer 0..100;
// the sum of weights across all variants in an experiment is 100.
type Variant struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// Assign maps a userID and experimentKey to one of the experiment's
// variants. The function is pure: the same inputs always return the
// same variant, no clock, no randomness, no I/O.
func Assign(userID, experimentKey string, variants []Variant) string {
	if len(variants) == 0 {
		return ""
	}

	// FNV-64 is a small, fast, non-cryptographic hash. We do not need
	// collision resistance — we need a uniform integer in 0..99.
	h := fnv.New64a()
	_, _ = h.Write([]byte(experimentKey))
	_, _ = h.Write([]byte{0x00}) // separator so "abc"+"def" != "ab"+"cdef"
	_, _ = h.Write([]byte(userID))
	bucket := int(h.Sum64() % 100)

	// Walk the weight bands until we find the bucket's owner.
	cumulative := 0
	for _, v := range variants {
		cumulative += v.Weight
		if bucket < cumulative {
			return v.Name
		}
	}
	// The weights should always sum to 100; if a misconfigured row
	// doesn't, return the first variant — the safe, predictable choice.
	return variants[0].Name
}
