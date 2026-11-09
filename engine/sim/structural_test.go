package sim

import (
	"testing"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
)

// TestStructuralInterventions: a forced construction, a push, a word-order
// value and a narrower syllable template spread to every community when
// applied everywhere, and replay and word traces stay exact.
func TestStructuralInterventions(t *testing.T) {
	s := New(42, DefaultConfig())
	s.Run(4)
	home := s.ByHex[s.homeland()].Variety
	slot := morph.NumSlots
	for sl := morph.Slot(0); sl < morph.NumSlots; sl++ {
		if e := home.M.Exp[sl]; e.Stage == morph.None && len(sl.Info().Sources) > 0 {
			slot = sl
			break
		}
	}
	if slot == morph.NumSlots {
		t.Skip("every slot is already marked in this run")
	}
	src := slot.Info().Sources[0]
	sched := func(kind, rule string) {
		t.Helper()
		if err := s.Intervene(Intervention{Kind: kind, Hex: -1, Rule: rule}); err != nil {
			t.Fatal(err)
		}
	}
	sched("gram", slot.String()+" "+src)
	sched("order", "clause order=VSO")
	sched("syllables", "coda-1")
	s.Step()
	sched("gram", slot.String()+" +")
	sched("syllables", "coda-1")
	sched("syllables", "coda-1")
	s.Step()
	srcID, _ := lex.ConceptByLabel(src)
	// Nearly every community took all three; a few may have relaxed their
	// codas again (a sound change creating clusters) or been settled
	// late.
	gram, order, open, n := 0, 0, 0, len(s.Communities())
	for _, c := range s.Communities() {
		v := c.Variety
		if e := v.M.Exp[slot]; e.Stage == morph.Fixed && e.Source == int32(srcID) {
			gram++
		}
		if v.M.Order[morph.Clause] == 2 {
			order++
		}
		if v.Phonotax.Template.MaxCoda != 0 {
			continue
		}
		open++
		for i := 0; i < v.Lex.Len(); i++ {
			w := v.Lex.At(lex.ConceptID(i)).Form
			for _, sy := range w.Syll {
				if sy.End-sy.Nucleus-1 > 0 {
					t.Fatalf("hex %d: %s still has a closed syllable", c.Hex, phon.RenderWord(w))
				}
			}
		}
	}
	t.Logf("of %d communities: %d took the %s construction, %d VSO, %d open syllables only", n, gram, slot, order, open)
	if gram*10 < n*9 || order*10 < n*9 || open*10 < n*9 {
		t.Errorf("the forced changes did not take")
	}
	// Exactness: replay rebuilds the variety, traces rebuild each word.
	v := s.ByHex[s.homeland()].Variety
	if r := history.Replay(s.Reg, s.Proto.Variety, v, nil); r.Key != v.Key {
		t.Fatal("replay key differs")
	} else {
		for i := 0; i < v.Lex.Len(); i++ {
			c := lex.ConceptID(i)
			if !r.Lex.At(c).Form.Equal(v.Lex.At(c).Form) {
				t.Fatalf("replay: %s vs %s", phon.RenderWord(r.Lex.At(c).Form), phon.RenderWord(v.Lex.At(c).Form))
			}
			if _, got := history.Trace(s.Reg, s.Proto.Variety, v, c); !got.Form.Equal(v.Lex.At(c).Form) {
				t.Fatalf("trace of '%s': %s, word is %s", lex.Concepts()[c].Label, phon.RenderWord(got.Form), phon.RenderWord(v.Lex.At(c).Form))
			}
		}
	}
	// A bound slot refuses a new construction (logged, not applied).
	if err := s.Intervene(Intervention{Kind: "gram", Hex: -1, Rule: "bogus x"}); err == nil {
		t.Error("unknown slot accepted")
	}
	if err := s.Intervene(Intervention{Kind: "syllables", Hex: -1, Rule: "sideways"}); err == nil {
		t.Error("unknown syllable change accepted")
	}
}

// TestQueue: several interventions queue for the same generation, can be
// cancelled before it, and all the rest apply.
func TestQueue(t *testing.T) {
	s := New(42, DefaultConfig())
	s.Run(3)
	for _, iv := range []Intervention{
		{Kind: "rule", Hex: -1, Rule: "rule one:\n    p => f\n"},
		{Kind: "rule", Hex: -1, Rule: "rule two:\n    t => θ\n"},
		{Kind: "order", Hex: -1, Rule: "clause order=VSO"},
	} {
		if err := s.Intervene(iv); err != nil {
			t.Fatal(err)
		}
	}
	idx, ivs := s.Pending()
	if len(ivs) != 3 {
		t.Fatalf("%d queued, want 3", len(ivs))
	}
	if err := s.Cancel(idx[1]); err != nil {
		t.Fatal(err)
	}
	if _, ivs = s.Pending(); len(ivs) != 2 || ivs[1].Kind != "order" {
		t.Fatalf("after cancel: %+v", ivs)
	}
	s.Step()
	forced := map[string]bool{}
	for _, e := range s.Log {
		if e.Kind == "intervention" && e.Tick == s.Tick {
			forced[e.Text] = true
		}
	}
	if !forced["forced change one"] || forced["forced change two"] || !forced["forced: clause order becomes VSO"] {
		t.Errorf("applied: %v", forced)
	}
	if _, ivs = s.Pending(); len(ivs) != 0 {
		t.Errorf("still pending after the generation: %d", len(ivs))
	}
	if s.Cancel(0) == nil {
		t.Error("cancelled an applied intervention")
	}
}
