package society

import (
	"strings"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

func TestOrthography(t *testing.T) {
	v := lex.GenerateProto(rng.New(5, rng.Key("proto"))).Variety
	water, _ := lex.ConceptByLabel("water")
	w := v.Lex.At(water).Form
	for _, ty := range []ScriptType{Alphabet, Abjad, Syllabary} {
		o := NewOrthography(ty, v, 0)
		sp := o.Spell[water]
		if sp == "" && ty != Abjad {
			t.Errorf("%s: no spelling for water", ty)
		}
		if g := o.Gap(water, v.Lex.At(water)); g != 0 {
			t.Errorf("%s: spelling gap %d‰ at adoption", ty, g)
		}
		if o.MeanGap(v) != 0 {
			t.Errorf("%s: mean gap at adoption should be 0", ty)
		}
		t.Logf("%-9s water %s ⟨%s⟩", ty, phon.RenderWord(w), sp)
	}
	ab := NewOrthography(Abjad, v, 0)
	if !w.Segs[0].IsVowel() && strings.Contains(ab.Spell[water], "ʾ") {
		t.Errorf("abjad marks a non-initial vowel: %s", ab.Spell[water])
	}
	// A changed pronunciation opens a gap; respelling closes it.
	o := NewOrthography(Alphabet, v, 0)
	l2 := *v.Lex.At(water)
	l2.Form = w.Clone()
	l2.Form.Segs = l2.Form.Segs[:len(l2.Form.Segs)-1]
	if o.Gap(water, &l2) == 0 {
		t.Error("dropping a sound leaves no spelling gap")
	}
}
