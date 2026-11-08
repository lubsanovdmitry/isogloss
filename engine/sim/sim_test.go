package sim

import (
	"testing"
	"time"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

func TestSmoke(t *testing.T) {
	start := time.Now()
	s := New(42, DefaultConfig())
	s.Run(40)
	el := time.Since(start)
	comms := s.Communities()
	langs := s.LivingLanguages()
	var sounds, lexical int
	for _, in := range s.Reg.All {
		if in.Kind == history.Sound {
			sounds++
		} else {
			lexical++
		}
	}
	t.Logf("40 ticks in %v: %d communities, %d languages, %d sound + %d lexical innovations, %d live, %d varieties",
		el, len(comms), len(langs), sounds, lexical, s.LiveCount(), len(s.vars))
	water, _ := lex.ConceptByLabel("water")
	t.Logf("proto %s: water %s", s.ProtoName(), phon.RenderWord(s.Proto.Variety.Lex.At(water).Form))
	for _, l := range langs {
		v := s.CentreVariety(l)
		t.Logf("  %-14s %3d hexes  water %-10s sound changes %d  retention %d%%", l.Name, len(l.Members),
			phon.RenderWord(v.Lex.At(water).Form), soundCount(s, v), retention(s, v))
	}
	if len(comms) < 100 {
		t.Errorf("only %d communities after 40 ticks", len(comms))
	}
	if len(langs) < 2 {
		t.Errorf("only %d languages after 40 ticks", len(langs))
	}
}

func soundCount(s *Sim, v *lex.Variety) int {
	n := 0
	for _, id := range history.InnovSet(v) {
		if s.Reg.Get(id).Kind == history.Sound {
			n++
		}
	}
	return n
}

// retention: share of core concepts still expressed by the proto root.
func retention(s *Sim, v *lex.Variety) int {
	kept, n := 0, 0
	for _, c := range lex.Concepts() {
		if !c.Core() {
			continue
		}
		n++
		if v.Lex.At(c.ID).Etym == lex.ProtoRoot(c.ID) {
			kept++
		}
	}
	return kept * 100 / n
}

func TestDeterministic(t *testing.T) {
	a := New(7, DefaultConfig())
	a.Run(16)
	b := New(7, DefaultConfig())
	b.Run(16)
	if a.Digest() != b.Digest() {
		t.Fatal("same seed gave different runs")
	}
}

// Inventory counts are maintained incrementally; they must match a recount.
func TestInventoryInvariant(t *testing.T) {
	s := New(11, DefaultConfig())
	s.Run(20)
	for _, c := range s.Communities() {
		v := c.Variety
		seen := map[phon.FeatureSet]bool{}
		for i := 0; i < v.Lex.Len(); i++ {
			for _, sg := range v.Lex.At(lex.ConceptID(i)).Form.Segs {
				seen[sg.F] = true
			}
		}
		if len(seen) != len(v.Inventory()) {
			t.Fatalf("hex %d: inventory %d, recount %d", c.Hex, len(v.Inventory()), len(seen))
		}
		for _, f := range v.Inventory() {
			if !seen[f] {
				t.Fatalf("hex %d: stale inventory entry %s", c.Hex, phon.RenderFeatures(f))
			}
		}
	}
}

func BenchmarkRun40(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := New(42, DefaultConfig())
		s.Run(40)
	}
}
