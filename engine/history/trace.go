package history

import (
	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

// TraceStep is one change in a word's history.
type TraceStep struct {
	Tick          int32
	Innov         uint32 // 0 for repairs and phonotactic relaxations
	Label         string
	Before, After phon.Word
}

// Trace replays variety v's history on the proto-language word for concept c
// and returns every step that changed it. Replay rebuilds the word exactly;
// the final form equals v.Lex.At(c).Form (tested as an invariant).
func Trace(reg *Registry, proto, v *lex.Variety, c lex.ConceptID) ([]TraceStep, lex.Lexeme) {
	adopted := v.Adopted()
	return traceTo(reg, proto, v, adopted, len(adopted), c)
}

// noConcept stands for a word outside the lexicon (a place name).
const noConcept = lex.ConceptID(0xFFFF)

// Evolve applies the sound changes v adopted from entry index from on to a
// word that entered the variety at that point, such as a place name taken
// over from a substrate, and returns the result with its steps.
func Evolve(reg *Registry, proto, v *lex.Variety, w phon.Word, from int, freq uint8) (phon.Word, []TraceStep) {
	adopted := v.Adopted()
	start := lex.Lexeme{Concept: noConcept, Form: w, Freq: freq, Etym: 1}
	steps, l := trace(reg, proto, v, adopted, from, len(adopted), noConcept, start)
	return l.Form, steps
}

// traceTo traces concept c through the first n entries of adopted. Meaning
// shifts depend on other words (the word that takes over, the sense being
// gained), which are traced to the same point recursively.
func traceTo(reg *Registry, proto, v *lex.Variety, adopted []lex.Entry, n int, c lex.ConceptID) ([]TraceStep, lex.Lexeme) {
	return trace(reg, proto, v, adopted, 0, n, c, *proto.Lex.At(c))
}

// trace follows lexeme cur (concept c) from entry index from to n; earlier
// entries only update the phonotactics and stress rule.
func trace(reg *Registry, proto, v *lex.Variety, adopted []lex.Entry, from, n int, c lex.ConceptID, cur lex.Lexeme) ([]TraceStep, lex.Lexeme) {
	ph := proto.Phonotax
	stress := proto.Stress
	var steps []TraceStep
	for i, e := range adopted[:n] {
		if e.Kind == lex.EntryRelax {
			ph.Template = e.Template
			if e.Template.MaxOnset >= 2 {
				ph.SClusters = true
			}
			continue
		}
		in := reg.Get(e.Innov)
		next := ph
		if i+1 < len(adopted) && adopted[i+1].Kind == lex.EntryRelax {
			next.Template = adopted[i+1].Template
			if next.Template.MaxOnset >= 2 {
				next.SClusters = true
			}
		}
		if in.Kind == Syllable {
			// Mirrors applySyll: the template changes; a narrower one
			// repairs the word.
			ph.Template = in.Syll.Apply(ph.Template)
			if ph.Template.MaxOnset < 2 {
				ph.SClusters = false
			}
			if i >= from && (in.Syll.Onset < 0 || in.Syll.Coda < 0) && !cur.Absent() {
				if f := finish(cur.Form, &ph, stress, v.Repair, v.Epenth); !f.Equal(cur.Form) {
					steps = append(steps, TraceStep{e.Tick, in.ID, in.Syll.Label() + ": repair (" + v.Repair.String() + ")", cur.Form, f})
					cur.Form = f
				}
			}
			continue
		}
		if i < from {
			if in.Kind == Sound && in.Stress >= 0 {
				stress = phon.StressRule(in.Stress)
			}
			continue
		}
		switch in.Kind {
		case Sound:
			newStress := stress
			if in.Stress >= 0 {
				newStress = phon.StressRule(in.Stress)
			}
			if covers(e, cur.Freq) {
				w, ch := in.Rule.Apply(cur.Form, &ph)
				f := w
				if ch {
					f = finish(w, &next, newStress, v.Repair, v.Epenth)
				}
				if ch && !f.Equal(cur.Form) { // skip changes undone by repair/re-stress
					steps = append(steps, TraceStep{e.Tick, in.ID, in.Template, cur.Form, w})
					if !f.Equal(w) {
						label := "re-stress (" + newStress.String() + ")"
						if !sameSegments(f, w) {
							label = "repair (" + v.Repair.String() + ")"
						}
						steps = append(steps, TraceStep{e.Tick, 0, label, w, f})
					}
					cur.Form = f
				}
			}
			if in.Stress >= 0 && newStress != phon.StressLexical {
				w := cur.Form.Clone()
				phon.CleanStress(&w, &next)
				phon.AssignStress(&w, newStress)
				if !w.Equal(cur.Form) {
					steps = append(steps, TraceStep{e.Tick, 0, "re-stress (" + newStress.String() + ")", cur.Form, w})
				}
				cur.Form = w
			}
			stress = newStress
		case Lexical, Loan:
			lc := in.Lex
			if lc.Concept != c {
				if lc.From == int32(c) && cur.Also == uint16(lc.Concept)+1 {
					// A shift away from c completed: the extra sense is gone.
					if _, x := traceTo(reg, proto, v, adopted, i, lc.Concept); shiftApplies(lc, &x, &cur) {
						cur.Also = 0
					}
				}
				continue
			}
			if lc.Gap && !cur.Absent() {
				continue
			}
			var f phon.Word
			st := lc.Stratum
			if lc.From >= 0 {
				_, src := traceTo(reg, proto, v, adopted, i, lex.ConceptID(lc.From))
				if !shiftApplies(lc, &cur, &src) {
					continue
				}
				f, st = src.Form, src.Stratum
			} else {
				f = finish(lc.Form, &ph, stress, v.Repair, v.Epenth)
				if in.Kind == Loan && cur.Etym == lc.Etym && sameSegments(f, cur.Form) {
					continue
				}
			}
			label := "replaced: " + lc.How
			if in.Kind == Loan {
				label = lc.How
			}
			steps = append(steps, TraceStep{e.Tick, in.ID, label, cur.Form, f})
			cur = lex.Lexeme{Concept: c, Form: f, Freq: lc.Freq, Stratum: st, Etym: lc.Etym}
		case Semantic:
			sc := in.Sem
			if sc.Word != c || cur.Etym != sc.Etym {
				continue
			}
			if _, sense := traceTo(reg, proto, v, adopted, i, sc.Sense); sense.Etym != sc.Etym && !sense.Absent() {
				cur.Also = uint16(sc.Sense) + 1
				steps = append(steps, TraceStep{e.Tick, in.ID, "also comes to mean '" + lex.Concepts()[sc.Sense].Label + "'", cur.Form, cur.Form})
			}
		}
	}
	cur.Hash = lex.FormHash(cur.Form)
	return steps, cur
}

// shiftApplies mirrors NewForm's test for a meaning shift: the concept
// (cur) takes src's word if src still has the shifting root.
func shiftApplies(lc *LexChange, cur, src *lex.Lexeme) bool {
	return !(lc.Gap && !cur.Absent()) && src.Etym == lc.Etym && !src.Absent() && cur.Etym != lc.Etym
}

// sameSegments compares segment features only (ignoring stress and tone).
func sameSegments(a, b phon.Word) bool {
	if len(a.Segs) != len(b.Segs) {
		return false
	}
	for i := range a.Segs {
		if a.Segs[i].F != b.Segs[i].F {
			return false
		}
	}
	return true
}
