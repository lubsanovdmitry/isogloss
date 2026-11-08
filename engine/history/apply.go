package history

import (
	"cmp"
	"slices"

	"isogloss/engine/lex"
	"isogloss/engine/morph"
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
	memo map[uint32]map[memoKey]*memoVal // per innovation
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

// unchanged is shared by every memo entry whose word the rule left alone.
var unchanged = &memoVal{}

// NewApplier returns an Applier over a registry.
func NewApplier(reg *Registry) *Applier {
	return &Applier{Reg: reg, memo: map[uint32]map[memoKey]*memoVal{}}
}

// Track enables memoisation for an innovation while it spreads; results
// for untracked innovations (late diffusion steps, replays) are not kept.
func (a *Applier) Track(id uint32) {
	if a.memo[id] == nil {
		a.memo[id] = map[memoKey]*memoVal{}
	}
}

// Retain keeps memoised results only for the given innovations; others
// still in use are re-tracked.
func (a *Applier) Retain(keep map[uint32]bool) {
	for id := range a.memo { //lint:ordered deleting entries is order-independent
		if !keep[id] {
			delete(a.memo, id)
		}
	}
	for id := range keep { //lint:ordered map inserts are order-independent
		if a.memo[id] == nil && a.Reg.Get(id).Kind == Sound {
			a.memo[id] = map[memoKey]*memoVal{}
		}
	}
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

func finish(w phon.Word, ph *phon.Phonotactics, stress phon.StressRule, mode phon.RepairMode, ep phon.Segment) phon.Word {
	return lex.Finish(w, ph, stress, mode, ep)
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
	case Lexical, Loan:
		applyLex(nv, in)
	case Semantic:
		sc := in.Sem
		w, sense := nv.Lex.At(sc.Word), nv.Lex.At(sc.Sense)
		if w.Etym == sc.Etym && sense.Etym != sc.Etym && !sense.Absent() {
			l := *w
			l.Also = uint16(sc.Sense) + 1
			nv.Lex.Set(sc.Word, l) // form unchanged: no inventory recount
		}
	case Gram:
		applyGram(nv, in.Gram, e.Tick)
	case Morph:
		if i := lex.TrackedIndex(in.Target); i >= 0 {
			nv.Regularize(i)
		}
	case Order:
		nv.M.Order[in.Order.Param] = in.Order.Value
	}
	nv.Invalidate()
	return nv
}

// NewForm is the form a lexical change or loan gives its concept in a
// variety (ok false where it does not apply).
func NewForm(v *lex.Variety, in *Innovation) (f phon.Word, st lex.Stratum, ok bool) {
	lc := in.Lex
	cur := v.Lex.At(lc.Concept)
	if lc.Gap && !cur.Absent() {
		return f, st, false
	}
	st = lc.Stratum
	switch {
	case lc.From >= 0:
		src := v.Lex.At(lex.ConceptID(lc.From))
		if !shiftApplies(lc, cur, src) {
			return f, st, false // the local word has gone another way
		}
		return src.Form, src.Stratum, true
	}
	// Loans were nativized where they were borrowed and spread as words.
	f = finish(lc.Form, &v.Phonotax, v.Stress, v.Repair, v.Epenth)
	if in.Kind == Loan && cur.Etym == lc.Etym && sameSegments(f, cur.Form) {
		return f, st, false // the loan sounds like the word already used
	}
	return f, st, true
}

func applyLex(nv *lex.Variety, in *Innovation) {
	lc := in.Lex
	f, st, ok := NewForm(nv, in)
	if !ok {
		return
	}
	nv.Set(lc.Concept, lex.Lexeme{Concept: lc.Concept, Form: f, Freq: lc.Freq, Stratum: st, Etym: lc.Etym, Hash: lex.FormHash(f)})
	if lc.From >= 0 { // the shift is complete: the extra sense is now the word's own
		if src := nv.Lex.At(lex.ConceptID(lc.From)); src.Also == uint16(lc.Concept)+1 {
			l := *src
			l.Also = 0
			nv.Lex.Set(lex.ConceptID(lc.From), l)
		}
	}
	if i := lex.TrackedIndex(lc.Concept); i >= 0 {
		nv.Reinflect(i) // a new word inflects by the productive pattern
	}
}

// cliticMaterial is what a free word contributes when it cliticizes: its
// first syllable, unstressed (grammatical words are short and reduced).
func cliticMaterial(w phon.Word) []phon.Segment {
	w = w.Clone()
	phon.Resyllabify(&w, nil)
	end := len(w.Segs)
	if len(w.Syll) > 1 {
		end = w.Syll[0].End
	}
	return w.Segs[:end]
}

func applyGram(nv *lex.Variety, g *GramChange, tick int32) {
	e := &nv.M.Exp[g.Slot]
	switch g.To {
	case morph.Periphrasis:
		if e.Stage != morph.None && e.Stage != morph.Lost {
			return
		}
		*e = morph.Exponent{Stage: morph.Periphrasis, Source: g.Source, Prefix: g.Prefix, Since: tick}
	case morph.Fixed:
		if e.Stage != morph.Periphrasis || e.Source != g.Source {
			return
		}
		e.Stage, e.Since = morph.Fixed, tick
	case morph.Clitic:
		if e.Stage != morph.Fixed || e.Source != g.Source || e.Source < 0 {
			return
		}
		mat := cliticMaterial(nv.Lex.At(lex.ConceptID(e.Source)).Form)
		pos := g.Slot.Info().POS
		cells := morph.Cells(pos)
		for i, t := range lex.Tracked() {
			if t.POS != pos || nv.M.Para[i] == nil {
				continue
			}
			forms := slices.Clone(nv.M.Para[i].Forms)
			for k, c := range cells {
				if c.Has(g.Slot) {
					forms[k] = nv.Finish(morph.Attach(forms[k], mat, g.Slot, e.Prefix, phon.BClitic))
				}
			}
			nv.SetPara(i, lex.NewParadigm(forms))
		}
		e.Stage, e.Since = morph.Clitic, tick
	case morph.Affix:
		if e.Stage != morph.Clitic || e.Source != g.Source {
			return
		}
		pos := g.Slot.Info().POS
		for i, t := range lex.Tracked() {
			if t.POS != pos || nv.M.Para[i] == nil {
				continue
			}
			forms := nv.M.Para[i].Forms
			var out []phon.Word
			for k, f := range forms {
				if r, ch := morph.Rekind(f, g.Slot, phon.BMorpheme); ch {
					if out == nil {
						out = slices.Clone(forms)
					}
					out[k] = r
				}
			}
			if out != nil {
				nv.SetPara(i, lex.NewParadigm(out))
			}
		}
		e.Stage, e.Since = morph.Affix, tick
	}
}

// checkLoss marks bound exponents that sound change has worn away: when at
// least three quarters of the cells with the slot sound the same as their
// partner cells without it, the distinction is lost.
func checkLoss(nv *lex.Variety, tick int32) {
	for s := morph.Slot(0); s < morph.NumSlots; s++ {
		e := &nv.M.Exp[s]
		if !e.Stage.Bound() {
			continue
		}
		pos := s.Info().POS
		pairs := morph.Pairs(s)
		same, total := 0, 0
		for i, t := range lex.Tracked() {
			p := nv.M.Para[i]
			if t.POS != pos || p == nil {
				continue
			}
			for _, pr := range pairs {
				total++
				if p.Seg[pr.Cell] == p.Seg[pr.Partner] {
					same++
				}
			}
		}
		if total > 0 && same*4 >= total*3 {
			e.Stage, e.Since = morph.Lost, tick
		}
	}
}

func (a *Applier) applySound(v, nv *lex.Variety, in *Innovation, e lex.Entry) {
	if in.Stress >= 0 {
		nv.Stress = phon.StressRule(in.Stress) // before memo context: finish uses it
	}
	ctx := tmplHash(v.Phonotax.Template, v.Phonotax.SClusters) ^
		uint64(nv.Stress)<<40 ^ uint64(v.Repair)<<44 ^ v.Epenth.F.Val*0x9E3779B97F4A7C15 ^ v.Epenth.F.Mask
	type changedWord struct {
		c  lex.ConceptID
		mv *memoVal
	}
	var changed []changedWord
	violators := 0
	memo := a.memo[in.ID]
	if memo == nil {
		memo = map[memoKey]*memoVal{} // scratch: not kept
	}
	for i := 0; i < v.Lex.Len(); i++ {
		l := v.Lex.At(lex.ConceptID(i))
		if !covers(e, l.Freq) {
			continue
		}
		if !in.Rule.MayApply(&l.Form) {
			continue
		}
		k := memoKey{l.Hash, ctx}
		mv, ok := memo[k]
		if !ok {
			w, ch := in.Rule.Apply(l.Form, &v.Phonotax)
			mv = unchanged
			if ch {
				mv = &memoVal{w: w, changed: true}
				probe := w.Clone()
				mv.illegal = !phon.Legal(&probe, &v.Phonotax)
				mv.fin = finish(w, &v.Phonotax, nv.Stress, v.Repair, v.Epenth)
			}
			memo[k] = mv
		}
		if !mv.changed {
			continue
		}
		changed = append(changed, changedWord{lex.ConceptID(i), mv})
		if mv.illegal {
			violators++
		}
	}
	// Paradigm cells change by the same rule (memoised alike).
	type changedCell struct {
		para, cell int
		mv         *memoVal
	}
	var cellChanges []changedCell
	for pi, t := range lex.Tracked() {
		p := v.M.Para[pi]
		if p == nil || !covers(e, v.Lex.At(t.Concept).Freq) {
			continue
		}
		for k, f := range p.Forms {
			if !in.Rule.MayApply(&f) {
				continue
			}
			mk := memoKey{p.Hash[k], ctx}
			mv, ok := memo[mk]
			if !ok {
				w, ch := in.Rule.Apply(f, &v.Phonotax)
				mv = unchanged
				if ch {
					mv = &memoVal{w: w, changed: true}
					mv.fin = finish(w, &v.Phonotax, nv.Stress, v.Repair, v.Epenth)
				}
				memo[mk] = mv
			}
			if mv.changed {
				cellChanges = append(cellChanges, changedCell{pi, k, mv})
			}
		}
	}
	// Relax the template if repair would hit too much of the lexicon.
	relaxedNow := false
	if violators*1000 > relaxShare*v.Lex.Len() {
		t := nv.Phonotax.Template
		relaxed := t
		for _, cw := range changed {
			w := cw.mv.w.Clone()
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
		f := cw.mv.fin
		if relaxedNow {
			f = finish(cw.mv.w, &nv.Phonotax, nv.Stress, nv.Repair, nv.Epenth)
		}
		nv.SetForm(cw.c, f)
	}
	for i := 0; i < len(cellChanges); {
		pi := cellChanges[i].para
		forms := slices.Clone(nv.M.Para[pi].Forms)
		for ; i < len(cellChanges) && cellChanges[i].para == pi; i++ {
			cc := cellChanges[i]
			forms[cc.cell] = cc.mv.fin
			if relaxedNow {
				forms[cc.cell] = finish(cc.mv.w, &nv.Phonotax, nv.Stress, nv.Repair, nv.Epenth)
			}
		}
		nv.SetPara(pi, lex.NewParadigm(forms))
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
		for pi, p := range nv.M.Para {
			if p == nil {
				continue
			}
			var forms []phon.Word
			for k, f := range p.Forms {
				w := f.Clone()
				phon.CleanStress(&w, &nv.Phonotax)
				phon.AssignStress(&w, nv.Stress)
				if !w.Equal(f) {
					if forms == nil {
						forms = slices.Clone(p.Forms)
					}
					forms[k] = w
				}
			}
			if forms != nil {
				nv.SetPara(pi, lex.NewParadigm(forms))
			}
		}
	}
	if len(cellChanges) > 0 {
		checkLoss(nv, e.Tick)
	}
}

func cmpFeat(a, b phon.FeatureSet) int {
	if a.Val != b.Val {
		return cmp.Compare(a.Val, b.Val)
	}
	return cmp.Compare(a.Mask, b.Mask)
}
