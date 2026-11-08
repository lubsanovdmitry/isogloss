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
	cur := *proto.Lex.At(c)
	ph := proto.Phonotax
	stress := proto.Stress
	var steps []TraceStep
	adopted := v.Adopted()
	for i, e := range adopted {
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
		case Lexical:
			if in.Lex.Concept == c {
				f := finish(in.Lex.Form, &ph, stress, v.Repair, v.Epenth)
				steps = append(steps, TraceStep{e.Tick, in.ID, "replaced: " + in.Lex.How, cur.Form, f})
				cur = lex.Lexeme{Concept: c, Form: f, Freq: in.Lex.Freq, Stratum: lex.Native, Etym: in.Lex.Etym}
			}
		}
	}
	cur.Hash = lex.FormHash(cur.Form)
	return steps, cur
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
