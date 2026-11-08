package lex

import (
	"testing"

	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

func TestProtoMorphology(t *testing.T) {
	tr := Tracked()
	var nouns, verbs int
	for _, x := range tr {
		if x.POS == morph.Noun {
			nouns++
		} else {
			verbs++
		}
	}
	t.Logf("tracked: %d nouns, %d verbs", nouns, verbs)
	if nouns < 10 || verbs < 10 {
		t.Fatal("too few tracked lexemes")
	}
	for seed := uint64(1); seed <= 3; seed++ {
		p := GenerateProto(rng.New(seed, rng.Key("proto")))
		v := p.Variety
		t.Logf("seed %d: order %v", seed, v.M.Order)
		for s := morph.Slot(0); s < morph.NumSlots; s++ {
			e := v.M.Exp[s]
			src := ""
			if e.Source >= 0 {
				src = Concepts()[e.Source].Label
			}
			t.Logf("  %-4s %-12s %s prefix=%v", s, e.Stage, src, e.Prefix)
		}
		for _, i := range []int{0, len(tr) - 1} {
			c := tr[i]
			line := Concepts()[c.Concept].Label + ":"
			for k, cell := range morph.Cells(c.POS) {
				line += " " + cell.Label + "=" + phon.RenderWord(v.M.Para[i].Forms[k])
			}
			t.Log(line)
			if v.Irregular(i) {
				t.Logf("  %s is irregular in the proto-language", Concepts()[c.Concept].Label)
			}
		}
	}
}
