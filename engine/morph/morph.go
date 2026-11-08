// Package morph models inflection and word order: grammatical slots whose
// exponents move through the grammaticalization cycle, paradigms of stored
// cell forms (stem + exponents with tagged '+'/'=' boundaries, so sound
// changes see and erode them), and word-order parameters with WALS priors.
//
// The package is pure data and helpers; the history package applies
// grammatical innovations to varieties.
package morph

import "isogloss/engine/phon"

// POS is a part of speech with a paradigm.
type POS uint8

const (
	Noun POS = iota
	Verb
)

func (p POS) String() string {
	if p == Verb {
		return "verb"
	}
	return "noun"
}

// Slot is one grammatical category value that can have an exponent.
type Slot uint8

const (
	PL Slot = iota
	ACC
	GEN
	DAT
	LOC
	DEF
	PST
	FUT
	PRF
	PROG
	P1SG
	P2SG
	P3SG
	P1PL
	P2PL
	P3PL
	NumSlots
)

// Group orders exponents inside a word: lower groups attach nearer the stem
// (number inside case, tense inside person; Greenberg's Universal 39, Bybee).
type Group uint8

const (
	GNumber Group = iota
	GCase
	GDef
	GTAM
	GAgr
)

// SlotInfo describes a slot.
type SlotInfo struct {
	Gloss string // Leipzig gloss
	Name  string // plain-language name
	POS   POS
	Group Group
	// Sources are concept labels that grammaticalize into this slot, after
	// Heine & Kuteva's World Lexicon of Grammaticalization.
	Sources []string
}

var slots = [NumSlots]SlotInfo{
	PL:   {"PL", "plural", Noun, GNumber, []string{"many", "all", "person"}},
	ACC:  {"ACC", "accusative", Noun, GCase, []string{"take", "at"}},
	GEN:  {"GEN", "genitive", Noun, GCase, []string{"thing", "from", "at"}},
	DAT:  {"DAT", "dative", Noun, GCase, []string{"give", "at", "near"}},
	LOC:  {"LOC", "locative", Noun, GCase, []string{"in", "belly", "back", "face", "head", "at"}},
	DEF:  {"DEF", "definite", Noun, GDef, []string{"that", "this"}},
	PST:  {"PST", "past", Verb, GTAM, []string{"finish", "have", "come"}},
	FUT:  {"FUT", "future", Verb, GTAM, []string{"go", "come", "want"}},
	PRF:  {"PRF", "perfect", Verb, GTAM, []string{"have", "hold", "finish"}},
	PROG: {"PROG", "progressive", Verb, GTAM, []string{"sit", "be", "stand", "lie"}},
	P1SG: {"1SG", "1st singular", Verb, GAgr, []string{"i"}},
	P2SG: {"2SG", "2nd singular", Verb, GAgr, []string{"thou"}},
	P3SG: {"3SG", "3rd singular", Verb, GAgr, []string{"he"}},
	P1PL: {"1PL", "1st plural", Verb, GAgr, []string{"we"}},
	P2PL: {"2PL", "2nd plural", Verb, GAgr, []string{"you"}},
	P3PL: {"3PL", "3rd plural", Verb, GAgr, []string{"they"}},
}

// Info returns a slot's description.
func (s Slot) Info() *SlotInfo { return &slots[s] }

// String returns the slot's gloss.
func (s Slot) String() string { return slots[s].Gloss }

// Tag is the boundary tag of the slot's exponent.
func (s Slot) Tag(prefix bool) uint8 {
	t := uint8(s) + 1
	if prefix {
		t |= phon.TagPrefix
	}
	return t
}

// SlotOfTag decodes a boundary tag; ok is false for untagged boundaries.
func SlotOfTag(t uint8) (s Slot, prefix, ok bool) {
	if t == 0 {
		return 0, false, false
	}
	return Slot(t&^phon.TagPrefix) - 1, t&phon.TagPrefix != 0, true
}

// Stage is a position in the grammaticalization cycle.
type Stage uint8

const (
	None        Stage = iota // no exponent (zero)
	Periphrasis              // a free construction: "go to eat"
	Fixed                    // frozen order, bleached meaning
	Clitic                   // unstressed, leans on its host ('=')
	Affix                    // part of the word ('+')
	Lost                     // eroded to zero; a new periphrasis may take over
)

var stageNames = [...]string{"none", "periphrasis", "fixed construction", "clitic", "affix", "lost"}

func (s Stage) String() string { return stageNames[s] }

// Bound reports whether the exponent is part of the word form.
func (s Stage) Bound() bool { return s == Clitic || s == Affix }

// Free reports whether the exponent is a separate word.
func (s Stage) Free() bool { return s == Periphrasis || s == Fixed }

// Exponent is how a variety expresses one slot.
type Exponent struct {
	Stage  Stage
	Source int32 // concept ID of the source word; -1 if inherited from before the proto-language
	Prefix bool
	Since  int32 // tick of the last stage change
}

// Cell is one paradigm cell: the slots it combines.
type Cell struct {
	Slots []Slot
	Label string
}

var (
	nounCells = []Cell{
		{nil, "SG"}, {[]Slot{PL}, "PL"},
		{[]Slot{ACC}, "ACC.SG"}, {[]Slot{PL, ACC}, "ACC.PL"},
		{[]Slot{GEN}, "GEN.SG"}, {[]Slot{PL, GEN}, "GEN.PL"},
		{[]Slot{DAT}, "DAT.SG"}, {[]Slot{PL, DAT}, "DAT.PL"},
		{[]Slot{LOC}, "LOC.SG"}, {[]Slot{PL, LOC}, "LOC.PL"},
		{[]Slot{DEF}, "DEF.SG"}, {[]Slot{PL, DEF}, "DEF.PL"},
	}
	verbCells = []Cell{
		{[]Slot{P1SG}, "1SG"}, {[]Slot{P2SG}, "2SG"}, {[]Slot{P3SG}, "3SG"},
		{[]Slot{P1PL}, "1PL"}, {[]Slot{P2PL}, "2PL"}, {[]Slot{P3PL}, "3PL"},
		{[]Slot{PST, P1SG}, "PST.1SG"}, {[]Slot{PST, P2SG}, "PST.2SG"}, {[]Slot{PST, P3SG}, "PST.3SG"},
		{[]Slot{PST, P1PL}, "PST.1PL"}, {[]Slot{PST, P2PL}, "PST.2PL"}, {[]Slot{PST, P3PL}, "PST.3PL"},
		{[]Slot{FUT, P3SG}, "FUT.3SG"}, {[]Slot{PRF, P3SG}, "PRF.3SG"}, {[]Slot{PROG, P3SG}, "PROG.3SG"},
	}
)

// Cells returns the paradigm cells of a part of speech. Cell 0 is the base
// (citation) cell. Do not modify.
func Cells(p POS) []Cell {
	if p == Verb {
		return verbCells
	}
	return nounCells
}

// BaseCell is the cell analogy levels from: the least marked, most frequent
// form (nominative singular; present 3rd singular).
func BaseCell(p POS) int {
	if p == Verb {
		return 2
	}
	return 0
}

// CellIndex finds the cell with exactly these slots, or -1.
func CellIndex(p POS, want ...Slot) int {
	for i, c := range Cells(p) {
		if sameSlots(c.Slots, want) {
			return i
		}
	}
	return -1
}

func sameSlots(a, b []Slot) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			found = found || x == y
		}
		if !found {
			return false
		}
	}
	return true
}

// Has reports whether a cell includes a slot.
func (c Cell) Has(s Slot) bool {
	for _, x := range c.Slots {
		if x == s {
			return true
		}
	}
	return false
}

// Partner returns the cell that differs from cell i only in lacking slot s,
// which is how loss of an exponent is detected (the two have merged). Person
// slots are exclusive, so a person cell's partner is the same tense with the
// 3rd singular (or, for 3SG, the 1st singular).
func Partner(p POS, i int, s Slot) int {
	c := Cells(p)[i]
	var rest []Slot
	for _, x := range c.Slots {
		if x != s {
			rest = append(rest, x)
		}
	}
	if s.Info().Group == GAgr {
		alt := P3SG
		if s == P3SG {
			alt = P1SG
		}
		rest = append(rest, alt)
	}
	return CellIndex(p, rest...)
}

// Paradigm holds a lexeme's cell forms, indexed like Cells(pos). Paradigms
// are immutable once stored in a variety; changes build a new one.
type Paradigm struct {
	Forms []phon.Word
	Hash  []uint64 // full form hash (lex.FormHash)
	Seg   []uint64 // segment features only, for syncretism checks
	Pat   []uint64 // affix pattern (everything but the stem), for finding the productive pattern
}

// SegHash hashes a form's segment features, ignoring stress, tone and
// boundaries: two cells with equal SegHash sound alike.
func SegHash(w phon.Word) uint64 {
	h := uint64(0xcbf29ce484222325)
	for _, s := range w.Segs {
		h ^= s.F.Val
		h *= 0x100000001b3
		h ^= s.F.Mask
		h *= 0x100000001b3
	}
	return h
}

// Pair is a cell with a slot and its partner without it.
type Pair struct{ Cell, Partner int }

var pairCache [NumSlots][]Pair

func init() {
	for s := Slot(0); s < NumSlots; s++ {
		pos := s.Info().POS
		for k, c := range Cells(pos) {
			if !c.Has(s) {
				continue
			}
			if j := Partner(pos, k, s); j >= 0 {
				pairCache[s] = append(pairCache[s], Pair{k, j})
			}
		}
	}
}

// Pairs lists the cells marked for s with their partner cells.
func Pairs(s Slot) []Pair { return pairCache[s] }

// State is a variety's morphology and word order. Para is indexed by
// tracked lexeme (see lex.Tracked) and shares paradigms copy-on-write.
type State struct {
	Exp   [NumSlots]Exponent
	Order Order
	Para  []*Paradigm
}

// Clone returns a copy that shares the paradigms.
func (s *State) Clone() State {
	n := *s
	n.Para = append([]*Paradigm(nil), s.Para...)
	return n
}

// StemSpan returns the stem's segment range [lo, hi) in a cell form: after
// the innermost tagged prefix boundary and before the innermost tagged
// suffix boundary. Untagged boundaries (inside compounds) belong to the stem.
func StemSpan(w phon.Word) (lo, hi int) {
	lo, hi = 0, len(w.Segs)
	for _, b := range w.Bounds {
		if b.Tag == 0 {
			continue
		}
		if b.Tag&phon.TagPrefix != 0 {
			lo = max(lo, b.At)
		} else {
			hi = min(hi, b.At)
		}
	}
	return lo, max(lo, hi)
}

// Split cuts a cell form into prefixes, stem and suffixes; each part keeps
// its own boundaries (with positions relative to the part).
func Split(w phon.Word) (pre, stem, suf phon.Word) {
	lo, hi := StemSpan(w)
	part := func(a, b int, incl func(at int) bool) phon.Word {
		p := phon.Word{Segs: append([]phon.Segment(nil), w.Segs[a:b]...)}
		for _, bd := range w.Bounds {
			if incl(bd.At) {
				bd.At -= a
				p.Bounds = append(p.Bounds, bd)
			}
		}
		return p
	}
	pre = part(0, lo, func(at int) bool { return at <= lo })
	stem = part(lo, hi, func(at int) bool { return at > lo && at < hi })
	suf = part(hi, len(w.Segs), func(at int) bool { return at >= hi })
	return
}

// Join is the inverse of Split.
func Join(pre, stem, suf phon.Word) phon.Word {
	var w phon.Word
	add := func(p phon.Word) {
		off := len(w.Segs)
		for _, b := range p.Bounds {
			b.At += off
			if b.At == 0 || len(w.Bounds) > 0 && w.Bounds[len(w.Bounds)-1] == b {
				continue
			}
			w.Bounds = append(w.Bounds, b)
		}
		w.Segs = append(w.Segs, p.Segs...)
	}
	add(pre)
	add(stem)
	add(suf)
	// A boundary at the very end delimits nothing.
	for len(w.Bounds) > 0 && w.Bounds[len(w.Bounds)-1].At >= len(w.Segs) {
		w.Bounds = w.Bounds[:len(w.Bounds)-1]
	}
	return w
}

// Attach adds exponent material at the outer edge of a form, delimited by a
// boundary of the given kind tagged with the slot. Material is unstressed.
func Attach(w phon.Word, mat []phon.Segment, s Slot, prefix bool, kind phon.BoundaryKind) phon.Word {
	if len(mat) == 0 {
		return w
	}
	m := make([]phon.Segment, len(mat))
	for i, x := range mat {
		x.Stress = 0
		m[i] = x
	}
	b := phon.Boundary{Kind: kind, Tag: s.Tag(prefix)}
	out := phon.Word{}
	if prefix {
		out.Segs = append(m, w.Segs...)
		b.At = len(m)
		out.Bounds = append(out.Bounds, b)
		for _, x := range w.Bounds {
			x.At += len(m)
			out.Bounds = append(out.Bounds, x)
		}
	} else {
		out.Segs = append(append([]phon.Segment(nil), w.Segs...), m...)
		out.Bounds = append(append([]phon.Boundary(nil), w.Bounds...), phon.Boundary{At: len(w.Segs), Kind: kind, Tag: b.Tag})
		if len(w.Segs) == 0 {
			out.Bounds = out.Bounds[:len(out.Bounds)-1]
		}
	}
	return out
}

// Rekind changes the kind of every boundary tagged with slot s (clitic →
// affix). It reports whether anything changed.
func Rekind(w phon.Word, s Slot, kind phon.BoundaryKind) (phon.Word, bool) {
	changed := false
	var out phon.Word
	for i, b := range w.Bounds {
		if sl, _, ok := SlotOfTag(b.Tag); ok && sl == s && b.Kind != kind {
			if !changed {
				out = w.Clone()
				changed = true
			}
			out.Bounds[i].Kind = kind
		}
	}
	if !changed {
		return w, false
	}
	return out, true
}

// SameSegments compares two forms' segment features, ignoring stress,
// tone and boundaries.
func SameSegments(a, b phon.Word) bool {
	if len(a.Segs) != len(b.Segs) {
		return false
	}
	for i := range a.Segs {
		if a.Segs[i].F != b.Segs[i].F {
			return false
		}
	}
	return true
}
