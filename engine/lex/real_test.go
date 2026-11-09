package lex

import (
	"testing"

	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

func TestRealProtosLoad(t *testing.T) {
	if err := realLoadError(); err != nil {
		t.Fatal(err)
	}
	if len(RealProtos()) == 0 {
		t.Fatal("no real proto-languages")
	}
	for _, info := range RealProtos() {
		sp := realSpecs[info.ID]
		tmpl, err := phon.ParseTemplate(sp.template)
		if err != nil {
			t.Fatalf("%s: template: %v", info.ID, err)
		}
		ph := phon.Phonotactics{Template: tmpl, SClusters: tmpl.MaxOnset >= 2}
		ph.Template.LongV = true
		for _, f := range sp.forms {
			if f.ok {
				continue
			}
			w := f.w.Clone()
			if !phon.Legal(&w, &ph) {
				t.Errorf("%s: %s %s breaks the template %s", info.ID, Concepts()[f.c].Label, phon.RenderWord(f.w), sp.template)
			}
		}
		t.Logf("%s (%s): %d forms", info.Name, info.ID, info.Forms)
	}
}

func TestRealProtoBuilds(t *testing.T) {
	for _, info := range RealProtos() {
		p := GenerateProtoWith(rng.New(1, rng.Key("proto")), ProtoOptions{Language: info.ID})
		v := p.Variety
		if p.Name != info.Name {
			t.Errorf("%s: name %q", info.ID, p.Name)
		}
		coined := 0
		for c := range Concepts() {
			if p.Coined[c] {
				coined++
			}
		}
		for _, f := range realSpecs[info.ID].forms {
			if p.Coined[f.c] || v.Lex.At(f.c).Absent() {
				t.Errorf("%s: %s is listed but missing", info.ID, Concepts()[f.c].Label)
			}
		}
		// Listed forms come through unchanged (stress aside).
		for _, f := range realSpecs[info.ID].forms {
			if f.ok {
				continue
			}
			got := v.Lex.At(f.c).Form
			if len(got.Segs) != len(f.w.Segs) {
				t.Errorf("%s: %s %s became %s", info.ID, Concepts()[f.c].Label, phon.RenderWord(f.w), phon.RenderWord(got))
			}
		}
		tr := Tracked()
		k := morph.CellIndex(morph.Verb, morph.PST, morph.P3SG)
		var verb, noun string
		for i, x := range tr {
			if x.POS == morph.Verb && verb == "" && !p.Coined[x.Concept] {
				verb = Concepts()[x.Concept].Label + " " + phon.RenderWord(v.M.Para[i].Forms[k])
			}
			if x.POS == morph.Noun && noun == "" && !p.Coined[x.Concept] {
				noun = Concepts()[x.Concept].Label + " " + phon.RenderWord(v.M.Para[i].Forms[morph.CellIndex(morph.Noun, morph.PL, morph.ACC)])
			}
		}
		t.Logf("%s: %d phonemes, %d coined concepts, PST.3SG %s, ACC.PL %s", info.Name, len(v.Inventory()), coined, verb, noun)
	}
}
