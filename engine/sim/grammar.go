package sim

import (
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
)

// Grammaticalization steps per 100 000 community-ticks, by the stage the
// exponent is leaving. A step needs only one community to start it and
// then spreads, so these are small.
var gramRatePer100k = [...]int{
	morph.None:        40, // a source word is recruited
	morph.Periphrasis: 50, // the construction fixes
	morph.Fixed:       40, // it cliticizes
	morph.Clitic:      50, // the clitic fuses into an affix
	morph.Affix:       0,  // affixes leave only by erosion
	morph.Lost:        40, // renewal after loss
}

// Analogical levelling per 100 000 community-ticks for an irregular
// paradigm, by frequency class: rare words are regularized, frequent ones
// keep their irregular forms (Bybee; Lieberman et al. 2007). Lieberman et
// al. find the rate ∝ frequency^-1/2; frequency classes are about ten times
// apart, so each class levels about √10 ≈ 3.2 times faster than the one
// above it.
var analogyRatePer100k = [6]int{0, 30, 100, 300, 1000, 3000}

// orderRatePer100k: word-order changes proposed per 100 000 community-ticks.
const orderRatePer100k = 80

func scaled(base int, k uint32) int { return base * int(k) / 1000 }

// prefixFor decides on which side a new construction sits, from word order.
func prefixFor(r *rng.Stream, s morph.Slot, o morph.Order) bool {
	return r.Permille(lex.PrefixPM(s.Info().Group, o))
}

// proposeGrammar lets a community start grammaticalization steps,
// analogical levelling and word-order changes.
func (s *Sim) proposeGrammar(r *rng.Stream, c *pop.Community) {
	v := c.Variety
	for sl := morph.Slot(0); sl < morph.NumSlots; sl++ {
		e := v.M.Exp[sl]
		rate := scaled(gramRatePer100k[e.Stage], s.Cfg.GramRate)
		if e.Stage == morph.Fixed && e.Prefix {
			rate = rate * 4 / 10 // preposed material fuses less readily
		}
		if r.IntN(100000) >= rate {
			continue
		}
		g := &history.GramChange{Slot: sl, Source: e.Source, Prefix: e.Prefix}
		switch e.Stage {
		case morph.None, morph.Lost:
			var srcs []int32
			for _, l := range sl.Info().Sources {
				if id, ok := lex.ConceptByLabel(l); ok {
					srcs = append(srcs, int32(id))
				}
			}
			if len(srcs) == 0 {
				continue
			}
			g.To, g.Source = morph.Periphrasis, srcs[r.IntN(len(srcs))]
			g.Prefix = prefixFor(r, sl, v.M.Order)
		case morph.Periphrasis:
			g.To = morph.Fixed
		case morph.Fixed:
			g.To = morph.Clitic
		case morph.Clitic:
			g.To = morph.Affix
		default:
			continue
		}
		s.newInnov(r, &history.Innovation{Kind: history.Gram, Gram: g, Stress: -1}, c.Hex)
	}
	for i, t := range lex.Tracked() {
		freq := v.Lex.At(t.Concept).Freq
		if r.IntN(100000) >= scaled(analogyRatePer100k[min(freq, 5)], s.Cfg.AnalogyRate) {
			continue
		}
		if v.Irregular(i) {
			s.newInnov(r, &history.Innovation{Kind: history.Morph, Target: t.Concept, Stress: -1}, c.Hex)
		}
	}
	if r.IntN(100000) < scaled(orderRatePer100k, s.Cfg.OrderRate) {
		if oc := proposeOrder(r, v); oc != nil {
			s.newInnov(r, &history.Innovation{Kind: history.Order, Order: oc, Stress: -1}, c.Hex)
		}
	}
}

// proposeOrder picks a parameter and a new value, favouring values that fit
// the other parameters (harmonic drift toward WALS correlations). Losing
// case marking pushes toward a fixed SVO clause order.
func proposeOrder(r *rng.Stream, v *lex.Variety) *history.OrderChange {
	o := v.M.Order
	p := morph.Param(r.IntN(int(morph.NumParams)))
	vals := p.Values()
	w := make([]uint32, len(vals))
	for i := range vals {
		if uint8(i) == o[p] {
			continue
		}
		w[i] = uint32(min(morph.Harmony(o, p, uint8(i)), 1<<30))
	}
	if p == morph.Clause {
		acc := v.M.Exp[morph.ACC].Stage
		if acc == morph.None || acc == morph.Lost {
			w[1] *= 3 // SVO
		}
	}
	nv := r.Weighted(w)
	if nv < 0 {
		return nil
	}
	// Accept in proportion to how much better the new value fits.
	hn, ho := morph.Harmony(o, p, uint8(nv)), morph.Harmony(o, p, o[p])
	if r.IntN(int(min(hn+ho, 1<<30))) >= int(min(hn, 1<<30)) {
		return nil
	}
	return &history.OrderChange{Param: p, Value: uint8(nv)}
}
