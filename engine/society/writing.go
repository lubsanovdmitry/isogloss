package society

import (
	"slices"
	"strings"

	"isogloss/engine/contact"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

// ScriptType is how a script writes sounds.
type ScriptType uint8

const (
	Alphabet  ScriptType = iota // consonants and vowels
	Abjad                       // consonants only (a word-initial vowel is marked ʾ)
	Syllabary                   // one sign per consonant+vowel; codas are not written
)

var scriptNames = [...]string{"alphabet", "abjad", "syllabary"}

func (t ScriptType) String() string { return scriptNames[t] }

// Orthography maps phonemes to graphemes at the moment a script is adopted
// (or reformed), and records every word's spelling then. Spellings stay
// fixed while pronunciation drifts; words that enter later are spelled by
// the same mapping as they then sound.
type Orthography struct {
	Type     ScriptType
	Since    int32             // tick adopted
	Reformed int32             // tick of the last reform (or Since)
	From     string            // polity it was borrowed from, "" if invented
	Phon     []phon.FeatureSet // mapped phonemes, sorted
	Graph    []string          // grapheme name per phoneme (romanized)
	Spell    []string          // per concept; "" for none
	Etym     []lex.EtymID      // root of the spelled word, per concept
}

// NewOrthography builds a script for variety v: every phoneme gets a
// grapheme named after its romanization, and every word is spelled.
func NewOrthography(t ScriptType, v *lex.Variety, tick int32) *Orthography {
	o := &Orthography{Type: t, Since: tick, Reformed: tick}
	o.Remap(v, tick)
	return o
}

// Remap re-derives the grapheme mapping from v's inventory and respells
// every word as it now sounds (a spelling reform).
func (o *Orthography) Remap(v *lex.Variety, tick int32) {
	o.Phon = slices.Clone(v.Inventory())
	o.Graph = make([]string, len(o.Phon))
	for i, f := range o.Phon {
		o.Graph[i] = phon.Romanize(phon.Word{Segs: []phon.Segment{{F: f}}})
	}
	n := v.Lex.Len()
	o.Spell = make([]string, n)
	o.Etym = make([]lex.EtymID, n)
	for i := 0; i < n; i++ {
		l := v.Lex.At(lex.ConceptID(i))
		if !l.Absent() {
			o.Spell[i], o.Etym[i] = o.Write(l.Form), l.Etym
		}
	}
	o.Reformed = tick
}

// Update spells words that entered v since they were last spelled (a new
// root for the concept): new words are written as they sound now.
func (o *Orthography) Update(v *lex.Variety) {
	for i := 0; i < v.Lex.Len(); i++ {
		l := v.Lex.At(lex.ConceptID(i))
		if !l.Absent() && o.Etym[i] != l.Etym {
			o.Spell[i], o.Etym[i] = o.Write(l.Form), l.Etym
		}
	}
}

// grapheme returns the grapheme for a phoneme: its own, or that of the
// nearest mapped phoneme (sounds that arose after the script was fixed).
func (o *Orthography) grapheme(f phon.FeatureSet) string {
	g := contact.Nearest(f, o.Phon)
	i, ok := slices.BinarySearchFunc(o.Phon, g, cmpFeat)
	if !ok {
		return "?"
	}
	return o.Graph[i]
}

// Write spells a word by the mapping. Syllabic signs are joined with "-".
func (o *Orthography) Write(w phon.Word) string {
	var b strings.Builder
	switch o.Type {
	case Alphabet:
		for _, s := range w.Segs {
			b.WriteString(o.grapheme(s.F))
		}
	case Abjad:
		for i, s := range w.Segs {
			switch {
			case !s.IsVowel():
				b.WriteString(o.grapheme(s.F))
			case i == 0:
				b.WriteString("ʾ")
			}
		}
	case Syllabary:
		w = w.Clone()
		phon.Resyllabify(&w, nil)
		var signs []string
		for _, sy := range w.Syll {
			v := o.grapheme(w.Segs[sy.Nucleus].F)
			if sy.Nucleus == sy.Start {
				signs = append(signs, v)
			}
			for k := sy.Start; k < sy.Nucleus; k++ {
				signs = append(signs, o.grapheme(w.Segs[k].F)+v) // extra onset consonants take the vowel too
			}
		}
		b.WriteString(strings.Join(signs, "-"))
	}
	return b.String()
}

// SpellingOf is how word l for concept c is written: its frozen spelling,
// or, for a word that arrived since the last update, as it sounds.
func (o *Orthography) SpellingOf(c lex.ConceptID, l *lex.Lexeme) string {
	if l.Absent() {
		return ""
	}
	if o.Etym[c] == l.Etym {
		return o.Spell[c]
	}
	return o.Write(l.Form)
}

// Gap is how far a word's spelling has drifted from its pronunciation, per
// mille: the edit distance between the spelling and the word written as it
// now sounds, over the longer of the two.
func (o *Orthography) Gap(c lex.ConceptID, l *lex.Lexeme) int {
	a, b := []rune(o.SpellingOf(c, l)), []rune(o.Write(l.Form))
	n := max(len(a), len(b))
	if n == 0 {
		return 0
	}
	return editDistance(a, b) * 1000 / n
}

// MeanGap is the mean spelling gap over v's core vocabulary, per mille.
func (o *Orthography) MeanGap(v *lex.Variety) int {
	sum, n := 0, 0
	for _, c := range lex.Concepts() {
		l := v.Lex.At(c.ID)
		if !c.Core() || l.Absent() {
			continue
		}
		sum += o.Gap(c.ID, l)
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func editDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := prev[j-1]
			if a[i-1] != b[j-1] {
				c++
			}
			cur[j] = min(c, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func cmpFeat(a, b phon.FeatureSet) int {
	switch {
	case a.Val < b.Val:
		return -1
	case a.Val > b.Val:
		return 1
	case a.Mask < b.Mask:
		return -1
	case a.Mask > b.Mask:
		return 1
	}
	return 0
}
