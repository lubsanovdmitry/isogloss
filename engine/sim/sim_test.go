package sim

import (
	"runtime"
	"sync"
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

func retention(_ *Sim, v *lex.Variety) int { return Retention(v) / 10 }

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

// TestSeedSweep checks calibration across seeds: after 1000 years most runs
// should have several languages and keep roughly 80% of core vocabulary.
func TestSeedSweep(t *testing.T) {
	heavy(t)
	stats := runSeeds(1000, 16, 40)
	var langs, ret []int
	for _, st := range stats {
		t.Logf("seed %d: %2d languages, %3d communities, retention %v‰, sound changes %v", st.Seed, st.Languages, st.Communities, st.Retention, st.Sounds)
		langs = append(langs, st.Languages)
		ret = append(ret, st.Retention[1])
	}
	l, r := spread3(langs), spread3(ret)
	t.Logf("languages min/median/max %v; median retention %v‰", l, r)
	if l[1] < 3 || l[1] > 14 {
		t.Errorf("median language count %d outside 3..14", l[1])
	}
	if r[1] < 650 || r[1] > 900 {
		t.Errorf("median retention %d‰ outside 650..900", r[1])
	}
}

// runSeeds runs n seeds from first for ticks each, at most GOMAXPROCS at a
// time (one at a time under wasm, whose memory is small), and summarises them.
func runSeeds(first uint64, n int, ticks int) []Stats {
	stats := make([]Stats, n)
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer func() { <-sem; wg.Done() }()
			s := New(first+uint64(i), DefaultConfig())
			s.Run(ticks)
			stats[i] = s.Summarise()
		}(i)
	}
	wg.Wait()
	return stats
}
