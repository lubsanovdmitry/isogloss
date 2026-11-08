package sim

import (
	"testing"

	"isogloss/engine/history"
	"isogloss/engine/lex"
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
	if split < n/2 {
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
