package phon

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"isogloss/engine/data"
)

// Symbol is one IPA table entry.
type Symbol struct {
	IPA  string // canonical NFD form; multi-letter segments carry a tie bar (t͡s)
	F    FeatureSet
	Freq int // PHOIBLE inventories per 10000
}

type diacritic struct {
	mark  string
	set   FeatureSet
	unset uint64
}

type ipaTable struct {
	symbols  []Symbol
	byIPA    map[string]int     // canonical symbol → index
	byFeat   map[FeatureSet]int // features → preferred symbol
	freqFeat map[FeatureSet]int // features → highest frequency among its symbols
	diac     []diacritic        // ordered by usefulness for rendering
	diacByMk map[string]int     // mark → index into diac
	nfc      map[rune]string    // precomposed → decomposed
	maxRunes int                // longest symbol, in runes
	cache    sync.Map           // FeatureSet → rendered string
}

var (
	tableOnce sync.Once
	table     *ipaTable
)

const tieBar = '͡'

func tbl() *ipaTable {
	tableOnce.Do(func() { table = loadTable() })
	return table
}

func isBase(r rune) bool {
	return unicode.IsLetter(r) && !unicode.Is(unicode.Lm, r)
}

// canonical inserts a tie bar between the first two base letters of a
// multi-letter segment (ts → t͡s), so that plain "ts" in input parses as two
// segments while t͡s parses as one affricate.
func canonical(sym string) string {
	rs := []rune(sym)
	bases := 0
	for i, r := range rs {
		if r == tieBar || r == '͜' {
			return sym
		}
		if isBase(r) {
			bases++
			if bases == 2 {
				return string(rs[:i]) + string(tieBar) + string(rs[i:])
			}
		}
	}
	return sym
}

func parseHex(s string) uint64 {
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		panic("phon: bad hex in data table: " + s)
	}
	return v
}

func dataLines(s string) [][]string {
	var out [][]string
	for _, line := range strings.Split(s, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		out = append(out, strings.Split(line, "\t"))
	}
	return out
}

func loadTable() *ipaTable {
	t := &ipaTable{
		byIPA:    map[string]int{},
		byFeat:   map[FeatureSet]int{},
		freqFeat: map[FeatureSet]int{},
		diacByMk: map[string]int{},
		nfc:      map[rune]string{},
	}
	for _, c := range dataLines(data.Segments) {
		f := FeatureSet{Val: parseHex(c[1]), Mask: parseHex(c[2])}
		freq, _ := strconv.Atoi(c[3])
		sym := Symbol{IPA: canonical(c[0]), F: f, Freq: freq}
		i := len(t.symbols)
		t.symbols = append(t.symbols, sym)
		t.byIPA[sym.IPA] = i
		if n := utf8.RuneCountInString(sym.IPA); n > t.maxRunes {
			t.maxRunes = n
		}
		if freq > t.freqFeat[f] {
			t.freqFeat[f] = freq
		}
		// Prefer the shortest, then the most frequent symbol for a bundle.
		if j, ok := t.byFeat[f]; !ok || betterSymbol(sym, t.symbols[j]) {
			t.byFeat[f] = i
		}
	}
	for _, c := range dataLines(data.Diacritics) {
		set := FeatureSet{Mask: parseHex(c[1]), Val: parseHex(c[2])}
		t.diacByMk[c[0]] = len(t.diac)
		t.diac = append(t.diac, diacritic{mark: c[0], set: set, unset: parseHex(c[3])})
	}
	for _, c := range dataLines(data.NFC) {
		r, _ := utf8.DecodeRuneInString(c[0])
		t.nfc[r] = c[1]
	}
	// Precomposed affricate ligatures.
	for k, v := range map[rune]string{'ʦ': "t͡s", 'ʣ': "d͡z", 'ʧ': "t͡ʃ", 'ʤ': "d͡ʒ", 'ʨ': "t͡ɕ", 'ʥ': "d͡ʑ", 'g': "ɡ"} { //lint:ordered map literal copy
		t.nfc[k] = v
	}
	return t
}

// betterSymbol prefers the shorter symbol (fewer diacritics), then the more
// frequent one. Symbols compared here usually share one feature bundle, so
// they denote the same sound and the plainest spelling wins (t͡ʃ over t̠͡ʃ).
func betterSymbol(a, b Symbol) bool {
	la, lb := utf8.RuneCountInString(a.IPA), utf8.RuneCountInString(b.IPA)
	if la != lb {
		return la < lb
	}
	if a.Freq != b.Freq {
		return a.Freq > b.Freq
	}
	return a.IPA < b.IPA
}

// Frequency returns how many PHOIBLE inventories per 10000 contain a segment
// with exactly these features (0 for bundles not in the table).
func Frequency(f FeatureSet) int { return tbl().freqFeat[f] }

// Symbols returns the IPA table (read-only).
func Symbols() []Symbol { return tbl().symbols }

// Lookup returns the feature bundle of one canonical IPA symbol.
func Lookup(ipa string) (FeatureSet, bool) {
	t := tbl()
	i, ok := t.byIPA[ipa]
	if !ok {
		return FeatureSet{}, false
	}
	return t.symbols[i].F, true
}

func (d diacritic) apply(f FeatureSet) FeatureSet {
	f = f.Apply(d.set)
	f.Mask &^= d.unset
	f.Val &^= d.unset
	return f
}

// RenderFeatures renders a feature bundle as IPA. A bundle with no exact
// symbol renders as the nearest base plus diacritics.
func RenderFeatures(f FeatureSet) string {
	t := tbl()
	if i, ok := t.byFeat[f]; ok {
		return t.symbols[i].IPA
	}
	if s, ok := t.cache.Load(f); ok {
		return s.(string)
	}
	if out, ok := t.exact(f); ok {
		t.cache.Store(f, out)
		return out
	}
	if out, ok := t.doubly(f); ok {
		t.cache.Store(f, out)
		return out
	}
	best, bestD := 0, 1<<30
	for i, s := range t.symbols {
		d := Distance(s.F, f)
		if d < bestD || (d == bestD && betterSymbol(s, t.symbols[best])) {
			best, bestD = i, d
		}
	}
	cur := t.symbols[best].F
	base, marks := t.symbols[best].IPA, ""
	for step := 0; step < 3 && bestD > 0; step++ {
		pick := -1
		for j, d := range t.diac {
			nf := d.apply(cur)
			if nd := Distance(nf, f); nd < bestD {
				pick, bestD = j, nd
			}
		}
		if pick < 0 {
			break
		}
		cur = t.diac[pick].apply(cur)
		marks += t.diac[pick].mark
	}
	out := compose(base, marks)
	t.cache.Store(f, out)
	return out
}

// exact looks for a base symbol plus up to three diacritics that denote
// exactly f, among the 48 bases nearest to it, taking diacritics only while
// each brings the bundle closer. The shortest spelling wins (then the base
// nearer f, then the earlier base and diacritics).
func (t *ipaTable) exact(f FeatureSet) (string, bool) {
	type cand struct{ i, d int }
	var cs []cand
	for i, s := range t.symbols {
		cs = append(cs, cand{i, Distance(s.F, f)})
	}
	slices.SortStableFunc(cs, func(a, b cand) int { return a.d - b.d })
	if len(cs) > 48 {
		cs = cs[:48]
	}
	best, bestN := "", 1<<30
	var dfs func(cur FeatureSet, base, marks string, n, from, depth int)
	dfs = func(cur FeatureSet, base, marks string, n, from, depth int) {
		if cur == f {
			if n < bestN {
				best, bestN = compose(base, marks), n
			}
			return
		}
		if depth == 3 || n+1 >= bestN {
			return
		}
		d0 := Distance(cur, f)
		for j := from; j < len(t.diac); j++ {
			nf := t.diac[j].apply(cur)
			if Distance(nf, f) < d0 {
				dfs(nf, base, marks+t.diac[j].mark, n+1, j+1, depth+1)
			}
		}
	}
	for _, c := range cs {
		s := t.symbols[c.i]
		dfs(s.F, s.IPA, "", utf8.RuneCountInString(s.IPA), 0, 0)
	}
	return best, bestN < 1<<30
}

// compose adds diacritics to a base symbol so that combining ones (̃ ̈ ̆)
// sit on the letter and spacing ones (ː ʰ ʲ) follow, including those the
// base symbol already ends with: ɨ̃ː plus ̈ is ɨ̈̃ː, not ɨ̃ː̈.
func compose(base, marks string) string {
	rs := []rune(base)
	cut := len(rs)
	for cut > 1 && !unicode.Is(unicode.Mn, rs[cut-1]) && !isBase(rs[cut-1]) {
		cut--
	}
	var comb, spacing []rune
	for _, r := range marks {
		if unicode.Is(unicode.Mn, r) {
			comb = append(comb, r)
		} else {
			spacing = append(spacing, r)
		}
	}
	return string(rs[:cut]) + string(comb) + string(rs[cut:]) + string(spacing)
}

// doubly spells a labial-velar consonant with no symbol of its own (a
// spirantized ɡ͡b) as its velar and labial halves joined by a tie bar
// (ɣ͡β), when each half has an exact spelling.
func (t *ipaTable) doubly(f FeatureSet) (string, bool) {
	if f.Has(Syllabic) || !f.Has(Labial) || !f.Has(Dorsal) || f.Has(Round) {
		return "", false
	}
	velar := f.Set(Labial, false).Unset(Round).Unset(Labiodental)
	labial := f.Set(Dorsal, false).Unset(High).Unset(Low).Unset(Front).Unset(Back).Set(Round, false).Set(Labiodental, false)
	v, ok := t.exact(velar)
	if !ok {
		return "", false
	}
	l, ok := t.exact(labial)
	if !ok {
		return "", false
	}
	return v + string(tieBar) + l, true
}

// Render renders one segment, with its tone mark.
func Render(s Segment) string {
	out := RenderFeatures(s.F)
	if s.Tone != ToneNone {
		out += string(toneMarks[s.Tone])
	}
	return out
}

// RenderWord renders a word with stress marks and morpheme/clitic boundaries.
// Syllables must be current (see Resyllabify) for stress marks to appear.
func RenderWord(w Word) string {
	var b strings.Builder
	bi := 0
	for i, s := range w.Segs {
		for bi < len(w.Bounds) && w.Bounds[bi].At == i {
			if i > 0 {
				b.WriteRune(w.Bounds[bi].Kind.Rune())
			}
			bi++
		}
		for _, sy := range w.Syll {
			if sy.Start == i {
				switch w.Segs[sy.Nucleus].Stress {
				case 1:
					b.WriteString("ˈ")
				case 2:
					b.WriteString("ˌ")
				}
			}
		}
		b.WriteString(Render(s))
	}
	return b.String()
}

// ParseError reports a position in the input string.
type ParseError struct {
	Input string
	Pos   int // rune offset
	Msg   string
}

func (e *ParseError) Error() string {
	return "ipa: " + e.Msg + " at rune " + strconv.Itoa(e.Pos) + " in " + strconv.Quote(e.Input)
}

// Normalize decomposes precomposed characters the engine knows about.
func Normalize(s string) string {
	t := tbl()
	var b strings.Builder
	for _, r := range s {
		if d, ok := t.nfc[r]; ok {
			b.WriteString(d)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isToneMark(r rune) (Tone, bool) {
	for i, m := range toneMarks {
		if i > 0 && m == r {
			return Tone(i), true
		}
	}
	return 0, false
}

// ParseSegments parses a run of IPA (no stress or boundary marks) into segments.
func ParseSegments(s string) ([]Segment, error) {
	w, err := ParseWord(s)
	if err != nil {
		return nil, err
	}
	return w.Segs, nil
}

// ParseWord parses an IPA word. It understands stress marks (ˈ ˌ, applied to
// the next nucleus), syllable breaks (. — ignored; the syllabifier decides),
// boundaries (+ = #, word-edge # ignored) and tone diacritics on vowels.
// Syllables are not computed; call Resyllabify.
func ParseWord(s string) (Word, error) {
	t := tbl()
	in := []rune(Normalize(strings.TrimSpace(s)))
	var w Word
	var pending int8
	for i := 0; i < len(in); {
		r := in[i]
		switch r {
		case 'ˈ', '\'':
			pending = 1
			i++
			continue
		case 'ˌ':
			pending = 2
			i++
			continue
		case '.':
			i++
			continue
		case '+', '=', '#':
			if r == '#' && (len(w.Segs) == 0 || i == len(in)-1) {
				i++
				continue
			}
			k := map[rune]BoundaryKind{'+': BMorpheme, '=': BClitic, '#': BWord}[r]
			w.Bounds = append(w.Bounds, Boundary{At: len(w.Segs), Kind: k})
			i++
			continue
		}
		if unicode.IsSpace(r) {
			return w, &ParseError{s, i, "space inside word"}
		}
		// Longest match against the table.
		n := t.maxRunes
		if rem := len(in) - i; rem < n {
			n = rem
		}
		matched := 0
		var seg Segment
		for ; n > 0; n-- {
			if j, ok := t.byIPA[string(in[i:i+n])]; ok {
				seg.F = t.symbols[j].F
				matched = n
				break
			}
		}
		if matched == 0 {
			return w, &ParseError{s, i, "unknown symbol " + strconv.QuoteRune(r)}
		}
		i += matched
		// Trailing diacritics and tone marks.
		for i < len(in) {
			if tone, ok := isToneMark(in[i]); ok {
				seg.Tone = tone
				i++
				continue
			}
			if j, ok := t.diacByMk[string(in[i])]; ok {
				seg.F = t.diac[j].apply(seg.F)
				i++
				continue
			}
			if in[i] == tieBar {
				return w, &ParseError{s, i, "tie bar joins an unknown pair"}
			}
			if unicode.Is(unicode.Mn, in[i]) {
				return w, &ParseError{s, i, "unknown diacritic " + strconv.QuoteRune(in[i])}
			}
			break
		}
		if pending != 0 && seg.IsVowel() {
			seg.Stress = pending
			pending = 0
		}
		w.Segs = append(w.Segs, seg)
	}
	if len(w.Segs) == 0 {
		return w, errors.New("ipa: empty word")
	}
	return w, nil
}

// MustParseWord is ParseWord for tests and built-in tables; it panics on error.
func MustParseWord(s string) Word {
	w, err := ParseWord(s)
	if err != nil {
		panic(err)
	}
	return w
}

// Spell writes a bundle for rule source: its IPA when that parses back to
// exactly this bundle, otherwise a full feature matrix, so a rule built
// from an inventory segment matches that segment and nothing else.
func Spell(f FeatureSet) string {
	s := RenderFeatures(f)
	if seg, err := ParseSegments(s); err == nil && len(seg) == 1 && seg[0].F == f {
		return s
	}
	return f.String()
}

// Labels renders an inventory so that distinct bundles get distinct
// labels: a bundle whose IPA collides with another's and does not denote it
// exactly gets the features its IPA misstates, as in ɣʷ[-round].
func Labels(fs []FeatureSet) []string {
	out := make([]string, len(fs))
	n := map[string]int{}
	for i, f := range fs {
		out[i] = RenderFeatures(f)
		n[out[i]]++
	}
	for i, f := range fs {
		if n[out[i]] < 2 {
			continue
		}
		seg, err := ParseSegments(out[i])
		if err != nil || len(seg) != 1 || seg[0].F == f {
			continue
		}
		g := seg[0].F
		var diff FeatureSet
		for k := Feature(0); k < NumFeatures; k++ {
			v, ok := f.Get(k)
			w, wok := g.Get(k)
			if ok && (!wok || v != w) {
				diff = diff.Set(k, v)
			}
		}
		out[i] += diff.String()
	}
	return out
}
