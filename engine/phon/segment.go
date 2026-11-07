package phon

// Tone is a syllable-nucleus tone. Phonation (breathy, creaky) is not stored
// separately: it is the spread/constricted glottis features of the nucleus,
// exposed through Segment.Phon, so there is one source of truth.
type Tone uint8

const (
	ToneNone Tone = iota
	ToneLow
	ToneMid
	ToneHigh
	ToneRise // low → high
	ToneFall // high → low
	NumTones
)

var toneNames = [NumTones]string{"", "L", "M", "H", "rise", "fall"}

// Tone marks rendered as combining diacritics on the vowel.
var toneMarks = [NumTones]rune{0, '̀', '̄', '́', '̌', '̂'}

func (t Tone) String() string { return toneNames[t] }

// ToneByName parses a tone name as used in rules (tone:H, tone:rise).
func ToneByName(s string) (Tone, bool) {
	for i, n := range toneNames {
		if i > 0 && n == s {
			return Tone(i), true
		}
	}
	switch s {
	case "LH":
		return ToneRise, true
	case "HL":
		return ToneFall, true
	case "none", "0":
		return ToneNone, true
	}
	return 0, false
}

// Phonation of a syllable nucleus.
type Phonation uint8

const (
	Modal Phonation = iota
	Breathy
	Creaky
)

var phonationNames = [...]string{"modal", "breathy", "creaky"}

func (p Phonation) String() string { return phonationNames[p] }

// PhonationByName parses phon:breathy etc.
func PhonationByName(s string) (Phonation, bool) {
	for i, n := range phonationNames {
		if n == s {
			return Phonation(i), true
		}
	}
	return 0, false
}

// Segment is one sound: a feature bundle plus, on syllable nuclei, tone and
// stress. Stress lives on the nucleus (not in a per-syllable array) so that
// lexical stress survives segment deletions and resyllabification.
type Segment struct {
	F      FeatureSet
	Tone   Tone
	Stress int8 // 0 none, 1 primary, 2 secondary; nuclei only
}

// IsVowel reports whether the segment is syllabic.
func (s Segment) IsVowel() bool { return s.F.Has(Syllabic) }

// Phon returns the phonation type, derived from glottal features.
func (s Segment) Phon() Phonation {
	switch {
	case s.F.Has(SpreadGlottis):
		return Breathy
	case s.F.Has(ConstrictedGlottis):
		return Creaky
	}
	return Modal
}

// WithPhon returns s with the given phonation.
func (s Segment) WithPhon(p Phonation) Segment {
	s.F = s.F.Set(SpreadGlottis, p == Breathy).Set(ConstrictedGlottis, p == Creaky)
	return s
}

// Sonority returns a sonority rank (higher = more sonorous) used by the
// syllabifier: vowels 6, glides 5, liquids 4, nasals 3, fricatives 2,
// affricates/stops 1, glottals 0 or by manner.
func (s Segment) Sonority() int {
	f := s.F
	switch {
	case f.Has(Syllabic):
		return 6
	case !f.Has(Consonantal) && f.Has(Sonorant):
		return 5
	case f.Has(Sonorant) && !f.Has(Nasal):
		return 4
	case f.Has(Nasal):
		return 3
	case f.Has(Continuant):
		return 2
	}
	return 1
}

// BoundaryKind marks a position between segments.
type BoundaryKind uint8

const (
	BMorpheme BoundaryKind = iota + 1 // '+'
	BClitic                           // '='
	BWord                             // '#' (inside a compound or phrase)
)

func (b BoundaryKind) Rune() rune {
	switch b {
	case BMorpheme:
		return '+'
	case BClitic:
		return '='
	}
	return '#'
}

// Boundary sits before segment index At.
type Boundary struct {
	At   int
	Kind BoundaryKind
}

// Span is a half-open segment range [Start, End) with its nucleus index.
type Span struct {
	Start, End, Nucleus int
}

// Word is a phonological word. Syll is derived and must be recomputed
// (Resyllabify) after every change to Segs.
type Word struct {
	Segs   []Segment
	Bounds []Boundary // sorted by At
	Syll   []Span
}

// Clone returns a deep copy.
func (w Word) Clone() Word {
	return Word{
		Segs:   append([]Segment(nil), w.Segs...),
		Bounds: append([]Boundary(nil), w.Bounds...),
		Syll:   append([]Span(nil), w.Syll...),
	}
}

// Stress returns the stress level of each syllable (from its nucleus).
func (w Word) Stress() []int8 {
	out := make([]int8, len(w.Syll))
	for k, s := range w.Syll {
		out[k] = w.Segs[s.Nucleus].Stress
	}
	return out
}

// Equal compares segments and boundaries (derived fields are ignored).
func (w Word) Equal(o Word) bool {
	if len(w.Segs) != len(o.Segs) || len(w.Bounds) != len(o.Bounds) {
		return false
	}
	for i := range w.Segs {
		if w.Segs[i] != o.Segs[i] {
			return false
		}
	}
	for i := range w.Bounds {
		if w.Bounds[i] != o.Bounds[i] {
			return false
		}
	}
	return true
}

// SyllableOf returns the index of the syllable containing segment i, or -1.
func (w Word) SyllableOf(i int) int {
	for k, s := range w.Syll {
		if i >= s.Start && i < s.End {
			return k
		}
	}
	return -1
}

// StressOf returns the stress level of the syllable containing segment i.
func (w Word) StressOf(i int) int8 {
	k := w.SyllableOf(i)
	if k < 0 {
		return 0
	}
	return w.Segs[w.Syll[k].Nucleus].Stress
}
