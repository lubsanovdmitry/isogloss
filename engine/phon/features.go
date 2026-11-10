// Package phon models sounds as bundles of distinctive features. IPA is only a
// rendering of a bundle.
package phon

import "strings"

// Feature is the index of one binary distinctive feature. The set and its
// order follow PHOIBLE's feature system so tables can be derived from it
// directly; prosody (stress, tone) lives outside the bundle.
type Feature uint8

const (
	Syllabic Feature = iota
	Short
	Long
	Consonantal
	Sonorant
	Continuant
	DelayedRelease
	Approximant
	Tap
	Trill
	Nasal
	Lateral
	Labial
	Round
	Labiodental
	Coronal
	Anterior
	Distributed
	Strident
	Dorsal
	High
	Low
	Front
	Back
	Tense
	RTR // retracted tongue root (also used for pharyngealisation)
	ATR
	Voice // PHOIBLE: periodicGlottalSource
	Epilaryngeal
	SpreadGlottis
	ConstrictedGlottis
	Fortis
	Ejective  // PHOIBLE: raisedLarynxEjective
	Implosive // PHOIBLE: loweredLarynxImplosive
	Click
	NumFeatures
)

// PhoibleNames maps each feature to its PHOIBLE column name.
var PhoibleNames = [NumFeatures]string{
	"syllabic", "short", "long", "consonantal", "sonorant", "continuant",
	"delayedRelease", "approximant", "tap", "trill", "nasal", "lateral",
	"labial", "round", "labiodental", "coronal", "anterior", "distributed",
	"strident", "dorsal", "high", "low", "front", "back", "tense",
	"retractedTongueRoot", "advancedTongueRoot", "periodicGlottalSource",
	"epilaryngealSource", "spreadGlottis", "constrictedGlottis", "fortis",
	"raisedLarynxEjective", "loweredLarynxImplosive", "click",
}

// shortNames are the names used in rule syntax ([+syl -cont]).
var shortNames = [NumFeatures]string{
	"syl", "short", "long", "cons", "son", "cont", "delrel", "approx", "tap",
	"trill", "nas", "lat", "lab", "round", "labdent", "cor", "ant", "dist",
	"strid", "dors", "high", "low", "front", "back", "tense", "rtr", "atr",
	"voice", "epilar", "sg", "cg", "fortis", "eject", "implo", "click",
}

var featureAliases = map[string]Feature{
	"syllabic": Syllabic, "consonantal": Consonantal, "sonorant": Sonorant,
	"continuant": Continuant, "nasal": Nasal, "lateral": Lateral,
	"labial": Labial, "coronal": Coronal, "anterior": Anterior,
	"distributed": Distributed, "strident": Strident, "dorsal": Dorsal,
	"voiced": Voice, "spread": SpreadGlottis, "constricted": ConstrictedGlottis,
	"phar": RTR, "pharyngeal": RTR, "asp": SpreadGlottis,
}

// Name returns the rule-syntax name of f.
func (f Feature) Name() string { return shortNames[f] }

// FeatureByName looks up a feature by its rule-syntax name, PHOIBLE name or alias.
func FeatureByName(s string) (Feature, bool) {
	for i, n := range shortNames {
		if n == s {
			return Feature(i), true
		}
	}
	for i, n := range PhoibleNames {
		if strings.EqualFold(n, s) {
			return Feature(i), true
		}
	}
	f, ok := featureAliases[s]
	return f, ok
}

// FeatureSet is a partial assignment of feature values: bit f of Mask says
// whether feature f is specified, and bit f of Val gives its value (+ = 1).
// Val bits outside Mask are always zero.
type FeatureSet struct {
	Val, Mask uint64
}

func bit(f Feature) uint64 { return 1 << f }

// Get returns (value, specified).
func (s FeatureSet) Get(f Feature) (plus, specified bool) {
	return s.Val&bit(f) != 0, s.Mask&bit(f) != 0
}

// Has reports whether feature f is specified and +.
func (s FeatureSet) Has(f Feature) bool { return s.Val&bit(f) != 0 }

// Set returns s with feature f specified as plus or minus.
func (s FeatureSet) Set(f Feature, plus bool) FeatureSet {
	s.Mask |= bit(f)
	if plus {
		s.Val |= bit(f)
	} else {
		s.Val &^= bit(f)
	}
	return s
}

// Unset returns s with feature f unspecified.
func (s FeatureSet) Unset(f Feature) FeatureSet {
	s.Mask &^= bit(f)
	s.Val &^= bit(f)
	return s
}

// Matches reports whether s agrees with every feature specified in want.
func (s FeatureSet) Matches(want FeatureSet) bool {
	return (s.Val^want.Val)&want.Mask == 0 && s.Mask&want.Mask == want.Mask
}

// Apply overwrites the features specified in delta.
func (s FeatureSet) Apply(delta FeatureSet) FeatureSet {
	s.Mask |= delta.Mask
	s.Val = (s.Val &^ delta.Mask) | delta.Val
	return s
}

// Distance counts features on which a and b disagree, treating a feature
// specified in one but not the other as half a difference (scaled ×2).
func Distance(a, b FeatureSet) int {
	both := a.Mask & b.Mask
	diff := popcount((a.Val ^ b.Val) & both)
	onlyOne := popcount(a.Mask ^ b.Mask)
	return 2*diff + onlyOne
}

func popcount(x uint64) int {
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}

// String renders the specified features as a matrix like [+syl -back].
func (s FeatureSet) String() string {
	var b strings.Builder
	b.WriteByte('[')
	first := true
	for f := Feature(0); f < NumFeatures; f++ {
		plus, ok := s.Get(f)
		if !ok {
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		first = false
		if plus {
			b.WriteByte('+')
		} else {
			b.WriteByte('-')
		}
		b.WriteString(f.Name())
	}
	b.WriteByte(']')
	return b.String()
}

// consonantDeps applies PHOIBLE's dependent-feature conventions to a
// consonant: features of an articulator are specified only when the
// articulator is used. A non-labial has no rounding or labiodentality; a
// labial is unrounded and bilabial unless it says otherwise, and a labialized
// dorsal or coronal (kʷ, ɣʷ, tʷ) leaves labiodentality unspecified; a non-dorsal has no tongue
// body position; a non-coronal no tip features; and a consonant that is not
// pharyngealized leaves the tongue root unspecified. So kʷ losing [+round
// +lab] is k, and losing [+dors +round] is p; tˤ losing [+rtr] is t.
func consonantDeps(f FeatureSet) FeatureSet {
	if lab, ok := f.Get(Labial); ok && !lab {
		f = f.Unset(Round).Unset(Labiodental)
	} else if lab {
		if _, ok := f.Get(Round); !ok {
			f = f.Set(Round, false)
		}
		if f.Has(Round) && (f.Has(Dorsal) || f.Has(Coronal)) {
			f = f.Unset(Labiodental)
		} else if _, ok := f.Get(Labiodental); !ok && !f.Has(Round) {
			f = f.Set(Labiodental, false)
		}
	}
	if d, ok := f.Get(Dorsal); ok && !d {
		f = f.Unset(High).Unset(Low).Unset(Front).Unset(Back)
	}
	if c, ok := f.Get(Coronal); ok && !c {
		f = f.Unset(Anterior).Unset(Distributed).Unset(Strident)
	}
	if r, ok := f.Get(RTR); ok && !r && !f.Has(ATR) {
		f = f.Unset(RTR).Unset(ATR)
	}
	return f
}

// Canon applies the redundancy rules of the segment table (PHOIBLE's
// conventions) to a bundle a sound change produced, so that a change never
// leaves a combination no symbol can show: an unrounded vowel is not
// labial, a labial vowel is rounded, non-low vowels are tense except the
// mid central one (and a vowel takes the other tenseness when only that
// is in the table), a vowel is not both front and back (fronting wins),
// consonants specify only the features of the articulators they use
// (consonantDeps), ejectives are voiceless, implosives are voiced stops, and a placeless
// fricative (h, ɦ) is a non-consonantal glide with spread glottis (or,
// glottalized, voiceless hˀ), and a glottal stop is voiceless. Bundles in the table are returned
// unchanged.
func Canon(f FeatureSet) FeatureSet {
	if _, ok := tbl().byFeat[f]; ok {
		return f
	}
	if f.Has(Syllabic) && !f.Has(Consonantal) {
		if lab, ok := f.Get(Labial); ok {
			if r, rok := f.Get(Round); lab && rok && !r {
				f = f.Set(Labial, false)
				lab = false
			}
			if lab {
				f = f.Set(Round, true).Set(Labiodental, false)
			} else {
				f = f.Unset(Round).Unset(Labiodental)
			}
		}
		if f.Has(Front) && f.Has(Back) && !f.Has(High) {
			f = f.Set(Back, false)
		}
		if f.Has(Low) {
			f = f.Unset(Tense)
		} else if _, ok := f.Get(Tense); !ok {
			_, fok := f.Get(Front)
			central := fok && !f.Has(Front) && !f.Has(Back) && !f.Has(High)
			f = f.Set(Tense, !central)
		}
		// Tenseness is redundant once height or backness has moved: ə
		// (lax by convention) raised is ɨ, not a lax high central vowel
		// no inventory has. Keep the given value if the table knows it.
		if _, ok := tbl().byFeat[f]; !ok {
			if t, ok := f.Get(Tense); ok {
				g := f.Set(Tense, !t)
				if _, known := tbl().byFeat[g]; known {
					return g
				}
			}
		}
		return f
	}
	if f.Has(Consonantal) {
		f = consonantDeps(f)
	}
	if f.Has(Ejective) {
		f = f.Set(Voice, false)
	}
	if f.Has(Implosive) {
		if f.Has(Continuant) {
			f = f.Set(Implosive, false)
		} else {
			f = f.Set(Voice, true)
		}
	}
	if placeless := !f.Has(Labial) && !f.Has(Coronal) && !f.Has(Dorsal) && !f.Has(Sonorant); placeless {
		if f.Has(Continuant) {
			f = f.Set(Consonantal, false)
			if !f.Has(ConstrictedGlottis) {
				f = f.Set(SpreadGlottis, true)
			} else {
				f = f.Set(Voice, false) // hˀ
			}
		} else if f.Has(ConstrictedGlottis) {
			f = f.Set(Voice, false)
		}
	}
	return f
}
