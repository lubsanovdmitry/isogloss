package lex

import (
	"isogloss/engine/morph"
	"isogloss/engine/phon"
)

// Typology summarises a variety's morphology over its tracked paradigms.
// All values are ×1000.
type Typology struct {
	// Synthesis: morphemes per word (1000 = isolating). A free periphrastic
	// marker counts as its own word.
	Synthesis int
	// Fusion: share (‰) of bound exponents fused with their host: the
	// boundary has worn away or the stem has changed beside it.
	Fusion int
	// Irregularity: distinct stem forms per paradigm (1000 = none).
	Irregularity int
}

// Typology computes the variety's typology metrics.
func (v *Variety) Typology() Typology {
	var morphemes, words, bound, fused, stems, paras int
	for i, t := range Tracked() {
		p := v.M.Para[i]
		if p == nil {
			continue
		}
		paras++
		stems += len(v.Stems(i))
		_, base, _ := morph.Split(p.Forms[0])
		for k, c := range morph.Cells(t.POS) {
			morphemes++
			words++
			_, st, _ := morph.Split(p.Forms[k])
			stemChanged := !morph.SameSegments(st, base)
			for _, s := range c.Slots {
				e := v.M.Exp[s]
				switch {
				case e.Stage.Free():
					morphemes++
					words++
				case e.Stage.Bound():
					morphemes++
					bound++
					if stemChanged || !hasTag(p.Forms[k], s) {
						fused++
					}
				}
			}
		}
	}
	var t Typology
	if words > 0 {
		t.Synthesis = morphemes * 1000 / words
	}
	if bound > 0 {
		t.Fusion = fused * 1000 / bound
	}
	if paras > 0 {
		t.Irregularity = stems * 1000 / paras
	}
	return t
}

// hasTag reports whether a form still has a boundary for slot s.
func hasTag(w phon.Word, s morph.Slot) bool {
	for _, b := range w.Bounds {
		if sl, _, ok := morph.SlotOfTag(b.Tag); ok && sl == s {
			return true
		}
	}
	return false
}
