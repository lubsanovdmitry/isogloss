package render

import (
	"strings"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
)

// Gloss segments an inflected form at its tagged boundaries and glosses it
// Leipzig-style: "kap-i" / "head-PL", with "=" for clitics. Exponents of the
// cell whose material has worn away or fused (no boundary, or nothing left
// between boundaries) are added to the stem gloss with "." (foot.PL).
func Gloss(w phon.Word, stemGloss string, cell morph.Cell, m *morph.State) (form, gloss string) {
	lo, hi := morph.StemSpan(w)
	type cut struct {
		b    phon.Boundary
		slot morph.Slot
	}
	var pre, suf []cut
	for _, b := range w.Bounds {
		if sl, isPre, ok := morph.SlotOfTag(b.Tag); ok {
			if isPre && b.At <= lo {
				pre = append(pre, cut{b, sl})
			} else if !isPre && b.At >= hi {
				suf = append(suf, cut{b, sl})
			}
		}
	}
	sep := func(k phon.BoundaryKind) string {
		if k == phon.BClitic {
			return "="
		}
		return "-"
	}
	shown := map[morph.Slot]bool{}
	var keep []phon.Boundary
	var pg []string // prefix glosses with separators, in order
	start := 0
	for _, c := range pre {
		if c.b.At > start {
			pg = append(pg, c.slot.String()+sep(c.b.Kind))
			keep = append(keep, c.b)
			shown[c.slot] = true
			start = c.b.At
		}
	}
	var sg []string
	for i, c := range suf {
		end := len(w.Segs)
		if i+1 < len(suf) {
			end = suf[i+1].b.At
		}
		if end > c.b.At {
			sg = append(sg, sep(c.b.Kind)+c.slot.String())
			keep = append(keep, c.b)
			shown[c.slot] = true
		}
	}
	stemG := stemGloss
	for _, s := range cell.Slots {
		if m.Exp[s].Stage.Bound() && !shown[s] {
			stemG += "." + s.String()
		}
	}
	shownW := phon.Word{Segs: w.Segs, Bounds: keep, Syll: w.Syll}
	form = strings.ReplaceAll(phon.RenderWord(shownW), "+", "-")
	gloss = strings.Join(pg, "") + stemG + strings.Join(sg, "")
	return form, gloss
}

func stageName(e morph.Exponent) string {
	if e.Stage.Bound() && e.Stage == morph.Affix {
		if e.Prefix {
			return "prefix"
		}
		return "suffix"
	}
	if e.Stage == morph.Clitic {
		if e.Prefix {
			return "proclitic"
		}
		return "enclitic"
	}
	return e.Stage.String()
}

// grammarJSON writes a variety's morphology: exponents, typology, order.
func grammarJSON(j *JSON, v *lex.Variety) {
	j.K("exponents").Arr()
	for s := morph.Slot(0); s < morph.NumSlots; s++ {
		e := v.M.Exp[s]
		info := s.Info()
		j.Obj().KS("slot", s.String()).KS("name", info.Name).KS("pos", info.POS.String()).
			KS("stage", e.Stage.String()).KS("kind", stageName(e)).KI("since", int64(e.Since))
		if e.Source >= 0 {
			j.KS("source", lex.Concepts()[e.Source].Label).KS("sourceForm", phon.RenderWord(v.Lex.At(lex.ConceptID(e.Source)).Form))
		}
		// The productive marker, shown on its own.
		if e.Stage.Bound() {
			cell := morph.CellIndex(info.POS, s)
			if info.Group == morph.GAgr {
				cell = morph.CellIndex(info.POS, s)
			} else if info.POS == morph.Verb {
				cell = morph.CellIndex(info.POS, s, morph.P3SG)
			}
			if cell >= 0 {
				pre, suf, ok := v.Productive(info.POS, cell)
				if ok {
					j.KS("marker", markerOf(pre, suf, s))
				}
			}
		}
		j.End()
	}
	j.EndArr()
	ty := v.Typology()
	j.K("typology").Obj().KI("synthesis", int64(ty.Synthesis)).KI("fusion", int64(ty.Fusion)).
		KI("irregularity", int64(ty.Irregularity)).KS("type", typeName(ty)).End()
	j.K("order").Arr()
	for p := morph.Param(0); p < morph.NumParams; p++ {
		j.Obj().KS("param", p.Name()).KS("value", v.M.Order.ValueName(p)).End()
	}
	j.EndArr()
}

// typeName gives the traditional morphological type from the metrics.
func typeName(t lex.Typology) string {
	switch {
	case t.Synthesis < 1300:
		return "isolating"
	case t.Fusion >= 450:
		return "fusional"
	default:
		return "agglutinative"
	}
}

// ParadigmJSON shows tracked lexeme c's paradigm in a language's centre
// variety, with irregular cells flagged, plus the list of tracked lexemes.
func ParadigmJSON(s *sim.Sim, lang int32, c lex.ConceptID) []byte {
	var j JSON
	if lang < 0 || int(lang) >= len(s.Langs) {
		return j.Null().Bytes()
	}
	v := s.ByHex[s.Langs[lang].Centre].Variety
	j.Obj().KS("lang", s.Langs[lang].Name).K("lexemes").Arr()
	for i, t := range lex.Tracked() {
		j.Obj().KS("concept", lex.Concepts()[t.Concept].Label).KS("pos", t.POS.String()).
			KI("freq", int64(v.Lex.At(t.Concept).Freq)).KB("irregular", v.Irregular(i)).End()
	}
	j.EndArr()
	i := lex.TrackedIndex(c)
	if i < 0 || v.M.Para[i] == nil {
		return j.End().Bytes()
	}
	t := lex.Tracked()[i]
	irr := v.IrregularCells(i)
	reg := v.RegularForms(t.POS, v.BaseStem(i))
	j.KS("concept", lex.Concepts()[c].Label).KS("pos", t.POS.String()).KI("stems", int64(len(v.Stems(i)))).K("cells").Arr()
	for k, cell := range morph.Cells(t.POS) {
		f := v.M.Para[i].Forms[k]
		form, gloss := Gloss(f, lex.Concepts()[c].Label, cell, &v.M)
		_, st, _ := morph.Split(f)
		j.Obj().KI("cell", int64(k)).KS("label", cell.Label).KS("form", form).KS("gloss", gloss).
			KS("stem", phon.RenderWord(st)).KB("irregular", irr[k]).KS("regular", strings.ReplaceAll(phon.RenderWord(reg[k]), "+", "-")).End()
	}
	j.EndArr().End()
	return j.Bytes()
}

// CellEtymJSON traces one paradigm cell of a tracked lexeme in a community.
func CellEtymJSON(s *sim.Sim, hex int32, c lex.ConceptID, cell int) []byte {
	var j JSON
	if hex < 0 || int(hex) >= len(s.ByHex) || s.ByHex[hex] == nil {
		return j.Obj().KS("error", "hex is not settled").End().Bytes()
	}
	i := lex.TrackedIndex(c)
	if i < 0 {
		return j.Obj().KS("error", "this word has no stored paradigm").End().Bytes()
	}
	t := lex.Tracked()[i]
	cells := morph.Cells(t.POS)
	if cell < 0 || cell >= len(cells) {
		return j.Obj().KS("error", "no such form").End().Bytes()
	}
	com := s.ByHex[hex]
	proto := s.ProtoOf(com.Variety)
	start, steps, _ := history.TraceCell(s.Reg, proto, com.Variety, c, cell)
	cur := com.Variety.M.Para[i].Forms[cell]
	form, gloss := Gloss(cur, lex.Concepts()[c].Label, cells[cell], &com.Variety.M)
	pform, pgloss := Gloss(start, lex.Concepts()[c].Label, cells[cell], &proto.M)
	j.Obj().KS("concept", lex.Concepts()[c].Label).KS("cell", cells[cell].Label).KS("form", form).KS("gloss", gloss).
		KS("proto", pform).KS("protoGloss", pgloss).KS("protoLang", s.FamilyName(com.Variety.Family))
	if com.Lang >= 0 {
		j.KS("lang", s.Langs[com.Lang].Name)
	}
	j.K("steps").Arr()
	for _, st := range steps {
		j.Obj().KI("tick", int64(st.Tick)).KU("innov", uint64(st.Innov)).KS("label", st.Label).
			KS("before", strings.ReplaceAll(phon.RenderWord(st.Before), "+", "-")).
			KS("after", strings.ReplaceAll(phon.RenderWord(st.After), "+", "-")).End()
	}
	j.EndArr()
	// The base form for comparison ("cf. sg kap").
	base := morph.BaseCell(t.POS)
	if base != cell {
		j.KS("baseLabel", cells[base].Label).KS("base", strings.ReplaceAll(phon.RenderWord(com.Variety.M.Para[i].Forms[base]), "+", "-"))
	}
	j.KI("freq", int64(com.Variety.Lex.At(c).Freq)).KB("irregular", com.Variety.IrregularCells(i)[cell])
	j.End()
	return j.Bytes()
}

// SketchJSON is LangJSON plus the language's morphology, word order and the
// most irregular frequent paradigm of each part of speech.
func SketchJSON(s *sim.Sim, id int32) []byte {
	base := LangJSON(s, id)
	if len(base) < 2 || base[0] != '{' {
		return base
	}
	v := s.ByHex[s.Langs[id].Centre].Variety
	var j JSON
	j.Obj()
	grammarJSON(&j, v)
	// Example paradigms: the frequent lexeme of each part of speech with the
	// most stem forms.
	j.K("examples").Arr()
	for _, pos := range []morph.POS{morph.Noun, morph.Verb} {
		best, bestN := -1, 0
		for i, t := range lex.Tracked() {
			if t.POS != pos {
				continue
			}
			n := len(v.Stems(i))*10 + 5 - int(v.Lex.At(t.Concept).Freq)
			if best < 0 || n > bestN {
				best, bestN = i, n
			}
		}
		if best >= 0 {
			j.S(lex.Concepts()[lex.Tracked()[best].Concept].Label)
		}
	}
	j.EndArr()
	j.End()
	extra := j.Bytes()
	// Splice the two objects: base without its closing brace + extra without its opening one.
	return append(append(base[:len(base)-1:len(base)-1], ','), extra[1:]...)
}

// markerOf writes slot s's material from a productive affix pattern as
// "-ka", "=ka", "ka-" or "ka=", or "∅" when it has worn away.
func markerOf(pre, suf phon.Word, s morph.Slot) string {
	for _, part := range []struct {
		w      phon.Word
		prefix bool
	}{{pre, true}, {suf, false}} {
		x := onlySlot(part.w, s)
		if len(x.Bounds) == 0 {
			continue
		}
		sep := "-"
		if x.Bounds[0].Kind == phon.BClitic {
			sep = "="
		}
		mat := phon.RenderWord(phon.Word{Segs: x.Segs})
		if mat == "" {
			return "∅"
		}
		if part.prefix {
			return mat + sep
		}
		return sep + mat
	}
	return "∅"
}
