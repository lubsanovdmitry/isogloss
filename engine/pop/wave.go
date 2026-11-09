package pop

import "isogloss/engine/fx"

// Wave is one innovation's adoption level in every hex.
type Wave struct {
	Level   []fx.Q16
	Applied []bool
	next    []fx.Q16
}

// NewWave allocates a wave over n hexes.
func NewWave(n int) *Wave {
	return &Wave{Level: make([]fx.Q16, n), Applied: make([]bool, n), next: make([]fx.Q16, n)}
}

// ResetScratch gives a copied wave its own scratch buffer.
func (w *Wave) ResetScratch() { w.next = make([]fx.Q16, len(w.Level)) }

var floor = fx.One / 200 // levels below this die out

// Step advances the wave one tick:
//
//	a_i ← a_i + r_i a_i (1 − a_i) + Σ_j c_ij π_j (a_j − a_i)⁺
//
// rate(i) gives r_i as signed Q16; spreading=false turns off neighbour
// influence (the innovation has stopped travelling). Influence between
// communities of different languages (lang) is scaled by cross; with cross
// 0 the innovation stays inside the languages it reaches. It returns the
// hexes that crossed one half this tick, ascending.
func (w *Wave) Step(present []bool, contacts [][]Contact, prestige []fx.Q16, lang []int32, cross fx.Q16, rate func(int) int32, spreading bool) []int32 {
	var crossed []int32
	for i := range w.Level {
		if !present[i] {
			w.next[i] = 0
			continue
		}
		a := int64(w.Level[i])
		v := a
		if a > 0 {
			r := int64(rate(i))
			v += r * a >> 16 * (int64(fx.One) - a) >> 16
		}
		if spreading {
			for _, c := range contacts[i] {
				aj := int64(w.Level[c.To])
				if aj > a {
					inf := int64(c.W) * int64(prestige[c.To]) >> 16
					if lang[i] != lang[c.To] {
						inf = inf * int64(cross) >> 16
					}
					v += inf * (aj - a) >> 16
				}
			}
		}
		if v < int64(floor) && !w.Applied[i] {
			v = 0
		}
		if v > int64(fx.One) {
			v = int64(fx.One)
		}
		if v < 0 {
			v = 0
		}
		w.next[i] = fx.Q16(v)
		if !w.Applied[i] && v >= int64(fx.Half) {
			w.Applied[i] = true
			crossed = append(crossed, int32(i))
		}
	}
	w.Level, w.next = w.next, w.Level
	return crossed
}

// Alive reports whether any hex holds the innovation without having applied
// it yet (so it may still cross).
func (w *Wave) Alive() bool {
	for i, a := range w.Level {
		if a > 0 && !w.Applied[i] {
			return true
		}
	}
	return false
}

// Count returns how many hexes applied the innovation.
func (w *Wave) Count() int {
	n := 0
	for _, a := range w.Applied {
		if a {
			n++
		}
	}
	return n
}

// Reach returns how many hexes hold the innovation at any level or applied it.
func (w *Wave) Reach() int {
	n := 0
	for i, a := range w.Level {
		if a > 0 || w.Applied[i] {
			n++
		}
	}
	return n
}
