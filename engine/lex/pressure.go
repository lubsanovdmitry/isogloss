package lex

import "slices"

// Pressure summarises what pushes a variety to change its words. It is
// computed once per variety (varieties are immutable once built).
type Pressure struct {
	// Clashes: homophones from different roots in the same semantic field.
	// Each pair is listed once, with the less frequent word as Loser (the
	// candidate for replacement, as with Gascon gat 'cat' / 'rooster').
	Clashes []Clash
	// Drifts: a word that has gained an extra sense competes with the
	// concept's own word (Sense gets Word's word if the shift completes).
	Drifts []Drift
	// Gaps: concepts with no word, ascending.
	Gaps []ConceptID
}

// Clash is one pair of homophones.
type Clash struct{ Loser, Winner ConceptID }

// Drift is one emerging sense.
type Drift struct{ Word, Sense ConceptID }

// Pressure returns the variety's lexical pressures (cached).
func (v *Variety) Pressure() *Pressure {
	if v.press != nil {
		return v.press
	}
	p := &Pressure{}
	type key struct {
		h     uint64
		field string
	}
	first := map[key]ConceptID{}
	cs := Concepts()
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
		k := key{l.Hash, cs[i].Field}
		o, seen := first[k]
		if !seen {
			first[k] = c
			continue
		}
		if v.Lex.At(o).Etym == l.Etym {
			continue // one word with two senses, not a clash
		}
		loser, winner := c, o
		if lo, lc := v.Lex.At(o).Freq, l.Freq; lo > lc || lo == lc && cs[o].Stab > cs[i].Stab {
			loser, winner = o, c
		}
		p.Clashes = append(p.Clashes, Clash{loser, winner})
	}
	slices.SortFunc(p.Clashes, func(a, b Clash) int { return int(a.Loser) - int(b.Loser) })
	v.press = p
	return p
}

// ClashWith returns the concept c's word clashes with, if c is the loser of
// a homophone clash.
func (p *Pressure) ClashWith(c ConceptID) (ConceptID, bool) {
	i, ok := slices.BinarySearchFunc(p.Clashes, c, func(x Clash, c ConceptID) int { return int(x.Loser) - int(c) })
	if !ok {
		return 0, false
	}
	return p.Clashes[i].Winner, true
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
