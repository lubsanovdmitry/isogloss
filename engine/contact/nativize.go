// Package contact holds what languages do to each other: nativizing loans,
// the borrowability of meanings, substrate languages and place names.
package contact

import (
	"slices"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

// Nativize adapts a donor word to a recipient variety:
//
//  1. each donor segment maps to the nearest recipient phoneme by feature
//     distance (vowels to vowels, consonants to consonants), unless keep is
//     set: under heavy bilingualism foreign segments survive as marginal
//     phonemes (like English /ʒ/ from French);
//  2. phonotactics are repaired (epenthesis or deletion);
//  3. stress follows the recipient's rule, and tone its most common tone.
//
// The loan is unanalysable in the recipient, so morpheme boundaries go.
func Nativize(w phon.Word, v *lex.Variety, keep bool) phon.Word {
	inv := v.Inventory()
	tone := defaultTone(v)
	out := phon.Word{Segs: make([]phon.Segment, 0, len(w.Segs))}
	for _, s := range w.Segs {
		f := s.F
		if !keep {
			f = Nearest(f, inv)
		}
		seg := phon.Segment{F: f}
		if seg.IsVowel() {
			seg.Tone = tone
			if v.Stress == phon.StressLexical {
				seg.Stress = s.Stress
			}
		}
		out.Segs = append(out.Segs, seg)
	}
	return v.Finish(out)
}

// Nearest returns the phoneme of inv closest to f (f itself if present).
// Ties go to the phoneme listed first (inventories are sorted).
func Nearest(f phon.FeatureSet, inv []phon.FeatureSet) phon.FeatureSet {
	if _, ok := slices.BinarySearchFunc(inv, f, cmpFeat); ok {
		return f
	}
	vowel := f.Has(phon.Syllabic)
	best, bestD := f, 1<<30
	for _, g := range inv {
		if g.Has(phon.Syllabic) != vowel {
			continue
		}
		if d := phon.Distance(f, g); d < bestD {
			best, bestD = g, d
		}
	}
	return best
}

// defaultTone is the recipient's most common nucleus tone (none for a
// language without tone).
func defaultTone(v *lex.Variety) phon.Tone {
	var n [phon.NumTones]int
	for i := 0; i < v.Lex.Len(); i++ {
		for _, s := range v.Lex.At(lex.ConceptID(i)).Form.Segs {
			if s.IsVowel() {
				n[s.Tone]++
			}
		}
	}
	best := phon.ToneNone
	for t := phon.ToneLow; t < phon.NumTones; t++ {
		if n[t] > n[best] {
			best = t
		}
	}
	return best
}

// Foreign counts the segments of w outside v's inventory.
func Foreign(w phon.Word, v *lex.Variety) int {
	inv := v.Inventory()
	n := 0
	for _, s := range w.Segs {
		if _, ok := slices.BinarySearchFunc(inv, s.F, cmpFeat); !ok {
			n++
		}
	}
	return n
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
