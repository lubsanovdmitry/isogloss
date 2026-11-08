package sim

import (
	"strings"
	"testing"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
)

func describeGrammar(t *testing.T, s *Sim, v *lex.Variety) {
	var b strings.Builder
	for sl := morph.Slot(0); sl < morph.NumSlots; sl++ {
		e := v.M.Exp[sl]
		if e.Stage == morph.None {
			continue
		}
		src := "inherited"
		if e.Source >= 0 {
			src = lex.Concepts()[e.Source].Label
		}
		b.WriteString(" " + sl.String() + ":" + e.Stage.String() + "(" + src + ")")
	}
	ty := v.Typology()
	t.Logf("  exponents:%s", b.String())
	t.Logf("  synthesis %d fusion %d‰ irregularity %d; order %v", ty.Synthesis, ty.Fusion, ty.Irregularity, v.M.Order)
}

// TestIrregularParadigmsEmerge is the M2 gate: after 1000 years, seeded runs
// produce paradigms with stem allomorphy in frequent lexemes, made by sound
// change across morpheme boundaries and not authored.
func TestIrregularParadigmsEmerge(t *testing.T) {
	heavy(t)
	hits := 0
	const n = 8
	var irr, tot [6]int // by frequency class
	for seed := uint64(1000); seed < 1000+n; seed++ {
		s := New(seed, DefaultConfig())
		s.Run(40)
		found := false
		for _, l := range s.LivingLanguages() {
			v := s.CentreVariety(l)
			for i, tr := range lex.Tracked() {
				f := v.Lex.At(tr.Concept).Freq
				tot[f]++
				if v.Irregular(i) {
					irr[f]++
				}
			}
			for i, tr := range lex.Tracked() {
				st := v.Stems(i)
				if v.Lex.At(tr.Concept).Freq > 2 || len(st) < 2 {
					continue
				}
				if !found {
					var forms []string
					for k, c := range morph.Cells(tr.POS) {
						forms = append(forms, c.Label+" "+phon.RenderWord(v.M.Para[i].Forms[k]))
					}
					t.Logf("seed %d %s '%s': %d stems: %s", seed, l.Name, lex.Concepts()[tr.Concept].Label, len(st), strings.Join(forms, ", "))
					describeGrammar(t, s, v)
				}
				found = true
			}
		}
		if found {
			hits++
		} else {
			t.Logf("seed %d: no stem allomorphy in a frequent lexeme", seed)
			describeGrammar(t, s, s.CentreVariety(s.LivingLanguages()[0]))
		}
	}
	t.Logf("%d of %d runs have irregular frequent paradigms", hits, n)
	if hits*2 < n {
		t.Errorf("irregular paradigms emerged in only %d of %d runs", hits, n)
	}
	// Analogy levels rare words faster, so irregularity should concentrate
	// in frequent ones.
	share := func(lo, hi int) int {
		a, b := 0, 0
		for f := lo; f <= hi; f++ {
			a += irr[f]
			b += tot[f]
		}
		return a * 1000 / max(b, 1)
	}
	for f := 1; f <= 5; f++ {
		t.Logf("frequency class %d: %d of %d paradigms irregular", f, irr[f], tot[f])
	}
	if share(1, 2) <= share(3, 5) {
		t.Errorf("frequent words (%d‰ irregular) should be more irregular than rare ones (%d‰)", share(1, 2), share(3, 5))
	}
}

// Replaying a community's history from the proto-language must rebuild its
// variety exactly, paradigms and morphology included, so that etymologies of
// inflected forms are true.
func TestReplayIsExact(t *testing.T) {
	s := New(77, DefaultConfig())
	s.Run(24)
	for n, c := range s.Communities() {
		if n%25 != 0 {
			continue
		}
		v := c.Variety
		r := history.Replay(s.Reg, s.Proto.Variety, v, nil)
		if r.Key != v.Key {
			t.Fatalf("hex %d: replay key %x, variety %x", c.Hex, r.Key, v.Key)
		}
		if r.M.Exp != v.M.Exp || r.M.Order != v.M.Order {
			t.Fatalf("hex %d: morphology differs after replay", c.Hex)
		}
		for i := range v.M.Para {
			for k, f := range v.M.Para[i].Forms {
				if !f.Equal(r.M.Para[i].Forms[k]) {
					t.Fatalf("hex %d lexeme %d cell %d: replay %s, variety %s", c.Hex, i, k,
						phon.RenderWord(r.M.Para[i].Forms[k]), phon.RenderWord(f))
				}
			}
		}
		for i := 0; i < v.Lex.Len(); i++ {
			if r.Lex.At(lex.ConceptID(i)).Hash != v.Lex.At(lex.ConceptID(i)).Hash {
				t.Fatalf("hex %d concept %d differs after replay", c.Hex, i)
			}
		}
	}
	// A cell trace ends at the cell's current form.
	v := s.CentreVariety(s.LivingLanguages()[0])
	for i, tr := range lex.Tracked() {
		if i%5 != 0 {
			continue
		}
		k := len(morph.Cells(tr.POS)) - 1
		start, steps, ok := history.TraceCell(s.Reg, s.Proto.Variety, v, tr.Concept, k)
		if !ok {
			t.Fatal("no cell trace")
		}
		end := start
		if len(steps) > 0 {
			end = steps[len(steps)-1].After
		}
		if !end.Equal(v.M.Para[i].Forms[k]) {
			t.Fatalf("cell trace ends at %s, cell is %s", phon.RenderWord(end), phon.RenderWord(v.M.Para[i].Forms[k]))
		}
		if i == 0 {
			t.Logf("%s %s: *%s", lex.Concepts()[tr.Concept].Label, morph.Cells(tr.POS)[k].Label, phon.RenderWord(start))
			for _, st := range steps {
				t.Logf("  t%d %-40s %s → %s", st.Tick, st.Label, phon.RenderWord(st.Before), phon.RenderWord(st.After))
			}
		}
	}
}
