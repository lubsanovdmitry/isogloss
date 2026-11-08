package contact

import (
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
	"isogloss/engine/world"
)

// Substrate is a language spoken on the land before the family expands into
// it. Its speakers shift to the family's language as it arrives; the shift
// leaves loans, sometimes a sound change or a word-order change, and the
// place names.
type Substrate struct {
	ID   int32
	Name string
	V    *lex.Variety // its lexicon and grammar (generated like a proto-language)
	Seed int32        // the hex at the centre of its area
}

// GenerateSubstrates places k substrate languages over the land outside the
// families' homelands (homeland[h]). Seeds are spread out by
// farthest-point sampling, away from the homelands; every other land hex
// belongs to the nearest seed. region[h] is the substrate index, or -1 for
// homelands and sea.
func GenerateSubstrates(seed uint64, w *world.World, homeland []bool, k int) ([]*Substrate, []int32) {
	region := make([]int32, len(w.Hexes))
	for i := range region {
		region[i] = -1
	}
	var cands []int32
	for _, h := range w.Land {
		near := homeland[h]
		for _, nb := range w.Hexes[h].Nbr {
			near = near || nb >= 0 && homeland[nb]
		}
		if !near {
			cands = append(cands, h)
		}
	}
	if len(cands) == 0 || k <= 0 {
		return nil, region
	}
	r := rng.New(seed, rng.Key("substrate-seeds"))
	seeds := []int32{cands[r.IntN(len(cands))]}
	for len(seeds) < k && len(seeds) < len(cands) {
		best, bestD := int32(-1), int32(-1)
		for _, h := range cands {
			d := int32(1 << 30)
			for _, s := range seeds {
				d = min(d, w.Distance(h, s))
			}
			d = d*1000 + int32(r.IntN(1000)) // random tie-break
			if d > bestD {
				best, bestD = h, d
			}
		}
		seeds = append(seeds, best)
	}
	for _, h := range w.Land {
		if homeland[h] {
			continue
		}
		best, bestD := int32(0), int32(1<<30)
		for i, s := range seeds {
			if d := w.Distance(h, s); d < bestD {
				best, bestD = int32(i), d
			}
		}
		region[h] = best
	}
	subs := make([]*Substrate, len(seeds))
	for i, s := range seeds {
		p := lex.GenerateProto(rng.New(seed, rng.Key("substrate"), uint64(i)))
		subs[i] = &Substrate{ID: int32(i), V: p.Variety, Seed: s}
	}
	return subs, region
}

// Place is a hex's name in the language that named it.
type Place struct {
	Form  phon.Word
	Gloss string // "white river", "Tamu's hill"
	Sub   int32  // substrate that named it, -1 for a family's proto-language
	Fam   uint8  // when Sub < 0: the family whose proto-language named it
}

// Landscape words a place may be named after, by biome; rivers name the
// hexes they run through most of the time.
var (
	biomeHeads = [world.NumBiomes][]string{
		world.Coast:    {"sea", "sand", "stone"},
		world.Plains:   {"earth", "grass", "tree", "water"},
		world.Forest:   {"forest", "tree", "wolf", "bear"},
		world.Steppe:   {"grass", "wind", "earth"},
		world.Desert:   {"sand", "stone", "sun"},
		world.Mountain: {"mountain", "stone", "snow"},
		world.Island:   {"sea", "stone"},
	}
	placeMods = []string{"big", "small", "white", "black", "red", "old", "new", "long", "good", "wide"}
)

// NamePlace names hex h in variety v: a landscape word, alone, with a
// property ("white river") or with a personal name ("Tamu's hill"), ordered
// by the language's adjective–noun order.
func NamePlace(r *rng.Stream, v *lex.Variety, h *world.Hex, sub int32) Place {
	heads := biomeHeads[h.Biome]
	if h.River && r.Permille(600) {
		heads = []string{"river", "water"}
	}
	var head lex.ConceptID
	ok := false
	for try := 0; try < 4 && !ok; try++ {
		head, ok = lex.ConceptByLabel(heads[r.IntN(len(heads))])
		ok = ok && !v.Lex.At(head).Absent()
	}
	if !ok {
		head, _ = lex.ConceptByLabel("earth")
	}
	hw := v.Lex.At(head).Form
	hg := lex.Concepts()[head].Label
	var mw phon.Word
	var gloss string
	switch r.Weighted([]uint32{25, 50, 25}) {
	case 0:
		return Place{Form: plain(v, hw), Gloss: hg, Sub: sub}
	case 1:
		m, _ := lex.ConceptByLabel(placeMods[r.IntN(len(placeMods))])
		mw = v.Lex.At(m).Form
		gloss = lex.Concepts()[m].Label + " " + hg
	default:
		mw = lex.NewRoot(r, v, 4)
		gloss = phon.Capitalize(phon.Romanize(mw)) + "’s " + hg
	}
	first, second := mw, hw
	if v.M.Order[morph.AdjN] == 1 {
		first, second = hw, mw
	}
	w := phon.Word{Segs: append(append([]phon.Segment(nil), first.Segs...), second.Segs...)}
	for i := range w.Segs {
		w.Segs[i].Stress = 0
	}
	w.Bounds = []phon.Boundary{{At: len(first.Segs), Kind: phon.BMorpheme}}
	return Place{Form: v.Finish(w), Gloss: gloss, Sub: sub}
}

func plain(v *lex.Variety, w phon.Word) phon.Word {
	return v.Finish(phon.Word{Segs: append([]phon.Segment(nil), w.Segs...)})
}
