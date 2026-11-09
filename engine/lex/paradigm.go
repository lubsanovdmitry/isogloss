package lex

import (
	"slices"
	"sync"

	"isogloss/engine/morph"
	"isogloss/engine/phon"
)

// TrackedLex is a lexeme whose full paradigm every variety stores. Other
// lexemes inflect regularly (by the productive pattern) when rendered.
type TrackedLex struct {
	Concept ConceptID
	POS     morph.POS
}

var (
	trackedOnce sync.Once
	tracked     []TrackedLex
	trackedIdx  []int16
)

// Tracked returns the lexemes with stored paradigms: Swadesh nouns and
// verbs, up to six from each frequency class and sixteen per part of
// speech, so that frequent and rare words can be compared (analogy levels
// rare ones).
func Tracked() []TrackedLex {
	trackedOnce.Do(func() {
		cs := Concepts()
		trackedIdx = make([]int16, len(cs))
		for i := range trackedIdx {
			trackedIdx[i] = -1
		}
		for _, pos := range []morph.POS{morph.Noun, morph.Verb} {
			cat := "Person/Thing"
			if pos == morph.Verb {
				cat = "Action/Process"
			}
			total := 0
			for f := uint8(1); f <= 5; f++ {
				n := 0
				for _, c := range cs {
					if n == 6 || total == 16 || !c.Core() || c.Taboo || c.Category != cat || c.Freq != f {
						continue
					}
					trackedIdx[c.ID] = int16(len(tracked))
					tracked = append(tracked, TrackedLex{c.ID, pos})
					n++
					total++
				}
			}
		}
	})
	return tracked
}

// TrackedIndex returns c's index in Tracked(), or -1.
func TrackedIndex(c ConceptID) int {
	Tracked()
	return int(trackedIdx[c])
}

// POSOf returns the part of speech that inflects a concept, if any.
func POSOf(c ConceptID) (morph.POS, bool) {
	switch Concepts()[c].Category {
	case "Person/Thing":
		return morph.Noun, true
	case "Action/Process":
		return morph.Verb, true
	}
	return 0, false
}

// Finish repairs and re-stresses a changed form, as every change does.
func Finish(w phon.Word, ph *phon.Phonotactics, stress phon.StressRule, mode phon.RepairMode, ep phon.Segment) phon.Word {
	w = w.Clone()
	phon.Repair(&w, ph, mode, ep)
	if stress != phon.StressLexical {
		phon.AssignStress(&w, stress)
	} else {
		phon.CleanStress(&w, ph)
	}
	return w
}

// Finish repairs and stresses w under v's phonology.
func (v *Variety) Finish(w phon.Word) phon.Word {
	return Finish(w, &v.Phonotax, v.Stress, v.Repair, v.Epenth)
}

// NewParadigm wraps cell forms with their hashes.
func NewParadigm(forms []phon.Word) *morph.Paradigm {
	n := len(forms)
	hs := make([]uint64, 3*n)
	p := &morph.Paradigm{Forms: forms, Hash: hs[:n:n], Seg: hs[n : 2*n : 2*n], Pat: hs[2*n:]}
	for i := range forms {
		w := &forms[i]
		p.Hash[i] = FormHash(*w)
		p.Seg[i] = morph.SegHash(*w)
		lo, hi := morph.StemSpan(*w)
		p.Pat[i] = mix(rangeHash(w, 0, lo), rangeHash(w, hi, len(w.Segs)))
	}
	return p
}

// prodPattern is the productive affixation of one cell.
type prodPattern struct {
	pre, suf phon.Word
	ok       bool
}

// SetPara stores tracked lexeme i's paradigm (v must be freshly derived).
func (v *Variety) SetPara(i int, p *morph.Paradigm) {
	v.M.Para[i] = p
	v.prod[Tracked()[i].POS] = nil
	v.prodH[Tracked()[i].POS] = 0
}

// Productive returns the affixes of cell k with the highest type frequency
// among the tracked paradigms of a part of speech (ties go to the earlier
// lexeme). This is the pattern new and levelled words follow. Results are
// cached per variety.
func (v *Variety) Productive(pos morph.POS, k int) (pre, suf phon.Word, ok bool) {
	if v.prod[pos] == nil {
		cells := morph.Cells(pos)
		ps := make([]prodPattern, len(cells))
		for c := range cells {
			p := &ps[c]
			p.pre, p.suf, p.ok = v.productive(pos, c)
		}
		v.prod[pos] = ps
	}
	p := v.prod[pos][k]
	return p.pre, p.suf, p.ok
}

func (v *Variety) productive(pos morph.POS, k int) (pre, suf phon.Word, ok bool) {
	type cand struct {
		h     uint64
		n     int
		first int // tracked index of the first lexeme with this pattern
	}
	var buf [16]cand
	cands := buf[:0]
	for i, t := range Tracked() {
		if t.POS != pos || i >= len(v.M.Para) || v.M.Para[i] == nil {
			continue
		}
		h := v.M.Para[i].Pat[k]
		found := false
		for j := range cands {
			if cands[j].h == h {
				cands[j].n++
				found = true
				break
			}
		}
		if !found {
			cands = append(cands, cand{h, 1, i})
		}
	}
	best := -1
	for j, c := range cands {
		if best < 0 || c.n > cands[best].n {
			best = j
		}
	}
	if best < 0 {
		return phon.Word{}, phon.Word{}, false
	}
	pre, _, suf = morph.Split(v.M.Para[cands[best].first].Forms[k])
	return pre, suf, true
}

// rangeHash hashes segments [a, b) of w with the boundaries inside or at
// the edges of that range (positions relative to a), like FormHash of the
// corresponding part from morph.Split.
func rangeHash(w *phon.Word, a, b int) uint64 {
	h := uint64(0xcbf29ce484222325)
	for _, s := range w.Segs[a:b] {
		h = mix(h, s.F.Val)
		h = mix(h, s.F.Mask^uint64(s.Tone)<<58^uint64(s.Stress)<<61)
	}
	for _, bd := range w.Bounds {
		if bd.At >= a && bd.At <= b {
			h = mix(h, uint64(bd.At-a)<<8|uint64(bd.Kind)|uint64(bd.Tag)<<16)
		}
	}
	return h
}

// RegularForms builds every cell of a paradigm from a stem and the
// productive affixes.
func (v *Variety) RegularForms(pos morph.POS, stem phon.Word) []phon.Word {
	st := phon.Word{Segs: stem.Segs, Bounds: stem.Bounds}
	key := [3]uint64{v.prodHash(pos), FormHash(st), v.finishHash()}
	if out, ok := regMemo.get(key); ok {
		return slices.Clone(out)
	}
	cells := morph.Cells(pos)
	out := make([]phon.Word, len(cells))
	for k := range cells {
		pre, suf, ok := v.Productive(pos, k)
		if !ok {
			out[k] = stem.Clone()
			continue
		}
		out[k] = v.Finish(morph.Join(pre, st, suf))
	}
	regMemo.put(key, out)
	return slices.Clone(out)
}

// regMemo remembers RegularForms results. They depend only on the
// productive patterns, the stem and the variety's phonology (the key), and
// neighbouring varieties share all three, so analogy spreading through
// them rebuilds the same paradigms. Forms are never modified once built.
var regMemo = memoForms{m: map[[3]uint64][]phon.Word{}}

type memoForms struct {
	mu sync.Mutex
	m  map[[3]uint64][]phon.Word
}

func (c *memoForms) get(k [3]uint64) ([]phon.Word, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.m[k]
	return f, ok
}

func (c *memoForms) put(k [3]uint64, f []phon.Word) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= 50_000 {
		clear(c.m) // only a cache
	}
	c.m[k] = f
}

// prodHash hashes the productive patterns of a part of speech (cached).
func (v *Variety) prodHash(pos morph.POS) uint64 {
	if v.prodH[pos] != 0 {
		return v.prodH[pos]
	}
	v.Productive(pos, 0)
	h := uint64(0x84222325cbf29ce4) ^ uint64(pos)
	for _, p := range v.prod[pos] {
		if !p.ok {
			h = mix(h, 1)
			continue
		}
		h = mix(mix(h, FormHash(p.pre)), FormHash(p.suf))
	}
	v.prodH[pos] = h | 1
	return v.prodH[pos]
}

// finishHash hashes what Finish depends on: phonotactics, stress rule,
// repair strategy and epenthetic segment.
func (v *Variety) finishHash() uint64 {
	t := v.Phonotax.Template
	h := mix(uint64(t.MinOnset)|uint64(t.MaxOnset)<<8|uint64(t.MinCoda)<<16|uint64(t.MaxCoda)<<24, uint64(v.Stress)<<8|uint64(v.Repair))
	if t.LongV {
		h = mix(h, 3)
	}
	if v.Phonotax.SClusters {
		h = mix(h, 5)
	}
	for _, b := range v.Phonotax.Banned {
		h = mix(mix(mix(mix(h, b[0].Val), b[0].Mask), b[1].Val), b[1].Mask)
	}
	return mix(mix(mix(h, v.Epenth.F.Val), v.Epenth.F.Mask), uint64(v.Epenth.Tone))
}

// BaseStem is the stem of tracked lexeme i's base cell, which analogy
// levels from (falling back to the citation form).
func (v *Variety) BaseStem(i int) phon.Word {
	t := Tracked()[i]
	if p := v.M.Para[i]; p != nil {
		_, st, _ := morph.Split(p.Forms[morph.BaseCell(t.POS)])
		if len(st.Segs) > 0 {
			return st
		}
	}
	return v.Lex.At(t.Concept).Form
}

// Regularize rebuilds tracked lexeme i's paradigm from its base stem
// (levelling and extension by analogy).
func (v *Variety) Regularize(i int) {
	t := Tracked()[i]
	v.SetPara(i, NewParadigm(v.RegularForms(t.POS, v.BaseStem(i))))
}

// Reinflect rebuilds tracked lexeme i's paradigm from its (new) citation
// form: a new word inflects by the productive pattern.
func (v *Variety) Reinflect(i int) {
	t := Tracked()[i]
	v.SetPara(i, NewParadigm(v.RegularForms(t.POS, v.Lex.At(t.Concept).Form)))
}

// Irregular reports whether tracked lexeme i has a form the productive
// pattern would not give its base stem.
func (v *Variety) Irregular(i int) bool {
	p := v.M.Para[i]
	if p == nil {
		return false
	}
	t := Tracked()[i]
	reg := v.RegularForms(t.POS, v.BaseStem(i))
	for k := range reg {
		if !morph.SameSegments(reg[k], p.Forms[k]) {
			return true
		}
	}
	return false
}

// IrregularCells flags the cells of tracked lexeme i that the productive
// pattern would not give.
func (v *Variety) IrregularCells(i int) []bool {
	p := v.M.Para[i]
	t := Tracked()[i]
	reg := v.RegularForms(t.POS, v.BaseStem(i))
	out := make([]bool, len(reg))
	for k := range reg {
		out[k] = p != nil && !morph.SameSegments(reg[k], p.Forms[k])
	}
	return out
}

// Stems returns the distinct stem allomorphs (segments only) in tracked
// lexeme i's paradigm, in cell order.
func (v *Variety) Stems(i int) []phon.Word {
	p := v.M.Para[i]
	if p == nil {
		return nil
	}
	var out []phon.Word
	for _, f := range p.Forms {
		_, st, _ := morph.Split(f)
		dup := false
		for _, o := range out {
			dup = dup || morph.SameSegments(o, st)
		}
		if !dup {
			out = append(out, st)
		}
	}
	return out
}

// CellForm returns concept c's form in cell k: the stored form for tracked
// lexemes, otherwise the regular one.
func (v *Variety) CellForm(c ConceptID, pos morph.POS, k int) phon.Word {
	if i := TrackedIndex(c); i >= 0 && i < len(v.M.Para) && v.M.Para[i] != nil && Tracked()[i].POS == pos {
		return v.M.Para[i].Forms[k]
	}
	stem := v.Lex.At(c).Form
	pre, suf, ok := v.Productive(pos, k)
	if !ok {
		return stem
	}
	return v.Finish(morph.Join(pre, phon.Word{Segs: stem.Segs, Bounds: stem.Bounds}, suf))
}
