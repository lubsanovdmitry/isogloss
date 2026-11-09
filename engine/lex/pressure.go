package lex

import "slices"

// Pressure summarises what pushes a variety to change its words. It is
// computed once per variety (varieties are immutable once built).
type Pressure struct {
	// Clashes: homophones from different roots. Each word that sounds like
	// a stronger one (more frequent, then more stable) is listed once as
	// Loser, the candidate for replacement (Gascon gat 'cat' / 'rooster');
	// Cross marks a clash across semantic fields, which hurts less.
	Clashes []Clash
	// Drifts: a word that has gained an extra sense competes with the
	// concept's own word (Sense gets Word's word if the shift completes).
	Drifts []Drift
	// Gaps: concepts with no word, ascending.
	Gaps []ConceptID
}

// Clash is one pair of homophones.
type Clash struct {
	Loser, Winner ConceptID
	Cross         bool // different semantic fields
}

// Drift is one emerging sense.
type Drift struct{ Word, Sense ConceptID }

// Pressure returns the variety's lexical pressures (cached).
func (v *Variety) Pressure() *Pressure {
	if v.press != nil {
		return v.press
	}
	p := &Pressure{}
	cs := Concepts()
	// Words sorted by (form, field): homophones in one field are adjacent.
	type entry struct {
		h     uint64
		field uint8
		c     ConceptID
	}
	es := make([]entry, 0, v.Lex.Len())
	for i := 0; i < v.Lex.Len(); i++ {
		c := ConceptID(i)
		l := v.Lex.At(c)
		if l.Absent() {
			p.Gaps = append(p.Gaps, c)
			continue
		}
		if s, ok := l.AlsoSense(); ok {
			p.Drifts = append(p.Drifts, Drift{c, s})
		}
		es = append(es, entry{l.Hash, cs[i].FieldID, c})
	}
	slices.SortFunc(es, func(a, b entry) int {
		switch {
		case a.h != b.h:
			if a.h < b.h {
				return -1
			}
			return 1
		case a.field != b.field:
			return int(a.field) - int(b.field)
		}
		return int(a.c) - int(b.c)
	})
	// Each group of words that sound alike: the strongest word (most
	// frequent, then most stable, then lowest ID) wins; every word of
	// another root loses to it.
	for lo := 0; lo < len(es); {
		hi := lo + 1
		for hi < len(es) && es[hi].h == es[lo].h {
			hi++
		}
		if hi-lo > 1 {
			w := lo
			for k := lo + 1; k < hi; k++ {
				a, b := v.Lex.At(es[k].c), v.Lex.At(es[w].c)
				if a.Freq < b.Freq || a.Freq == b.Freq && cs[es[k].c].Stab < cs[es[w].c].Stab {
					w = k
				}
			}
			win := v.Lex.At(es[w].c)
			for k := lo; k < hi; k++ {
				if k == w || v.Lex.At(es[k].c).Etym == win.Etym {
					continue // the winner, or one word with two senses
				}
				p.Clashes = append(p.Clashes, Clash{Loser: es[k].c, Winner: es[w].c, Cross: es[k].field != es[w].field})
			}
		}
		lo = hi
	}
	slices.SortFunc(p.Clashes, func(a, b Clash) int { return int(a.Loser) - int(b.Loser) })
	v.press = p
	return p
}

// ClashWith returns the clash in which concept c's word is the loser, if
// any.
func (p *Pressure) ClashWith(c ConceptID) (Clash, bool) {
	i, ok := slices.BinarySearchFunc(p.Clashes, c, func(x Clash, c ConceptID) int { return int(x.Loser) - int(c) })
	if !ok {
		return Clash{}, false
	}
	return p.Clashes[i], true
}

// DriftTo returns a concept whose word is gaining sense c, if any.
func (p *Pressure) DriftTo(c ConceptID) (ConceptID, bool) {
	for _, d := range p.Drifts {
		if d.Sense == c {
			return d.Word, true
		}
	}
	return 0, false
}

// Senses lists the concepts that share concept c's word (same root and
// form), c included, ascending: the word's senses.
func (v *Variety) Senses(c ConceptID) []ConceptID {
	l := v.Lex.At(c)
	if l.Absent() {
		return nil
	}
	var out []ConceptID
	for i := 0; i < v.Lex.Len(); i++ {
		o := v.Lex.At(ConceptID(i))
		if o.Etym == l.Etym && o.Hash == l.Hash {
			out = append(out, ConceptID(i))
		}
	}
	return out
}

// Doublets lists concepts whose word shares c's root but not its form: the
// root reached the variety twice (an inherited word and a loan, or a word
// that split).
func (v *Variety) Doublets(c ConceptID) []ConceptID {
	l := v.Lex.At(c)
	if l.Absent() {
		return nil
	}
	var out []ConceptID
	for i := 0; i < v.Lex.Len(); i++ {
		o := v.Lex.At(ConceptID(i))
		if ConceptID(i) != c && o.Etym == l.Etym && o.Hash != l.Hash {
			out = append(out, ConceptID(i))
		}
	}
	return out
}
