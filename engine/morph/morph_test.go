package morph

import (
	"testing"

	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

func TestAttachSplit(t *testing.T) {
	stem := phon.MustParseWord("kap")
	w := Attach(stem, phon.MustParseWord("i").Segs, PL, false, phon.BMorpheme)
	w = Attach(w, phon.MustParseWord("na").Segs, GEN, false, phon.BClitic)
	w = Attach(w, phon.MustParseWord("ta").Segs, DEF, true, phon.BClitic)
	if got := phon.RenderWord(w); got != "ta=kap+i=na" {
		t.Fatalf("got %s", got)
	}
	pre, st, suf := Split(w)
	if phon.RenderWord(st) != "kap" || phon.RenderWord(pre) != "ta" || phon.RenderWord(suf) != "i=na" {
		t.Fatalf("split %q %q %q", phon.RenderWord(pre), phon.RenderWord(st), phon.RenderWord(suf))
	}
	if j := Join(pre, st, suf); phon.RenderWord(j) != "ta=kap+i=na" {
		t.Fatalf("join %s", phon.RenderWord(j))
	}
	r, ch := Rekind(w, GEN, phon.BMorpheme)
	if !ch || phon.RenderWord(r) != "ta=kap+i+na" {
		t.Fatalf("rekind %s", phon.RenderWord(r))
	}
	// A new stem spliced into the same affixes.
	if j := Join(pre, phon.MustParseWord("mur"), suf); phon.RenderWord(j) != "ta=mur+i=na" {
		t.Fatalf("splice %s", phon.RenderWord(j))
	}
}

func TestPartner(t *testing.T) {
	if Cells(Noun)[Partner(Noun, CellIndex(Noun, PL, ACC), ACC)].Label != "PL" {
		t.Error("ACC.PL partner")
	}
	if Cells(Verb)[Partner(Verb, CellIndex(Verb, PST, P1SG), P1SG)].Label != "PST.3SG" {
		t.Error("PST.1SG partner")
	}
	if Cells(Verb)[Partner(Verb, CellIndex(Verb, P3SG), P3SG)].Label != "1SG" {
		t.Error("3SG partner")
	}
	for _, p := range []POS{Noun, Verb} {
		for i, c := range Cells(p) {
			for _, s := range c.Slots {
				if len(c.Slots) == 2 && c.Slots[0] != PST && s == P3SG {
					continue // FUT/PRF/PROG have only 3SG: no person contrast
				}
				if Partner(p, i, s) < 0 {
					t.Errorf("%s %s: no partner for %s", p, c.Label, s)
				}
			}
		}
	}
}

func TestOrderPriors(t *testing.T) {
	// OV languages should mostly be postpositional, VO mostly prepositional.
	var ovPost, ov, voPre, vo int
	for i := 0; i < 2000; i++ {
		o := SampleOrder(rng.New(uint64(i), rng.Key("order")))
		if o.OV() {
			ov++
			if o.Postpositional() {
				ovPost++
			}
		} else {
			vo++
			if !o.Postpositional() {
				voPre++
			}
		}
	}
	t.Logf("OV %d (postpositional %d), VO %d (prepositional %d)", ov, ovPost, vo, voPre)
	if ovPost*10 < ov*8 || voPre*10 < vo*7 {
		t.Error("adposition order does not follow clause order")
	}
	o := Order{Clause: 0, Adp: 0, GenN: 0, AdjN: 1, DemN: 0, RelN: 1}
	if Harmony(o, Adp, 0) <= Harmony(o, Adp, 1) {
		t.Error("SOV should prefer postpositions")
	}
}
