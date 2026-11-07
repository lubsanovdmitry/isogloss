// Package ord gives deterministic iteration over maps. Go randomises map
// order, so engine code must never range over a map where order matters.
package ord

import (
	"cmp"
	"slices"
)

// Keys returns the keys of m in ascending order.
func Keys[K cmp.Ordered, V any](m map[K]V) []K {
	ks := make([]K, 0, len(m))
	for k := range m { //lint:ordered keys are sorted below
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}
