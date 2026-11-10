package sim

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

// TestLoanStrataAndDoublets is the M3 gate: the comparative table shows
// loan strata (words from other languages and from substrates, each layer
// with its own donor) and doublets (a root present twice in one language:
// natively and as a loan). Runs where the family split late have had no
// time to borrow from sisters, so the doublet check covers runs that split
// by year 800 (tick 32); families stay one language for 700 years or more.
func TestLoanStrataAndDoublets(t *testing.T) {
	heavy(t)
	const n = 12
	stats := runSeeds(1, n, 40)
	split, withDbl, withLayers := 0, 0, 0
	for _, st := range stats {
		t.Logf("seed %2d: %d languages (first split t%d), loans %d, substrate words %d, strata %d, loan doublets %d",
			st.Seed, st.Languages, st.FirstSplit, st.Loans, st.SubstrateWords, st.Strata, st.LoanDoublets)
		if st.SubstrateWords == 0 {
			t.Errorf("seed %d: no substrate words in the table", st.Seed)
		}
		if st.FirstSplit < 0 || st.FirstSplit > 32 {
			continue
		}
		split++
		if st.LoanDoublets > 0 {
			withDbl++
		}
		if st.Loans > 0 && st.Strata >= 3 {
			withLayers++
		}
	}
	t.Logf("%d runs split by t32: %d with loan doublets, %d with ≥3 strata", split, withDbl, withLayers)
	// Splits come every ClusterEvery ticks, so one pass either way moves a
	// run across t32: these 12 seeds gave 7 splits before the rare-sound
	// changes of 2026-11-10 and 5 after, while seeds 100–115 gave 10 of 16
	// both before and after. The gate only makes sure the checks below
	// are not vacuous.
	if split*12 < n*5 {
		t.Fatalf("only %d of %d runs split by t32", split, n)
	}
	if withDbl*4 < split*3 {
		t.Errorf("loan doublets in %d of %d split runs, want ≥ 3/4", withDbl, split)
	}
	if withLayers*10 < split*9 {
		t.Errorf("loan strata in %d of %d split runs, want ≥ 9/10", withLayers, split)
	}
}

// A loan only undergoes the changes its variety adopted after it arrived:
// its trace starts with the borrowing, and every later step is later.
func TestLoansOnlyTakeLaterChanges(t *testing.T) {
	s := New(7, DefaultConfig())
	s.Run(40)
	checked := 0
	for _, l := range s.LivingLanguages() {
		v := s.CentreVariety(l)
		for i := 0; i < v.Lex.Len() && checked < 40; i++ {
			c := lex.ConceptID(i)
			x := v.Lex.At(c)
			if !x.Stratum.Loan() {
				continue
			}
			steps, got := history.Trace(s.Reg, s.Proto.Variety, v, c)
			if !got.Form.Equal(x.Form) || got.Stratum != x.Stratum {
				t.Fatalf("%s '%s': trace does not rebuild the loan", l.Name, lex.Concepts()[c].Label)
			}
			arrived := -1
			for k, st := range steps {
				if st.Innov != 0 && s.Reg.Get(st.Innov).Kind == history.Loan || st.Innov != 0 && s.Reg.Get(st.Innov).Lex != nil {
					arrived = k
				}
			}
			if arrived < 0 {
				t.Fatalf("%s '%s': no borrowing step in the trace", l.Name, lex.Concepts()[c].Label)
			}
			for _, st := range steps[arrived+1:] {
				if st.Tick < x.Stratum.Tick {
					t.Errorf("%s '%s': change at t%d before the loan arrived at t%d", l.Name, lex.Concepts()[c].Label, st.Tick, x.Stratum.Tick)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no loans to check")
	}
}

// TestPolysemyInRange: words shared between concept slots (colexification)
// stay near what real languages show (roughly 10-25% of a 300-concept list)
// and do not keep accumulating: meaning shifts create them, replacements of
// either sense remove them. Measured at 1000 years: medians 9-24%; at 3000
// years they are no higher.
func TestPolysemyInRange(t *testing.T) {
	heavy(t)
	var shares []int
	for seed := uint64(1); seed <= 6; seed++ {
		s := New(seed, DefaultConfig())
		s.Run(40)
		for _, l := range s.LivingLanguages() {
			v := s.CentreVariety(l)
			n, poly := 0, 0
			for i := 0; i < v.Lex.Len(); i++ {
				c := lex.ConceptID(i)
				if v.Lex.At(c).Absent() {
					continue
				}
				n++
				if len(v.Senses(c)) > 1 {
					poly++
				}
			}
			shares = append(shares, poly*1000/n)
		}
	}
	slices.Sort(shares)
	if m := shares[len(shares)/2]; m < 80 || m > 250 {
		t.Errorf("median polysemy %d‰ (all: %v)", m, shares)
	}
}

// TestCalques: compounds borrowed between languages are sometimes
// translated part by part, and the calque is built from the borrower's own
// words for the parts.
func TestCalques(t *testing.T) {
	heavy(t)
	n := 0
	for seed := uint64(1); seed <= 8 && n == 0; seed++ {
		s := New(seed, DefaultConfig())
		s.Run(40)
		for _, in := range s.Reg.All {
			if in.Lex == nil || !strings.HasPrefix(strings.TrimPrefix(in.Lex.How, "a new thing, "), "calque on ") {
				continue
			}
			n++
			rt, ok := s.root(in.Lex.Etym)
			if !ok || !rt.Compound {
				t.Fatalf("calque %q has no compound root", in.Lex.How)
			}
			c := s.ByHex[in.Origin]
			if c == nil {
				continue
			}
			if b := in.Lex.Form.Bounds; len(b) != 1 || b[0].At == 0 {
				t.Errorf("calque %q is not a two-part compound: %v", in.Lex.How, in.Lex.Form)
			}
		}
		t.Logf("seed %d: %d calques so far", seed, n)
	}
	if n == 0 {
		t.Error("no calques in 8 runs")
	}
}

// TestSubstrateFeatures: a substrate can pass on a sound the family lacked,
// which then stands in the inventory beside the sound it came from.
func TestSubstrateFeatures(t *testing.T) {
	heavy(t)
	n, kept := 0, 0
	for seed := uint64(1); seed <= 6; seed++ {
		s := New(seed, DefaultConfig())
		s.Run(40)
		for _, in := range s.Reg.All {
			if in.Family != "substrate" || !strings.HasPrefix(in.Template, "substrate ") || strings.HasPrefix(in.Template, "substrate merger") {
				continue
			}
			n++
			c := s.ByHex[in.Origin]
			if c == nil {
				continue
			}
			name := strings.TrimPrefix(in.Template, "substrate ")
			name = name[:strings.Index(name, " (")]
			for _, f := range c.Variety.Inventory() {
				if phon.RenderFeatures(f) == name {
					kept++
					break
				}
			}
		}
	}
	t.Logf("%d substrate sounds introduced, %d still in their origin's inventory", n, kept)
	if n == 0 || kept == 0 {
		t.Error("no substrate sound took hold")
	}
}

// coreShares measures the reference varieties' core words: the share
// borrowed from another language and the share sounding exactly like a
// word of another root.
func coreShares(s *Sim) (loans, homo, n int) {
	for _, l := range s.LivingLanguages() {
		v := s.CentreVariety(l)
		forms := map[uint64][]lex.EtymID{}
		for i := 0; i < v.Lex.Len(); i++ {
			x := v.Lex.At(lex.ConceptID(i))
			if !lex.Concepts()[i].Core() || x.Absent() {
				continue
			}
			n++
			if x.Stratum.Kind == lex.Borrowed {
				loans++
			}
			forms[x.Hash] = append(forms[x.Hash], x.Etym)
		}
		for _, es := range forms { //lint:ordered counting
			for _, e := range es {
				if e != es[0] {
					homo += len(es)
					break
				}
			}
		}
	}
	return
}

// TestCoreVocabularyStaysDistinct: basic vocabulary resists borrowing and
// homophony. Measured 2026-11-09 (six seeds): at 2000 years 13% of core
// words are loans (was 34%) and 20% are in homophone sets (was 28%). On 3
// seeds the homophone share swung 20–30% between versions that 16 seeds
// rate alike (23–24%), so the gate pools 8. Changes aimed at rare sounds
// (2026-11-10) merge more of them into common ones (q > k, ɖ > d): over
// seeds 1–32 homophony went from 23% to 24%, and seeds 1–8, which run high,
// from 24% to 25–26%; the homophone limit went from 26% to 27%.
func TestCoreVocabularyStaysDistinct(t *testing.T) {
	heavy(t)
	var mu sync.Mutex
	var loans, homo, n int
	eachSeed(1, 8, func(seed uint64) {
		s := New(seed, DefaultConfig())
		s.Run(80)
		l, h, k := coreShares(s)
		mu.Lock()
		loans, homo, n = loans+l, homo+h, n+k
		mu.Unlock()
	})
	t.Logf("2000 years: core loans %d%%, homophones %d%%", loans*100/n, homo*100/n)
	if loans*100 > n*22 || homo*100 > n*27 {
		t.Errorf("core loans %d%%, homophones %d%%", loans*100/n, homo*100/n)
	}
}
