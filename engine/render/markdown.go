package render

import (
	"strconv"
	"strings"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
)

// GrammarMarkdown writes an auto-generated grammar sketch of a living
// language: inventory, phonotactics, stress, morphological type, exponents,
// word order, example paradigms and the parallel text.
func GrammarMarkdown(s *sim.Sim, id int32) string {
	if id < 0 || int(id) >= len(s.Langs) {
		return ""
	}
	l := s.Langs[id]
	v := s.ByHex[l.Centre].Variety
	var b strings.Builder
	w := func(xs ...string) {
		for _, x := range xs {
			b.WriteString(x)
		}
	}
	pm := func(x int) string { return strconv.Itoa(x/1000) + "." + strconv.Itoa(x%1000/10+100)[1:] }
	ty := v.Typology()
	w("# ", l.Name, "\n\n")
	w("A descendant of ", s.ProtoName(), ", spoken in ", strconv.Itoa(len(l.Members)), " communities")
	if l.Born > 0 {
		w(", a separate language since year ", strconv.Itoa(int(l.Born)*25))
	}
	w(". Morphologically ", typeName(ty), "; basic order ", v.M.Order.ValueName(morph.Clause), ".\n\n")

	w("## Sounds\n\n")
	var vs, cs []string
	for _, f := range v.Inventory() {
		if f.Has(phon.Syllabic) {
			vs = append(vs, phon.RenderFeatures(f))
		} else {
			cs = append(cs, phon.RenderFeatures(f))
		}
	}
	w("- **Consonants** (", strconv.Itoa(len(cs)), "): ", strings.Join(cs, " "), "\n")
	w("- **Vowels** (", strconv.Itoa(len(vs)), "): ", strings.Join(vs, " "), "\n")
	w("- **Syllables**: ", v.Phonotax.Template.String(), "; illegal clusters are repaired by ", v.Repair.String(), "\n")
	w("- **Stress**: ", v.Stress.String(), "\n\n")

	w("## Morphology\n\n")
	w("Synthesis ", pm(ty.Synthesis), " morphemes per word; ", strconv.Itoa(ty.Fusion/10), "% of affixes fused with the stem; ",
		pm(ty.Irregularity), " stem forms per paradigm.\n\n")
	w("| Category | Expressed by | Marker | Source |\n|---|---|---|---|\n")
	for sl := morph.Slot(0); sl < morph.NumSlots; sl++ {
		e := v.M.Exp[sl]
		if e.Stage == morph.None {
			continue
		}
		info := sl.Info()
		marker, src := "", "inherited"
		if e.Source >= 0 {
			src = "'" + lex.Concepts()[e.Source].Label + "'"
		}
		switch {
		case e.Stage.Bound():
			cell := morph.CellIndex(info.POS, sl)
			if info.POS == morph.Verb && info.Group != morph.GAgr {
				cell = morph.CellIndex(info.POS, sl, morph.P3SG)
			}
			if pre, suf, ok := v.Productive(info.POS, cell); ok {
				marker = markerOf(pre, suf, sl)
			}
		case e.Stage.Free():
			marker = phon.RenderWord(v.Lex.At(lex.ConceptID(e.Source)).Form)
		}
		w("| ", info.Name, " (", sl.String(), ") | ", stageName(e), " | ", marker, " | ", src, " |\n")
	}
	w("\n## Word order\n\n")
	for p := morph.Param(0); p < morph.NumParams; p++ {
		w("- ", p.Name(), ": ", v.M.Order.ValueName(p), "\n")
	}
	w("\n## Paradigms\n\n")
	for _, pos := range []morph.POS{morph.Noun, morph.Verb} {
		best, bestN := -1, -1
		for i, t := range lex.Tracked() {
			if t.POS != pos {
				continue
			}
			n := len(v.Stems(i))*10 + 5 - int(v.Lex.At(t.Concept).Freq)
			if n > bestN {
				best, bestN = i, n
			}
		}
		if best < 0 {
			continue
		}
		t := lex.Tracked()[best]
		label := lex.Concepts()[t.Concept].Label
		irr := v.IrregularCells(best)
		w("**", label, "** (", pos.String(), ")\n\n| Cell | Form | Gloss |\n|---|---|---|\n")
		for k, c := range morph.Cells(pos) {
			f, g := Gloss(v.M.Para[best].Forms[k], label, c, &v.M)
			mark := ""
			if irr[k] {
				mark = " ●"
			}
			w("| ", c.Label, " | ", f, mark, " | ", g, " |\n")
		}
		w("\n")
	}
	w("● irregular: not what the regular pattern gives.\n\n")
	w("## Parallel text\n\n")
	for _, c := range Fable {
		var fs, gs []string
		for _, x := range renderClause(v, c) {
			fs = append(fs, x.Form)
			gs = append(gs, x.Gloss)
		}
		w("    ", strings.Join(fs, " "), "\n    ", strings.Join(gs, " "), "\n    '", c.English, "'\n\n")
	}
	w("## Changes\n\n")
	for _, e := range v.Adopted() {
		if e.Kind != lex.EntryInnov || !(e.Lo == 1 && e.Hi == 5 || e.Lo == e.Hi && (e.Lo == 1 || e.Lo == 5)) {
			continue
		}
		in := s.Reg.Get(e.Innov)
		if !structural(in) || in.Kind == history.Gram && !gramTook(v, in) {
			continue
		}
		w("- year ", strconv.Itoa(int(e.Tick)*25), ": ", in.Label(), "\n")
	}
	return b.String()
}
