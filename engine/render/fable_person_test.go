package render

import (
	"testing"

	"isogloss/engine/morph"
	"isogloss/engine/sim"
)

// TestTensesAgreeWithEveryPerson: future, perfect and progressive forms
// differ by person wherever the present does (bound tenses swap the 3SG
// agreement; periphrastic ones inflect the auxiliary), and a definite
// noun in an oblique case keeps the article inside the case marker.
func TestTensesAgreeWithEveryPerson(t *testing.T) {
	bound, aux, def := 0, 0, 0
	for seed := uint64(1); seed <= 6; seed++ {
		s := sim.New(seed, sim.DefaultConfig())
		s.Run(24)
		for _, l := range s.LivingLanguages() {
			v := s.CentreVariety(l)
			r := &renderer{v}
			// Does the present distinguish 1SG from 3SG on this verb?
			p1 := v.CellForm(concept("go"), morph.Verb, morph.CellIndex(morph.Verb, morph.P1SG))
			p3 := v.CellForm(concept("go"), morph.Verb, morph.CellIndex(morph.Verb, morph.P3SG))
			if morph.SameSegments(p1, p3) {
				continue
			}
			for _, tam := range []morph.Slot{morph.FUT, morph.PRF, morph.PROG} {
				c1 := Clause{Subj: NP{Pron: "i"}, Verb: "go", TAM: tam, Person: morph.P1SG}
				c3 := Clause{Subj: NP{Pron: "he"}, Verb: "go", TAM: tam, Person: morph.P3SG}
				w1, w3 := r.verb(c1), r.verb(c3)
				if len(w1) != len(w3) {
					t.Fatalf("seed %d %s %s: %d vs %d words", seed, l.Name, tam, len(w1), len(w3))
				}
				same := true
				for i := range w1 {
					same = same && w1[i].Form == w3[i].Form
				}
				switch e := v.M.Exp[tam]; {
				case e.Stage.Bound():
					bound++
					if same {
						t.Errorf("seed %d %s: bound %s does not agree: %v", seed, l.Name, tam, w1)
					}
				case e.Stage.Free() && e.Source >= 0:
					if _, ok := r.auxVerb(tam); ok {
						aux++
						if same {
							t.Errorf("seed %d %s: auxiliary for %s does not agree: %v", seed, l.Name, tam, w1)
						}
					}
				}
			}
			if v.M.Exp[morph.DEF].Stage.Bound() && v.M.Exp[morph.DAT].Stage.Bound() {
				def++
				ws := r.np(&NP{Noun: "woman", Def: true}, morph.DAT)
				defForm := v.CellForm(concept("woman"), morph.Noun, morph.CellIndex(morph.Noun, morph.DEF))
				if len(ws) != 1 || len([]rune(ws[0].Form)) < len(defForm.Segs) {
					t.Errorf("seed %d %s: definite dative %v", seed, l.Name, ws)
				}
			}
		}
	}
	if bound == 0 || aux == 0 {
		t.Errorf("cases not exercised: %d bound, %d auxiliary, %d definite oblique", bound, aux, def)
	}
}
