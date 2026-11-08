package history

import (
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
	"isogloss/engine/tmpl"
)

// Applying a sequence of innovations and replaying the trace for every word
// must give identical forms.
func TestTraceReplaysApply(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		proto := lex.GenerateProto(rng.New(seed, rng.Key("proto"))).Variety
		reg := &Registry{}
		ap := NewApplier(reg)
		r := rng.New(seed, rng.Key("test"))
		v := proto
		lib := tmpl.Library()
		applied := 0
		for k := 0; k < 60; k++ {
			tp := lib[r.IntN(len(lib))]
			if k%9 == 4 { // make sure stress-shifting templates are exercised
				for _, x := range lib {
					if x.Stress >= 0 && r.Permille(500) {
						tp = x
					}
				}
			}
			c, ok := tp.Instantiate(r, v.Inventory())
			if !ok {
				continue
			}
			in := reg.Add(&Innovation{Kind: Sound, Template: tp.Name, Src: c.Src, Rule: c.Rule, Stress: tp.Stress})
			e := lex.Entry{Kind: lex.EntryInnov, Innov: in.ID, Tick: int32(k), Lo: 1, Hi: 5}
			if tp.Diffuse != "" {
				e.Hi = 2
			}
			v = ap.Apply(v, e)
			applied++
			add := func(in *Innovation) {
				reg.Add(in)
				v = ap.Apply(v, lex.Entry{Kind: lex.EntryInnov, Innov: in.ID, Tick: int32(k), Lo: 1, Hi: 5})
			}
			cid := lex.ConceptID(r.IntN(lex.NumConcepts()))
			other := lex.ConceptID((int(cid) + 5) % lex.NumConcepts())
			switch k % 7 {
			case 3: // a new word
				lc := &LexChange{Concept: cid, Form: v.Lex.At(other).Form, Freq: v.Lex.At(cid).Freq, Etym: 99999, How: "test", From: -1}
				add(&Innovation{Kind: Lexical, Lex: lc, Stress: -1})
			case 5: // a word gains a sense, then the shift completes
				add(&Innovation{Kind: Semantic, Sem: &SemChange{Word: other, Sense: cid, Etym: v.Lex.At(other).Etym}, Stress: -1})
				lc := &LexChange{Concept: cid, Form: v.Lex.At(other).Form, Freq: v.Lex.At(cid).Freq, Etym: v.Lex.At(other).Etym, How: "shift", From: int32(other)}
				add(&Innovation{Kind: Lexical, Lex: lc, Stress: -1})
			case 6: // a loan, and a word filling a gap (only where there is one)
				lc := &LexChange{Concept: cid, Form: v.Lex.At(other).Form, Freq: v.Lex.At(cid).Freq, Etym: 88888 + lex.EtymID(k), How: "loan", From: -1,
					Stratum: lex.Stratum{Kind: lex.Borrowed, Donor: 1, Tick: int32(k)}}
				add(&Innovation{Kind: Loan, Lex: lc, Stress: -1})
				gap := &LexChange{Concept: other, Form: v.Lex.At(cid).Form, Freq: 3, Etym: 77777, How: "gap", From: -1, Gap: true}
				add(&Innovation{Kind: Lexical, Lex: gap, Stress: -1})
			}
		}
		for _, c := range lex.Concepts() {
			_, got := Trace(reg, proto, v, c.ID)
			want := v.Lex.At(c.ID)
			if got.Etym != want.Etym || got.Also != want.Also || got.Stratum != want.Stratum {
				t.Fatalf("seed %d concept %s: trace gives root %d also %d %v, variety has %d %d %v", seed, c.Label,
					got.Etym, got.Also, got.Stratum, want.Etym, want.Also, want.Stratum)
			}
			if !got.Form.Equal(v.Lex.At(c.ID).Form) {
				steps, _ := Trace(reg, proto, v, c.ID)
				t.Fatalf("seed %d concept %s: trace gives %s, variety has %s (%d steps)", seed, c.Label,
					phon.RenderWord(got.Form), phon.RenderWord(v.Lex.At(c.ID).Form), len(steps))
			}
		}
		w, _ := lex.ConceptByLabel("water")
		steps, _ := Trace(reg, proto, v, w)
		if seed == 1 {
			t.Logf("seed 1 'water': %s → %s in %d steps (%d changes applied)", phon.RenderWord(proto.Lex.At(w).Form), phon.RenderWord(v.Lex.At(w).Form), len(steps), applied)
			for _, s := range steps {
				t.Logf("  t%d %-28s %s → %s", s.Tick, s.Label, phon.RenderWord(s.Before), phon.RenderWord(s.After))
			}
		}
	}
}

func TestBuildTree(t *testing.T) {
	// A and B share {1,2,3}; C shares only {1} with them.
	nodes := BuildTree([]Leaf{
		{Lang: 10, Innovs: []uint32{1, 2, 3, 4}},
		{Lang: 11, Innovs: []uint32{1, 2, 3, 5}},
		{Lang: 12, Innovs: []uint32{1, 6}},
	})
	root := nodes[0]
	if len(root.Defining) != 1 || root.Defining[0] != 1 {
		t.Fatalf("root defining %v", root.Defining)
	}
	var sub *TreeNode
	for _, c := range root.Children {
		if nodes[c].Lang < 0 {
			sub = &nodes[c]
		}
	}
	if sub == nil || len(sub.Children) != 2 || len(sub.Defining) != 2 {
		t.Fatalf("expected AB subgroup with defining {2,3}: %+v", nodes)
	}
}
