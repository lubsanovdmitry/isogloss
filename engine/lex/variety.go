package lex

import (
	"slices"

	"isogloss/engine/morph"
	"isogloss/engine/phon"
)

// EtymID names a root. Proto roots are ProtoRoot(concept); coined roots are
// allocated by the simulation from FirstCoinedRoot upward.
type EtymID uint32

const FirstCoinedRoot EtymID = 1 << 16

// ProtoRoot is the root of a concept's word in the proto-language.
func ProtoRoot(c ConceptID) EtymID { return EtymID(c) + 1 }

// Stratum records where a lexeme came from.
type Stratum struct {
	Donor int32 // -1 native; otherwise the donor language (M3)
	Tick  int32
}

// Native is the stratum of inherited words.
var Native = Stratum{Donor: -1}

// Lexeme is a variety's word for one concept. M1 keeps one lexeme per
// concept; polysemy (many-to-many senses) arrives with meaning drift in M3.
type Lexeme struct {
	Concept ConceptID
	Form    phon.Word
	Freq    uint8
	Stratum Stratum
	Etym    EtymID
	Hash    uint64 // FormHash(Form), kept current
}

// EntryKind distinguishes entries in a variety's adoption history.
type EntryKind uint8

const (
	EntryInnov EntryKind = iota // an innovation (restricted to freq classes Lo..Hi)
	EntryRelax                  // phonotactic template relaxed to Template
)

// Entry is one step of a variety's history. Replaying a variety's entries on
// the proto-language rebuilds it exactly, which is how etymologies are traced.
type Entry struct {
	Kind     EntryKind
	Innov    uint32
	Tick     int32
	Lo, Hi   uint8         // EntryInnov: frequency classes covered (1..5)
	Template phon.Template // EntryRelax
}

// Variety is a community's speech: the proto-language plus adopted
// innovations, in order. Varieties are immutable once built and shared
// between communities (copy-on-write).
type Variety struct {
	Key      uint64    // hash of the history
	Hist     *HistNode // adoption history, newest first (persistent list)
	Lex      Lexicon   // indexed by ConceptID
	Phonotax phon.Phonotactics
	Stress   phon.StressRule
	Repair   phon.RepairMode
	Epenth   phon.Segment
	M        morph.State // exponents, word order, tracked paradigms

	// Vacated lists phonemes the most recent sound change removed, so a
	// change moving a neighbour into the gap can be boosted (chain shifts).
	Vacated []phon.FeatureSet
	VacTick int32

	counts []segCount        // phoneme occurrence counts, sorted by features
	inv    []phon.FeatureSet // cached inventory
	prod   [2][]prodPattern  // cached productive affixes per part of speech
}

// Derive returns a mutable copy of v with entry e appended to its history.
// The lexicon slice is copied; word forms are shared until replaced.
func (v *Variety) Derive(e Entry) *Variety {
	n := &Variety{
		Key:      HashEntry(v.Key, e),
		Hist:     &HistNode{Parent: v.Hist, E: e, N: v.Hist.Len() + 1},
		Lex:      v.Lex.Clone(),
		Phonotax: v.Phonotax,
		Stress:   v.Stress,
		Repair:   v.Repair,
		Epenth:   v.Epenth,
		M:        v.M.Clone(),
		prod:     v.prod, // valid until a paradigm changes (SetPara)
		Vacated:  v.Vacated,
		VacTick:  v.VacTick,
		counts:   slices.Clone(v.counts),
	}
	n.Phonotax.Banned = slices.Clip(v.Phonotax.Banned)
	return n
}

// HistNode is one entry of a variety's history. Varieties derived from a
// common ancestor share the tail of the list.
type HistNode struct {
	Parent *HistNode
	E      Entry
	N      int32 // length of the list ending here
}

// Len returns the number of entries (0 for nil).
func (h *HistNode) Len() int32 {
	if h == nil {
		return 0
	}
	return h.N
}

// Adopted returns the history in order, oldest first.
func (v *Variety) Adopted() []Entry {
	out := make([]Entry, v.Hist.Len())
	for n := v.Hist; n != nil; n = n.Parent {
		out[n.N-1] = n.E
	}
	return out
}

// Push appends an entry to v's own history (v must be freshly derived).
func (v *Variety) Push(e Entry) {
	v.Hist = &HistNode{Parent: v.Hist, E: e, N: v.Hist.Len() + 1}
	v.Key = HashEntry(v.Key, e)
}

// HashEntry extends a history hash with one entry.
func HashEntry(h uint64, e Entry) uint64 {
	h = mix(h, uint64(e.Kind)<<56|uint64(e.Lo)<<48|uint64(e.Hi)<<40|uint64(e.Innov))
	if e.Kind == EntryRelax {
		t := e.Template
		h = mix(h, uint64(t.MinOnset)|uint64(t.MaxOnset)<<8|uint64(t.MinCoda)<<16|uint64(t.MaxCoda)<<24)
	}
	return h
}

func mix(h, x uint64) uint64 {
	h ^= x + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
	h *= 0xbf58476d1ce4e5b9
	return h ^ (h >> 31)
}

// FormHash hashes a word's segments and boundaries.
func FormHash(w phon.Word) uint64 {
	h := uint64(0xcbf29ce484222325)
	for _, s := range w.Segs {
		h = mix(h, s.F.Val)
		h = mix(h, s.F.Mask^uint64(s.Tone)<<58^uint64(s.Stress)<<61)
	}
	for _, b := range w.Bounds {
		h = mix(h, uint64(b.At)<<8|uint64(b.Kind)|uint64(b.Tag)<<16)
	}
	return h
}

// segCount is one phoneme with the number of its occurrences in the lexicon.
type segCount struct {
	F phon.FeatureSet
	N int32
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

func (v *Variety) count(w phon.Word, d int32) {
	for _, s := range w.Segs {
		i, found := slices.BinarySearchFunc(v.counts, s.F, func(c segCount, f phon.FeatureSet) int { return cmpFeat(c.F, f) })
		if found {
			v.counts[i].N += d
			if v.counts[i].N == 0 {
				v.counts = slices.Delete(v.counts, i, i+1)
			}
		} else if d > 0 {
			v.counts = slices.Insert(v.counts, i, segCount{s.F, d})
		}
	}
	v.inv = nil
}

// Set stores a lexeme, keeping inventory counts current.
func (v *Variety) Set(c ConceptID, l Lexeme) {
	if int(c) < v.Lex.Len() {
		v.count(v.Lex.At(c).Form, -1)
	}
	v.count(l.Form, +1)
	v.Lex.Set(c, l)
}

// Inventory returns the distinct segments (features only) used in the
// lexicon, sorted. Do not modify the result.
func (v *Variety) Inventory() []phon.FeatureSet {
	if v.inv != nil {
		return v.inv
	}
	out := make([]phon.FeatureSet, len(v.counts))
	for i, c := range v.counts {
		out[i] = c.F
	}
	v.inv = out
	return out
}

// Invalidate drops cached derived data. Inventory counts are maintained by
// Set, so this only clears the cached slice.
func (v *Variety) Invalidate() { v.inv = nil }

// Homophones counts core concepts whose form equals another core concept's.
func (v *Variety) Homophones() int {
	seen := map[uint64]int{}
	for i := 0; i < v.Lex.Len(); i++ {
		if concepts[i].Core() {
			seen[v.Lex.At(ConceptID(i)).Hash]++
		}
	}
	n := 0
	for _, c := range seen { //lint:ordered summing counts is order-independent
		if c > 1 {
			n += c
		}
	}
	return n
}

// SetForm replaces a lexeme's form and keeps its hash current.
func (v *Variety) SetForm(c ConceptID, w phon.Word) {
	l := *v.Lex.At(c)
	l.Form = w
	l.Hash = FormHash(w)
	v.Set(c, l)
}
