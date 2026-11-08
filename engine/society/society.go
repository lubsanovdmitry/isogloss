// Package society holds what changes who talks to whom and whose speech
// carries weight: polities, trade routes and, later, writing. It never
// changes words directly; the simulation turns its state into contact
// weights, prestige and language shift.
package society

import (
	"slices"

	"isogloss/engine/world"
)

// Polity is a group of communities under one ruler.
type Polity struct {
	ID        int32
	Name      string
	Capital   int32 // hex
	Founded   int32
	Fell      int32 // tick it broke up, -1 while it stands
	Lang      int32 // language of the capital when founded
	Seafaring bool
}

// Alive reports whether the polity still stands.
func (p *Polity) Alive() bool { return p.Fell < 0 }

// Route is a trade route between two hubs along the cheapest path.
type Route struct {
	A, B  int32   // hub hexes
	Path  []int32 // hexes from A to B inclusive
	Since int32
}

// Outcome of foreign rule over a community, decided after rule has lasted
// a while (by population ratio and prestige).
type Outcome uint8

const (
	Undecided       Outcome = iota
	EliteDominance          // the locals shift to the rulers' language, leaving a substrate
	EliteAssimilate         // the rulers shift; their language leaves loans (rule, war, law)
	Bilingual               // both persist; heavy two-way borrowing
)

var outcomeNames = [...]string{"undecided", "elite dominance", "elite assimilation", "stable bilingualism"}

func (o Outcome) String() string { return outcomeNames[o] }

// seaCost is the cost (×100) of crossing one sea hex for seafarers.
const seaCost = 160

// Costs runs Dijkstra from a over land (and sea when sea is true), up to
// maxCost. dist[h] is the cheapest cost to h (-1 unreachable) and prev[h]
// the hex before it. Ties break by hex ID, so results are deterministic.
func Costs(w *world.World, a int32, sea bool, maxCost int32) (dist, prev []int32) {
	n := len(w.Hexes)
	dist = make([]int32, n)
	prev = make([]int32, n)
	for i := range dist {
		dist[i], prev[i] = -1, -1
	}
	var h minHeap
	dist[a] = 0
	h.push(item{0, a})
	for len(h) > 0 {
		it := h.pop()
		if it.d != dist[it.h] {
			continue
		}
		for _, nb := range w.Hexes[it.h].Nbr {
			if nb < 0 {
				continue
			}
			c := w.Cost(it.h, nb)
			if c < 0 {
				if !sea {
					continue
				}
				c = seaCost
			}
			d := it.d + c
			if d > maxCost {
				continue
			}
			if dist[nb] < 0 || d < dist[nb] {
				dist[nb], prev[nb] = d, it.h
				h.push(item{d, nb})
			}
		}
	}
	return dist, prev
}

// Trace rebuilds the path to b from Costs' prev array (nil if unreachable).
func Trace(prev, dist []int32, a, b int32) []int32 {
	if dist[b] < 0 {
		return nil
	}
	var path []int32
	for h := b; h >= 0; h = prev[h] {
		path = append(path, h)
		if h == a {
			break
		}
	}
	slices.Reverse(path)
	return path
}

// Path finds the cheapest path from a to b (nil if none within maxCost).
func Path(w *world.World, a, b int32, sea bool, maxCost int32) []int32 {
	dist, prev := Costs(w, a, sea, maxCost)
	return Trace(prev, dist, a, b)
}

type item struct{ d, h int32 }

func (x item) less(y item) bool { return x.d < y.d || x.d == y.d && x.h < y.h }

// minHeap is a binary heap of items.
type minHeap []item

func (h *minHeap) push(x item) {
	*h = append(*h, x)
	i := len(*h) - 1
	for i > 0 {
		p := (i - 1) / 2
		if !(*h)[i].less((*h)[p]) {
			break
		}
		(*h)[i], (*h)[p] = (*h)[p], (*h)[i]
		i = p
	}
}

func (h *minHeap) pop() item {
	old := *h
	top := old[0]
	n := len(old) - 1
	old[0] = old[n]
	*h = old[:n]
	i := 0
	for {
		l, r, m := 2*i+1, 2*i+2, i
		if l < n && (*h)[l].less((*h)[m]) {
			m = l
		}
		if r < n && (*h)[r].less((*h)[m]) {
			m = r
		}
		if m == i {
			break
		}
		(*h)[i], (*h)[m] = (*h)[m], (*h)[i]
		i = m
	}
	return top
}

// SeaReach lists land hexes reachable from a across the sea within d hex
// steps, crossing at least one sea hex (for seafaring links), ascending.
func SeaReach(w *world.World, a int32, d int32) []int32 {
	var out []int32
	seen := map[int32]int32{a: 0}
	frontier := []int32{a}
	for step := int32(1); step <= d; step++ {
		var next []int32
		for _, h := range frontier {
			for _, nb := range w.Hexes[h].Nbr {
				if nb < 0 {
					continue
				}
				if _, ok := seen[nb]; ok {
					continue
				}
				seen[nb] = step
				if w.Hexes[nb].Biome == world.Sea {
					next = append(next, nb)
				} else if h != a {
					out = append(out, nb) // land reached across sea
				}
			}
		}
		frontier = next
	}
	slices.Sort(out)
	return out
}
