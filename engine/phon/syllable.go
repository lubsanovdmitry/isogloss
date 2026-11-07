package phon

import (
	"errors"
	"strings"
)

// Template is a syllable template such as (C)(C)V(C).
type Template struct {
	MinOnset, MaxOnset int
	MinCoda, MaxCoda   int
	LongV              bool // long vowels allowed (VV / Vː)
}

// ParseTemplate parses a template written with C, V and optional parens.
// Repeated V (VV) allows long vowels.
func ParseTemplate(s string) (Template, error) {
	var t Template
	seenV := false
	opt := false
	for _, r := range s {
		switch r {
		case '(':
			opt = true
		case ')':
			opt = false
		case 'V':
			if seenV {
				t.LongV = true
			}
			seenV = true
		case 'C':
			if !seenV {
				t.MaxOnset++
				if !opt {
					t.MinOnset++
				}
			} else {
				t.MaxCoda++
				if !opt {
					t.MinCoda++
				}
			}
		case ' ':
		default:
			return t, errors.New("phon: bad syllable template " + s)
		}
	}
	if !seenV {
		return t, errors.New("phon: template without V: " + s)
	}
	return t, nil
}

func (t Template) String() string {
	var b strings.Builder
	for i := 0; i < t.MaxOnset; i++ {
		if i >= t.MinOnset {
			b.WriteString("(C)")
		} else {
			b.WriteString("C")
		}
	}
	b.WriteString("V")
	if t.LongV {
		b.WriteString("(V)")
	}
	for i := 0; i < t.MaxCoda; i++ {
		if i >= t.MinCoda {
			b.WriteString("(C)")
		} else {
			b.WriteString("C")
		}
	}
	return b.String()
}

// Phonotactics holds a language's syllable structure constraints.
type Phonotactics struct {
	Template Template
	// SClusters allows s+stop onsets that violate sonority (as in Latin, English).
	SClusters bool
	// Banned lists forbidden two-segment sequences within a syllable margin.
	Banned [][2]FeatureSet
}

// DefaultPhonotactics is permissive, for the standalone sound-change applier:
// up to three onset and three coda consonants, s-clusters allowed.
var DefaultPhonotactics = Phonotactics{
	Template:  Template{MaxOnset: 3, MaxCoda: 3, LongV: true},
	SClusters: true,
}

func isSibilant(s Segment) bool {
	return s.F.Has(Strident) && s.F.Has(Continuant) && s.F.Has(Coronal)
}

// validOnset reports whether segs (consonants) form a legal onset.
func (p *Phonotactics) validOnset(segs []Segment) bool {
	if len(segs) > p.Template.MaxOnset {
		return false
	}
	for i := 1; i < len(segs); i++ {
		a, b := segs[i-1], segs[i]
		if a.Sonority() >= b.Sonority() {
			if !(i == 1 && p.SClusters && isSibilant(a) && b.Sonority() <= 2) {
				return false
			}
		}
		if p.banned(a, b) {
			return false
		}
	}
	return true
}

func (p *Phonotactics) banned(a, b Segment) bool {
	for _, pair := range p.Banned {
		if a.F.Matches(pair[0]) && b.F.Matches(pair[1]) {
			return true
		}
	}
	return false
}

// Resyllabify recomputes w.Syll with onset maximisation under p (nil means
// DefaultPhonotactics). Syllables never straddle a '#' boundary. A word with
// no syllabic segment gets one syllable centred on its most sonorous segment.
func Resyllabify(w *Word, p *Phonotactics) {
	if p == nil {
		p = &DefaultPhonotactics
	}
	w.Syll = w.Syll[:0]
	n := len(w.Segs)
	if n == 0 {
		return
	}
	var nuclei []int
	for i, s := range w.Segs {
		if s.IsVowel() {
			// A long vowel written as two identical vowels stays one nucleus
			// only if marked long; adjacent vowels are separate nuclei.
			nuclei = append(nuclei, i)
		}
	}
	if len(nuclei) == 0 {
		best := 0
		for i, s := range w.Segs {
			if s.Sonority() > w.Segs[best].Sonority() {
				best = i
			}
		}
		w.Syll = append(w.Syll, Span{0, n, best})
		return
	}
	start := 0
	for k, nu := range nuclei {
		end := n
		if k+1 < len(nuclei) {
			next := nuclei[k+1]
			// Consonants strictly between this nucleus and the next.
			lo, hi := nu+1, next
			split := hi // index where the next syllable starts
			if wb := w.wordBoundaryIn(lo, hi); wb >= 0 {
				split = wb
			} else {
				for s := lo; s <= hi; s++ {
					if p.validOnset(w.Segs[s:hi]) {
						split = s
						break
					}
				}
			}
			end = split
		}
		w.Syll = append(w.Syll, Span{start, end, nu})
		start = end
	}
}

// wordBoundaryIn returns the position of a '#' boundary in (lo, hi], or -1.
func (w *Word) wordBoundaryIn(lo, hi int) int {
	for _, b := range w.Bounds {
		if b.Kind == BWord && b.At >= lo && b.At <= hi {
			return b.At
		}
	}
	return -1
}

// Heavy reports whether syllable k is heavy (long vowel or a coda).
func (w Word) Heavy(k int) bool {
	s := w.Syll[k]
	return w.Segs[s.Nucleus].F.Has(Long) || s.End-1 > s.Nucleus
}

// StressRule assigns primary stress.
type StressRule uint8

const (
	StressLexical StressRule = iota // keep marked stress
	StressInitial
	StressPenult
	StressFinal
	StressLatin // weight-sensitive: penult if heavy, else antepenult
	NumStressRules
)

var stressNames = [NumStressRules]string{"lexical", "initial", "penult", "final", "weight"}

func (r StressRule) String() string { return stressNames[r] }

// StressRuleByName parses a stress rule name.
func StressRuleByName(s string) (StressRule, bool) {
	for i, n := range stressNames {
		if n == s {
			return StressRule(i), true
		}
	}
	return 0, false
}

// AssignStress sets primary stress per rule. Syllables must be current.
// Lexical stress keeps existing marks but guarantees exactly one primary,
// defaulting to the first syllable.
func AssignStress(w *Word, r StressRule) {
	ns := len(w.Syll)
	if ns == 0 {
		return
	}
	target := -1
	switch r {
	case StressLexical:
		for k, s := range w.Syll {
			if w.Segs[s.Nucleus].Stress == 1 {
				if target < 0 {
					target = k
				} else {
					w.Segs[s.Nucleus].Stress = 2
				}
			}
		}
		if target < 0 {
			target = 0
		}
	case StressInitial:
		target = 0
	case StressFinal:
		target = ns - 1
	case StressPenult:
		target = max(ns-2, 0)
	case StressLatin:
		switch {
		case ns < 3:
			target = max(ns-2, 0)
		case w.Heavy(ns - 2):
			target = ns - 2
		default:
			target = ns - 3
		}
	}
	for k, s := range w.Syll {
		st := &w.Segs[s.Nucleus].Stress
		if k == target {
			*st = 1
		} else if r != StressLexical || *st == 1 {
			*st = 0
		}
	}
	// Stress lives only on nuclei.
	for i := range w.Segs {
		if w.SyllableOf(i) >= 0 && w.Syll[w.SyllableOf(i)].Nucleus != i {
			w.Segs[i].Stress = 0
		}
	}
}

// CleanStress resyllabifies w and moves any stress left on a non-nucleus
// (after a vowel became a consonant, say) off it, so stress lives only on
// nuclei. It never adds stress.
func CleanStress(w *Word, p *Phonotactics) {
	Resyllabify(w, p)
	for i := range w.Segs {
		k := w.SyllableOf(i)
		if k >= 0 && w.Syll[k].Nucleus != i {
			w.Segs[i].Stress = 0
		}
	}
}
