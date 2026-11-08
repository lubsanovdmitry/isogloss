package history

import (
	"cmp"
	"slices"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

// relaxShare: if more than this per-mille of the lexicon would need
// phonotactic repair after a change, the language relaxes its syllable
// template instead (after enough syncope, clusters become legal).
const relaxShare = 80

// Applier applies innovations to varieties, memoising per-word rule results
// (neighbouring varieties share most of their forms).
type Applier struct {
	Reg  *Registry
	memo map[uint32]map[memoKey]memoVal // per innovation
}

type memoKey struct {
	form uint64
	ctx  uint64
}

type memoVal struct {
	w       phon.Word // raw rule output
	fin     phon.Word // after repair and stress
	changed bool
	illegal bool // raw output breaks the variety's phonotactics
}

// NewApplier returns an Applier over a registry.
func NewApplier(reg *Registry) *Applier {
	return &Applier{Reg: reg, memo: map[uint32]map[memoKey]memoVal{}}
}

// Forget drops memoised results for an innovation that no longer spreads.
func (a *Applier) Forget(id uint32) {
	delete(a.memo, id)
}

func tmplHash(t phon.Template, sc bool) uint64 {
	h := uint64(t.MinOnset) | uint64(t.MaxOnset)<<8 | uint64(t.MinCoda)<<16 | uint64(t.MaxCoda)<<24
	if t.LongV {
		h |= 1 << 32
	}
	if sc {
		h |= 1 << 33
	}
	return h
}

// covers reports whether the entry's frequency range includes a lexeme.
func covers(e lex.Entry, freq uint8) bool {
	return freq >= e.Lo && freq <= e.Hi
}

// finish repairs and re-stresses a changed form, as every change does.
func finish(w phon.Word, ph *phon.Phonotactics, stress phon.StressRule, mode phon.RepairMode, ep phon.Segment) phon.Word {
	w = w.Clone()
	phon.Repair(&w, ph, mode, ep)
	if stress != phon.StressLexical {
		phon.AssignStress(&w, stress)
	} else {
		phon.CleanStress(&w, ph)
	}
	return w
}

// Apply returns the variety that results from adopting entry e (an
// innovation) in v, plus a relaxation entry if one was needed. The input
// variety is not modified.
func (a *Applier) Apply(v *lex.Variety, e lex.Entry) *lex.Variety {
	in := a.Reg.Get(e.Innov)
	nv := v.Derive(e)
	switch in.Kind {
	case Sound:
		a.applySound(v, nv, in, e)
		nv.Invalidate()
		old := v.Inventory()
		now := nv.Inventory()
		var lost []phon.FeatureSet
		for _, f := range old {
			if _, found := slices.BinarySearchFunc(now, f, cmpFeat); !found {
				lost = append(lost, f)
			}
		}
		if len(lost) > 0 {
			nv.Vacated, nv.VacTick = lost, e.Tick
		}
	case Lexical:
		lc := in.Lex
		if int(lc.Concept) >= nv.Lex.Len() {
			break
		}
		f := finish(lc.Form, &nv.Phonotax, nv.Stress, nv.Repair, nv.Epenth)
		nv.Set(lc.Concept, lex.Lexeme{Concept: lc.Concept, Form: f, Freq: lc.Freq, Stratum: lex.Native, Etym: lc.Etym, Hash: lex.FormHash(f)})
	}
	nv.Invalidate()
	return nv
}

func (a *Applier) applySound(v, nv *lex.Variety, in *Innovation, e lex.Entry) {
	if in.Stress >= 0 {
		nv.Stress = phon.StressRule(in.Stress) // before memo context: finish uses it
	}
	ctx := tmplHash(v.Phonotax.Template, v.Phonotax.SClusters) ^
		uint64(nv.Stress)<<40 ^ uint64(v.Repair)<<44 ^ v.Epenth.F.Val*0x9E3779B97F4A7C15 ^ v.Epenth.F.Mask
	type changedWord struct {
		c   lex.ConceptID
		raw phon.Word
		fin phon.Word
	}
	var changed []changedWord
	violators := 0
	memo := a.memo[in.ID]
	if memo == nil {
		memo = map[memoKey]memoVal{}
		a.memo[in.ID] = memo
	}
	for i := 0; i < v.Lex.Len(); i++ {
		l := v.Lex.At(lex.ConceptID(i))
		if !covers(e, l.Freq) {
			continue
		}
		k := memoKey{l.Hash, ctx}
		mv, ok := memo[k]
		if !ok {
			w, ch := in.Rule.Apply(l.Form, &v.Phonotax)
			mv = memoVal{w: w, changed: ch}
			if ch {
				probe := w.Clone()
				mv.illegal = !phon.Legal(&probe, &v.Phonotax)
				mv.fin = finish(w, &v.Phonotax, nv.Stress, v.Repair, v.Epenth)
			}
			memo[k] = mv
		}
		if !mv.changed {
			continue
		}
		changed = append(changed, changedWord{lex.ConceptID(i), mv.w, mv.fin})
		if mv.illegal {
			violators++
		}
	}
	// Relax the template if repair would hit too much of the lexicon.
	relaxedNow := false
	if violators*1000 > relaxShare*v.Lex.Len() {
		t := nv.Phonotax.Template
		relaxed := t
		for _, cw := range changed {
			w := cw.raw.Clone()
			phon.Resyllabify(&w, &phon.DefaultPhonotactics)
			for _, s := range w.Syll {
				relaxed.MaxOnset = max(relaxed.MaxOnset, min(s.Nucleus-s.Start, 3))
				relaxed.MaxCoda = max(relaxed.MaxCoda, min(s.End-s.Nucleus-1, 3))
			}
		}
		if relaxed != t {
			relaxed.MinOnset = 0
			nv.Phonotax.Template = relaxed
			if relaxed.MaxOnset >= 2 {
				nv.Phonotax.SClusters = true
			}
			nv.Push(lex.Entry{Kind: lex.EntryRelax, Tick: e.Tick, Template: relaxed})
			relaxedNow = true
		}
	}
	for _, cw := range changed {
		f := cw.fin
		if relaxedNow {
			f = finish(cw.raw, &nv.Phonotax, nv.Stress, nv.Repair, nv.Epenth)
		}
		nv.SetForm(cw.c, f)
	}
	// A fixed stress rule installed by this change re-stresses every word.
	if in.Stress >= 0 && nv.Stress != phon.StressLexical {
		for i := 0; i < nv.Lex.Len(); i++ {
			w := nv.Lex.At(lex.ConceptID(i)).Form.Clone()
			phon.CleanStress(&w, &nv.Phonotax)
			phon.AssignStress(&w, nv.Stress)
			if !w.Equal(nv.Lex.At(lex.ConceptID(i)).Form) {
				nv.SetForm(lex.ConceptID(i), w)
			}
		}
	}
}

func cmpFeat(a, b phon.FeatureSet) int {
	if a.Val != b.Val {
		return cmp.Compare(a.Val, b.Val)
	}
	return cmp.Compare(a.Mask, b.Mask)
}
