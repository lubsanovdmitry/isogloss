package lex

import (
	"testing"

	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

func TestConcepts(t *testing.T) {
	cs := Concepts()
	if len(cs) < 300 {
		t.Fatalf("%d concepts", len(cs))
	}
	core := 0
	for _, c := range cs {
		if c.Core() {
			core++
		}
	}
	if core != 207 {
		t.Errorf("core = %d, want 207", core)
	}
	w, ok := ConceptByLabel("water")
	if !ok || cs[w].Stab != 1 {
		t.Errorf("water: %v %+v", ok, cs[w])
	}
	if len(Colex(w)) == 0 {
		t.Error("water should have colexifications")
	}
}

func TestProtoDeterministic(t *testing.T) {
	a := GenerateProto(rng.New(7, rng.Key("proto")))
	b := GenerateProto(rng.New(7, rng.Key("proto")))
	for i := 0; i < a.Variety.Lex.Len(); i++ {
		if a.Variety.Lex.At(ConceptID(i)).Hash != b.Variety.Lex.At(ConceptID(i)).Hash {
			t.Fatal("proto generation not deterministic")
		}
	}
}

func TestProtoShape(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		p := GenerateProto(rng.New(seed, rng.Key("proto")))
		v := p.Variety
		if len(p.Vowels) < 3 || len(p.Consonants) < 8 {
			t.Errorf("seed %d: inventory %d V %d C", seed, len(p.Vowels), len(p.Consonants))
		}
		if h := v.Homophones(); h > 6 {
			t.Errorf("seed %d: %d homophones in core", seed, h)
		}
		var short, long int
		for _, c := range Concepts() {
			w := v.Lex.At(c.ID).Form
			if !phon.Legal(&w, &v.Phonotax) {
				t.Errorf("seed %d: %s illegal under %s", seed, phon.RenderWord(w), v.Phonotax.Template)
			}
			if c.Freq == 1 {
				short += len(w.Segs)
			}
			if c.Freq == 4 {
				long += len(w.Segs)
			}
		}
		if seed == 1 {
			t.Logf("seed 1: %s stress=%s, water=%s person=%s", v.Phonotax.Template, v.Stress,
				phon.RenderWord(v.Lex.At(mustC("water")).Form), phon.RenderWord(v.Lex.At(mustC("person")).Form))
		}
	}
}

func mustC(l string) ConceptID {
	c, ok := ConceptByLabel(l)
	if !ok {
		panic(l)
	}
	return c
}
