package lex

import (
	"slices"

	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

// Initial exponent probabilities (per-mille) for an agglutinating
// proto-language: bound (affix or clitic), else periphrastic, else none.
var (
	boundPM = [morph.NumSlots]uint32{
		morph.PL: 600, morph.ACC: 450, morph.GEN: 450, morph.DAT: 350, morph.LOC: 350, morph.DEF: 100,
		morph.PST: 550, morph.FUT: 200, morph.PRF: 150, morph.PROG: 50,
	}
	periPM = [morph.NumSlots]uint32{
		morph.PL: 200, morph.ACC: 150, morph.GEN: 400, morph.DAT: 400, morph.LOC: 500, morph.DEF: 350,
		morph.PST: 250, morph.FUT: 500, morph.PRF: 450, morph.PROG: 400,
	}
)

// PrefixPM is the per-mille chance that an exponent of a group sits before
// its host, given word order: case markers follow adposition order, tense
// auxiliaries follow the verb in OV languages, articles follow demonstrative
// order, person markers come from subject pronouns. Suffixes are favoured
// throughout (the suffixing preference).
func PrefixPM(g morph.Group, o morph.Order) uint32 {
	switch g {
	case morph.GNumber:
		if o[morph.AdjN] == 0 {
			return 150
		}
		return 50
	case morph.GCase:
		if o.Postpositional() {
			return 30
		}
		return 300
	case morph.GDef:
		if o[morph.DemN] == 0 {
			return 300
		}
		return 50
	case morph.GTAM:
		if o.OV() {
			return 50
		}
		return 300
	default:
		if o.SV() {
			return 350
		}
		return 100
	}
}

// affix generates one exponent's material: a short syllable, often vowel-
// initial for suffixes.
func (p *Proto) affix(r *rng.Stream, prefix bool) []phon.Segment {
	w := p.root(r, 1)
	segs := w.Segs
	if !prefix && len(segs) > 1 && !segs[0].IsVowel() && p.Variety.Phonotax.Template.MinOnset == 0 && r.Permille(400) {
		segs = segs[1:]
	}
	if prefix && len(segs) > 1 && !segs[len(segs)-1].IsVowel() && r.Permille(400) {
		segs = segs[:len(segs)-1]
	}
	out := slices.Clone(segs)
	for i := range out {
		out[i].Stress = 0
	}
	return out
}

func sameSegs(a, b []phon.Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].F != b[i].F {
			return false
		}
	}
	return true
}

// genMorph gives the proto-language word order, exponents and the
// paradigms of tracked lexemes.
func (p *Proto) genMorph(r *rng.Stream) {
	v := p.Variety
	m := &v.M
	m.Order = morph.SampleOrder(r)
	for i := range m.Exp {
		m.Exp[i].Source = -1
	}
	scale := []uint32{150, 1000, 1400}[r.Weighted([]uint32{20, 50, 30})] // isolating, agglutinating, synthetic
	var prefix [morph.GAgr + 1]bool
	for g := morph.GNumber; g <= morph.GAgr; g++ {
		prefix[g] = r.Permille(PrefixPM(g, m.Order))
	}
	mats := make([][]phon.Segment, morph.NumSlots)
	fresh := func(s morph.Slot, pre bool) []phon.Segment {
		var mat []phon.Segment
		for try := 0; try < 12; try++ {
			mat = p.affix(r, pre)
			clash := false
			for o := range mats {
				clash = clash || mats[o] != nil && morph.Slot(o).Info().POS == s.Info().POS && sameSegs(mats[o], mat)
			}
			if !clash {
				break
			}
		}
		return mat
	}
	set := func(s morph.Slot, bound bool) {
		info := s.Info()
		e := &m.Exp[s]
		e.Source = -1
		e.Prefix = prefix[info.Group]
		switch {
		case bound:
			e.Stage = morph.Affix
			if r.Permille(200) {
				e.Stage = morph.Clitic
			}
			mats[s] = fresh(s, e.Prefix)
		case r.Permille(periPM[s]):
			var srcs []int32
			for _, l := range info.Sources {
				if c, ok := ConceptByLabel(l); ok {
					srcs = append(srcs, int32(c))
				}
			}
			if len(srcs) == 0 {
				return
			}
			e.Stage = morph.Periphrasis
			if r.Permille(500) {
				e.Stage = morph.Fixed
			}
			e.Source = srcs[r.IntN(len(srcs))]
		}
	}
	for s := morph.PL; s <= morph.PROG; s++ {
		set(s, r.Permille(min(boundPM[s]*scale/1000, 950)))
	}
	agr := r.Permille(min(600*scale/1000, 950))
	zero3 := r.Permille(500)
	for s := morph.P1SG; s <= morph.P3PL; s++ {
		if agr && !(s == morph.P3SG && zero3) {
			set(s, true)
		}
	}
	// Paradigms: attach bound exponents innermost group first.
	tr := Tracked()
	m.Para = make([]*morph.Paradigm, len(tr))
	for i, t := range tr {
		stem := v.Lex.At(t.Concept).Form
		cells := morph.Cells(t.POS)
		forms := make([]phon.Word, len(cells))
		for k, c := range cells {
			order := slices.Clone(c.Slots)
			slices.SortStableFunc(order, func(a, b morph.Slot) int { return int(a.Info().Group) - int(b.Info().Group) })
			w := stem.Clone()
			for _, s := range order {
				e := m.Exp[s]
				if !e.Stage.Bound() {
					continue
				}
				kind := phon.BMorpheme
				if e.Stage == morph.Clitic {
					kind = phon.BClitic
				}
				w = morph.Attach(w, mats[s], s, e.Prefix, kind)
			}
			forms[k] = v.Finish(w)
		}
		m.Para[i] = NewParadigm(forms)
	}
}
