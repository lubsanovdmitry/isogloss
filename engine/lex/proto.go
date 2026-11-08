package lex

import (
	"slices"
	"strings"

	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

// poolSeg is a candidate phoneme for proto-inventories.
type poolSeg struct {
	F    phon.FeatureSet
	Freq int
}

// pools groups PHOIBLE symbols by feature bundle (taking the highest
// frequency among symbols for the same bundle) and splits vowels from
// consonants. Rare, contour, click and tone-bearing segments are excluded.
func pools() (vowels, cons []poolSeg) {
	best := map[phon.FeatureSet]int{}
	var order []phon.FeatureSet
	for _, s := range phon.Symbols() {
		if strings.HasPrefix(s.IPA, "*") || s.F.Has(phon.Click) {
			continue
		}
		if _, ok := best[s.F]; !ok {
			order = append(order, s.F)
		}
		if s.Freq > best[s.F] {
			best[s.F] = s.Freq
		}
	}
	for _, f := range order {
		freq := best[f]
		if freq < 150 { // fewer than 1.5% of inventories
			continue
		}
		v := f.Has(phon.Syllabic)
		switch {
		case v && f.Has(phon.Consonantal): // syllabic consonants
		case v && (f.Has(phon.Long) || f.Has(phon.Nasal)):
			// length and nasality are added as whole series below
		case v:
			vowels = append(vowels, poolSeg{f, freq})
		default:
			cons = append(cons, poolSeg{f, freq})
		}
	}
	return vowels, cons
}

func weighted(r *rng.Stream, pool []poolSeg, taken map[phon.FeatureSet]bool) int {
	w := make([]uint32, len(pool))
	for i, p := range pool {
		if !taken[p.F] {
			w[i] = uint32(p.Freq)
		}
	}
	return r.Weighted(w)
}

func pickCount(r *rng.Stream, counts []int, weights []uint32) int {
	return counts[r.Weighted(weights)]
}

func mustLookup(ipa string) phon.FeatureSet {
	f, ok := phon.Lookup(ipa)
	if !ok {
		panic("lex: missing IPA symbol " + ipa)
	}
	return f
}

// Proto describes a generated proto-language.
type Proto struct {
	Variety    *Variety
	Vowels     []phon.FeatureSet
	Consonants []phon.FeatureSet
	freqOf     map[phon.FeatureSet]int
	maxSyl     int
}

// GenerateProto builds a proto-language: an inventory sampled from PHOIBLE
// frequencies under implicational constraints, a syllable template, a stress
// rule, and one root per concept (frequent concepts get shorter roots).
func GenerateProto(r *rng.Stream) *Proto { return GenerateProtoWith(r, ProtoOptions{}) }

// ProtoOptions constrain a generated proto-language (for scenarios). Zero
// values leave the default sampling unchanged.
type ProtoOptions struct {
	Template     string // syllable template, e.g. "CV(C)"
	MaxSyllables int    // cap on syllables per root
	FinalGlottal uint32 // per-mille of roots that end in a glottal stop
	Family       uint8  // family index: roots are FamilyRoot(Family, concept)
}

// GenerateProtoWith is GenerateProto under constraints.
func GenerateProtoWith(r *rng.Stream, opt ProtoOptions) *Proto {
	vpool, cpool := pools()
	p := &Proto{freqOf: map[phon.FeatureSet]int{}}
	taken := map[phon.FeatureSet]bool{}
	add := func(f phon.FeatureSet, freq int, vowel bool) {
		if taken[f] {
			return
		}
		taken[f] = true
		p.freqOf[f] = freq
		if vowel {
			p.Vowels = append(p.Vowels, f)
		} else {
			p.Consonants = append(p.Consonants, f)
		}
	}
	freqIn := func(pool []poolSeg, f phon.FeatureSet) int {
		for _, s := range pool {
			if s.F == f {
				return s.Freq
			}
		}
		return 100
	}

	// Vowels: always the corner vowels, then by frequency.
	nV := pickCount(r, []int{3, 4, 5, 6, 7, 8, 9}, []uint32{10, 10, 35, 15, 15, 8, 7})
	for _, ipa := range []string{"a", "i", "u"} {
		f := mustLookup(ipa)
		add(f, freqIn(vpool, f), true)
	}
	for len(p.Vowels) < nV {
		i := weighted(r, vpool, taken)
		if i < 0 {
			break
		}
		add(vpool[i].F, vpool[i].Freq, true)
	}
	// Consonants by frequency, with implications: a voiced obstruent implies
	// its voiceless counterpart.
	nC := pickCount(r, []int{10, 12, 14, 16, 18, 20, 22, 25}, []uint32{5, 10, 15, 20, 20, 15, 10, 5})
	for len(p.Consonants) < nC {
		i := weighted(r, cpool, taken)
		if i < 0 {
			break
		}
		f := cpool[i].F
		add(f, cpool[i].Freq, false)
		if !f.Has(phon.Sonorant) && f.Has(phon.Voice) {
			vl := f.Set(phon.Voice, false)
			for _, s := range cpool {
				if s.F == vl {
					add(vl, s.Freq, false)
				}
			}
		}
	}
	// Optional length series.
	long := r.Permille(250)
	if long {
		for _, v := range slices.Clone(p.Vowels) {
			lf := v.Set(phon.Long, true).Set(phon.Short, false)
			add(lf, p.freqOf[v]/3, true)
		}
	}

	v := &Variety{Lex: NewLexicon(NumConcepts())}
	tmpl := []string{"CV", "CV(C)", "(C)V(C)", "(C)(C)V(C)", "(C)(C)V(C)(C)"}[r.Weighted([]uint32{10, 25, 25, 25, 15})]
	if opt.Template != "" {
		tmpl = opt.Template
	}
	v.Phonotax.Template, _ = phon.ParseTemplate(tmpl)
	v.Phonotax.Template.LongV = long
	v.Phonotax.SClusters = v.Phonotax.Template.MaxOnset >= 2
	v.Stress = []phon.StressRule{phon.StressInitial, phon.StressPenult, phon.StressFinal, phon.StressLatin, phon.StressLexical}[r.Weighted([]uint32{30, 30, 15, 15, 10})]
	if r.Permille(300) {
		v.Repair = phon.RepairDeletion
	}
	v.Epenth = phon.Segment{F: p.epenthetic()}
	p.Variety = v
	p.maxSyl = opt.MaxSyllables
	glottal := mustLookup("ʔ")
	if opt.FinalGlottal > 0 {
		add(glottal, 300, false)
		if v.Phonotax.Template.MaxCoda < 1 {
			v.Phonotax.Template.MaxCoda = 1
		}
	}
	var gr *rng.Stream
	if opt.FinalGlottal > 0 {
		gr = rng.New(r.Uint64(), rng.Key("glottal"))
	}

	seen := map[uint64]bool{}
	for _, c := range Concepts() {
		var w phon.Word
		for try := 0; try < 40; try++ {
			w = p.root(r, c.Freq)
			if gr != nil && w.Segs[len(w.Segs)-1].IsVowel() && gr.Permille(opt.FinalGlottal) { // ends in a vowel: add ʔ
				w.Segs = append(w.Segs, phon.Segment{F: glottal})
				phon.AssignStress(&w, v.Stress)
			}
			h := FormHash(w)
			if !seen[h] {
				seen[h] = true
				break
			}
		}
		v.Set(c.ID, Lexeme{Concept: c.ID, Form: w, Freq: c.Freq, Stratum: Native, Etym: FamilyRoot(opt.Family, c.ID), Hash: FormHash(w)})
	}
	p.genSenses(rng.New(r.Uint64(), rng.Key("senses")))
	p.genMorph(rng.New(r.Uint64(), rng.Key("morph")))
	v.Family = opt.Family
	v.Key = 0x1505 ^ uint64(opt.Family)<<40
	return p
}

// epenthetic picks the language's default inserted vowel: the most frequent
// short non-low vowel, preferring central or high ones.
func (p *Proto) epenthetic() phon.FeatureSet {
	best := p.Vowels[0]
	score := -1
	for _, v := range p.Vowels {
		if v.Has(phon.Long) {
			continue
		}
		s := p.freqOf[v]
		if v.Has(phon.Low) {
			s /= 4
		}
		if !v.Has(phon.Front) && !v.Has(phon.Back) {
			s *= 2 // central: ə, ɨ
		}
		if s > score {
			best, score = v, s
		}
	}
	return best
}

func (p *Proto) pick(r *rng.Stream, set []phon.FeatureSet, filter func(phon.FeatureSet) bool) (phon.FeatureSet, bool) {
	w := make([]uint32, len(set))
	for i, f := range set {
		if filter == nil || filter(f) {
			w[i] = uint32(p.freqOf[f]) + 1
		}
	}
	i := r.Weighted(w)
	if i < 0 {
		return phon.FeatureSet{}, false
	}
	return set[i], true
}

// root generates one root of a length suited to the frequency class.
func (p *Proto) root(r *rng.Stream, freq uint8) phon.Word {
	t := p.Variety.Phonotax.Template
	var nsyl int
	switch freq {
	case 1:
		nsyl = 1
	case 2:
		nsyl = 1 + r.IntN(2)
	case 3:
		nsyl = []int{1, 2, 2, 2, 3}[r.IntN(5)]
	case 4:
		nsyl = 2 + r.IntN(2)
	default:
		nsyl = []int{2, 3, 3}[r.IntN(3)]
	}
	if p.maxSyl > 0 {
		nsyl = min(nsyl, p.maxSyl)
	}
	isObstruent := func(f phon.FeatureSet) bool { return !f.Has(phon.Sonorant) }
	isLiquidGlide := func(f phon.FeatureSet) bool {
		return f.Has(phon.Sonorant) && !f.Has(phon.Nasal)
	}
	var w phon.Word
	for s := 0; s < nsyl; s++ {
		// Onset.
		needOnset := t.MinOnset > 0 || s > 0
		if needOnset || r.Permille(750) {
			if t.MaxOnset >= 2 && r.Permille(120) {
				c1, ok1 := p.pick(r, p.Consonants, isObstruent)
				c2, ok2 := p.pick(r, p.Consonants, isLiquidGlide)
				if ok1 && ok2 {
					w.Segs = append(w.Segs, phon.Segment{F: c1}, phon.Segment{F: c2})
				}
			} else if t.MaxOnset >= 1 {
				c, _ := p.pick(r, p.Consonants, nil)
				w.Segs = append(w.Segs, phon.Segment{F: c})
			}
		}
		// Nucleus.
		vf, _ := p.pick(r, p.Vowels, nil)
		w.Segs = append(w.Segs, phon.Segment{F: vf})
		// Coda: mostly word-final or before another syllable's onset.
		if t.MaxCoda >= 1 && r.Permille(280) {
			c, _ := p.pick(r, p.Consonants, func(f phon.FeatureSet) bool {
				return f.Has(phon.Sonorant) || f.Has(phon.Continuant) || s == nsyl-1
			})
			w.Segs = append(w.Segs, phon.Segment{F: c})
			if t.MaxCoda >= 2 && s == nsyl-1 && r.Permille(100) {
				c2, _ := p.pick(r, p.Consonants, isObstruent)
				if c2 != c {
					w.Segs = append(w.Segs, phon.Segment{F: c2})
				}
			}
		}
	}
	// Avoid identical adjacent consonants (no geminates in proto roots).
	out := w.Segs[:0]
	for i, s := range w.Segs {
		if i > 0 && s == w.Segs[i-1] && !s.IsVowel() {
			continue
		}
		out = append(out, s)
	}
	w.Segs = out
	phon.Repair(&w, &p.Variety.Phonotax, p.Variety.Repair, p.Variety.Epenth)
	phon.AssignStress(&w, p.Variety.Stress)
	return w
}

// NewRoot coins a fresh root for a later variety from its own inventory and
// phonotactics (used when a concept gets a brand-new word).
func NewRoot(r *rng.Stream, v *Variety, freq uint8) phon.Word {
	p := &Proto{Variety: v, freqOf: map[phon.FeatureSet]int{}}
	for _, f := range v.Inventory() {
		if f.Has(phon.Syllabic) {
			p.Vowels = append(p.Vowels, f)
		} else {
			p.Consonants = append(p.Consonants, f)
		}
		p.freqOf[f] = phon.Frequency(f) + 50
	}
	if len(p.Vowels) == 0 || len(p.Consonants) == 0 {
		return v.Lex.At(0).Form.Clone()
	}
	return p.root(r, freq)
}

// Shares of cultural items the proto-language already has words for, and of
// strong colexification edges (attested in at least polyFamilies language
// families) the proto-language already colexifies.
const (
	oldCulturePermille = 400
	polyFamilies       = 8
	polyPermille       = 80
)

// genSenses removes cultural items the proto-language does not know yet
// (they are invented later and spread as words and loans) and lets some
// pairs of concepts share one word, along strong CLICS edges: the less
// frequent concept takes the more frequent one's word.
func (p *Proto) genSenses(r *rng.Stream) {
	v := p.Variety
	cs := Concepts()
	for _, c := range cs {
		if c.Role == RoleCulture && !r.Permille(oldCulturePermille) {
			v.Set(c.ID, Lexeme{Concept: c.ID, Freq: c.Freq, Stratum: Native, Hash: FormHash(phon.Word{})})
		}
	}
	merged := make([]bool, len(cs))
	for _, c := range cs {
		for _, e := range Colex(c.ID) {
			if e.To < c.ID || e.Families < polyFamilies || !r.Permille(polyPermille) {
				continue
			}
			a, b := c.ID, e.To // b takes a's word
			if cs[a].Freq > cs[b].Freq || cs[a].Freq == cs[b].Freq && cs[a].Stab > cs[b].Stab {
				a, b = b, a
			}
			la, lb := v.Lex.At(a), v.Lex.At(b)
			if merged[a] || merged[b] || la.Absent() || lb.Absent() || TrackedIndex(b) >= 0 {
				continue
			}
			merged[a], merged[b] = true, true
			v.Set(b, Lexeme{Concept: b, Form: la.Form, Freq: lb.Freq, Stratum: Native, Etym: la.Etym, Hash: la.Hash})
		}
	}
}
