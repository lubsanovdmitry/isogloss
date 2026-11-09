package pop

import (
	"slices"

	"isogloss/engine/fx"
	"isogloss/engine/lex"
)

// DefaultThreshold is the intelligibility above which two varieties count
// as one language. Calibrated on IE-CoR Romance data (TestRomanceCalibration):
// midway between Spanish–Portuguese (537‰, grouped) and Spanish–Italian
// (439‰, separate). The spec's initial guess of 0.7 was for a different scale.
var DefaultThreshold = fx.FromPermille(490)

// Clusterer groups communities into languages, caching intelligibility
// between variety pairs.
type Clusterer struct {
	M     Metric
	Thr   fx.Q16
	cache map[[2]uint64]fx.Q16
}

// NewClusterer returns a clusterer with the default metric and threshold.
func NewClusterer() *Clusterer {
	return &Clusterer{M: DefaultMetric, Thr: DefaultThreshold, cache: map[[2]uint64]fx.Q16{}}
}

// Intel returns cached intelligibility between two varieties.
func (cl *Clusterer) Intel(a, b *lex.Variety) fx.Q16 {
	if a.Key == b.Key {
		return fx.One
	}
	k := [2]uint64{a.Key, b.Key}
	if k[0] > k[1] {
		k[0], k[1] = k[1], k[0]
	}
	if v, ok := cl.cache[k]; ok {
		return v
	}
	v := cl.M.Intel(a, b)
	cl.cache[k] = v
	return v
}

// Prune drops cached pairs whose varieties are no longer spoken.
func (cl *Clusterer) Prune(live map[uint64]bool) {
	for k := range cl.cache { //lint:ordered deleting entries is order-independent
		if !live[k[0]] || !live[k[1]] {
			delete(cl.cache, k)
		}
	}
}

// Group is one language found by clustering.
type Group struct {
	Centre    int32   // hex of the centre community
	Members   []int32 // hexes, ascending
	Continuum int32   // chain component index; groups sharing it form a dialect continuum
}

// Cluster partitions communities into languages. Neighbours whose varieties
// are mutually intelligible form chains (components); inside each chain,
// languages grow from the most populous unassigned community, taking
// connected members still intelligible with that centre. A chain that splits
// into several languages is a dialect continuum: its ends are unintelligible
// though every link is.
func (cl *Clusterer) Cluster(comms []*Community, byHex []*Community, nbrs func(int32) []int32) []Group {
	// Chain components over intelligible neighbour links.
	comp := map[int32]int32{}
	var compOf [][]int32
	for _, c := range comms {
		if _, done := comp[c.Hex]; done {
			continue
		}
		id := int32(len(compOf))
		comp[c.Hex] = id
		members := []int32{c.Hex}
		for q := 0; q < len(members); q++ {
			h := members[q]
			for _, nb := range nbrs(h) {
				if byHex[nb] == nil {
					continue
				}
				if _, done := comp[nb]; done {
					continue
				}
				if cl.Intel(byHex[h].Variety, byHex[nb].Variety) >= cl.Thr {
					comp[nb] = id
					members = append(members, nb)
				}
			}
		}
		slices.Sort(members)
		compOf = append(compOf, members)
	}
	var groups []Group
	for ci, members := range compOf {
		order := slices.Clone(members)
		slices.SortStableFunc(order, func(a, b int32) int {
			pa, pb := byHex[a].Pop, byHex[b].Pop
			if pa != pb {
				return int(pb - pa)
			}
			return int(a - b)
		})
		assigned := map[int32]bool{}
		var made []Group
		for _, centre := range order {
			if assigned[centre] {
				continue
			}
			cv := byHex[centre].Variety
			g := Group{Centre: centre, Continuum: -1}
			assigned[centre] = true
			queue := []int32{centre}
			for q := 0; q < len(queue); q++ {
				h := queue[q]
				g.Members = append(g.Members, h)
				for _, nb := range nbrs(h) {
					if byHex[nb] == nil || assigned[nb] || comp[nb] != int32(ci) {
						continue
					}
					if cl.Intel(cv, byHex[nb].Variety) >= cl.Thr {
						assigned[nb] = true
						queue = append(queue, nb)
					}
				}
			}
			slices.Sort(g.Members)
			made = append(made, g)
		}
		if len(made) > 1 {
			for i := range made {
				made[i].Continuum = int32(ci)
			}
		}
		groups = append(groups, made...)
	}
	return groups
}

// Reset empties the intelligibility cache (a clone starts with none).
func (cl *Clusterer) Reset() { cl.cache = map[[2]uint64]fx.Q16{} }
