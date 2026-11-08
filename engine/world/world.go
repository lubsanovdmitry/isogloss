// Package world generates the hex map: elevation, sea, rivers, biomes and
// the cost of crossing each hex edge. Everything is integer arithmetic.
package world

import (
	"slices"

	"isogloss/engine/rng"
)

// Biome of a hex.
type Biome uint8

const (
	Sea Biome = iota
	Coast
	Plains
	Forest
	Steppe
	Desert
	Mountain
	Island
	NumBiomes
)

var biomeNames = [NumBiomes]string{"sea", "coast", "plains", "forest", "steppe", "desert", "mountain", "island"}

func (b Biome) String() string { return biomeNames[b] }

// Per-biome movement cost (×100) and carrying capacity (people).
var (
	moveCost = [NumBiomes]int32{0, 100, 100, 180, 110, 260, 450, 120}
	capacity = [NumBiomes]int32{0, 1400, 1200, 700, 600, 150, 180, 600}
)

// Hex is one map cell. Coordinates are odd-r offset (Col, Row).
type Hex struct {
	ID       int32
	Col, Row int32
	Elev     int32 // 0..1000
	Moist    int32 // 0..1000
	Biome    Biome
	River    bool
	Down     int32 // downstream hex for river cells, -1 otherwise
	Nbr      [6]int32
	Capacity int32
}

// Land reports whether the hex can be settled.
func (h *Hex) Land() bool { return h.Biome != Sea }

// World is a generated map.
type World struct {
	W, H  int32
	Hexes []Hex
	Land  []int32 // land hex IDs, ascending
}

// Shape of the generated land.
type Shape uint8

const (
	Continent   Shape = iota
	Archipelago       // many islands: finer noise, weak falloff
	Corridor          // a long east–west land strip
)

// Config controls map generation.
type Config struct {
	W, H   int32
	Land   int   // target number of land hexes
	Shape  Shape // continent by default
	Rivers int   // rivers to trace (0 = default 9)
}

// DefaultConfig gives ~300 land hexes.
var DefaultConfig = Config{W: 26, H: 20, Land: 300}

// odd-r neighbour offsets: [parity][dir] → (dc, dr)
var nbrOff = [2][6][2]int32{
	{{+1, 0}, {0, -1}, {-1, -1}, {-1, 0}, {-1, +1}, {0, +1}},
	{{+1, 0}, {+1, -1}, {0, -1}, {-1, 0}, {0, +1}, {+1, +1}},
}

func hash3(seed uint64, x, y, layer int32) uint32 {
	h := seed ^ (uint64(uint32(x)) * 0x9E3779B97F4A7C15) ^ (uint64(uint32(y)) * 0xC2B2AE3D27D4EB4F) ^ (uint64(uint32(layer)) << 48)
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return uint32(h)
}

// valueNoise returns smooth noise in 0..1000 at (x, y) given in 1/256 units,
// summing three octaves of bilinear value noise.
func valueNoise(seed uint64, x, y, layer int32) int32 {
	var sum, norm int32
	amp := int32(4)
	scale := int32(6 * 256) // cells per lattice step (×256)
	for o := int32(0); o < 3; o++ {
		fx, fy := x*256/scale, y*256/scale // lattice coords ×256
		ix, iy := fx>>8, fy>>8
		tx, ty := fx&255, fy&255
		v := func(a, b int32) int32 { return int32(hash3(seed, a, b, layer*8+o) % 1001) }
		top := v(ix, iy)*(256-tx) + v(ix+1, iy)*tx
		bot := v(ix, iy+1)*(256-tx) + v(ix+1, iy+1)*tx
		val := (top*(256-ty) + bot*ty) >> 16
		sum += val * amp
		norm += amp
		amp /= 2
		scale /= 2
	}
	return sum / norm
}

// Generate builds a world from the seed.
func Generate(seed uint64, cfg Config) *World {
	s := rng.New(seed, rng.Key("world")).Uint64()
	w := &World{W: cfg.W, H: cfg.H}
	n := cfg.W * cfg.H
	w.Hexes = make([]Hex, n)
	cx, cy := cfg.W*256/2, cfg.H*256/2
	for r := int32(0); r < cfg.H; r++ {
		for c := int32(0); c < cfg.W; c++ {
			id := r*cfg.W + c
			h := &w.Hexes[id]
			h.ID, h.Col, h.Row, h.Down = id, c, r, -1
			// Hex centre in 1/256 cells (odd rows shifted half a cell).
			x := c*256 + (r&1)*128
			y := r * 222 // √3/2 × 256
			e := valueNoise(s, x, y, 1)
			// Radial falloff makes a continent rather than a noise field.
			dx, dy := (x-cx)*1000/cx, (y-cy*222/256)*1000/(cy*222/256)
			d2 := (dx*dx + dy*dy) / 1000 // 0..~2000
			switch cfg.Shape {
			case Archipelago:
				e = (e + valueNoise(s, x*3, y*3, 3)) / 2
				e = e - d2*20/100
			case Corridor:
				d2 = (dx*dx/6 + dy*dy) / 1000
				e = e - d2*70/100
			default:
				e = e - d2*55/100
			}
			h.Elev = e
			h.Moist = valueNoise(s, x, y, 2)
			for k, off := range nbrOff[r&1] {
				nc, nr := c+off[0], r+off[1]
				if nc < 0 || nr < 0 || nc >= cfg.W || nr >= cfg.H {
					h.Nbr[k] = -1
				} else {
					h.Nbr[k] = nr*cfg.W + nc
				}
			}
		}
	}
	// Sea level: keep the cfg.Land highest hexes as land.
	elevs := make([]int32, n)
	for i := range w.Hexes {
		elevs[i] = w.Hexes[i].Elev
	}
	slices.Sort(elevs)
	sea := elevs[max(int(n)-cfg.Land, 0)]
	minE, maxE := elevs[0], elevs[n-1]
	for i := range w.Hexes {
		h := &w.Hexes[i]
		land := h.Elev >= sea
		// Normalise elevation to 0..1000 with sea level at 300.
		if land {
			h.Elev = 300 + (h.Elev-sea)*700/max(maxE-sea, 1)
		} else {
			h.Elev = (h.Elev - minE) * 299 / max(sea-minE, 1)
		}
		if !land {
			h.Biome = Sea
		} else {
			h.Biome = Plains
		}
	}
	w.classify()
	w.rivers(s, cfg.Rivers)
	for i := range w.Hexes {
		h := &w.Hexes[i]
		h.Capacity = capacity[h.Biome]
		if h.River {
			h.Capacity = h.Capacity * 3 / 2
		}
		if h.Land() {
			w.Land = append(w.Land, h.ID)
		}
	}
	return w
}

func (w *World) classify() {
	// Mountains: top 12% of land elevation.
	var landElev []int32
	for _, h := range w.Hexes {
		if h.Biome != Sea {
			landElev = append(landElev, h.Elev)
		}
	}
	slices.Sort(landElev)
	mountain := landElev[len(landElev)*88/100]
	for i := range w.Hexes {
		h := &w.Hexes[i]
		if h.Biome == Sea {
			continue
		}
		coastal := false
		for _, nb := range h.Nbr {
			if nb < 0 || w.Hexes[nb].Biome == Sea {
				coastal = true
			}
		}
		switch {
		case h.Elev >= mountain:
			h.Biome = Mountain
		case coastal:
			h.Biome = Coast
		case h.Moist < 250:
			h.Biome = Desert
		case h.Moist < 420:
			h.Biome = Steppe
		case h.Moist < 650:
			h.Biome = Plains
		default:
			h.Biome = Forest
		}
	}
	// Small land components become islands.
	comp := w.components()
	size := map[int32]int{}
	for _, c := range comp {
		if c >= 0 {
			size[c]++
		}
	}
	for i := range w.Hexes {
		if c := comp[i]; c >= 0 && size[c] < 10 {
			w.Hexes[i].Biome = Island
		}
	}
}

// components labels connected land components (-1 for sea).
func (w *World) components() []int32 {
	comp := make([]int32, len(w.Hexes))
	for i := range comp {
		comp[i] = -1
	}
	var next int32
	for i := range w.Hexes {
		if w.Hexes[i].Biome == Sea || comp[i] >= 0 {
			continue
		}
		stack := []int32{int32(i)}
		comp[i] = next
		for len(stack) > 0 {
			h := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, nb := range w.Hexes[h].Nbr {
				if nb >= 0 && w.Hexes[nb].Biome != Sea && comp[nb] < 0 {
					comp[nb] = next
					stack = append(stack, nb)
				}
			}
		}
		next++
	}
	return comp
}

func (w *World) rivers(seed uint64, want int) {
	if want <= 0 {
		want = 9
	}
	r := rng.New(seed, rng.Key("rivers"))
	var sources []int32
	for _, h := range w.Hexes {
		if h.Biome != Sea && h.Biome != Island && h.Elev > 550 && h.Moist > 450 {
			sources = append(sources, h.ID)
		}
	}
	// Shuffle deterministically and take a handful of spaced sources.
	for i := len(sources) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		sources[i], sources[j] = sources[j], sources[i]
	}
	used := 0
	for _, src := range sources {
		if used >= want || w.Hexes[src].River {
			continue
		}
		cur := src
		path := []int32{cur}
		for steps := 0; steps < 60; steps++ {
			h := &w.Hexes[cur]
			best := int32(-1)
			for _, nb := range h.Nbr {
				if nb >= 0 && (best < 0 || w.Hexes[nb].Elev < w.Hexes[best].Elev) {
					best = nb
				}
			}
			if best < 0 || w.Hexes[best].Elev >= h.Elev {
				break // local minimum: the river ends in a lake/marsh
			}
			if w.Hexes[best].Biome == Sea || w.Hexes[best].River {
				path = append(path, best)
				break
			}
			cur = best
			path = append(path, cur)
		}
		if len(path) < 4 {
			continue
		}
		for i := 0; i+1 < len(path); i++ {
			h := &w.Hexes[path[i]]
			if h.Biome == Sea {
				break
			}
			h.River = true
			if w.Hexes[path[i+1]].Biome != Sea {
				h.Down = path[i+1]
			}
		}
		used++
	}
}

// Cost is the cost (×100) of moving between neighbouring hexes a and b,
// or -1 if impassable (sea). Travel along a river is cheap; crossing one
// costs extra.
func (w *World) Cost(a, b int32) int32 {
	ha, hb := &w.Hexes[a], &w.Hexes[b]
	if !ha.Land() || !hb.Land() {
		return -1
	}
	c := (moveCost[ha.Biome] + moveCost[hb.Biome]) / 2
	switch {
	case ha.River && hb.River && (ha.Down == b || hb.Down == a):
		c = c / 2
	case ha.River != hb.River:
		c += 30
	}
	return c
}

// Distance is the hex-grid distance between two hexes.
func (w *World) Distance(a, b int32) int32 {
	ha, hb := &w.Hexes[a], &w.Hexes[b]
	q1, r1 := ha.Col-(ha.Row-(ha.Row&1))/2, ha.Row
	q2, r2 := hb.Col-(hb.Row-(hb.Row&1))/2, hb.Row
	dq, dr := q1-q2, r1-r2
	ds := -dq - dr
	return (abs(dq) + abs(dr) + abs(ds)) / 2
}

func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
